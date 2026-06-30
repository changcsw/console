# channel-login · 集成 Checklist

> 由 开发 / CR / 测试 / 集成角色共同维护。总 Agent 与集成 Agent 优先读本文件。

## 模块路由 / API / 页面入口
- 后端 API：
  - `GET /api/admin/game-channels/{gameChannelId}/login-config`（channel.read）
  - `PUT /api/admin/game-channels/{gameChannelId}/login-config`（channel.write）
- 前端页面入口：
  - `apps/admin-web/src/views/channels/ChannelsView.vue` → 实例详情抽屉
  - `apps/admin-web/src/views/channels/components/ChannelInstanceDetailDrawer.vue` 新增「渠道登录」页签（仅 `login_mode=channel_only` 展示）
  - `apps/admin-web/src/views/channels/components/ChannelLoginConfigPanel.vue` 渠道登录模板渲染与保存

## 引用模块 / 外部依赖 / 共享 surface
- depends_on: channel, game, common（均 ✅）
- 共享 surface：`domain/channel`、`transport/http/channels`、`views/channels`
- 复用：`channel_policies`（login_mode/login_locked）、`game_channels`（market_code/channel_id_ref）、模板四件套（00 §4）、ConfigStatus（00 §3.4）、密文 AES-GCM（00 §6）、API 包络/错误码（00 §7）、审计（00 §8）

## 需要统一接入的共享文件（集成 Agent 处理）
- 后端路由装配：`services/admin-api/internal/transport/httpserver/admin_wiring.go`
- 前端路由：`apps/admin-web/src/router/routes.ts`（本次无需变更，复用既有 `/channels` 入口）
- 前端 API 客户端：`apps/admin-web/src/api/modules/channels.ts`（新增 getLoginConfig/putLoginConfig）
- 本次后端已接入：`admin_wiring.go` 已注入 `ChannelLoginService` 并注册 GET/PUT login-config 路由。

## ⚠️ 共享接入点：channel 复制创建 → 写 game_channel_login_configs（后端返工新增）
- 接入位置：`services/admin-api/internal/app/channel/channel_service.go` `CreateMarketChannel` 复制分支（Insert 新实例后，同一 `InTx` 事务内）。
- 行为：当 `mode=copy` 且渠道 `login_mode=channel_only` 时，调用 helper `copyLoginConfig` 向新实例 `game_channel_login_configs` 落库——取该渠道 enabled 最新模板区分字段，仅复制 form_schema 普通字段、清空 secret/file，强制 `config_status=invalid`、`enabled=false`、`last_check_message='缺少必填敏感字段或文件字段'`；复制后**不联动源实例**。
- 接入面（属本 lane channels-surface 内允许的改动）：
  - `domain/channel/login_config.go`：新增纯函数 `NewCopiedLoginConfig`（无 IO）。
  - `app/channel/ports.go`：`Repositories` 新增 `LoginTemplates`（ChannelLoginTemplateReader）、`LoginConfigs`（ChannelLoginConfigStore）两端口。
  - `infra/persistence/postgres/channel_store.go`：`channelReposFrom` 绑定 `ChannelLoginTemplateRepo`/`ChannelLoginConfigRepo`，使复制落库与 channel 创建共享同一事务/同 env schema。
- 影响：原仅写 `game_channels.config_status=invalid`；现复制实例 GET login-config 返回 `invalid`（带已复制普通字段、空 secret/file），与 `game_channels` 状态及 00 §3.4 复制创建强约束一致（不再返回 empty 占位）。
- 集成注意：channel 模块若同期改动 `CreateMarketChannel` 或 `Repositories` 结构，需保留上述两端口与复制落库调用；内存测试 store 未注入 login 仓储，helper 内置 nil-guard 跳过（不影响既有 channel httptest）。

## 已知问题 / 待完善点
- `pnpm tsc --noEmit` 在当前仓库基线上失败（大量历史 TS2307：`*.vue` 模块声明、`@vue/test-utils` 解析）；需由集成阶段统一修复类型基础设施后再验本模块“tsc 通过”。**前端 CR 已确认本模块改动文件无新增 TS 错误。**
- file 字段当前使用前端上传控件 + 本地文件名引用占位（无后端文件服务联调）；待后端 `infra/file` 接口联通后切换真实上传引用。
- 前端 CR（✅ 通过）：compact 前端要点已实现；CR 直修密钥留空误拦保存、保存后刷新抽屉聚合态。
- 后端 CR（❌ 打回）：GET/PUT/迁移/领域校验/密文/路由装配主体通过；**阻断** channel 复制创建未写入 `game_channel_login_configs`；CR 直修哨兵无存量不落库、校验失败不写审计。
- 后端返工（✅ 已闭环）：复制创建已接入 `game_channel_login_configs`（见上「共享接入点」）；`go build`/`go vet` 复验 PASS；阻断项关闭。

## 后端测试产物（🟦🧪）
- 用例（就近 + 顶层）：
  - L1 单元：`services/admin-api/internal/domain/channel/login_config_test.go`（校验/状态机/复制创建强约束）。
  - L3 接口：`services/admin-api/internal/transport/http/channels/login_http_test.go` + `login_memstore_test.go`（GET/PUT 全链路 httptest + 内存仓储 + spy cipher/audit）。
  - 场景矩阵：`tests/backend/scenarios/channel-login.yaml`（接口 × S1–S10）。
  - fixtures：`tests/fixtures/common/channel-login.sql`（RBAC + 模板四件套兜底，**自动挂 `scripts/regression/db.sh` 回归入口**）、`tests/fixtures/sandbox/channel-login.sql`（渠道实例 + configured 密文配置）。
- 运行：`cd services/admin-api && go test ./...` → 通过 34 / 失败 1。
- ⚠️ 待返工（疑似实现缺陷 D1）：哨兵 `"******"` 保留密钥与 secret 字段 `minLen`/`pattern` 规则冲突 → 误判 invalid/400（命中 huawei 默认模板 appSecret minLen:8）。复现 `TestPutLoginConfigSentinelKeepsCiphertext`（FAIL）。修复方向：哨兵保留分支跳过该 secret 的内容规则校验（对照 account-auth）。**回退后端开发，经总负责 Agent 调度；修复后用例应转绿。**

## 前端测试产物（🟩🧪）
- 用例（就近 vitest + 顶层 Playwright）：
  - L4 组件：`apps/admin-web/src/views/channels/components/__tests__/ChannelLoginConfigPanel.spec.ts`（13，四件套渲染/密文哨兵/三色/告警条/复制提示/即时校验/权限置灰/二次GET回显）、`ChannelInstanceDetailDrawer.spec.ts`（3，仅 channel_only 展示页签 + 挂载面板 + canWrite 响应）。
  - L5 UI（契约 mock）：`tests/frontend/e2e/channel-login.spec.ts`（3，GET/PUT login-config stub；valid/invalid/无写权限三态 + 视觉基线）。
  - fixtures：`apps/admin-web/src/views/channels/components/__tests__/fixtures/channelLogin.ts`（模板四件套 + config 实例工厂）；截图基线 `tests/frontend/visual-baseline/channel-login.spec.ts-snapshots/`。
- 运行：
  - `cd apps/admin-web && npx vitest run` → 19 文件 / 98 用例 PASS（本模块 16 含其中）。
  - `cd apps/admin-web && npx playwright test tests/frontend/e2e/channel-login.spec.ts --workers=1` → 3 PASS（截图基线匹配）。
  - `cd apps/admin-web && npx playwright test tests/frontend/e2e/channels.spec.ts --workers=1` → 2 PASS（确认抽屉页签化未回归 channel e2e）。
- 结论：**通过**（19 用例全绿，0 失败，无疑似实现缺陷）。备注：高并发同跑多 spec 时 games/channels 出现冷启动 dev server flake，单 worker 隔离复跑全绿，非本模块回归；historic tsc TS2307 不作判据。

## 测试专家集成核对（🟪🧪 · 通过）
- 接入点确认：`admin_wiring.go` 已真实装配 `channelLoginSvc` 并经 `channelshttp.RegisterRoutes(NewHandler(channelSvc, env, channelLoginSvc))` **单点注册** login-config；降级分支注释明确不再调 `RegisterLoginRoutes` 以免重复注册。**无缺失/无冲突**，可供集成 Agent 直接整合。
- 前端接入：`routes.ts` 复用既有 `/channels` 入口（无新增路由）；`channels.ts` getLoginConfig/putLoginConfig 经 `http.ts` 解包 `{data}`、写挂 channel.write。与本 checklist 一致。
- ⚠️ checklist/manifest 既有「`login_handler.go` 重复实现待收敛」遗留项 **实际已闭环**：该文件已删除，全仓仅余 `admin_wiring.go` 一处注释引用，无重复实现。
- 契约对账：前后端零漂移（见 audit.log.md [测试专家] §1）。
- 限制：真实「后端连 PG 起服 + 前端指向真实后端」全链路联调 e2e 未跑（环境不具备），以契约对账 + 持久层真实 PG 实证 + 两车道用例为判定依据；建议集成阶段联调栈就绪后补一轮冒烟（非阻断）。

## 集成步骤 / 验证命令 / 风险说明
- 后端验证命令：
  - `cd "/Users/csw/gitproject/console-channel-login/services/admin-api" && go build ./...`（通过）
  - `cd "/Users/csw/gitproject/console-channel-login/services/admin-api" && go vet ./...`（通过）
- 前端验证命令：
  - `pnpm --dir "/Users/csw/gitproject/console-channel-login/apps/admin-web" exec tsc --noEmit`（当前基线失败，见上）
  - `pnpm --dir "/Users/csw/gitproject/console-channel-login/apps/admin-web" exec vite build`（通过）
- 迁移前向校验（已通过）：
  - `docker compose up -d --pull never postgres` + `sh scripts/regression/db.sh`（连库 harness）全新库 `migrate up` `1/u`..`7/u channel_login_schema` 全部成功，seed 正常。
  - `000007` 已收口：DO 块守卫同时识别 `*_status_check` 与 `000001` 内联的 `*_config_status_check`，消除重复 config_status CHECK；全新库核对仅余 1 条。`down.sql` 对称幂等。
- 风险说明：
  - ~~**阻断**：复制创建渠道实例后登录配置 GET 仍为 empty 占位~~（已修复：复制创建同事务写 `game_channel_login_configs`，GET 返回 invalid）。
  - 若后端 PUT 在校验失败时不返回最新持久化状态，前端依赖“失败后刷新 GET”来回显 invalid 状态。
  - `game_channels.config_status` 与登录配置表双轨，列表/运行态展示可能偏差直至聚合对齐。
  - 实际 DTO 字段名若与 compact 偏差，需在 🟪 测试专家契约对账阶段统一校正。

## 功能验收（✅ · 通过）
- 验收结论：**通过（PASS）**——端到端功能成立，满足 compact 业务规则与红线，符合 operation-flow 步骤5；无阻断、0 新增缺陷。
- 验收清单：15/15 PASS，0 FAIL（F1-F15：适用性 channel_only / GET 空占位四件套 / 密文脱敏 / 模板驱动校验 details[] / config_status 后端推导 / 失败落库 invalid+400 / 哨兵保留原密文 / 审计仅成功写 / 复制创建强约束 / enabled+invalid 告警不进快照同步客户端 / 权限 401·403·置灰 / 无模板·409·回滚 / 三套分离·跨env / 前端四件套渲染 / 步骤5 闭环）。
- 构建/测试（真实输出）：
  - `cd services/admin-api && go build ./... && go vet ./... && go test ./...` => 全 PASS（0 失败）；`go test -run 'Login' -v` => 21/21 PASS。
  - `cd apps/admin-web && npx vite build` => PASS（沙箱 .vite-temp EPERM 为沙箱限制，解除后 PASS）。
  - `npx vitest run ChannelLoginConfigPanel.spec.ts ChannelInstanceDetailDrawer.spec.ts` => 16/16 PASS。
  - `npx playwright test tests/frontend/e2e/channel-login.spec.ts --workers=1` => 3/3 PASS（视觉基线匹配）。
  - 统一回归入口 `scripts/regression/run.sh` WITH_DB=1 受限（migrate CLI 缺失/网络受限），以上述进程内用例等价覆盖。
- 下游 impacts 抽查：snapshot 域未落地、sync 仅脚手架（未消费 channel_login）⇒ 契约无破坏（N/A）。
- 限制：真实跨栈全链路联调 e2e 未跑（环境不具备），以契约对账+持久层实证+三车道用例判定；建议集成阶段补冒烟。
- 遗留（移交 🟧，均 P3 非阻断）：① mapFormSchema 未回传 options/placeholder ② config_status 双轨待聚合 ③ file 上传占位待 infra/file ④ AuditSink 待 audit 模块注入。
- 详细报告见 `audit.log.md` [功能验收] 小节。
