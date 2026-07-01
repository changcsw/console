# feature-plugin（#15）集成清单

> 维护者：开发 / CR / 测试 / 集成角色共同维护。总负责 Agent 仅初始化骨架。
> 主事实源：`docs/architecture/v2/modules/15-feature-plugin/spec.compact.md`。

## 1. 模块路由 / API / 页面入口

> 由 🟦/🟩 开发角色在实现后填写真实方法/路径/权限码/页面路由。

- 后端 API：
  - GET  `/api/admin/game-channels/{gameChannelId}/plugins`（`plugin.read`）
  - POST `/api/admin/game-channels/{gameChannelId}/plugins`（`plugin.write`，audit=`plugin.configure`）
  - PATCH `/api/admin/game-channel-plugins/{id}`（`plugin.write`，audit=`plugin.configure`）
  - GET  `/api/admin/channel-packages/{packageId}/plugins`（`plugin.read`）
  - POST `/api/admin/channel-packages/{packageId}/plugins`（`plugin.write`，audit=`plugin.configure`）
  - 错误码：`MARKET_CHANNEL_INCOMPATIBLE` / `VALIDATION_FAILED` / `CONFLICT`
- 前端页面入口：
  - `apps/admin-web/src/views/channels/components/ChannelInstanceDetailDrawer.vue`：新增「功能插件」Tab（与基础设置/渠道包/渠道登录并列）。
  - `apps/admin-web/src/views/channels/components/FeaturePluginConfigPanel.vue`：渠道实例插件列表、模板驱动配置、必接引导、scope 提示、plugin.write 权限置灰。
  - `apps/admin-web/src/views/channels/components/ChannelPackageDetailDrawer.vue`：渠道包插件“继承渠道插件/自定义覆盖”开关与覆盖保存。
  - `apps/admin-web/src/api/modules/channels.ts`：补齐 feature-plugin compact 契约 API client（GET/POST/PATCH）。

## 2. 引用模块 / 外部依赖 / 共享 surface

- depends_on：`channel`(#12)、`game`(#11)、`common`(#00)。
- 强依赖 #12 channel：`GameMarketChannel`（渠道实例）、`channel_packages`（渠道包）作为挂载点；`config_status` 三态、隐藏态。
- 共享 surface（channels-surface lane）：
  - `services/admin-api/internal/domain/channel`
  - `services/admin-api/internal/transport/http/channels`
  - `apps/admin-web/src/views/channels`
- 本模块新增 domain：`services/admin-api/internal/domain/plugin`。

## 3. 需要统一接入的共享文件（集成 Agent 统一整合）

> 本 worktree 仅做本地验证修改；最终由集成 Agent 统一合并，避免与 #18 game-cashier 冲突。

- [x] `services/admin-api/internal/transport/httpserver/admin_wiring.go`（plugin handler/repo 装配）—— **与 #18 game-cashier 并行可能同时改，记录此处变更**
- [x] `services/admin-api/internal/transport/http/channels/router.go`（plugin 子路由挂载）
- [x] `services/admin-api/internal/transport/http/channels/handler.go`（扩展 plugin handler）
- [x] `apps/admin-web/src/views/channels/`（扩展 ChannelInstanceDetailDrawer Tab + 渠道包插件覆盖区）
- [ ] `apps/admin-web/src/router/routes.ts`（如需新路由）

### 共享文件实际改动记录（开发角色填写）

| 文件 | 改动摘要 | 责任角色 | 备注 |
| --- | --- | --- | --- |
| `services/admin-api/internal/transport/httpserver/admin_wiring.go` | 注入 `plugin.Service` + `postgres.NewPluginStore`，并挂到 channels handler | 🟦 后端开发 | 与 #18 可能冲突，集成阶段按文件块合并 |
| `services/admin-api/internal/transport/http/channels/router.go` | 新增 5 条 plugin 路由，权限码 `plugin.read/plugin.write` | 🟦 后端开发 | 路由仍挂在 channels 子域 |
| `services/admin-api/internal/transport/http/channels/handler.go` | 新增 plugin DTO/handler 与错误映射 `pluginapp.Error` | 🟦 后端开发 | 复用统一包络与 `writeError` |
| `apps/admin-web/src/views/channels/components/ChannelInstanceDetailDrawer.vue` | 新增「功能插件」Tab 并挂载 `FeaturePluginConfigPanel` | 🟩 前端开发 | 使用 `plugin.write` 权限控制编辑 |
| `apps/admin-web/src/views/channels/components/ChannelPackageDetailDrawer.vue` | 新增渠道包插件覆盖区（inherit/custom + 模板驱动编辑） | 🟩 前端开发 | 复用 `TemplateConfigRenderer` |
| `apps/admin-web/src/api/modules/channels.ts` | 新增 feature-plugin API DTO 与 client（camelCase） | 🟩 前端开发 | 对齐 compact 契约 5 个接口 |

## 4. 数据库迁移编号协调

- 迁移从 `000012` 起追加。
- ⚠️ #18 game-cashier 在另一 worktree 并行，可能同样使用 `000012`。本 worktree 先用 `000012`；合并 main 前须与 game-cashier 协调最终编号。
- 实际使用序号（本 worktree）：`000012`
- 文件：
  - `services/admin-api/migrations/000012_feature_plugin_schema.up.sql`
  - `services/admin-api/migrations/000012_feature_plugin_schema.down.sql`

## 5. 已知问题 / 待完善点

- `apps/admin-web` 全量 `vue-tsc --noEmit` 当前存在既有问题（与 #15 无关，非本模块责任）：
  - `src/api/modules/cashier.ts(173)` 类型断言冲突；
  - `src/views/games/detail/__tests__/sync-section-drawer.spec.ts(23)` 类型不匹配。
- **P2（#15 前端 CR 登记）**：渠道包插件覆盖区 file 字段经 `TemplateConfigRenderer` 仍为文本输入，未复用 `el-upload` 统一上传；实例侧 `FeaturePluginConfigPanel` 已正确实现，可随 TemplateConfigRenderer 统一增强。（前端测试复核：未引入新缺陷，本阶段以组件级 + 契约 mock UI 用例为主，P2 维持跟进。）
- 前端测试已补全（frontend-test）：vitest 本模块 33/33（原 16→新增 17，未破坏既有）；Playwright `tests/frontend/e2e/feature-plugin.spec.ts` 3/3 含截图基线，覆盖徽标/必接引导/selectable 强制/locked/scope/密文/file/继承覆盖/权限置灰/空错态。
- 后端 CR 遗留（非阻断）：~~`domain/plugin` 缺单元测试~~ ✅ 后端测试已补 L1 单测（compatibility_test.go + plugin_config_test.go，15 Test/32 子用例 PASS）；列表接口按插件 N+1 拉模板（可后续 batch）；审计仅写 `plugin.configure`（未拆分 enable/disable）；迁移 `000012` 与 #18 编号协调仍待集成阶段。
- ~~后端测试登记疑似实现缺陷 **P3（非阻断）**：`ResolvePluginConfigStatus` 的 allowed 集合未纳入 secret/file 字段键~~ ✅ **已修复（fullstack-fix）**：`domain/plugin/plugin_config.go` 遍历 `SecretFields`/`FileFields` 时同时写入 `allowed`，纯规则无 IO、签名不变；新增用例 `TestResolvePluginConfigStatus_SecretFileOnlyFieldsAllowed`。
- ~~**I-1（集成测试发现 / 契约漂移 / 建议阻断，回退 🟧 高级全栈）**：渠道包插件覆盖项 config vs configJson 漂移~~ ✅ **已修复（fullstack-fix）**：后端 `PackagePluginItemView.Config` → `ConfigJSON`（`json:"configJson"`），赋值点同步（`query_list_channel_plugins.go`/`command_override_package_plugin.go`）；与实例项 `ChannelPluginItemView.ConfigJSON` 及前端 `ChannelPackagePluginItem.configJson` 口径统一，请求侧 `config` 入参不变（符合 compact POST 契约）；新增 `app/dto/plugin_test.go` 序列化回归。待回 🟪 复测渠道包覆盖 round-trip。
- **P2（维持非阻断遗留）**：渠道包插件覆盖区 file 字段经 `TemplateConfigRenderer` 仍为文本输入，未复用 `el-upload`。fullstack-fix 修 I-1 仅触及后端 DTO/app 层，**未改动** `ChannelPackageDetailDrawer.vue`，无「顺带极低成本」前提，按约束不扩大改动范围 → 保留为后续增强。
- **功能验收复核（✅ acceptance）**：上述 P2 与「审计 plugin.enable/disable 未拆分」均维持为**非阻断遗留**；连库 harness 为前向声明（S3/S6/S10 SKIP），以契约对账 + L1 域单测（32 子用例）+ 组件/e2e（33+3）替代覆盖。**既有非 #15 阻塞**（cashier.ts:173 / sync-section-drawer.spec.ts:23）明确不计入 #15。

## 6. 集成步骤 / 验证命令 / 风险说明

- 后端：`go build ./... && go vet ./... && go test ./...`（在 `services/admin-api`）
- 前端：`pnpm -C apps/admin-web build && pnpm -C apps/admin-web test`
- 场景矩阵：`tests/backend/scenarios/feature-plugin.yaml`（5 接口×S1–S10）；进程内 manifest 解析 + S2 401 已 PASS；连库执行 `SCENARIO_WITH_DB=1`。
- fixtures：`tests/fixtures/common/feature-plugin.sql`（自动挂 `scripts/regression/db.sh`）+ `tests/fixtures/sandbox/feature-plugin.sql`（连库前向声明）。
- 后端测试运行结论（backend-test）：`go test ./...` ✅ 全包通过 0 失败；domain/plugin L1 32 子用例 PASS；feature-plugin manifest 解析 OK。
- 前端测试运行结论（frontend-test）：vitest 本模块 3 文件 33/33 ✅（`node_modules/.bin/vitest run <三件套>`）；vitest 全量 215/216（唯一失败 sync-section-drawer.spec.ts 为既有阻塞，非 #15）；Playwright `E2E_PORT=5193 playwright test feature-plugin.spec.ts` 3/3 ✅（含 toHaveScreenshot 基线比对）。环境注意：仓内 `pnpm exec` 会误触根目录 install，改走 worktree 本地 bin；vite 需写 node_modules 临时文件、Playwright 端口 5187 可能被并行 worktree 占用（用 E2E_PORT 切换）。
- 风险：与 #18 game-cashier 在 `admin_wiring.go` 与迁移编号上的潜在冲突，统一交集成阶段处理。
- 集成测试运行结论（integration-test，🟪 测试专家）：
  - 契约对账：5 接口方法/路径/权限码/请求 DTO/错误码/包络/实例项响应字段**全一致**；**发现 1 处漂移 I-1**（渠道包覆盖 `config` vs `configJson`，见 §5）。
  - 后端回归：`go build ./... && go test ./...` ✅ 全包 0 失败（复跑）；`scenario feature-plugin` manifest 解析 OK + S2 进程内 401 PASS。
  - 前端回归：模块 vitest 33/33 ✅；全量 215/216（唯一失败 `sync-section-drawer.spec.ts` 既有非 #15）；Playwright `feature-plugin.spec.ts` 3/3 ✅（截图基线绿）。
  - 红线：scope=server 标注 / 隐藏·不兼容·非 valid 三标全 false（`ResolveRuntimeFlags`）/ secret masked / 权限 plugin.read·write / 跨 env 无 schema 前缀 / `InTx` 回滚 / required 缺口引导 —— 代码+单测+组件/e2e 核验通过（S3/S6/S10 运行待连库）。
  - **连库限制**：scenario 连库 harness 为前向声明（`scenarios_test.go` 即便 `SCENARIO_WITH_DB=1` 仍以无 DSN 的 `httpserver.New` 装配），本机缺 `golang-migrate`、无本地 PG（docker 可用但 harness 无 DSN 注入）→ requiresDB 用例继续 SKIP；以契约对账 + L1 域单测 + 进程内 S2 + 前端 vitest/Playwright 替代覆盖。
  - 下游抽查：snapshot(#20) 未落地、sync(#21) 为 section 骨架，无消费方可对账；口径单一来源 `ResolveRuntimeFlags` 三标同口径，列前向兼容备忘。
  - **通过判定：否**（暂不进入功能验收）—— 需先修复 I-1（建议顺带 P3）后回 🟪 复测渠道包覆盖 round-trip；主链路/红线/回归均通过。
- 全栈修复运行结论（fullstack-fix，🟧 高级全栈）：
  - I-1 已修（后端字段 `Config`→`ConfigJSON` json:"configJson"，前后端响应键统一）；P3 已修（allowed 并入 secret/file 键）；P2 维持非阻断遗留（未触及 ChannelPackageDetailDrawer，不扩大改动）。
  - §3 共享文件：本次修复仅改本模块专属 `app/dto`、`app/plugin`、`domain/plugin` 文件，**未新增共享 surface 改动**（handler/router/wiring/前端视图均未动）。
  - 自检：`go build ./... && go vet ./... && go test ./...` ✅ 全包 0 失败；vitest 本模块三件套 33/33 ✅；`vite build` ✅。
  - 待回 🟪 复测渠道包覆盖 round-trip 后进入功能验收。
- 第 2 轮复测结论（integration-retest，🟪 测试专家）：
  - **I-1 已闭合**：后端包覆盖响应键 `configJson`（dto/plugin.go:76）与前端读取键（channels.ts:275 / drawer:297）完全一致；query/command 两处赋值同步；实例项 service.go:157 未误伤；POST 入参仍 `config`（compact）；`app/dto/plugin_test.go` 两用例 PASS。
  - **P3 已闭合**：plugin_config.go:52,57 secret/file 键并入 `allowed`；`TestResolvePluginConfigStatus_SecretFileOnlyFieldsAllowed` PASS，既有用例无回归。
  - 回归复跑：`go build ./...` ✅；`go test ./...` ✅ 全包 0 失败（含 dto 序列化回归 + domain/plugin + scenario + transport/http/channels）；前端模块 vitest 33/33 ✅。
  - Playwright 未重跑（前端代码未改、mock e2e 与第 1 轮 3/3 等价）；契约对账现无漂移；红线复核仍成立（S3/S6/S10 运行待连库 harness）。
  - **通过判定：是（可进入 ✅ 功能验收）**——I-1/P3 闭合、回归红线全绿；仅余 P2 + 迁移 000012 编号协调 + 连库 harness 为非阻断遗留。
- 功能验收运行结论（acceptance，✅ 功能验收师 / Cursor Auto · 第 3 轮）：
  - **验收清单 29 项全 PASS / 0 FAIL**（数据模型 6 / API 7 / 状态机规则 4 / 前端 7 / 红线 4 / 下游 1；含 AC-13 审计粒度一条非阻断观察）。详见 `audit.log.md` §功能验收。
  - 构建测试真实输出：`go build/vet/test ./...` ✅ 全包 0 失败；`domain/plugin` 32 子用例 PASS；`scenario feature-plugin` S2×5 进程内 401 PASS + requiresDB SKIP；前端模块 vitest **33/33** ✅；全量 **215/216**（唯一失败 `sync-section-drawer.spec.ts` 既有非 #15）；`vite build` ✅；Playwright `feature-plugin.spec.ts` **3/3** ✅（视觉基线绿）。
  - 操作主线（02-operation-flow B-步骤 4「加功能插件·引导必接」）走查：能力闭环 / 三态流转 / 错误冲突如约 / 脱敏权限生效 / 必接缺口挡快照同步 —— 均成立。
  - **结论：验收通过（PASS）**。遗留 P2（包覆盖 file 未走 el-upload）/ 迁移 000012 与 #18 编号协调 / 连库 harness 前向声明 / 审计 enable·disable 未拆 —— 均非阻断。
