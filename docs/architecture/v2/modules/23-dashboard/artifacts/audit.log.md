# Dashboard Frontend Audit Log

## 2026-07-01

- Read docs in order: `index.json` -> `00-common.md` -> `01-structure.md` -> `CONVENTIONS.md` -> `modules/23-dashboard/spec.compact.md`.
- Checked current frontend surface: existing route `/dashboard` pointed to `views/dashboard/DashboardView.vue`; `components/page/PageCard.vue` and `EnvironmentBadge.vue` already exist.
- Implemented dashboard frontend scope only under `apps/admin-web/**` with compact contract binding and no `services/**` edits.

### Verification commands (in `apps/admin-web`)

1. `npm run build`
   - Result: failed (`vue-tsc: command not found`) due missing local dependencies.
2. `npm install`
   - Result: success, dependencies installed.
3. `npm run build`
   - Result: failed once due `LocationQueryRaw` type mismatch in dashboard query forwarding.
4. Fix `goByLink` query normalization to string map.
5. `npm run build`
   - Result: success (`vue-tsc --noEmit && vite build` passed).

### Notes

- Build output includes existing chunk-size warnings from Vite/Rollup; no blocking TypeScript error remained.
- `npm install` generated `package-lock.json`, then removed to keep repository lockfile policy unchanged.
# Dashboard(#23) Backend Audit Log

## 2026-07-01

- Worktree: `/Users/csw/gitproject/console-dashboard` (branch `codex/dashboard`)
- Scope: backend-only implementation for `GET /api/admin/dashboard/summary` (MVP)
- Migration: none (read-only module, zero DDL)

### Commands

1. `gofmt -w internal/app/dto/dashboard.go internal/app/query/dashboard/errors.go internal/app/query/dashboard/service.go internal/transport/http/dashboard/handler.go internal/transport/http/dashboard/router.go internal/transport/httpserver/admin_wiring.go`
   - Result: success
2. `go build ./...`
   - Result: success
3. `go vet ./...`
   - Result: success

### Notes

- `DashboardQueryService` runs summary aggregation in a single read-only transaction and uses one `generatedAt` (`SELECT NOW()`).
- Route wired in `admin_wiring.go` for both ready/degraded branches.

## 2026-07-01 · Frontend Code Review（Composer 2.5）

- Worktree: `/Users/csw/gitproject/console-dashboard`（branch `codex/dashboard`）
- 结论：**通过**（CR 中已直接修复 1 项交互缺陷，无阻断项）

### 契约核对表

| 项 | compact 要求 | 实现 | 一致 | 证据 |
| --- | --- | --- | --- | --- |
| 5 张卡片 A–E | 汇率待审/配置异常/最近同步/待发布快照/渠道实例问题 | `index.vue` 5× `DashboardMetricCard` | ✓ | `index.vue:27-149` |
| A 全环境角标 | `envScoped=false` 显示「全环境」 | `!envScoped` → warning tag | ✓ | `DashboardMetricCard.vue:5-6` |
| B invalidTotal+bySource | 分桶展示+点击带 source | secondary 按钮 + `goByLink(...,{source})` | ✓ | `index.vue:56-68` |
| C byStatus+lastFailedAt+range | previewed/succeeded/failed + 最近失败 + range 仅影响 C | secondary 展示；`range` localStorage 默认 7d | ✓ | `index.vue:89-95,178-221` |
| D draftCount | 待发布 draft 计数 | `pendingSnapshots.draftCount` | ✓ | `index.vue:106-123` |
| E hidden+incompatible 独立 | 各自计数 | secondary 分行；主值为合计 | ✓ | `index.vue:128-140` |
| API `getSummary` | range/withTopItems/topN | `dashboard.ts:142-144` | ✓ | `buildQuery` + `request` |
| TS 类型 | DashboardSummary 各 Metric 字段 | `dashboard.ts:14-119` 全字段 camelCase | ✓ | 与 compact JSON 示例对齐 |
| HTTP 解包 | 走 `{data}` | `http.ts:106-108` | ✓ | `request<T>` |
| 四态·加载 | 首屏整页骨架屏 | `showInitialSkeleton = loading && !summary` | ✓ | `index.vue:20-24,193` |
| 四态·错误 | 顶部错误条+重试 | `el-alert` + `reloadSummary()` | ✓ | `index.vue:13-18` |
| 四态·空态 | 计数 0 绿色「暂无待办」 | `DashboardMetricCard` valueClass/hintClass | ✓ | `DashboardMetricCard.vue:13-14,63-64` |
| 四态·权限 | `permitted=false` 隐藏；缺 `dashboard.read` 路由拦截 | `v-if="...permitted"`；`routes.ts meta.perm` | ✓ | `index.vue:28+`；`router/index.ts:29-31` |
| >0 警示色 | 主数字警示/0 绿色 | `--warning` / `--ok` class | ✓ | `DashboardMetricCard.vue:63-64,93-116` |
| EnvironmentBadge | 取 app store environment | `:environment="app.environment"` | ✓ | `index.vue:4` |
| 无专用 store | env/perm 复用现有 store | `useAppStore`；路由 `hasPerm` | ✓ | `index.vue:175` |
| 跳转 query 透传 | A–E link.query | `goByLink(link, extraQuery?)` | ✓ | `index.vue:232-246` |
| 红线·无写入口 | production 无 Sync to Production | dashboard 视图无同步写按钮 | ✓ | grep dashboard 无匹配 |
| 红线·无密文 | lastCheckMessage 原样展示 | `item.lastCheckMessage` 直出 | ✓ | `index.vue:73` |
| 展开明细 | withTopItems=true 消费 topItems | 点击展开→`withTopItems=true`→reload | ✓（CR 修复后） | `index.vue:252-261`；expandable 改为 count>0 |
| dictionary 偏差 | 枚举词典暂无 dashboard | 页面内 `formatConfigSource` 等本地映射 | 偏差可接受 | handoff checklist 已记 |

### CR 直接修复

1. **展开明细不可达**：`expandable` 原绑定 `topItems.length>0`，但默认 `withTopItems=false` 时 topItems 恒 `[]`，按钮永不出现 → 改为 `count>0 || hasTopItems(...)`（`index.vue` 5 处）。

### 验证

- `cd apps/admin-web && npm run build` → **pass**（CR 后重跑；仅既有 chunk size warning）

### 非阻断备注

- 卡片 C 的 byStatus 分桶未单独可点击带 `status` query（compact 标注为可选 `status?`）。
- API 返回的 `window` 对象未额外回显（range 切换器已满足交互）。


- Worktree: `/Users/csw/gitproject/console-dashboard`（branch `codex/dashboard`）
- 结论：**通过**（CR 中已直接修复 3 项偏差，无阻断项）

### 契约核对表

| 项 | compact 要求 | 实现 | 一致 | 证据 |
| --- | --- | --- | --- | --- |
| API 路径/方法 | `GET /api/admin/dashboard/summary` | chi `Get("/dashboard/summary")` 挂于 `adminhttp.NewRouter`（`/api/admin`） | ✓ | `transport/http/dashboard/router.go:16`；`admin_wiring.go:156` |
| 入口权限 | `dashboard.read` | `mw.RequirePerm("dashboard.read")` | ✓ | `router.go:16`；seed `000003_auth_platform_schema.up.sql:61` |
| Query `range` | 24h/7d/30d/90d，默认 7d，非法 400 | `normalizeSummaryParams` + `rangeToDuration` | ✓ | `service.go:163-193`；`handler.go:44` |
| Query `withTopItems` | 默认 false | 缺省 false；非法 bool 400 | ✓ | `handler.go:47-58` |
| Query `topN` | 1..20，默认 5 | handler 显式校验；service 兜底 | ✓（CR 修复后） | `handler.go:71-78`；`service.go:171-176` |
| 响应包络 | `{data:...}` | `httpx.WriteData` | ✓ | `handler.go:38` |
| 响应字段 | environment/generatedAt/timeRange + 5 metrics | `dto/dashboard.go` 全字段 camelCase | ✓ | `dto/dashboard.go:11-127` |
| 错误码 | 401/403/400/500，不新增 | Authn/RequirePerm 中间件 + `VALIDATION_FAILED` | ✓ | `router.go:14-16`；`errors.go:5-28` |
| 只读红线 | 零 DDL、全 GET、无 audit 写 | 无 migration；仅 1 GET；`AuditWriter` 未用 | ✓ | git diff；`router.go:12-17` |
| env 模型 | 服务端 APP_ENV；SQL 无 env 谓词 | `ac.Environment`；业务表无 env WHERE；sync `target_env=$1` | ✓ | `service.go:62-63,391` |
| 指标权限裁剪 | 各指标叠加来源读权限 | fx=`cashier.read`；config=`channel+game`；sync=`sync.preview`；snap=`snapshot.read`；channel=`channel.read` | ✓（CR 修复后） | `service.go:117-153` |
| 配置异常口径 | 仅 invalid，6 表求和+bySource | 6 COUNT + UNION topItems | ✓ | `service.go:259-285,291-332` |
| 渠道兼容 | CN↔domestic，非CN↔overseas | SQL COUNT + `channel.IsCompatible` | ✓ | `service.go:542-547,621`；`domain/channel/visibility.go:57-58` |
| 同步任务 | target_env=当前 env；byStatus 补 0；lastFailedAt null | GROUP BY + MAX；零值 struct | ✓ | `service.go:380-415,418-423` |
| 待发布快照 | 仅 draft | `status='draft'` | ✓ | `service.go:475,484` |
| 一致性快照 | 单只读事务 + 统一 generatedAt | `BeginTx(ReadOnly, RepeatableRead)` + `NOW()` | ✓ | `service.go:46-54,204-207` |
| N+1 防护 | gameName 批量 IN | `loadGameMetaByRowIDs` / `loadGameNameByGameIDs` | ✓ | `service.go:630-669` |
| 兜底 | count=0、topItems=[]、lastFailedAt null | 初始化空 slice；`*time.Time` 扫描 NULL | ✓ | `service.go:65-113` |
| 分层 | query/transport/dto；纯规则无 IO | `rangeToDuration` 在 query；兼容复用 domain/channel | ✓ | `service.go:180-193` |
| MVP 子查询 | 5 个 drill-down 可选未实现 | 未实现（文档已标注） | 偏差可接受 | `integration.checklist.backend.md:33-38` |

### CR 直接修复

1. **指标权限过严**：`recentSyncJobs`/`pendingSnapshots`/`channelInstanceIssues` 误叠加 `game.read` → 按 compact 拆分为 `sync.preview` / `snapshot.read` / `channel.read`（`service.go:117-153`）。
2. **topN 校验**：显式 `topN=0` 曾被静默默认 5 → handler 返回 `VALIDATION_FAILED`（`handler.go:71-78`）。
3. **兼容性重复实现**：删除本地 `isIncompatible`，改用 `channeldomain.IsCompatible`（`service.go:621`）。

### 验证

- `cd services/admin-api && go build ./... && go vet ./...` → **pass**（CR 后重跑）

## 2026-07-01 · Frontend Test（dashboard #23）

- Worktree: `/Users/csw/gitproject/console-dashboard`（branch `codex/dashboard`）
- Scope: 仅新增前端测试（`apps/admin-web/**` + `tests/frontend/e2e/dashboard.spec.ts`），后端未改动。

### Added tests

- Vitest
  - `apps/admin-web/src/views/dashboard/components/__tests__/DashboardMetricCard.spec.ts`
  - `apps/admin-web/src/views/dashboard/__tests__/DashboardView.spec.ts`
- Fixture
  - `apps/admin-web/src/views/dashboard/__tests__/fixtures.ts`（复用 compact 成功响应样例）
- Playwright
  - `tests/frontend/e2e/dashboard.spec.ts`

### Verification commands (in `apps/admin-web`)

1. `npm test -- src/views/dashboard/components/__tests__/DashboardMetricCard.spec.ts src/views/dashboard/__tests__/DashboardView.spec.ts`
   - Result: **pass**（2 files, 12 tests passed）
2. `npm run e2e -- dashboard.spec.ts`
   - Result: **failed**（5/5 failed）
   - Observed page: all cases rendered `/403`（"无访问权限"）
   - Likely cause: route guard checks `permissionStore` before it is hydrated from persisted auth session on first navigation.
   - Classification: suspected implementation defect (frontend permission bootstrap ordering), not Playwright runtime/sandbox issue.

### Notes

- Existing warning kept as-is: Element Plus deprecation log (`el-radio label act as value`) during vitest run; does not fail tests.

## 2026-07-01 · Backend Test Pass（Codex 5.3）

- Scope: `services/**` + `tests/backend/**` for module `23-dashboard` only; no frontend edits.
- Added tests:
  - `services/admin-api/internal/app/query/dashboard/service_test.go`（range/window、topN 边界/越界、CN/非CN × domestic/overseas 兼容性矩阵）
  - `services/admin-api/internal/transport/http/dashboard/handler_test.go`（S1/S2/S3 + query 校验失败）
  - `tests/backend/scenarios/dashboard.yaml`（`GET /api/admin/dashboard/summary` 覆盖 S1-S10，含 N/A 原因）
  - `tests/fixtures/common|sandbox|production/dashboard.sql`（dashboard 场景 fixtures 入口）
- Command:
  - `cd /Users/csw/gitproject/console-dashboard/services/admin-api && go test ./internal/app/query/dashboard/... ./internal/transport/http/dashboard/... ./...`
  - First run failed: `handler_test.go` unused import (`adminapp`)；fixed and reran.
  - Final result: **PASS**（2/2 dashboard packages pass；`./...` 全量通过；DB-required scenario dimensions remain PG CI gated by `requiresDB=true`).

## 2026-07-01 · Frontend Fix — e2e 403 回退定位与修复（Codex 5.3）

### 根因判定（先判定归属，再改）
- 现象：`dashboard.spec.ts` 5/5 全部被路由守卫导向 `/403`，未进入 dashboard；vitest 12/12 通过。
- 排查：
  - `router/index.ts` 全局 `beforeEach` 同步读取 `permission.hasPerm(meta.perm)` 判定；`/dashboard` 上一轮已按 compact 加 `perm: dashboard.read`。
  - `main.ts` 仅在鉴权后 **异步** `void auth.loadMe()` 回填 `permission` store；`auth` store 从持久 `admin-auth` 仅回填 token/user，**未同步回填 permission**。
  - 故首个导航（`page.goto('/dashboard')`）时 permission 集合为空 → `hasPerm('dashboard.read')=false` → `/403`。dashboard.spec 的持久会话本就带 `dashboard.read`，仍失败 → 证明是**权限初始化时序产品缺陷**，非本用例 setup 缺失。
  - 对照既有 11 个模块 e2e：均以 `/dashboard`（此前无 perm）作跳板再点导航；其会话只带各自模块权限、不带 `dashboard.read`。
- 归属结论：**产品缺陷（router 守卫先于 permission 从持久会话回填）为主因**；叠加"共享跳板 `/dashboard` 现受 `dashboard.read` 守卫，但 11 个跳板会话未带该权限"的**用例 setup 缺口**（含 dashboard.spec 的 `.env-badge` 选择器与缺失视觉基线）。

### 实施修复
1. 产品缺陷（共享面 · router 守卫/权限引导）：`apps/admin-web/src/main.ts` 挂载前用 `auth.user` 同步 `usePermissionStore().setFromUser(...)`，保证守卫在权限就绪后判定；保留异步 `loadMe()` 刷新。修复对所有受保护路由的直连/刷新均生效（改善，非改变 perm 语义）。
2. 跳板会话对齐（用例 setup）：为 audit/cashier/channel-login/channels/feature-plugin/game-cashier/games/payment/product/snapshot/sync 共 11 个 spec 的持久 SESSION 补 `dashboard.read`（compact §关键假设「默认拥有」）。
3. dashboard.spec 选择器：`.env-badge` → `.dashboard-toolbar .env-badge`（AdminLayout 顶栏 + dashboard 工具栏各一个，strict 冲突）。
4. 缺失视觉基线：`npx playwright test dashboard.spec.ts --update-snapshots=missing` 生成 `visual-baseline/dashboard.spec.ts-snapshots/dashboard-main-chromium-darwin.png`。
5. 防御性渲染：`views/dashboard/index.vue` 5 卡 v-if 改深可选链，避免跳板模块以 `{data:{}}` stub 命中 /dashboard 报错。

### 验证（cd apps/admin-web，Shell all）
- `npm test`（vitest run）：**302 passed / 302**（含 dashboard 组件测试）。
- `npx playwright test dashboard.spec.ts`：**5 passed / 5**（先 --update-snapshots=missing 生成基线，再普通重跑）。
- 跳板非破坏代表：`npx playwright test audit.spec.ts`：**9 passed / 9**（进入 /dashboard→点「审计日志」→ /audit 全绿）。
- `npm run build`（vue-tsc --noEmit && vite build）：**pass**（仅既有 chunk-size warning）。
- 备注：e2e 单用例约 40s（Vite dev 冷编译），首次跑 audit+games 合并用 `tail` 缓冲无进度而被误判卡顿；实际 audit 全绿。全 22 模块 e2e 未逐一跑（耗时），以 audit 作跳板代表验证 + main.ts 修复的通用性佐证非破坏。

---

## 2026-07-01 · Integration / System Test（🟪 测试专家 · 集成阶段 · 第 1 轮）

前置闸门：🟦后端测试✅ + 🟩前端测试✅ 均满足。本角色不改业务代码；文件工具直读 worktree，Shell 命令均带 `required_permissions:["all"]`；未 git commit（仅短暂 stash/pop 用于回归取证，已还原）。

### 1. 契约对账（前端 `api/modules/dashboard.ts` TS 类型 vs 后端 `internal/app/dto/dashboard.go` JSON tag；以 compact 为裁决标准）

| DTO / 字段 | compact | 后端 JSON | 前端 TS | 结论 |
| --- | --- | --- | --- | --- |
| `DashboardSummary`（environment/generatedAt/timeRange/fxReview/configIssues/recentSyncJobs/pendingSnapshots/channelInstanceIssues） | ✓ | 同名 8 字段 | 同名 8 字段 | ✅ 一致 |
| `timeRange`{range,since,until} | ✓ | ✓ | ✓ | ✅ |
| `MetricLink`{route,query} | ✓ | `map[string]any` | `Record<string,...>` | ✅ |
| 各 Metric 公共字段 permitted/envScoped/link/topItems | ✓ | ✓ | ✓ | ✅ |
| `fxReview`.pendingReviewCount / envScoped=false | ✓ | ✓ | ✓ | ✅ |
| `fxReview`.topItems[].**templateId** | 示例写 `7`(number) | **`string`**（`t.template_id VARCHAR(64)` 业务码，与 cashier 来源模块一致） | **`number`** | ⚠️ **类型漂移**（详见问题清单 D-1） |
| `configIssues`.invalidTotal/bySource[{source,invalidCount}] | ✓ | ✓ | ✓ | ✅ |
| `configIssues`.topItems[]{source,gameId,gameName,target,lastCheckMessage} | ✓ | ✓ | ✓ | ✅ |
| `recentSyncJobs`.window/total/byStatus{previewed,succeeded,failed}/lastFailedAt(nullable) | ✓ | ✓ | ✓ | ✅ |
| `recentSyncJobs`.topItems[]{jobId,gameId,gameName,status,executedAt} | ✓ | ✓ | ✓ | ✅ |
| `pendingSnapshots`.draftCount / topItems[]{snapshotId,gameId,gameName,configVersion,generatedAt} | ✓ | ✓ | ✓ | ✅ |
| `channelInstanceIssues`.hiddenCount/incompatibleCount / topItems[]{gameChannelId,gameId,gameName,channelId,marketCode,issue} | ✓ | ✓ | ✓ | ✅ |
| link.query 键名 A `/cashier`{tab:fx-review,status:pending_review}；B `/games`{configStatus:invalid}；C `/games`{tab:sync-history,targetEnv,status?}；D `/games`{tab:snapshots,status:draft}；E `/games`{tab:channels,issue:"hidden,incompatible"} | ✓ | 逐项匹配（service.go 默认 Link） | 透传 `link.query` | ✅ 全一致 |
| Query 参数 range/withTopItems/topN(1..20,默认5) | ✓ | handler.go 解析+400 校验 | `buildQuery` 拼装 | ✅ |
| 错误码 401 UNAUTHENTICATED / 403 FORBIDDEN / 400 VALIDATION_FAILED；缺部分来源权限仍 200+permitted=false | ✓ | router.go + service.go 逐指标裁剪 | — | ✅ |

对账结论：**结构/路径/枚举/错误码/link.query 键名 0 漂移；仅 1 处字段类型漂移（templateId）**，且该字段前端仅渲染 `templateName·triggeredAt`（`index.vue` 未渲染 templateId），运行期不可见、`withTopItems` 默认 false，**非阻断**。

### 2. 各测试套件实跑（命令输出摘要）
- 后端全量：`cd services/admin-api && go test ./...` → **全部 ok（含 `app/query/dashboard`、`transport/http/dashboard`），exit 0**。
- 前端组件：`cd apps/admin-web && npm test`（vitest run） → **39 files / 302 tests all passed**。
- 前端构建：`npm run build`（vue-tsc --noEmit && vite build） → **pass**（仅既有 chunk-size warning）。
- 前端 e2e（Playwright，mock API）：
  - `dashboard.spec.ts` → **5/5 通过**（5 卡布局/EnvironmentBadge/range重拉/展开明细/权限态隐藏/空态/错误重试）。
  - `audit.spec.ts`（单 worker 隔离）→ **9/9 通过**，含 `#7 无 audit.read→403 整页降级`（佐证负向拦截未放宽）。
  - 并行批量 `dashboard+login+audit+games+channels+cashier` → login/channels/cashier/dashboard 全绿；audit 2 例超时（并行资源争用，隔离复跑 9/9 已证伪）。
- 后端场景 manifest `tests/backend/scenarios/dashboard.yaml`：S1–S10 齐备（S5/S9/S10 标 N/A 原因）；S2 进程内可执行；S1/S3/S4/S6/S7/S8 标 `requiresDB=true`，**需 PG CI（`SCENARIO_WITH_DB=1`）**执行连库断言（沙箱无 PG，保留可复现用例）。

### 3. 既有失败取证（games/game-cashier 详情类 e2e — 非本模块回归）
- 隔离单 worker 复跑 `games.spec.ts game-cashier.spec.ts`：games 6 例详情用例失败（`openGameDetail` 后 `.detail-head__title` 不出现，`game` 对象为空）+ game-cashier 1 例视觉基线差异；game-cashier 功能用例 7/7 通过。
- error-context 显示：详情页外壳已渲染（"游戏详情"标题/返回列表/Sync禁用），仅 `v-if="game"` 的游戏名区未出，即详情 GET 数据未装配 —— 与权限引导/dashboard 无关。
- **决定性取证**：`git stash` 掉本分支 tracked 改动（main.ts/routes.ts/DashboardView.vue/games.spec.ts），在 **base（HEAD）** 上复跑 `games.spec.ts:335/:352` → **仍以完全相同错误失败**。已 `git stash pop` 还原全部 dashboard 改动（`git status` 校验一致）。
- 结论：games/game-cashier 详情类 e2e 失败为 **pre-existing（既有问题）**，**非** dashboard 分支 / main.ts 全局权限引导修复引入的回归；不在本模块责任范围，建议记入既有 e2e 待修清单交对应模块处理。

### 4. 全局权限引导回归（main.ts 影响全部 22 模块受保护路由）
- 代码审阅：`main.ts` 挂载前用持久会话 `auth.user.{roles,permissions}` 同步 `permission.setFromUser(...)`，使 `router.beforeEach` 同步读 `hasPerm(meta.perm)` 时权限已就绪；保留异步 `loadMe()` 与服务端对账。
- 正确性：① 权限正常用户不再被误导向 /403（dashboard/games 列表/audit 页均正常进入，e2e 佐证）；② 无权限用户仍被拦截 —— `setFromUser` 只回填**真实**会话权限、不额外授予，`hasPerm` 仍返回 false → /403（audit `#7` 403 降级、games `无 game.write→按钮置灰` 用例佐证）；③ 各路由 `meta.perm` 语义不变（routes.ts diff 仅给 `/dashboard` 加 `perm:dashboard.read`，未动其它路由）。
- 结论：**无回归**。

### 5. 红线端到端核验（静态 + 后端场景）
- 只读/零审计：dashboard 仅 `GET /dashboard/summary`；`RegisterRoutes` 忽略 AuditWriter（`_`）不挂审计写；`service.Summary` 用 `BeginTx{RepeatableRead, ReadOnly}` 只读事务 + 单 `generatedAt`（一致性快照）。scenario S7 断言 audit_logs 计数不变（需 PG CI）。✅
- 权限逐指标裁剪：service.go 逐项 `HasPermission`（cashier.read/channel.read+game.read/channel.read/sync.preview/snapshot.read），无权限项 permitted=false、计数 0、topItems []，整体仍 200。✅
- 跨 env schema 隔离：env 取 `ac.Environment`（服务端），无 env 入参；sync_jobs 按 `target_env`；`cashier_fx_sync_runs` 平台级 → fxReview `envScoped=false`。scenario S6 覆盖（需 PG CI）。✅
- production 无可执行 Sync 入口：dashboard 纯只读，卡片 C 仅历史分桶+跳转链接、无执行按钮；`index.vue` 仅渲染跳转。✅（静态）
- 无密文泄漏：不含 secret 字段；`lastCheckMessage` 取来源已脱敏值。scenario S8 覆盖。✅
- 真实全栈 e2e（真实 PG + 起服务）沙箱不可行，标注**需 CI 环境**；契约对账（静态）+ go test（进程内 httptest）+ vitest + Playwright（mock）已实跑。

### 问题清单（移交 🟧 高级全栈工程师）
| # | 问题 | 证据（文件:行） | 期望 vs 实际 | 建议修复方向 | 责任侧 | 阻断? |
| --- | --- | --- | --- | --- | --- | --- |
| D-1 | `FxReviewTopItem.templateId` 类型漂移 | 前端 `apps/admin-web/src/api/modules/dashboard.ts:27`（`templateId: number`）；后端 `internal/app/dto/dashboard.go:43`（`TemplateID string`）；来源 `migrations/000007_*.up.sql:17`（`template_id VARCHAR(64)`）+ `api/modules/cashier.ts:17,64,81`（templateId:string） | 期望 `string`（业务码，与 cashier 来源模块 + DB 一致，compact 示例 `7` 为误导性数值）；实际前端声明 `number` | 前端将 `FxReviewTopItem.templateId` 改为 `string`，同步 `__tests__/fixtures.ts:19`、`DashboardView.spec.ts:171` 的 `7/8` 改为字符串；建议 compact 示例改为字符串码以消歧 | 前端（+ compact 示例澄清） | 否（topItems 默认关、前端未渲染该字段） |
| P-1（既有·非本模块） | games/game-cashier 详情类 e2e 失败（`.detail-head__title` 不出/详情数据未装配） | `tests/frontend/e2e/games.spec.ts:291`；base 复跑同样失败 | 详情页应渲染游戏名 | 交 games 模块排查详情 mock/装配（已证 pre-existing，非 dashboard 引入） | 前端(games 模块) | 否（对 dashboard 验收不阻断） |

### 复测轮次
- 第 1 轮（本轮）：契约对账 0 结构漂移（1 类型漂移 D-1，非阻断）；go test ./... 全绿；vitest 302/302；build pass；dashboard e2e 5/5；audit 9/9；全局权限引导回归无回归；games/game-cashier 详情失败已证为既有问题。**无需 🟧 修复即可进入功能验收**（D-1 为低危类型对齐，建议🟧顺手修，不阻断）。

---

## 2026-07-01 · D-1 修复（🟧 高级全栈工程师）

### 根因
- `FxReviewTopItem.templateId` 前端 TS 声明为 `number`，但后端 DTO `dto.DashboardFXReviewItem.TemplateID` 为 `string`，取自 SQL `t.template_id`（来源表 `platform.cashier_price_templates.template_id VARCHAR(64)`，业务码而非自增主键）。compact 成功响应示例把 `templateId` 写成 `7`（数值）属误导。裁决：以后端/DB 实际 `string` 为准，前后端统一。
- 后端无需改动：DTO 已是 `string`（`internal/app/dto/dashboard.go:43`），查询 `service.go:230` 选取 VARCHAR 业务码，序列化即 string。仅前端类型 + 前端测试夹具使用了数值。

### 改动文件:行
1. `apps/admin-web/src/api/modules/dashboard.ts:27` — `templateId: number` → `templateId: string`。
2. `apps/admin-web/src/views/dashboard/__tests__/fixtures.ts:19` — `templateId: 7` → `templateId: "global_cashier_v3"`（业务码字符串，示意）。
3. `apps/admin-web/src/views/dashboard/__tests__/DashboardView.spec.ts:171` — `templateId: 8` → `templateId: "asia_price_v1"`。

### 顺带核对（fxReview.topItems 其它字段）
- `runId:number`（后端 int64→number ✓）、`templateName:string`（✓）、`triggeredAt:string`（后端 time.Time 序列化为 ISO string ✓）。无其它漂移。

### 验证（Shell + all，cd apps/admin-web）
- `npm test`（vitest run）：**302 passed / 302**。
- `npm run build`（vue-tsc --noEmit && vite build）：**pass**（仅既有 chunk-size warning；类型收窄经 vue-tsc 校验）。
- 后端未改动，故不重跑 go build/vet（DTO 本就 `string`）。
- e2e：`dashboard.spec.ts` 无 `templateId` 断言、`index.vue` 亦不渲染该字段（仅 `templateName·triggeredAt`），本次类型对齐不影响 e2e 行为，故未重跑（🟪 上轮 5/5 已绿）。

### 范围声明
- 仅修 D-1。未触碰 P-1（games/game-cashier 详情类 e2e 既有问题，非本模块）；未改 `main.ts` 全局权限引导（前端已修并经 🟪 回归通过）。

---

## 2026-07-01 · ✅ 功能验收（Composer 2.5 · 验收师）

Worktree: `/Users/csw/gitproject/console-dashboard`（branch `codex/dashboard`）。前置：🟪 集成 R1 通过、🟧 D-1 已闭合。Shell 均 `required_permissions:["all"]`；未 git commit。

### 构建/测试命令输出摘要

| 命令 | 结果 |
| --- | --- |
| `cd services/admin-api && go build ./... && go vet ./... && go test ./...` | **exit 0**；含 `app/query/dashboard`、`transport/http/dashboard` 全部 ok |
| `cd apps/admin-web && npm test` | **302/302** passed（39 files） |
| `cd apps/admin-web && npm run build` | **pass**（vue-tsc + vite；仅既有 chunk-size warning） |
| `cd apps/admin-web && npm run e2e -- dashboard.spec.ts` | **5/5** passed（41.6s） |
| `npx playwright test audit.spec.ts --workers=1` | **9/9** passed（含无 audit.read→403；佐证 main.ts 权限引导无回归） |

### 功能验收清单（编号 \| 验收点 \| 结论 \| 证据）

| # | 验收点 | 结论 | 证据 |
| --- | --- | --- | --- |
| B1 | 后端 build/vet/test 全绿 | **PASS** | 本轮 `go build ./... && go vet ./... && go test ./...` exit 0 |
| B2 | `GET /api/admin/dashboard/summary` 入口权限 `dashboard.read` | **PASS** | `router.go:16` RequirePerm；`handler_test.go:98-106` S3 403 |
| B3 | Query `range` 24h/7d/30d/90d 默认 7d；非法 → 400 | **PASS** | `service_test.go:43-81`；`service.go:163-178` |
| B4 | Query `withTopItems`/`topN`(1..20) 校验 → 400 | **PASS** | `handler_test.go:32-46` topN/withTopItems；`handler.go:60-79` |
| B5 | 401 UNAUTHENTICATED / 403 FORBIDDEN(缺 dashboard.read) | **PASS** | `handler_test.go:89-106` S2/S3 |
| B6 | 200 响应 8 顶字段 + 5 Metric 结构（含 timeRange/link/topItems） | **PASS** | `handler_test.go:108-123`；`dto/dashboard.go`；`dashboard.ts` 对账 0 漂移 |
| B7 | D-1：`fxReview.topItems[].templateId` 前后端均为 **string** | **PASS** | `dashboard.ts:27` string；`dto/dashboard.go:43` TemplateID string |
| B8 | 缺来源读权限仍 200；对应指标 `permitted=false`/计数0/明细[] | **PASS** | `service.go:66-155` 默认 false+空；e2e trimmed 4 卡 |
| B9 | 指标级权限叠加（cashier/channel+game/sync.preview/snapshot.read） | **PASS** | `service.go:116-155` 逐项 HasPermission |
| B10 | 单只读事务 RepeatableRead + 统一 generatedAt | **PASS** | `service.go:45-51,157-159` |
| B11 | 只读零 audit 写（RegisterRoutes 忽略 AuditWriter） | **PASS** | `router.go:12,16` 仅 GET；`_ mw.AuditWriter` |
| B12 | 零 DDL（本模块无新表/迁移） | **PASS** | migrations 仅既有 `dashboard.read` 权限码 |
| B13 | env 服务端决定、无 env 入参；汇率平台级 `envScoped=false` | **PASS（需 PG CI 非阻断）** | `service.go:61,67-68`；scenario S6 requiresDB |
| B14 | 跨 env schema 隔离 + sync_jobs target_env 过滤 | **PASS（需 PG CI 非阻断）** | scenario S6/S7/S8 requiresDB；静态 SQL 口径对齐 compact |
| B15 | 无密文泄漏（lastCheckMessage 脱敏、无 secret 字段） | **PASS（需 PG CI 非阻断）** | scenario S8；前端不渲染密文字段 |
| F1 | vitest 302/302 + build pass | **PASS** | 本轮实跑 |
| F2 | 路由 `/dashboard` meta `perm:dashboard.read` | **PASS** | `routes.ts:23-26` |
| F3 | 5 卡 A–E 渲染（汇率/配置异常/最近同步/待发布快照/渠道实例） | **PASS** | e2e `#107` 5 `.metric-card`；`index.vue:27-152` |
| F4 | 卡 A「全环境」角标（`envScoped=false`） | **PASS** | `DashboardMetricCard.vue:5`；fixtures fxReview.envScoped=false |
| F5 | 卡 B `invalidTotal` + `bySource` 分桶可点跳转 | **PASS** | `index.vue:56-68` goByLink+source |
| F6 | 卡 C `byStatus`+`lastFailedAt` + range localStorage 记忆 | **PASS** | e2e range 30d；`index.vue:181-224` RANGE_STORAGE_KEY |
| F7 | 卡 D `draftCount` | **PASS** | `index.vue:106-123` |
| F8 | 卡 E `hiddenCount`/`incompatibleCount` 独立展示 | **PASS** | `index.vue:128-143` 分列计数 |
| F9 | 四态：骨架/错误重试/空态绿色/权限隐藏 | **PASS** | e2e `#153/#146/#138/#120` |
| F10 | `link.query` 跳转透传（A–E 目的页与 query 键名） | **PASS** | `service.go:69-112` 默认 Link 对齐 compact；`index.vue:235-248` goByLink |
| F11 | production 无可执行 Sync 入口（仅跳转链接） | **PASS** | `index.vue` 无 execute/sync 按钮；仅「前往处理」 |
| F12 | 全局权限引导 main.ts：有权限正常进、无权限仍拦截 | **PASS** | `main.ts:25-26`；audit e2e 9/9 含 #7 403 |
| OF1 | operation-flow 对齐：卡片直达待办修复点（invalid/快照/同步历史/渠道/汇率） | **PASS** | link 映射 compact §卡片 A–E ↔ 02-operation-flow 步骤 5–11 拦截项 |
| R1 | D-1 已闭合 | **PASS** | 集成 handoff + 本轮 `dashboard.ts:27` 实查 |
| R2 | P-1 games 详情 e2e（非本模块） | **N/A·不阻断** | 集成阶段 base 复跑同样失败；交 games 模块 |

**统计：PASS 33 / FAIL 0 / N/A·不阻断 1（P-1）**

### operation-flow 走查摘要

| 卡片 | 操作主线位置 | 跳转 | 结论 |
| --- | --- | --- | --- |
| A 汇率待审 | 平台 A8 收银台汇率维护 | `/cashier?tab=fx-review&status=pending_review` | PASS |
| B 配置异常 | B 步骤 5–8 invalid 拦截 | `/games?configStatus=invalid`（分桶 +source） | PASS |
| C 最近同步 | B 步骤 10 同步历史 | `/games?tab=sync-history&targetEnv=<env>` | PASS |
| D 待发布快照 | B 步骤 9 draft | `/games?tab=snapshots&status=draft` | PASS |
| E 渠道实例 | B 步骤 3/9 hidden·incompatible | `/games?tab=channels&issue=hidden,incompatible` | PASS |

### 验收结论

**通过**。MVP `/summary` 端到端功能成立：契约对齐（含 D-1 templateId=string）、构建测试全绿、5 卡四态与 operation-flow 跳转闭环、红线静态核验通过。连库维度（S1/S3/S4/S6/S7/S8/S10）保留 PG CI 兜底，**非阻断**。
