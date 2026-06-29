# 代码生成执行日志归档（Human-Only）

> 本文件从 `docs/architecture/v2/codegen-progress.md` 迁出，默认仅供人类审计。
> 总 Agent、集成 Agent、后续模块 Agent 默认**不读取**本文件；它们应优先读取 `spec.compact.md`、`module.manifest.json`、`integration.checklist.md`、`handoff.summary.md`。

## 10 · auth — 2026-06-24 收尾验收通过

- 🟦 后端：开发✅ / CR✅（直接修复：`GET /system/admin-users` status 枚举校验缺失→补 400；create/update email format=email 校验补齐；`PUT .../roles`、`PUT .../permissions` 的 roleIds/permissionIds「必填」语义补齐，缺字段→400，空数组=全量覆盖合法）/ 测试✅。
  - 关键产出：19 个 API（/auth/login·refresh·logout·feishu/callback、/me、/system/{admin-users,roles,permissions} 全 CRUD），权限码 system.read/admin_user.write/role.write/permission.write；6 张 admin_* 平台表（migration 000003 归位 platform + status CHECK + 索引 + 权限码/super_admin/初始 admin seed）；领域纯函数（MaskIdentityKey、MergePermissionCodes、PermissionCode 校验、状态机 ApplyStatus/CanTransitionStatus、AuthContext.HasPermission/super_admin 短路、NormalizePage）。
  - 测试：domain/jwt/crypto 单元 41 用例；新增进程内 L2/L3 httptest（内存仓储+真实 bcrypt/JWT）24 个测试函数，真实覆盖 S1/S3/S4/S5/S7/S8/S9/S10（登录/刷新/禁用拒绝/RBAC403/校验/冲突/审计/脱敏/分页钳制/事务回滚）；场景矩阵 auth.yaml 101 用例（S1–S10 × 19 接口）。go build/vet/test 全绿（17 包 0 FAIL）。
- 🟩 前端：开发✅ / CR✅（直接修复：`AdminUserListItem.roles` 由 `string[]` 改为 `RoleRef[]`，面板渲染角色名，修复与后端 RoleBrief 的契约漂移）/ 测试✅。页面/组件：登录页（密码/飞书 Tab）、/system（AdminUsersPanel/RolesPanel/PermissionsPanel/PermissionTree）、stores auth/permission/app、http 客户端（Bearer/401 续期/403 toast/X-Environment）、路由守卫、v-perm 指令。vitest 30 用例（8 文件）全绿；vue-tsc build 全绿。
- 🟪 测试专家：契约对账揪出 1 处并行开发漂移（list.roles 形状）→交 🟧 修复后复测通过；统一回归后端 backend.sh ✅；Playwright e2e 6 用例（登录功能 4 + 视觉基线 2，基线已生成入库）全绿。
- 🟧 全栈修复：4 项（后端 status 枚举校验、email 格式校验、roleIds/permissionIds 必填；前端 roles 契约漂移），均最小改动并复测。
- ✅ 验收：通过。9 阶段全 ✅。遗留风险：飞书 HTTPClient 为占位（生产需接真实 OpenAPI）；refresh 无状态、不启用 denylist（禁用即时踢下线需另补会话表）；scenario.yaml 中 requiresDB 用例需连库 harness 落地后由 SCENARIO_WITH_DB=1 执行（当前由内存 httptest 等价覆盖）。

## 10 · auth — 收尾验收（已有产出评估）
- 🟦 后端：开发✅ / CR✅ / 测试✅。补 L2/L3 httptest(内存仓储+真实 bcrypt/JWT) 24 函数，修 status枚举/email格式/roleIds·permissionIds必填校验。API 19、迁移 000003(platform schema 6 表)、纯函数 6+。
- 🟩 前端：开发✅ / CR✅ / 测试✅。登录页+/system 四面板+stores+http客户端+v-perm。vitest 30、e2e 6。
- 🟪 测试专家：集成✅。契约对账修前后端 roles 形状漂移。
- ✅ 验收：通过。9 阶段全✅。
- 遗留：飞书 HTTPClient 占位；refresh 无 denylist；auth.yaml requiresDB 用例待连库 harness。

## 11 · game — 游戏主数据
- 🟦 后端：开发✅ / CR✅(直修事务原子性 InTx + 迁移 schema 限定) / 测试✅。API 6、迁移 000004、纯函数 GenerateGameID/Secret·ApplyDefaultMarket·ValidateMarkets·ValidateLegalScope 等。domain 单测 20 + httptest 24。
- 🟩 前端：开发✅ / CR✅(直修详情非404错误误判) / 测试✅。views/games(列表/创建抽屉/详情多Tab/市场/法务)。vitest 35 + e2e 9(视觉基线入库)。
- 🟪 测试专家：集成✅(1轮)。契约对账无字段漂移；移交 🟧 3 项低危。
- 🟧 全栈修复：修 3 项(默认市场须enabled前后端校验 / legal-links 返 GameDetail / URL 前端格式校验)，补测全绿。
- ✅ 验收：通过 26/26。
- 遗留(非阻断)：市场删除保护降级(channel未落地,CountChannelsByMarket恒0)；审计 sink nil(audit未落地)；game_id 并发重试缺失(低频)。

## 12 · channel — 2026-06-25 后端车道收尾
- 🟦 后端：开发✅ / CR✅ / 测试✅。在已有产出上补齐收尾：`channel` 列表查询参数布尔校验（`compatible`/`hidden` 非法值改为 400 `VALIDATION_FAILED`），新增 `internal/transport/http/channels/handler_test.go`；补 `tests/backend/scenarios/channel.yaml`（覆盖 S1–S10 维度声明，含 requiresDB 分层执行约定）。
- 契约/CR核对：按 compact 逐项核对表结构（`000005_channel_schema`）、API 路由/权限码（`channel.read`/`channel.write`）、错误码（`MARKET_CHANNEL_INCOMPATIBLE`/`CONFLICT`/`VALIDATION_FAILED`）、兼容性与运行态规则、隐藏状态机与包级 market 约束；发现并修复 query bool 静默吞错问题。
- 关键验证证据：`cd services/admin-api && go build ./...` ✅；`cd services/admin-api && go vet ./...` ✅；`cd services/admin-api && go test ./...` ✅（`ok` 包 19，失败 0；无测试文件包 11）；新增 `channel.yaml` 已被 `internal/testkit/scenario` 解析执行链路纳入。
- 偏差/未决：无新增阻断项（后续待前端 CR/测试、再进入 🟪 集成）。

## 12 · channel — 2026-06-25 前端 CR 收尾
- 🟩 前端：CR✅（基于现有实现最小修复，不重写）。按 compact 核对页面/组件/关键交互、API 字段契约、权限态/空错态、运行态与状态展示。
- 直接修复 4 项：1) `ChannelInstancesTab.vue` 新建/复制候选从“当前列表页”升级为“全量实例（含隐藏）”数据源，避免分页/过滤导致漏判与冲突；2) `ChannelInstanceTable.vue` 的「复制创建」补 `v-perm='channel.write'`，与写操作权限规范一致；3) `ChannelInstanceDetailDrawer.vue` 将 `canWrite` 改为 `computed`，确保权限异步加载后可响应更新；4) `ChannelInstanceTable.vue` 行信息改为“渠道名 + channelId + displayKey”，与 compact 的信息架构口径一致。
- 关键验证证据：`cd apps/admin-web && pnpm build` ✅（含 `vue-tsc --noEmit` 与 `vite build`，通过）；`ReadLints` 针对上述 3 个改动文件无新增告警/错误。
- 偏差/未决：未发现阻断项；`apps/admin-web/src/views/games/detail/ChannelInstancesTab.vue` 为未接线路径的旧实现，不影响 channel 当前路由交付，建议后续清理归并。

## 12 · channel — 2026-06-25 前端测试收尾
- 🟩 前端：测试✅。新增/调整 `channel` 前端测试：`ChannelInstancesTab.spec.ts`（创建抽屉候选集合拉全量实例含隐藏、跨分页）、`ChannelInstanceTable.spec.ts`（渠道名优先映射与行状态样式逻辑）、`ChannelInstanceDetailDrawer.spec.ts`（`canWrite` 响应式权限更新），以及 `tests/frontend/e2e/channels.spec.ts`（列表展示与复制创建权限置灰）。
- 修复与测试基建：`apps/admin-web/playwright.config.ts` 的 webServer 启动命令补 `--host 127.0.0.1`，解决 Playwright 健康检查对 `127.0.0.1` 探测时的连接拒绝。
- 实跑结果：`pnpm --dir apps/admin-web test -- src/views/channels/components/__tests__/ChannelInstancesTab.spec.ts src/views/channels/components/__tests__/ChannelInstanceTable.spec.ts src/views/channels/components/__tests__/ChannelInstanceDetailDrawer.spec.ts` ✅（17 文件 / 68 用例全通过）；`cd apps/admin-web && E2E_PORT=5194 pnpm exec playwright test channels.spec.ts` ✅（2/2）；`pnpm --dir apps/admin-web build` ✅。
- 偏差/未决：无阻断缺陷；可进入 🟪测试专家阶段做跨栈联调与契约对账。

## 12 · channel — 2026-06-25 测试专家首轮联调（阻断）
- 🟪 测试专家：首轮结论 🔄（未通过）。前后端契约主路径对账完成：方法/路径/DTO 字段/错误码/权限码与 `modules/12-channel/spec.compact.md` 对齐，未发现主链路漂移。
- 实跑证据：`cd services/admin-api && go test ./... -count=1` ✅；`cd services/admin-api && go test ./internal/testkit/scenario -run TestScenarioManifests/channel -v -count=1` ✅（PASS 7，SKIP 24；SKIP 原因：requiresDB 用例需 `SCENARIO_WITH_DB=1` + PG 全装配）；`cd apps/admin-web && pnpm test` ✅（17 文件 / 68 用例）；`cd apps/admin-web && pnpm e2e` ❌（17/17 失败，`browserType.launch` 缺失 Chromium 可执行文件）。
- 阻断项：
  1) `WITH_DB=1 sh scripts/regression/run.sh` 在当前环境执行 `docker compose up -d postgres` 即被 `Killed: 9`，导致真实 PG + migrate + seed 的跨栈链路无法启动，S1/S3/S4/S5/S6/S7/S9/S10 的 requiresDB 场景无法落地执行。
  2) Playwright 浏览器依赖缺失（提示需 `pnpm exec playwright install`），前端 e2e/视觉基线无法在本机完成实跑。
- 建议移交：先由 🟧 处理环境阻断（Docker 可用性、Playwright 浏览器安装），再回 🟪 复测并补齐真实后端+前端集成证据。

## 12 · channel — 2026-06-25 测试专家复跑（环境解阻后，真实证据）
- 🟪 测试专家：复测第 2 轮，结论 🔄（核心功能通过，但发现 1 项需 🟧 补齐才达 auth/game 同等基线）。环境阻断已排除：本机 Docker 29.5.3 正常、Playwright 浏览器齐备（以 `required_permissions:["all"]` 脱沙箱执行）。
- 契约对账（复核无回归）：前端 `api/modules/channels.ts` 10 个调用 ↔ 后端 `transport/http/channels/{router,handler}.go` 方法/路径/权限码（channel.read/write）/DTO 字段/错误码（MARKET_CHANNEL_INCOMPATIBLE/CONFLICT/VALIDATION_FAILED）逐项一致；gameChannelId(int64) 路径口径、displayKey 仅展示一致。
- 真实回归证据（`WITH_DB=1 sh scripts/regression/run.sh`，完整权限）：docker PG 拉起✓；golang-migrate（go run 兜底）`1..5/u` 全部前向执行（含 `5/u channel_schema`）✓；seed common ✓；后端 `go test ./...` PASS（summary：pass 234 / fail 0）✓；前端 vitest 17 文件 / 68 用例✓。
- Playwright：全量默认并发（6 worker）11/17 失败（channels 2 + games 9 均 30s page-setup 超时，login/smoke 6 通过）；**隔离重跑 channels.spec.ts 2/2 通过**；**`--workers=1` 串行全量 17/17 通过（含 channels 2 + games 9 + 视觉基线）**——确证全量失败为并发 flakiness（单 vite server 被 6 worker 压垮），非 channel/games 缺陷。
- 红线核验：复制创建清空 secret/file + 强制 invalid（domain 单测 `TestNewCopiedMarketChannelClearsSensitiveFields` 验证）✓；运行态三态 `ResolveRuntimeFlags`（hidden/incompatible/invalid_config 口径，IncludedInSnapshot/Sync 一致）与 snapshot/sync 下游契约对齐✓；channel 自身 API DTO 不含 secret/file（密文属下游 account-auth/channel-login/product）→ S8 于 channel API 层 N/A；S2(401) 进程内场景通过✓。
- 阻断/遗留（移交说明见问题清单）：
  1) [测试基建·跨模块] 场景 harness（`internal/testkit/scenario`）用 `httpserver.New` 构造**进程内无 DSN/无 JWT** 的降级 handler，`SCENARIO_WITH_DB=1` 不会连库也不签发 `auth.role` 令牌——实证：置 1 后 channel.yaml 24 个 requiresDB 用例全部 401 失败。故 S1/S3/S4/S5/S6/S7/S9/S10 的连库执行当前不可达（auth/game 亦带此遗留）。
  2) [覆盖缺口·channel 专项] channel **缺少 auth(`admin_http_test.go`)/game(`games_http_test.go`) 同款内存 L3 httptest**；app/channel、infra 仓储均无 _test.go。S1/S3/S4/S5/S6/S7/S9/S10 行为维度既无连库执行、也无内存等价覆盖，低于 auth/game 基线。
  3) [回归基建 flakiness] Playwright 默认 6 worker 致全量超时；建议 `playwright.config.ts`/run.sh 钳制 workers 或加大 webServer 预热/超时。
- 通过判定：契约/功能/真实环境证据通过；建议 🟧 补 1 项（channel 内存 L3 httptest 达 auth/game 同等基线）后放行 ✅；问题 1、3 为跨模块基建遗留，建议总负责 Agent 统一排期。

## 12 · channel — 2026-06-25 测试专家复跑第 3 轮（覆盖闭合，放行）
- 🟪 测试专家：复测第 3 轮，结论 ✅ **通过，可进入功能验收**。首轮唯一差距（问题2：channel 缺内存 L3 httptest）已由 🟧 闭合。
- 覆盖闭合确认：新增 `channels/memstore_test.go`（内存仓储 + TxManager 克隆回滚 + fakeAudit spy）+ `channels/channels_http_test.go`（26 个 httptest，真实 JWT/chi/中间件/权限链）。维度齐全且达 auth/game 同等基线：S1（空白/复制创建·列表·详情·编辑·隐藏/恢复·包 CRUD）、S2（10 端点无令牌 + 伪造 Bearer 401）、S3（read-only 令牌对全部写 403 + 无 channel.read 读 403）、S4（空 channelId/缺 copyFromMarket/非法 mode/不存在 channel/remark 超长/非法 market 路径/非法 query×4/非法 int64 路径/包 market 不一致）、S5（重复实例 CONFLICT + 隐藏非法态 CONFLICT + 包 code 重复 CONFLICT + MARKET_CHANNEL_INCOMPATIBLE 双向）、S7（spy 断言 channel.create/update/hide/unhide + package.create/update）、S9（pageSize 999→100 钳制）、S10（复制来源缺失 InTx 回滚 + 列表无副作用 + 回滚未占用序号）；S6 跨env（env 回显 + schema 隔离交连库 harness）/ S8 脱敏（channel API 不返密文，附带断言响应不含 secretConfig/normalConfig/fileConfig）本层 N/A 并注明。
- 本轮实跑证据：`go test ./internal/transport/http/channels/... -count=1 -v` → 26 channels httptest + 2 bool 解析全 PASS（`ok ... channels`）；`go build ./... && go vet ./... && go test ./... -count=1` 全绿（channels/games/admin/...所有包 ok，0 失败）。未改 channel 业务实现、未触碰 admin_wiring/games（account-auth 并行区）。
- 重申首轮已确认（无需重跑）：契约对账无漂移；真实环境证据 docker PG/迁移(含 channel_schema)/后端 go test 234/vitest 68/Playwright 串行 17/17 均通过。
- 遗留（与 auth/game 同口径，跨模块测试基建，不阻塞 channel，由总负责统一排期）：① 场景 harness 连库执行（requiresDB 用例进程内不可达）；② Playwright 默认 6 worker 并发 flakiness（建议钳制 workers）。
- 通过判定：**是**。channel 9 阶段集成测试通过，移交 ✅ 功能验收。

## 12 · channel — 2026-06-25 收尾验收通过（9 阶段全✅）

- 🟦 后端：开发✅ / CR✅ / 测试✅。API（渠道实例创建 空白+复制 / 候选 / 列表过滤 / 详情 / 编辑 / 隐藏·恢复 / 渠道包 CRUD），权限码 channel.read·channel.write；迁移 000005_channel_schema；domain 纯函数 ResolveRuntimeFlags(三态)·兼容性规则·隐藏状态机·复制清空 secret/file 强制 invalid。补 query bool 校验(400)、channel.yaml。
- 🟩 前端：开发✅ / CR✅(直修4项：候选集合拉全量含隐藏 / 复制创建 v-perm / canWrite computed / 行信息口径) / 测试✅。views/channels(列表/详情抽屉/创建抽屉/运行态标签)；vitest 18 文件 83 用例；Playwright channels 2 + 视觉基线。
- 🟪 测试专家：3 轮。首轮诊断「环境阻断」实为子 agent 沙箱权限(docker/playwright 本机可用)；脱沙箱复跑真实证据全绿(docker PG/5迁移前向含 channel_schema/后端 go test 234/vitest 68/Playwright 串行 17/17)，契约对账无漂移；唯一差距=L3 httptest 行为维度覆盖低于 auth/game 基线 → 交 🟧。第3轮放行。
- 🟧 全栈修复：补 channel 内存 L3 httptest（memstore_test + channels_http_test，26~27 用例，真实 JWT/chi/中间件/权限 + spy 审计 + TxManager 回滚），覆盖 S1/S2/S3/S4/S5/S7/S9/S10（S6/S8 本层 N/A 注明），未发现实现缺陷，未触碰并行区(admin_wiring/games)。
- ✅ 验收：通过 18/18。后端 build/vet/test 全绿、前端 build + vitest 83 全绿、channel e2e 串行 2/2。
- 遗留（非阻断，与 auth/game 同口径的跨模块基建）：Playwright 默认 6 worker 并发 flakiness（建议钳制 workers + 行级显式等待 + 端口回收）；scenario harness 连库（requiresDB 用例需 SCENARIO_WITH_DB=1+PG，已由内存 L3 等价覆盖）；audit sink wiring nil 待模块 22 接通（spy 断言）。
- 下游关注项（移交 snapshot 20 / sync 21 验收时统一处理，非 channel 未达项）：`GameMarketChannel.IncludedInRuntimeConfig()`（供 snapshot/sync 合并）仅判 `!hidden && valid`，**未含兼容性维度**；channel 自身 API 用含 compatible 的 ResolveRuntimeFlags 正确。当前实例创建即经兼容校验无差异，但若 region 规则变动使既存实例变不兼容，简化方法可能让其仍进快照/同步。建议下游改用含兼容性判定或显式记录该假设。

## 13 · account-auth — 2026-06-25 收尾验收通过（9 阶段全✅）

- 🟦 后端：开发✅ / CR✅(打回1项B5审计→返工闭合；CR直修B1-B4:密文二次加密跳过/login_mode过滤/mergeConfigWithExisting留空不修改/secret·file隐式必填/formSchema label·order) / 测试✅。API 4（GET account-auth/types·channels/{id}/account-auth-types·games/{id}/account-auth-configs[game.read]、PUT games/{id}/account-auth-configs[game.write] 整体替换）；错误码 VALIDATION_FAILED/ACCOUNT_AUTH_TYPE_NOT_ALLOWED/TEMPLATE_NOT_FOUND/SECRET_ENCRYPT_FAILED（CONFLICT 决策移除=last-writer-wins）；迁移 000006（platform 三表+game_account_auth_configs+**承接 channels/channel_policies 归位 platform**）；domain 纯函数 ValidateConfigAgainstTemplate（empty/invalid/valid 三态）+13 单测。后端测试 21 L3 httptest + account-auth.yaml(29~30 用例) + fixtures 挂回归入口。
- 🟩 前端：开发✅ / CR✅(直修3项:渠道并集对齐/密文恒显masked/password组件) / 测试✅。games 详情真实「自有账号认证」Tab（AccountAuthTab.vue + api/modules/accountAuth.ts：四件套渲染/secret 恒显 masked+留空不修改/config_status 三态/启用 invalid 告警/locked 禁编/无 game.write 置灰/整体保存回填）。vitest 新增 15 + 全量 83。
- 🟪 测试专家：2 轮。首轮:并集疑点裁定=**误报**（后端 GetGameConfigs→mergeViews 遍历 allowed 并集回填空行，前端以 GET 为行源等价并集，符合 compact）；契约无漂移；发现 1 阻断=迁移 000006 干净库失败（platform.channels 未归位）。第2轮:迁移闭合复核 + 查清 🟧 报的「前端15 E2E 根节点不渲染」=Playwright 6-worker 并发 flake（非真实回归，串行 15/15 全过）→放行。
- 🟧 全栈修复：修迁移 000006——顶部承接 channels/channel_policies 归位 platform（遵守「不改历史」放首个硬依赖者）+ down 对称反向（seed 反删 + schema 反向）；迁移链 1..6 干净往返 version=6、go test 全绿、psql 校验归位、不破坏 channel/game（裸名+search_path 解析正常）。
- ✅ 验收：通过 20/20。后端 build 净 + go test 301、前端 vitest 83、迁移链 1..6 干净、account-auth Tab 串行 4/4。密文恒 masked、明文绝不落库/外泄。
- 遗留（非阻断，与 auth/game/channel 同口径跨模块基建）：① **Playwright workers 未钳制（最高优先）**——playwright.config.ts `fullyParallel:true` 无 workers 上限 + frontend.sh 直接 `pnpm e2e`，单 vite+MSW 被 6-worker 压垮致根视图不挂载、统一回归前端段全量误红；建议 config 固定 `workers:1`（或 CI 钳制）使 run.sh 默认串行变绿。② vite 冷编译预热 flake（建议 webServer 预热/放宽 gotoGames 超时）。③ game 模块「法务链接」动画 element-not-stable flake（game 侧）。④ scenario harness 未连库（requiresDB 用例跳过，内存 httptest 等价覆盖，待 SCENARIO_WITH_DB=1 落地）。⑤ audit sink wiring nil（service 层审计就绪，待模块 22 接通）。
