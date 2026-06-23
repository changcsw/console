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
- 单库多 schema（D1）：同一物理库内每个环境一个同名 schema（`develop` / `sandbox` / `production`），"游戏维度业务表"在每个环境 schema 下各一份同名同结构表，**不带 `env` 列**；平台级共享数据在 `platform` schema。
- 运行时按当前环境设置连接的 `search_path = <当前env>, platform`，业务表自动落当前环境 schema、平台表落 `platform`；同步在同库内**跨 schema**（`sandbox.*` ↔ `production.*`）进行。
- **最小权限角色加固（必读）**：`search_path` 只是路由，不是隔离边界——单库多 schema 必须配套「每环境独立连接池（建连时钉死 `search_path`）+ 每环境最小权限 DB 角色（对其它环境 schema 无任何权限）」，从权限层杜绝误连/误查/误写；跨环境只允许 `sync` 专用角色 + 全限定名。详见 §4.4。
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
- 所有仓储方法接收 `ctx`；业务表仓储的 SQL **不写 schema 前缀、不带 `env` 谓词**，目标环境 schema 由连接**固定的** `search_path` 决定（`00` §2.1）。`search_path` 由**每环境独立连接池在建连时一次性钉死**（§4.4），**不在请求层逐次 `SET`**（避免连接池归还后泄漏到下一个借用者的 footgun）。
- 仅 `sync` 域仓储需要**显式跨 schema**（用 `sandbox.<表>` / `production.<表>` 限定名，且用专用最小权限角色连接），因为它要同时读写源/目标两个环境（见 §4.4 与 `sync` §3）。

### 4.3 表间引用与环境隔离
- 「游戏维度业务表」之间的引用在**同一环境 schema 内**用普通外键即可：父子行必然同 schema（= 同 env），不存在「父子 env 不一致」的可能，**无需复合外键、也无需应用层 env 一致性校验**（`00` §2.2）。
- 业务表引用平台级表时跨 schema 指向 `platform.<表>`（如 `game_channels.channel_id_ref REFERENCES platform.channels(id)`），用普通外键即可。
- `audit_logs` / `sync_jobs` 等在 `platform` schema，按普通表处理（见 `00` §2.2 特例）。

### 4.4 连接与权限加固（search_path 安全模型，D1 配套）

> 背景：单库多 schema 下，未写全限定名的 SQL 会按连接的 `search_path` 命中**第一个**同名 schema；若用默认 `public` + 共享高权限角色，存在「误连 / 误查 / 误写」到其它环境 schema 的风险（PostgreSQL 官方亦说明默认配置只适合彼此信任的少量用户）。本节把「靠 `search_path` 隐式路由」加固为「**最小权限角色 + 固定 `search_path`**」，从权限层结构性杜绝跨环境误写。以下为**强约束**，与 D1（`00` §2）配套。

#### 4.4.1 每环境一个独立连接池，`search_path` 在建连时固定
- 运行环境由 `APP_ENV` 固定（`00` §2.1）。主连接池在 **pgx 的 `AfterConnect` 钩子**里对每条物理连接执行一次 `SET search_path = <APP_ENV>, platform`（**不含 `public`**），此后该连接整个生命周期内不再改 `search_path`。
- **禁止「每请求 `SET search_path` 后归还连接池」**：这是连接池下最大的 footgun——连接被下一个请求借用时会继承到错误的 `search_path`。env 由连接池在建连时一次性钉死，请求层不再切 `search_path`。
- 业务表仓储 SQL 仍**不写 schema 前缀、不带 `env` 谓词**（§4.2）；落到哪个 env 由该池连接固定的 `search_path` 决定。

#### 4.4.2 每环境一个最小权限 DB 角色
- 为每个环境建独立登录角色 `app_develop` / `app_sandbox` / `app_production`，各自**只**对「自己的环境 schema + `platform`」授权，对**其它环境 schema 连 `USAGE` 都不授**：

```sql
-- 以 sandbox 为例（develop / production 同构，仅 schema 名不同）
REVOKE ALL ON SCHEMA public FROM app_sandbox;                 -- 不依赖 public
GRANT USAGE ON SCHEMA sandbox, platform TO app_sandbox;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA sandbox  TO app_sandbox;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA platform TO app_sandbox;
ALTER DEFAULT PRIVILEGES IN SCHEMA sandbox  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_sandbox;
ALTER DEFAULT PRIVILEGES IN SCHEMA platform GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_sandbox;
-- 关键：对 production / develop schema 不授任何权限
```

- 效果：即便代码写错或 `search_path` 被意外串改，`app_sandbox` 也**物理无法**读/写 `production.*`（直接 `permission denied`）。这把官方「默认配置只适合彼此信任的少量用户」翻译成「**我们根本不用默认配置**」。
- 应用运行时连库一律用 `app_<env>` 角色；**禁止用超级用户 / 库 owner 跑业务请求**。建 schema、跑迁移、建角色用单独的高权限角色（见 §6）。

#### 4.4.3 sync 域：唯一的跨环境通道，专角色 + 全限定名
- `sync` 是唯一允许同时访问两个环境 schema 的域（`00` §2.1）。它**不复用** `app_<env>` 主池，而是用一条专用、被审计的连接（角色 `app_sync`）：对 `sandbox` 授 `SELECT`、对 `production` 授 `SELECT, INSERT, UPDATE, DELETE`、对 `platform` 授读写（写 `sync_jobs` / `sync_job_items` / `sync_consumed_tokens` / `audit_logs`）。

```sql
GRANT USAGE ON SCHEMA sandbox, production, platform TO app_sync;
GRANT SELECT                          ON ALL TABLES IN SCHEMA sandbox    TO app_sync;  -- 源：只读
GRANT SELECT, INSERT, UPDATE, DELETE  ON ALL TABLES IN SCHEMA production TO app_sync;  -- 目标：读写
GRANT SELECT, INSERT, UPDATE, DELETE  ON ALL TABLES IN SCHEMA platform   TO app_sync;  -- 任务/审计流水
```

- `app_sync` 连接的 `search_path` 仍固定，但 sync 仓储 SQL **必须写全限定名** `sandbox.<表>` / `production.<表>`（§4.2），**不依赖** `search_path` 解析跨环境对象。
- 这样「危险的跨环境写」被收敛成**一条显式、集中、可审计**的路径，而非散落在各业务仓储里的隐式 `search_path` 行为。

#### 4.4.4 装配与配置（main.go 目标形态）
- `main.go` 按 `APP_ENV` 装配连接池：
  - 始终创建 `poolMain`（角色 `app_<APP_ENV>`，`search_path=<APP_ENV>,platform`）供全部业务仓储使用。
  - **仅当 `APP_ENV=sandbox`** 时，额外创建 `poolSync`（角色 `app_sync`）注入 `SyncService`；其它环境**不创建** `poolSync`（production 运行环境无同步入口，`00` §9、`sync` §8）。
- 角色名、连接串与 `search_path` 由 `infra/config` 按 `APP_ENV` 决定；`AfterConnect` 钩子统一在 `infra/persistence/postgres` 的池构造函数里设置，避免散落。

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
- schema 划分与迁移组织（D1）：
  - **bootstrap 迁移**先创建 schema：`platform`、`develop`、`sandbox`、`production`。
  - **bootstrap 同时建最小权限角色与授权**（§4.4）：登录角色 `app_develop` / `app_sandbox` / `app_production` 各自只授「本环境 schema + `platform`」（读写），对其它环境 schema 不授权；`app_sync` 授「`sandbox` 只读 + `production` 读写 + `platform` 读写」。迁移/建 schema/建角色由单独的**高权限角色（库 owner）**执行，应用运行时一律用 `app_*` 角色（禁用超级用户跑业务请求）。角色与授权迁移须**幂等**（`CREATE ROLE` 前置 `pg_roles` 存在性判断，`GRANT` 可重复执行；对后续新建表用 `ALTER DEFAULT PRIVILEGES` 兜住）。
  - **平台表 DDL** 只在 `platform` schema 建一份（含 `admin_*`、字典/模板/基础数据、`sync_jobs`/`sync_job_items`/`sync_consumed_tokens`、`audit_logs`）。
  - **业务表 DDL**（schema 无关、不带 `env` 列）需在 `develop` / `sandbox` / `production` 三个 schema 各应用一份；通过迁移运行器对三个环境 schema 循环执行同一套业务迁移（每个 schema 各自维护一份 `schema_migrations` 历史），保证三套结构始终一致。
  - 业务表唯一键**不含 `env`**（D1）；`game_channels` 唯一键 `(game_id_ref, market_code, channel_id_ref)` 并加 `market_code`（D2）。
  - `channels` 加 `region` 并回填 seed（D3，平台表，建在 `platform`）。
  - 业务表→平台表的跨 schema 外键指向 `platform.<表>`；业务表→业务表外键在本环境 schema 内（§4.3）。
- seed（基础数据）固定写入 `platform` schema，见 `00` 各表 seed 值。`region` 回填：
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
- `sync` 以 `snapshot` 的快照/section 数据为基础，跨 schema（`sandbox` ↔ `production`）diff。
- `dashboard` 只读聚合（汇率待审、配置异常、同步状态）。
- `testing`（`03-testing.md`）横切：每个模块的接口场景矩阵 + 跨栈回归都依赖对应模块，改模块需联动核对测试。

---

## 8. 核心数据流：sandbox → production

```text
1. 运营在 sandbox 完成配置（games/channels/products/cashier/payments...，写入 sandbox schema）
2. 生成 config snapshot（per-game，按 market 合并，存 sandbox.game_config_snapshots）
3. POST /sync/preview：对比 sandbox.* 与 production.* 同一 game 的各 section，产出按 section 的 add/update/delete 差异，
   返回 baseline_token(含 target_hash_before)，密文 masked
4. 运营勾选 selected_sections（+ 可选 include_deletes）
5. POST /sync/execute：携带 baseline_token；服务端复核 production schema 当前 hash == target_hash_before，
   不一致 -> SYNC_BASELINE_MISMATCH，要求重新预览
6. 一致则按 section 有序 upsert 到 production schema，写 platform.sync_jobs / platform.sync_job_items / platform.audit_logs
7. 被隐藏/不兼容/无效数据全程排除
```

---

## 9. 与现状的差距（仅说明，不在本文落实）

- `main.go` 未连库、`httpserver` 全 scaffold service → v2 全量替换为真实仓储/服务。
- 现状所有表都在默认 `public` schema 且无环境隔离 → v2 重建为 `platform` + 三环境 schema，业务表去掉 `env` 列、按 schema 隔离；`game_channels` 补 `market_code` / `channels` 补 `region`。
- 鉴权、商品、收银台、支付、快照、同步执行均为桩 → 按各模块文档实现。
- 执行细节由 `writing-plans` 产出的实现计划承接。
