# 18 · game-cashier — 审计日志（audit.log.md）

> 仅供人类审计。各角色在此追加完整执行日志、命令、失败记录、证据。总负责 Agent 不读本文件。

## [setup] 总负责 Agent 编排初始化
- 依赖闸门：cashier-template(#17) ✅、game(#11) ✅、common ✅；同 lane cashier-surface 在制看板为空 → 允许开工。
- 环境处置：原 worktree `/Users/csw/gitproject/console-game-cashier`（分支 codex/game-cashier）在 console 工作区外，sandbox 不可写；`move_agent_to_root` 因 codex/game-cashier 未推远端而失败。经用户选择，移除该 worktree 并在 console 工作区检出 codex/game-cashier 分支开发。
- 创建 artifacts 四件套骨架。

## [backend] 🟦 game-cashier 后端实现（Codex 5.3）
- 读取规范：`00-common.md`、`01-structure.md`、`CONVENTIONS.md`、`modules/18-game-cashier/spec.compact.md`、`modules/17-cashier-template/spec.compact.md`、`modules/11-game/spec.compact.md`。
- 基线核对：复用现有 `services/admin-api/internal/domain/cashier`、`internal/app/cashier`、`internal/infra/persistence/postgres/cashier_repo.go`、`internal/transport/http/cashier`，在 #17 模板能力上增量扩展。
- 迁移新增：`services/admin-api/migrations/000012_game_cashier_schema.up.sql`、`000012_game_cashier_schema.down.sql`。
  - 新增/兜底业务表：`game_cashier_profiles`、`game_cashier_price_overrides`。
  - 约束命名按 compact：`gcp_game_key`、`gcpo_key`；并补跨 schema FK 到 `platform.cashier_price_templates` / `platform.cashier_price_template_versions` / `platform.currency_specs`。
- 领域新增：`internal/domain/cashier/game_cashier.go`
  - `GameCashier` 聚合模型（Profile + PriceOverrides）
  - 纯规则：`NormalizePriceOverride`（金额 minor + 币种约束校验入口）与 `OverlayTemplateRows`（整行覆盖语义）。
- 应用层新增（`internal/app/cashier/service.go`）：
  - `GetProfile` / `BindProfile` / `ListPriceOverrides` / `SavePriceOverrides`
  - `BindProfile`：校验 published 版本、读取 checksum、upsert profile、审计 `cashier.profile.bind`
  - `SavePriceOverrides`：归一化 + 全量替换、审计 `cashier.override.update`
- 仓储扩展（`internal/infra/persistence/postgres/cashier_repo.go`）：
  - `ResolveGameRowID`
  - `Get/UpsertGameCashierProfile`
  - `List/ReplaceGameCashierPriceOverrides`
- HTTP 扩展（`internal/transport/http/cashier/router.go` + `handler.go`）：
  - GET `/api/admin/games/{gameId}/cashier/profile`
  - PUT `/api/admin/games/{gameId}/cashier/profile`
  - GET `/api/admin/games/{gameId}/cashier/price-overrides`
  - PUT `/api/admin/games/{gameId}/cashier/price-overrides`
  - 权限码：读 `cashier.read` / 写 `cashier.write`，统一包络与错误码映射沿用 `httpx`。
- 兼容测试基建：更新 `internal/transport/http/cashier/memstore_test.go` 以实现新增仓储接口；新增 `internal/domain/cashier/game_cashier_test.go`。
- 自检命令与结果：
  - 命令：`cd services/admin-api && go build ./... && go vet ./...`
  - 结果：`go build` ✅、`go vet` ✅

## [frontend] 🟩 game-cashier 实现记录（Codex 5.3）
- 文档依据：`index.json`、`01-structure.md` §5、`CONVENTIONS.md`、`00-common.md` §5/§7、`modules/18-game-cashier/spec.compact.md`、`modules/17-cashier-template/spec.compact.md`。
- 复用基线：`apps/admin-web/src/views/cashier/CashierView.vue`、`templates/PriceMatrixEditor.vue`、`stores/dictionary.ts`、`views/games/detail/*`。
- 新增 API client（compact 契约）：`GET/PUT /api/admin/games/{gameId}/cashier/profile`、`GET/PUT /api/admin/games/{gameId}/cashier/price-overrides`，落地于 `apps/admin-web/src/api/modules/cashier.ts`。
- 新增页面：`apps/admin-web/src/views/cashier/game/GameCashierTab.vue`（模板绑定快照、切换/升级版本、覆盖编辑、舍入预览、模板矩阵与覆盖边界高亮）。
- 接入游戏详情 Tab：`apps/admin-web/src/views/games/detail/GameDetailView.vue`（替换原 cashier 占位 Tab 为真实组件）。
- 用例更新：`apps/admin-web/src/views/games/detail/__tests__/GameDetailView.spec.ts`（增加 `GameCashierTab` stub + 文案断言）。

### 自检命令与结果
- `pnpm --dir "apps/admin-web" exec vue-tsc --noEmit` ❌  
  失败原因：仓库内既有错误（非本次改动）  
  `src/views/channels/components/ChannelLoginConfigPanel.vue`（TS7006/TS2345）  
  `src/views/games/detail/__tests__/sync-section-drawer.spec.ts`（TS2322）。
- `pnpm --dir "apps/admin-web" build` ❌  
  同上，因既有 `vue-tsc` 错误导致中断（未进入 vite build 阶段）。
- `pnpm --dir "apps/admin-web" exec vitest run src/views/games/detail/__tests__/GameDetailView.spec.ts` ✅（3 passed）。
- `pnpm --dir "apps/admin-web" exec vitest run src/views/cashier/__tests__/CashierView.spec.ts` ✅（6 passed）。

## [frontend-cr] 🟩🔎 game-cashier 前端 Code Review（Composer 2.5）
- 依据：`spec.compact.md` 前端章节、`01-structure.md` §5、`CONVENTIONS.md`、`17-cashier-template/spec.compact.md`（PriceMatrixEditor 复用约定）。
- 改动范围：`api/modules/cashier.ts`、`views/cashier/game/GameCashierTab.vue`（新增）、`views/games/detail/GameDetailView.vue`、`__tests__/GameDetailView.spec.ts`。
- 核对结论：compact 前端要点均已实现；API DTO 字段 camelCase 与 GET/PUT profile、GET/PUT price-overrides 契约一致；复用 PageCard/PageStatusTag/EnvironmentBadge/v-perm/dictionary.currency_specs；未改动 `/cashier` 模板页（CashierView.spec 6 passed）。
- 发现问题（已修复）：未绑定 profile 时 GET `/profile` 返回 404，原 `getGameCashierProfile` 抛错导致 Tab 整页 error alert 而非空态 → 在 `cashier.ts` 捕获 `NOT_FOUND` 返回 `null`。
- 建议（非阻断）：`loadingProfile` 未挂 v-loading；缺 `GameCashierTab` 专用单测；后端 `taxRate` JSON 为 string，接口类型可扩为 `number | string`。
- tsc 区分：
  - 本模块文件（cashier.ts、GameCashierTab.vue、GameDetailView 改动）：**0 错误**
  - 仓库既有：`ChannelLoginConfigPanel.vue` TS7006/TS2345×4；`sync-section-drawer.spec.ts` TS2322×1
- 复验命令：
  - `pnpm --dir apps/admin-web exec vitest run src/views/games/detail/__tests__/GameDetailView.spec.ts src/views/cashier/__tests__/CashierView.spec.ts` ✅（9 passed）
  - `pnpm --dir apps/admin-web exec vue-tsc --noEmit | rg cashier/game|api/modules/cashier|GameDetailView` → 无输出（本模块 0 错误）
- CR 结论：**通过**（1 处空态 bug 已直接修复）。

## [backend-cr] 🟦🔎 game-cashier 后端 Code Review（Composer 2.5）
- 评审范围：`services/admin-api`（000012 迁移、domain/cashier/game_cashier.go、app/cashier、infra/cashier_repo.go、transport/http/cashier）。
- 对照文档：`spec.compact.md`、`00-common.md` §2/§5/§7/§8、`17-cashier-template/spec.compact.md`、`CONVENTIONS.md`。
- **结论：通过 ✅**（无阻断项；未改代码）。

### 契约核对表（compact → 实现 → 证据）
| 要点 | 已实现 | 一致 | 证据 |
| --- | --- | --- | --- |
| 两表字段/类型/默认 | ✅ | ✅ | `000012_game_cashier_schema.up.sql:5-70` |
| 唯一键 gcp_game_key / gcpo_key | ✅ | ✅ | `000012:20-28,75-83` |
| FK games(id) 同 schema | ✅ | ✅ | `000012:7,57` |
| FK platform 模板/版本/currency_specs | ✅ | ✅ | `000012:31-53,86-96` |
| 迁移追加 000012、幂等、未改历史 | ✅ | ✅ | `000012` DO 块 + `CREATE IF NOT EXISTS`；历史 000001–000011 未动 |
| region_code='*'/snapshot_checksum=''/reason=''/applied_at NOW() | ✅ | ✅ | `000012:10-11,59,66` |
| 金额归一化 00 §5 | ✅ | ✅ | `game_cashier.go:65-73`；`service.go:488-494` |
| 整行覆盖 OverlayTemplateRows | ✅ | ✅ | `game_cashier.go:77-106`；`game_cashier_test.go:41-57` |
| 绑定 published + snapshot_checksum | ✅ | ✅ | `service.go:432-445` |
| 升级绑定显式 PUT profile upsert | ✅ | ✅ | `service.go:419-464`；`cashier_repo.go:373-388` |
| GET/PUT profile、GET/PUT price-overrides | ✅ | ✅ | `router.go:32-35`；`handler.go:401-485` |
| 前缀 /api/admin/games/{gameId}/cashier | ✅ | ✅ | `router.go` + `admin_wiring.go:130-131` |
| 权限 cashier.read / cashier.write | ✅ | ✅ | `router.go:32-35` |
| DTO camelCase + 包络 httpx | ✅ | ✅ | `handler.go:105-123,407,443,484` |
| 错误码 CURRENCY_NOT_SUPPORTED/VALIDATION_FAILED/NOT_FOUND/CONFLICT | ✅ | ✅ | `ports.go:22-28`；`service.go:383-387,409-416,432-437,490` |
| 审计 cashier.profile.bind / cashier.override.update | ✅ | ✅ | `service.go:456-461,506-509` |
| 分层 domain 纯规则无 IO | ✅ | ✅ | `game_cashier.go` 无 context/DB |
| 业务 SQL 无 schema 前缀、无 env 谓词 | ✅ | ✅ | `cashier_repo.go:351-436` |
| 写操作 InTx 事务边界 | ✅ | ✅ | `service.go:419,481`；`cashier_store.go:25-34` |
| 平台表只读（currency_specs/模板查询） | ✅ | ✅ | `cashier_repo.go:257-260`（SELECT）；写落 game_* 表 |

### 问题清单
**阻断**：无。

**建议**（非打回）：
1. `SavePriceOverrides` 未预检 items 内重复 `(countryCode,regionCode,currency,priceId)`，重复键会触发 DB unique 冲突而非 `VALIDATION_FAILED`（`service.go:486-498`）。
2. `000012` down 仅回滚命名约束/索引，未恢复 init 态 FK 命名（best-effort 可接受）。
3. 无 game-cashier HTTP 路由级用例（memstore 已 stub 仓储接口，`memstore_test.go:385-435`）。
4. 建议后续跑 `go test ./...` 全量回归（本 CR 已跑 domain+cashier transport 子集 ✅）。

**已直接修复**：无。

### 验证命令（CR 执行）
```bash
cd services/admin-api && go build ./... && go vet ./... && go test ./internal/domain/cashier/... ./internal/transport/http/cashier/... -count=1
```
结果：build ✅、vet ✅、test ✅（domain/cashier + transport/cashier）。

## [backend-test] 🟦🧪 game-cashier 后端测试（Cursor Auto）

### 读取规范 / 基线核对
- 文档：`03-testing.md`（分层 L1–L5、目录约定、S1–S10 标准维度、scenario manifest 形态、fixtures 约定）、`spec.compact.md`、`tests/backend/scenarios/README.md`。
- 基线：`cashier-template.yaml`（参照 yaml 形态/`requiresDB`/`auth.role`/`fixture` 约定）、`cashier_http_test.go`（进程内 httptest harness：真实 JWT + 内存仓储 + fakeAudit + InTx 真回滚）、`memstore_test.go`（已 stub game-cashier 仓储接口与 games 映射 100001→1）。
- harness 机制确认：`scenario` 包以降级 handler 进程内执行；`requiresDB:false` 的 S2 真跑、`requiresDB:true` 跳过仅校验 YAML 解析；`KnownFields(true)` 限定字段。`admin_wiring.go` degraded 已挂载 cashier 路由形状 → game-cashier S2 进程内可断言。

### 产出 1：单元测试（L1 domain 纯函数，无 IO）
- 文件：`services/admin-api/internal/domain/cashier/game_cashier_test.go`（在原 2 个用例上扩展至 10 个）。
- 覆盖对象：
  - `NormalizePriceOverride`：金额归一化下限（< MinAmountMinor 报错）、负金额报错、缺必填键字段（country/currency/priceId）、effectiveAt 零值报错、大小写/空白规范 + region 默认 `*` + taxRate 默认 `0`。
  - `OverlayTemplateRows`：同键**整行覆盖**（非字段级深合并，模板非覆盖字段不保留）、新键追加、空覆盖集原样返回、region 区分唯一键不互相覆盖。

### 产出 2：接口场景矩阵
- manifest：`tests/backend/scenarios/game-cashier.yaml`（28 cases，4 接口 × S1–S10 逐项标注，含 N/A 说明）。
- 进程内 L3 等价覆盖：`services/admin-api/internal/transport/http/cashier/game_cashier_http_test.go`（新增 19 个 httptest 用例）。
  - 维度映射：S1（绑定成功+读取、覆盖归一化保存、全量替换清空）、S2（4 接口缺/伪造令牌 401）、S3（缺 write→403、缺 read→403）、S4（缺 templateId/版本非法、currency 不支持→CURRENCY_NOT_SUPPORTED、低于下限→VALIDATION_FAILED、effectiveAt 非法、缺 priceId、未绑定→404、模板/游戏不存在→404）、S5（绑定 draft→CONFLICT）、S7（cashier.profile.bind / cashier.override.update 审计 spy）、S10（批量替换含非法 currency→整体回滚，原覆盖不变）。
  - S6：业务表写落当前 env schema（search_path 决定）—进程内无法断言 schema，由连库 harness（`SCENARIO_WITH_DB=1`）承担，已在 yaml 与测试头注明。
  - S8 脱敏 N/A：无 secret/file 密文字段。S9 分页 N/A：profile 单对象、overrides 全量 items[]。S10 对 PUT /profile N/A（单表 upsert）。

### 产出 3：fixtures + 回归入口
- `tests/fixtures/sandbox/game-cashier.sql`（sandbox schema 业务样本：base/with_draft/publishable_with_checksum/bound/with_overrides 片段，依赖 cashier-template.sql + game.sql + currency_specs + 000012 建表）。
- 回归入口：新增 yaml 自动被 `internal/testkit/scenario` 的 `TestScenarioManifests` glob 纳入；就近 `*_test.go` 由 `go test ./...` 与 `scripts/regression/backend.sh` 串联。

### 运行结果（实际执行）
```bash
cd services/admin-api && go build ./... && go vet ./... && go test ./... -count=1
```
- build ✅、vet ✅、`go test ./...` 全绿（无回归）。
- `go test ./internal/domain/cashier/...`：10/10 PASS。
- `go test ./internal/transport/http/cashier/...`：含新增 19 个 game-cashier L3 全 PASS（cashier 包整体 PASS）。
- `go test ./internal/testkit/scenario/ -run TestScenarioManifests/game-cashier`：YAML 解析 ✅；S2（4 个 requiresDB:false）真跑 PASS；其余 requiresDB:true 跳过（manifest parsed OK）。
- 失败数：0。

### 疑似实现缺陷（→ 回退 🟦后端开发）
1. **【主流程阻断】published 版本 checksum 恒空 → 绑定不可达**：`app/cashier/service.go` 创建版本恒 `Checksum:""`，发布流程（`PublishVersion`）不计算/写入 checksum；而 `BindProfile`（service.go:435-438）要求 `version.Checksum != ""`，否则 `conflictErr("published 版本缺少 checksum")`。结论：生产 API 路径下「绑定收银台模板版本」恒返回 409，无法成功。根因在 #17 cashier-template 发布逻辑未生成版本 checksum；game-cashier 依赖其快照校验和。证据用例 `TestBindProfilePublishedMissingChecksum`（断言现状 CONFLICT）；S1 绑定成功仅能在白盒注入/fixture seed checksum 后通过。
2. **【边界/CR 已提】SavePriceOverrides 未预检 items 内重复唯一键**：`service.go:486-498` 未对 `(countryCode,regionCode,currency,priceId)` 去重预检。进程内内存仓储下两条重复键均写入（返回 200/2 条）；真实 DB 命中 `gcpo_key` UNIQUE → INSERT 阶段冲突（非 VALIDATION_FAILED 友好错误）。建议服务层预检返回 400 VALIDATION_FAILED。证据用例 `TestSavePriceOverridesDuplicateKeyNotPrevalidated`（固化现状，含修复后自动 skip 守卫）；yaml `save_overrides_duplicate_key_conflict` 标注期望。

## [frontend-test] 🟩🧪 game-cashier 前端测试（Cursor Auto）

### 读取规范 / 基线核对
- 文档：`index.json`、`03-testing.md`（L4 vitest 组件、L5 Playwright 截图/视觉基线、目录约定）、`spec.compact.md` 前端要点、前端开发/CR handoff（CR 建议：补 `GameCashierTab` 单测、`loadingProfile` 未挂 v-loading）。
- 基线参照：`views/cashier/__tests__/CashierView.spec.ts`（vitest + pinia + perm 指令 mock 约定）、`views/games/detail/__tests__/GameDetailView.spec.ts`、`tests/frontend/e2e/cashier.spec.ts` + `games.spec.ts`（Playwright 契约 mock/截图/视觉基线约定）。
- 约束遵循：对契约做 mock/stub（不连真实后端，跨栈联调属 🟪测试专家）。

### 产出 1：vitest 组件测试（L4）
- 文件：`apps/admin-web/src/views/cashier/game/__tests__/GameCashierTab.spec.ts`（11 用例）。
- 手法：`vi.mock("@/api/modules/cashier")` 全量打桩 7 个接口；pinia 注入 permission/dictionary（直接 seed `currencySpecs` + `loaded=true` 规避 onMounted 网络请求，USD decimalPlaces=2/minAmountMinor=50、JPY decimalPlaces=0 用于边界）；`global.directives.perm` 注入 v-perm；经 `wrapper.vm`（script setup 绑定）断言 computed/方法。
- 覆盖交互点：
  1. 已绑定 → 渲染快照（templateId/snapshotChecksum/「已绑定模板」）。
  2. 未绑定（getGameCashierProfile→null）→ 空态「尚未绑定收银台模板」+「暂无价格矩阵数据」，且**不**请求 `getCashierPriceRows`。
  3. 加载失败（listCashierTemplates reject ApiError）→ 错误 alert 透传 message。
  4. 边界视图高亮：模板矩阵 ∪ 覆盖 merge，同键 `overridden=true`、`matrixRowClassName` 返回 `matrix-row--overridden`；模板独有行 `overridden=false`。
  5. currency_specs 舍入预览：合法行 `previewText` = `preTax=999/tax=100/afterTax=1099`（9.99 USD, taxRate 0.1）。
  6. 币种不在 specs → `CURRENCY_NOT_SUPPORTED`。
  7. 小数位超限（9.999/USD→「最大小数位 2」）+ 最小金额下限（0.10→「最小金额（minor）为 50」）。
  8. 切换/升级版本 → `putGameCashierProfile(gameId,{templateId,templateVersion})` 调用 + 刷新价格行。
  9. 保存覆盖 → 归一化（country 大写、region 空→`*`、priceId trim、major→minor）后 `putGameCashierPriceOverrides`。
  10. 含非法行时不下发保存请求。
  11. 无 cashier.write → 只读 alert + 切换/新增覆盖/保存覆盖按钮 disabled（v-perm 置灰）。

### 产出 2：Playwright UI 用例（L5，契约 mock + 截图 + 视觉基线）
- 文件：`tests/frontend/e2e/game-cashier.spec.ts`（8 用例）。导航：dashboard → 游戏管理 → 星际远征详情 → 「收银台」Tab（守卫前 /me 注入权限）。
- 契约 mock：/me、games 列表/详情、system/currency-specs、cashier templates 列表/详情/rows、`/games/{id}/cashier/profile`（GET/PUT/404）、`/games/{id}/cashier/price-overrides`（GET/PUT）。
- 覆盖：已绑定快照展示、覆盖行『游戏覆盖』vs『模板公共矩阵』tag + `tr.matrix-row--overridden` 高亮、舍入预览 `preTax=1999`、切换/升级触发 PUT profile、保存覆盖触发 PUT price-overrides（body 含 items）、未绑定(404→null) 空态、无 cashier.write 置灰、Tab 视觉基线 `game-cashier-tab.png`。
- 产物：截图 `tests/frontend/screenshots/game-cashier-{profile,matrix,overrides,empty}.png`；视觉基线 `tests/frontend/visual-baseline/game-cashier.spec.ts-snapshots/`。

### fixtures
- vitest 用例内联工厂（summary/detail/profile/templateRow/override），无独立 fixture 文件；Playwright 用例内联契约常量。前端测试不引入连库 fixture（属后端/测试专家）。

### 运行结果（实际执行）
- `pnpm --dir apps/admin-web exec vitest run src/views/cashier/game/__tests__/GameCashierTab.spec.ts` → **11 passed**。
- 合并复跑 `CashierView.spec + GameDetailView.spec + GameCashierTab.spec` → **3 files / 20 passed**（无回归）。
- `vue-tsc --noEmit | rg cashier/game|GameCashierTab` → 无输出（本模块新增测试 0 tsc 错误）。
- `pnpm exec playwright test game-cashier.spec.ts --project=chromium --workers=1` → **8 passed**（首跑 `--update-snapshots` 建基线；含视觉基线比对通过）。单文件耗时 ~6min（本机 vite 冷编译 + EP 渲染慢，已抬高超时；首测偶发 30s tab 等待超时→已加详情标题/cell 就绪等待 + 45s 超时修复）。
- 失败数：0（最终）。修复记录：①首版 e2e currency 预览用例因冷编译 tab 渲染慢超时 → 强化导航等待；②无权限用例 `当前账号仅有查看权限` 命中 4 个兄弟 Tab alert（strict mode 违规）→ 改用本 Tab 专属文案精确匹配。

### 既有错误区分（非本模块引入）
- 全局 `vue-tsc` 仍失败于 `channels/components/ChannelLoginConfigPanel.vue`（TS7006/TS2345）、`games/detail/__tests__/sync-section-drawer.spec.ts`（TS2322）；与 game-cashier 无关，定向运行已规避。

### 疑似实现缺陷（前端测试发现）
- 无新增前端缺陷。CR 建议项「`loadingProfile` 未挂 v-loading」属体验级非阻断，未回退（绑定区已有按钮 loading 态，矩阵/覆盖表已挂 v-loading）。后端阻断项（published checksum 恒空 → 绑定 API 不可达）会令真实跨栈绑定流程失败，但前端契约 mock 层已正确处理 PUT 成功路径；连库联调属 🟪测试专家，标注沿用后端测试结论「回退 🟦后端开发」。

## [integration-test] 🟪 game-cashier 集成/系统测试（Cursor Auto）— 第 1 轮

### 读取规范 / 输入
- 文档：`index.json` → `18-game-cashier/spec.compact.md` → `03-testing.md` → `02-operation-flow.md`；依赖片段 `17-cashier-template/spec.compact.md`。
- 输入：后端/前端车道全部 handoff（audit.log/handoff.summary/manifest/integration.checklist）+ 实际代码（`api/modules/cashier.ts`、`transport/http/cashier`、`app/cashier/service.go`、`domain/cashier/game_cashier.go`、`infra/.../cashier_repo.go`）。

### 1. 契约对账（前端实际调用 vs 后端实际 API）逐项
| 接口 | 方法/路径 | DTO/字段 | 错误码/包络 | 结论 |
| --- | --- | --- | --- | --- |
| GET /profile | ✅一致 `/api/admin/games/{gameId}/cashier/profile` | ✅ {templateId, appliedTemplateVersion(BE strconv.Itoa→string), snapshotChecksum, appliedAt(RFC3339)} | ✅ 未绑定 404 NOT_FOUND→前端捕获返回 null | 一致 |
| PUT /profile | ✅一致 | ✅ {templateId, templateVersion(前端 string→后端 Atoi)} | ✅ VALIDATION/NOT_FOUND/CONFLICT | 一致（受阻断1影响） |
| GET /price-overrides | ✅一致 | ⚠️ `taxRate`：后端 JSON **string**，前端类型标 `number`；运行期 `fromMinorToEditable` 用 `typeof==='string'?Number():` 兜底，**读路径不崩**（非阻断类型卫生问题） | ✅ {items:[]} | 基本一致（类型标注漂移） |
| PUT /price-overrides | ✅一致 | ❌ **`taxRate` 阻断漂移**：前端 `el-input-number`→以 JSON **number** 下发；后端结构体 `TaxRate string`，`json.Decode(number→string)` 直接报错 → handler 返回 **400 VALIDATION_FAILED「请求体格式错误」** | 包络一致 | **不一致（阻断）** |

- 旁证：后端 L3 `validOverrideItem()` 与 `tests/backend/scenarios/game-cashier.yaml` 全部以 `taxRate:"0.1"`（字符串）发送 → 后端用例全绿；真实前端发 number → 必 400。后端/前端两侧自测均 mock/字符串，未触达该跨栈漂移。
- 独立佐证（standalone Go，stdlib encoding/json）：
  - `{"taxRate":"0.1"}` → decode err=nil；
  - `{"taxRate":0.1}` / `{"taxRate":0}` → `json: cannot unmarshal number into Go struct field ...taxRate of type string`。

### 2. 复现已知缺陷
- **缺陷1（阻断）published 版本 checksum 恒空 → BindProfile 恒 CONFLICT**：已代码级确认。
  - `app/cashier/service.go` 全部建版本路径（CreateVersion/CopyToDraft/TriggerFXSyncRun）均 `Checksum:""`；`PublishVersion`（service.go:231-266 + repo `UPDATE ... SET status='published', published_at=...` cashier_repo.go:183-189）**不计算/不写 checksum**；`BindProfile`（service.go:435-438）`if checksum==""→conflictErr("published 版本缺少 checksum")`。
  - 证据用例 `TestBindProfilePublishedMissingChecksum` PASS（固化现状 409 CONFLICT）；`TestBindProfileSuccessAndAudit` 仅靠白盒 `setVersionChecksum` 注入才 200 → 证明真实发布 API 路径下绑定不可达。
  - **裁决（以 compact 为准）**：主因在 **#17 cashier-template**——发布流程应对版本计算并持久化确定性 checksum（覆盖价格行），使 published 版本带非空校验和，以满足 #18「snapshot_checksum=绑定时刻该版本校验和」及 sync drift 检测的设计意图（#17 compact 版本表 checksum 字段存在但发布规则未要求计算 → 实现缺口）。次因在 **#18 BindProfile**——`checksum==""→CONFLICT` 不在 #18 compact 的 PUT/profile 明列校验（仅「模板存在 + 版本 published」），属超规格硬闸，放大了 #17 缺口。建议：**主修 #17（计算 checksum）**；#18 该闸可保留作 defense-in-depth 或放宽记录现有 checksum。
- **缺陷2（非阻断）SavePriceOverrides 重复键未预检**：已确认。`service.go:486-498` 未对 items 内 `(country,region,currency,priceId)` 去重；进程内内存仓储两条同键均写入（`TestSavePriceOverridesDuplicateKeyNotPrevalidated` PASS，返回 200/2 条）；连库走 `ReplaceGameCashierPriceOverrides`（DELETE+INSERT 循环）第 2 条命中 `gcpo_key` UNIQUE → 经 `mapErr`→`ErrConflict` 直接返回（service 未 mapRepoErr 包装）→ **409 CONFLICT（通用「资源冲突」）** 而非 yaml 期望的 400 VALIDATION_FAILED（yaml `save_overrides_duplicate_key_conflict` 已标注现状）。事务整体回滚、安全，但错误语义不友好。建议服务层预检去重返回 400 VALIDATION_FAILED。

### 3. 全量场景回归（实际执行）
- 后端：`cd services/admin-api && go build ./... && go vet ./... && go test ./... -count=1` → **全绿（0 失败，无回归）**。
- 定向：`go test ./internal/transport/http/cashier/... -run 'Game|BindProfile|SavePriceOverrides...' -v` → 19 个 game-cashier L3 用例全 PASS（含 2 个缺陷固化用例）。
- 场景矩阵：`tests/backend/scenarios/game-cashier.yaml`（28 cases）解析校验随 `go test ./...` 纳入并 PASS；S2(requiresDB:false) 进程内真跑 PASS，其余 requiresDB:true 由连库 harness 承担（本机沙箱无 PG/SCENARIO_WITH_DB，未连库）。
- 前端：vitest 复跑因本会话 shell 沙箱 helper 故障（`cursorsandbox ENOENT`）+ 先前 vitest 进程占用持久会话而**无法重跑**；沿用前端车道已记录结论：`GameCashierTab.spec`(11) + 合并 20 passed、Playwright game-cashier.spec(8) passed（均为契约 mock，**不触达** taxRate 跨栈漂移）。
- 统一回归入口 `scripts/regression/run.sh` 连库全栈未执行（需 docker + golang-migrate + 浏览器二进制，且本机 shell 故障）。

### 4. 红线端到端核验
- 脱敏 S8：**N/A**（本模块无 secret/file 密文字段，价格/checksum 均明文业务数据）。
- 权限：GET=cashier.read / PUT=cashier.write（router.go:32-35）；`TestGameCashierForbidden`（缺 write→403、缺 read→403）PASS。✅
- 跨 env（schema 隔离）：业务表 `games`/`game_cashier_profiles`/`game_cashier_price_overrides` SQL **无 schema 前缀、无 env 谓词**（cashier_repo.go:353,363,374,396,420,425），写落当前 env schema（search_path 决定）；平台表显式 `platform.` 前缀且只读（SELECT）。✅
- 事务回滚（全量替换）：`SavePriceOverrides` InTx 内 DELETE+INSERT；任一行归一化/约束失败整体回滚，原覆盖不变（`TestSavePriceOverridesTransactionRollback` PASS）。✅
- 与 payment/IAP 路由隔离：本模块仅注册 `/games/{gameId}/cashier/*`，不触碰 payment-routes / product / IAP。✅
- production 无可执行 Sync：本模块无 sync 入口/动作（sync 属 #21）；GameCashierTab 仅展示 EnvironmentBadge，无 Sync 按钮。✅（红线本身归 sync 模块）

### 5. impacts 下游契约抽查（payment/snapshot/sync）
- snapshot/sync：读 `game_cashier_profiles.snapshot_checksum` 做配置快照与跨 schema drift 比对。受**缺陷1**牵连——checksum 恒空将令 cashier section 的 drift/baseline 检测失去校验意义（连带影响，根因仍是 #17 发布不算 checksum）。
- payment：依赖 game-cashier 价格但本模块未对 payment 暴露新契约（仅新增两张业务表 + 4 个 REST）。无新增漂移。

### 6. 复测轮次结论
- 第 1 轮：**未通过功能验收**。新发现 1 个阻断（taxRate PUT 漂移），复核确认 1 个阻断（checksum）+ 1 个非阻断（重复键）。问题清单移交总负责 Agent → 调度 🟧 修复后回本角色复测。

### 环境备注
- 本会话 shell 在中途出现沙箱 helper `spawn .../cursorsandbox ENOENT`，导致前端 vitest 复跑与连库回归无法执行；后端 `go test ./...` 已在故障前成功执行并全绿。建议复测轮在可用 shell + docker/PG 环境补连库 S1/S6/S10 与真实前后端 e2e。

---

## [fix] 🟧 高级全栈工程师 — 集成第 1 轮 4 项问题修复（branch codex/game-cashier）

裁决基准：以各模块 `spec.compact.md` 为唯一契约。修复保持分层、不扩大范围、不绕过测试。

### 阻断1 + 非阻断3 — PUT/GET `/price-overrides` taxRate 跨栈类型统一为 string
- 问题：前端以 JSON number 下发 `taxRate`，后端 `savePriceOverridesRequest.Items[].TaxRate string`，`json.Decode(number→string)` 报错 → 真实保存恒 400；GET 响应 `taxRate` 后端为 `string`（`tax_rate::text`），前端 interface 标 `number`（标注漂移）。
- 根因：前端契约类型与后端 DTO/GET 响应/`tests/backend/scenarios/game-cashier.yaml`/#17 `PutPriceRow` 既有 string 约定不一致。
- 裁决：统一以 **string** 为准（compact 列出 taxRate 字段，DECIMAL(8,6) 以字符串承载避免浮点漂移；对齐既有 GET/模板/scenario）。后端本就是 string，**两侧对齐＝前端改为 string**。
- 改动：
  - `apps/admin-web/src/api/modules/cashier.ts:132-145`（`GameCashierPriceOverride.taxRate: number → string`，含注释说明）。
  - `apps/admin-web/src/views/cashier/game/GameCashierTab.vue`：`fromMinorToEditable`（`Number(row.taxRate)||0`）、`normalizeOverrideRow` 返回 `taxRate: String(row.taxRate)`、`displayRows` 覆盖分支 `taxRate: Number(...)||0`（UI 内部仍用 number 输入，仅出入参为 string）。
  - 测试同步：`GameCashierTab.spec.ts` override 工厂 `taxRate:"0.1"`；e2e `game-cashier.spec.ts` PRICE_OVERRIDES `taxRate:"0.1"`。
- 验证：前端 vitest 11/11 PASS；本模块 vue-tsc 0 错误。

### 阻断2 — published checksum 恒空 → BindProfile 恒 CONFLICT
- 跨模块改动 **#17 cashier-template**（同 lane cashier-surface，已合并依赖）：
  - 新增确定性 checksum 纯函数 `domain/cashier/cashier.go ComputeVersionChecksum(rows)`（按价格行唯一键排序 → 业务字段规范串 → sha256 hex）。
  - 发布流程在发布时计算并写入 `cashier_price_template_versions.checksum`：`app/cashier/service.go` `PublishVersion`（手动发布）与 `approveRunInTx`（FX approve 发布候选）均先 `ListRows` 计算 checksum 再发布；新增 helper `computeVersionChecksum`。
  - 仓储接口/实现 `PublishVersion(ctx,id,at,checksum)` 增 checksum 入参：`app/cashier/ports.go`、`infra/persistence/postgres/cashier_repo.go`（`UPDATE ... SET ..., checksum=$3`）、`transport/http/cashier/memstore_test.go`（memRepo）。
- #18 改动：`app/cashier/service.go BindProfile` 移除 `checksum==""→CONFLICT` 硬闸（compact #18 PUT/profile 仅列「templateId 存在 + 版本 published」，空 checksum 硬闸超规格），改为原样记录该版本 checksum（schema 默认 ''）。
- 验证：`TestBindProfilePublishedComputesChecksum`（真实发布路径不注入 checksum → 绑定 200 且 snapshotChecksum 非空）PASS；#17 既有发布相关用例（cashier_http_test.go）全 PASS（无用例断言 checksum 为空）。

### 非阻断4 — SavePriceOverrides 未预检重复唯一键
- 根因：未对 items 内 `(country,region,currency,priceId)` 去重，连库命中 `gcpo_key` UNIQUE → 409 CONFLICT，与 compact 期望校验失败=VALIDATION_FAILED 不符。
- 改动：`app/cashier/service.go SavePriceOverrides` 归一化后用 `seen` 集合预检重复键，命中即 `validationErr("...唯一键重复...", duplicate_key)` → 400 VALIDATION_FAILED。
- 测试同步：`TestSavePriceOverridesDuplicateKeyPrevalidated`（期望 400 VALIDATION_FAILED）；scenario `game-cashier.yaml` `save_overrides_duplicate_key_prevalidated`（409→400 VALIDATION_FAILED）。

### 自检结果
- 后端：`go build ./...` ✅；`go vet ./internal/app/cashier/... ./internal/domain/cashier/... ./internal/transport/http/cashier/... ./internal/infra/persistence/postgres/...` ✅；`go test ./...` 全绿（0 失败，含 cashier domain/L3/scenario 解析）。
- 前端：`vitest run GameCashierTab.spec.ts` 11/11 ✅；`vue-tsc --noEmit` 本模块 0 错误，仅余既有非本模块错误（`ChannelLoginConfigPanel.vue` TS7006/TS2345、`sync-section-drawer.spec.ts` TS2322）定向规避。
- 连库 S1/S6/S10 与真实前后端 e2e：本会话无 PG，未执行，留待 🟪 复测轮。

---

## [integration-test] 🟪 game-cashier 集成/系统测试（Cursor Auto）— 第 2 轮（复测 🟧 修复）

### 复测范围 / 输入
- 复测 🟧 第 1 轮 4 项修复（taxRate 统一 string、#17 发布计算 checksum、#18 移除空 checksum 硬闸、SavePriceOverrides 重复键预检）。
- 实际执行环境：本会话 shell 初始仍受沙箱 helper 故障（cwd 卡死于已删除 `/tmp/taxrate_check` → spawn ENOENT），经 `working_directory`/显式 `cd` 重置后恢复；前端 e2e 需 `required_permissions:["all"]` 出沙箱方能启动浏览器（沙箱内 `kill EPERM`/`SIGABRT` 致浏览器无法 launch）。

### 1. 契约对账复核（taxRate 漂移是否清除）
| 面 | 现状 | 结论 |
| --- | --- | --- |
| PUT /price-overrides 请求体 | 前端 `normalizeOverrideRow` 出参 `taxRate: String(row.taxRate)`（DTO `taxRate: string`）；后端 `savePriceOverridesRequest.Items[].TaxRate string` | ✅ 一致（number→string 漂移已消除） |
| GET /price-overrides 响应 | 后端 `gamePriceOverrideResponse.TaxRate string`（`tax_rate::text`）；前端 DTO `taxRate: string`，`fromMinorToEditable` 用 `Number()` 还原供 el-input-number | ✅ 一致（类型标注漂移已消除，读路径正常） |
| scenario yaml | 全部 `taxRate:"0.1"` 字符串 | ✅ 与前后端一致 |
- 其余 3 接口（GET/PUT profile、GET overrides）方法/路径/DTO/错误码/包络仍一致。**不再存在 number/string 漂移。**

### 2. 修复复现验证（实际执行）
- **阻断2（checksum 可达）**：`TestBindProfilePublishedComputesChecksum` PASS——走真实 API 发布路径（不白盒注入），`PublishVersion` 经 `computeVersionChecksum`→`domaincashier.ComputeVersionChecksum` 计算并写入版本 checksum；BindProfile 返回 **200 且 snapshotChecksum 非空**。绑定主流程生产 API 路径**已可达**。
  - checksum 函数复核（cashier.go:63-92）：按唯一键排序 + 纳入全部业务字段（keys + preTax/taxRate/tax/afterTax + effectiveAt，排除 DB id/时间戳）+ sha256 hex → **确定性、内容敏感**，满足快照/跨 schema drift 比对语义。FX approve 路径（approveRunInTx）同样计算 checksum。
- **非阻断4（重复键）**：`TestSavePriceOverridesDuplicateKeyPrevalidated` PASS——items 内重复 `(country,region,currency,priceId)` 经 `seen` 集合预检 → **400 VALIDATION_FAILED**（不再 409）；预检在 InTx 内，触发即整体回滚。

### 3. 全量回归（实际执行）
- 后端：`cd services/admin-api && go build ./... && go vet ./... && go test ./... -count=1` → **全绿（0 失败，无回归）**。
- 定向：`go test ./internal/transport/http/cashier/... ./internal/domain/cashier/... -run 'Game|BindProfile|SavePriceOverrides|Checksum|Duplicate'` → game-cashier L3 全 PASS（含 2 个回归用例 `...ComputesChecksum`、`...DuplicateKeyPrevalidated`）。
- 场景矩阵：`go test ./internal/testkit/scenario/ -run TestScenarioManifests/game-cashier` → 解析校验 PASS；S2(requiresDB:false) 真跑 PASS，requiresDB:true SKIP（无 PG）。yaml 已同步（duplicate→VALIDATION_FAILED、taxRate string）。
- 前端：`pnpm exec vitest run GameCashierTab.spec + CashierView.spec + GameDetailView.spec` → **3 files / 20 passed**。
- 前端 tsc：`vue-tsc --noEmit` 全仓 5 条错误**全部既有非本模块**（`ChannelLoginConfigPanel.vue` TS7006/TS2345×4、`sync-section-drawer.spec.ts` TS2322×1）；**本模块文件 0 错误**（定向 rg `cashier/game|api/modules/cashier|GameDetailView` 无命中）。
- 前端 e2e：`pnpm exec playwright test game-cashier.spec.ts --project=chromium --workers=1`（出沙箱）→ **8 passed (5.1m)**（含视觉基线 `game-cashier-tab.png`）。沙箱内首跑因浏览器 launch 被拦（kill EPERM/SIGABRT）8 failed 系**环境假阴性**，已用沙箱外重跑确认全绿。

### 4. 红线端到端核验（保持）
- 脱敏 S8 N/A（无密文字段）；权限 read/write（`TestGameCashierForbidden` PASS）；env schema 隔离（业务表无前缀/无 env 谓词、平台表只读）；事务回滚（SavePriceOverrides InTx，归一化/重复键失败整体回滚，`TestSavePriceOverridesTransactionRollback` PASS）；与 payment/IAP 路由隔离；本模块无 production Sync 入口。均 ✅。

### 5. 下游契约抽查（payment/snapshot/sync）
- snapshot/sync：版本 checksum 现为**非空且确定性** → `snapshot_checksum` 快照有意义，cashier section 跨 schema drift/baseline 比对恢复有效（**第 1 轮该连带隐患已随阻断2修复消除**）。
- payment：无新增契约/漂移。

### 6. 复测轮次结论
- 第 2 轮：**4 项问题（阻断1/阻断2/非阻断3/非阻断4）全部清除**，无回归、无新增缺陷。
- 残留（非阻断·环境）：连库维度 S1/S6/S10（`expect.db` 落库断言）与真实「后端+PG+前端」跨栈 e2e 因本机无 docker/PG **未执行**，由进程内 L3（内存仓储 + 真实 JWT/审计 spy + InTx 真回滚）+ 契约 mock e2e 等价覆盖；连库维度待集成 Chat / 带 PG 的 CI 补跑。
- 残留（非阻断·测试可选）：`ComputeVersionChecksum` 无 domain 级确定性单测（现由 L3 非空断言间接覆盖）；建议后续补一条纯函数稳定性用例（test-only，不阻断）。
- 通过判定：**可进入 ✅功能验收**（连库维度作为环境性残留，标注待 CI/集成补跑，不阻断验收）。

---

## [acceptance] ✅功能验收（2026-06-30，模型 Cursor Auto）

> 验收基准：功能端到端可用 + 满足 compact 业务规则 + 符合 operation-flow 操作主线（功能成立而非"代码写了"）。逐条 PASS/FAIL + 证据。

### 0. 真实执行命令与输出
- 后端统一回归入口：`WITH_DB=0 sh scripts/regression/backend.sh` → **backend tests PASS**；`scripts/regression/summarize.sh` 汇总 `tests/reports/summary.md`：**后端 go test pass=591 / fail=0**。
- 后端定向：`go build ./... && go vet ./... && go test ./... -count=1` → 全绿（0 失败）。
- 后端 L3：`go test ./internal/transport/http/cashier/... -run 'Game|BindProfile|SavePriceOverrides|ListPriceOverrides|GetProfile' -v` → **19/19 PASS**（含 `TestBindProfilePublishedComputesChecksum`、`TestSavePriceOverridesTransactionRollback`、`TestSavePriceOverridesDuplicateKeyPrevalidated`）。
- 后端 domain+scenario：`go test ./internal/domain/cashier/... ./internal/testkit/scenario/ -run 'Normalize|Overlay|...|ScenarioManifests/game-cashier'` → PASS；scenario 28-case 解析 PASS，requiresDB:false 真跑 PASS，requiresDB:true SKIP（无 PG）。
- 前端类型：`pnpm exec vue-tsc --noEmit` → 全仓仅 5 条**既有非本模块**错误（`ChannelLoginConfigPanel.vue`×4、`sync-section-drawer.spec.ts`×1）；本模块文件 0 错误。
- 前端组件：`pnpm exec vitest run GameCashierTab.spec + CashierView.spec + GameDetailView.spec` → **3 files / 20 passed**。
- 前端 e2e（出沙箱 required_permissions=all）：`pnpm exec playwright test game-cashier.spec.ts --project=chromium --workers=1` → **8 passed (5.1m)**，含视觉基线。

### 1. 验收清单（编号 | 验收点 | 期望 | 实际 | 证据 | 判定）
| # | 验收点 | 期望 | 实际 | 证据 | 判定 |
| --- | --- | --- | --- | --- | --- |
| A1 | GET /profile | cashier.read；`{templateId,appliedTemplateVersion,snapshotChecksum,appliedAt}`；未绑定→404 NOT_FOUND | 一致 | router.go:32；handler.go:401-408,105-110；`TestGetProfileUnboundNotFound`/`TestBindProfileSuccessAndAudit` | PASS |
| A2 | PUT /profile | cashier.write；templateId+templateVersion；仅 published；审计 cashier.profile.bind | 一致 | router.go:33；service.go:409-468；`TestBindProfileSuccessAndAudit/UpgradeVersion/DraftRejected` | PASS |
| A3 | GET /price-overrides | cashier.read；`{items[]}` | 一致 | router.go:34；handler.go:433-444；`TestListPriceOverridesEmpty` | PASS |
| A4 | PUT /price-overrides | cashier.write；整体替换；金额归一化；审计 cashier.override.update | 一致 | router.go:35；service.go:479-528；`TestSavePriceOverridesSuccessAndAudit`（含空替换清空） | PASS |
| A5 | 权限/鉴权码 | 缺令牌→401 UNAUTHENTICATED；缺 perm→403 FORBIDDEN | 一致 | `TestGameCashierRequiresAuth`/`TestGameCashierForbidden` | PASS |
| A6 | 错误码/包络 | VALIDATION_FAILED/CURRENCY_NOT_SUPPORTED/NOT_FOUND/CONFLICT + 统一包络 | 一致 | ports.go:22-51；httpx.WriteData/WriteError；L3 各错误码用例 | PASS |
| B1 | 绑定版本快照 | 必须 published，记录 snapshot_checksum，非实时跟随，显式升级 | 一致 | service.go:436-449（published 校验+记录 checksum）；`TestBindProfileUpgradeVersion`（显式升级才更新） | PASS |
| B2 | checksum 非空+确定性 | 发布计算确定性 checksum，绑定原样快照 | 一致 | cashier.go:63-92（排序+业务字段+sha256）；service.go:257-261,563-567；`TestBindProfilePublishedComputesChecksum` | PASS |
| B3 | 整行覆盖语义 | 覆盖>模板快照同键，整行替换不深合并 | 一致 | game_cashier.go:77-106 `OverlayTemplateRows`；domain 单测；前端 displayRows merge | PASS |
| B4 | 金额归一化 | 受 currency_specs 精度/下限约束 | 一致 | game_cashier.go:44-75 `NormalizePriceOverride`；`TestSavePriceOverridesBelowMinimum`/`CurrencyNotSupported` | PASS |
| B5 | 全量替换+事务回滚 | DELETE+INSERT 单事务，任一失败整体回滚不部分写 | 一致 | service.go:485-527 InTx；**进程内** `TestSavePriceOverridesTransactionRollback` PASS | PASS |
| B6 | 重复键预检 | items 内同键→400 VALIDATION_FAILED | 一致 | service.go:502-511；`TestSavePriceOverridesDuplicateKeyPrevalidated` | PASS |
| F1 | Tab 展示绑定模板+版本+时间+校验和 | 已绑定显示四项 | 一致 | GameCashierTab.vue:19-27；e2e #1 | PASS |
| F2 | 切换/升级 published | 仅可选 published 版本 | 一致 | GameCashierTab.vue:48-69,275；e2e #4（触发 PUT profile） | PASS |
| F3 | 覆盖编辑+currency_specs 约束+舍入预览 | 编辑行+精度/下限校验+minor 预览 | 一致 | GameCashierTab.vue:106-197,414-466；e2e #3/#5 | PASS |
| F4 | 矩阵 vs 覆盖边界高亮 | 区分模板矩阵/游戏覆盖，覆盖行高亮 | 一致 | GameCashierTab.vue:73-104,468-470,643-645；e2e #2 | PASS |
| F5 | 无 cashier.write 置灰 | 写入口禁用 | 一致 | GameCashierTab.vue:3-8,39,49,60,109-117；e2e #7 | PASS |
| F6 | 空/错/未绑定态 | 404→空态，加载错误提示 | 一致 | cashier.ts:284-298（404→null）；GameCashierTab.vue:9,29-34,100-103；e2e #6 | PASS |
| R1 | env schema 隔离 | 业务表无 env 列/SQL 无 schema 前缀；平台表 platform. 前缀只读 | 一致 | 000012 迁移：业务表无前缀、唯一键不前置 env、跨 schema FK 用 platform. | PASS |
| R2 | 事务回滚 | 同 B5 | 一致 | 见 B5 | PASS |
| R3 | 不涉及 payment/IAP | 无 payment/product 耦合 | 一致 | rg payment/product/iap 于 game_cashier.go 无命中 | PASS |
| R4 | 审计事件 | cashier.profile.bind / cashier.override.update | 一致 | service.go:460-465,521-524；L3 audit spy 断言 | PASS |
| O1 | operation-flow 步骤7 | 前置 A8 published 模板；产出 profiles+overrides；完成判定「已绑定有效版本」→下一步 8 支付路由 | 一致 | 绑定主流程真实可达（B1/B2）；与 02-operation-flow.md §B.7 对齐 | PASS |
| D1 | 下游 snapshot/sync | cashier section drift 比对依赖非空确定性 checksum，无破坏 | 一致 | sync.go:17 `SectionCashier`；checksum 非空确定性；`go test ./...` 全绿 591/0 | PASS |
| D2 | 下游 payment | 无契约漂移；#17 对外 API 形状未改 | 一致 | manifest cross_module_changes 仅补 checksum 字段；全量测试全绿 | PASS |

### 2. 构建/测试结果汇总
- 后端：unified `backend.sh` PASS（pass=591/fail=0）；build/vet/test ./... 全绿；L3 19/19；domain+scenario PASS。
- 前端：vitest 20 passed；vue-tsc 本模块 0 错误（5 条既有非本模块）；e2e 8 passed（出沙箱，含视觉基线）。

### 3. 结论
- **验收通过**。25 项验收点全部 PASS（A6+B6+F6+R4+O1+D2），无 FAIL。

### 4. 遗留风险与建议（非阻断）
- 【环境性】连库 S1/S6/S10（`expect.db` 落库断言）+ 真实 PG「后端+PG+前端」跨栈 e2e 因本机无 docker/PG 未执行；由进程内 L3（含真 InTx 回滚）+ 契约 mock e2e 等价覆盖。沿用 🟪 第 2 轮结论，**标注环境性残留，待带 PG 的 CI / 集成 Chat 补跑**（统一入口 `WITH_DB=1 scripts/regression/run.sh`）。
- 【测试可选】`ComputeVersionChecksum` 无 domain 级确定性纯函数单测（现由 L3 非空断言间接覆盖），建议补一条 test-only 稳定性用例。
- 【体验·既有】`loadingProfile` 未挂 v-loading（矩阵/覆盖表已 v-loading、绑定按钮有 loading 态），非阻断。
