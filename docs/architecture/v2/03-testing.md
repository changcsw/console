---
id: testing
code: "03"
title: 测试体系与回归（全局契约）
status: target
code_paths:
  - tests
  - scripts/regression
  - services/admin-api/internal
  - apps/admin-web/src
depends_on: [common, structure, auth, game, channel, account-auth, channel-login, feature-plugin, product, cashier-template, game-cashier, payment, snapshot, sync, audit, dashboard]
impacts: []
children: []
---

# 03 · 测试体系与回归（全局契约）

> 本文件是 v2 架构文档集的**全局测试契约**：定义测试分层、目录结构（与代码结构对齐）、后端「接口场景矩阵」标准维度、前端 Playwright 截图、统一 fixtures、统一回归入口与报告产物。
> 各模块文档不再各自重定义测试方法，只在自己的 README「接口场景矩阵」小节列出**本模块接口 × 本文标准维度**的覆盖清单，并回链本文。
> 与 `00`（契约）、`01`（结构）配合：本文的「跨 env（schema 隔离）/ 审计 / 脱敏」等维度直接复用 `00` 的红线与约定。

---

## 1. 目标与原则

- **可按模块测，也可整体回归**：测试结构与代码功能结构（`domain/<module>`、`views/<module>`）对齐，既能只跑单模块，又能一条命令跑全量回归。
- **真实依赖优先**：后端集成/接口测试跑**真实 PostgreSQL**（容器或本地实例）+ 真实迁移（含 `platform` + 三环境 schema 建立）+ seed，不用 mock 库糊弄跨 schema / 事务 / 唯一约束这类必须真实 DB 才能验证的场景。
- **前端截图用真实页面**：组件级逻辑用 `vitest`，**真实页面截图 / trace / 可视回归**用 **Playwright**。
- **场景可脚本化**：后端接口测试由 **scenario manifest** 驱动，覆盖成功与各类失败分支；维度清单见 §4。
- **产物统一**：所有测试输出统一汇集到 `tests/reports/`，回归脚本产出一份 `summary`。

---

## 2. 测试分层（5 层）

| 层 | 跑什么 | 工具 | 位置（hybrid） | 依赖 |
| --- | --- | --- | --- | --- |
| L1 单元 | `domain` 纯逻辑（枚举、状态机、currency 归一化、hash 规范化、route 匹配…） | Go `testing` | 就近 `internal/domain/<module>/*_test.go` | 无 IO |
| L2 集成 | `app` 服务 + `infra` 仓储（真实 PG、事务、唯一约束、跨 schema 外键、search_path 环境隔离） | Go `testing` + 真实 PG | 就近 `internal/app/.../*_test.go`、`internal/infra/.../*_test.go` | PG + migrate + seed |
| L3 接口 | `transport/http` 全链路（鉴权/权限/包络/错误码/分页/审计） | Go `httptest` + 真实 PG | 就近 `internal/transport/http/<module>/*_test.go` + 顶层 `tests/backend/scenarios` manifest | PG + 全装配 |
| L4 前端组件 | 组件/store/表单渲染器/权限指令 | `vitest` + `@testing-library/vue` | 就近 `apps/admin-web/src/**/*.spec.ts` | 无后端（mock API） |
| L5 跨栈 e2e + 截图 | 真实页面操作、截图、可视回归、关键操作主线 | **Playwright** | 顶层 `tests/frontend/e2e` | 真实前后端 + PG |

> 单元/集成/组件就近放（贴代码、改谁测谁）；跨栈 e2e、场景矩阵 manifest、fixtures、报告放顶层 `tests/`（统一回归入口）。

---

## 3. 目录结构（与代码结构对齐）

```text
console/
  services/admin-api/
    internal/
      domain/<module>/        *_test.go          # L1 单元（就近）
      app/command|query/      *_test.go          # L2 集成（就近，真实 PG）
      infra/persistence/...   *_test.go          # L2 仓储（就近，真实 PG）
      transport/http/<module> *_test.go          # L3 接口（就近，httptest）
  apps/admin-web/
    src/**/                   *.spec.ts          # L4 组件（就近，vitest）

  tests/                                          # 跨栈 / 统一回归（顶层）
    frontend/
      e2e/                    # L5 Playwright 用例，按模块拆：
        games.spec.ts  channels.spec.ts  cashier.spec.ts
        payment.spec.ts  sync.spec.ts  audit.spec.ts  dashboard.spec.ts
        login.spec.ts
      screenshots/            # 运行期采集的真实页面截图（产物，git 不跟踪正本）
      visual-baseline/        # 截图基线（git 跟踪，用于可视回归 diff）
    backend/
      api/                    # 跨模块接口 smoke（健康检查、鉴权链路、env 上下文）
      scenarios/              # 场景矩阵 manifest，按模块拆：
        auth.yaml  game.yaml  channel.yaml  account-auth.yaml
        channel-login.yaml  feature-plugin.yaml  product.yaml
        cashier-template.yaml  game-cashier.yaml  payment.yaml
        snapshot.yaml  sync.yaml  audit.yaml  dashboard.yaml
    fixtures/                 # 统一 seed/fixtures（按 schema 维度组织）
      common/                 # platform schema：字典/枚举/currency_specs/channels(region)/模板/admin_*
      sandbox/                # sandbox schema 业务数据样本
      production/             # production schema 基线样本（用于同步 diff 测试）
    reports/                  # 测试产物（junit.xml / html / trace / coverage / summary）

  scripts/
    regression/               # 统一回归入口脚本（见 §6）
      run.sh                  # 一键串联：deps -> migrate -> seed -> backend -> frontend -> summary
      backend.sh              # 仅后端全场景
      frontend.sh             # 仅前端 e2e + 截图
```

> 模块短名与 §F 各模块 `code_paths` 同源；新增模块时，`tests/backend/scenarios/<id>.yaml` 与 `tests/frontend/e2e/<id>.spec.ts` 一并新增，保持文档↔代码↔测试三者结构对齐。

---

## 4. 后端「接口场景矩阵」标准维度（权威清单）

每个**写/读接口**按下表维度被 scenario manifest 驱动执行；模块 README 的「接口场景矩阵」只声明「本接口覆盖哪些维度 + 期望结果」，不重复定义维度本身。

| # | 维度 | 触发 | 期望 | 关联 `00` |
| --- | --- | --- | --- | --- |
| S1 | 成功 | 合法请求 | 2xx + 统一包络 `{data}`；写操作落库正确 | §7.2 |
| S2 | 鉴权 | 缺/失效 `Bearer` | 401 `UNAUTHENTICATED` | §7.1/§7.5 |
| S3 | 权限 | 登录但缺该接口权限码 | 403 `FORBIDDEN` | §7.5 |
| S4 | 校验失败 | 缺必填/类型错/越界/格式错 | 400 `VALIDATION_FAILED`（含 `details`） | §7.4 |
| S5 | 冲突 | 唯一键/状态机非法流转/路由冲突 | 409 `CONFLICT` 系（`VERSION_STATE_INVALID`/`ROUTE_CONFLICT`/…） | §3.3/§7.4 |
| S6 | 跨 env（schema 隔离） | 写操作落**当前运行环境对应 schema**（由 `search_path` 决定，业务行无 `env` 列）；不允许前端指定/跨 schema 写；仅 sync 域显式跨 `sandbox`/`production` schema | schema 隔离正确，无越权写 production | §2 |
| S7 | 审计 | 有意义的写操作 | 写一条 `audit_logs`（`action` 同源权限码，detail 脱敏） | §8 |
| S8 | 脱敏 | 含 secret/file 字段的读/同步预览 | 响应密文恒 `masked`，绝不回明文 | §6 |
| S9 | 分页 | 列表接口 | `page/pageSize/total` 正确，`pageSize` 上限 100，`sort` 生效 | §7.3 |
| S10 | 事务回滚 | 多表写中途失败 | 整体回滚，无部分写入 | §9 |

适用性约定：
- **只读接口**：必覆盖 S1/S2/S3（若挂权限）/S9（列表）/S8（若含密文）；S4 覆盖非法 query。
- **写接口**：必覆盖 S1/S2/S3/S4/S5（若有唯一/状态约束）/S6/S7/S8（若含密文）/S10（若多表事务）。
- **危险/同步类接口**（如 `sync.execute`、`cashier.publish`）：额外覆盖各自模块私有维度（基线复核、nonce 幂等、版本流转等），在模块矩阵里列出。
- 模块矩阵中**不适用**的维度标 `—` 并简述原因（如「单表无事务 → S10 N/A」）。

### 4.1 scenario manifest 形态（约定，非实现）

每个 `tests/backend/scenarios/<id>.yaml` 描述一组 case：

```yaml
module: payment
cases:
  - name: create_route_success
    api: "POST /api/admin/games/{gameId}/payment-routes"
    dimension: S1
    auth: { role: payment_admin }
    fixture: sandbox/payment/base
    request: { ... }
    expect: { status: 200, bodyMatch: { data.routeId: "*" }, db: { payment_routes: +1 }, audit: "payment.route.create" }
  - name: create_route_conflict
    api: "POST /api/admin/games/{gameId}/payment-routes"
    dimension: S5
    expect: { status: 409, error.code: ROUTE_CONFLICT }
```

> manifest 字段（`auth/fixture/request/expect.db/expect.audit` 等）由后端 scenario harness 解释执行；harness 与具体 schema 在实现期定稿，本文只固化维度与产物口径。

---

## 5. 前端测试（vitest + Playwright）

### 5.1 组件级（vitest）
- 渲染器/表单四件套消费、权限指令置灰隐藏、状态标签、差异抽屉、env 徽标等用 `vitest + @testing-library/vue`，mock API。
- 关注**逻辑与可达性**，不负责像素级截图。

### 5.2 跨栈 e2e + 截图（Playwright）
- **真实页面截图**：关键页面/抽屉在关键状态下 `page.screenshot()`，存 `tests/frontend/screenshots/`。
- **trace**：失败用例保留 `trace`（`retain-on-failure`），与失败 artifacts 一并存 `tests/reports/playwright-artifacts/`。
- **HTML report**：输出到 `tests/reports/playwright-html/`，机器可读结果到 `tests/reports/playwright-results.json`。
- **可视回归**：与 `tests/frontend/visual-baseline/` 比对（`toHaveScreenshot`），基线随设计变更显式更新。
- **必跑主线**（对齐 `02-operation-flow`）：
  - 登录 → 环境徽标展示（`production` 视图**不出现** `Sync to Production`，硬验证 `00` §9 红线）。
  - 游戏 → 市场 → 渠道实例 → 登录/IAP/插件配置 → 商品 → 收银台 → 支付路由 → 快照 → 同步预览/执行。
  - 同步抽屉：按 section 勾选、密文不可见明文、基线不一致引导重新预览、重复提交被拦。
  - 审计查询页、Dashboard 只读聚合卡片跳转。

---

## 6. 统一回归入口（scripts/regression）

一条命令串起全链路（脚本职责描述，实现期落地）：

```text
scripts/regression/run.sh
  1. 启动依赖：拉起 PostgreSQL（容器/本地），等待健康
  2. 迁移：golang-migrate up（services/admin-api/migrations）
  3. seed：灌入 tests/fixtures（common + sandbox + production 基线）
  4. 后端全场景：
       - go test ./...（L1/L2/L3 就近用例）
       - scenario harness 跑 tests/backend/scenarios/*.yaml（接口场景矩阵）
       - 输出 junit + coverage 到 tests/reports/backend/
  5. 前端：
       - vitest run（L4，输出到 tests/reports/frontend-unit/）
       - 启动前后端 → Playwright（L5）跑 tests/frontend/e2e，采集截图/trace/HTML
       - 输出到 tests/reports/playwright-html/、tests/reports/playwright-results.json、tests/frontend/screenshots/
  6. 汇总：聚合各产物，生成 tests/reports/summary.{md,json}
       （通过/失败计数、按模块分组、失败用例与维度、截图/trace 链接）
  7. 退出码：任一层失败则非 0
```

- `scripts/regression/backend.sh` / `frontend.sh`：分别只跑后端 / 前端，便于按需。
- 按模块回归：`run.sh --module payment` 仅跑该模块的就近用例 + `scenarios/payment.yaml` + `e2e/payment.spec.ts`。

### 6.1 报告产物（tests/reports）

| 产物 | 内容 | 来源 |
| --- | --- | --- |
| `summary.md` / `summary.json` | 总览：按模块的通过/失败、覆盖维度、失败明细 | 回归脚本聚合 |
| `backend/junit.xml` · `coverage.out` | 后端用例结果与覆盖率 | go test / harness |
| `frontend-unit/` | vitest 结果 | vitest |
| `playwright-html/index.html` | e2e HTML report | Playwright |
| `playwright-results.json` | e2e 机器可读结果（供 summary 引用） | Playwright |
| `playwright-artifacts/**/trace.zip` | 失败用例 trace 与 artifacts | Playwright |
| `../frontend/screenshots/` | 真实页面截图 | Playwright |

---

## 7. fixtures 约定

- **按 schema 维度组织**：`common/`（灌入 `platform` schema，全环境共享，含 `currency_specs`、`channels`+`region`、各模板四件套）、`sandbox/`（灌入 `sandbox` schema，同步源样本）、`production/`（灌入 `production` schema，同步目标基线，用于 diff/baseline/nonce 测试）。
- **幂等可重复**：seed 用 `ON CONFLICT DO NOTHING`，可重复灌入；每个 scenario 可声明所需 fixture 子集。
- **覆盖关键边界**：至少包含一份「含 secret/file 的模板配置实例」（验 S8 脱敏）、「跨 market（GLOBAL + 具体 market）的渠道实例」（验合并/覆盖）、「production 与 sandbox 存在 add/update/delete 三类差异的样本」（验 sync）。
- fixtures 与 §F 模块矩阵引用的 `fixture:` 名称对应。

---

## 8. 各模块对接（接口场景矩阵小节）

每个模块 README 增设/升级「接口场景矩阵」小节，模板如下（维度引用本文 §4，不重复定义）：

```markdown
## N. 接口场景矩阵（→ 见 `../../03-testing.md`）
| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| POST .../xxx | ✓ | ✓ | ✓ | ✓ | ✓(CONFLICT) | ✓ | ✓ | — | — | ✓ | … |
| GET .../xxx  | ✓ | ✓ | ✓ | 非法query | — | — | — | ✓(密文) | ✓ | — | … |
- 前端：列出走 Playwright e2e/截图的页面与状态；列出走 vitest 的组件。
- manifest：`tests/backend/scenarios/<id>.yaml`；e2e：`tests/frontend/e2e/<id>.spec.ts`。
```

> 关联维护（`CONVENTIONS §3.3`）：各模块 front-matter `impacts` 含 `testing`，本文 `depends_on` 列出各模块，双向一致。

---

## 9. 实现状态（harness 已落地，模块场景增量补充）

- **已落地**：`tests/` 目录树、`docker-compose.yml`（Postgres）、`scripts/regression/*`（`lib.sh`/`db.sh`/`backend.sh`/`frontend.sh`/`run.sh`/`summarize.sh`）、后端 scenario harness（`services/admin-api/internal/testkit/scenario`，进程内 httptest + manifest 加载/点路径断言/入口测试）、前端 Playwright（截图 / trace / HTML report / 视觉基线）、贯穿现有 scaffold 的 smoke 切片。
- **增量补充**：各模块 S1–S10 完整场景 YAML 与 `expect.db` 断言，随对应模块连库实现后按 `tests/backend/scenarios/README.md` 的 schema 追加；`tests/fixtures/{common,sandbox,production}` 分别按 `platform`/`sandbox`/`production` schema 灌入实体样本。
- **运行**：全量 `sh scripts/regression/run.sh`（需 docker + golang-migrate）；快路径 `WITH_DB=0 sh scripts/regression/run.sh`（仅进程内场景 + 前端，不依赖 docker/migrate）。前端浏览器二进制需在本机可运行的环境下安装（`pnpm exec playwright install chromium`）。
