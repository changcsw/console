# Integration Checklist (Canonical) · module 23 dashboard

> 归并 backend/frontend 两份 checklist 的关键集成点。裁决源：`spec.compact.md`。集成复测：见 `audit.log.md`（2026-07-01 集成阶段第 1 轮）。

## API / 路由
- [x] `GET /api/admin/dashboard/summary`，进入权限 `dashboard.read`（`00 §7.5` 新增只读码）。
  - Query：`range`(24h|7d|30d|90d，默认 7d，仅作用卡片 C)、`withTopItems`(默认 false)、`topN`(1..20，默认 5)。
  - 指标级叠加来源读权限裁剪：fxReview←`cashier.read`；configIssues←`channel.read`+`game.read`；channelInstanceIssues←`channel.read`；recentSyncJobs←`sync.preview`；pendingSnapshots←`snapshot.read`。缺来源权限项 `permitted=false`+计数0+`topItems:[]`，整体仍 200。
  - 错误码：401 UNAUTHENTICATED / 403 FORBIDDEN(缺 dashboard.read) / 400 VALIDATION_FAILED(range/topN 非法)。
- [x] 子查询钻取接口（pending-fx-runs/config-issues/recent-sync-jobs/pending-snapshots/channel-instance-issues）**本期未实现**（MVP 仅 `/summary`；前端仅调 `getSummary`）。
- [x] 前端路由 `/dashboard`（`router/routes.ts` meta `perm: dashboard.read`），组件 `views/dashboard/DashboardView.vue` → 委托 `index.vue`。

## 契约对账（前端 dashboard.ts ↔ 后端 dto/dashboard.go）
- [x] DashboardSummary 8 字段、5 个 Metric 结构、timeRange/link/byStatus/topItems、link.query 键名 —— **0 漂移**。
- [ ] **D-1（移交🟧·低危非阻断）**：`fxReview.topItems[].templateId` 前端 `number` vs 后端/来源 cashier/DB(VARCHAR) `string`。建议前端改 `string` 并同步 fixtures/spec；compact 示例 `7` 建议改字符串码消歧。前端未渲染该字段、topItems 默认关。

## 共享面（shared surfaces）
- [x] 后端 `services/admin-api/internal/transport/httpserver/admin_wiring.go`：新增 dashboard 路由装配（ready 与非 ready 分支各一），`NewQueryService(pool)`。
- [x] **全局权限引导（本轮重点）** `apps/admin-web/src/main.ts`：挂载前用持久会话 `auth.user` 同步 `permission.setFromUser(...)`，保证 `router.beforeEach` 权限就绪后判定 `meta.perm`；保留异步 `loadMe()`。影响全部 22 模块受保护路由的直连/刷新。**回归结论：无回归**（权限正常者不误跳403；无权限仍拦截；perm 语义不变）。
- [x] `apps/admin-web/src/router/routes.ts`：`/dashboard` 加 `perm: dashboard.read`（未改动其它路由）。

## 来源仓储 / 来源表（只读，不新增表/索引/仓储写方法）
- [x] 平台级：`platform.cashier_fx_sync_runs`(+`cashier_price_templates`)、`platform.sync_jobs`(按 target_env)、`platform.channels`(region 兼容判定)、`platform.account_auth_types`。
- [x] 当前 env schema：`game_account_auth_configs`/`game_channel_login_configs`/`game_channel_iap_configs`/`channel_package_iap_overrides`/`game_channel_plugin_configs`/`channel_package_plugin_overrides`(config_status='invalid')、`game_config_snapshots`(status='draft')、`game_channels`(hidden/incompatible)、`games`(gameName 批量回查防 N+1)、`channel_packages`。
- [x] 一致性快照：单只读事务 `RepeatableRead` + 统一 `generatedAt=NOW()`。

## 红线核验
- [x] 只读/零写/零审计：仅 GET；只读事务；RegisterRoutes 不挂审计写（AuditWriter 忽略）。scenario S7。
- [x] env 隔离：env 服务端决定、无 env 入参；平台级汇率待审 `envScoped=false`。scenario S6。
- [x] production 无可执行 Sync 入口：卡片纯跳转链接、无执行按钮（静态）。
- [x] 无密文泄漏：无 secret 字段；lastCheckMessage 取来源脱敏值。scenario S8。

## 已知问题
- [ ] D-1 templateId 类型漂移（移交🟧，低危非阻断）。
- [ ] P-1 **既有非本模块**：games/game-cashier 详情类 e2e 失败（`.detail-head__title` 不出/详情数据未装配），base 复跑同样失败，非 dashboard 回归 → 交 games 模块。
- [ ] 前端 dictionary store 无 dashboard 枚举字典，source/status 文案由视图层本地格式化（MVP 可接受）。

## 验证命令
- 后端：`cd services/admin-api && go test ./...`
- 前端组件：`cd apps/admin-web && npm test`（vitest）
- 前端构建：`cd apps/admin-web && npm run build`
- 前端 e2e：`cd apps/admin-web && npx playwright test dashboard.spec.ts`（单模块）；权限回归代表 `npx playwright test audit.spec.ts --workers=1`
- 连库/全栈（需 CI）：`SCENARIO_WITH_DB=1` 跑 `tests/backend/scenarios/dashboard.yaml`；`scripts/regression/run.sh`（需 docker+migrate）。

## 风险
- Dashboard 只读依赖多张上游表，任一上游 schema 漂移会导致 summary 运行期查询失败（需 PG CI 连库断言兜底）。
- config-issue topItems 依赖来源 `last_check_at` 排序与 join 语义；channelInstance 兼容性判定复用 `channel.IsCompatible`。
- e2e 并行跑多 spec 于单机易资源争用致 15s 超时；权限回归抽查建议 `--workers=1` 隔离。
