# cashier-template 集成清单

## 模块路由 / API / 页面入口
- 后端路由接入：`services/admin-api/internal/transport/httpserver/admin_wiring.go` 已注册 cashier 路由组（鉴权链 + 权限码）。
- 后端 API（`/api/admin/cashier`）：
  - `GET/POST /templates`
  - `GET /templates/{templateId}`
  - `POST /templates/{templateId}/versions`
  - `POST /templates/{templateId}/versions/{version}/copy-to-draft`
  - `GET/PUT /templates/{templateId}/versions/{version}/rows`
  - `POST /templates/{templateId}/versions/{version}/publish`
  - `POST /templates/{templateId}/fx-sync/runs`
  - `POST /fx-sync-runs/{runId}/approve`
- 前端路由：`/cashier`（`apps/admin-web/src/router/routes.ts`，菜单名「收银台」，权限 `cashier.read`）。
- 页面入口：`apps/admin-web/src/views/cashier/CashierView.vue`。
- API 客户端：`apps/admin-web/src/api/modules/cashier.ts`，覆盖：
  - `GET/POST /api/admin/cashier/templates`
  - `GET /api/admin/cashier/templates/{templateId}`
  - `POST /api/admin/cashier/templates/{templateId}/versions`
  - `POST /api/admin/cashier/templates/{templateId}/versions/{version}/copy-to-draft`
  - `GET/PUT /api/admin/cashier/templates/{templateId}/versions/{version}/rows`
  - `POST /api/admin/cashier/templates/{templateId}/versions/{version}/publish`
  - `POST /api/admin/cashier/templates/{templateId}/fx-sync/runs`
  - `POST /api/admin/cashier/fx-sync-runs/{runId}/approve`

## 引用模块 / 外部依赖 / 共享 surface
- 依赖公共契约：`common`（版本状态机、currency_specs、统一包络、权限码）。
- 依赖共享接线：`services/admin-api/internal/transport/httpserver/admin_wiring.go`。
- 前端共享 surface：`apps/admin-web/src/views/cashier` 与 `apps/admin-web/src/router/routes.ts`。
- UI/状态复用：`PageCard`、`EnvironmentBadge`、`v-perm` 指令。

## 需要统一接入的共享文件
- `services/admin-api/internal/transport/httpserver/admin_wiring.go`：已临时接入 cashier service/router（后续集成 Agent 统一核对与其它模块冲突）。
- `services/admin-api/migrations/000007_cashier_template_platform_schema.up.sql`：新增迁移，需按整体迁移计划统一验证前向执行。
- `apps/admin-web/src/router/routes.ts`：已接入 cashier 顶层路由和权限码。
- 当前未修改其它共享入口文件；若后端实际 API 路径差异，请由集成阶段统一调整。

## 已知问题 / 待完善点
- compact 示例将 `version` 展示为字符串；当前后端以数值版本对外（数据库仍保存为 VARCHAR 数字串）。
- `ignore` 动作：`POST /fx-sync-runs/{runId}/approve` + `{ "action": "ignore", "reviewNote": "..." }` 已由后端 CR 实现（置 run=ignored，候选 draft 保留）。
- `fxSyncRuns` 列表当前从模板详情读取；后端 `GET /templates/{id}` 现无 `fxSyncRuns` 字段，需补字段或独立列表 API。
- `GET /templates/{id}` 响应为嵌套 `{ template, versions }`；前端 CR 已在 `getCashierTemplate` 做归一化，待后端定稿。
- PUT rows：前端按 compact 发送 minor 字段；后端 handler 仍收 `preTaxAmount` 字符串，集成阶段须统一。
- `currency_specs` 前端暂用 compact 固定种子做预校验，后续可切换到系统字典接口。

## 🟪 集成测试问题清单（移交 🟧；阻断须修复后回 🟪 复测）
> 复测轮次：后端 `go build/go test ./...` ✅ 全绿；前端 `vitest` cashier 39/39 ✅、全量 121/122（1 既有非本模块 games 用例失败）。契约对账发现 3 阻断 DTO 漂移（前后端并行开发未对齐），**不可进入 ✅ 验收**。

> 🟧 集成修复（Opus 4.8 High）已处理 P1/P2/P3/P4（+P5 去 GET template internal id），自检全绿，待 🟪 复测。详见 audit.log.md「🟧 高级全栈工程师 · 集成修复」与下方「🟧 集成修复结果」。

- [x] **P1（阻断）PUT rows 金额字段三重漂移（字段名/类型/语义）** —— 🟧 已修复（前端改发 preTaxAmount major 字符串 + taxRate 字符串，归一化留后端）
  - 定位：前端 `apps/admin-web/src/api/modules/cashier.ts:37-50,196-201` + `views/cashier/templates/PriceMatrixEditor.vue:289-302`（下发 `preTaxAmountMinor:number` + `taxRate:number`）vs 后端 `services/admin-api/internal/transport/http/cashier/handler.go:38-48`（收 `preTaxAmount:string`(major) + `taxRate:string`）。
  - 现象：Go json 解码 number→string 直接失败 → PUT body 整体 400 VALIDATION_FAILED（实证）；即字段名对齐，`preTaxAmount` 缺失 → 后端归一化空串报错。「保存矩阵」连真实后端必失败。
  - 期望（compact §价格行 / 00 §5：归一化读 currency_specs→校验→舍入→存 minor，归一化职责在后端）。
  - 建议：**前端改为下发 `preTaxAmount`(major 字符串，直接用现有 `preTaxMajorInput`) + `taxRate`(字符串)**，与后端归一化入口对齐；后端保持现状。
- [x] **P2（阻断）FX runs 来源缺失 + run DTO 字段不匹配** —— 🟧 已修复（GET template 内嵌 fxSyncRuns；DTO 对齐 runId/candidateVersion 对外版本号/diffSummary；新增 ListFXSyncRuns）
  - 定位：后端 `handler.go:150-161 GetTemplate`（仅回 `{template,versions}`，无 fxSyncRuns）+ `handler.go:64-74 fxRunResponse`（`id/candidateVersionIdRef/diffSummaryJson`）vs 前端 `cashier.ts:52-61,174`（`runId/candidateVersion/diffSummary`）。
  - 现象：FX 审核列表连真实后端永远空；即便补来源，runId/候选版本/差异摘要全空，approve 用 `row.runId`(undefined) → 调 `/fx-sync-runs/undefined/approve`。
  - 期望（compact §前端要点「汇率同步审核列表」可消费）。
  - 建议：**后端 `GET /templates/{id}` 增 `fxSyncRuns:[]`（或新增 `GET /templates/{id}/fx-sync/runs`），run DTO 对齐：`runId`=id、`candidateVersion`=候选版本号字符串（注意当前给的是 versions 表 id，须换成版本号）、`diffSummary`=diffSummaryJson**。需总负责定契约落点。
- [x] **P3（阻断·次路径）CreateVersion sourceVersion 类型漂移** —— 🟧 已修复（后端 handler 改收 string + Atoi；service 内部仍 int）
  - 定位：后端 `handler.go:33-36 createVersionRequest.SourceVersion int` vs 前端 `cashier.ts:88-91 sourceVersion:string`。
  - 现象：copy 路径下发字符串 "7" → Go 解码 string→int 失败 → 400（实证）。主复制入口走 copy-to-draft（URL 参数）不受影响。
  - 期望（compact §POST versions：`sourceVersion string`）。
  - 建议：**后端 `SourceVersion` 改 `string` + `strconv.Atoi`**（对齐 compact）。
- [x] **P4（缺陷·非阻断）auto_apply 触发响应 status 滞后** —— 🟧 已修复（TriggerFXSyncRun 事务内 reload run，响应 status 与落库一致）
  - 定位：`app/cashier/service.go:268-321 TriggerFXSyncRun`（auto_apply 分支未 reload run）+ `handler.go:258-265 CreateFXRun`。
  - 现象：`POST .../fx-sync/runs` 回 `data.status=pending_review`，落库实为 applied。manual_confirm 默认不触发。
  - 建议：**事务结束前 `repo.GetFXSyncRun(run.ID)` reload 或返回最终态**。
- [ ] **P5（一致性·非阻断·可选）**：approve 响应 `{status}` 与前端 `approveFxSyncRun` 标注 `FxSyncRun` 不一致（仅 refresh，可接受）；publish 响应 `{status:"published"}` 无 version（仅 refresh，可接受）；`GET template.template` 内嵌 internal `id`（建议后端去除）。

## 🟪 契约 5 项遗留裁决（以 compact 为唯一标准）
1. FX ignore payload → ✅ **已解决**（后端 `handler.go:278` action:"ignore"→IgnoreFXSyncRun，置 ignored，候选 draft 保留）。
2. fxSyncRuns 来源 → ❌ **未解决·阻断 P2**（GET template 无该字段 + run DTO 不匹配）。
3. PUT rows minor vs preTaxAmount → ❌ **未解决·阻断 P1**。
4. sourceVersion int/string → ❌ **未解决·阻断 P3**；version 对外 int·DB VARCHAR → ✅ 可接受（前端 normalizeVersion 已 String() 兼容，排序按 int）。
5. auto_apply 响应 status 回 pending_review·落库 applied → ⚠️ **缺陷确认 P4**（非阻断 manual 主路径）。

---

## 🟪 第 2 轮复测对账（🟧 修复后，以 compact 为唯一裁决标准）
> 复测时间：2026-06-29 23:0x。前置：🟧 已修 P1–P5。回归：后端 `go build`+`go test ./... -count=1` ✅ 全绿（含 `testkit/scenario` manifest）；前端 `vitest` cashier 39/39 ✅、全量 121/122（1 既有非本模块 `games/sync-section-drawer` 失败）。
> 解码实证（Go json，新载荷形态）：PUT rows / CreateVersion(sourceVersion string) / FX run DTO 三者 `decode err = <nil>` 且字段正确——上轮三处解码失败全部消除。

### 各问题复测裁决
- [x] **P1 PUT rows → ✅ 已解决**。前端 `cashier.ts:101-113 PutPriceRow{preTaxAmount,taxRate:string}` + `PriceMatrixEditor.vue:300-308`（下发 major 字符串 + taxRate 字符串，保留 currency_specs 预校验/舍入预览，published/archived 只读不变）；后端 `handler.go:39-49 upsertRowsRequest` 同字段、`service.go:normalizeRow` 归一化为 minor。证据：`TestUpsertRowsNormalizesAmount`（preTaxAmount "10.00"→preTaxAmountMinor 1000/tax 100/afterTax 1100）、`TestUpsertRowsCurrencyNotSupported`、`TestUpsertRowsBelowMinimum`、`TestUpsertRowsOnPublishedRejected`。
- [x] **P2 FX runs 来源 + DTO → ✅ 已解决**。后端 `handler.go:178-195 GetTemplate` 内嵌 `fxSyncRuns: mapFXRuns(runs)`；`fxRunResponse`(`handler.go:65-74`) 字段 `runId/candidateVersion(对外版本号 strconv.Itoa)/diffSummary`，经 `FXSyncRunView`(`ports.go:83-86`)+`ListFXSyncRuns`(`service.go:332`,`postgres`,`memstore`) 提供版本号。前端 `cashier.ts:52-61 FxSyncRun` 字段一一对齐。证据：`TestFXManualConfirmTriggerThenApprove`(用 `runId`、approve→applied)、`TestFXAutoApplyTriggerPublishes`(`candidateVersion` 非空字符串)、`TestFXIgnore`。
- [x] **P3 sourceVersion → ✅ 已解决**。后端 `handler.go:34-37 createVersionRequest.SourceVersion string` + `handler.go:207-215` `strings.TrimSpace`→空=0、`strconv.Atoi` 非法→400。前端 `cashier.ts:88-91 sourceVersion?:string`。证据：`TestCreateVersionDefaultsToDraft`、`TestCreateVersionCopyMissingSourceVersion`、`TestCreateVersionCopyFromDraftRejected`；主复制入口 `copy-to-draft`（`TestCopyToDraftFromPublished/Archived`）不受影响。
- [x] **P4 auto_apply status 滞后 → ✅ 已消除**。`service.go:315-325 TriggerFXSyncRun` auto_apply 分支事务内 `repo.GetFXSyncRun(run.ID)` reload，`view.Run` 反映 applied。证据：`TestFXAutoApplyTriggerPublishes` 断言响应 `status=applied` + 候选 published + 旧 archived。
- [x] **P5 GET template internal id → ✅ 已解决**。`handler.go:85-94 toTemplateResponse` 仅出对外字段（无 `id`）；`versionResponse` 同样无内部 id。前端归一化无需再忽略 internal id。

### 红线复核（不回归）
- 权限码：`router.go` cashier.read/write/publish + fx.approve 与前端 `v-perm` 一致 ✅（未改 router/admin_wiring/routes）。
- platform schema 无 env：迁移 000007 四表平台级 ✅。
- 事务回滚：`TestPublishTransactionRollback`、`TestFXApproveTransactionRollback` 复跑通过 ✅。
- 脱敏：本模块无密文 N/A；下游 game-cashier 绑版本快照、published 只读→`cashier_price_rows.price_id` 语义稳定 ✅。

### 遗留 / 最终判定
- 无阻断遗留。唯一失败用例 `apps/admin-web/src/views/games/detail/__tests__/sync-section-drawer.spec.ts`（"Create draft from published v7" 文案陈旧）= 仓库既有 games 存量问题，**非 cashier**，建议平台/games 角色另行清理（不阻断本模块）。
- **最终通过判定：✅ 可进入功能验收。**

## 后端测试产物（backend-test）
- L1 单元（无 IO）：`internal/domain/cashier/template_version_test.go`、`internal/domain/common/currency_test.go`、`internal/app/command/copy_template_version_test.go`、`internal/app/cashier/calc_test.go`。
- L3 接口（进程内 httptest + 内存仓储 + 真实 JWT/审计 spy）：`internal/transport/http/cashier/cashier_http_test.go`（22 Test）、`internal/transport/http/cashier/memstore_test.go`。
- 场景矩阵 manifest：`tests/backend/scenarios/cashier-template.yaml`（10 接口 × S1–S10；本模块平台级 → S6 按 platform schema 隔离 + search_path 说明，S8 N/A 无密文）。
- fixtures：`tests/fixtures/common/cashier-template.sql`（cashier_admin/cashier_reader RBAC + global_default/auto_tpl 模板 + 版本/价格行，幂等）。
- 回归入口：已挂统一入口——`scripts/regression/backend.sh` 跑 `go test ./...`（含上述就近用例）+ scenario harness glob `tests/backend/scenarios/*.yaml`，无需改脚本。连库全维度跑：`SCENARIO_WITH_DB=1`。
- 测试使能改动：`internal/transport/httpserver/admin_wiring.go` 降级路由挂载 cashier(ready=false)；`internal/transport/httpserver/router_test.go` copy-to-draft 断言改 401（真实路由接管）。
- 待集成 Agent 关注：连库 harness 需按 `cashier-template.yaml` 的 `fixture:` 片段名（base/with_draft/with_published/publishable/auto_apply_published/fx_pending/fx_applied）拼装 SQL；`expect.db`/`expect.audit` 断言可后续在 harness 扩展。

## 前端测试产物（frontend-test）
- L4 vitest 组件（就近 `src/views/cashier/**/__tests__`，5 文件 / 39 Test）：`PriceMatrixEditor.spec.ts`(12)、`TemplateVersionsTab.spec.ts`(6)、`FxSyncRunsReviewList.spec.ts`(8)、`CashierView.spec.ts`(6)、`components/CashierDialogs.spec.ts`(7)。mock `@/api/modules/cashier`，覆盖归一化/只读/状态/权限/对话框。
- L5 Playwright（`tests/frontend/e2e/cashier.spec.ts`，12 Test）：对 `/api/admin/cashier/**` 契约 mock，覆盖列表/详情/版本入口/价格矩阵/FX 审核/权限置灰/空·错态 + 截图 + 视觉基线。
- 截图：`tests/frontend/screenshots/cashier-*.png`；视觉基线：`tests/frontend/visual-baseline/cashier.spec.ts-snapshots/cashier-list-*.png`（已生成纳入）。
- 回归入口：vitest 自动收 `src/**/*.spec.ts`；`playwright.config.ts` 自动收 `tests/frontend/e2e/*.spec.ts`，并入 `scripts/regression/frontend.sh`，无需改脚本。
- **测试使能改动（需集成 Agent 知悉）**：`apps/admin-web/package.json` 新增 devDependency `@vue/test-utils@^2.4.11`。原仅声明 `@testing-library/vue`，其 peer `@vue/test-utils` 仅作传递依赖、pnpm 默认不在顶层暴露，致 `src/test/setup.ts` 导入失败、**整套 vitest（含既有 games/channels 用例）无法启动**。锁文件已存在 2.4.11，安装无新增解析。
- 运行结果：vitest cashier 39/39 ✅、全量 121/1（1 失败为既有 games sync-section-drawer，非本模块）；Playwright cashier 12/12 ✅。
- 环境注记：本机 vite dev 冷编译 + Element Plus 首屏渲染慢（~20-30s/页），e2e 已在 spec 内抬高超时（单测 120s、详情等待/截图 30s）并以 `--workers=1` 串行跑通；导航与契约本身无误。

## 疑似实现缺陷（待 🟦后端开发，经总负责调度）
- `Service.TriggerFXSyncRun` 在 `auto_apply` 路径返回的 `run` 未在事务内 reload：响应体 `POST /fx-sync/runs` 的 `data.status` 仍回 `pending_review`，而落库实为 `applied`（候选已发布、旧 published 已归档）。前端若据响应判断会误判，建议返回最终态。

## 集成步骤 / 验证命令 / 风险说明
- 集成步骤：
  1. 在数据库执行新增迁移，确认四张 cashier 平台表及约束/索引生效（`platform` schema）。
  2. 验证后端发布事务语义：同模板仅一个 published，发布新版本自动归档旧版本。
  3. 验证 FX 审核语义：`manual_confirm` 下 approve 同事务 apply（发布候选版本并置 run=applied）。
  1. 确认后端返回字段与 `api/modules/cashier.ts` 类型对齐（尤其 `fxSyncRuns`、approve payload）。
  2. 联调 `publish` 与 `copy-to-draft` 状态流转（draft/published/archived）。
  3. 联调 FX 审核链路（trigger → pending_review → approve/ignore）。
- 验证命令：
  - `cd services/admin-api && go build ./...`
  - `cd services/admin-api && go vet ./...`
  - `pnpm --dir apps/admin-web exec vue-tsc --noEmit`（当前仓库已有测试类型错误，会失败）
  - `pnpm --dir apps/admin-web exec vite build`（当前通过）
- 风险说明：
  - 现阶段类型检查失败来自仓库既有测试依赖/类型问题，不是 cashier 页面编译错误；建议由测试/平台角色统一补齐。

## 🟧 集成修复结果（移交回 🟪 复测）
- 契约统一以 compact 为准：
  - PUT rows：前后端统一为 `preTaxAmount`(major 字符串) + `taxRate`(字符串)，后端归一化为 minor（00 §5）；前端保留 currency_specs 预校验 + 舍入预览；published/archived 只读语义不变。
  - FX run DTO：统一 `runId` / `candidateVersion`(对外版本号字符串) / `diffSummary` / `status` / `triggeredAt` / `reviewedBy` / `reviewedAt` / `reviewNote`（camelCase）；FX 审核列表数据源由 `GET /templates/{id}` 内嵌 `fxSyncRuns` 提供。
  - `sourceVersion`：统一为字符串（后端 handler Atoi）。
- 改动文件：后端 `handler.go`/`service.go`/`ports.go`/`cashier_repo.go`；前端 `api/modules/cashier.ts`/`PriceMatrixEditor.vue`；测试/契约 `cashier_http_test.go`/`memstore_test.go`/`PriceMatrixEditor.spec.ts`/`tests/backend/scenarios/cashier-template.yaml`。
- **共享文件**：本次修复**未改动** `admin_wiring.go` 与 `routes.ts`；改动范围限定在 cashier-surface（cashier handler/service/ports + postgres cashier 仓储 + 前端 cashier api/视图）。
- 自检：后端 `go build/vet/test ./...` ✅ 全绿（含 scenario manifest `-count=1`）；前端 `vitest run src/views/cashier` ✅ 39/39、`vite build` ✅。
- 复测建议：重点回归 ①价格矩阵保存（连真实后端 PUT）②FX 审核列表渲染 + approve 携带正确 runId ③auto_apply 触发响应 status=applied ④copy 版本下发字符串 sourceVersion。
