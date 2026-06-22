---
id: structure
code: "01"
title: 整体项目结构
status: target
code_paths:
  - .
depends_on: [common]
impacts: [testing]
children: []
---

# 01 · 整体项目结构

> 本文件定义技术栈、环境模型、monorepo 结构、前后端分层与目录、迁移/seed、模块依赖总览与核心数据流。
> 与 `00-common.md` 配合构成所有模块文档的共享基座。

---

## 1. 技术栈（固化）

### 后端 `services/admin-api`
- 语言：Go（沿用现有 `go.mod`，module 路径 `github.com/csw/console/services/admin-api`）。
- 路由：**chi**（`github.com/go-chi/chi/v5`）。替换现有 `net/http.ServeMux` 手写前缀匹配。
- 数据库驱动：**pgx**（`github.com/jackc/pgx/v5` + `pgxpool`）。
- 迁移：**golang-migrate**（`migrate`），迁移文件维持 `migrations/NNNNNN_*.up.sql / .down.sql`。
- 配置：现有 `internal/infra/config`（环境变量 + `.env`）。
- 日志：结构化日志（`log/slog`）。
- 测试：标准 `testing` + `httptest`；集成/接口测试跑**真实 PostgreSQL**（容器/本地实例）+ 真实迁移 + seed，并由 scenario manifest 驱动「接口场景矩阵」（详见 `03-testing.md`）。
- 密码哈希：`bcrypt`；JWT：`github.com/golang-jwt/jwt/v5`。
- 加密：AES-GCM（密钥来自配置/KMS），封装在 `infra/crypto`。

### 前端 `apps/admin-web`
- `Vue 3 + Vite + TypeScript`，状态 `Pinia`，路由 `Vue Router`，UI `Element Plus`。
- 思路沿用 `pure-admin-thin` 薄壳（已去除 demo）。
- 测试：组件级 `vitest` + `@testing-library/vue`；跨栈 e2e / 真实页面截图 / trace / HTML report / 可视回归用 **Playwright**（详见 `03-testing.md`）。
- 包管理：`pnpm`。

---

## 2. 环境模型

- 三环境：`develop` / `sandbox` / `production`（详见 `00` §2）。
- 单库多 env（D1）：同一物理库内用 `env` 列区分；同步在同库内进行。
- 当前运行环境由 `APP_ENV` 指定，前端通过 `/api/admin/me` 或专用接口获取并常驻展示（`EnvironmentBadge`）。
- `production` 运行环境下，前端隐藏一切 `Sync to Production` 入口。

---

## 3. Monorepo 结构

```text
console/
  apps/
    admin-web/            # 前端管理后台
  services/
    admin-api/            # 后端 Go 服务
  docs/
    architecture/
      v2/                 # 本文档集（目标架构）
      ...（旧文档归档保留）
```

---

## 4. 后端目录与分层

分层原则：`domain`（纯领域，无 IO）→ `app`（编排，command/query/dto）→ `infra`（持久化/crypto/file/fx）→ `transport`（HTTP）。依赖方向单向向内。

```text
services/admin-api/
  cmd/admin-api/main.go            # 装配：config -> pgxpool -> repos -> services -> router -> server
  migrations/                      # golang-migrate 迁移
  internal/
    domain/
      common/                      # Environment/Market/枚举/值对象/currency 归一化纯逻辑
      admin/                       # 管理员、角色、权限
      auth/                        # 令牌、身份
      game/                        # 游戏聚合
      channel/                     # GameMarketChannel、可见性、复制、隐藏
      product/                     # 商品、覆盖解析
      cashier/                     # 模板版本、价格行、汇率
      payment/                     # 路由匹配/唯一性
      sync/                        # section/diff/baseline
      snapshot/                    # 运行时配置合并、快照
    app/
      command/                     # 写用例
      query/                       # 读用例
      dto/                         # 传输对象
    infra/
      config/
      persistence/postgres/        # pgx 仓储实现，每聚合一个 repo
      crypto/                      # AES-GCM 加解密
      file/                        # 文件上传/对象存储抽象
      fx/                          # 汇率源抽象
    transport/
      http/
        admin/  auth/  games/  channels/  products/  cashier/  payment/  sync/  snapshot/  audit/
        middleware/                # 鉴权、权限、env 上下文、审计、recover
      httpserver/                  # chi router 装配
```

### 4.1 装配链（main.go 目标形态）
```text
MustLoad(config) -> pgxpool.New -> NewXxxRepository(pool)
   -> NewXxxService(repo, crypto, file, ...) -> NewXxxHandler(service)
   -> httpserver.New(cfg, handlers...) -> ListenAndServe
```
> 现状：`main.go` 未连库，`httpserver` 用 scaffold service。v2 目标是用真实 repo/service 替换全部 scaffold。

### 4.2 仓储接口原则
- 仓储保持**窄**：单聚合 CRUD + 必要查询，不放跨表编排。
- 跨表编排、差异计算、模板驱动校验放在 `app` 服务层。
- 所有仓储方法接收 `ctx` 与 `env`（带 env 的表）。

### 4.3 跨表 env 一致性（复合外键优先）
- 「游戏维度业务表」之间的引用**优先在 DB 层用复合唯一键 +（同 env）复合外键**强制同 env，而非仅靠应用层（`00` §2.2）：
  - 被引用表暴露含 `env` 的复合唯一键（如 `game_channels UNIQUE(env, game_id_ref, market_code, channel_id_ref)`）。
  - 引用表以 `FOREIGN KEY (env, <ref>) REFERENCES <parent>(env, ...)` 复合外键绑定，使「父子行 env 不一致」在 DB 层即不可写入。
- 仅在 PG 复合外键确实不可行（如引用目标为合成键、跨可空列）时降级为应用层 env 校验，并在该表数据模型小节注明降级原因。
- 平台级表（无 env）被业务表引用时按普通外键处理；`audit_logs` 不参与上述约束（见 `00` §2.2 特例）。

---

## 5. 前端目录与分层

```text
apps/admin-web/src/
  api/
    http.ts                        # axios 实例 + 鉴权拦截 + 统一包络解包 + 错误处理
    modules/                       # 按业务模块的 API 客户端（games.ts, channels.ts, ...）
  stores/
    auth.ts permission.ts app.ts dictionary.ts
  router/
    index.ts routes.ts            # 路由分组 + 守卫
  layouts/AdminLayout.vue
  components/
    page/                          # PageCard / PageStatusTag / EnvironmentBadge / 列表-详情-抽屉-差异抽屉 等通用容器
  views/
    dashboard/ games/ channels/ cashier/ payment/ audit/ system/ login/
  styles/                          # 设计 token（颜色/间距/圆角/字号/表格密度）
```

### 5.1 路由分组
```text
/login
/dashboard
/games            (列表) -> /games/:gameId (详情，含多 Tab)
/cashier          (模板列表/详情/版本/价格矩阵/汇率审核)
/payment          (pay way/provider/主体/商户/路由)
/audit
/system           (基础数据/字典/模板/管理员与权限)
```

### 5.2 Pinia stores
- `auth`：token、当前用户、登录/登出/刷新。
- `permission`：权限码集合、`hasPerm(code)`、动态路由/菜单过滤。
- `app`：当前 `environment`、全局 UI 状态。
- `dictionary`：全局枚举/基础数据缓存（与 `00` §3 一致）。

### 5.3 通用 UI 契约
- 页面优先**抽屉式**交互，少跳新页面。
- 列表/详情/抽屉表单/差异抽屉/状态标签/密文展示统一走 `components/page`。
- 模板驱动表单统一一个渲染器（消费 `form_schema_json` 四件套）。
- 配置异常态（`empty/invalid`）不得隐藏，必须行内可见。
- 写/危险操作统一挂权限指令（无权限置灰或隐藏）。

---

## 6. 迁移与 seed

- 迁移目录 `services/admin-api/migrations/`，golang-migrate 顺序执行。
- v2 需要新增迁移（不改历史迁移内容，追加新文件）：
  - 给带 env 的表加 `env` 列并调整唯一约束（D1）。
  - 给 `game_channels` 加 `market_code` 并改唯一键（D2）。
  - 给 `channels` 加 `region` 并回填 seed（D3）。
  - 为"游戏维度业务表"间引用补**复合唯一键 + 同 env 复合外键**（§4.3）；无法落地复合外键处保留应用层校验并在迁移注释说明。
- seed（`000002` + 后续）固定基础数据，见 `00` 各表 seed 值。`region` 回填：
  - `domestic`：`huawei_cn` / `xiaomi_cn` / `oppo_cn` / `vivo_cn` / `wechat_mini_game` / `douyin_mini_game`
  - `overseas`：`google` / `apple`
- seed 必须幂等（`ON CONFLICT DO NOTHING`）。

---

## 7. 模块依赖总览

> 节点标注稳定 `id`（括号内为排序编号，仅供定位，引用一律用 `id`，见 `CONVENTIONS §3.2`）。

```text
                 ┌─────────────────────────┐
                 │  公共: 枚举/模板/currency │
                 │  /密文/审计/env (common)  │
                 └─────────────┬────────────┘
                               │ 被所有模块依赖
   ┌───────────────────────────┼───────────────────────────┐
   │                           │                            │
[auth]                     [game]                  [基础数据/字典(折叠入各模块+system)]
   │                           │
   │              ┌────────────┼───────────────┬───────────────┐
   │        [channel]   [account-auth]    [product]      [game-cashier]
   │              │       [channel-login]      │               │
   │              │       [feature-plugin]     │               │
   │      [cashier-template]──────────────────┘           [payment]
   │              │                                            │
   │              └──────────────┬─────────────────────────────┘
   │                        [snapshot]
   │                             │
   │                          [sync]
   │
[audit] 横切所有写操作        [dashboard] 聚合只读视图
```

依赖要点：
- `channel` 依赖 `game`（market 集合）+ `公共可见性规则`。
- `snapshot` 聚合 `game`→`payment`（即 `game / channel / account-auth / channel-login / feature-plugin / product / cashier-template / game-cashier / payment`）的"有效数据"，按 market 合并产出最终配置。
- `sync` 以 `snapshot` 的快照/section 数据为基础，按 env diff。
- `dashboard` 只读聚合（汇率待审、配置异常、同步状态）。
- `testing`（`03-testing.md`）横切：每个模块的接口场景矩阵 + 跨栈回归都依赖对应模块，改模块需联动核对测试。

---

## 8. 核心数据流：sandbox → production

```text
1. 运营在 sandbox 完成配置（games/channels/products/cashier/payments...，env=sandbox）
2. 生成 config snapshot（per-game，按 market 合并，env=sandbox）
3. POST /sync/preview：对比 sandbox 与 production 同一 game 的各 section，产出按 section 的 add/update/delete 差异，
   返回 baseline_token(含 target_hash_before)，密文 masked
4. 运营勾选 selected_sections（+ 可选 include_deletes）
5. POST /sync/execute：携带 baseline_token；服务端复核 production 当前 hash == target_hash_before，
   不一致 -> SYNC_BASELINE_MISMATCH，要求重新预览
6. 一致则按 section 有序 upsert 到 env=production，写 sync_jobs / sync_job_items / audit_logs
7. 被隐藏/不兼容/无效数据全程排除
```

---

## 9. 与现状的差距（仅说明，不在本文落实）

- `main.go` 未连库、`httpserver` 全 scaffold service → v2 全量替换为真实仓储/服务。
- 业务表缺 `env` / `game_channels` 缺 `market_code` / `channels` 缺 `region` → 新增迁移补齐。
- 鉴权、商品、收银台、支付、快照、同步执行均为桩 → 按各模块文档实现。
- 执行细节由 `writing-plans` 产出的实现计划承接。
