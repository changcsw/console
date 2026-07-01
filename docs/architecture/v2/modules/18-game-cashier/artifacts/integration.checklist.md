# 18 · game-cashier — 集成清单（integration.checklist.md）

> 由开发 / CR / 测试 / 集成角色共同维护。本模块在 `console` 工作区的 `codex/game-cashier` 分支开发。

## 模块路由 / API / 页面入口
- 后端 API 前缀：`/api/admin/games/{gameId}/cashier`
  - GET `/profile`（`cashier.read`）：返回 `{templateId, appliedTemplateVersion, snapshotChecksum, appliedAt}`；未绑定时 404
  - PUT `/profile`（`cashier.write`）：`templateId` + `templateVersion`，仅 published，审计 `cashier.profile.bind`
  - GET `/price-overrides`（`cashier.read`）：`{items:[...]}`
  - PUT `/price-overrides`（`cashier.write`）：整体替换，审计 `cashier.override.update`
- 前端：游戏详情 →「收银台」Tab（`GameCashierTab.vue`）；顶层 `/cashier` 模板页未改动

## 引用模块 / 共享 surface
- 依赖：`cashier-template`(#17)、`game`(#11)、`common`(#00)
- 共享 surface（cashier-surface lane）：
  - `services/admin-api/internal/domain/cashier`
  - `services/admin-api/internal/transport/http/cashier`
  - `apps/admin-web/src/views/cashier`、`api/modules/cashier.ts`

## 共享文件接入
- [x] `admin_wiring.go`（复用 cashier 注册）
- [x] `cashier/router.go` + `handler.go`
- [x] `router/routes.ts`（无需新增路由）
- [x] `GameDetailView.vue` + `game/GameCashierTab.vue`

## CR 核对（前端 compact 要点）
- [x] 游戏详情「收银台」Tab — `GameDetailView.vue:49-51`
- [x] 已绑定模板/版本/时间/校验和 — `GameCashierTab.vue:19-27`
- [x] 切换/升级 published 版本 — `GameCashierTab.vue:36-69`
- [x] 游戏级覆盖列表/编辑 + currency_specs 舍入预览 — `GameCashierTab.vue:106-197,414-466`
- [x] 模板矩阵 vs 覆盖边界 + 高亮 — `GameCashierTab.vue:73-104,277-314,643-645`
- [x] API client camelCase DTO — `cashier.ts:120-145,282-312`
- [x] EnvironmentBadge + cashier.write 置灰 — `GameCashierTab.vue:3-8,15,60,109`
- [x] 空/错态 — `GameCashierTab.vue:29-34,9,100-103,529-539`
- [x] 未绑定 profile 404→空态（CR 修复）— `cashier.ts:getGameCashierProfile`

## 后端测试（🟦🧪 后端测试，已完成）
- [x] L1 domain 单测：`internal/domain/cashier/game_cashier_test.go`（10 用例，归一化边界 + 整行覆盖优先级）
- [x] L3 接口 httptest：`internal/transport/http/cashier/game_cashier_http_test.go`（19 用例，S1/S2/S3/S4/S5/S7/S10）
- [x] 场景矩阵 manifest：`tests/backend/scenarios/game-cashier.yaml`（28 cases，4 接口 × S1–S10，含 N/A 说明）
- [x] fixtures：`tests/fixtures/sandbox/game-cashier.sql`（base/with_draft/publishable_with_checksum/bound/with_overrides）
- [x] 运行：`go build/vet/test ./...` 全绿（失败 0）；scenario S2 真跑 PASS、requiresDB 跳过（连库承担）

## 前端测试（🟩🧪 前端测试，已完成）
- [x] L4 vitest 组件测试：`apps/admin-web/src/views/cashier/game/__tests__/GameCashierTab.spec.ts`（11 用例：绑定/升级、覆盖编辑、currency_specs 约束与舍入预览、矩阵 vs 覆盖高亮、无 write 置灰、空/错/未绑定 null 态）
- [x] L5 Playwright UI 用例：`tests/frontend/e2e/game-cashier.spec.ts`（8 用例，契约 mock + 截图 + 视觉基线）
- [x] 视觉基线：`tests/frontend/visual-baseline/game-cashier.spec.ts-snapshots/game-cashier-tab*.png`；截图 `tests/frontend/screenshots/game-cashier-*.png`
- [x] 运行：vitest 11 passed（合并复跑 20 passed，无回归）；Playwright 8 passed；本模块 vue-tsc 0 错误
- [x] CR 建议「补 GameCashierTab 单测」已完成；「loadingProfile 未挂 v-loading」属体验级非阻断（矩阵/覆盖表已 v-loading、绑定按钮有 loading 态），未回退

## 集成 / 系统测试（🟪 测试专家，第 1 轮，❌未通过）
- [x] 契约对账：4 接口逐项核对（方法/路径/DTO/错误码/包络）— 见下「疑似缺陷」新增第 3、4 条
- [x] 后端全量回归：`go build/vet/test ./...` 全绿；game-cashier L3 19 用例 + `game-cashier.yaml` 28-case 解析全 PASS
- [x] 红线核验：脱敏 N/A、权限 read/write(403 验证 PASS)、env schema 隔离(业务 SQL 无前缀/无 env 谓词、平台表只读)、事务回滚(全量替换 InTx PASS)、与 payment/IAP 路由隔离、本模块无 production Sync 入口
- [x] 下游抽查：snapshot/sync 读 `snapshot_checksum` 做 drift 比对，受阻断 checksum 缺陷牵连；payment 无新增契约漂移
- [ ] 连库 S1/S6/S10 + 真实前后端 e2e：本机 shell 沙箱故障(`cursorsandbox ENOENT`) + 无 PG，未执行（待修复后复测轮补）
- [ ] 前端 vitest 复跑：同上未执行，沿用前端车道结论(20 + e2e 8 passed，均 mock)

## 已知问题
- 全局 `vue-tsc` 既有错误（`ChannelLoginConfigPanel.vue`、`sync-section-drawer.spec.ts`），非本模块；本模块文件 0 错误。
- 前端测试以契约 mock 为主；连库跨栈联调属 🟪测试专家。

## 跨模块改动（🟧 集成修复，⚠️ 供集成 Chat 知悉）
- ⚠️ **#17 cashier-template（同 lane `cashier-surface`，已合并依赖）发布流程补齐 version.checksum**：
  - `services/admin-api/internal/domain/cashier/cashier.go`：新增 `ComputeVersionChecksum(rows []PriceRow) string`（按唯一键排序 + 业务字段规范串 + sha256 hex，确定性）。
  - `services/admin-api/internal/app/cashier/service.go`：`PublishVersion`（手动发布）与 `approveRunInTx`（FX approve 发布候选）发布前计算并写入 checksum；新增 helper `computeVersionChecksum`。
  - `services/admin-api/internal/app/cashier/ports.go` + `infra/persistence/postgres/cashier_repo.go` + `transport/http/cashier/memstore_test.go`：仓储 `PublishVersion` 增 `checksum` 入参（SQL `UPDATE ... SET ..., checksum=$3`）。
  - 影响面：#17 发布的 published 版本自此带非空 checksum；下游 snapshot/sync 的 cashier section drift/baseline 比对恢复有效校验语义。不改 #17 对外 API 形状/DTO，仅补已存在但未计算的 checksum 字段；#17 既有测试全 PASS。

## 集成第 1 轮 4 项问题（🟧 已修复，回 🟪 复测）
- [x] **【阻断1】PUT /price-overrides `taxRate` 契约漂移** → 统一以 **string** 为准（compact + GET/scenario/#17 模板既有约定）。后端本为 string，前端改为 string：`apps/admin-web/src/api/modules/cashier.ts:132-145`、`GameCashierTab.vue`(`fromMinorToEditable`/`normalizeOverrideRow`/`displayRows`)；同步 `GameCashierTab.spec.ts`、e2e `game-cashier.spec.ts`。
- [x] **【阻断2】绑定主流程不可达** → 见上「跨模块改动 #17」补 checksum 计算；#18 `BindProfile` 移除 `checksum==""→CONFLICT` 超规格硬闸（compact 仅列模板存在 + 版本 published），原样记录版本 checksum。回归 `TestBindProfilePublishedComputesChecksum` PASS。
- [x] **【非阻断3】GET /price-overrides `taxRate` 类型标注漂移** → 随阻断1 一并对齐为 string。
- [x] **【非阻断4】重复键未预检** → `SavePriceOverrides` 归一化后以 `seen` 集合预检重复 `(country,region,currency,priceId)` → 400 `VALIDATION_FAILED`。同步 `TestSavePriceOverridesDuplicateKeyPrevalidated`、scenario `save_overrides_duplicate_key_prevalidated`（409→400）。

## 集成 / 系统测试（🟪 测试专家，第 2 轮，✅通过 → 可进入功能验收）
- [x] 契约对账复核：`taxRate` PUT 请求体（前端 `String()` 出参 / 后端 `string`）+ GET 响应（后端 `string` / 前端 DTO `string`）+ scenario yaml（`"0.1"`）三处一致，**number/string 漂移已清除**；其余 3 接口契约保持一致
- [x] 修复复现：BindProfile 真实发布路径 → 200 且 snapshotChecksum 非空（`TestBindProfilePublishedComputesChecksum` PASS）；重复 items → 400 VALIDATION_FAILED（`TestSavePriceOverridesDuplicateKeyPrevalidated` PASS）
- [x] 后端全量回归：`go build/vet/test ./...` 全绿（0 失败，无回归）；game-cashier L3 + domain + scenario 解析全 PASS
- [x] 前端：vitest 3 files/20 passed；本模块 vue-tsc 0 错误（5 条均既有非本模块，定向规避）；Playwright e2e **8 passed (5.1m)**（出沙箱重跑；沙箱内 8 failed 系浏览器 launch 被拦的环境假阴性）
- [x] 红线核验保持：脱敏 N/A、权限、env 隔离、事务回滚、路由隔离、无 production sync
- [x] 下游抽查：checksum 非空且确定性 → snapshot/sync drift 比对恢复有效（第 1 轮连带隐患已消除）
- [ ] 连库 S1/S6/S10（expect.db）+ 真实 PG 跨栈 e2e：本机无 docker/PG 未执行 → **环境性残留，待集成 Chat / 带 PG 的 CI 补跑**（不阻断验收）
- 结论：**4 项问题（阻断1/阻断2/非阻断3/非阻断4）全部清除，无回归、无新增缺陷；可进入 ✅功能验收**。

## ✅功能验收（验收师，2026-06-30，✅通过）
- [x] 验收清单 25 项全 PASS / 0 FAIL（详见 `audit.log.md [acceptance]` 表格）：4 接口契约/权限码/错误码/包络 · 快照绑定(published+确定性checksum+显式升级) · 整行覆盖 · 金额归一化 · 全量替换事务回滚 · 重复键预检 · 前端6点 · 红线4项 · operation-flow 步骤7 · 下游 snapshot/sync/payment 无破坏
- [x] 统一回归入口：`WITH_DB=0 sh scripts/regression/backend.sh` PASS；`summarize.sh` → `tests/reports/summary.md` 后端 go test **pass=591 / fail=0**
- [x] 后端：build/vet/test ./... 全绿；game-cashier L3 **19/19**；domain + scenario(28-case 解析) PASS
- [x] 前端：vitest **20 passed**；vue-tsc 本模块 0 错误（5 条均既有非本模块，定向规避）；Playwright e2e **8 passed (5.1m)**（出沙箱，含视觉基线）
- [x] operation-flow 步骤7 端到端：前置 A8 published 模板 → 产出 profiles+overrides → 完成判定「已绑定有效版本」→ 下一步 8 支付路由；绑定主流程真实 API 可达
- [x] 红线：env schema 隔离 · 事务回滚(进程内真回滚 `TestSavePriceOverridesTransactionRollback`) · 无 payment/IAP 耦合 · 审计 cashier.profile.bind/cashier.override.update 均 ✅
- [ ] 连库 S1/S6/S10(expect.db)+真实 PG 跨栈 e2e：本机无 docker/PG 未执行 → **环境性残留，待 `WITH_DB=1 scripts/regression/run.sh` CI 补跑**（不阻断）
- 结论：**功能验收通过**；环境性连库维度标注待 CI 补跑。

## 验证命令
- 本模块前端：`pnpm --dir apps/admin-web exec vitest run src/views/games/detail/__tests__/GameDetailView.spec.ts src/views/cashier/__tests__/CashierView.spec.ts`
- 后端：`cd services/admin-api && go build ./... && go vet ./... && go test ./... -count=1`
- 定向：`go test ./internal/domain/cashier/... ./internal/transport/http/cashier/... ./internal/testkit/scenario/ -count=1`
- 迁移：`000012_game_cashier_schema.{up,down}.sql`
