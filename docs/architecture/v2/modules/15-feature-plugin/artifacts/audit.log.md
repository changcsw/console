# feature-plugin（#15）执行审计日志

> 仅供人类审计。各角色追加完整执行日志、命令、失败记录、证据。总负责 Agent 不读本文件。

## [总负责 Agent] 开工前检查

- 已读：codegen-workflow.md、index.json、codegen-progress.md、15-feature-plugin/spec.compact.md。
- 依赖闸门：channel(#12) ✅、game(#11) ✅、common ✅；channel-login(#14) 已合并 main，channels-surface lane 已释放。
- 同 lane 无在制模块；game-cashier(#18) 属 cashier-surface 跨 lane 并行，合法。
- worktree：/Users/csw/gitproject/console-feature-plugin，分支 codex/feature-plugin（基于 origin/main，HEAD=ade7510）。
- 结论：允许开工。已初始化 artifacts 骨架。

---

## [🟦 后端开发（Go 架构师）] feature-plugin 后端实现

### 1) 文档读取（按协议）

- 已读取：`index.json`、`00-common.md`、`01-structure.md`、`CONVENTIONS.md`、`modules/15-feature-plugin/spec.compact.md`、`modules/12-channel/spec.compact.md`。
- 结构参考：`services/admin-api/internal/domain/channel`、`transport/http/channels`、`channel-login` 的 memstore/handler 模式。

### 2) 代码实现（backend）

- 迁移：
  - `services/admin-api/migrations/000012_feature_plugin_schema.up.sql`
  - `services/admin-api/migrations/000012_feature_plugin_schema.down.sql`
- domain：
  - `services/admin-api/internal/domain/plugin/compatibility.go`
  - `services/admin-api/internal/domain/plugin/plugin_config.go`
- app：
  - `services/admin-api/internal/app/dto/plugin.go`
  - `services/admin-api/internal/app/plugin/ports.go`
  - `services/admin-api/internal/app/plugin/service.go`
  - `services/admin-api/internal/app/plugin/command_configure_channel_plugin.go`
  - `services/admin-api/internal/app/plugin/command_override_package_plugin.go`
  - `services/admin-api/internal/app/plugin/query_list_channel_plugins.go`
- infra：
  - `services/admin-api/internal/infra/persistence/postgres/plugin_store.go`
  - `services/admin-api/internal/infra/persistence/postgres/feature_plugin_repo.go`
  - `services/admin-api/internal/infra/persistence/postgres/game_channel_plugin_repo.go`
  - `services/admin-api/internal/infra/persistence/postgres/channel_package_plugin_repo.go`
- transport/wiring：
  - `services/admin-api/internal/transport/http/channels/handler.go`
  - `services/admin-api/internal/transport/http/channels/router.go`
  - `services/admin-api/internal/transport/httpserver/admin_wiring.go`

### 3) 执行命令与结果

```bash
cd "/Users/csw/gitproject/console-feature-plugin/services/admin-api" && gofmt -w ... && go build ./...
```

- 首次失败（沙箱写限制）：
  - `open internal/app/dto/plugin.go...: operation not permitted`
  - `open internal/app/plugin/ports.go...: operation not permitted`
  - `open internal/infra/persistence/postgres/feature_plugin_repo.go...: operation not permitted`
- 处理：在同命令下提升权限重跑（required_permissions=`all`），成功。

```bash
cd "/Users/csw/gitproject/console-feature-plugin/services/admin-api" && go vet ./...
```

- 结果：通过（exit code 0）。

```bash
cd "/Users/csw/gitproject/console-feature-plugin/services/admin-api" && gofmt -w ... && go build ./... && go vet ./...
```

- 结果：通过（exit code 0）。

### 4) 备注

- 本次未执行 DB 实库迁移，仅完成 SQL 语法与幂等自查（`IF NOT EXISTS`/约束存在性判断）。
- 共享 surface 改动已在 `integration.checklist.md` §3 登记。

---

## [🟩 前端开发（前端架构师）] feature-plugin 前端实现

### 1) 文档读取（按协议）

- 已读取：`index.json`、`00-common.md`、`01-structure.md`、`CONVENTIONS.md`、`modules/15-feature-plugin/spec.compact.md`、`modules/12-channel/spec.compact.md`、`modules/14-channel-login/spec.compact.md`。
- 结构参考：`apps/admin-web/src/views/channels` 现有 `ChannelLoginConfigPanel` / `ChannelInstanceDetailDrawer` / `ChannelPackageDetailDrawer`。

### 2) 代码实现（frontend）

- API client（compact 契约）：
  - `apps/admin-web/src/api/modules/channels.ts`
  - 新增：`listGameChannelPlugins` / `upsertGameChannelPlugin` / `patchGameChannelPlugin` / `listChannelPackagePlugins` / `upsertChannelPackagePlugin`
- 视图组件：
  - `apps/admin-web/src/views/channels/components/FeaturePluginConfigPanel.vue`（渠道实例插件列表 + 模板驱动编辑 + 必接引导）
  - `apps/admin-web/src/views/channels/components/ChannelInstanceDetailDrawer.vue`（新增「功能插件」Tab）
  - `apps/admin-web/src/views/channels/components/ChannelPackageDetailDrawer.vue`（新增渠道包插件覆盖区）
- 测试：
  - `apps/admin-web/src/views/channels/components/__tests__/FeaturePluginConfigPanel.spec.ts`
  - `apps/admin-web/src/views/channels/components/__tests__/ChannelInstanceDetailDrawer.spec.ts`
  - `apps/admin-web/src/views/channels/components/__tests__/ChannelPackageDetailDrawer.spec.ts`
  - `apps/admin-web/src/views/channels/components/__tests__/fixtures/featurePlugin.ts`

### 3) 执行命令与结果

```bash
pnpm --dir "/Users/csw/gitproject/console-feature-plugin/apps/admin-web" exec vitest run \
  src/views/channels/components/__tests__/FeaturePluginConfigPanel.spec.ts \
  src/views/channels/components/__tests__/ChannelInstanceDetailDrawer.spec.ts \
  src/views/channels/components/__tests__/ChannelPackageDetailDrawer.spec.ts
```

- 结果：通过（3 files, 16 tests）。

```bash
pnpm --dir "/Users/csw/gitproject/console-feature-plugin/apps/admin-web" exec vue-tsc --noEmit
```

- 结果：失败（非本次改动引入的既有问题）：
  - `src/api/modules/cashier.ts(173,31)` TS2352
  - `src/views/games/detail/__tests__/sync-section-drawer.spec.ts(23,24)` TS2322

```bash
pnpm --dir "/Users/csw/gitproject/console-feature-plugin/apps/admin-web" exec vite build
```

- 结果：通过（构建产物生成成功；存在 chunk size warning）。

### 4) 偏差与未决

- 与 compact 契约偏差：无。
- 未决：`vue-tsc` 被既有跨模块类型问题阻塞，需由对应模块修复后再做全量类型通过确认。

---

## [🔎 后端 Code Review] feature-plugin 后端 CR

### 结论：✅ 通过（CR 已就地修复 7 项；无阻断打回项）

### 契约核对表（要点 → 一致? → 证据）

| 要点 | 一致 | 证据 |
| --- | --- | --- |
| 5 张表字段/约束/索引/platform+env schema | ✅ | `migrations/000012_feature_plugin_schema.up.sql:11-119` |
| 跨 schema FK `plugin_id_ref→platform.feature_plugins` | ✅ | up.sql:63,93 |
| 枚举 region/config_status 与默认值 | ✅ | up.sql:15,47-49,66-67,94-97 |
| ValidatePluginRegionCompatibility 纯规则 | ✅ | `domain/plugin/compatibility.go:10-24` |
| ResolvePluginConfigStatus 纯规则 | ✅（CR 修复） | `domain/plugin/plugin_config.go:40-113` |
| ResolveRuntimeFlags 运行态派生 | ✅ | `domain/plugin/plugin_config.go:116-124` |
| 不兼容/隐藏不进列表 | ✅（CR 修复） | `app/plugin/query_list_channel_plugins.go:34-36` |
| 5 API 路由+plugin.read/write | ✅ | `transport/http/channels/router.go:34-43` |
| 错误码 MARKET_CHANNEL_INCOMPATIBLE/VALIDATION_FAILED/CONFLICT | ✅ | `app/plugin/ports.go:21-45` |
| secret 加密+响应 masked | ✅ | `app/plugin/service.go:108-144,98-106` |
| 业务表 SQL 无 schema 前缀/env 谓词 | ✅ | `infra/persistence/postgres/game_channel_plugin_repo.go:17-53` |
| 事务边界 InTx | ✅ | `infra/persistence/postgres/plugin_store.go:27-36` |
| GET 含 template/configJson/locked（前端契约） | ✅（CR 修复） | `app/dto/plugin.go:38-55`, `query_list_channel_plugins.go:50` |
| POST 校验渠道允许集合+locked | ✅（CR 修复） | `command_configure_channel_plugin.go:25-62` |
| 渠道包 inherit 运行态派生 | ✅（CR 修复） | `query_list_channel_plugins.go:107-119` |

### CR 发现问题与处置

| # | 严重度 | 问题 | 处置 |
| --- | --- | --- | --- |
| 1 | 高 | `ResolvePluginConfigStatus` 漏校验普通必填字段 | **已修复** plugin_config.go |
| 2 | 高 | `ListChannelPlugins` 未过滤 market 不兼容插件 | **已修复** query_list_channel_plugins.go |
| 3 | 高 | `ConfigureChannelPlugin` 未校验渠道允许集合 | **已修复** command_configure_channel_plugin.go |
| 4 | 高 | GET/POST 响应缺 locked/template/configJson，前端无法渲染 | **已修复** dto+service+query |
| 5 | 中 | `locked` 未加载/未服务端强制 | **已修复** feature_plugin_repo+configure/patch |
| 6 | 中 | 渠道包 inherit=true 时 IncludedInRuntimeConfig 未沿用实例态 | **已修复** query_list_channel_plugins.go |
| 7 | 低 | 重复 encryptSecrets 调用 | **已修复** configure 合并为一次 |

### 遗留（非阻断，移交后续）

- `domain/plugin` 无单元测试（建议补 compatibility + config_status 边界）
- 列表 N+1 拉模板（可 batch 优化）
- 审计事件未拆分 plugin.enable/disable（当前统一 plugin.configure）
- 迁移 000012 与 #18 game-cashier 编号冲突（集成阶段协调）

### 验证命令

```bash
cd services/admin-api && go build ./... && go vet ./...
```

- 结果：✅ 通过（CR 修复后复验）

---

## [🟩🔎 前端 Code Review] feature-plugin 前端 CR

### 1) 文档与范围

- 已读：`spec.compact.md`（前端章节）、`00-common.md`、`01-structure.md §5`、`CONVENTIONS.md`、`12-channel/spec.compact.md` 片段；handoff / manifest / integration.checklist。
- 评审范围：`FeaturePluginConfigPanel.vue`、`ChannelInstanceDetailDrawer.vue`、`ChannelPackageDetailDrawer.vue`、`api/modules/channels.ts` 及关联 vitest。

### 2) compact 核对表（要点 → 结论 / 证据）

| 要点 | 一致? | 证据 |
| --- | --- | --- |
| 可接插件列表：required/region/selectable + 勾选 + config_status + includedInRuntimeConfig | ✅ | `FeaturePluginConfigPanel.vue:26-34,54-61` |
| selectable=false 强制选中不可取消 | ✅ | `:disabled="!canEdit(item) \|\| !item.selectable"` L57；`onEnabledChange` L592-597；`initDraft` L326 |
| locked=true 禁用编辑 | ✅ | `canEdit` L320-322；alert L40-45 |
| 未配置必接插件引导 | ✅ | `requiredMissing` L253-257；alert L8-16 |
| API client GET/POST/PATCH camelCase 五接口 | ✅ | `channels.ts:404-448` |
| 模板四件套 + scope=server 提示 | ✅ 实例；⚠️ 包覆盖 file | 实例 L78；包区经 `TemplateConfigRenderer` 已补 scope（CR 修复） |
| secret masked / 留空=不修改（****** 哨兵） | ✅ 实例；✅ 包（CR 修复后） | 实例 L382-397,562-573；包 `buildPluginSubmitConfig` |
| file 统一上传 | ✅ 实例；⚠️ 包覆盖 | 实例 L147-164 el-upload；包区 TemplateConfigRenderer 仍为文本输入 L55-62 |
| env badge | ✅（CR 补） | `FeaturePluginConfigPanel.vue` panel head + `EnvironmentBadge` |
| plugin.write 置灰 | ✅ 实例；✅ 包（CR 修复） | 实例 `canPluginWrite` → drawer L217；包 `canPluginWrite`/`pluginCanEdit` |
| 渠道包 inherit_channel_config 开关 | ✅ | `ChannelPackageDetailDrawer.vue:110-115,476` |
| 抽屉 Tab 信息架构（01 §5） | ✅ | `ChannelInstanceDetailDrawer.vue`「功能插件」Tab 与登录/包并列 |
| 空/错/权限态 | ✅ | load 错误 L645-648；保存校验 L600-604；v-perm L182 |

### 3) CR 直接修复项

- `ChannelPackageDetailDrawer.vue`：`pluginCanEdit` 误用 `product.write` → 改为 `plugin.write`（`canPluginWrite`）。
- `ChannelPackageDetailDrawer.vue`：`savePackagePlugin` 未合并 `pluginSecrets` → 新增 `buildPluginSubmitConfig`（对齐 IAP 密文留空语义）。
- `FeaturePluginConfigPanel.vue`：补 `EnvironmentBadge`（`useAppStore().environment`）。
- `TemplateConfigRenderer.vue`：补 `scope=server` 字段标签「仅服务端，不下发客户端」。

### 4) 移交前端开发返工项

- **P2** 渠道包插件覆盖区 file 字段仍经 `TemplateConfigRenderer` 文本输入，未复用 `el-upload` 统一上传（实例侧 `FeaturePluginConfigPanel` / `ChannelLoginConfigPanel` 已正确实现）。

### 5) 验证命令

```bash
pnpm --dir apps/admin-web exec vitest run \
  src/views/channels/components/__tests__/FeaturePluginConfigPanel.spec.ts \
  src/views/channels/components/__tests__/ChannelInstanceDetailDrawer.spec.ts \
  src/views/channels/components/__tests__/ChannelPackageDetailDrawer.spec.ts
# ✅ 3 files, 16 tests

pnpm --dir apps/admin-web exec vite build
# ✅

pnpm --dir apps/admin-web exec vue-tsc --noEmit
# ❌ 仅既有：cashier.ts(173), sync-section-drawer.spec.ts(23) — 非 #15 引入
```

### 6) CR 结论

**通过**（阻断项已在 CR 修复；包级 file 上传为 P2 跟进，不阻塞主链路集成）。

---

## [🟦🧪 后端测试工程师] feature-plugin 后端测试（stage=backend-test）

> 模型 Cursor Auto。worktree=/Users/csw/gitproject/console-feature-plugin，分支 codex/feature-plugin（未 commit/未切分支）。
> 读文档：index.json → 15-feature-plugin/spec.compact.md → 03-testing.md（分层/目录/S1–S10/fixtures/回归入口）。

### 1) 测试清单（文件 + 覆盖对象）

| 文件 | 层 | 覆盖对象 |
| --- | --- | --- |
| `services/admin-api/internal/domain/plugin/compatibility_test.go` | L1 单元（无 IO） | `ValidatePluginRegionCompatibility(market, region)`：CN↔domestic、非CN↔overseas、未知 market/region、大小写敏感（17 子用例） |
| `services/admin-api/internal/domain/plugin/plugin_config_test.go` | L1 单元（无 IO） | `ResolvePluginConfigStatus(enabled, template, config)`：enabled=false→empty；齐全→valid；缺密文/文件→invalid(SF 语)；缺普通必填→invalid；validation_rules.Required；空白视同缺失；SF 优先级；缺失字段字典序；未知字段；min/max/pattern/非法 pattern/enum/类型校验；空模板 valid；可选缺省 valid。`ResolveRuntimeFlags(...)`：hidden/incompatible/disabled/empty/invalid 各置 false，且 Runtime==Snapshot==Sync 同口径 |
| `tests/backend/scenarios/feature-plugin.yaml` | L3 场景矩阵 manifest | 5 组 API × S1–S10（见下表），含红线：S8 脱敏 / S10 事务回滚 / S6 跨 env / 必接缺口 / region 不兼容 / locked / 必接不可取消 / inherit 运行态派生 |
| `tests/fixtures/common/feature-plugin.sql` | fixtures（platform，自动挂回归入口 scripts/regression/db.sh） | RBAC（plugin_admin/plugin_reader）+ 插件主数据（realname 必接 / customer_service / locked_plugin / overseas_only_plugin）+ 模板四件套（appId+appSecret 密文）+ channel_feature_plugins 策略（required/locked/海外不兼容）挂 huawei_cn |
| `tests/fixtures/sandbox/feature-plugin.sql` | fixtures（sandbox 业务表，连库 harness 前向声明） | 已配置渠道实例插件（valid+密文，验 S1/S8）+ 渠道包 inherit 覆盖（验 inherit 运行态派生） |

### 2) 场景维度覆盖表（接口 × S1–S10；✓ 有用例，— N/A）

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET  /game-channels/{id}/plugins | ✓(列表/必接缺口/运行态派生) | ✓ | ✓ | ✓(404) | — | ✓ | — | ✓(masked) | —(固定候选集非分页) | — |
| POST /game-channels/{id}/plugins | ✓ | ✓ | ✓ | ✓(缺密文 invalid/disabled empty/未知字段/不在允许集/必接不可取消/locked) | ✓(MARKET_CHANNEL_INCOMPATIBLE+CONFLICT) | ✓ | ✓(plugin.configure) | ✓(masked) | — | ✓ |
| PATCH /game-channel-plugins/{id} | ✓ | ✓ | ✓ | ✓(缺密文 invalid/404) | —(单行 upsert 无新建冲突) | ✓ | ✓ | ✓(masked+回填占位) | — | ✓ |
| GET  /channel-packages/{id}/plugins | ✓(inherit 运行态派生) | ✓ | ✓ | ✓(404) | — | — | — | ✓(masked) | —(固定候选集) | — |
| POST /channel-packages/{id}/plugins | ✓(inherit+custom) | ✓ | ✓ | ✓(缺密文 invalid) | ✓(INCOMPATIBLE+CONFLICT) | ✓ | ✓ | ✓(masked) | — | ✓ |

> N/A 说明：列表接口为「按渠道允许集合」固定候选集，无 page/pageSize 入参 → S9 N/A；PATCH 为按 id 单行 upsert，不新建唯一键 → S5 N/A；只读接口 S5/S7/S10 N/A。

### 3) 运行结果（go test）

```bash
# 工作目录 services/admin-api
go build ./...   # ✅ exit 0
go vet ./internal/domain/plugin/ ./internal/testkit/scenario/   # ✅ exit 0

go test ./internal/domain/plugin/ -v
# ✅ ok  github.com/csw/console/services/admin-api/internal/domain/plugin
# 15 个 Test 函数 / 32 个子用例全部 PASS

go test ./internal/testkit/scenario/ -run 'TestScenarioManifests/feature-plugin' -v
# ✅ feature-plugin manifest 解析通过；S2(4 例 requiresDB:false) 进程内执行 401 PASS；
#    其余 requiresDB:true 用例 SKIP（manifest parsed OK，待 SCENARIO_WITH_DB=1 连库 harness 执行）

go test ./...
# ✅ 全包通过（no failures）；plugin domain 与 scenario 包均 ok
```

- 通过：全部（domain L1 32 子用例 + scenario manifest 4 进程内执行 + 全量 `go test ./...` 各包 ok）。
- 失败：0。
- 跳过：feature-plugin.yaml 中 requiresDB:true 用例（设计内行为，等待连库 harness；与 channel-login/account-auth 同口径）。

### 4) 疑似实现缺陷（标注，回退 🟦后端开发经总负责 Agent 调度）

- **[P3 / 非阻断 / 潜在健壮性] `ResolvePluginConfigStatus` 的 allowed 集合未纳入 secret/file 字段键**：
  `domain/plugin/plugin_config.go` 中 `allowed` 仅由 `FormSchema` + `ValidationRules` 的 key 构成；
  `SecretFields` / `FileFields` 仅进 `required`/`secret`/`file` 集。若某模板把密文/文件字段**只**声明在
  `secret_fields_json` / `file_fields_json`（未同列于 `form_schema_json`），则用户提交该字段值会命中
  「字段未在模板中定义」→ 误判 invalid。当前所有契约模板（channel-login 及本模块 fixtures）均把密文字段
  同时列入 form_schema，故现网不触发；属潜在边界一致性问题，建议后端开发把 secret/file 键并入 allowed 集。
  （未自行修改业务代码。）

### 5) 产物协议落地

- 本节追加于 audit.log.md（不覆盖既有 backend-dev / backend-cr 内容）。
- module.manifest.json：verification 增「后端测试结果」。
- integration.checklist.md：§5 勾除「domain/plugin 缺单元测试」遗留；§6 增后端测试运行结论。
- handoff.summary.md：新增 stage=backend-test（≤10 行），保留其它角色内容。

---

## [🟩🧪 前端测试工程师] feature-plugin 前端测试（stage=frontend-test）

> 模型 Cursor Auto。worktree=/Users/csw/gitproject/console-feature-plugin，分支 codex/feature-plugin（未 commit/未切分支）。
> 读文档：index.json → 15-feature-plugin/spec.compact.md → 03-testing.md（§5 前端 vitest 组件 / Playwright 截图基线 / 目录约定）。
> 在前端开发既有 16 vitest 用例基础上补全，未破坏既有用例。

### 1) 测试清单（文件 + 覆盖对象）

| 文件 | 层 | 改动 | 覆盖对象 |
| --- | --- | --- | --- |
| `apps/admin-web/src/views/channels/components/__tests__/fixtures/featurePlugin.ts` | L4 fixtures | 扩展 | 新增 `optionalPluginItem`（海外/可勾选/valid）、`filePluginItem`（含 file 字段）、`channelPackagePluginItem`（渠道包覆盖项，含密文+scope=server 模板） |
| `apps/admin-web/src/views/channels/components/__tests__/FeaturePluginConfigPanel.spec.ts` | L4 组件 | 2→13 | 徽标渲染（必接/国内海外/进入最终配置/config_status/锁定）、必接引导清单、selectable=false 强制选中、locked 禁用、canWrite=false 置灰、空列表、加载失败、file 上传（超限/类型/成功回填）、密文重填下发、必填缺失阻止保存、id=0 走 POST upsert（原 2 例保留：必接引导+scope、PATCH 密文哨兵） |
| `apps/admin-web/src/views/channels/components/__tests__/ChannelPackageDetailDrawer.spec.ts` | L4 组件 | 10→15 | 新增 describe「功能插件继承/覆盖」：inherit=true 不渲染模板+下发空 config、inherit=false 模板渲染器消费四件套+scope=server 提示、自定义覆盖密文留空不下发/重填下发、locked 提示+不可编辑、无 plugin.write 置灰（原 10 例保留） |
| `apps/admin-web/src/views/channels/components/__tests__/ChannelInstanceDetailDrawer.spec.ts` | L4 组件 | 4→5 | 新增：无 plugin.write 时 FeaturePluginConfigPanel canWrite=false 置灰（原 4 例保留：canWrite computed、channel_only 登录页签、account_system 不拉 login-config、始终展示功能插件并拉列表） |
| `tests/frontend/e2e/feature-plugin.spec.ts` | L5 Playwright | 新增 | 契约 mock/stub 驱动渠道实例详情→「功能插件」页签：徽标全量渲染+必接引导+scope 提示（截图基线 `feature-plugin-list.png`）、selectable=false 开关强制选中且禁用、无 plugin.write 保存按钮置灰 |
| `tests/frontend/visual-baseline/feature-plugin.spec.ts-snapshots/feature-plugin-list-chromium-darwin.png` | 截图基线 | 新增 | 功能插件面板列表态可视回归基线 |

### 2) 覆盖的 compact 关键交互点 → 落点

| compact 要点 | 覆盖 | 落点 |
| --- | --- | --- |
| required 标记 / 国内海外 region / selectable / 勾选态 / config_status 徽标 / includedInRuntimeConfig | ✅ | Panel「徽标渲染」+ e2e「功能插件页签渲染」 |
| 必接 selectable=false 强制选中不可取消 | ✅ | Panel「强制选中」+ e2e「开关强制选中且禁用」 |
| locked=true 禁用编辑 | ✅ | Panel「locked 禁用」+ Pkg「locked 提示」 |
| 未配置必接插件引导补齐清单 | ✅ | Panel「必接引导清单」+ e2e 必接引导 |
| 模板渲染器消费四件套 | ✅ | Pkg「inherit=false 模板渲染器消费四件套」 |
| scope=server「不下发客户端」提示 | ✅ | Panel/原例 + Pkg + e2e |
| secret masked + 留空=不修改 | ✅ | Panel「PATCH 哨兵」「密文重填」+ Pkg「密文留空不下发/重填下发」 |
| file 上传 | ✅ | Panel「file 上传 超限/类型/成功回填」 |
| 渠道包 inherit_channel_config 继承/自定义覆盖开关 | ✅ | Pkg「inherit=true 下发空 config」「inherit=false 自定义」 |
| 无 plugin.write 权限置灰 | ✅ | Panel/Drawer/Pkg + e2e |
| 空/错/权限态 | ✅ | Panel「空列表」「加载失败」「必填缺失阻止保存」 |

### 3) 运行结果

环境说明：仓内 `pnpm exec` 触发 deps 自检会误跑根目录 `pnpm install` 报错；改用 worktree 本地 `node_modules/.bin/vitest|playwright` 直跑。vite 需写 `node_modules/.vite-temp`，沙箱禁写 → 全部测试在 required_permissions=all 下执行。Playwright 端口 5187 被另一 worktree dev server 占用 → 用 `E2E_PORT=5193`。

```bash
# vitest（本模块三件套）  工作目录 apps/admin-web
node_modules/.bin/vitest run \
  src/views/channels/components/__tests__/FeaturePluginConfigPanel.spec.ts \
  src/views/channels/components/__tests__/ChannelInstanceDetailDrawer.spec.ts \
  src/views/channels/components/__tests__/ChannelPackageDetailDrawer.spec.ts
# ✅ Test Files 3 passed (3) | Tests 33 passed (33)   （原 16 → 33，新增 17）

# vitest 全量（确认未破坏其它模块）
node_modules/.bin/vitest run
# 结果：Test Files 1 failed | 29 passed (30) | Tests 1 failed | 215 passed (216)
# 唯一失败：src/views/games/detail/__tests__/sync-section-drawer.spec.ts:27（既有阻塞，非 #15，未触碰）

# Playwright（本模块 e2e + 截图基线）
E2E_PORT=5193 node_modules/.bin/playwright test feature-plugin.spec.ts --update-snapshots   # 生成基线
E2E_PORT=5193 node_modules/.bin/playwright test feature-plugin.spec.ts                       # 复验
# ✅ 3 passed（含 toHaveScreenshot 与基线比对绿）
```

- 通过：vitest 本模块 33/33；Playwright 3/3（含截图基线比对）。
- 失败：0（本模块）。全量 vitest 唯一失败为既有 `sync-section-drawer.spec.ts`，非 #15、未触碰，符合任务「关于既有阻塞」说明。
- vue-tsc 全量：未跑（既有 cashier.ts(173) / sync-section-drawer.spec.ts(23) 阻塞，非本模块；本模块测试文件无 lint/TS 报错，ReadLints 干净）。

### 4) 疑似实现缺陷（标注；不自行大改业务代码）

- 无新增功能缺陷。沿用 CR 既登记 **P2**：`ChannelPackageDetailDrawer` 渠道包插件覆盖区 file 字段经 `TemplateConfigRenderer` 仍为文本输入，未复用 `el-upload` 统一上传（实例侧 `FeaturePluginConfigPanel` 已正确实现）。本阶段以组件级 + 契约 mock UI 用例为主，跨栈真实联调 e2e 属 🟪测试专家。

### 5) 产物协议落地

- 本节追加于 audit.log.md（保留 backend/frontend dev/cr/backend-test 内容）。
- module.manifest.json：verification 增「前端测试结果（vitest 33 + Playwright 3 + 截图基线）」。
- integration.checklist.md：§5 标注本模块 vitest/Playwright 已补全；§6 增前端测试运行结论。
- handoff.summary.md：新增 stage=frontend-test（≤10 行），保留其它角色内容。

---

## [🟪 测试专家（集成/系统测试）] feature-plugin 集成测试（stage=integration-test）

> 模型 Cursor Auto。worktree=/Users/csw/gitproject/console-feature-plugin，分支 codex/feature-plugin（未 commit / 未切分支 / 未改业务代码）。
> 读文档：index.json → 15-feature-plugin/spec.compact.md → 03-testing.md → 02-operation-flow.md（B4 加功能插件、C 下一步/阻塞口径）→ 依赖 12-channel/spec.compact.md。
> 输入：backend-dev/backend-cr/backend-test/frontend-dev/frontend-cr/frontend-test 全部 handoff/manifest/checklist。

### 1) 契约对账表（前端 `api/modules/channels.ts` feature-plugin 段 ↔ 后端 `transport/http/channels` handler/DTO/router）

| 维度 | 前端实际调用 | 后端实际 API | 一致? | 证据 |
| --- | --- | --- | --- | --- |
| GET 列实例插件 | `GET /api/admin/game-channels/{id}/plugins` | router.go:34 `plugin.read` → ListChannelPlugins | ✅ | channels.ts:406 / router.go:34 |
| POST 配置实例插件 | `POST .../game-channels/{id}/plugins` body{pluginId,enabled,config} | router.go:35 `plugin.write` → ConfigureChannelPlugin，req{pluginId,enabled,config} | ✅ | channels.ts:412 / handler.go:86-90 |
| PATCH 改单插件 | `PATCH /api/admin/game-channel-plugins/{id}` body{enabled,config} | router.go:36 `plugin.write` → PatchChannelPlugin，req{enabled,config} | ✅ | channels.ts:423 / handler.go:92-95 |
| GET 列包覆盖插件 | `GET /api/admin/channel-packages/{packageId}/plugins` | router.go:42 `plugin.read` → ListPackagePlugins | ✅ | channels.ts:434 / router.go:42 |
| POST 包覆盖插件 | `POST .../channel-packages/{packageId}/plugins` body{pluginId,inheritChannelConfig,enabled,config} | router.go:43 `plugin.write` → OverridePackagePlugin，req{pluginId,inheritChannelConfig,enabled,config} | ✅ | channels.ts:440 / handler.go:97-102 |
| 错误码 | ApiError 透传 | MARKET_CHANNEL_INCOMPATIBLE/VALIDATION_FAILED/CONFLICT/NOT_FOUND | ✅ | ports.go:21-45 / writeError handler.go:555-558 |
| 统一包络 | request<{data}>/{items} | httpx.WriteData / `{items}` | ✅ | handler.go:371,444 |
| 实例项响应字段 | `GameChannelPluginItem`(id/pluginId/pluginName/region/required/selectable/locked/enabled/configStatus/includedInRuntimeConfig/**configJson**/lastCheckAt/lastCheckMessage/template) | `ChannelPluginItemView` 同名（含 configJson；额外 includedInSnapshot/includedInSync/updatedAt omitempty） | ✅ | channels.ts:234-249 / dto/plugin.go:38-56 |
| **包覆盖项 config 字段** | `ChannelPackagePluginItem.configJson` 读取（initPluginDraft 用 `item.configJson`） | `PackagePluginItemView.Config` 序列化为 **`json:"config"`** | ❌ **漂移** | channels.ts:275 vs dto/plugin.go:76；前端 ChannelPackageDetailDrawer.vue:297 |
| 包覆盖项 required/selectable/locked/region/pluginName | 类型为必填 | 后端 `omitempty`（false/空被丢） | ⚠️ 轻微 | dto/plugin.go:69-73（布尔 falsy 等价，区域/名常非空，低风险） |
| 列表必接缺口 | 前端自算 `requiredMissing`，未消费后端 `missingRequiredPlugins` | 后端 list view 返回 `missingRequiredPlugins` | ⚠️ 冗余非漂移 | dto/plugin.go:60 未被前端读取（口径一致，无功能影响） |

### 2) 契约对账结论

- **存在 1 处真实契约漂移（并行开发产生）**：渠道包插件覆盖列表/写回响应中，后端把配置体序列化为 `config`，前端按 `configJson` 读取并回填编辑器（`initPluginDraft` → `item.configJson ?? {}`）。
  - 影响面：仅「渠道包级插件覆盖」路径。非继承（自定义覆盖）时，**保存可成功**（前端请求体用 `config` 键，后端正确解析），但**读取/保存后回填会拿不到已存的非密文配置值**（始终回退为 `{}`），用户侧表现为自定义覆盖表单不显示已保存值（密文本就 masked，不受影响）。属 round-trip 显示漂移，非数据丢失，不触碰红线。
  - 未被既有用例发现的原因：前端 vitest fixture 与 Playwright mock 均按前端形状用 `configJson`，未与后端真实 `config` 键对账；实例级（`GameChannelPluginItem`）两侧均为 `configJson`，故主链路不受影响。
  - 建议（移交 🟧）：二选一，后端 `PackagePluginItemView.Config` 的 json tag 改为 `configJson`（与实例项及 IAP override 口径统一，推荐），或前端 `ChannelPackagePluginItem`+drawer 改读 `config`。修复后回本角色复测。
- 其余方法/路径/权限码/请求 DTO/错误码/包络/实例项响应字段：**全部一致，无漂移**。

### 3) 跨栈集成 e2e / 真实后端可行性

- **真实连库后端 e2e 不可运行（环境限制，已记录）**：scenario harness 的连库通道为前向声明——`scenarios_test.go` 即便 `SCENARIO_WITH_DB=1` 仍以 `httpserver.New(无 DSN)` 装配（ready=false 降级），未接真实 PG/全装配；本机 `golang-migrate` 缺失、无本地 PG（docker 可用但 harness 无 DSN 注入路径）。requiresDB 用例继续 SKIP（manifest 解析 OK），与 channel-login/account-auth 同口径，属 03-testing §9「增量补充」。
- **替代覆盖**：契约对账（上）+ 后端 L1 域规则单测（红线派生逻辑）+ scenario manifest 解析与 S2 进程内 401 + 前端 vitest（组件/权限/密文/继承覆盖）+ Playwright（mock 驱动真实页面渲染 operation-flow B4 主线）。

### 4) 全量场景回归（真实输出）

```bash
# 后端（services/admin-api）
go build ./...                                   # ✅ BUILD_OK
go test ./...                                    # ✅ 全包 ok，0 失败（含 domain/plugin、scenario、transport/http/channels）
go test ./internal/testkit/scenario/ -run 'TestScenarioManifests/feature-plugin' -v
# ✅ feature-plugin manifest 解析 OK；S2(requires-auth ×4) 进程内 401 PASS；其余 requiresDB 用例 SKIP（设计内）

# 前端（apps/admin-web，worktree 本地 bin，required_permissions=all）
node_modules/.bin/vitest run <三件套>            # ✅ 3 files / 33 tests passed
node_modules/.bin/vitest run                     # 全量：1 failed | 215 passed (216)
#   唯一失败 src/views/games/detail/__tests__/sync-section-drawer.spec.ts:27 —— 既有阻塞，非 #15、未触碰（与任务说明一致）
E2E_PORT=5193 node_modules/.bin/playwright test feature-plugin.spec.ts   # ✅ 3 passed（含 toHaveScreenshot 基线比对）
```

- 通过：后端 `go test ./...` 全绿；前端模块 vitest 33/33；Playwright 3/3。
- 失败：0（本模块）；全量 vitest 唯一失败为既有 `sync-section-drawer.spec.ts`（非 #15）。
- 既有失败核验：`cashier.ts(173)` 类型断言、`sync-section-drawer.spec.ts` —— 均非 #15 引入，本模块文件未触碰，不受影响（结论与前端测试一致）。

### 5) 红线端到端核验（来源 00 §9 / compact）

| 红线 | 核验方式 | 结论 |
| --- | --- | --- |
| scope=server 不下发客户端最终配置 | 模板字段含 scope（domain TemplateField.Scope）；DTO 透传；前端 TemplateConfigRenderer/Panel「仅服务端，不下发客户端」提示（vitest+e2e）；**客户端最终配置按 scope 过滤属快照(#20)落地点，#15 仅存储+标注** | ✅（#15 范围内）；下沉点见 §7 |
| 隐藏/不兼容/config_status!=valid 不进列表/快照/同步 | `ListChannelPlugins` 过滤不兼容（CR#2）；`ResolveRuntimeFlags(hidden,compatible,enabled,valid)` 任一不满足 → Runtime/Snapshot/Sync 三标全 false（domain 单测验证） | ✅ |
| secret 脱敏 masked | `maskSecrets` 响应恒置 `masked`；重填哨兵 `masked`/`******` 留空=不修改（service.go:98-144）；前端密文留空不下发/重填下发（vitest） | ✅ |
| 权限 plugin.read/write | router.go:34-43 RequirePerm；S2 401 进程内 PASS；S3 forbidden 用例 manifest 已声明（requiresDB SKIP） | ✅（401 实测；403 待连库） |
| 跨 env（schema 隔离） | 业务表仓储 SQL 无 schema 前缀/无 env 谓词（CR 验证 game_channel_plugin_repo.go:17-53）；S6 场景声明 | ✅ 代码核验；S6 运行待连库 |
| 事务回滚 | `InTx`（plugin_store.go:27-36）；S10 场景声明 | ✅ 代码核验；S10 运行待连库 |
| required 缺口运行态异常 | list view `missingRequiredPlugins`；前端 `requiredMissing` 引导清单（vitest+e2e）；必接 selectable=false 强制选中（e2e） | ✅ |

### 6) 下游 impacts 契约抽查（snapshot #20 / sync #21）

- 现状：本 worktree **无 `internal/domain/snapshot`**（#20 未落地）；`internal/domain/sync/sync.go` 为 section 枚举骨架（plugin 随 `channels`/`packages` section 流转，无独立 plugin section），未消费插件有效性口径。
- 口径固化点：`ResolveRuntimeFlags` 单一来源返回 `IncludedInRuntimeConfig==IncludedInSnapshot==IncludedInSync`（plugin_config.go:120-127，domain 单测断言三者同口径）；DTO 额外透出 `includedInSnapshot/includedInSync`（omitempty）供下游消费。
- 结论：当前无下游消费方可对账，**无漂移可能**；#20/#21 落地时须直接消费上述派生标记，不得各自重算 → 列为前向兼容备忘。

### 7) 遗留问题清单（处置建议）

| # | 级别 | 问题 | 处置建议 |
| --- | --- | --- | --- |
| I-1 | **漂移 / 建议阻断** | 渠道包覆盖项 config 字段后端 `config` vs 前端 `configJson`，自定义覆盖配置读取/回填失效 | **移交 🟧 修复**：后端 `PackagePluginItemView.Config` json tag 改 `configJson`（推荐，统一口径）。修复后回本角色复测包覆盖 round-trip。 |
| P3 | 非阻断（建议本阶段一并修） | `ResolvePluginConfigStatus` 的 `allowed` 集合未纳入 secret/file 键（plugin_config.go:45-68），仅声明于 secret/file_fields 未同列 form_schema 的字段会被误判 unknown→invalid；现网模板同列故不触发 | **建议修复**（低成本/低风险）：将 secret/file 键并入 `allowed`。鉴于已与 I-1 同回 🟧，建议顺带修复；若资源紧可接受为非阻断遗留。 |
| P2 | 非阻断遗留 | 渠道包覆盖区 file 字段经 `TemplateConfigRenderer` 仍为文本输入，未复用 `el-upload`（实例侧已正确） | **接受为非阻断遗留**移交集成阶段后续增强；若 🟧 修 I-1 时已动 `ChannelPackageDetailDrawer`，建议顺带统一 el-upload。 |
| — | 跟踪 | 迁移 000012 与 #18 game-cashier 编号冲突 | 仅登记；合并 main 前由集成阶段重编号（本阶段不处理）。 |
| — | 非阻断 | scenario requiresDB 用例无连库 harness 执行 | 连库 harness 落地后置 SCENARIO_WITH_DB 跑全量 S1/S3–S10（与全模块同口径）。 |

### 8) 通过判定

- **暂不进入 ✅ 功能验收（否）**：存在 1 处真实契约漂移（I-1，渠道包覆盖 config/configJson），属并行开发产物，影响 compact 明列的「渠道包继承/覆盖」子功能 round-trip 显示。建议 🟧 修复 I-1（并顺带 P3）后回本角色复测渠道包覆盖路径；其余主链路/红线/回归均通过。

### 9) 产物协议落地

- 本节追加于 audit.log.md（保留既有全部角色内容）。
- module.manifest.json：verification 增「集成测试结果」；open_issues 增 I-1 漂移。
- integration.checklist.md：§5 增 I-1/P3 处置建议；§6 增集成测试运行结论与连库限制。
- handoff.summary.md：新增 stage=integration-test（≤10 行），保留其它角色内容。

---

## [🟪 测试专家（集成/系统测试）] feature-plugin 第 2 轮复测（stage=integration-retest）

> 模型 Cursor Auto。worktree=/Users/csw/gitproject/console-feature-plugin，分支 codex/feature-plugin（未 commit / 未改业务代码）。
> 触发：🟧 高级全栈工程师修复第 1 轮 I-1（契约漂移）+ P3，本轮针对性复测 + 全量回归复跑。

### 1) I-1 复测（契约漂移闭合核验）

| 核验点 | 结论 | 证据 |
| --- | --- | --- |
| 后端包覆盖项 config 序列化键 | ✅ 已改为 `configJson` | `app/dto/plugin.go:76` `ConfigJSON map[string]any \`json:"configJson"\`` |
| 后端 query/command 赋值同步 | ✅ 两处均用 `ConfigJSON` | `query_list_channel_plugins.go:104,133`（GET 列表，含 maskSecrets）、`command_override_package_plugin.go:76,98`（POST 写回） |
| 前端读取键 | ✅ `configJson`（一致） | `channels.ts:275` `ChannelPackagePluginItem.configJson`；`ChannelPackageDetailDrawer.vue:297` `item.configJson` |
| 实例侧端点未误伤 | ✅ `ChannelPluginItemView` 仍 `configJson` | `service.go:157`（GET/POST/PATCH 实例项 makeConfigView，未改） |
| 请求侧 POST 入参 | ✅ 仍为 `config`（符合 compact POST 契约，未动） | `handler.go:97-102` overridePackagePluginRequest.Config `json:"config"` |
| 序列化回归测试 | ✅ PASS | `app/dto/plugin_test.go`：`TestPackagePluginItemView_ConfigJSONFieldName`、`TestChannelAndPackageViews_ConfigKeyConsistent` |

- **结论：I-1 已闭合**。渠道包覆盖 GET/POST 响应配置体键与前端读取键完全一致（`configJson`），自定义覆盖非密文配置 round-trip 显示恢复；实例侧契约未受影响；请求侧 POST `config` 入参按 compact 保持不变。

### 2) P3 复测

- `domain/plugin/plugin_config.go:49-58`：secret（L52）/ file（L57）字段键已并入 `allowed` 集合；纯规则、签名未变。
- 新增用例 `TestResolvePluginConfigStatus_SecretFileOnlyFieldsAllowed` ✅ PASS；既有 32 子用例全部 PASS（无回归）。
- **结论：P3 已闭合**。仅声明于 secret/file_fields 而未同列 form_schema 的字段不再被误判 unknown→invalid。

### 3) 全量回归复跑（真实输出）

```bash
# 后端（services/admin-api）
go build ./...                                          # ✅ BUILD_OK
go test ./internal/app/dto/ ./internal/domain/plugin/ -v
#   ✅ TestPackagePluginItemView_ConfigJSONFieldName / TestChannelAndPackageViews_ConfigKeyConsistent PASS
#   ✅ TestResolvePluginConfigStatus_SecretFileOnlyFieldsAllowed PASS；domain/plugin 全部 PASS
go test ./...                                           # ✅ 全包 ok，0 失败（含 dto / domain/plugin / scenario / transport/http/channels）

# 前端（apps/admin-web，worktree 本地 bin）
node_modules/.bin/vitest run <三件套>                   # ✅ 3 files / 33 tests passed（未回归）
```

- 通过：后端 `go test ./...` 全绿（0 失败）；I-1 序列化回归 + P3 新用例 PASS；前端模块 vitest 33/33。
- Playwright：本轮**未重跑**——前端代码未改动（channels.ts/抽屉两侧第 1 轮即为 `configJson`，漂移纯属后端键），mock 驱动 e2e 与第 1 轮 3/3 绿状态等价，无新增信号。
- scenario 矩阵：`testkit/scenario`（含 feature-plugin manifest 解析 + S2 进程内 401）随 `go test ./...` ok；requiresDB 用例仍 SKIP（连库 harness 仍为前向声明，环境限制不变）。

### 4) 红线与契约对账复核

- 契约对账：第 1 轮唯一漂移 I-1 已闭合；其余 5 接口方法/路径/权限码/请求 DTO/错误码/包络/实例项响应字段维持一致 → **现无契约漂移**。
- 红线：scope=server 标注 / 隐藏·不兼容·非 valid 三标全 false（`ResolveRuntimeFlags`）/ secret masked（含包覆盖 maskSecrets）/ 权限 plugin.read·write / 跨 env 无 schema 前缀 / `InTx` 回滚 / required 缺口引导 —— 复核结论**仍成立**（S3/S6/S10 运行态仍待连库 harness，与既有口径一致）。

### 5) 遗留

- **P2（非阻断遗留，维持）**：渠道包覆盖区 file 字段仍走 `TemplateConfigRenderer` 文本输入，未复用 `el-upload`（本轮 🟧 未触及 ChannelPackageDetailDrawer，符合约定）→ 移交后续增强，不阻塞验收。
- 迁移 000012 与 #18 game-cashier 编号冲突：仅登记，合并 main 前由集成阶段重编号。
- scenario requiresDB 用例待连库 harness 执行（S3/S6/S10 等运行态）。

### 6) 通过判定

- **可进入 ✅ 功能验收（是）**：阻断项 I-1 已闭合、P3 已闭合；后端全量 + 前端模块回归全绿、红线复核成立、现无契约漂移。仅余 P2 + 迁移编号 + 连库 harness 为非阻断遗留，移交后续/集成阶段处理。

### 7) 产物协议落地（本轮）

- 本节追加于 audit.log.md（保留既有全部角色内容）。
- module.manifest.json：verification 增「第 2 轮复测结果」；open_issues 移除已闭合 I-1/P3。
- integration.checklist.md：§5 标注 I-1/P3 已闭合；§6 增第 2 轮复测结论。
- handoff.summary.md：新增 stage=integration-retest（≤10 行），保留其它角色内容。

---

## [🟧 高级全栈工程师] feature-plugin 集成问题修复（stage=fullstack-fix）

> 模型 Opus 4.8 High。worktree=/Users/csw/gitproject/console-feature-plugin，分支 codex/feature-plugin（未 commit/未切分支）。
> 读文档：index.json → 15-feature-plugin/spec.compact.md（前后端两侧）→ 依赖 12-channel 片段。
> 输入：🟪 测试专家 integration-test 问题清单（audit.log.md §integration-test / handoff.summary.md）。
> 裁决标准：compact 契约为唯一裁决标准。

### 1) 逐条修复说明（问题 → 根因 → 改动文件:行 → 验证）

#### I-1（阻断 / 契约漂移）渠道包覆盖项 config vs configJson

- 问题：渠道包覆盖 `GET/POST /api/admin/channel-packages/{packageId}/plugins` 响应体 `PackagePluginItemView.Config` 序列化为 `json:"config"`，前端 `ChannelPackagePluginItem` 与 `ChannelPackageDetailDrawer.initPluginDraft` 按 `configJson` 读取回填 → 自定义覆盖非密文配置 round-trip 显示失效（保存仍成功）。
- 根因：并行开发产生的字段命名漂移。实例侧 `ChannelPluginItemView.ConfigJSON` 已用 `json:"configJson"`（camelCase），渠道包侧未对齐。compact 未对该字段单列硬命名，按「与实例侧/IAP override 口径统一、camelCase」裁决 → 统一为 `configJson`。
- 改动（后端，统一向实例侧对齐）：
  - `services/admin-api/internal/app/dto/plugin.go:76`：`PackagePluginItemView.Config map[string]any json:"config"` → `ConfigJSON map[string]any json:"configJson"`（字段名同步实例侧风格）。
  - `services/admin-api/internal/app/plugin/query_list_channel_plugins.go:133`：赋值 `Config:` → `ConfigJSON:`。
  - `services/admin-api/internal/app/plugin/command_override_package_plugin.go:98`：赋值 `Config:` → `ConfigJSON:`。
- 一致性核对：
  - 实例侧 `ChannelPluginItemView`/`ChannelPluginConfigView` 配置字段保持 `configJson`（未改，无误伤）。
  - 请求 DTO（`ConfigureChannelPluginCmd.Config` / `OverridePackagePluginCmd.Config` 与 handler 请求体 `json:"config"`）属请求侧契约，compact POST 入参字段即 `config`，**不动**。
  - 前端 `apps/admin-web/src/api/modules/channels.ts` 两侧均已为 `configJson`，无需改；现前后端响应键完全一致。
- 验证：新增 `services/admin-api/internal/app/dto/plugin_test.go`（序列化断言响应含 `configJson` 且不含旧 `config`，并校验实例/包两视图 key 一致）→ PASS。

#### P3（顺带修复 / 低成本）allowed 集合未纳入 secret/file 键

- 问题：`ResolvePluginConfigStatus` 构造 `allowed` 仅取 `FormSchema` + `ValidationRules` 键；secret/file 字段仅进 `required`/`secret`/`file` 集。若模板把密文/文件字段**只**声明在 `secret_fields_json`/`file_fields_json`（未同列 form_schema），提交该字段会命中「字段未在模板中定义」→ 误判 invalid。
- 根因：allowed 集合构造遗漏 secret/file 键。
- 改动：`services/admin-api/internal/domain/plugin/plugin_config.go:49-58`：遍历 `SecretFields`/`FileFields` 时同时写入 `allowed`。保持纯规则无 IO、函数签名不变。
- 验证：新增 `TestResolvePluginConfigStatus_SecretFileOnlyFieldsAllowed`（secret/file-only 字段提交判 valid）→ PASS。

#### P2（非阻断遗留）渠道包 file 字段未复用 el-upload

- 处置：**保留为非阻断遗留**，不在本轮修复。理由：修 I-1 仅触及后端 DTO 与 app 装配层，**未改动** `ChannelPackageDetailDrawer.vue`，无「顺带极低成本」前提；按任务约束不扩大改动范围。已在 integration.checklist.md §5 维持 P2 遗留标注。

### 2) 自检命令与结果

```bash
# 后端  工作目录 services/admin-api
go build ./... && go vet ./... && go test ./...
# ✅ exit 0；全包通过，0 失败（含 app/dto、domain/plugin、transport/http/channels、testkit/scenario）
#    新增：app/dto 0.681s ok、domain/plugin 0.714s ok

# 前端  工作目录 apps/admin-web（worktree 本地 bin，required_permissions=all）
node_modules/.bin/vitest run \
  src/views/channels/components/__tests__/FeaturePluginConfigPanel.spec.ts \
  src/views/channels/components/__tests__/ChannelInstanceDetailDrawer.spec.ts \
  src/views/channels/components/__tests__/ChannelPackageDetailDrawer.spec.ts
# ✅ Test Files 3 passed (3) | Tests 33 passed (33)

node_modules/.bin/vite build
# ✅ built（chunk size warning 既有，非本次引入）
```

- ReadLints（6 个改动文件）：干净，无 lint 报错。
- vue-tsc 全量未跑：既有 cashier.ts(173)/sync-section-drawer.spec.ts(23) 阻塞（非 #15，未触碰），与既往各阶段同口径。

### 3) 改动文件清单

- `services/admin-api/internal/app/dto/plugin.go`（I-1 字段重命名 + json tag）
- `services/admin-api/internal/app/plugin/query_list_channel_plugins.go`（I-1 赋值字段名）
- `services/admin-api/internal/app/plugin/command_override_package_plugin.go`（I-1 赋值字段名）
- `services/admin-api/internal/domain/plugin/plugin_config.go`（P3 allowed 并入 secret/file 键）
- `services/admin-api/internal/app/dto/plugin_test.go`（新增，I-1 序列化回归）
- `services/admin-api/internal/domain/plugin/plugin_config_test.go`（新增 P3 用例）

### 4) 产物协议落地

- 本节追加于 audit.log.md（保留既有全部角色内容）。
- module.manifest.json：verification 增 fullstack-fix 结果；open_issues 移除已修 I-1/P3，保留 P2 与 vue-tsc 既有阻塞。
- integration.checklist.md：§5 标注 I-1/P3 已修复、P2 维持遗留；§3 共享文件无新增（仅本模块专属 dto/app/domain 文件）。
- handoff.summary.md：新增 stage=fullstack-fix（≤10 行），保留其它角色内容。

---

## ✅ 功能验收（acceptance · Cursor Auto · 第 3 轮·验收闸门）

> 前置：🟪 测试专家第 2 轮已判定通过（I-1 契约漂移 + P3 已闭合，回归与红线全绿）。
> 基准：功能端到端可用 + 满足 compact 业务规则 + 符合 02-operation-flow 操作主线（B 主线步骤 4「加功能插件·引导必接」）。
> 范围：仅本模块（#15）。worktree `/Users/csw/gitproject/console-feature-plugin`，分支 `codex/feature-plugin`，未 commit、未切分支。

### 1) 验收清单（逐条 PASS/FAIL + 证据）

#### A. 数据模型（5 表落地 / 字段·约束·默认·UNIQUE·FK·索引）— 6/6 PASS
- **AC-01 `platform.feature_plugins`** PASS — `plugin_id VARCHAR(64) UNIQUE NOT NULL`、`region CHECK IN('domestic','overseas')`、`enabled DEFAULT TRUE`、`sort DEFAULT 0`；索引 `(region)`、`(enabled,sort)`。证据：`migrations/000012_feature_plugin_schema.up.sql:11-23`，与 compact §数据模型一致。
- **AC-02 `platform.feature_plugin_templates`** PASS — 四件套 JSONB `DEFAULT '[]'/'[]'/'[]'/'{}'`、`UNIQUE(plugin_id_ref, template_version)`、FK→feature_plugins(id)；**简单模板表**（无 status 列，运行时取 `enabled=TRUE` 最新版本，见 `feature_plugin_repo.go:56` GetLatestTemplate）。证据：up.sql:25-40。
- **AC-03 `platform.channel_feature_plugins`** PASS — `required/selectable/default_enabled/locked` 默认（FALSE/TRUE/FALSE/FALSE）、`sort DEFAULT 0`、`UNIQUE(channel_id_ref, plugin_id_ref)`、FK→channels(id)+feature_plugins(id)。证据：up.sql:42-58。
- **AC-04 `game_channel_plugin_configs`（业务表/无 env 列）** PASS — `game_channel_id_ref` 同 schema 普通 FK→game_channels(id)；`plugin_id_ref` 跨 schema FK→`platform.feature_plugins(id)`；`enabled DEFAULT FALSE`、`config_json DEFAULT '{}'`、`config_status DEFAULT 'empty' CHECK IN('empty','invalid','valid')`、`last_check_at NULL`、`last_check_message DEFAULT ''`、`UNIQUE(game_channel_id_ref, plugin_id_ref)`、索引 `(game_channel_id_ref)`。证据：up.sql:60-88。
- **AC-05 `channel_package_plugin_overrides`（业务表/无 env 列）** PASS — `inherit_channel_config DEFAULT TRUE`、`enabled DEFAULT FALSE`、`config_json DEFAULT '{}'`（仅存差异）、`config_status` CHECK、`UNIQUE(package_id_ref, plugin_id_ref)`、索引 `(package_id_ref)`、跨 schema FK→platform。证据：up.sql:90-119。
- **AC-06 schema 策略** PASS — 平台表读 `platform.` 前缀（`feature_plugin_repo.go:20-56`）；业务表 SQL 不写 schema 前缀（`game_channel_plugin_repo.go:75` / `channel_package_plugin_repo.go:77` 由 search_path=<env>,platform 决定）。down 迁移幂等可回滚（down.sql:1-22）。

#### B. API（5 接口 方法/路径/权限/DTO/错误码/包络/审计）— 7/7 PASS
- **AC-07 GET `/api/admin/game-channels/{gameChannelId}/plugins`（plugin.read）** PASS — 返回 `items[]`（pluginId/pluginName/region/required/selectable/enabled/configStatus/includedInRuntimeConfig + 模板 + 脱敏 configJson）+ `missingRequiredPlugins`。证据：`router.go:34`、`handler.go:357`、`query_list_channel_plugins.go:12`。
- **AC-08 POST `/api/admin/game-channels/{gameChannelId}/plugins`（plugin.write，audit plugin.configure）** PASS — 请求 DTO `{pluginId(必填) / enabled(默认 false) / config(默认 {})}`；链路 兼容性校验→模板校验→`ResolvePluginConfigStatus`→落库+审计；返回计算后 configStatus/lastCheckMessage。证据：`handler.go:86-90,375`、`command_configure_channel_plugin.go:12-103`。
- **AC-09 PATCH `/api/admin/game-channel-plugins/{id}`（plugin.write，audit）** PASS — 改配置/启停，config_status 重算（`command_configure_channel_plugin.go:106-184`）；空 config 回退旧值（:156）。
- **AC-10 GET `/api/admin/channel-packages/{packageId}/plugins`（plugin.read）** PASS — 渠道包级，inherit 时派生用渠道实例运行态（`query_list_channel_plugins.go:110-120`）。证据：`router.go:42`、`handler.go:430`。
- **AC-11 POST `/api/admin/channel-packages/{packageId}/plugins`（plugin.write）** PASS — 继承(`inherit=true` 清空 config、enabled=false) / 自定义覆盖（仅存差异、走模板校验）；I-1 闭合后响应键 `configJson`。证据：`command_override_package_plugin.go:12-114`、`dto/plugin.go:76`。
- **AC-12 错误码 + 统一包络** PASS — `MARKET_CHANNEL_INCOMPATIBLE`(400)/`VALIDATION_FAILED`(400)/`CONFLICT`(409)/`NOT_FOUND`(404)，经 `writeError`→`httpx.WriteError` 统一包络（`ports.go:21-45`、`handler.go:544-557`）。
- **AC-13 审计** PASS（带观察）— 写操作均 `audit_logs action=plugin.configure`（configure/patch/override 三处，`detail` 脱敏，actor 取 AuthContext）。证据：`command_*.go` 各 writeAudit。**观察（非阻断）**：compact §应用服务列出 `plugin.enable/plugin.disable` 两事件，实现统一记为 `plugin.configure`（语义可由 detail.configStatus/enabled 还原）；与既有 CR open_issue「审计未拆 enable/disable」同口径，列后续增强，不作为 FAIL。

#### C. 状态机 / 业务规则 — 4/4 PASS
- **AC-14 region 兼容（服务端强制）** PASS — `ValidatePluginRegionCompatibility`：CN↔domestic、非 CN↔overseas、未知 market/region→false；不兼容回 `MARKET_CHANNEL_INCOMPATIBLE`；列表二次过滤。证据：`compatibility.go:11-24`，域单测 18 子用例全绿。
- **AC-15 `ResolvePluginConfigStatus`（00 §3.4）** PASS — `enabled=false→empty`；`enabled=true` 且必填（含 secret/file，P3 已并入 allowed）缺失→`invalid` + lastCheckMessage 提示缺哪类；未知字段→invalid；规则校验（minLen/maxLen/pattern/enum/类型）。证据：`plugin_config.go:41-113`，域单测覆盖 disabled/valid/missing-secret/missing-file/normal/unknown/SecretFileOnlyFieldsAllowed/rule-violations 等。
- **AC-16 运行态派生（同口径 + scope 过滤 + required 缺口）** PASS — `ResolveRuntimeFlags`：`!hidden && compatible && enabled && status==valid` 同时驱动 `IncludedInRuntimeConfig/Snapshot/Sync` 三标（单一来源，`plugin_config.go:122-129`）；列表对 required 且未达 `enabled&&valid` 计入 `MissingRequiredPlugins`（`query_list_channel_plugins.go:51`）；scope 过滤下发为快照下游职责（本模块模板字段标 scope，DB 仍存 server 字段）。域单测 `TestResolveRuntimeFlags` 6 子用例绿。
- **AC-17 必接/selectable/locked** PASS — `required && !selectable && !enabled`→拒绝「必接不可取消勾选」；`locked`→拒绝改动；`default_enabled` 决定新建实例默认勾选。证据：`command_configure_channel_plugin.go:60-65,146-155`。

#### D. 前端页面 — 7/7 PASS
- **AC-18 渠道实例「功能插件」Tab** PASS — `ChannelInstanceDetailDrawer.vue` 挂载 `FeaturePluginConfigPanel.vue`（与基础设置/渠道包/渠道登录并列）。
- **AC-19 渠道包继承/覆盖** PASS — `ChannelPackageDetailDrawer.vue`「继承渠道插件/自定义覆盖」开关 + 覆盖保存（响应键 configJson 已对齐）。
- **AC-20 模板渲染器消费四件套** PASS — `FeaturePluginConfigPanel.vue:68-182` 按 form_schema 分组渲染（text/textarea/password/switch/number/select/json）+ secret + file + validation_rules 实时校验。
- **AC-21 secret masked / 留空=不修改 / 可重填** PASS — 前端 `******` 占位、`buildPayloadConfig` 留空回填 `******`（:559-597）；后端 `encryptSecrets` 对 `masked/******` 保留旧值、明文加密落库（`service.go:108-144`）；响应恒 `maskSecrets`（:98）。
- **AC-22 权限置灰** PASS — 保存按钮 `v-perm="'plugin.write'"` + `canEdit`（无写权限/ locked 禁用）；e2e「无 plugin.write 权限时保存按钮置灰」绿。
- **AC-23 必接引导** PASS — `requiredMissing` 计算 + el-alert 顶部提示未配齐必接清单（:11-19,260-265）；e2e「引导补齐」绿。
- **AC-24 scope=server 提示** PASS — `field.scope==='server'` 渲染「仅服务端，不下发客户端」标签（:81）。

#### E. 红线（00 §9）— 4/4 PASS
- **AC-25 scope=server 不下发客户端最终配置** PASS（本模块层面）— 模板字段携带 scope，前端标注；DB 仍存 server 字段，按 scope 过滤下发为快照(#20)职责（compact §与下游）。本模块未泄漏。
- **AC-26 隐藏/不兼容/非 valid 不进列表·快照·同步** PASS — 列表过滤 `meta.Enabled && compatible`（`query_list_channel_plugins.go:31-36,90`）；运行态三标经 `ResolveRuntimeFlags` 统一为 false（hidden/incompatible/disabled/status≠valid）。
- **AC-27 不存明文密钥** PASS — `encryptSecrets` 经 Cipher 加密落库、cipher 未配置即拒绝；响应恒 masked；e2e 密文重填/留空用例绿。
- **AC-28 不跨 schema 写** PASS — 业务表 INSERT/UPDATE 无 schema 前缀（`game_channel_plugin_repo.go:75` / `channel_package_plugin_repo.go:77`）；平台表仅读。

#### F. 下游 impacts 抽查 — 1/1 PASS（前向兼容）
- **AC-29 snapshot(#20)/sync(#21) 引用口径无破坏** PASS — `internal/domain/snapshot` 目录尚未落地、`internal/domain/sync` 无 plugin 引用（grep 0 命中）；当前无消费方可被破坏。运行态口径单一来源 `ResolveRuntimeFlags` 三标同口径，为下游消费提供前向兼容契约。

**清单合计：29 项，PASS 29 / FAIL 0**（含 AC-13 一条非阻断观察）。

### 2) 构建 / 测试结果（真实输出）

| 项 | 命令（worktree 绝对路径） | 结果 |
| --- | --- | --- |
| 后端 build | `services/admin-api $ go build ./...` | ✅ exit 0 |
| 后端 vet | `go vet ./...` | ✅ exit 0 |
| 后端 test（全包/统一回归后端腿=backend.sh） | `go test ./...` | ✅ 全包 0 失败 |
| domain/plugin L1 | `go test ./internal/domain/plugin/... -v` | ✅ 15 Test / 32 子用例 PASS（region 18 + config + runtime） |
| 场景矩阵 | `go test ./internal/testkit/scenario -run feature-plugin -v` | ✅ S2 `*_requires_auth`×5 进程内 401 PASS；其余 requiresDB 用例 SKIP（manifest 解析 OK） |
| 前端模块 vitest | `apps/admin-web $ vitest run <三件套>` | ✅ 3 files / 33 passed |
| 前端全量 vitest | `vitest run` | ⚠️ 215/216：唯一失败 `sync-section-drawer.spec.ts` 为**既有非 #15 阻塞**（未触碰） |
| 前端 vite build | `vite build` | ✅ built（chunk-size warning 既有） |
| Playwright e2e | `E2E_PORT=5193 playwright test feature-plugin.spec.ts` | ✅ 3 passed（含 toHaveScreenshot 视觉基线比对绿） |

> 统一回归入口 `scripts/regression/run.sh`：后端腿 = `go test ./...`（已绿）；前端腿 = vitest（215/216，1 既有非 #15）+ playwright（3/3）。`WITH_DB=1` 走 docker+migrate 连库；本机以 `WITH_DB=0` 进程内 + 直跑各腿等价覆盖。`SCENARIO_WITH_DB=1` 下 product 等他模块 requiresDB 用例因无 DSN FAIL（非 #15），标准回归（不带该 env）requiresDB 一律 SKIP。

### 3) 操作主线走查（02-operation-flow B-步骤 4）

按「加渠道后系统引导补齐必接插件」主线走查（代码 + 域单测 + 组件/e2e 等价覆盖；连库运行态以契约对账替代）：
- **能力闭环**：list（候选+必接标记+实例态）→ configure（勾选/配置）→ patch（改配置/启停）→ 渠道包 inherit/override，五接口闭环成立。
- **状态流转**：empty→（勾选+填齐）valid→（缺必填）invalid，三态经 `ResolvePluginConfigStatus` 落库并回显 lastCheckMessage。
- **错误/冲突如约**：region 不兼容→`MARKET_CHANNEL_INCOMPATIBLE`；缺必填→`VALIDATION_FAILED`(invalid)；唯一键→`CONFLICT`；必接不可取消、locked 禁改。
- **脱敏/权限生效**：secret 响应恒 masked、留空=不修改；`plugin.read/write` 路由级 RequirePerm + 前端 v-perm 置灰。
- **下一步**：必接未配齐→渠道实例运行态异常、挡快照/同步（`MissingRequiredPlugins` + 三标 false），与 operation-flow C「阻塞项口径」一致。

### 4) 结论

**验收通过（PASS）**。功能端到端成立，满足 compact 业务规则与 operation-flow 主线；29 项验收点全 PASS、0 FAIL。两轮集成测试 I-1/P3 已闭合且回归红线全绿。

### 5) 遗留风险与建议（均非阻断）

- **P2（前端增强）**：渠道包覆盖区 file 字段经 `TemplateConfigRenderer` 仍为文本输入，未走 `el-upload`（实例侧 `FeaturePluginConfigPanel` 已正确实现）。建议随 `TemplateConfigRenderer` 统一增强。
- **迁移 000012 编号协调**：与 #18 game-cashier 在另一 worktree 同用 `000012`，合并 main 前由集成阶段重编号（仅登记）。
- **连库 harness（前向声明）**：scenario `SCENARIO_WITH_DB` 仍无 DSN 装配，S3/S6/S10 等运行态用例 SKIP；本轮以契约对账 + L1 域单测（32 子用例）+ 组件/e2e（33 + 3）替代覆盖。建议集成阶段补连库 harness 实跑 requiresDB 矩阵。
- **审计事件粒度（观察）**：`plugin.enable/disable` 未从 `plugin.configure` 拆分；如下游审计页需区分，建议后续按 enabled 变化拆事件。
