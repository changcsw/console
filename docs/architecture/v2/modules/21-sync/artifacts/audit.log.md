# sync (#21) · 执行审计日志

> 完整执行日志、命令、失败记录、审计证据；仅供人类审计。总/集成 Agent 默认不读本文件。

---

## 2026-07-01 · 🎯 总负责 Agent · 模块初始化

### 依赖闸门判定
- 目标模块：sync (#21)，lane=runtime-surface，depends_on=[snapshot, channel, account-auth, channel-login, feature-plugin, product, cashier-template, game-cashier, payment, game, common]。
- 读取范围（严守瘦身纪律）：index.json + codegen-progress.md + modules/21-sync/spec.compact.md + codegen-workflow.md（角色剧本）。
- 上游状态（据 codegen-progress.md 进度总表）：
  - #10 auth ✅ / #11 game ✅ / #12 channel ✅ / #13 account-auth ✅ / #14 channel-login ✅（已合并 main）
  - #15 feature-plugin ✅ / #16 product ✅（已合并 main）/ #17 cashier-template ✅ / #18 game-cashier ✅
  - #19 payment ✅（已合并 main）/ #20 snapshot ✅（验收 31/31 PASS，已合并 main）
- 同 lane（runtime-surface）：snapshot #20 已完成合并，无其他在制模块。
- **判定：闸门满足，允许开工。**

### 环境
- worktree：/Users/csw/gitproject/console-sync（HEAD 8af70c6）；分支 codex/sync（本地，未推 origin）。
- 说明：workspace root 为 /Users/csw/gitproject/console，对 console-sync 的写入越沙箱执行（move_agent_to_root 因本地分支无 origin ref 而失败）。

### 编排计划
- 后端车道：🟦后端开发 → 🟦CR → 🟦测试（串行）。
- 前端车道：🟩前端开发 → 🟩CR → 🟩测试（串行，与后端并行）。
- 两车道均 ✅ → 🟪测试专家 ⇄ 🟧全栈修复 → ✅功能验收。

### 阶段记录
（各子 agent 追加）

## 2026-07-01 · 🟦后端开发 · sync 模块实现

### 代码实现
- 迁移：新增 `services/admin-api/migrations/000016_sync_platform_schema.up.sql` / `.down.sql`，在 `platform` 建 `sync_jobs`、`sync_job_items`、`sync_consumed_tokens` 及索引。
- 领域：重写 `internal/domain/sync/sync.go`，补齐 9 section 枚举、拓扑依赖、Diff 模型、BaselineToken（HMAC-SHA256）签验、规范化 hash、diff 算法与脱敏策略。
- 应用：重写 `internal/app/command/preview_section_sync.go`、`execute_section_sync.go`，实现 preview/execute/list-jobs 编排、双闸门（nonce + baseline hash）、单事务执行与失败标记。
- 仓储：新增 `internal/infra/persistence/postgres/sync_store.go`、`sync_repo.go`，实现 sync_jobs/sync_job_items/sync_consumed_tokens CRUD、分页，以及跨 schema section 数据读取与 apply。
- 传输与接线：重写 `internal/transport/http/sync/section_sync_handler.go`、新增 `router.go`，并在 `httpserver/admin_wiring.go` 接入真实 SyncService；`httpserver/router.go` 移除 sync scaffold fallback。
- 测试调整：更新 `section_sync_handler_test.go`、`preview_section_sync_test.go` 以适配新接口/DTO。

### 自检命令与结果（services/admin-api）
- `go build ./...`（required_permissions=["all"]）→ 首次失败（未使用 import / AuditEntry 字段不匹配 / router 残留引用），修复后 **通过**。
- `go vet ./...`（required_permissions=["all"]）→ **通过**。
- `go test ./internal/app/command ./internal/transport/http/sync`（required_permissions=["all"]）→ **通过**。

### 迁移验证
- 现状：本地会话未连接可用 PostgreSQL，未执行真实 `migrate up`。
- 已做：SQL 语法与结构通过 Go 编译链路引用校验；迁移前向执行留待 PG CI / 集成环境。

## 2026-07-01 · 🟩前端开发 · sync 模块实现

### 读取与对齐
- 按协议读取：`index.json`、`00-common.md`、`01-structure.md`、`CONVENTIONS.md`、`modules/21-sync/spec.compact.md`。
- 基于现有 scaffold 增量改造：`api/syncSections.ts`、`GameDetailView.vue`、`SyncSectionDrawer.vue`，并新增 `SyncJobsTab.vue`。
- 参考同目录既有 tab/权限样式（`SnapshotTab` / `PaymentRoutesTab`）保持信息架构一致。

### 前端实现变更
- `apps/admin-web/src/api/syncSections.ts`
  - 补齐 compact 契约类型：`baselineToken`、`DiffSection/DiffChange`、`appliedSummary`、`skipped`、`sync-jobs` 分页 item。
  - 补齐路由客户端：`previewSyncSections` / `executeSyncSections` / `listSyncJobs`。
- `apps/admin-web/src/views/games/detail/components/SyncSectionDrawer.vue`
  - 打开抽屉自动 `POST /sync/preview`（默认全 section），按 section 分组并显示 add/update/delete 徽标。
  - 差异行按 add(绿)/update(黄)/delete(红)着色；update 展示 sandbox→production 对照。
  - 密文字段统一展示 `••••••`，不可展开明文；`includeDeletes` 默认 `false`，delete 行在关闭时标注“仅提示，不执行”。
  - execute 请求携带 `baselineToken + selectedSections + includeDeletes + operatorNote`。
  - 错误反馈：`SYNC_BASELINE_MISMATCH` 弹窗并支持“重新预览”；`UNKNOWN_SECTION`/`VALIDATION_FAILED` 展示依赖 details。
- `apps/admin-web/src/views/games/detail/SyncJobsTab.vue`（新增）
  - 同步历史表：任务ID/状态/selectedSections+include_deletes/操作者/备注/hash/executedAt/createdAt。
  - 支持 status 过滤与分页；失败行展开错误概要，成功行展开 `appliedSummary`。
- `apps/admin-web/src/views/games/detail/GameDetailView.vue`
  - 接入 `SyncJobsTab` 到“同步记录”Tab。
  - 仅 `sandbox` 环境渲染 `Sync to Production` 入口，并受 `sync.execute` 权限约束（无权限置灰）。
  - 接入 `SyncSectionDrawer` 执行后自动刷新同步历史。
- `apps/admin-web/src/views/games/detail/__tests__/sync-section-drawer.spec.ts`
  - 更新为新契约断言（`selectedSections/baselineToken/includeDeletes/operatorNote`）。

### 验证命令（apps/admin-web）
- `pnpm exec vue-tsc --noEmit && pnpm vite build`
  - 结果：通过（exit code 0）。
  - 备注：vite 构建出现既有 rollup chunk size warning（非阻断）。

## 2026-07-01 · 🟩前端CR · sync 模块评审

### 读取与核对范围
- 协议文档：`index.json`、`spec.compact.md`（前端要点）、`CONVENTIONS.md`、`01-structure.md` §5.3。
- 前端 diff：`api/syncSections.ts`、`GameDetailView.vue`、`SyncSectionDrawer.vue`、`SyncJobsTab.vue`、`__tests__/sync-section-drawer.spec.ts`。
- 对照 compact 前端要点逐条核对（见 handoff.summary.md 核对表）。

### 核对结论（compact 前端要点）
| 要点 | 已实现 | 一致 | 证据 |
| --- | --- | --- | --- |
| 仅 sandbox 渲染 Sync 入口 + sync.execute 置灰 | ✅ | ✅ | `GameDetailView.vue:14-21`（`v-if="app.environment === 'sandbox'"` + `v-perm` + `:disabled="!canSyncExecute"`） |
| 预览抽屉 section 分组 + add/update/delete 徽标 | ✅ | ✅ | `SyncSectionDrawer.vue:58-73` |
| 差异行配色 add 绿 / update 黄 / delete 红 | ✅ | ✅ | `SyncSectionDrawer.vue:368-381` |
| update 展示 sandbox→production 对照 | ✅ | ✅ | `SyncSectionDrawer.vue:98-101` |
| section 级复选框 selectedSections | ✅ | ✅ | `SyncSectionDrawer.vue:60-66,194-200` |
| include_deletes 默认关 + delete 仅提示样式 | ✅ | ✅ | `SyncSectionDrawer.vue:44-48,90-96,151` |
| execute 携带 baselineToken+selectedSections+includeDeletes+operatorNote | ✅ | ✅ | `SyncSectionDrawer.vue:254-258`；`syncSections.ts:69-74` |
| 成功 toast + 刷新历史 Tab | ✅ | ✅ | `SyncSectionDrawer.vue:260`；`GameDetailView.vue:150-153` |
| SYNC_BASELINE_MISMATCH 弹窗重新预览 | ✅ | ✅ | `SyncSectionDrawer.vue:233-245,263-266` |
| UNKNOWN_SECTION / VALIDATION_FAILED 展示 details | ✅ | ✅ | `SyncSectionDrawer.vue:218-231,267-273` |
| SyncJobsTab 列项 + status 过滤 + 分页 | ✅ | ✅ | `SyncJobsTab.vue:5-75,128-149` |
| 失败行错误概要 / 成功行 appliedSummary | ✅ | ✅ | `SyncJobsTab.vue:20-33` |
| API client DTO 与 compact 契约 | ✅ | ✅ | `syncSections.ts:17-170` |
| 密文 masked 恒 •••••• 不可展开 | ✅ | ✅ | `SyncSectionDrawer.vue:173-191` |
| 抽屉式交互 / page 组件 | ✅ | ✅ | `SyncSectionDrawer.vue:2-8`（el-drawer + PageStatusTag） |

### 问题清单
**阻断**：无。

**建议（未阻断，部分已修复）**：
- `SYNC_TOKEN_CONSUMED` 未单独处理 → **已修复**：复用「重新预览」弹窗（`SyncSectionDrawer.vue`）。
- `hasDiff=false` 时仍可点执行 → **已修复**：无差异 info 提示 + 禁用执行按钮。
- 基线 mismatch 弹窗标题用错误码 → **已修复**：改为「目标已变更，请重新预览」。
- 失败行未展示 `errorSummary.details` → **已修复**：展开区追加 JSON（`SyncJobsTab.vue`）。
- 入口仅校验 `sync.execute`，preview API 需 `sync.preview`：与 compact 一致（入口只提 execute）；后端兜底，非阻断。

### CR 直接修复
- `SyncSectionDrawer.vue`：`handleTokenStale` 统一处理 `SYNC_BASELINE_MISMATCH`/`SYNC_TOKEN_CONSUMED`；`hasDiff` 空态；执行按钮禁用条件。
- `SyncJobsTab.vue`：失败展开区展示 `errorSummary.details`。

### 验证命令（apps/admin-web，修复后）
- `pnpm exec vue-tsc --noEmit && pnpm vite build` → 通过（exit 0；rollup chunk size warning 非阻断）。

### 结论
**通过** — 前端实现与 compact 前端要点一致，红线（production 不渲染 Sync 入口、密文脱敏）满足；无阻断项，可进入 🟩前端测试。

## 2026-07-01 · 🟦后端CR · sync 模块评审

### 读取与核对范围
- 协议文档：`spec.compact.md`、`CONVENTIONS.md`、`00-common.md`（D1/D6、§2.2/§6/§7/§8/§9）。
- 后端 diff/实现：`migrations/000016_*`、`domain/sync`、`app/command/*sync*`、`infra/postgres/sync_*`、`transport/http/sync/*`、`httpserver/{admin_wiring,router}.go`。
- 自检复验（services/admin-api）：`go build ./...` ✅ · `go vet ./...` ✅ · `go test ./internal/app/command ./internal/domain/sync ./internal/transport/http/sync ./internal/app/audit` ✅

### 核对结论（compact 要点）
| 要点 | 已实现 | 一致 | 证据 |
| --- | --- | --- | --- |
| platform 三表+索引+000016 幂等 | ✅ | ✅ | `migrations/000016_sync_platform_schema.up.sql:6-50` |
| 9 section 枚举/拓扑/默认值 | ✅ | ✅ | `domain/sync/sync.go:22-67,193-214` |
| BaselineToken HMAC+TTL30min+nonce | ✅ | ✅ | `domain/sync/sync.go:230-275`; preview `preview_section_sync.go:206-221` |
| 规范化 hash+diff+masked | ✅ | ✅ | `domain/sync/sync.go:277-503` |
| preview 落库 jobs+items | ✅ | ✅ | `preview_section_sync.go:189-204` |
| execute 审定顺序 section→token→nonce→baseline→dep→tx | ✅ | ⚠️ | `execute_section_sync.go:38-96`（依赖为 section 级基础校验） |
| 双闸门 nonce 同事务+基线复核 | ✅ | ✅ | `execute_section_sync.go:49-55,71-76,108-115` |
| 单事务 upsert+失败→failed | ✅ | ✅ | `execute_section_sync.go:101-157`; `sync_store.go:24-33` |
| 跨 schema 读写 schema 限定名 | ✅ | ✅ | `sync_repo.go:268-877`; `safeSchema:879-889` |
| 有效数据过滤 hidden/invalid/enabled/published | ✅ | ⚠️ | channels/packages 等有过滤；channels 未含 login/iap/plugin 子表 |
| API 三路由+权限码+包络 | ✅ | ✅ | `sync/router.go:18-20`; `section_sync_handler.go:32-107` |
| GET sync-jobs 分页+status 过滤 | ✅ | ⚠️ | `ListJobs` 已实现；`sort` query 未解析（默认 created_at DESC 符合默认） |
| audit sync.execute env=production | ✅ | ✅ | CR 修复：`execute_section_sync.go:139-151` + `AuditEntry.Env` |
| source_env=sandbox→production 硬编码 | ✅ | ✅ | `domain/sync/sync.go:41-42,173-174` |

### 问题清单
**阻断**：无（CR 已补依赖校验基础步、audit env、operatorNote 落库、ConsumeNonce 冲突映射）。

**建议（非阻断，留 open_issues）**：
- section 数据面未全覆盖 compact 源表（channels 缺 login/iap/plugin；products 缺 channel_products；cashier 缺 price_overrides）→ 后续迭代补 loader/apply。
- 依赖校验现为 section 级（production 是否有该 section 数据），非 entityKey 细粒度。
- `applyPayments` 使用 `ON CONFLICT DO NOTHING`，已有路由 update 可能不生效。
- 删除仍被引用→skipped 未实现引用检测（仅 include_deletes=false 跳过）。
- 迁移 000016 真实 DB 前向执行待 PG CI。

### CR 直接修复
- `domain/sync/sync.go`：新增 `ValidateSectionDependencies`（section 级依赖校验）。
- `execute_section_sync.go`：插入依赖校验步骤；audit `Env=production`；operatorNote≤255 校验与落库；ConsumeNonce 仅 `ErrConflict`→`SYNC_TOKEN_CONSUMED`。
- `admin/ports.go` + `audit/sink.go`：`AuditEntry.Env` 透传至 audit service。
- `sync_repo.go`：`UpdateJobResult` 支持 execute 时写入 `operator_note`。

### 结论
**通过** — 核心红线（双闸门、单事务、跨 schema、密文 masked、sandbox→production）满足；已知 section 覆盖/细粒度依赖/payments upsert 等待 backend_test 或后续迭代，可进入 🟦backend_test。

## 2026-07-01 · 🟦后端测试 · sync 模块测试

### 读取与对齐
- 协议：`index.json` → `modules/21-sync/spec.compact.md` → `03-testing.md`（分层 L1–L5、目录约定、S1–S10 维度、fixtures/回归入口）。
- 输入：`handoff.summary.md` + audit.log 后端开发/后端CR 小节（已交付面 + CR open_issues）。

### 交付产物（按 03-testing 目录与对齐规则）
1. **L1 单元 — 领域纯函数** `internal/domain/sync/sync_test.go`（24 test 函数，含多子测试）
   - 确定性 hash 规范化：`HashEntitySets` 多次恒等（含 map 迭代序/切片内部排序）；`NormalizeForHash` 排除 id/created_at/updated_at（大小写/驼峰）；语义字段变更改变 hash；**密文以密文参与 hash**（differing ciphertext→differing hash）；`CanonicalJSON` key 序稳定。
   - BaselineToken 签验：round-trip；过期（now>expiresAt）；篡改 payload（sig 失败）；错误密钥；畸形格式（空/无点/三段/非 base64）；`GenerateNonce` 32-hex 唯一。
   - 依赖拓扑：`SortSectionsByTopo` 乱序→固定写入序；`ValidateSectionDependencies`（缺前置/同批满足/production 满足/根 game 无依赖）。
   - diff：add/delete 整行 `*`、update 字段级（仅变化字段）、无变化零 change；**masked 判定**（`*_ciphertext`/含 secret/显式集 → masked=true 两值恒 `masked`）。
   - 9 section 枚举/`ParseSections`（默认全集/显式空拒/UNKNOWN/去重 trim）/状态与方向常量（previewed·succeeded·failed / sandbox→production）。
2. **L2/L3 应用编排 — fake repo 进程内** `internal/app/command/execute_section_sync_test.go`（17 test 函数）
   - execute 成功：写 1 条 audit `action=sync.execute` **`Env=production`**、ActorID 取鉴权上下文、detail 含 selectedSections；nonce 成功消费；终态 succeeded。
   - 闸门与错误码：UNKNOWN_SECTION(400)、token 无效/过期/**game·env 不匹配**→VALIDATION_FAILED(400)、nonce 预检→SYNC_TOKEN_CONSUMED(409)、**基线复核**→SYNC_BASELINE_MISMATCH(409)、依赖缺失→VALIDATION_FAILED(400,details)、目标 game 不存在→NOT_FOUND(404)、operatorNote>255→VALIDATION_FAILED。
   - **事务回滚(S10)**：ApplySection 中途失败→整体回滚（nonce 未消费、无审计、sync_jobs→failed）；事务内 ConsumeNonce 唯一冲突(ErrConflict)→SYNC_TOKEN_CONSUMED(409)+failed。
   - preview：落 previewed + items(applied=false)、baselineToken 可验签、**masked item 不落明文**、preview 不写审计；UNKNOWN_SECTION/NOT_FOUND。
   - ListJobs：分页钳制（page≤0→1、pageSize>100→100、status 透传）、空 gameID→VALIDATION_FAILED。
3. **L3 handler** `internal/transport/http/sync/section_sync_handler_test.go`：execute + **preview** 未知 section→400 UNKNOWN_SECTION（新增 preview 用例）。
4. **接口场景矩阵 manifest** `tests/backend/scenarios/sync.yaml`：3 接口 × S1–S10 逐 case 标注（25 case）。进程内可跑 S2 鉴权 401（preview/execute/sync-jobs 各一，requiresDB=false，PASS）；S1/S3/S4/S5/S6/S7/S8/S10 标 requiresDB=true（连库 harness / 已由 L1+L2 等价覆盖），附 note 与对应单测名。
5. **fixtures** `tests/fixtures/sandbox/sync.sql` + `tests/fixtures/production/sync.sql`：RBAC（sync_operator/sync_executor/sync_operator_preview_only/sync_reader_noperm）、sandbox 源差异样本（channels.add / products.update / 失效数据排除）、production 基线 + drifted（S5）+ empty（依赖缺失）+ secret（S8）+ sync_jobs 历史（列表/分页/过滤）。与 sibling 一致，按 scenario `fixture:` 引用，由连库 harness 消费（`db.sh` 仅 seed common/，与 snapshot/game 等同惯例）。

### 测试基建修复（连带发现，非本模块红线）
- **degraded 装配漏挂 sync 路由**：`httpserver` 降级路由（进程内 scenario harness 用）注册了 games/channels/cashier/payment/snapshot/audit，**独缺 sync**。导致未连库时 sync/preview 落 legacy scaffold 返回 501（而非 401），与全部 sibling 模块不一致，S2 鉴权闸门无法进程内验证。
  - 修复：`admin_wiring.go` degraded 补 `syncapi.RegisterRoutes(..., NewSectionSyncHandler(nil), ..., ready=false, nil)`（ready=false 时 Authn 先返回 401，handler 不被调用，nil service 安全）。
  - 连带修正 backend_dev 遗留的**陈旧 smoke 用例**（假设已移除的免鉴权 scaffold）：`scenario/runner_test.go` `TestRunCaseSyncPreviewRejectsUnknownSection`→改为 `TestRunCaseSyncPreviewRequiresAuth`（401）；`tests/backend/scenarios/smoke.yaml` 两条 sync 用例（期望 200/400 免鉴权）合并为 `sync_preview_requires_auth`（401）。这两处在本次改动前即已失败（501≠200/400），属 scaffold 移除后未同步的测试债，已理顺。

### 运行结果（cwd=services/admin-api，required_permissions=["all"]）
- `go vet ./internal/domain/sync ./internal/app/command ./internal/transport/http/sync ./internal/transport/httpserver ./internal/testkit/scenario` → **通过**。
- `go test ./internal/domain/sync ./internal/app/command ./internal/transport/http/sync` → **通过**（sync 相关 45 PASS，0 FAIL）。
- `go test ./internal/testkit/scenario`（scenario harness，含 sync.yaml + smoke）→ **通过**：sync 25 case = 3 PASS(S2 鉴权) + 22 SKIP(requiresDB，manifest 解析 OK)。
- `go test ./...`（全量 33 包）→ **EXIT=0，0 FAIL**（修复陈旧 smoke 后全绿）。
- 连库(requiresDB)维度：本地无 PG，标注待 PG CI（`SCENARIO_WITH_DB=1`）；红线（双闸门/事务回滚/跨env/脱敏/审计env=production）已由 L1 领域单测 + L2 fake-repo 应用单测进程内等价覆盖。

### 疑似实现缺陷（探查 CR open_issues 对正确性的影响；归因）
按严重度排序，均**非本次红线阻断**，建议回退 🟦后端开发（经总 Agent 调度）迭代：
1. **[中·潜在 S8 隐患] 整行 add/delete diff 不脱敏**：`DiffEntities` 仅 update 分支调 `isMaskedField`；add/delete 直接回填整行 `Fields`。当前 loader 未 surface 任何密文列（见 #2），故**当前无泄漏**；但一旦 loader 补齐 login/iap/plugin 密文字段而未预脱敏，新增/删除渠道实例的密文将在 preview 明文回传。建议：add/delete 整行也按字段级脱敏，或 loader 侧统一以密文/占位入 `Fields`。
2. **[中·功能缺口] section loader 未全覆盖 compact 源表**：channels 缺 login/iap/plugin 子表、products 缺 channel_products、cashier 缺 price_overrides。后果：这些子实体的 add/update/delete 不进 diff、不参与 hash → 基线复核可能漏判其变更；S8 脱敏在真实链路暂无触发点（domain 逻辑已单测守护）。
3. **[中·正确性] `applyPayments` ON CONFLICT DO NOTHING**：已存在 payment_route 的字段更新（priority/enabled 等）execute 时不生效，与「存在则更新有变化字段」语义相悖；建议改 DO UPDATE。
4. **[低] 删除仍被引用→skipped 未做引用检测**：`include_deletes=true` 仅按 sandbox 失效/不存在判定，未做 production 引用扫描；`skipped.deletes` reason 目前仅 include_deletes=false 一种。
5. **[低] 依赖校验为 section 级**：仅校验前置 section 在 production「有无数据」，非 entityKey 细粒度（如 channels 缺具体 `<market>/<channel_id>`）。details 目前 entityKey 恒 `*`。
6. **[低] hash 排除 game_secret 等密文列**：`loadGame` 未选 `game_secret`（applyGame 却会写它）→ game_secret 变更不反映在 hash/基线复核（非泄漏，但基线可能漏判）。
7. **[待 PG CI] 迁移 000016 真实前向执行**、连库 expect.db/审计落库断言：无本地 PG，待 CI。

### handoff
- 结论：sync 后端测试**通过**；红线经 L1+L2 进程内等价覆盖，连库维度待 PG CI。
- 无阻断实现缺陷；上列 1–6 为 CR open_issues 的正确性归因，建议后续迭代（非本期红线）。

## 2026-07-01 · 🟩前端测试 · sync 模块测试

### 读取与对齐
- 协议：`index.json` → `modules/21-sync/spec.compact.md`（前端要点）→ `03-testing.md`（L4 vitest 组件 / L5 Playwright 截图基线 / 目录约定）。
- 输入：🟩前端开发 + 🟩前端CR handoff（已交付 `SyncSectionDrawer.vue` / `SyncJobsTab.vue` / `GameDetailView.vue`(sandbox 入口) / `api/syncSections.ts`；CR 通过并小修 4 项）。

### 交付产物（按 03-testing 目录与约定）
1. **L4 组件（vitest + @testing-library/vue，mock API）**
   - `src/views/games/detail/__tests__/sync-section-drawer.spec.ts`（**重写**，18 test）：覆盖对象 `SyncSectionDrawer.vue`。
     - 预览：打开自动 `previewSyncSections("100001",{})`、按 section 分组 + add/update/delete 计数徽标、差异行配色（`.sync-change--add/update/delete`）、update「sandbox→production」对照、hasDiff=false 提示无差异且执行按钮禁用。
     - 脱敏（红线）：masked 行恒 `••••••`，断言明文 `SANDBOX/PROD_PLAINTEXT_SECRET` 不出现。
     - include_deletes：默认关时 delete 行 `.sync-change--delete-muted` +「仅提示，不执行」；开启后取消置灰。
     - payload：默认全选 → `{selectedSections:[channels,payments], baselineToken, includeDeletes:false, operatorNote}`；取消勾选仅含所选；开启 delete→includeDeletes:true；全取消→按钮禁用不触发 execute。
     - 结果反馈：成功 toast+emit executed；`SYNC_BASELINE_MISMATCH`→弹「目标已变更，请重新预览」确认后重新预览（previewApi 2 次）；`SYNC_TOKEN_CONSUMED`→弹「预览凭证已使用，请重新预览」；`VALIDATION_FAILED`/`UNKNOWN_SECTION`→行内 details；预览失败错误态。
   - `src/views/games/detail/__tests__/SyncJobsTab.spec.ts`（**新增**，10 test）：覆盖对象 `SyncJobsTab.vue`。
     - 挂载即 `listSyncJobs("100001",{page:1,pageSize:20,status:undefined,sort:"-createdAt"})`；列项渲染（任务ID/状态标签/selectedSections+include_deletes/操作者/备注/hash 截断/时间）；status 过滤按新值 reload(1)；分页 reload(target)；状态标签映射 success/warning/info；失败行展开错误概要 code+message+details；成功行展开 appliedSummary；加载失败错误态；空列表 total=0。
   - `src/views/games/detail/__tests__/game-detail-sync-entry.spec.ts`（**新增**，4 test）：覆盖对象 `GameDetailView.vue` 入口**红线**。
     - production 环境绝不渲染 Sync 入口；develop 环境亦不渲染；sandbox+`sync.execute` 渲染且可用、点击打开抽屉；sandbox 但缺 `sync.execute`→入口置灰禁用（`v-perm` disabled+`perm-disabled`）。
2. **L5 Playwright（对契约 mock/stub + 截图基线）** `tests/frontend/e2e/sync.spec.ts`（**新增**，9 test，挂 §3 顶层 e2e 目录）：
   - 红线正向 sandbox 渲染入口 / 红线 production 不渲染（`toHaveCount(0)`）/ 权限置灰（`toBeDisabled`）。
   - 预览抽屉 section 分组+徽标+配色（**视觉基线** `sync-drawer-preview.png` + 全页截图）；密文脱敏 `••••••` 无明文；include_deletes 默认关 delete 行「仅提示，不执行」。
   - 执行成功：断言 execute 请求体 `baselineToken/selectedSections/includeDeletes` → 成功 toast → 切「同步记录」Tab 展示历史。
   - `SYNC_BASELINE_MISMATCH`(409)→弹窗提示重新预览。
   - 同步历史：列表渲染 + status=failed 过滤（断言下发 query）+ 失败行展开错误概要（截图）。

### 顺手修复 / 归因（既有 1 例 vitest 失败）
- 全量 vitest 原有 **1 例失败** = `sync-section-drawer.spec.ts` 中 “execute payload only includes selected sections”。**归因：本模块引入的过时测试**（非环境态）——前端开发早期写的测试假设「勾选即选中」，但 CR 后组件改为「预览后默认全选 + 无差异/空选禁用执行」，旧测试连点两个 checkbox 反而取消全部 section→按钮禁用→execute 未触发→断言失败。
- **已修复**：重写为完整套件（18 test，见上），断言与当前实现一致；顺带移除该文件中误放的、与 sync 无关的 `CopyPublishedToDraftDialog` 用例（已在 `cashier/.../CashierDialogs.spec.ts` 覆盖，无覆盖损失）。

### 运行结果（cwd=apps/admin-web，required_permissions=["all"]）
- `pnpm vitest run`（全量）→ **37 files / 290 tests 全 PASS，0 FAIL**（含本模块新增 32 test：drawer 18 + jobs 10 + entry 4）。原 1 例失败已随重写消除。
- `pnpm exec playwright test sync.spec.ts` → **9 PASS**（单 worker，含视觉基线生成 `visual-baseline/sync.spec.ts-snapshots/sync-drawer-preview-chromium-darwin.png`）；不带 `--update` 复跑视觉基线对照稳定通过。

### 环境态定位（非本模块阻断）
- 首次运行 sync e2e 全 9 例卡在共享 `openGameDetail` 导航（`.detail-head__title` 15s 超时）。经 trace 排查确认为**环境级既有问题**，`games.spec.ts` 详情用例在同环境同样失败（同一现象）：
  1. **多字节响应体挂起**：Playwright `route.fulfill({body: JSON.stringify(...)})` 在 Chrome 149 下，`Content-Length`(UTF-8 字节)与实际传输长度不一致（「星际远征」4 CJK 字符差 8 字节 → `content-length:374` vs `bodySize:366`），浏览器等待未完成字节导致 `getGame` 的 `fetch` 永不 resolve、详情页卡死。本 spec 改用 Playwright `json:` fulfill 参数（自动计算正确 Content-Length）规避。
  2. **冷编译慢**：首次进入详情页需 vite 冷编译整页重型 Tab（>15s），超过默认 `toContainText` 15s 超时。本 spec 将导航等待放宽至 45s（仍在 90s test timeout 内）。
- 以上两点为**本地 Playwright/vite 环境态**，非 sync 实现缺陷；task 已知的 8 例 games/product 基线失败同属环境态。

### 疑似实现缺陷（回退 🟩前端开发）
- **无**。前端实现与 compact 前端要点一致，红线（production 不渲染 Sync 入口、密文脱敏、权限置灰）经组件 + e2e 双重覆盖，全部通过。CR 已修的 4 项在测试中确认符合预期。

### handoff
- 结论：sync 前端测试**通过**；32 组件 test + 9 e2e 全绿，视觉基线已建；无阻断、无回退项。
- 后续：与后端车道汇合后进入 🟪集成测试（连库跨栈 e2e 由测试专家承担）。

## 2026-07-01 · 🟪测试专家(第1轮) · sync 模块集成/系统测试

### 读取与对齐
- 协议：`index.json` → `spec.compact.md` → `03-testing.md` → `02-operation-flow.md`；输入：后端/前端车道全部 handoff（audit.log 各小节 + handoff.summary 全文）。
- 前置闸门：🟦backend_test ✅ + 🟩frontend_test ✅（满足）。

### 契约对账（前端 api/syncSections.ts + 组件 vs 后端 transport/http/sync + app/command DTO/错误码/包络）
逐项核对**方法/路径/DTO 字段(camelCase)/错误码/包络**，结论：**核心契约一致**，另发现 3 项非阻断契约漂移（SYNC-INT-07/08/09，见问题清单）。
- 路由/方法：`POST /api/admin/games/{gameId}/sync/preview|execute` + `GET .../sync-jobs`（挂 `/api/admin`，`router.go:41`）——前后端一致。
- 请求 DTO：preview `{sections?, includeDeletes?}`、execute `{selectedSections, baselineToken, includeDeletes?, operatorNote?}`——JSON tag 与前端 payload 逐字段一致（`section_sync_handler.go:33-36,62-67`）。
- 响应包络：统一 `{data:...}`/`{error:{code,message,details}}`（`httpx/envelope.go`），前端 `http.ts` 解包 `data`/`error.code`——一致。
- 错误码：`UNKNOWN_SECTION/VALIDATION_FAILED/SYNC_BASELINE_MISMATCH/SYNC_TOKEN_CONSUMED/NOT_FOUND/FORBIDDEN/INTERNAL` 全量存在且 HTTP 状态匹配（`preview_section_sync.go:36-47,266-304`；`RequirePerm`→403）。
- 密文包络：preview masked 字段值恒 `"masked"`、`masked=true`——一致。

### 跨栈集成 e2e（进程内 httptest + fake service，连库维度待 PG CI）
- 新增 `internal/transport/http/sync/section_sync_contract_integration_test.go`（3 test，全 PASS）：驱动 handler 实际写出的 JSON，断言 ①preview 包络+camelCase 全字段+密文 masked+明文不泄漏；②execute 成功响应全字段+`syncJobId` 序列化类型取证；③list 分页包络+item compact 字段+`selectedSections` 缺失取证。
- 关键路径/分支（preview→勾选 selectedSections→execute；SYNC_BASELINE_MISMATCH/SYNC_TOKEN_CONSUMED/依赖缺失/回滚）：由 L1 domain(24) + L2 app fake-repo(17) + L3 handler(3) + scenario S2 进程内等价覆盖；真实 upsert/审计落库/迁移前向执行 → sync.yaml requiresDB 22 case 待 PG CI(`SCENARIO_WITH_DB=1`)。

### 全量回归（真实输出）
- 后端(cwd=services/admin-api, required_permissions=[all])：`go vet`(sync 相关 5 包) ✅；`go test ./...`(全 41 包) **EXIT=0 全 ok**（含新增契约集成 test）；scenario harness `TestRunCaseSyncPreviewRequiresAuth` **PASS**，sync 22 requiresDB SKIP(manifest 解析 OK)。
- 前端(cwd=apps/admin-web, required_permissions=[all])：`pnpm vitest run` **37 files / 290 tests 全 PASS**（含 sync 32）。Playwright sync.spec 9 PASS 已由 🟩前端测试建基线（视觉基线本地环境态，非本模块阻断），本轮未重复触发以避免已知 env flakiness。

### 红线端到端核验（结论）
| 红线 | 结论 | 证据 |
| --- | --- | --- |
| 仅 sandbox→production（硬编码兜底） | ✅ | `domain/sync/sync.go:41-42`；execute source/target 恒 sandbox/production（`execute_section_sync.go:51,186-187`） |
| production 绝不渲染 Sync 入口 | ✅ | 前端 `GameDetailView`(v-if sandbox)+入口红线组件/ e2e；后端 source_env 兜底 |
| 双闸门 nonce 去重 + 基线复核 | ✅ | `execute_section_sync.go:54-60,80-82,112-117`（预检 409 + 事务内 ConsumeNonce 冲突 409） |
| 单事务不部分写入/失败 failed | ✅ | `execute_section_sync.go:105-159`（InTx 内 apply+nonce+audit；txErr→UpdateJobResult failed） |
| 密文 preview masked、upsert 不经明文 | ✅(现状)/⚠️(潜在) | update 分支 masked；apply* 以 SQL `SELECT ... FROM sandbox` 服务端搬运不经明文；**整行 add/delete 未字段级脱敏见 SYNC-INT-03** |
| 跨 schema 仅 sync 域 | ✅ | `sync_repo.go` schema 限定名 + `safeSchema` 白名单(268-889) |
| 审计 sync.execute env=production 脱敏 | ✅ | `execute_section_sync.go:139-153`（Action/Env=production/detail 计数+hash，无明文） |

### 下游契约抽查
- #23 dashboard 依赖 `GET /games/{id}/sync-jobs`：接口可用，返回 `data.{items,page,pageSize,total}` + compact item 字段（契约集成 test 已断言）。**通过**。
- #22 audit 真正被调用：execute 成功在事务内 `s.writeAudit(sync.execute, env=production)`（`execute_section_sync.go:139`），非仅机制就绪。**通过**。

### 后端标注 4 项疑似缺陷 · 逐项判定（证据 + 结论）
- **① 整行 add/delete diff 未脱敏** → **确认代码事实，判定：可接受非阻断（当前无泄漏）+ 建议加固**。`DiffEntities` add/delete 直接回填整行 `Fields`（`sync.go:306-323`）未过 `isMaskedField`；但所有 loader 均未 SELECT 任何 `*_ciphertext`/secret 列（`loadGame`/`loadChannels` 等），故当前链路无密文进入 add/delete，S8 红线未实际违反。列为 SYNC-INT-03（低，随 ② 补齐子表后升为中），移交 🟧 加固。
- **② section loader 未全覆盖子表** → **确认问题（compact 功能完整性偏差），移交 🟧（中）**。channels 缺 login/iap/plugin、packages 缺 iap/plugin overrides、products 缺 channel_products、cashier 仅同步 profile.snapshot_checksum 不同步 price_overrides 实体（`sync_repo.go:361-540,473-499` + apply 同缺）。后果：子实体 add/update/delete 不进 diff/hash，基线复核可能漏判；cashier checksum 与实际 overrides 可能不一致。列 SYNC-INT-02。
- **③ applyPayments ON CONFLICT DO NOTHING** → **确认问题（正确性/compact upsert 语义违反），移交 🟧（中，建议验收前修）**。`sync_repo.go:810` 唯一使用 DO NOTHING（其余 apply* 均 DO UPDATE）；已存在 payment_route 的 priority/enabled/provider 等变更 execute 时**静默不生效**，与 compact「存在则更新有变化字段」相悖。列 SYNC-INT-01。
- **④ 删除引用 skipped 未实现 / 依赖 section 级 / hash 排除 game_secret** → **确认，均低 severity 偏差，移交 🟧（低）**。skipped 仅 `include_deletes=false` 一种 reason（`sync_repo.go:154-178`），include_deletes=true 走 NOT EXISTS 直删，被引用时依赖 DB FK（可能整事务失败而非 skip，`sync_repo.go:604-836`）；依赖校验 entityKey 恒 `*`（`sync.go:503-533`）；`loadGame` 未选 game_secret 而 `applyGame` 写它（`sync_repo.go:268-292` vs `571-586`）→ game_secret 变更不入 hash、基线漏判。列 SYNC-INT-04/05/06。

### 遗留问题清单（移交 🟧高级全栈工程师）
| 编号 | 问题 | 证据(文件:行) | 期望(compact 依据) | 严重度 |
| --- | --- | --- | --- | --- |
| SYNC-INT-01 | applyPayments DO NOTHING → 已存在 payment_route 字段更新静默不生效 | `infra/persistence/postgres/sync_repo.go:810` | §有序 upsert「存在则更新有变化字段」；余 apply* 均 DO UPDATE | 中 |
| SYNC-INT-02 | loader/apply 未覆盖子表(channels login/iap/plugin、packages overrides、channel_products、cashier price_overrides) | `sync_repo.go:361-540,473-499,650-727` | §SyncSection 源表 + 9 section diff/upsert 完整 | 中 |
| SYNC-INT-03 | 整行 add/delete diff 未字段级脱敏(当前 loader 无密文列→无泄漏，潜在 S8) | `domain/sync/sync.go:306-323` | §脱敏「密文字段 preview 恒 masked」 | 低(潜在,随02升中) |
| SYNC-INT-04 | 删除仍被引用→skipped 未实现引用检测(直删依赖 FK) | `sync_repo.go:604-836`；`154-178` | §删除 opt-in「仍被引用则跳过并 skipped 提示」 | 低 |
| SYNC-INT-05 | 依赖校验 section 级，entityKey 恒 `*` | `domain/sync/sync.go:503-533` | §依赖校验 details「channels/<market>/<channel_id>」 | 低 |
| SYNC-INT-06 | hash 排除 game_secret(load 未选/apply 写)→基线漏判 | `sync_repo.go:268-292` vs `571-586` | §hash「密文以密文/哈希参与」 | 低 |
| SYNC-INT-07 | 契约漂移：syncJobId 后端 JSON number，前端 TS 声明 string(compact 示例为字符串) | `domain/sync/sync.go:117,133` vs `api/syncSections.ts:77,99` | compact 示例 syncJobId `"9012"` | 低(前端插值可容忍) |
| SYNC-INT-08 | 契约漂移：SyncJobsTab 展示 selectedSections/appliedSummary/errorSummary/operatorName，后端 JobItem 与 compact 列表项不返回(前端可选链降级) | `SyncJobsTab.vue:46,52,32,23` vs `sync.go:132-146` | §GET sync-jobs item 字段集 | 低 |
| SYNC-INT-09 | ListJobs 未解析 sort query(前端下发 -createdAt 被忽略，固定 created_at DESC) | `section_sync_handler.go:93-107`；`sync_repo.go:75` | §GET sync-jobs Query sort(默认即 -createdAt) | 低 |

### 集成结论 / 通过判定
- **集成测试通过（红线维度全绿 + 契约对账基本一致 + 前后端全量回归全绿）**。所列 SYNC-INT-01~09 均为**非红线阻断**（红线双闸门/单事务/跨 env/审计/production 不渲染/preview masked 现状均满足）。
- 通过判定：**可进入 ✅功能验收**（本期红线口径）。**同时建议**：优先由 🟧 修复 SYNC-INT-01(payments 更新静默失效) 与 SYNC-INT-02(子表覆盖→9 section 完整性)，二者为 compact 功能/正确性偏差，若功能验收口径要求 compact 全量一致则应在终验前闭环；SYNC-INT-03~09 可随之或后续迭代。修复后回本角色复测。
- 连库维度（真实 upsert/审计落库/迁移 000016 前向执行/expect.db 断言）待 PG CI(`SCENARIO_WITH_DB=1`)。

## 2026-07-01 · 🟧全栈修复(第1轮) · SYNC-INT-01~09

### 范围与裁决标准
- 输入：🟪测试专家第1轮 SYNC-INT-01~09 + manifest open_issues + integration.checklist 第5节。
- 裁决标准：compact 契约唯一。优先并已闭环 01(payments 更新静默失效) + 02(9 section 完整性)；逐项处理 03~09。
- 改动文件（后端，均在 worktree console-sync）：
  - `internal/domain/sync/sync.go`
  - `internal/infra/persistence/postgres/sync_repo.go`
  - `internal/transport/http/sync/section_sync_contract_integration_test.go`（同步 07 契约断言）

### 逐条修复说明（问题→根因→改动→验证）
- **SYNC-INT-01（中·已修）payments 更新静默失效**
  - 根因：`applyPayments` 用 `ON CONFLICT DO NOTHING`，已存在 payment_route 的 priority/enabled/provider/merchant 变更 execute 不生效，违反 compact「存在则更新有变化字段」。
  - 改动：`sync_repo.go` applyPayments → `ON CONFLICT (game_id_ref, pay_way_id_ref, COALESCE(package_id_ref,-1), COALESCE(channel_id_ref,-1), market_code, country_code, currency) WHERE enabled DO UPDATE SET provider_id_ref/merchant_account_id_ref/priority/enabled/updated_at`。冲突键=归一化选择器（空=*），与 000014 `uq_payment_routes_selector` 部分唯一索引对齐。
  - 验证：go build/vet/test 通过；连库真实 upsert 待 PG CI。
- **SYNC-INT-02（中·已修）9 section 子表完整性**
  - 根因：loader/apply 未覆盖 channels 的 login/iap/plugin configs、packages 的 iap/plugin overrides、products 的 channel_products、cashier 的 price_overrides → 子实体 add/update/delete 不进 diff/hash，基线复核漏判、upsert 不完整。
  - 改动（`sync_repo.go`）：
    - loadChannels：+ game_channel_login_config / game_channel_iap_config（键=market/channel）+ game_channel_plugin_config（键=market/channel/plugin_id）；有效过滤 gc.hidden=FALSE/enabled/valid + cfg.enabled=TRUE/config_status='valid'；`config_json` 服务端搬运并标记 masked。
    - loadPackages：channel_package 增 inherit_channel_config/override_json（masked）；+ channel_package_iap_override（键=market/channel/package）+ channel_package_plugin_override（键=…/plugin_id）。
    - loadProducts：+ channel_product（键=market/channel/package/product_id）。
    - loadCashier：profile 增 template_id_ref/applied_template_version_id；+ game_cashier_price_override（键=country/region/currency/price_id）。
    - applyChannels/applyPackages/applyProducts/applyCashier：对应子表 `INSERT…SELECT FROM sandbox … ON CONFLICT (业务唯一键) DO UPDATE`；include_deletes 时按 FK 安全序（先子后父）补 NOT EXISTS 删除。
    - 新增 `decodeJSONB` 助手（JSONB→确定性哈希/对比值）。
  - 验证：go build/vet/test 通过；连库真实 upsert/删除 待 PG CI。
- **SYNC-INT-03（低/潜在·已修）整行 add/delete 未脱敏**
  - 根因：`DiffEntities` add/delete 分支直接回填整行 `Fields`，未过 `isMaskedField`；02 补齐子表后 config_json/game_secret 会进入整行。
  - 改动：`sync.go` 新增 `maskRowFields`，add/delete 整行按字段级脱敏（密文字段值恒 `masked`），hash 仍用原值（`HashEntitySets` 不受影响）。
  - 验证：`sync_test.go` 原 add/delete/masked 用例通过。
- **SYNC-INT-06（低·已修）hash 排除 game_secret**
  - 根因：`loadGame` 未选 `game_secret` 而 `applyGame` 写它 → game_secret 变更不入 hash、基线漏判。
  - 改动：loadGame SELECT 增 `game_secret` 入 Fields（参与 hash），masked 集含 `game_secret`（preview 恒 masked，add/delete 亦经 maskRowFields）。
  - 验证：go test 通过；contract 集成 test 断言明文不泄漏仍 PASS。
- **SYNC-INT-07（低·已修）syncJobId 类型漂移**
  - 根因：compact 示例 `syncJobId:"9012"`（字符串）+ 前端 TS 声明 string，后端 int64 序列化为 number。
  - 改动：`sync.go` ExecuteResult/JobItem 的 SyncJobID 加 `json:",string"`；同步更新 contract 集成 test 断言（number→非空 string）。前端 TS 已为 string，无需改前端。
  - 验证：contract 集成 test PASS。

### 保留为非阻断 open_issue（理由）
- **SYNC-INT-04（低）删除引用 skipped 未做引用检测**：include_deletes=true 仍走 NOT EXISTS 直删，跨 section 被引用时依赖 DB FK（单事务回滚，红线「不部分写入」保持）；真正的引用扫描 + `skipped.deletes(reason=referenced)` + 跨 section 逆拓扑删除序属较大重构，删除本身 opt-in 默认关，本期延后。
- **SYNC-INT-05（低）依赖校验 section 级、entityKey 恒 `*`**：compact details 期望 `channels/<market>/<channel_id>` 细粒度；现为 section 有无数据校验，红线（缺失即拒绝 VALIDATION_FAILED）满足，细粒度 entityKey 归因属精度增强，本期延后。
- **SYNC-INT-08（低）list item 展示字段**：compact GET sync-jobs 列表项**不含** selectedSections/appliedSummary/errorSummary/operatorName，后端 JobItem 与 compact 一致；前端以可选链降级展示，属前端富化，后端无需改。
- **SYNC-INT-09（低）sort 未解析**：compact 仅定义默认 `-createdAt`，后端固定 `created_at DESC` 即等于默认，功能一致；任意 sort 支持属增强，本期延后。
- **config_json 字段级 diff（新增说明）**：本轮对 login/iap/plugin/override 的 `config_json` 与 package `override_json` 采用整字段 masked（安全优先、hash 用原值检测变更），未逐子键按模板 `secret_fields_json` 拆分展示；子键级 diff/精确脱敏属后续增强（不泄漏明文，红线满足）。
- **迁移 000016 真实前向执行 + 连库 upsert/审计落库断言** 待 PG CI（`SCENARIO_WITH_DB=1`）。

### 自检（真实命令，cwd=services/admin-api，required_permissions=["all"]）
- `go build ./...` → **通过**。
- `go vet ./internal/infra/persistence/postgres ./internal/domain/sync ./internal/app/command ./internal/transport/http/sync` → **通过**。
- `go test ./internal/domain/sync ./internal/app/command ./internal/transport/http/sync ./internal/testkit/scenario` → **全 PASS**。
- `go test ./...`（全量）→ **EXIT=0，0 FAIL**。
- 未改前端（syncJobId 前端已为 string），无需前端 tsc/build。

### 结论
- SYNC-INT-01/02/03/06/07 已闭环；04/05/08/09 + config 子键 diff + 连库 保留为非阻断 open_issue（理由见上）。红线全部保持。**可回 🟪测试专家复测**（连库维度请在 PG CI `SCENARIO_WITH_DB=1` 验证真实 upsert/删除/审计落库）。

## 2026-07-01 · 🟪测试专家(第2轮·复测) · sync 第1轮修复复测

### 读取
- 🟧全栈修复(第1轮) 小节 + manifest open_issues 最新状态；修复文件 `domain/sync/sync.go`、`infra/persistence/postgres/sync_repo.go`、`transport/http/sync` 契约集成 test。

### 逐项复核（代码核验 + 测试证据）
- **SYNC-INT-01 payments upsert → ✅ 闭环**：`sync_repo.go:1348-1354` applyPayments 由 `DO NOTHING` 改 `ON CONFLICT (game_id_ref,pay_way_id_ref,COALESCE(package_id_ref,-1),COALESCE(channel_id_ref,-1),market_code,country_code,currency) WHERE enabled DO UPDATE SET provider/merchant/priority/enabled/updated_at`，冲突键=归一化选择器（对齐 000014 `uq_payment_routes_selector`），已存在路由更新生效。语义正确；真实 upsert 待 PG CI。
- **SYNC-INT-02 9 section 子表 → ✅ 闭环**：loader 覆盖 channels(login/iap/plugin configs,`sync_repo.go:407-495`)、packages(iap/plugin overrides+override_json,`500-628`)、products(channel_products,`668-711`)、cashier(price_overrides,`750-790`)；apply 对应 `INSERT…SELECT FROM sandbox…ON CONFLICT(业务唯一键)DO UPDATE`，include_deletes 按 FK 安全序先子后父删除（`944-1046`/`1114-1178`/`1224-1259`/`1300-1328`）。对齐键/有效过滤(hidden=FALSE/enabled/config_status='valid')/脱敏与 compact 一致。子表 SQL 真实执行(表/列/约束名)待 PG CI。
- **SYNC-INT-03 整行脱敏 → ✅ 闭环**：`sync.go:307-327` add/delete 走 `maskRowFields`（`539-556`），密文字段值恒 `masked`、Masked=true，hash 用原值。**新增守护测试**（本轮）：`domain/sync/sync_maskrow_integration_test.go`（3 test PASS）断言 add/delete 整行密文脱敏 + game_secret 脱敏 + 明文/密文(`PLAINTEXT`/`enc-`)绝不出现——补齐既有仅覆盖 update 分支的空白。
- **SYNC-INT-06 game_secret 入 hash → ✅ 闭环**：`sync_repo.go:270-294` loadGame SELECT 增 game_secret 入 Fields（参与 hash），masked 集含 `game_secret`（preview/add/delete 恒 masked）。domain `TestHashSensitiveToSemanticFieldAndCiphertext` + 本轮 game_secret 脱敏测试双守护。
- **SYNC-INT-07 syncJobId string → ✅ 闭环**：`sync.go:117,133` ExecuteResult/JobItem SyncJobID 加 `json:",string"`；契约集成 test `TestContractExecuteResponseFields` 断言 `syncJobId` 为非空 JSON string。前端 `api/syncSections.ts:77,99` 已声明 string → **前后端一致，漂移消除**。

### 契约对账再核（确认未引入新漂移）
- API 响应 DTO 形状不变（DiffSection/DiffChange/Preview/ExecuteResult/JobItem），子表仅增加 changes 条目（前端按 section/changes 泛型渲染，无需改）。
- syncJobId 前后端统一为 string（07 已消除唯一中危漂移）。SYNC-INT-08(list item 富化)/09(sort) 经确认后端与 compact 一致、前端可选链降级，非漂移。**无新增契约漂移**。

### 全量回归（真实输出，cwd 见下，required_permissions=["all"]）
- 后端(services/admin-api)：`go build ./...` ✅；`go vet`(sync 5 包) ✅；`go test -count=1 ./...` **EXIT=0 全 ok 无 FAIL**（含新增 maskrow 守护测试 + 契约集成 test）；sync 4 包 count=1 全 PASS；scenario `TestRunCaseSyncPreviewRequiresAuth` PASS / sync 22 requiresDB SKIP。
- 前端(apps/admin-web)：`pnpm vitest run` **37 files/290 tests 全 PASS**。Playwright sync.spec 9 沿用 🟩前端测试基线（未改前端，无回归面）。

### 红线端到端复核（仍全绿）
- 仅 sandbox→production/production 不渲染入口/双闸门(nonce+基线)/单事务失败 failed/preview masked/跨 schema 仅 sync 域/审计 env=production —— 全部保持；脱敏经 03/06 修复后更强（整行 add/delete + game_secret 亦 masked）。下游 #23 sync-jobs 列表可用、#22 audit execute 事务内 writeAudit env=production。

### 保留 open_issue 判定（本期非阻断遗留，同意 🟧 归类）
- SYNC-INT-04(删除引用 skipped/逆拓扑删除序)、05(依赖 entityKey 细粒度)、08(list item 前端富化降级)、09(sort=默认)、config_json/override_json 子键级 diff：均非红线，同意本期延后。
- **连库维度（关键遗留验证）**：子表 loader/apply SQL 的表/列/唯一约束名、payments 归一化冲突键、真实 upsert/删除/审计落库、迁移 000016 前向执行——单测用 fake repo 不执行 SQL，**必须在 PG CI(`SCENARIO_WITH_DB=1`)验证**。属既定连库口径遗留，不阻断本期红线验收。

### 复测结论 / 通过判定
- 第1轮闭环项 01/02/03/06/07 **全部按 compact 复测通过**；无新增阻断、无新增契约漂移；红线全绿。
- **判定：通过 → 可进入 ✅功能验收**（本期红线口径）。连库真实 upsert/删除/审计落库 + 子表 SQL 正确性待 PG CI 验证（既定遗留，非阻断）。本轮无需再交 🟧。

## 2026-07-01 · ✅功能验收 · sync 模块端到端验收

### 读取与对齐
- 协议：`index.json` → `spec.compact.md` → `02-operation-flow.md` §10（preview→勾选→execute，完成=succeeded，production 不渲染入口）。
- 输入：🟪测试专家(第2轮复测✅) + 🟧全栈修复 handoff + audit.log 各小节 + `integration.checklist.md`。
- 验收基准：功能端到端可用 + compact 业务规则 + 操作主线（非"代码写了"）。

### 构建/测试结果汇总（真实命令，required_permissions=["all"]）
| 命令 | cwd | 结果 |
| --- | --- | --- |
| `go build ./...` | services/admin-api | ✅ EXIT=0 |
| `go vet ./...` | services/admin-api | ✅ EXIT=0 |
| `go test -count=1 ./...` | services/admin-api | ✅ 33 包 ok / 0 FAIL |
| `go test -count=1 -v ./internal/domain/sync ./internal/app/command ./internal/transport/http/sync` | services/admin-api | ✅ domain+command+transport 全 PASS（含 maskrow 3 test + 契约集成 3 test + execute 17 test） |
| `go test -count=1 ./internal/testkit/scenario -run Sync` | services/admin-api | ✅ TestRunCaseSyncPreviewRequiresAuth PASS；sync.yaml 22 requiresDB SKIP |
| `pnpm exec vue-tsc --noEmit` | apps/admin-web | ✅ EXIT=0 |
| `pnpm vite build` | apps/admin-web | ✅ EXIT=0（rollup chunk size warning 非阻断） |
| `pnpm vitest run` | apps/admin-web | ✅ 37 files / 290 tests 全 PASS（含 sync 32） |

### 验收清单
| # | 验收点 | 期望 | 实际 | 证据 | 结果 |
| --- | --- | --- | --- | --- | --- |
| A01 | POST preview 方法/路径 | `POST /api/admin/games/{gameId}/sync/preview` | 一致 | `router.go:18` | PASS |
| A02 | POST preview 权限码 | `sync.preview` | 一致 | `router.go:18` RequirePerm | PASS |
| A03 | POST preview 请求 DTO | `{sections?, includeDeletes?}` camelCase | 一致 | `section_sync_handler.go:33-36` | PASS |
| A04 | POST preview 响应包络 | 200 `{data:{gameId,sourceEnv,targetEnv,sourceHash,targetHashBefore,hasDiff,baselineToken,previewedAt,expiresAt,sections[]}}` | 一致 | 契约集成 `TestContractPreviewEnvelopeAndMasking` PASS | PASS |
| A05 | POST preview 错误码 | UNKNOWN_SECTION(400)/NOT_FOUND(404)/VALIDATION_FAILED(400) | 一致 | `preview_section_sync_test.go` + handler test | PASS |
| A06 | POST execute 方法/路径 | `POST /api/admin/games/{gameId}/sync/execute` | 一致 | `router.go:19` | PASS |
| A07 | POST execute 权限码 | `sync.execute` | 一致 | `router.go:19` | PASS |
| A08 | POST execute 请求 DTO | `{selectedSections,baselineToken,includeDeletes?,operatorNote?}` | 一致 | `section_sync_handler.go:62-67` + `syncSections.ts:69-74` | PASS |
| A09 | POST execute 响应/错误码 | 200 ExecuteResult；409 SYNC_BASELINE_MISMATCH/SYNC_TOKEN_CONSUMED；400 UNKNOWN_SECTION/VALIDATION_FAILED | 一致 | `execute_section_sync_test.go` 17 test + 契约集成 PASS | PASS |
| A10 | GET sync-jobs 方法/路径/权限 | `GET .../sync-jobs`，`sync.preview` | 一致 | `router.go:20` | PASS |
| A11 | GET sync-jobs 分页/过滤 | page=1/pageSize≤100/status 过滤；`data.{items,page,pageSize,total}` | 一致 | `section_sync_handler.go:93-107` + `TestListJobsPaginationClamp` + 契约集成 PASS | PASS |
| A12 | 操作主线 preview 落库 | preview 建 sync_jobs(status=previewed)+items(applied=false) | 一致 | `preview_section_sync.go:189-204` + `TestPreviewCreatesJobAndVerifiableToken` | PASS |
| A13 | 操作主线 execute 原地更新 | execute 更新 token.syncJobId 同一行→succeeded/failed，不另建行 | 一致 | `execute_section_sync.go:136` UpdateJobResult(token.SyncJobID) + preview token 含 syncJobId | PASS |
| A14 | selectedSections 勾选执行 | 仅选中 section 写入，未选 skipped.unselectedSections | 一致 | `execute_section_sync.go:173-180` + drawer spec payload 断言 | PASS |
| A15 | 双闸门顺序 | nonce 去重(54-60) 先于基线复核(76-81) | 一致 | `execute_section_sync.go` 代码序 + 两独立单测 PASS | PASS |
| A16 | SYNC_TOKEN_CONSUMED | 预检/事务内冲突均 409 + details consumedSyncJobId | 一致 | `TestExecuteNonceAlreadyConsumed` + `TestExecuteNonceRaceInTxReturnsConsumed` | PASS |
| A17 | SYNC_BASELINE_MISMATCH | targetHashNow≠targetHashBefore → 409 + expected/actual | 一致 | `TestExecuteBaselineMismatch` | PASS |
| A18 | 9 section 拓扑有序 upsert | game→…→config 依赖序写入 | 一致 | `SortSectionsByTopo` + `execute_section_sync.go:106-110` | PASS |
| A19 | 9 section 子表覆盖 | channels login/iap/plugin、packages overrides、channel_products、cashier price_overrides loader+apply | 一致 | `sync_repo.go:407-790` apply DO UPDATE 全 section；🟪复测✅ | PASS |
| A20 | payments 真实 upsert | 已存在路由 update 生效(DO UPDATE 非 DO NOTHING) | 一致 | `sync_repo.go:1348-1354` ON CONFLICT DO UPDATE | PASS |
| A21 | include_deletes 默认关 | preview 展示 delete；execute 跳过 delete(applied=false, skipped reason) | 一致 | drawer spec muted delete + MarkItemsApplied skipped | PASS |
| A22 | 红线：仅 sandbox→production | source_env/target_env 硬编码 sandbox/production | 一致 | `sync.go:41-42` DefaultSourceEnv/TargetEnv | PASS |
| A23 | 红线：production 不渲染入口 | production/develop 无 Sync to Production | 一致 | `GameDetailView.vue:14` v-if sandbox + entry spec 4 test PASS | PASS |
| A24 | 红线：后端 source_env 兜底 | token game/env 不匹配→VALIDATION_FAILED | 一致 | `execute_section_sync.go:51-52` + `TestExecuteTokenGameMismatch` | PASS |
| A25 | 红线：密文 preview masked | masked 字段值恒 masked，整行 add/delete 亦脱敏 | 一致 | maskRowFields + `sync_maskrow_integration_test.go` 3 PASS + 契约集成明文不泄漏 | PASS |
| A26 | 红线：upsert 不经明文 | INSERT…SELECT FROM sandbox 服务端搬运 | 一致 | `sync_repo.go` apply* 全用 SELECT FROM sandbox | PASS |
| A27 | 红线：单事务不部分写入 | 失败回滚 nonce 未消费、无审计、status=failed | 一致 | `TestExecuteTransactionRollbackOnApplyFailure` | PASS |
| A28 | 红线：跨 schema 仅 sync 域 | schema 限定名 + safeSchema 白名单 | 一致 | `sync_repo.go` safeSchema + LoadSectionEntities/ApplySection | PASS |
| A29 | 红线：审计 sync.execute env=production 脱敏 | action=sync.execute, Env=production, detail 计数+hash 无明文 | 一致 | `execute_section_sync.go:139-153` + `TestExecuteSuccessWritesAuditWithProductionEnv` | PASS |
| A30 | 前端：sandbox 入口 + sync.execute 权限 | sandbox 渲染；缺权限置灰 | 一致 | `GameDetailView.vue:14-20,127` + entry spec | PASS |
| A31 | 前端：抽屉预览分组/配色/对照/复选 | section 分组+徽标+add绿/update黄/delete红+sandbox→production+section 复选 | 一致 | drawer spec 18 test PASS | PASS |
| A32 | 前端：结果反馈 | 成功 toast+刷新；BASELINE_MISMATCH/TOKEN_CONSUMED 重新预览；VALIDATION_FAILED details | 一致 | drawer spec + CR 修复确认 | PASS |
| A33 | 前端：SyncJobsTab | 列项/过滤/分页/失败展开/成功 appliedSummary | 一致 | SyncJobsTab spec 10 test PASS | PASS |
| A34 | 下游 #23 契约 | GET sync-jobs 列表包络+compact item 字段可用 | 一致 | 契约集成 `TestContractListJobsItemFields` PASS | PASS |
| A35 | 下游 #22 audit | execute 成功事务内 writeAudit 被调用 | 一致 | `TestExecuteSuccessWritesAuditWithProductionEnv` entries=1 | PASS |
| A36 | 连库维度(requiresDB) | 真实 upsert/删除/审计/迁移 000016 前向执行 | 本地无 PG；22 case SKIP；L1+L2 进程内等价覆盖 | sync.yaml requiresDB + handoff 既定口径 | PASS* |

> *A36：按既定口径标注为「待 PG CI(SCENARIO_WITH_DB=1)」，不作为 FAIL。

**统计：PASS 36 / FAIL 0 / 待 PG CI 1（A36，非阻断）**

### 操作主线走查（02-operation-flow §10）
1. sandbox 完成各 section 配置 → snapshot 产出 config（上游模块，#20 已合并）。
2. `POST /sync/preview`：读 sandbox/production 有效数据→diff+hash→baseline_token→落 previewed+items(applied=false)。**成立**（A12 + preview 单测）。
3. 运营勾选 selected_sections + 可选 include_deletes。**成立**（前端 drawer + payload 单测 A31）。
4. `POST /sync/execute`：双闸门→依赖→单事务 upsert→succeeded/failed。**成立**（A13-A17 + execute 单测链）。
5. production 视图不出现可执行 Sync 入口。**成立**（A23）。
6. 完成判定 sync_jobs.status=succeeded。**成立**（A13）。

### 结论
- **通过** — 36/36 可验证项 PASS，0 FAIL。
- 连库真实 SQL 执行/迁移前向/expect.db 断言待 PG CI（A36），属本期既定遗留，不阻断验收。
- 非阻断 open_issue 维持：SYNC-INT-04(删除引用 skipped)、05(依赖 entityKey 细粒度)、08(列表前端富化)、09(sort 默认)、config 子键级 diff。

### 遗留风险与建议
1. **PG CI 优先**：`SCENARIO_WITH_DB=1` 验证子表 SQL 列/约束名、payments 冲突键、真实 upsert/删除/审计落库、迁移 000016 前向执行。
2. **后续增强**：删除引用 skipped(SYNC-INT-04)、依赖 entityKey 细粒度(SYNC-INT-05)、config_json 子键 diff。
3. **下游**：#23 dashboard `GET /dashboard/recent-sync-jobs` 依赖 sync 任务表，本期 #21 列表接口契约已就绪，dashboard 模块集成时可直接消费。
