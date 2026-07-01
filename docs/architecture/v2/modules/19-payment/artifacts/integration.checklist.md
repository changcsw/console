# #19 payment · Integration Checklist

> 由开发 / CR / 测试 / 集成角色共同维护。总 Agent 仅登记骨架，细节由各子角色补全。

## 模块路由 / API / 页面入口
- 后端 API（前缀 `/api/admin`，权限 `payment.read` / `payment.write`）：
  - GET `/pay-ways`、GET `/cashier/providers`、GET `/billing-subjects`、GET `/cashier/merchant-accounts`
  - GET `/cashier/providers/{providerId}/template`（🟧 I1 新增，payment.read，返回 enabled 最新模板四件套；无模板→404）
  - POST `/billing-subjects`、POST `/cashier/merchant-accounts`
  - GET/PUT `/games/{gameId}/payment-routes`
  - 列表 GET 统一返回分页包络 `{items,page,pageSize,total}`（🟧 I2，对齐 cashier）
- 前端页面：`/pay-ways`、`/providers`、`/billing-subjects`、`/merchant-accounts`（列表+抽屉）
- 游戏详情 Tab：`/games/:gameId` 新增「支付路由」Tab

## 引用模块 / 外部依赖 / 共享 surface
- 依赖：channel(#12) / product(#16, 仅确认 IAP 隔离) / cashier-template(#17) / game-cashier(#18) / game(#11) / common
- AuditSink：#22 audit（统一审计注入）
- lane：payment-surface（独立串行）

## 需要统一接入的共享文件（本模块本地改动 → 集成阶段统一整合）
- [x] `services/admin-api/internal/transport/httpserver/admin_wiring.go`
      已注册 payment service（`paymentapp.NewService(postgres.NewPaymentStore(pool), cipher, auditSink, time.Now)`）+ `paymenthttp.RegisterRoutes`；auditSvc/auditSink 注入；ready + degraded 两态路由形状均挂载（ready=false → Authn→503）
- [x] `apps/admin-web/src/router/routes.ts`（payment 路由分组，已接入 /payment 子路由）
- [x] `apps/admin-web/src/views/games/detail/GameDetailView.vue`（已新增 PaymentRoutesTab，Product/IAP Tab 保持不变）

## Migration
- 从 `000014` 起追加，不改历史；平台 5 表 + provider 模板 + payment_routes + v2 唯一索引；幂等风格 `IF NOT EXISTS`/`DROP CONSTRAINT IF EXISTS`。

## 已知问题 / 待完善点
- 后端已完成：domain（Route/MatchInput/RouteTarget + MatchedRoutes/PickBestRoute/marketMatches/ValidateRouteSet）、app/payment、infra 仓储、transport/http/payment、000014 迁移、admin_wiring 接入。
- **后端 CR（2026-07-01）通过**：compact 数据模型/算法/API/红线逐项一致；CR 已修 GET routes 响应 DTO（camelCase json + 嵌套 `selector`）与 groups 按 payWay sort 序。
- `go build ./...`、`go vet ./...`、`go test ./internal/domain/payment/...` 均通过；迁移 000014 up/down 成对幂等（沙箱无 DB，未做 live up/down 执行）。
- provider 模板拉取端点：🟧 已补 `GET /api/admin/cashier/providers/{providerId}/template`（payment.read，返回 enabled 最新模板四件套；无模板→404）；merchant-account 创建时后端仍用最新模板做字段校验 + secret 加密。
- selector 三态：DB 中 market/country/currency 通配存 `'*'`，package/channel 通配存 NULL（唯一索引用 `COALESCE(-1)`）；domain 侧 ""/"*"/NULL 三态等价。
- GET payment-routes 组内 routes 按 priority 升序（管理台优先级链路）；运行时 ResolveRoute 走 domain PickBestRoute 全链——集成测试需分别验证。
- GET routes 已返 `hasDisabledReference`/`*Enabled`（🟧 I4 enrich），前端「引用对象已禁用」标红据此触发。
- 列表端点（pay-ways 等）已返回 `{items,page,pageSize,total}`（🟧 I2 补齐，对齐 cashier）；前端分页器 total 正常。

## 集成步骤 / 验证命令 / 风险说明
- 后端：`go build ./... && go test ./...`
- 后端（payment 快速验证）：`go test ./internal/domain/payment/... ./internal/app/payment/... ./internal/transport/http/payment/...`
- 前端：`pnpm -C apps/admin-web build` / `vitest` / Playwright
- 前端测试实绩（2026-07-01）：
  - `pnpm -C apps/admin-web test` ✅（34 files / 238 tests）
  - `pnpm -C apps/admin-web e2e payment.spec.ts --workers=1` ✅（3 passed）
  - 覆盖：payment 列表页冒烟、游戏详情支付路由 Tab 冒烟、与 Product/IAP Tab 共存无回归；组件层覆盖 ROUTE_CONFLICT 双类型高亮、provider→merchant 过滤、模板降级路径、billing-subjects 抽屉校验
- 场景矩阵：`tests/backend/scenarios/payment.yaml`，fixtures：`tests/fixtures/common/payment.sql`
- 回归入口：`scripts/regression/backend.sh --module payment`（或全量 `scripts/regression/run.sh`）
- fixtures 现状：已补 `tests/fixtures/common/payment.sql`（platform 基础数据 + payment RBAC）；`payment_routes` 的 sandbox/production 样本待 PG CI 场景编排。

## 下游契约
- `PaymentRouteService.ResolveRoute(ctx, gameID, MatchInput)` 供 #20 snapshot 调用；无候选 → NOT_FOUND。

- 前端联调（🟧 已闭环）：provider 模板拉取端点已由后端实现 `GET /api/admin/cashier/providers/{providerId}/template`；前端占位（`PROVIDER_TEMPLATE_ENDPOINT_UNCONFIRMED`/`providerTemplatePath`）已移除，`getProviderTemplate` 直连真实端点；provider 无模板时后端 404、抽屉降级为仅基础字段。
- 前端 CR 联调偏差（🟧 已闭环）：① 列表 GET 已返 `{items,page,pageSize,total}`，分页器正常；② `ROUTE_CONFLICT` details 已含 `leftIndex/rightIndex`(+可选 `leftRouteId/rightRouteId`)，行级高亮生效；③ GET routes 已回传 `hasDisabledReference`/`*Enabled`，「引用对象已禁用」标红生效。

## 🟪 集成/系统测试（2026-07-01 · R1）→ 详见 `audit.log.md`
- 全量回归（真实输出）：`go build ./...` ✅ / `go test ./...`(admin-api) ✅ / `pnpm -C apps/admin-web test` ✅(34f/238) / `e2e payment.spec.ts --workers=1` ✅(3) / scenario `payment` 连库维度按设计 SKIP(requiresDB=true)、S2 PASS。
- 契约对账：8 端点 方法/路径/权限/DTO(selector 嵌套 camelCase)/错误码 一致；三态映射正确；下游 `ResolveRoute(ctx,gameID,MatchInput)->RouteTarget`、无候选 `NOT_FOUND` 满足 #20。
- 红线核验通过：脱敏 masked / 权限 read·write / schema 隔离(业务表无前缀,静态) / 事务回滚(InTx) / IAP 隔离 / production 无可执行 Sync / v2 唯一索引。
- 集成判定：**通过，可进入 ✅ 功能验收**；无阻断性失败。

### 遗留问题清单（🟧 全栈修复状态 · 2026-07-01）
- **I1 provider 模板 GET 端点缺失** → ✅**已修复**：后端新增 `GET /api/admin/cashier/providers/{providerId}/template`（payment.read，复用 `PaymentRepo.GetLatestProviderTemplate`+`accountauth.Template` 四件套 camelCase；无模板 provider→404）；前端移除 `PROVIDER_TEMPLATE_ENDPOINT_UNCONFIRMED` 占位、`getProviderTemplate` 直连真实端点、`MerchantAccountsView` 抽屉正常渲染四件套并可录 secrets。测试：`handler_test.go:TestGetProviderTemplate` + `MerchantAccountsView.spec.ts`。
- **I2 列表缺分页元数据** → ✅**已修复**：现状对齐 cashier(返 total)→后端补齐。`Repository`/`Service` 四个 List 返回 total(repo 加 `COUNT(*)`)，handler `writeList` 输出 `{items,page,pageSize,total}`。测试：`handler_test.go:TestListPayWaysPaginationEnvelope`。
- **I3 ROUTE_CONFLICT 缺行定位** → ✅**已修复**：`RouteConflictError` 增 `LeftIndex/RightIndex`(+`LeftID/RightID`)，`ValidateRouteSet` 记录首现位置；`routeConflictErr` 映射 `details=[{kind,leftIndex,rightIndex,(leftRouteId?),(rightRouteId?)}]`，前端行级双色高亮（duplicate_priority/duplicate_selector）。测试：`handler_test.go:TestPutGameRoutesConflictDetailsCarryPositions` + `PaymentRoutesTab.spec.ts`。
- **I4 GET routes 缺禁用引用字段** → ✅**已修复**：`ListGameRoutes` SELECT 追加 `pw/p/ma/ch/cp` enabled（channel/package `COALESCE(...,TRUE)`），`GameRouteRecord`+`RouteItemDTO` enrich `hasDisabledReference`/`payWayEnabled/providerEnabled/merchantAccountEnabled/channelEnabled/packageEnabled`；前端「引用对象已禁用」行内标红据此触发。
- **I5 GET 组内排序口径** → ✅**文档已标注**（不改代码）：管理台组内按 `priority` ASC 展示（GET 返回顺序，前端不重排）；运行时 `ResolveRoute` 用 domain `PickBestRoute`(specificity) 序。两口径明确区分。
- **残留风险** | `tests/backend/scenarios/payment.yaml` requiresDB=true 用例 | 真实 PG 下 S6 schema 隔离 / S10 事务回滚 / DB 唯一索引兜底断言 | PG CI 执行连库场景（当前 memstore+httptest 等价覆盖核心行为） | 提示（非缺陷）

## 🟪 集成/系统测试（2026-07-01 · R2 — 验证 🟧 I1–I4 修复闭环）→ 详见 `audit.log.md`
- 全量回归（真实输出）：`go build ./... && go vet ./...` ✅ / `go test ./...`(admin-api) ✅ / `pnpm -C apps/admin-web test` ✅(34f/238) / `e2e payment.spec.ts --workers=1` ✅(3)；新增 transport 测试 `TestGetProviderTemplate`/`TestListPayWaysPaginationEnvelope`/`TestPutGameRoutesConflictDetailsCarryPositions` 均通过。
- **I1 闭环✅**：`GET /api/admin/cashier/providers/{providerId}/template`（payment.read，四件套 camelCase，无模板→404）；前端 `getProviderTemplate` 直连、占位常量/`providerTemplatePath` 已移除（grep 无残留），商户抽屉可渲染四件套并录密钥。
- **I2 闭环✅**：4 个 List 返 `{items,page,pageSize,total}`（repo COUNT(*)），前端分页器 total 正确。
- **I3 闭环✅**：domain `RouteConflictError` 携 `LeftIndex/RightIndex(+ID)`，details=`[{kind,leftIndex,rightIndex,leftRouteId?,rightRouteId?}]`，前端行级双色高亮定位。
- **I4 闭环✅**：GET routes 返 `hasDisabledReference`+5 `*Enabled`（repo COALESCE 通配=true），「引用对象已禁用」标红据此触发。
- **I5**：仅文档标注（管理台 priority 序 / 运行时 PickBestRoute 序），未改代码。
- 无回归：脱敏 masked / 权限 read·write / schema 隔离（业务表无前缀）/ 事务回滚 InTx / IAP 隔离 / production 无可执行 Sync / v2 唯一索引 均保持；下游 `ResolveRoute->RouteTarget/NOT_FOUND` 不变，满足 #20。
- 残留（非阻断，照旧）：连库维度（S6/S10/唯一索引）requiresDB=true 待 PG CI；`payment.yaml` 未补 模板/total/index 用例（handler_test 等价覆盖）。
- **R2 通过判定：I1–I4 全部闭环，无阻断/无回归，可进入 ✅ 功能验收。**

## ✅ 功能验收（2026-07-01 · Composer 2.5）→ 详见 `audit.log.md`
- 验收清单 36 项：**36 PASS / 0 FAIL**（数据模型·纯函数·8+1 API·密文/审计·前端 4 页+游戏 Tab·migration·operation-flow §B.8·#20 ResolveRoute）。
- 构建测试（真实输出）：`go build ./... && go test ./...` ✅ · `pnpm -C apps/admin-web build` ✅ · `pnpm test` 34f/238 ✅ · `e2e payment.spec.ts` 3/3 ✅ · `scripts/regression/backend.sh --module payment` ✅。
- 结论：**功能验收通过**；P3 遗留：PG CI requiresDB、000014 未 live up/down、全量 regression playwright 5187 端口冲突（环境，payment 专项 e2e 独立通过）。

### 🟧 全栈修复自检（2026-07-01）
- 后端：`go build ./...` ✅ / `go vet ./...` ✅ / `go test ./...`(services/admin-api) ✅（含新增 3 个 handler 用例；payment 连库场景 requiresDB SKIP）。
- 前端：`pnpm -C apps/admin-web test` ✅（34 files / 238 tests）/ `e2e payment.spec.ts --workers=1` ✅（3 passed）。
- 判定：I1–I4 闭环，前后端 DTO/错误码以 compact 为准自洽；可进入 ✅ 功能验收。
