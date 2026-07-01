stage: acceptance / ✅（功能验收通过·第 3 轮·验收闸门关闭）
module: feature-plugin (#15)
worktree: /Users/csw/gitproject/console-feature-plugin (codex/feature-plugin)，未 commit/未切分支/未改业务代码
verdict: 通过（PASS）— 端到端验收清单 29 项全 PASS / 0 FAIL（数据模型6/API7/状态机规则4/前端7/红线4/下游1；AC-13 审计粒度为非阻断观察）
基准: 功能端到端可用 + 满足 compact 业务规则 + 符合 02-operation-flow B-步骤4 主线（能力闭环/三态流转/错误冲突如约/脱敏权限/必接缺口挡快照同步）
build/test: go build/vet/test ./... ✅ 全包0失败；domain/plugin 32 子用例 PASS；scenario S2×5 进程内401 + requiresDB SKIP；前端模块 vitest 33/33 ✅；vite build ✅；Playwright 3/3 ✅(视觉基线)
回归既有阻塞: 全量 vitest 215/216 唯一失败 sync-section-drawer.spec.ts(既有非#15)；vue-tsc cashier.ts(173) 同口径——本模块文件不受影响，不计入 #15 FAIL
红线: scope=server 不下发 / 隐藏·不兼容·非valid 三标false不进列表快照同步 / 密钥加密不存明文响应恒masked / 业务表不跨schema写 —— 全核验通过
leftover(非阻断): P2(包覆盖 file 未走 el-upload)；迁移 000012 与 #18 编号协调；连库 harness 前向声明(S3/S6/S10 待实跑)；审计 enable/disable 未拆
next: 集成阶段合并共享 surface（admin_wiring.go）+ 迁移重编号 + 连库 harness 实跑 requiresDB 矩阵

stage: fullstack-fix / ✅（待 🟪 复测渠道包覆盖 round-trip）
module: feature-plugin (#15)
worktree: /Users/csw/gitproject/console-feature-plugin (codex/feature-plugin)，未 commit/未切分支
I-1 修复: 后端 PackagePluginItemView.Config → ConfigJSON（json:"configJson"），赋值点同步；与实例项 + 前端 configJson 口径统一；请求侧 config 入参不变（符合 compact POST）；新增 app/dto/plugin_test.go 序列化回归
P3 修复: ResolvePluginConfigStatus 的 allowed 集合并入 secret/file 键（纯规则无 IO、签名不变）；新增 domain/plugin 用例 SecretFileOnlyFieldsAllowed
P2 处置: 维持非阻断遗留（修 I-1 未触及 ChannelPackageDetailDrawer，按约束不扩大改动）
changed: app/dto/plugin.go, app/plugin/{query_list_channel_plugins,command_override_package_plugin}.go, domain/plugin/plugin_config.go (+2 test files)
verify: go build/vet/test ./... ✅ 全包0失败；vitest 本模块 33/33 ✅；vite build ✅；vue-tsc 全量仍被既有 cashier/sync-section 阻塞（非#15）
next: 回 🟪 复测渠道包覆盖 config round-trip → 功能验收

stage: integration-retest / ✅（第 2 轮·可进入功能验收）
module: feature-plugin (#15)
worktree: /Users/csw/gitproject/console-feature-plugin (codex/feature-plugin)，未 commit/未改业务代码
I-1 闭合: 后端包覆盖响应键 configJson（dto/plugin.go:76，query/command 两处同步）↔ 前端 channels.ts:275/drawer:297 一致；实例项未误伤；POST 入参仍 config；plugin_test.go 序列化回归 PASS
P3 闭合: ResolvePluginConfigStatus allowed 并入 secret/file 键；SecretFileOnlyFieldsAllowed 用例 PASS，无回归
regression: go build ✅ / go test ./... ✅ 全包 0 失败；前端模块 vitest 33/33 ✅；Playwright 未重跑（前端未改、mock e2e 等价第1轮 3/3）
contract: 现无契约漂移；红线复核仍成立（scope/三标 false/masked/权限/跨env/InTx/required；S3/S6/S10 运行待连库）
leftover: P2（包覆盖 file 未走 el-upload）非阻断遗留；迁移 000012 与 #18 编号协调；连库 harness 前向声明
verdict: 是（可进入 ✅ 功能验收）

stage: integration-test / ❌（第 1 轮·需修 I-1 后复测，已由第 2 轮取代）
module: feature-plugin (#15)
worktree: /Users/csw/gitproject/console-feature-plugin (codex/feature-plugin)，未 commit/未改业务代码
contract: 5 接口方法/路径/权限/请求DTO/错误码/包络/实例项响应全一致；发现 1 处漂移 I-1
drift I-1（建议阻断）: 渠道包覆盖项 config（后端 json:"config"）vs configJson（前端读取），自定义覆盖 round-trip 显示失效，保存可成功 → 移交 🟧（建议后端 PackagePluginItemView.Config 改 json:"configJson"）
regression: 后端 go test ./... ✅ 0 失败；前端模块 vitest 33/33 ✅、全量 215/216（唯一失败 sync-section-drawer 既有非#15）；Playwright 3/3 ✅
redlines: scope/隐藏·不兼容·非valid 三标全false/masked/权限/跨env/InTx回滚/required引导 均核验通过（S3/S6/S10 运行待连库）
limit: 连库 harness 为前向声明（SCENARIO_WITH_DB 仍无 DSN），requiresDB SKIP；以契约对账+L1+S2+前端用例替代
P3 处置: 建议修复（与 I-1 同回 🟧 顺带）；P2 处置: 接受为非阻断遗留（修 I-1 时可顺带 el-upload）
verdict: 否（暂不进入功能验收）→ 🟧 修 I-1(+P3) 后回 🟪 复测渠道包覆盖

stage: backend-test / ✅
module: feature-plugin (#15)
tests: domain/plugin L1 单测（compatibility+plugin_config，15 Test/32 子用例 PASS）；scenarios/feature-plugin.yaml（5 接口×S1–S10）；fixtures common+sandbox
run: go build/vet/test ./... ✅ 全包通过 0 失败；manifest 解析+S2 401 PASS，requiresDB 待连库 harness
redlines: S8 脱敏 / S10 回滚 / S6 跨env / 必接缺口 / region 不兼容 / scope=server 派生 均有用例
defect: P3 非阻断 — ResolvePluginConfigStatus 的 allowed 未纳入 secret/file 键（现网模板同列 form_schema 不触发）
next: 总负责 Agent 集成收口；连库 harness 跑 requiresDB 用例

stage: frontend-test / ✅
module: feature-plugin (#15)
worktree: /Users/csw/gitproject/console-feature-plugin (codex/feature-plugin)
vitest: 本模块 33/33 ✅（原 16→新增 17，未破坏既有）；全量 215/216（唯一失败 sync-section-drawer.spec.ts 为既有阻塞，非 #15）
playwright: tests/frontend/e2e/feature-plugin.spec.ts 3/3 ✅ + 截图基线 feature-plugin-list.png
covered: 必接/region/selectable/勾选态/config_status/includedInRuntimeConfig 徽标；selectable=false 强制选中；locked 禁用；必接引导；四件套渲染+scope=server 提示；secret 留空=不修改+重填；file 上传；渠道包 inherit/自定义覆盖；无 plugin.write 置灰；空/错态
defect: 无新增（沿用 CR 既登记 P2：渠道包 file 未走 el-upload）
next: 总负责 Agent 集成收口（→ 🟪测试专家 跨栈真实联调 e2e）

stage: backend-cr / ✅
module: feature-plugin (#15)
worktree: /Users/csw/gitproject/console-feature-plugin (codex/feature-plugin)
backend-cr: 契约核对通过；CR 就地修复 7 项（config_status 必填校验、不兼容过滤、渠道允许集合、locked、GET 扩展字段、inherit 运行态、encrypt 去重）
verification: go build ./... ✅, go vet ./... ✅（services/admin-api）
open: domain 单测缺失、列表 N+1 模板、审计未拆 enable/disable、迁移 000012 与 #18 协调（非阻断）
next: 总负责 Agent 集成收口

stage: backend-dev / ✅
module: feature-plugin (#15)
backend: 已实现 plugin domain/app/infra/transport + wiring + 000012 迁移（up/down）
apis: GET/POST game-channel plugins, PATCH game-channel-plugin, GET/POST package plugins（plugin.read/plugin.write）
verification: go build ./... ✅, go vet ./... ✅
deviation: 无（CR 前）
next: 已由 backend-cr 收口

stage: frontend-dev / ⚠️
frontend: channels 内 feature-plugin Tab + 渠道包 inherit/custom 覆盖区已完成
tests: vitest 16/16 ✅
build: vite build ✅
tsc: vue-tsc --noEmit ❌（既有 cashier/games 阻塞，非本模块）
next: 集成阶段合并共享 surface + 全量 tsc
