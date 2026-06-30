# audit · 执行日志（仅供人类审计）

> 各角色追加。总 Agent 不读本文件。

## [orchestrator] 模块启动
- 闸门判断：depends_on=common ✅；lane=audit-surface 单模块、无在制冲突；可与其他 lane 并行 → 允许开工。
- 创建 worktree：`../console-audit`（分支 `codex/audit`）。
- 初始化 artifacts：module.manifest.json / integration.checklist.md / audit.log.md。
- 后端、前端两车道并行启动。

## [backend] audit 模块实现（本次）
- 新增迁移：`services/admin-api/migrations/000007_audit_indexes.up.sql` + `.down.sql`，仅追加 compact 指定 6 个索引，幂等。
- 新增值对象：`internal/domain/common/audit.go`（`AuditLog` / `AuditDetail` / `AuditRequestMeta`）。
- 新增应用服务：`internal/app/audit/`（`AuditService.Write/Query`、递归脱敏、分页查询、ctx 去重标记）。
- 新增仓储：`internal/infra/persistence/postgres/audit_repository.go`（仅 `Insert` + `Query`，`Query` 含过滤、分页、排序、join `admin_users`）。
- 新增 transport：`internal/transport/http/audit/handler.go` + `router.go`，提供：
  - `GET /api/admin/audit-logs`（列表）
  - `GET /api/admin/audit-logs/{id}`（详情）
- 重写审计中间件：`internal/transport/http/middleware/middleware.go`
  - 注入 ctx 字段（actorID/env/requestID/ip/userAgent/method/path）
  - 非 GET/HEAD/OPTIONS 且 2xx 时兜底写审计
  - 显式写入后通过 ctx 标志去重
  - 失败仅结构化日志，不阻断主流程
- wiring 接入：`internal/transport/httpserver/admin_wiring.go`
  - 注入 `AuditService` 到 admin/game/channel/account-auth 现有 `AuditSink`
  - 注册 audit 路由
  - 所有既有 router `Audit(...)` 签名升级并透传 `audit writer`
- 自检：
  - `cd services/admin-api && go build ./...` ✅
  - `cd services/admin-api && go vet ./...` ✅

## [frontend] 审计页实现（2026-06-29）
- 新增 `apps/admin-web/src/api/modules/audit.ts`：按 compact 契约定义 `AuditLogItem` / `ListAuditLogsQuery` / `AuditLogFacets`，接入 `GET /api/admin/audit-logs`、可选 `/{id}`、`/facets`，并明确 `id`/`actorId` string 化。
- 重写 `apps/admin-web/src/views/audit/AuditView.vue`：实现提交式 FilterBar（env/action/resourceType/operator/timeRange/keyword）、createdAt 排序、分页保留过滤、动作/环境标签、资源标识复制、详情抽屉 before/after 对照与 raw JSON。
- 状态态补齐：骨架屏、空态（含重置过滤）、错误态（非 403 可重试）、403 权限降级态；页面保持全只读（无写/删按钮）。
- 构建验证：`pnpm run build` 因仓库既有测试类型依赖缺失（`@vue/test-utils`）失败；`pnpm vite build` 成功（产物含 `AuditView` chunk）。

## [frontend-cr] 审计页 Code Review（2026-06-29，Composer 2.5）
- **结论：通过**（无阻断项，3 处小修已落地）。

### 核对表（compact 前端要点 → 证据）
| 要点 | 已实现 | 一致 | 证据 |
| --- | --- | --- | --- |
| FilterBar 提交式 env/action/resourceType/operator/timeRange/keyword | ✅ | ✅ | `AuditView.vue:9-67,506-518` draft/applied 分离 + 查询按钮 |
| 切页/排序保留过滤 | ✅ | ✅ | `AuditView.vue:467-527` reload 读 appliedFilters；sort-change 仅重置 page |
| env 默认不限 + EnvironmentBadge 常驻 | ✅ | ✅ | `AuditView.vue:331-338,6` 空 env 不传参；顶部 Badge |
| Table createdAt 倒序 + 本地/hover UTC | ✅ | ✅ | `AuditView.vue:94-102,326,529-548` default-sort + tooltip |
| 操作者 displayName 兜底 / System | ✅ | ✅ | `AuditView.vue:551-558` actorId `"0"` → System |
| 动作 Tag 动词色系 | ✅ | ✅ | `AuditView.vue:566-574` create绿/delete红/publish蓝/execute橙/hide灰 |
| 资源标识可复制 | ✅ | ✅ | `AuditView.vue:115-121,582-588` |
| 环境 Tag production 高亮 | ✅ | ✅ | `AuditView.vue:576-579` production→danger |
| 详情抽屉 before/after 三态 + changed 高亮 | ✅ | ✅ | `AuditView.vue:172-208,377-397` 仅 after/before/左右对照 + switch |
| 密文 `******` 不解密 | ✅ | ✅ | `AuditView.vue:615-623` 仅 normalized masked/****** |
| extra + request 折叠 + raw JSON | ✅ | ✅ | `AuditView.vue:211-231` el-collapse |
| API id/actorId string + 查询参数 + 包络 | ✅ | ✅ | `audit.ts:36-61,80-89`；`http.ts:106-108` unwrap data |
| 加载骨架 / 空态重置 / 错误重试 / 403 降级 | ✅ | ✅ | `AuditView.vue:69-88,138-141,363,489-500` |
| 菜单 hasPerm audit.read + 路由守卫 | ✅ | ✅ | `routes.ts:56`；`router/index.ts:29-31`；`permission.ts:41-48` |
| 全只读无写删按钮 | ✅ | ✅ | 全文无 v-perm write / 无 POST/DELETE |
| 分页 total>pageSize | ✅ | ✅ | `AuditView.vue:145-154` |
| 01 §5 抽屉式交互 | ✅ | ✅ | `AuditView.vue:159` el-drawer |

### 问题清单
**阻断：** 无。

**建议（未改）：**
- 可选深链 `/audit?resourceType=&resourceId=` 预填过滤（compact 可选增强）。
- `operatorKeyword` 查询参数 UI 未暴露（compact 标注本期默认不做）。
- 静态 KNOWN_ACTIONS/RESOURCE_TYPES 为子集，facets 已兜底合并。

### CR 已直接修复
1. `@row-click="openDetail"` — 对齐 compact「点击行开抽屉」。
2. 403 态隐藏 FilterBar — 对齐「整页降级」。
3. 抽屉 `v-loading` 替代底部冗余 skeleton；移除 `formatDetailValue` 字段名启发式脱敏，仅展示后端 masked 值。

## [backend-cr] audit 后端 Code Review（2026-06-29，Composer 2.5）
- **结论：通过**（无阻断项，1 处小修已落地）。

### 核对表（compact 后端要点 → 证据）
| 要点 | 已实现 | 一致 | 证据 |
| --- | --- | --- | --- |
| 迁移 000007 仅 6 索引、幂等、down 逆序 | ✅ | ✅ | `000007_audit_indexes.{up,down}.sql` |
| audit_logs 7 列未改 | ✅ | ✅ | `000001_init.up.sql:471-479` |
| AuditLog/AuditDetail/AuditRequestMeta 字段与 omitempty | ✅ | ✅ | `domain/common/audit.go:6-34` |
| AuditService.Write 五步（ctx/脱敏/规整/Insert/markWritten） | ✅ | ✅ | `app/audit/service.go:94-146,215-257` |
| AuditService.Query WHERE+分页+created_at/id 排序+join | ✅ | ✅ | `audit_repository.go:41-163`；`service.go:148-169` |
| actor_id=0 系统占位 operator | ✅ | ✅ | `handler.go:149-160` |
| 仓储仅 Insert/Query 无 Update/Delete | ✅ | ✅ | `audit_repository.go:19-110`；`service.go:74-77` |
| GET /audit-logs 分页包络 camelCase string id | ✅ | ✅ | `handler.go:23-54` |
| GET /audit-logs/{id} NOT_FOUND | ✅ | ✅ | `handler.go:57-80`；`service.go:160-162` |
| 权限 audit.read + 401/403 错误码 | ✅ | ✅ | `router.go:19-20`；`middleware.go:32-71` |
| from>to VALIDATION_FAILED | ✅ | ✅ | `handler.go:127-128`；`service.go:149-151` |
| 中间件 ctx 注入 + 写识别 + 去重 + 失败日志 | ✅ | ✅ | `middleware.go:103-149` |
| wiring AuditService + SinkAdapter + 路由注册 | ✅ | ✅ | `admin_wiring.go:83-119` |
| facets 可选接口 | ❌ | — | 标注可选，未实现 |

### 问题清单
**阻断：** 无。

**建议（未改）：**
- 可选 `GET /api/admin/audit-logs/facets` 未实现（compact 标注可选）。
- 历史 `AuditSink` 仍无错误返回、不传递 `SecretKeys`；高危操作同事务回滚待逐模块增强。
- 中间件链实际为 recover→EnvContext→Authn→RequireBackend→Audit→RequirePerm（与 compact 图示 env/权限顺序略有差异，功能等价且 integration.checklist 已记录）。
- 缺少 audit 模块专属单元/集成测试。

### CR 已直接修复
1. 中间件兜底 `inferFallbackAudit` 对 `resourceID` 截断至 128 字符，避免超长 path 导致 INSERT 失败（`middleware.go:196-199`）。

## [backend-test] audit 后端测试落地（2026-06-29，Cursor Auto）
- **结论：通过** — 新增 4 个就近单测文件 + 1 个场景 manifest + 1 个 fixtures；`go test ./...` 全绿，新增 51 个单测子用例全部通过，无回归。

### 测试清单（文件 → 覆盖对象）
| 文件 | 层 | 覆盖对象（红线粗体） |
| --- | --- | --- |
| `internal/app/audit/service_test.go` | L1（无 IO，fakeRepo） | **递归脱敏**（before/after/extra 嵌套 map+slice、大小写不敏感、masked）、changed 规整（去空白/去重/排序/空→nil）、summary 缺省、**env 取值**（显式>ctx>runtime）、**actor 取值**（显式>ctx meta>AuthContext、actor=0 系统、负数→0）、request 元补全、**ctx 去重标志**（Write 后 IsWritten=true；Insert 失败不置位以便中间件兜底）、Query from>to 校验、分页钳制（page<1→1、pageSize 0→20/500→100/=100）、按 id NOT_FOUND、SortDesc 透传 |
| `internal/infra/persistence/postgres/audit_repository_test.go` | 仓储纯逻辑（fake DBTX，无连库） | **WHERE 组装**（等值 env/action/resourceType/resourceId/actor_id 参数序）、operator 优先于 operatorKeyword、keyword 双参（resource_id+summary）、operatorKeyword 双参、**from/to UTC 归一**、**排序 created_at DESC + id DESC**（及 ASC）、分页 LIMIT/OFFSET 参数、Insert 6 参数 + 空 detail 序列化（含 omitempty 观察）、Insert 保留非空 detail |
| `internal/transport/http/middleware/middleware_test.go` | L1（无 IO） | isWriteMethod（GET/HEAD/OPTIONS 否）、methodToAction、singular、normalizeSegment、**inferFallbackAudit**（动词后缀 action、方法回退 action、id 取自 query、**resourceID 截断 128**、空 path 兜底 admin.create/unknown）、clientIP（XFF 优先/RemoteAddr 兜底）、bearerToken |
| `internal/transport/http/audit/handler_test.go` | L1（无 IO） | **toOperator**（actor=0 系统占位 / 删用户→null / join 展开）、parseQuery（默认 page1/size20、sort desc/asc/未知回退、过滤解析、operator 非法/负数报错、from>to 报错、坏时间格式报错、pageSize 透传由 service 钳制） |

### 场景维度覆盖表（接口 × S1–S10，manifest `tests/backend/scenarios/audit.yaml`）
| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET /audit-logs | ✓ | ✓(live 401×2) | ✓ | ✓(from>to/operator) | — N/A 只读 | ✓(env 过滤) | —*被写方 | ✓(masked) | ✓(钳100/排序) | — N/A 只读 |
| GET /audit-logs/{id} | ✓(含 404) | ✓(live 401) | ✓ | ✓(id 非法) | — | — | —* | ✓(masked) | — | — |
| 写侧触发（S7/S8/S10 重点） | — | — | — | — | — | — | ✓(显式落库+兜底补写+去重) | ✓(写前递归脱敏) | — | ✓(显式同事务失败回滚业务) |
- 运行形态：S2（requiresDB:false）3 例在进程内 httptest 真实执行并断言 401；其余 20 例 requiresDB:true，进程内仅解析校验、跳过，待 `SCENARIO_WITH_DB=1` 连库 harness 执行落库断言。
- 可选 `GET /audit-logs/facets` 标注未实现（占位 case）。

### fixtures（挂统一回归入口）
- 新增 `tests/fixtures/common/audit.sql`：`audit_reader` 角色（仅 audit.read）+ base（跨 env 多行、同 created_at 验 id 二级排序）+ secret（detail 已 masked）+ system（actor_id=0）。由 `scripts/regression/db.sh` 的 `tests/fixtures/common/*.sql` glob 自动灌入；scenario 由 harness glob 自动发现 → 已挂统一回归入口。

### 运行结果
- `cd services/admin-api && go test ./...`：**全部 ok（0 失败）**。新增 4 包 51 子用例全过；`go vet` 4 包通过。
- `go test ./internal/testkit/scenario/...`：audit manifest 解析通过，3 live PASS / 20 SKIP（requiresDB）。
- 基线：改动前 `go test ./...` 全绿；改动后仍全绿，无回归。

### 疑似实现缺陷（待回退后端开发，经 Leader 调度）
1. **[低] schema 命名差异**：compact 将 `audit_logs` 归 `platform` schema，但 `000001` 建于默认（public）、`000003` 仅迁移 `admin_*` 至 platform，仓储以未限定名查询依赖 search_path。功能自洽，但与文档不一致，建议统一（迁移移入 platform 或更新文档）。
2. **[极低] omitempty 抵消默认 {}**：`audit_repository.Insert` 将 nil 的 before/after/extra 置空 map，但 `AuditDetail` 字段带 `json:omitempty`，空 map 仍被省略 → detail_json 不含这些键（DB 列默认 '{}' 兜底，无功能影响）；该置空逻辑实为 dead code，可移除以免误导。
3. **[低] SinkAdapter 不传 SecretKeys 且吞错**（CR 已记录）：历史模块经 `AuditSink` 写审计时 `SecretKeys` 恒空，依赖各模块写前自行脱敏；失败仅日志不回传，"高危操作审计失败回滚业务"未生效。属本期已知范围，连库集成测试阶段需重点验证显式写脱敏与回滚红线。

## [backend-fix] 回退缺陷修复（2026-06-29，Codex 5.3）
- 修复 #1（schema 一致性）：核对 `000001_init.up.sql` 权威定义，`audit_logs` 位于默认 schema（public）；确认并保持 `000007_audit_indexes` 与 `audit_repository.go` 均使用同一未限定表名 `audit_logs`，避免 schema 歧义。
- 修复 #2（SinkAdapter 红线）：升级 `adminapp.AuditSink` 为 `Write(... ) error` 并新增 `AuditEntry.SecretKeys`；`audit/sink.go` 透传 `SecretKeys -> SecretAwareAuditInput.SecretKeys` 且返回写入错误；`admin/game/channel/account-auth` 显式调用点改为错误上抛，移除静默吞错语义；新增 `internal/app/audit/sink_test.go` 覆盖“透传 secret keys + 错误传播”。
- 修复 #2（secret 字段数据流打通）：`accountauth/service.go` 在写审计时汇总模板 `secret_fields_json` 并注入 `AuditEntry.SecretKeys`，保证跨模块 adapter 层具备脱敏输入通道（即便本次 detail 未落密文字段）。
- 修复 #3（dead code）：`audit_repository.Insert` 改为零值 detail 直接写 `'{}'`，非零值才 JSON 序列化；删除“置空 map 后被 omitempty 抵消”的无效分支；新增仓储测试 `TestInsert_ZeroDetailWritesJSONEmptyObject`。
- 兼容调整：更新 admin/channels/games 三处 httptest fakeAudit 签名以匹配 `AuditSink` 新接口。

## [frontend-test] AuditView 组件级测试（2026-06-30 Cursor Auto）
- 接续前一轮（上轮因连接中断未完成自检）。先通读已有产物 `apps/admin-web/src/views/audit/__tests__/AuditView.spec.ts`，对照 `AuditView.vue` 实现、`api/modules/audit.ts` 契约与 22-audit compact 前端章节（§前端 / §状态态）逐条核对：现有 24 例覆盖完整、断言正确，未推倒重来。
- 补齐 1 例：`operator 选中后以 actor_id 提交`（FilterBar operator 过滤维度此前未断言，compact §过滤器要求"操作者下拉提交 id"）。
- 测试文件与覆盖交互点：
  - 挂载与列表渲染：首屏加载第 1 页（page/pageSize/sort 默认）、列渲染（动作/资源/摘要/操作者 displayName）、actorId=0→System / displayName 缺失兜底 actorId、大整数 id 字符串无损。
  - 动词色系 + production 高亮：actionTagType(create=success/delete=danger/publish=primary/execute=warning/hide=info/默认 info)、envTagType(production=danger/sandbox=warning/develop=success)、列表 production 行渲染 `.el-tag--danger`。
  - FilterBar 提交式查询 + 切页保留过滤：改 draft 不发请求、submit 才发请求且 trim、reload(2) 保留已提交过滤、onSortChange 回第 1 页保留过滤、空过滤不下发空串、operator 提交 id、timeRange→ISO-8601 from/to、resetFilters 清空并重查第 1 页。
  - 详情抽屉 before/after 三态：create 仅 after 单列 / delete 仅 before 单列 / update 左右对照（changed 高亮，默认仅看变更字段，开关后展示全部）。
  - 脱敏：密文字段 `masked`/`******` 恒展示 `******`、非密文原样、抽屉渲染含 `******` 文案。
  - 详情接口异常回退：非 404 设 detailError 并回退列表快照；404 静默回退不报错。
  - 状态态：空态文案 / 错误态(非 403 展示 message + 重试入口) / 403 整页降级隐藏 FilterBar / 无 audit.read 任意错误均降级 forbidden。
  - 全只读：页面按钮无 新建/删除/保存/编辑/提交/发布/执行/导出 等写动词，仅查询/重置只读交互。
  - API 契约全部 mock/stub：`listAuditLogs`/`getAuditLogDetail`/`listAuditLogFacets`/`listAdminUsers` 及 `@/router`。
- 依赖与运行：worktree `apps/admin-web/node_modules` 已含 `@vue/test-utils@2.4.11`、`vitest@4.1.9`（package.json devDependencies 既有），无需重新 pnpm install。
- 运行结果：`cd apps/admin-web && pnpm vitest run src/views/audit/__tests__/AuditView.spec.ts` → 1 file / 25 tests passed，0 failed（环境 jsdom，setupFiles 注入 ElementPlus/ResizeObserver/matchMedia/localStorage）。
- Playwright（L5 e2e/截图）：本轮聚焦 vitest 组件级，未跑 e2e（需真实前后端 + chromium 二进制 + DB，环境不具备）；跳过并记录，待统一回归入口在可运行环境执行 `tests/frontend/e2e/audit.spec.ts`。
- 疑似实现缺陷：无（实现与 compact 一致，所有断言通过）。

---

## 集成/系统测试（integration-test，2026-06-30 Cursor Auto · 🟪 测试专家）

范围：前后端两车道闸门均 ✅ 后的集成/系统测试。本角色不改业务代码，问题汇总移交 🟧。

### 1. 契约对账（前端 `api/modules/audit.ts` ↔ 后端 `transport/http/audit/` + 路由）
- 方法/路径：`GET /api/admin/audit-logs`（列表）、`GET /api/admin/audit-logs/{id}`（详情）✓ 路由与 handler 一一对应。
- query 参数：env/action/resourceType/resourceId/operator/operatorKeyword/from/to/keyword/page/pageSize/sort —— 与 `parseQuery` 全部一致 ✓；`sort` 默认 `-createdAt`（`SortDesc=sort!="createdAt"`）✓；`from>to`→400 VALIDATION ✓；operator 非整数→400 ✓；pageSize 上限 100 钳制 ✓。
- 响应包络 `{data:{items,page,pageSize,total}}`：`httpx.WriteData` + 前端 `request()` 自动解包 `data` ✓。
- `AuditLogItem` 字段：id/actorId 均 `strconv.FormatInt`→string ✓；operator 对象 `{id,userName,displayName}`、actor_id=0→系统占位 `{id:"0",userName:"system",displayName:"System"}`、删用户→null ✓；detail JSON tag（summary/before/after/changed/extra/request{ip,userAgent,requestId,method,path}）与前端 `AuditDetailPayload` 完全对齐 ✓；createdAt RFC3339 UTC ✓。
- 错误码：401 UNAUTHENTICATED / 403 FORBIDDEN（audit.read）/ 400 VALIDATION_FAILED / 404 NOT_FOUND ✓。
- **漂移**：前端 `listAuditLogFacets()` 调 `GET /audit-logs/facets`，后端无 facets 路由 → 被 `/{id}` 捕获、`ParseInt("facets")` 失败 → 返回 **400**（非 404）。前端 try/catch 回退静态字典，功能不受影响，但 scenario `facets_not_implemented` 期望 404 → 连库下判失败。见问题 P2-3。

### 2. 全量回归（真实输出）
- 后端 `cd services/admin-api`：`go build ./...` ✓、`go vet ./...` ✓、`go test ./...` 全绿（所有包 ok，audit 相关 app/transport/middleware/repo 子用例 43 PASS + 0 FAIL）。
- 前端 `cd apps/admin-web`：`pnpm vitest run`（全量）→ 19 files / 108 tests passed，0 failed；audit 聚焦 `src/views/audit/__tests__/` → 25 passed。（注：vitest 在沙箱内首次因 `.vite-temp` EPERM 启动失败，解除沙箱后正常。）
- scenario harness：`go test ./internal/testkit/scenario/... -run TestScenarioManifests` → audit.yaml 解析通过；requiresDB:false 的 S2 鉴权（401×3）live PASS；requiresDB:true 用例 SKIP（manifest parsed OK）。

### 3. 红线端到端核验
- 脱敏：`audit.service.sanitizeDetail` 对 before/after/extra 递归脱敏（secret key→"masked"，嵌套 map/[]any 递归）✓（单测覆盖）；前端 `formatDetailValue` 对 "masked"/"******" 恒显 `******`、不解密 ✓。
- 权限：两路由均 `RequirePerm("audit.read")` ✓；前端路由 `meta.perm=audit.read` + 403 整页降级隐藏 FilterBar ✓。
- env 过滤：查询按 env 等值过滤 ✓；写侧 env 不取前端、由中间件注入 ctx / 缺省运行环境（`sync.execute` 由调用方传 production）✓。
- 写入失败策略：`SinkAdapter.Write` 返回 error 且业务调用方向上冒泡（如 game_service `writeAudit` → `return …, err`）✓；中间件兜底失败仅记结构化错误日志、不阻断响应 ✓。**但**显式路径「同事务回滚」未对历史模块强约束（见 P4-5，已知）。
- 仅 INSERT/SELECT：`AuditRepository` 仅 `Insert`/`Query`，无 Update/Delete（编译期杜绝篡改）✓。
- 去重：`InjectRequestContext` 注入 marker，`Write` 调 `markWritten`，中间件 `IsWritten` 命中则不补写 ✓（单测覆盖）。

### 4. 真实 DB 校验（运行中的 `console-test-pg` @ :55432，库 admin_console / 用户 admin）
> 连库 scenario harness 未真正接库（见 P3-4），改为直接对真实 PG 取证，发现 P0 阻断。
- `audit_logs` 表存在于 **public** schema，7 列与 spec 一致；env CHECK(develop/sandbox/production) ✓。
- `schema_migrations.version=7, dirty=f`，但 6 个 `idx_audit_logs_*` 索引**全部缺失**（仅 PK）。手工执行 `000007_audit_indexes.up.sql` → 6 索引成功创建（SQL 本身有效）。→ 迁移号 000007 与并行车道冲突致静默跳过（P1-2）。
- 运行期连接池 `pool.go:20` 设 `search_path = <env>, platform`（**不含 public**）。实测 `SET search_path=sandbox,platform; SELECT FROM audit_logs;` → `ERROR: relation "audit_logs" does not exist`；repo 的 JOIN 查询同样报错。→ **审计读(GET)与写(显式+兜底)在真实 DB 全部失效**（P0-1 阻断）。
- `admin_users` 在 platform、`audit_logs` 在 public；000003 把 admin_* 归位 platform 却未归位 audit_logs，而 spec §数据模型明确「audit_logs 位于 platform schema」。

### 5. 下游抽查
- `views/dashboard/DashboardView.vue` 当前未调用 audit-logs（无契约破坏）；未来「最近 N 条/今日同步次数」复用 `GET /audit-logs` 时应沿用同一包络。

### 6. 遗留问题清单（移交 🟧 高级全栈工程师）
- **P0-1 [阻断]** audit_logs 在 public，但运行期 search_path=`<env>, platform` 不含 public → 仓储 `FROM/INSERT audit_logs` 运行期 `relation "audit_logs" does not exist`，审计读写全失效。定位：`migrations/000001`(建表 public) + `pool.go:20`(search_path) + 仓储 `audit_repository.go` 不带 schema 限定。建议：新增迁移 `ALTER TABLE public.audit_logs SET SCHEMA platform;`（与 000003 处理 admin_* 一致、契合 spec），索引迁移与仓储一并对齐 platform；或仓储显式用 `platform.audit_logs`。
- **P1-2 [高]** 迁移号 000007 与并行模块车道冲突：共享 DB 已 version=7（他车道推进）→ audit 的 000007_audit_indexes 被 `migrate up` 视为已应用而跳过，索引实际缺失。建议集成时重排为唯一更高序号。
- **P2-3 [低]** `GET /audit-logs/facets` 无路由 → 落入 `/{id}` 返回 400（非 404）。前端已回退静态字典，但 scenario `facets_not_implemented` 期望 404 连库下失败。建议实现 facets 或显式注册返回 404 / 修正 scenario 期望为 400。
- **P3-4 [信息/已知]** 连库 scenario harness 未注入 DSN：`scenarios_test.go` 用 `httpserver.New` 无 POSTGRES_DSN，`SCENARIO_WITH_DB=1` 也仅解除 skip、handler 仍降级(ready=false)，requiresDB 用例无法端到端执行。建议补齐 DSN+迁移+seed 接库。
- **P4-5 [信息/已知]** 历史模块显式审计已冒泡 error，但未保证与业务写同事务回滚（spec §5.6 高危操作要求）；文档已记为后续逐模块增强。

### 7. 通过判定（第 1 轮）
- **有问题需 🟧 修复（存在 P0 阻断），不可进入 ✅功能验收。** 单测/契约/前端回归全绿，但真实 DB 下审计读写因 schema/search_path 不匹配整体失效，必须先修 P0-1（并建议同修 P1-2）后重测连库主线。

---

## 第 2 轮复测（integration-test round 2，2026-06-30 Cursor Auto · 🟪 测试专家）

针对 🟧 修复（P0-1 / P1-2 / P2-3）复测，裁决基准：compact「audit_logs 属 platform」+ 运行期 search_path。

### 修复改动复核
- 迁移：删冲突 `000007`；新增 `000008_audit_logs_platform_schema`（`ALTER TABLE IF EXISTS public.audit_logs SET SCHEMA platform`，幂等 + down 归位 public）；`000009_audit_indexes`（6 索引显式 `platform.audit_logs`，IF NOT EXISTS）✓。
- 仓储 `audit_repository.go`：刻意保持未限定表名（`FROM/INSERT audit_logs`），遵循 admin_* 平台表「靠 search_path 命中」约定 ✓。
- facets：`router.go` 在 `/{id}` **之前**显式注册 `GET /audit-logs/facets` → `h.Facets` 返回 `404 NOT_FOUND` ✓；前端 `audit.ts`/scenario 期望无需改 ✓。

### 全量回归（真实输出）
- 后端 `go build ./...` ✓ / `go vet ./...` ✓ / `go test ./...` 全绿（所有包 ok，0 FAIL）；audit transport 单测 12 PASS（含 sort 回退/operator 校验/operator 占位）。
- 前端 `pnpm vitest run src/views/audit/__tests__/` → 25 passed / 0 failed。

### 连库验证（console-test-pg:55432，事务内复刻 migrate up 后 ROLLBACK，非破坏性）
> migrate/golang-migrate CLI 本机不可用；按 checklist「修复重测须知」以事务内执行 000008+000009 SQL 复刻 `migrate up` 效果，验证后 ROLLBACK 不改动共享库版本指针（与 🟧 自检同法）。
- 基线复现：迁移前 `search_path=sandbox,platform; SELECT FROM audit_logs(public)` → `ERROR: relation "audit_logs" does not exist`（P0 旧症状确认）。
- 修复后（事务内迁表到 platform）：
  - **读**：未限定 `SELECT count(*) FROM audit_logs` 在 `search_path=sandbox,platform` 下解析到 `platform.audit_logs`，0 行无错——**P0 不再复现** ✓。
  - **写（显式路径）**：仓储原样 `INSERT INTO audit_logs(...)`（未限定）成功命中 platform，RETURNING id ✓。
  - **读（List 形状）**：`LEFT JOIN admin_users` + `WHERE env='sandbox'` + `ORDER BY created_at DESC,id DESC` + `LIMIT/OFFSET` 正常返回新插入行 ✓（JOIN platform.admin_users 命中）。
  - **索引**：迁表后 6 个 `idx_audit_logs_*`(+PK) 全部落在 platform schema ✓。
  - ROLLBACK 后共享库 audit_logs 仍在 public（版本指针/数据未改）✓。

### 红线连库核验
- 脱敏：detail 内 `apiKey:"masked"` 原样落库/回传（脱敏在 `AuditService.Write` 应用层完成，单测覆盖递归脱敏）✓；前端 `formatDetailValue` 恒显 `******` ✓。
- audit.read 403 + env 过滤 + 仅 INSERT/SELECT（仓储无 Update/Delete）+ 中间件 ctx 去重：逻辑 + 单测 + 上述连库 SQL 验证一致 ✓。
- 写入失败策略：显式 SinkAdapter 冒泡 error（调用方 `return …, err`）；中间件兜底失败仅 `logger.Error` 留痕、不阻断 ✓。

### 验证边界（如实记录）
- 未跑「booted server 全鉴权 HTTP e2e」：受限于 ① 无 migrate CLI；② 共享测试库为跨车道 fixture，缺 develop/sandbox/production env schema（非干净全迁移库）；③ 连库 scenario harness 仍未注入 DSN（P3-4）；④ 需签发携 `audit.read` 的有效 access token。P0 失败点位于 schema/search_path 解析层，已由上述连库 SQL 端到端证伪「relation 不存在」；HTTP handler/middleware/service/repo 行为由单测 + 契约对账全覆盖，非缺陷来源。建议在 CI（DSN+migrate up+seed 齐备）跑 `tests/backend/scenarios/audit.yaml` requiresDB + `tests/frontend/e2e/audit.spec.ts` 收口可视主线。

### 第 2 轮通过判定
- **可进入 ✅功能验收。** P0-1/P1-2/P2-3 复测通过（连库读写命中 platform.audit_logs、索引到位、facets 404）；全量回归全绿；P3-4（连库 harness 注入 DSN）/ P4-5（历史模块显式审计同事务回滚）登记为已知非阻断遗留，建议后续闭环。

---

## 🟧 修复（高级全栈工程师，2026-06-30 Opus 4.8）

裁决标准：以 compact 契约为唯一标准（audit_logs 属 **platform** schema）+ 运行期行为（`pool.go` 连接池 `search_path=<env>, platform`，不含 public）。上一轮「对齐 public」方向已被集成专家真实库取证推翻，本轮按 platform 修复。

### 修复 P0-1 [阻断]：audit_logs 迁入 platform schema
- 根因：`000001_init.up.sql` 在默认 schema(public) 建 `audit_logs`；运行期连接池 `pool.go:20` 钉死 `search_path=<env>, platform`（不含 public）→ 仓储未限定的 `FROM/INSERT audit_logs` 运行期 `relation "audit_logs" does not exist`。`000003` 已把 admin_* 归位 platform 却漏了 audit_logs。
- 改动文件:行
  - 新增 `services/admin-api/migrations/000008_audit_logs_platform_schema.up.sql:1-12`：`CREATE SCHEMA IF NOT EXISTS platform; ALTER TABLE IF EXISTS public.audit_logs SET SCHEMA platform;`（同 000003 既有做法、幂等、可前向）。
  - 新增 `services/admin-api/migrations/000008_audit_logs_platform_schema.down.sql:1-4`：`ALTER TABLE IF EXISTS platform.audit_logs SET SCHEMA public;`（归位回 public）。
  - 仓储 `audit_repository.go` **不改**：遵循项目对 platform 级表的既有约定——`admin_user_repo.go`/`role_repo.go` 等平台表 SQL 均**不写 schema 前缀**，靠 search_path 命中（`pool.go` 包注释明确）。故 `audit_logs`/`admin_users` 未限定写法在 search_path 含 platform 后正确命中。
- 验证：连真实库 `console-test-pg:55432` 事务内执行 000008+000009 up SQL → `SET search_path=sandbox,platform` → `SELECT COUNT(1) FROM audit_logs`(OK, rows=0) + `INSERT INTO audit_logs(...)`(OK) → ROLLBACK（DB 未改动）。P0 复现的 `relation does not exist` 已消除。

### 修复 P1-2 [高]：迁移号重排
- 根因：`000007_audit_indexes` 与并行车道共享 DB 的 version=7 冲突 → audit 索引被 `migrate up` 视为已应用而静默跳过。
- 改动文件:行
  - 删除 `services/admin-api/migrations/000007_audit_indexes.{up,down}.sql`。
  - 新增 `services/admin-api/migrations/000009_audit_indexes.up.sql:1-26`：内容同原 6 索引，但对表显式限定 `platform.audit_logs`（migrate 连接默认 search_path 不含 platform，且 000008 已迁表，必须显式限定才能命中）；`IF NOT EXISTS` 幂等。
  - 新增 `services/admin-api/migrations/000009_audit_indexes.down.sql:1-8`：逆序 `DROP INDEX IF EXISTS platform.idx_audit_logs_*`。
  - 序号顺序：000008（迁表）必须先于 000009（建索引）。
- 备注：**最终全局唯一序号由集成 Agent across-branch 复核**（本 worktree 内 8/9 未占；其它车道可能已用）。已在 integration.checklist 注明。
- 验证：事务内验证 platform.audit_logs 索引数=6（旧库 public 上 6 个旧索引随 `SET SCHEMA` 迁到 platform，000009 `IF NOT EXISTS` 自动跳过，幂等无冲突）。

### 修复 P2-3 [低]：facets 返回 404 而非 400
- 根因：`/audit-logs/facets` 无显式路由 → 被 `/{id}` 捕获，`ParseInt("facets")` 失败 → 返回 400（非 404）。
- 方案选择：facets 为 compact §6.3 可选接口、前端有静态字典回退。选「显式注册 facets 路由返回 404 NOT_FOUND」——成本最低、REST 语义正确、且**不破坏** `get_by_id_invalid_id`（`abc`→400 VALIDATION_FAILED，若改 `/{id}` 对非数字统一返 404 会误伤该用例）。
- 改动文件:行
  - `services/admin-api/internal/transport/http/audit/router.go:19-22`：在 `/{id}` 之前注册 `ar.With(RequirePerm("audit.read")).Get("/audit-logs/facets", h.Facets)`。
  - `services/admin-api/internal/transport/http/audit/handler.go:83-89`：新增 `Handler.Facets`，`httpx.WriteError(w, 404, CodeNotFound, "facets 接口未实现")`。
- 同步：scenario `tests/backend/scenarios/audit.yaml` `facets_not_implemented` 期望 404 **本就一致、无需改**；前端 `api/modules/audit.ts` `listAuditLogFacets` 失败已 catch 回退静态字典、**无需改**。

### P3-4 / P4-5（本期未改，记录待办）
- P3-4：连库 scenario harness 未注入 POSTGRES_DSN，留待 🟪/集成在可运行环境注入后跑 requiresDB 用例（本期未改 harness）。
- P4-5：历史模块显式审计未保证同事务回滚，按约定本期不扩大改动，仅记录待办。

### 改动文件列表
- 新增：`migrations/000008_audit_logs_platform_schema.up.sql` / `.down.sql`
- 新增：`migrations/000009_audit_indexes.up.sql` / `.down.sql`
- 删除：`migrations/000007_audit_indexes.up.sql` / `.down.sql`
- 改：`internal/transport/http/audit/router.go`、`internal/transport/http/audit/handler.go`
- 未改（刻意）：`internal/infra/persistence/postgres/audit_repository.go`（遵循平台表无前缀约定）

### 自检结果
- `cd services/admin-api && go build ./... && go vet ./... && go test ./...` → **全绿**（所有包 ok，无回归；audit app/transport/middleware/repo + testkit/scenario 全过）。
- 迁移前向执行：连真实库非破坏性事务验证（执行 up SQL + 运行期 search_path SELECT/INSERT 命中 + ROLLBACK）通过，DB 未改动。
- 前端：本次无前端代码改动，未跑 vitest（scenario/audit.ts 与改动一致，无需改）。
- 连库主线 e2e：受 P3-4（harness 未接 DSN）限制，本期未端到端跑，**待 🟪 连库重测**。

### 通过判定（🟧）
- P0-1 / P1-2 / P2-3 已修并自检全绿 + 真实库非破坏性验证通过；P3-4 / P4-5 留待 🟪/集成。建议 🟪 在干净库或对取证库 `migrate force 6 && migrate up` 后重测连库主线（详见 integration.checklist「🟧 修复重测须知」）。

---

## ✅ 功能验收（acceptance，2026-06-30 · 功能验收师 / Cursor Auto）

> 前置闸门：🟪 第 2 轮复测判定「✅ 可进入功能验收」已满足。基准 = 功能端到端可用 + 满足 compact 业务规则 + 符合 02-operation-flow 主线。验收清单由 compact（API/页面/状态机/规则）+ operation-flow 推导，逐条 PASS/FAIL + 证据。

### 一、构建 / 测试结果汇总（真实输出）
| 命令 | 工作目录 | 结果 | 证据 |
| --- | --- | --- | --- |
| `go build ./...` | services/admin-api | PASS | 退出 0，无输出 |
| `go vet ./...` | services/admin-api | PASS | 退出 0，无输出 |
| `go test ./...` | services/admin-api | PASS | 全包 ok / 无 FAIL（含 app/audit、infra/postgres、transport/http/{audit,middleware}、testkit/scenario） |
| `go test testkit/scenario -run TestScenarioManifests/audit -v` | services/admin-api | PASS | audit 用例：3 live PASS（S2 401 鉴权）/ 20 SKIP（requiresDB 待连库 harness）；manifest 解析全过 |
| `pnpm vitest run src/views/audit/__tests__/` | apps/admin-web | PASS | 1 file / 25 passed（沙箱 .vite-temp EPERM，解除沙箱后通过） |
| `pnpm vitest run`（全量） | apps/admin-web | PASS | 19 files / 108 passed |
| `WITH_DB=0 sh scripts/regression/run.sh`（统一回归入口） | repo root | 后端 PASS / 前端 e2e FAIL | summary.md：后端 go test pass=383 fail=0；前端 Playwright 23 failed / 7 passed |
| Playwright e2e（audit/games/channels 全模块） | repo root | FAIL（环境受限） | 失败因无 booted 前后端+DB（`getByText` 元素找不到），跨全部模块一致；非 audit 代码缺陷，沿用 🟪 边界说明 |

说明：Playwright L5 e2e 在本环境无法起真实前后端+chromium+DB（与 🟪「booted-server 全鉴权 HTTP e2e 未跑」边界一致），失败为环境性、跨模块普遍，非 audit 功能缺陷；audit HTTP 栈由单测 + 契约 + 连库 SQL（🟪 round2）覆盖。

### 二、验收清单
| 编号 | 验收点 | 期望 | 实际 | 证据 | 判定 |
| --- | --- | --- | --- | --- | --- |
| A1 | 写侧显式同事务路径 | 业务命令在同事务调 AuditService.Write，拿精确 before/after | game/channel/account-auth/admin 经 `AuditSink`→`SinkAdapter.Write`→`AuditService.Write`，sink 返回 error 不吞错 | sink.go:21-41；admin_wiring.go:84,99-118；game_service.go:352-360 | PASS |
| A2 | 写侧中间件兜底 | 非 GET/HEAD/OPTIONS 且 2xx、未显式写则补粗粒度审计 | `Audit` 中间件 `isWriteMethod`+status∈[200,300) 判定，`inferFallbackAudit` 出 action/resource，summary="middleware fallback" | middleware.go:104-160 | PASS |
| A3 | ctx 去重 | 显式写置标志后中间件不重复补写 | `markWritten`/`IsWritten` 原子标志；中间件 `IsWritten` 命中即 return | context.go:49-63；middleware.go:127-129 | PASS |
| A4 | 写入唯一入口、禁直接 INSERT | 仅 AuditService.Write+中间件；业务不直接 INSERT audit_logs | 仅 `audit_repository.Insert` 拼 INSERT；全仓 rg 无其它 `INSERT audit_logs` | audit_repository.go:28-31；rg 结果 NONE | PASS |
| A5 | 只增不改（无 UPDATE/DELETE API） | 无编辑/清空 API | 路由仅 GET ×3；无 POST/PUT/DELETE audit | router.go:15-24 | PASS |
| A6 | 仓储编译期无 Update/Delete | Repository 接口仅 Insert/Query | 接口与实现仅 Insert/Query；rg 无 Update/Delete | service.go:74-77；audit_repository.go | PASS |
| A7 | 脱敏 masked 不解密 | secret 键替换 masked，明文不落库 | `sanitizeMap` 命中 SecretKeys→"masked"；前端密文恒 `******` | service.go:215-242；AuditView.vue:617-634 | PASS |
| A8 | 脱敏递归 | before/after/extra 嵌套 map/数组递归脱敏 | `sanitizeValue` 递归 map[string]any 与 []any | service.go:244-257 | PASS |
| A9 | 文件字段只记引用 | 不记文件内容 | 由调用方声明 secret/仅传 storage key（compact §5.5 约定），audit 不接收文件内容 | compact §5.5；service 仅按声明键脱敏 | PASS（约定层） |
| A10 | GET /audit-logs 列表（分页/filter/排序） | env/action/resourceType/resourceId/operator/operatorKeyword/from/to/keyword/page/pageSize/sort | parseQuery 全参；WHERE 等值+范围+ILIKE keyword；分页钳制≤100；排序 created_at DESC,id DESC | handler.go:90-143；service.go:171-182；audit_repository.go:44-166 | PASS |
| A11 | GET /audit-logs/{id} 详情 | 同构对象；不存在→404 | GetByID；ID 过滤 len==0→ErrNotFound→404；非数字 id→400 | handler.go:57-81；service.go:160-162 | PASS |
| A12 | /facets | 404 或实现 | 显式注册返回 404 NOT_FOUND（可选未实现），前端 catch 回退静态字典 | router.go:22；handler.go:86-88；audit.ts:88-90,AuditView.vue:446-450 | PASS |
| A13 | 权限 audit.read（403/菜单隐藏） | 缺权限接口 403、前端整页降级+菜单隐藏 | 路由 `RequirePerm("audit.read")`；前端 meta.perm + 403→forbidden 整页降级且隐藏 FilterBar | router.go:19-23；routes.ts:53-56；AuditView.vue:69-74,492-498 | PASS |
| A14 | id/actorId 字符串 | 避免 JS 大整数精度 | handler `strconv.FormatInt`→string；前端类型 string | handler.go:38-39,71-72；audit.ts:37-38 | PASS |
| A15 | 统一包络 | `{data:{items,page,pageSize,total}}` | httpx.WriteData 包 data；List 装 items/page/pageSize/total | handler.go:49-54 | PASS |
| A16 | from>to 校验 | 400 VALIDATION_FAILED | handler+service 双重校验 From.After(To)→ErrValidation→400 | handler.go:134-136；service.go:149-151 | PASS |
| A17 | action=resource.action 与权限码同源 | 显式 action 用权限码 | game.update / admin_user.create / role.delete 等 `resource.action`，与权限码同源 | game/admin/channel/accountauth 各 writeAudit | PASS（注：history 模块个别用 game.legal.update/game.account_auth.update 三段式，属各自模块命名，见遗留） |
| A18 | env 不取前端 | 写侧 env 取运行环境，非请求体 | Write env: in.Env→ctx.meta.Env（中间件注入运行 env）→runtimeEnv；请求体无 env 写入路径 | service.go:115-121；middleware.go:118 | PASS |
| A19 | sync.execute=production | 同步执行写 production | AuditWriteInput.Env 由调用方显式传（service 不覆盖非空 env）；机制支持。sync 模块不在本 lane，未端到端走查 | service.go:115-117；service_test.go:244-247 | PASS（机制就绪，跨模块待 sync lane 落地） |
| A20 | 前端 FilterBar 提交式 | 改 draft 不请求，submit 才请求 | draftFilters/appliedFilters 分离；submitFilters→applyFilters+reload | AuditView.vue:430-511；vitest 覆盖 | PASS |
| A21 | Table 倒序 + 动词色系 + production 高亮 | created_at 倒序、create绿/delete红/publish蓝/execute橙/hide灰、production danger | default-sort descending；actionTagType/envTagType | AuditView.vue:94,568-582 | PASS |
| A22 | 详情抽屉 before/after 三态 + changed 高亮 | create 仅 after / delete 仅 before / update 对照+高亮 | hasBefore/hasAfter 分支单列/对照；displayCompareRows + changed class | AuditView.vue:186-211,379-399 | PASS |
| A23 | 403 整页降级 + 全只读 | 无写/删按钮 | pageState=forbidden 隐藏 FilterBar+el-result；全页无写/删按钮 | AuditView.vue:9,69-74 | PASS |
| A24 | schema 迁移 platform | 000008 迁表 platform + 000009 索引 platform | 000008 `ALTER ... SET SCHEMA platform`（幂等+down）；000009 显式 `platform.audit_logs` 6 索引 | 000008/000009 up/down.sql | PASS |
| A25 | 运行期 search_path 命中 | 未限定 SELECT/INSERT 命中 platform.audit_logs | 🟪 round2 连库事务内验证命中（P0 不再复现），本验收沿用其取证 | 🟪 integration.checklist round2；manifest reverify_round2 | PASS（沿用 🟪 连库取证） |
| A26 | 下游 dashboard 复用 GET /audit-logs 无破坏 | 读契约稳定 | dashboard 为独立 lane 尚未消费 audit-logs（grep 无引用）；GET 契约未变，无破坏面 | DashboardView.vue 无 audit 引用 | PASS（无破坏，待 dashboard lane 接入） |

### 三、结论
**✅ 通过（功能验收）**。26 项验收点全 PASS（含 3 项「机制就绪/约定层/沿用连库取证」标注）。后端 build/vet/test 全绿（go test 383 pass / 0 fail）、前端 vitest 108 passed、audit 聚焦 25 passed；audit 写侧两路径（显式同事务+中间件兜底去重）、只增不改（编译期无 Update/Delete）、递归脱敏、读 API（列表/详情/facets-404/权限/包络/字符串 id）、前端只读页（提交式过滤/倒序/三态详情/403 降级/全只读）、schema 迁移 platform 均满足 compact 与 operation-flow。

### 四、遗留风险与建议
- **P3-4（非阻断）**：连库 scenario harness 未注入 POSTGRES_DSN，requiresDB 用例（落库断言 expect.db/expect.audit、L2 仓储事务）+ booted-server 全鉴权 HTTP e2e 本环境未端到端执行；audit HTTP 栈由单测+契约+🟪 连库 SQL 覆盖。建议 CI/集成在可运行环境注入 DSN 收口。责任：🟪/集成。
- **P4-5（非阻断）**：历史模块显式审计已冒泡 error，但未保证与业务写同事务回滚（compact §5.6 高危 publish/execute/approve 要求强一致）。本期按约定不回改。建议后续逐模块（sync.execute/*.publish/fx.approve）增强同事务。责任：各业务模块 owner。
- **观察项（非阻断，建议登记）**：① 个别历史模块显式 action 命名与 compact 样例略有出入（如 `game.legal.update` vs `game_legal_link.update`、`game.account_auth.update` vs `game_account_auth_config.update`），action 为开放 VARCHAR 不影响功能，建议统一同源命名。② 迁移序号 000007 已删、跳到 000008/000009，最终全局唯一序号需集成 Agent across-branch 复核（其它车道可能已占用 8/9）。③ Playwright L5 e2e 待可运行环境补跑视觉/全链路基线。
