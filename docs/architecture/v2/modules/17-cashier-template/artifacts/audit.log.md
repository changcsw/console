# cashier-template · audit.log（完整执行日志，仅供人类审计）

> 模块：17 cashier-template ｜ lane：cashier-surface ｜ worktree：../console-cashier-template（branch codex/cashier-template）
> 本文件由各角色追加；总负责 Agent 不读本文件。

## [orchestrator] 启动编排
- 闸门检查：depends_on=common ✅；cashier-surface 无在制模块；允许开工。
- 创建 worktree ../console-cashier-template（branch codex/cashier-template，base main@88a377c）。
- 注意：worktree 在默认沙箱写边界外，子 agent 的 shell/构建/测试命令需使用提权（required_permissions: all）。
- 初始化 artifacts 目录与进度账本（cashier-template → 🔄 在制）。

## [orchestrator] 流水线收尾（全流程 ✅）
- 后端车道 ✅：开发(go build/vet ✅) → CR(通过, 修 5 处 DTO/权限/错误码) → 测试(单元 8 + 接口 22 全绿)。
- 前端车道 ✅：开发(vite build ✅) → CR(通过, 修 3 处) → 测试(vitest 39/39 + Playwright 12/12)。
- 集成回路：🟪 第 1 轮揪出 P1/P2/P3 阻断 + P4 缺陷 → 🟧 Opus 修复 P1–P5 → 🟪 第 2 轮复测全部解决、回归全绿 → 判定可验收。
- ✅ 功能验收：29/29 PASS；统一回归入口 backend.sh ✅、vite build ✅、vitest 全量 121/122（唯一失败为既有非本模块 games/sync-section-drawer，不计入）。
- 遗留非阻断·跨模块：审计 sink 生产 nil（待 audit 模块 22 统一注入，同 game/channel 模式）；FX provider 占位未接真实算价；version 对外数值 / DB VARCHAR 与 compact「字符串」兼容。
- 账本 #17 全 ✅；#18 game-cashier 阻塞解除（cashier-template + game 均完成）。
- 共享集成点：admin_wiring.go / routes.ts 由各车道按需接入，详见 integration.checklist.md，留待集成 Agent 统一整合。

## [backend] cashier-template 实现（Go）
- 依据 `spec.compact.md` 落地：迁移 `000007_cashier_template_platform_schema.up.sql`（platform 归位、CHECK/FK/唯一 published 索引、必要索引）。
- domain：扩展 `TemplateVersion`（`SourceType`、`CanTransition`），补 `command.BuildDraftFromTemplateVersion` 来源标记规则。
- common：新增金额归一化工具 `internal/domain/common/currency.go`（decimal/rounding/min 校验）。
- app：新增 `internal/app/cashier`（模板/版本/行 upsert+归一化/发布自动归档/FX run+approve 编排/审计钩子）。
- infra：新增 `CashierStore/CashierRepo`（templates/versions/rows/runs/currency_specs 窄仓储）与 `infra/fx/provider.go` 抽象。
- transport：新增 `internal/transport/http/cashier/handler.go` + `router.go`，覆盖 compact 端点全集，统一包络与错误码映射。
- wiring：在 `internal/transport/httpserver/admin_wiring.go` 接入 cashier 路由和 service（共享 admin 鉴权链）。

### 自检命令与结果
- `cd /Users/csw/gitproject/console-cashier-template/services/admin-api && gofmt -w ...` ✅
- `cd /Users/csw/gitproject/console-cashier-template/services/admin-api && go build ./...` ✅
- `cd /Users/csw/gitproject/console-cashier-template/services/admin-api && go vet ./...` ✅
- `cd /Users/csw/gitproject/console-cashier-template/services/admin-api/migrations && ls`（确认新增迁移文件命名顺序）✅

## [frontend] 实现 cashier-template 前端模块
- 读取文档：`index.json`、`00-common.md`、`01-structure.md`、`CONVENTIONS.md`、`modules/17-cashier-template/spec.compact.md`，并补读 `modules/17-cashier-template/README.md`（确认 FX ignore 交互要求）。
- 参考风格：`views/games` / `views/channels` 的 PageCard、EnvironmentBadge、权限指令 `v-perm`、错误与空态处理。
- 新增 API 客户端：`apps/admin-web/src/api/modules/cashier.ts`，覆盖 compact 约定的模板/版本/矩阵/发布/FX 触发与审核端点。
- 重构页面：`apps/admin-web/src/views/cashier/CashierView.vue` 改为模板列表 + 详情工作区；接入版本列表、价格矩阵编辑器、汇率同步审核。
- 组件实现：
  - `templates/TemplateVersionsTab.vue`：版本状态展示、draft 发布、published 复制入口；
  - `templates/components/CopyPublishedToDraftDialog.vue`：复制 published 为 draft；
  - `templates/components/CreateTemplateDialog.vue`、`CreateVersionDialog.vue`：模板与版本创建；
  - `templates/PriceMatrixEditor.vue`：金额输入按 currency_specs 做精度/下限/舍入预览；published 只读禁用；
  - `templates/FxSyncRunsReviewList.vue`：FX run 差异摘要与 approve/ignore 操作。
- 路由更新：`apps/admin-web/src/router/routes.ts` 中 cashier 菜单改为顶层「收银台」，并新增 `perm: "cashier.read"`。
- 清理旧文件：删除 `apps/admin-web/src/api/templateVersions.ts`（由新模块 API 取代）。

## [frontend] 自检记录（required_permissions: all）
- 命令：`pnpm --dir apps/admin-web exec vue-tsc --noEmit`
  - 结果：❌ 失败（仓库既有问题，非本模块引入）
  - 主要报错：缺失 `@vue/test-utils` 类型、已有测试文件的历史 TS 报错（如 `games` 测试中的隐式 any）。
- 命令：`pnpm --dir apps/admin-web exec vite build`
  - 结果：✅ 通过（仅有 rollup chunk size warning，未阻断构建）。

## [frontend] 契约偏差/未决
- compact 仅明确 `POST /fx-sync-runs/{runId}/approve`，但前端需求需要 `ignore` 操作；当前实现为向同一 approve 端点传 `action: "ignore"`（待集成阶段与后端 DTO 对账确认）。
- compact 未单列 FX run 查询接口；当前优先消费 `GET /templates/{templateId}` 返回的 `fxSyncRuns`，如后端改为独立列表端点需在集成阶段对齐。

## [frontend-cr] cashier-template 前端 Code Review
- 依据：`spec.compact.md` 前端要点、`CONVENTIONS.md`、`01-structure.md` §5、handoff.summary.md、worktree diff。
- 结论：**通过**（compact 前端要点已覆盖；后端联调对账项保留在 integration.checklist）。
- 核对：模板列表/详情/版本/矩阵/FX 审核页面与组件齐全；API client 路径与 camelCase DTO 对齐 compact；权限 `cashier.read/write/publish`、`fx.approve` 已挂 `v-perm`；空/错态与 `EnvironmentBadge`、统一包络消费正确。
- CR 直接修复：
  1. `PriceMatrixEditor.vue`：`archived` 版本只读（原仅 `published`）。
  2. `PriceMatrixEditor.vue`：`taxRate` 字符串响应兼容解析。
  3. `cashier.ts`：`getCashierTemplate` 增加 `{ template, versions }` 嵌套响应与 `Version`/`version` 字段归一化。
- 集成对账项（不打回）：ignore `{ action: "ignore" }` 假设；`fxSyncRuns` 详情内嵌来源；后端 PUT rows 仍收 `preTaxAmount` 字符串 vs 前端 minor 字段；`sourceVersion` int/string；后端 ignore 未实现（ignore 按钮联调前有风险）。
- 建议（非阻断）：archived 行可补「复制为 draft」入口；`currency_specs` 后续接字典 API。
- 自检：`pnpm --dir apps/admin-web exec vite build` ✅；`vue-tsc --noEmit` ❌（仓库既有测试类型问题，非本模块引入）。

## [frontend-test] cashier-template 前端测试
- 输入：frontend + frontend-cr handoff（CR 通过；已修复 archived 只读 / taxRate 字符串解析 / getCashierTemplate 嵌套归一化）。
- 读文档：`03-testing.md`（L4 vitest 组件 + L5 Playwright 截图/视觉基线 + 目录约定）、`spec.compact.md` 前端要点。
- 测试基线设施修复（阻断项）：`apps/admin-web/package.json` 此前仅有 `@testing-library/vue`，缺失其 peer `@vue/test-utils`（仅作为传递依赖，pnpm 不在顶层暴露），导致 `src/test/setup.ts` 的 `import { config } from "@vue/test-utils"` 无法解析、**整个 vitest 套件无法启动**（既有 games/channels 等用例同样受影响）。已显式 `pnpm add -D @vue/test-utils@^2.4.11`（锁定 lock 既有 2.4.11，无新增解析）。属测试基础设施修复，非 cashier 业务改动。

### vitest 组件用例（就近 `__tests__`，L4）
- `views/cashier/templates/__tests__/PriceMatrixEditor.spec.ts`（12）：USD half_up 归一化预览(999/100/1099)、JPY 小数位超限报错、低于 minAmountMinor 报错、CURRENCY_NOT_SUPPORTED、floor/ceil/truncate/half_up 舍入、published & archived 只读（CR 修复点回归）、draft 无 cashier.write 只读、taxRate 字符串解析（CR 修复点回归）、空版本不请求、非法行阻断 PUT、合法行下发 minor 字段并 emit saved。
- `views/cashier/templates/__tests__/TemplateVersionsTab.spec.ts`（6）：状态标签映射(draft/published/archived)、发布时间占位、行点击 emit select-version、新建 draft 工具栏按钮权限置灰+emit、空态文案。
- `views/cashier/templates/__tests__/FxSyncRunsReviewList.spec.ts`（8）：差异摘要 JSON 化、状态标签映射、仅 pending_review 可审核、review(approve) 下发 action=approve + emit refresh、review(ignore) 带 reviewNote、触发同步工具栏按钮权限置灰、触发同步调用接口 + emit refresh、空态文案。
- `views/cashier/__tests__/CashierView.spec.ts`（6）：初始加载列表+自动选中首行加载详情、查询关键字下发、默认优先选中 draft、发布版本调用接口并刷新、空态、无 cashier.write 新建模板置灰。
- `views/cashier/templates/components/__tests__/CashierDialogs.spec.ts`（7）：CreateTemplate 必填校验/创建 emit/CONFLICT 不 emit；CreateVersion copy 缺 sourceVersion 阻断/manual 下发 undefined/copy_published 下发来源；CopyPublishedToDraft 确认复制调用接口+emit。
- 说明：el-table **行内单元格 scoped slot 在 jsdom 不渲染**，行内 approve/ignore/复制为 draft/发布 按钮与差异单元格的禁用/可视化改由 Playwright（真实浏览器）覆盖；组件级聚焦纯函数/计算属性/工具栏按钮/空态。

### Playwright UI 用例（L5，契约 mock + 截图基线）
- `tests/frontend/e2e/cashier.spec.ts`（12）：对 `/api/admin/cashier/**` 单处理器 mock 全端点。覆盖：模板列表行/FX 模式/周期 + 自动选中首行加载详情；版本列表状态 + published「复制为 draft」入口 + draft「发布」；复制对话框打开；发布触发 `POST .../versions/2/publish`（el-popconfirm 弹层 primary 按钮，locale 无关）；价格矩阵 draft 可编辑 + 归一化预览(preTax=999)；FX 差异摘要 + pending_review 可 approve / applied 行禁用；approve 下发 action=approve；无 cashier.write → 新建模板/触发 FX 同步置灰；无 fx.approve → approve/ignore 置灰；空模板列表空态；列表 500 错误提示；视觉基线 `cashier-list.png`。
- 截图：`tests/frontend/screenshots/cashier-{list-detail,price-matrix,fx-review,empty}.png`；视觉基线：`tests/frontend/visual-baseline/cashier.spec.ts-snapshots/`（已 `--update-snapshots` 生成并纳入）。
- 回归挂载：vitest 自动收集 `src/**/*.spec.ts`；Playwright `playwright.config.ts` testDir 自动收 `tests/frontend/e2e/*.spec.ts`——无需改脚本即并入 `scripts/regression/frontend.sh`。

### 自检命令与结果（required_permissions: all，worktree 内）
- `pnpm exec vitest run src/views/cashier` → ✅ 5 文件 / **39 passed**。
- `pnpm exec vitest run`（全量）→ 121 passed / 1 failed；唯一失败 `views/games/detail/__tests__/sync-section-drawer.spec.ts`（断言文案 "Create draft from published v7" 不存在）属**仓库既有 games 用例问题，非 cashier 引入**。
- `pnpm exec playwright test cashier.spec.ts --workers=1 --update-snapshots` → ✅ **12 passed**（首跑 11 passed，修正 `published 版本只读` strict-mode 后单独复跑该用例通过）。
- `pnpm exec vue-tsc --noEmit` → 仓库整体仍 ❌（既有测试类型问题），但**本模块新增的 5 个 spec + e2e 无任何 cashier 相关类型报错**（已过滤确认）。
- 环境注记：本机 vite dev 冷编译 + Element Plus 首屏渲染慢（单页约 20-30s），Playwright 在默认 fullyParallel 下多 worker 抢占单一 dev server 致首编译超 5s/30s 超时；遂在 spec 内将单测超时抬到 120s、详情区等待/截图抬到 30s，并以 `--workers=1` 串行跑通。导航/契约本身无误（已用调试用例确认点击「收银台」后 URL=/cashier、无 pageerror）。

### 疑似实现缺陷（前端测试视角）
- 组件级未发现新的前端实现缺陷；compact 前端要点（精度/下限/舍入预览、published+archived 只读、复制为 draft 入口、draft 发布、FX approve/ignore、权限置灰）均按预期工作。
- 沿用既有 open_issues（归集成对账，不打回前端）：approve payload `{action:"ignore"}`、`fxSyncRuns` 来源、PUT rows minor vs 后端 preTaxAmount、`sourceVersion` 类型、`GET template` 嵌套响应。
- 旁路观察（非本模块，供总负责知悉）：`tests/frontend/e2e/channels.spec.ts` 用例断言按钮名「复制创建」已与现实现「新建渠道实例」不符（既有用例陈旧）；`games` sync-section-drawer 组件用例文案断言陈旧——均非 cashier 范畴。

## [backend-cr] cashier-template 后端 Code Review（Composer 2.5）
- 依据：`spec.compact.md`、`00-common.md`（§2.2/§3.3/§5/§7/§8）、`CONVENTIONS.md`、worktree 全量后端 diff。
- 结论：**通过**（原 2 项阻断已在 CR 阶段直接修复；余下集成项不打回）。

### 契约核对表（compact 要点 → 证据）
| 要点 | 已实现 | 一致 | 证据 |
| --- | --- | --- | --- |
| 4 表 platform schema、CHECK/FK/索引、幂等迁移 | ✅ | ✅ | `migrations/000007_*.up.sql:5-141` |
| 同模板最多一个 published（部分唯一索引 + 应用层同事务归档） | ✅ | ✅ | 迁移 `:139-141`；`service.go:PublishVersion/approveRunInTx` |
| VersionStatus/FX 枚举与默认 | ✅ | ✅ | 迁移 CHECK；`domain/cashier/template_version.go`、`cashier.go` |
| CanTransition 仅 draft→published、published→archived | ✅ | ✅ | `template_version.go:36-40` |
| CopyToDraft/BuildDraftFromTemplateVersion draft+source_type | ✅ | ✅ | `command/copy_template_version.go:12-22`；`service.go:CopyToDraft` |
| 发布同事务归档旧 published | ✅ | ✅ | `service.go:206-240` |
| 金额归一化 currency_specs→minor | ✅ | ✅ | `domain/common/currency.go`；`service.go:normalizeRow` |
| FX manual_confirm approve 即 publish+applied（无单独 apply） | ✅ | ✅ | `service.go:approveRunInTx:325-352` |
| auto_apply 触发后自动 approve | ✅ | ✅ | `service.go:TriggerFXSyncRun:290-292` |
| API 路径/权限码/包络/错误码 | ✅ | ✅ | `transport/http/cashier/router.go`；`app/cashier/ports.go` |
| 审计事件 cashier.template.create/publish、fx.approve | ⚠️ | ⚠️ | 钩子已实现 `service.go:writeAudit`；wiring audit=nil（同 game/channel，待 audit 模块） |
| version 对外 int、DB VARCHAR 数字串 | ✅ | ⚠️可接受 | `cashier_repo.go:scanVersion/NextVersion`；`handler.go:parseVersion` |

### 问题清单
**阻断（已修复）**
1. GetTemplate/CreateVersion/CreateFXRun 直接序列化 domain 记录 → PascalCase 泄漏 → 已加 `versionResponse`/`fxRunResponse`（`handler.go`）。
2. FX ignore 未实现 → 已加 `IgnoreFXSyncRun` + approve `{action:"ignore"}` 分支（`service.go`、`handler.go`）。

**建议（未阻断）**
- audit sink 仍为 nil（`admin_wiring.go:120`），audit 模块落地后统一注入。
- FX `NoopProvider` 仅占位，trigger 仅复制 published 行，未接真实汇率算价（`infra/fx/provider.go`）。
- GET template 未返回 `fxSyncRuns`，前端需独立列表或集成补字段。
- PUT rows 收 `preTaxAmount` 字符串（major），与前端 minor 字段待集成统一。

**CR 直接修复**
- GET rows 权限 `cashier.write` → `cashier.read`（`router.go:25`）。
- CreateVersion 复制时按来源状态写 `copy_published`/`copy_archived` + copy 时校验 sourceVersion（`service.go:CreateVersion`）。
- NormalizeAmountToMinor 失败映射 `VALIDATION_FAILED`（`service.go:normalizeRow`）。

### 自检
- `go build ./...` ✅；`go vet ./...` ✅（CR 修复后复跑）。

## [backend-test] cashier-template 后端测试（Cursor Auto）
- 依据：codegen-workflow §1（index.json → spec.compact.md）、`03-testing.md`（分层 L1/L3、目录约定、S1–S10 维度、scenario manifest 形态、fixtures 约定）、backend + backend-cr handoff。
- 读源：`domain/cashier/{template_version,cashier}.go`、`domain/common/currency.go`、`app/command/copy_template_version.go`、`app/cashier/{service,ports}.go`、`transport/http/cashier/{handler,router}.go`、`migrations/000007`、既有 games L3 httptest 范式与 scenario harness（`testkit/scenario`）。

### 产出文件
- L1 单元（纯函数，无 IO）：
  - `internal/domain/cashier/template_version_test.go`（扩展）：`CopyToDraft`（恒 draft + 保留 templateId/sourceType）、`CanTransition` 全量矩阵（仅 draft→published、published→archived，其余含同态/反向/跳态/非法枚举一律拒绝）。
  - `internal/domain/common/currency_test.go`（新增）：`NormalizeAmountToMinor` 精度 + 4 种舍入（half_up/floor/ceil/truncate，含正负）、0 位小数币种、下限（below/at minimum）、非法输入（空/空白/非数字/非法 rounding mode）。
  - `internal/app/command/copy_template_version_test.go`（新增）：`BuildDraftFromTemplateVersion` 产物恒 draft + nextVersion + source_type 按来源状态映射（published→copy_published / archived→copy_archived / draft→manual）。
  - `internal/app/cashier/calc_test.go`（新增）：`calcTaxAmount` 半进位舍入（正/负/边界/分数税率/0/非法）。
- L3 接口（进程内 httptest 全链路 transport→app→domain + 内存仓储 + 真实 JWT + 审计 spy）：
  - `internal/transport/http/cashier/memstore_test.go`（新增）：实现 `cashierapp.TxManager`/`CashierTemplateRepository` 内存仓储，InTx 克隆/回填真实回滚语义；seed currency_specs(USD/JPY)；`forcePublishErr` 故障注入用于 S10。
  - `internal/transport/http/cashier/cashier_http_test.go`（新增，22 个 Test）：S1/S2/S3/S4/S5/S7/S9/S10 + 模块私有维度（状态机流转、copy-to-draft 产物 draft/source_type/复制行、金额归一化与 CURRENCY_NOT_SUPPORTED/下限、published 只读 VERSION_STATE_INVALID、发布同事务归档旧 published、FX manual_confirm approve→publish+applied、auto_apply 自动发布、ignore、发布/审核同事务回滚）。
- 场景矩阵 manifest：`tests/backend/scenarios/cashier-template.yaml`（10 接口 × S1–S10，逐用例标维度/auth/fixture/note；S2 用例 requiresDB=false 进程内真实跑 401，其余 requiresDB=true 由连库 harness 执行，进程内仅解析校验，等价覆盖已由 transport httptest 承担）。
- fixtures：`tests/fixtures/common/cashier-template.sql`（cashier_admin/cashier_reader RBAC + global_default/auto_tpl 模板 + 版本/价格行；幂等 ON CONFLICT；片段差异化状态由连库 harness 拼装）。

### 测试使能的共享改动
- `internal/transport/httpserver/admin_wiring.go`：降级路由新增 `cashierhttp.RegisterRoutes(...,ready=false)`（与 games/channels 一致），使 cashier 受保护路由在无 DB 时仍先过 Authn → S2 契约用例可进程内真实执行。
- `internal/transport/httpserver/router_test.go`：`TestTemplateVersionCopyRouteIsRegistered` 随之更新——copy-to-draft 现由真实 cashier 路由接管，无令牌 → 401 UNAUTHENTICATED（不再走旧 scaffold 免鉴权 201）。

### 自检命令与结果（worktree 内提权运行）
- `go test ./internal/domain/cashier/... ./internal/domain/common/... ./internal/app/command/... ./internal/app/cashier/...` ✅（L1 全通过）。
- `go test ./internal/transport/http/cashier/...` ✅（22 个 L3 全通过）。
- `go test ./internal/testkit/scenario/... -run TestScenarioManifests` ✅（cashier-template.yaml 解析通过；S2 用例真实执行 401，requiresDB 用例正确 SKIP）。
- `go test ./...`（全后端，回归入口等价） ✅ exit 0；`go vet ./...` ✅；`gofmt -l` ✅（无差异）。
- 运行结果汇总：通过 **0 失败**；新增 L1 8 个 Test（含数十子用例）+ L3 22 个 Test 全绿；scenario 56 用例（11 个 S2 真实跑、45 个 requiresDB SKIP-解析通过）。

### 疑似实现缺陷（非阻断，回退 🟦后端开发经总负责调度）
1. **auto_apply 触发响应体 status 滞后**：`Service.TriggerFXSyncRun` 在 `auto_apply` 路径下事务内调用 `approveRunInTx` 将 run 置 applied，但返回给 handler 的 `run` 变量仍是 `CreateFXSyncRun` 时的 `pending_review`（未在事务内 reload）。落库效果正确（候选已发布、旧 published 已归档、run=applied），仅 `POST /fx-sync/runs` 响应体 `data.status` 回 `pending_review`，前端据此判断会误判。建议事务结束前 reload run 或返回最终态。测试已以版本状态断言真实效果并规避（`TestFXAutoApplyTriggerPublishes`）。

## [integration-test] cashier-template 集成/系统测试（🟪 测试专家，Cursor Auto）
- 前置闸门：🟦🧪后端测试 ✅、🟩🧪前端测试 ✅（均满足，已读两车道 handoff/audit/checklist）。
- 读文档：codegen-workflow §1（index.json → spec.compact.md）→ 03-testing.md → 02-operation-flow.md（定位本模块端到端位置：平台级价格模板定义方，下游 game-cashier 绑版本快照）。
- 读源（逐项契约对账实证）：前端 `apps/admin-web/src/api/modules/cashier.ts`、`views/cashier/templates/{PriceMatrixEditor,FxSyncRunsReviewList}.vue`；后端 `transport/http/cashier/{handler,router}.go`、`app/cashier/{service,ports}.go`、`domain/cashier/cashier.go`。

### 复测轮次 / 回归结果（worktree 内提权运行，真实输出）
- 后端 `go build ./...` → ✅ exit 0。
- 后端 `go test ./...`（全量回归入口等价）→ ✅ 全部 ok（含 `app/cashier`、`domain/cashier`、`transport/http/cashier`、`testkit/scenario`），0 失败。
- 前端 `pnpm exec vitest run src/views/cashier` → ✅ 5 文件 / 39 passed。
- 前端 `pnpm exec vitest run`（全量）→ 122 中 121 passed / 1 failed；唯一失败 `views/games/detail/__tests__/sync-section-drawer.spec.ts`（"Create draft from published v7" 文案陈旧）= 仓库既有 games 用例，**非 cashier**。
- 跨栈手段说明：本机起真实 PG+服务成本高，采用「后端进程内 httptest 全链路（transport→app→domain，22 个 L3 已覆盖关键路径）+ 前端契约 mock 对齐 + 契约级逐字段对账 + Go json 解码实证」组合，等价覆盖 operation-flow 主线；连库全维度留待统一连库 harness（`SCENARIO_WITH_DB=1`）。

### 契约对账（前端实际调用 vs 后端实际 API，逐项裁决）
| # | 项 | 前端 | 后端 | 裁决 |
| --- | --- | --- | --- | --- |
| 1 | FX ignore payload | POST approve `{action:"ignore",reviewNote}` | handler.go:278 分支 → `IgnoreFXSyncRun`（置 ignored，候选 draft 保留） | ✅ 已对齐（解决） |
| 2 | fxSyncRuns 来源 + run DTO | 读 `GET template` 的 `fxSyncRuns`；DTO 期望 `runId/candidateVersion/diffSummary` | `GetTemplate` 仅回 `{template,versions}`（无 fxSyncRuns）；run DTO 为 `id/candidateVersionIdRef/diffSummaryJson` | ❌ **阻断 P2**：FX 审核列表连真实后端永远空 + 字段全不匹配 |
| 3 | PUT rows 金额字段 | 下发 `preTaxAmountMinor:number`+`taxRate:number`（minor 已算） | 收 `preTaxAmount:string`(major)+`taxRate:string`，后端归一化 | ❌ **阻断 P1**：字段名/类型/语义三重漂移，Go 解码 number→string 直接失败 |
| 4 | sourceVersion / version | `sourceVersion:string`；version 当字符串（normalizeVersion 已 String()兼容） | `createVersionRequest.SourceVersion int`；versionResponse.Version int | sourceVersion ❌ **阻断 P3**（次路径）；version int 对外 ✅可接受（前端兼容） |
| 5 | auto_apply 触发响应 | 据响应 status 判断 | `TriggerFXSyncRun` 未 reload，回 pending_review，落库 applied | ⚠️ **缺陷 P4**：非阻断 manual 主路径，需修 |

### 实证（Go json 解码，证据）
- PUT rows：`json: cannot unmarshal number into Go struct field ...taxRate of type string` → PUT 整体 400 VALIDATION_FAILED（即编辑价格矩阵「保存矩阵」连真实后端必失败）。
- CreateVersion copy：`json: cannot unmarshal string into Go struct field ...sourceVersion of type int` → copy 路径 400。

### 红线端到端核验
- 脱敏：本模块无密文 → **N/A**。
- 权限：`router.go` 已挂 cashier.read（GET templates/template/rows）、cashier.write（POST template/versions/copy-to-draft/PUT rows/触发 FX）、cashier.publish（publish）、fx.approve（approve）；前端 `v-perm` 同码置灰 → ✅一致。
- 跨 env：四表平台级、共享 schema `platform`、不带 env（迁移 000007）→ ✅符合 compact 边界。
- 事务回滚：发布同事务归档旧 published（`service.go:PublishVersion`）、FX approve 同事务（归档+发布候选+run 更新，`approveRunInTx`）→ ✅；后端 L3 已用 `forcePublishErr`(S10) 验证回滚。
- 下游 game-cashier：本 worktree 未含 game-cashier 实现（下游模块）；compact 明确游戏绑「某版本快照」的 price_id 引用；published 只读（UpsertRows 拒非 draft）保证快照行不可变、`cashier_price_rows.price_id` VARCHAR(64) 语义稳定 → ✅（前向兼容 OK，跨模块 e2e 待 game-cashier 落地）。

### 通过判定
- **不可进入 ✅ 验收**。存在 3 个阻断契约漂移（P1 PUT rows、P2 FX runs 来源+DTO、P3 sourceVersion），其中 P1/P2 直接破坏 compact 关键路径（编辑价格矩阵保存、FX 同步审核）端到端，须 🟧 修复后复测。回归基线全绿（除既有非本模块 games 用例）。

### 问题清单（移交 🟧 高级全栈工程师）
- **P1（阻断）PUT rows 金额字段三重漂移**：定位 前端 `api/modules/cashier.ts:37-50,196-201`+`PriceMatrixEditor.vue:289-302`（发 minor+number） vs 后端 `handler.go:38-48`（收 major string）。期望（compact §价格行/00 §5：归一化读 currency_specs→校验→舍入→存 minor，归一化在后端）→ **建议前端改为下发 `preTaxAmount`(major 字符串，直接用现有 `preTaxMajorInput`)+`taxRate`(字符串)**，与后端归一化入口对齐；后端保持现状。
- **P2（阻断）FX runs 来源缺失 + run DTO 不匹配**：定位 后端 `handler.go:150-161 GetTemplate`（无 fxSyncRuns）+`handler.go:64-74 fxRunResponse`（id/candidateVersionIdRef/diffSummaryJson） vs 前端 `cashier.ts:52-61,174`（runId/candidateVersion/diffSummary）。期望（compact §前端要点 FX 审核列表可消费）→ **建议后端 `GET /templates/{id}` 增 `fxSyncRuns:[]`（或新增 `GET /templates/{id}/fx-sync/runs`），run DTO 字段对齐：`runId`=id、`candidateVersion`=候选版本号字符串（注意当前给的是 versions 表 id，须换版本号）、`diffSummary`=diffSummaryJson**。需总负责定契约落点。
- **P3（阻断·次路径）CreateVersion sourceVersion 类型**：定位 后端 `handler.go:33-36`（int） vs 前端 `cashier.ts:88-91`（string）。期望（compact §POST versions：sourceVersion string）→ **建议后端 `createVersionRequest.SourceVersion` 改 string + `strconv.Atoi`**（后端对齐 compact；主复制入口 copy-to-draft 不受影响）。
- **P4（缺陷·非阻断）auto_apply 触发响应 status 滞后**：定位 `service.go:268-321`+`handler.go:258-265`。期望 响应反映最终态 → **建议事务结束前 `repo.GetFXSyncRun(run.ID)` reload 或返回最终态**。manual_confirm 不触发，不阻断主验收。
- **P5（一致性·非阻断·可选）**：approve 响应 `{status}` 与前端 `approveFxSyncRun` 标注的 `FxSyncRun` 不一致（仅触发 refresh，可接受）；publish 响应 `{status:"published"}` 无 version（前端仅 refresh，可接受）；`GET template.template` 内嵌 internal `id`（建议后端去除）。

### 自检命令与结果
- `cd services/admin-api && go build ./...` ✅；`go test ./...` ✅。
- `cd apps/admin-web && pnpm exec vitest run src/views/cashier` ✅ 39/39；`pnpm exec vitest run` 121/122（1 既有非本模块失败）。
- 契约漂移 Go json 解码实证：PUT rows / CreateVersion copy 均如期解码失败（见上「实证」）。

---

## 🟧 高级全栈工程师 · 集成修复（Opus 4.8 High）

> 输入：🟪 集成测试问题清单（P1–P5）。裁决标准：spec.compact.md（前后端两侧 + §价格行 / §FX）+ 00-common §5 金额归一化 / §7 包络。
> 修复策略：契约以 compact 为唯一标准，归一化职责留后端；不扩大改动范围；调整契约处同步更新单测/组件测试/场景 manifest（不弱化断言）。

### P1（阻断）PUT rows 金额字段三重漂移 —— 已修复
- 根因：前端 PUT 下发 `preTaxAmountMinor:number`+`taxRate:number`，后端 `upsertRowsRequest` 收 `preTaxAmount:string(major)`+`taxRate:string`；Go json 解码 number→string 直接失败，整体 400。归一化职责（00 §5）在后端。
- 裁决/改动：前端改为下发 major 字符串 + taxRate 字符串，后端保持归一化入口不变。
  - `apps/admin-web/src/api/modules/cashier.ts`：新增 `PutPriceRow{ countryCode,regionCode,currency,priceId,preTaxAmount(string),taxRate(string),effectiveAt }`；`PutRowsPayload.rows` 改用 `PutPriceRow[]`。
  - `apps/admin-web/src/views/cashier/templates/PriceMatrixEditor.vue:saveRows`：仍用 `normalizeRow` 做 currency_specs 预校验 + 舍入预览（前端预览保留），但下发 `preTaxAmount=preTaxMajorInput.trim()`、`taxRate=String(row.taxRate)`，不再下发 `*_minor`。published/archived 只读语义（`readonlyByStatus`/`readonly`）未变。
- 验证：`PriceMatrixEditor.spec.ts` 保存用例改断言新契约（preTaxAmount/taxRate 字符串 + 断言无 `preTaxAmountMinor`）；vitest cashier 39/39 ✅。

### P2（阻断）FX runs 来源缺失 + run DTO 不匹配 —— 已修复
- 根因：后端 `GetTemplate` 仅回 `{template,versions}` 无 fxSyncRuns；`fxRunResponse` 用 `id/candidateVersionIdRef/diffSummaryJson`，前端要 `runId/candidateVersion(版本号)/diffSummary`。
- 裁决/改动：后端在 GET template 内嵌 `fxSyncRuns` 并对齐 camelCase DTO，候选版本给对外版本号（非 versions 表内部 id）。
  - `internal/app/cashier/ports.go`：新增 `FXSyncRunView{ Run, CandidateVersion int }`；仓储接口加 `ListFXSyncRuns(ctx, templateIDRef) ([]FXSyncRun, error)`。
  - `internal/app/cashier/service.go`：新增 `ListFXSyncRuns(templateID)`（解析模板→列 run→按 `GetVersionByID` 回填候选版本号）；`TriggerFXSyncRun` 返回 `FXSyncRunView`（候选版本号取 `candidate.Version`）。
  - `internal/transport/http/cashier/handler.go`：`fxRunResponse` 改 `runId/candidateVersion(string)/diffSummary/status/triggeredAt/reviewedBy/reviewedAt/reviewNote`；`toFXRunResponse(view)`+`mapFXRuns`；`GetTemplate` 增 `fxSyncRuns: mapFXRuns(...)`；`CreateFXRun` 用 view。
  - 仓储实现：`internal/infra/persistence/postgres/cashier_repo.go` 加 `ListFXSyncRuns`（按 template_id_ref，triggered_at DESC）；`memstore_test.go` 加同名内存实现。
- 验证：前端 `getCashierTemplate` 读 `raw.fxSyncRuns`、`FxSyncRunsReviewList` 用 runId/candidateVersion/diffSummary（已对齐，无需改）；e2e mock 早已是新契约。后端 L3 用例改读 `runId`。

### P3（阻断·次路径）CreateVersion sourceVersion 类型 —— 已修复
- 根因：后端 `createVersionRequest.SourceVersion int` vs 前端/compact `string`；copy 路径下发 "7" 解码失败 400。
- 改动：`handler.go` `SourceVersion` 改 `string`，handler 内 `strings.TrimSpace`+`strconv.Atoi`（空=0；非数字/≤0 → 400 VALIDATION_FAILED），再传 `CreateVersionInput.SourceVersion int`（service 内部仍按 int 查版本，version 对外 int 结论不破坏）。
- 验证：L3 `TestCreateVersionCopyFromDraftRejected` 改发字符串版本号；scenario yaml `sourceVersion: "1"`。

### P4（缺陷·非阻断）auto_apply 触发响应 status 滞后 —— 已修复
- 根因：`TriggerFXSyncRun` 在 auto_apply 分支调 `approveRunInTx` 后未 reload run，响应 `data.status` 仍 `pending_review`（落库 applied）。
- 改动：`service.go:TriggerFXSyncRun` auto_apply 分支在事务内 `repo.GetFXSyncRun(run.ID)` reload 后再组 view；响应 status 与落库一致。
- 验证：L3 `TestFXAutoApplyTriggerPublishes` 改为断言触发响应 `status=="applied"` + 返回 `candidateVersion`；scenario `fx_trigger_auto_apply_publishes` 加 `data.status: applied`。

### P5（可选·非阻断）—— 一并处理
- GET template 去除内嵌 internal `id`：`handler.go` 新增 `templateResponse`（无 id）+`toTemplateResponse`，`GetTemplate` 返回该 DTO。create/list 响应未改（不引入回归）。
- approve/publish 响应类型：前端仅触发 refresh，不消费返回值，保持现状（不扩大改动）。

### 改动文件列表
- 后端：`internal/transport/http/cashier/handler.go`、`internal/app/cashier/service.go`、`internal/app/cashier/ports.go`、`internal/infra/persistence/postgres/cashier_repo.go`。
- 前端：`apps/admin-web/src/api/modules/cashier.ts`、`apps/admin-web/src/views/cashier/templates/PriceMatrixEditor.vue`。
- 测试/契约：`internal/transport/http/cashier/cashier_http_test.go`、`internal/transport/http/cashier/memstore_test.go`、`apps/admin-web/src/views/cashier/templates/__tests__/PriceMatrixEditor.spec.ts`、`tests/backend/scenarios/cashier-template.yaml`。

### 共享文件影响
- 未改 `admin_wiring.go` / `routes.ts`。改动局限于 cashier handler/service/ports + postgres cashier 仓储 + 前端 cashier api/视图（均 cashier-surface 内）。

### 自检（提权 Shell 实跑，worktree = console-cashier-template）
- `services/admin-api`：`go build ./...` ✅；`go vet ./internal/...cashier/postgres` ✅；`go test ./...` ✅（全套件 exit 0，含 cashier L1/L3 + scenario manifest `-count=1` 重跑 ✅）。
- `apps/admin-web`：`pnpm exec vitest run src/views/cashier` ✅ 39/39；`pnpm exec vite build` ✅（仅 chunk size warning）。

---

## ✅ 功能验收师 · cashier-template 端到端功能验收（2026-06-29）

> 模型：Cursor Auto。worktree=/Users/csw/gitproject/console-cashier-template（branch codex/cashier-template）。提权实跑（required_permissions:["all"]）。
> 验收基准：功能端到端可用 + 满足 compact 业务规则 + 符合 02-operation-flow 主线（A8 平台基础数据 / B7 收银台绑定前置「published 版本」）。

### 一、构建 / 测试 / 回归实跑结果（真实输出）
| 命令 | 结果 | 证据 |
| --- | --- | --- |
| `go build ./...`（services/admin-api） | ✅ PASS | exit 0，无输出 |
| `go test ./... -count=1`（services/admin-api） | ✅ PASS | 全 package `ok`，含 `transport/http/cashier`、`domain/cashier`、`app/cashier`、`app/command`、`testkit/scenario` |
| `sh scripts/regression/backend.sh`（统一回归入口） | ✅ PASS | `[regression] backend tests PASS` |
| `pnpm exec vitest run`（admin-web 全量） | ⚠️ 121/122 | cashier 全绿；唯一失败 `views/games/detail/__tests__/sync-section-drawer.spec.ts`（"Create draft from published v7" 文案陈旧）= 既有非本模块用例，按指令不计入 |
| `pnpm exec vite build`（admin-web） | ✅ PASS | `✓ built in 5.08s`，`CashierView-*.js 27.42 kB`，仅 chunk size warning |

> 前端统一入口 `scripts/regression/frontend.sh` = `pnpm test`(vitest) + `pnpm e2e`(playwright)；vitest 已实跑（同上），Playwright cashier 12/12 由 🟪 测试专家实跑留底（本机 vite 冷编译慢，env 注记已抬超时串行）。

### 二、验收清单（编号 | 验收点 | 期望 | 实际 | 证据 | 判定）
| # | 验收点 | 期望 | 实际 | 证据 | 判定 |
| --- | --- | --- | --- | --- | --- |
| 1 | 模板创建 + 审计 | POST templates 201；写 cashier.template.create | 一致 | `cashier_http_test.go:232 TestCreateTemplateSuccessAndAudit`、`:240` byAction | PASS |
| 2 | templateId 唯一冲突 | 重复 → CONFLICT | 一致 | `:264 TestCreateTemplateConflict`、`:271` errCode==CONFLICT；DB `UNIQUE(template_id)` + 部分唯一索引 | PASS |
| 3 | 模板分页列表 | items+page+pageSize+total，pageSize 上限 100 | 一致 | `handler.go:137 ListTemplates`/`parsePage`；`:287 TestListTemplatesPaginationAndClamp` | PASS |
| 4 | 创建版本默认 draft | 产物 status=draft | 一致 | `:304 TestCreateVersionDefaultsToDraft`；`service.go:126` StatusDraft | PASS |
| 5 | 状态机 draft→published | publish 校验 draft，否则 VERSION_STATE_INVALID | 一致 | `domain/cashier/template_version.go:37 CanTransition`；`:471 TestPublishNonDraftRejected` | PASS |
| 6 | 发布自动归档旧 published（同事务） | 新 published 时旧 published→archived | 一致 | `service.go:245-259 PublishVersion`；`:442 TestPublishArchivesOldPublishedSameTx`；DB `uq_cashier_versions_one_published` | PASS |
| 7 | 发布事务回滚 | 失败整体回滚不残留 | 一致 | `:484 TestPublishTransactionRollback` | PASS |
| 8 | published 只读 | PUT rows on published → VERSION_STATE_INVALID | 一致 | `service.go:207`；`:426 TestUpsertRowsOnPublishedRejected`；前端 `PriceMatrixEditor.vue:156 readonlyByStatus` | PASS |
| 9 | copy-to-draft（published） | 产物 draft + source_type=copy_published + 复制行 | 一致 | `service.go:141 CopyToDraft`+`CopyRows`；`:342 TestCopyToDraftFromPublished` | PASS |
| 10 | copy-to-draft（archived） | 产物 draft + source_type=copy_archived | 一致 | `:366 TestCopyToDraftFromArchived` | PASS |
| 11 | copy 来源非法 | 从 draft 复制 → VERSION_STATE_INVALID；缺 sourceVersion→VALIDATION | 一致 | `:328 TestCreateVersionCopyFromDraftRejected`、`:315 TestCreateVersionCopyMissingSourceVersion` | PASS |
| 12 | sourceVersion string | 后端收 string→Atoi（compact 对齐） | 一致 | `handler.go:34/207-215`；前端 `cashier.ts:90` | PASS |
| 13 | 金额归一化→minor | 读 currency_specs→精度/下限/舍入→存 minor | 一致 | `service.go:439 normalizeRow`+`common.NormalizeAmountToMinor`；`:383 TestUpsertRowsNormalizesAmount`("10.00"→1000) | PASS |
| 14 | 币种不支持 | → CURRENCY_NOT_SUPPORTED | 一致 | `service.go:445`；`:399 TestUpsertRowsCurrencyNotSupported`；DB currency FK | PASS |
| 15 | 低于下限 | < minAmountMinor → 拒绝 | 一致 | `:414 TestUpsertRowsBelowMinimum` | PASS |
| 16 | PUT rows 契约对齐 | 前后端同 preTaxAmount(major str)+taxRate(str) | 一致 | 前端 `PriceMatrixEditor.vue:300-308`；后端 `handler.go:39-49` | PASS |
| 17 | FX manual_confirm 触发 | 候选 draft + run(pending_review) + 差异摘要，不自动应用 | 一致 | `service.go:268-313`（默认不进 auto 分支）；`:516 TestFXManualConfirmTriggerThenApprove` | PASS |
| 18 | FX approve 同事务 publish+applied | approve→候选 published + run=applied，无单独 apply 端点 | 一致 | `service.go:380 approveRunInTx`（publish+UpdateReview applied）；`:531-534` status=applied | PASS |
| 19 | FX auto_apply | 触发即 approve→apply，响应 status=applied | 一致 | `service.go:315-325`（事务内 reload run P4 修复）；`:553 TestFXAutoApplyTriggerPublishes` | PASS |
| 20 | FX ignore | action:ignore→run=ignored，候选 draft 保留 | 一致 | `handler.go:321`+`service.go:363 IgnoreFXSyncRun`；`:594 TestFXIgnore` | PASS |
| 21 | FX approve 事务回滚 | 失败回滚 | 一致 | `:621 TestFXApproveTransactionRollback` | PASS |
| 22 | fxSyncRuns 来源 + DTO | GET template 内嵌 fxSyncRuns；runId/candidateVersion(版本号)/diffSummary | 一致 | `handler.go:178-194 GetTemplate`+`mapFXRuns`/`fxRunResponse`；前端 `cashier.ts:52-61` | PASS |
| 23 | 权限码 | read/write/publish/fx.approve；缺权 403 | 一致 | `router.go:19-30`；`:181 TestCashierRBAC`（read 不能写/发布/审核→403） | PASS |
| 24 | 前端权限置灰 | 无权限按钮 v-perm 隐藏/置灰 | 一致 | `PriceMatrixEditor.vue:21/22 v-perm cashier.write`；`FxSyncRunsReviewList.vue:42/51 v-perm fx.approve`；`TemplateVersionsTab.vue:27/41` | PASS |
| 25 | 前端页面端到端 | 列表/详情/版本列表/价格矩阵/FX 审核 + 空错权限态 | 一致 | 7 视图 + vitest 39/39 + Playwright 12/12（截图/视觉基线）；vite build 产出 CashierView chunk | PASS |
| 26 | 版本列表 published 复制入口 | published 行提供"复制为 draft" | 一致 | `TemplateVersionsTab.vue:26-32`（v-if status==published + v-perm + emit copy-version） | PASS |
| 27 | 审计事件 | create/publish/approve 写 audit_logs | 逻辑实现+测试断言；生产 sink 待注入 | `service.go:62/260/419 writeAudit`；`:240/451/538` audit.byAction 断言 | PASS（注1） |
| 28 | env 红线 | platform schema 四表均不带 env | 一致 | 迁移 `000007_*.up.sql`（platform schema、无 env 列/无 search_path） | PASS |
| 29 | 下游契约稳定 | game-cashier 依赖 price_id 快照语义稳定 | 一致 | published 只读 + 唯一 published + copy-to-draft 隔离；`cashier_price_rows.price_id` NOT NULL，绑版本快照不联动 | PASS |

> 注1（审计 sink）：service 层审计逻辑已实现且被 spy 断言覆盖；生产装配 `admin_wiring.go:123 cashierapp.NewService(store, nil, time.Now)` 仍注入 `nil` audit sink → 落库需待 audit 模块(22)统一接通。该模式与 game/channel/account-auth **完全一致**（`admin_wiring.go:99/112/115/119` 均 nil，注释「audit 模块 22 落地后统一接通，非本模块新增遗留」）→ 属跨模块集成事项，不计为本模块阻断。

### 三、operation-flow 主线走查（02 §A8 / §B7）
- §A8「收银台 价格模板/汇率」平台基础数据：CRUD + 版本生命周期 + FX 审核闭环均成立（见 #1–#22）。
- §B7「绑定收银台模板版本」前置=「A8 价格模板存在 published 版本」：本模块 publish 链路（draft→published，旧自动 archived，唯一 published）保证下游 game-cashier 可绑定稳定 published 快照（#5/#6/#29）。
- 能力闭环 ✅ / 状态流转 ✅ / 错误·冲突如约（CONFLICT/VERSION_STATE_INVALID/CURRENCY_NOT_SUPPORTED/VALIDATION_FAILED）✅ / 权限生效 ✅ / 脱敏 N/A（本模块无密文）。

### 四、结论
- **判定：✅ 通过。** 29 项验收点全 PASS，构建/后端测试/统一回归入口全绿，前端 cashier vitest 全绿 + vite build 通过。
- 唯一失败用例为既有非本模块 `games/sync-section-drawer`，按指令不计入。

### 五、遗留风险与建议
1. 审计 sink 生产注入：待平台 audit 模块(22)统一接通 cashier/game/channel 等 service 层 nil sink（跨模块，非本模块阻断）。
2. FX Provider 为占位：trigger 生成候选并复制现有 published 行，差异摘要为 source/candidate/copiedRows，未接真实汇率算价；接真实 fx provider 时补差异计算（compact 已声明 fx provider 抽象）。
3. version 对外数值 / DB VARCHAR：与 compact「字符串型自增」语义兼容（前端 String() 归一化、sourceVersion 已 string）；建议后续文档统一口径。
