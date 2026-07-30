---
id: channel/platform-admin
parent: channel
title: 平台渠道与渠道模版管理（system 侧）
status: target
code_paths:
  - services/admin-api/internal/domain/channel/platform_admin.go
  - services/admin-api/internal/app/platformchannel
  - services/admin-api/internal/app/dto/platform_channel.go
  - services/admin-api/internal/infra/persistence/postgres/platform_channel_repo.go
  - services/admin-api/internal/transport/http/platformchannel
  - apps/admin-web/src/api/modules/platformChannels.ts
  - apps/admin-web/src/views/channels/ChannelsView.vue
  - apps/admin-web/src/views/channels/components/platform
depends_on: [common, channel]
impacts: [channel-login, product]
---

# 平台渠道与渠道模版管理（system 侧）

> 本文是 `channel` 模块的子文档，描述**系统管理员**维护「平台渠道主数据 / 渠道策略 / 渠道模版」的口径。父文档 [`channel`](./README.md) §1.1.1 给出分层，§6.7 给出完整 API 契约，本文只讲**语义、不变量与为什么**，不重复 DTO 表格。
>
> 一句话定位：**渠道是平台基础数据，不是游戏配置。** 系统管理员先建渠道、再给渠道挂登录 / IAP 模版；游戏运营在自己的渠道实例上**引用模版填参**。本文对应 `00 §4.4`「模板本身由基础数据/模板管理后台维护」——该约定一直存在，**本轮把 system 侧实现补齐了**。

---

## 1. 边界：谁维护什么

| 对象 | 表（均在共享 schema `platform`） | 维护者 | 权限码 |
| --- | --- | --- | --- |
| 渠道主数据 | `channels` | 系统管理员 | `platform_channel.read/write` |
| 渠道策略（登录/支付模式 + 锁定位） | `channel_policies` | 系统管理员 | `platform_channel.read/write` |
| 渠道登录模版四件套 | `channel_login_templates` | 系统管理员 | `channel_template.read/write` |
| 渠道 IAP 模版四件套 | `channel_iap_templates` | 系统管理员 | `channel_template.read/write` |

**负责**：渠道主数据与策略的新建/编辑/启停；两类模版版本的新建/编辑/启停；四件套自洽性校验（`channel/platform_admin.go` 纯函数，无 IO）。

**不负责**：

- 渠道实例（`game_channels`）、渠道包（`channel_packages`）的增删改 —— 属父文档 [`channel`](./README.md)，权限 `channel.*`。
- 用模版**填参**（`game_channel_login_configs` / IAP 配置）与 `config_status` 推导 —— 属 [`channel-login`](../14-channel-login/README.md) 与 [`product`](../16-product/README.md)。
- `account_auth_templates`（[`account-auth`](../13-account-auth/README.md)）、`feature_plugin_templates`（[`feature-plugin`](../15-feature-plugin/README.md)）、`cashier_provider_templates`（[`cashier-template`](../17-cashier-template/README.md)）—— 同为简单模板表、同一维护理念，但**不在本入口内**，各自模块自行提供或待补。
- 删除渠道 / 删除模版版本：**不提供**。渠道被实例与模版引用，模版被历史配置引用，一律用 `enabled=false` 下线（`00 §7.5` `write` 覆盖启停的口径）。

---

## 2. 数据模型：不新增表

本轮**没有新表**。渠道主数据/策略见父文档 §3.1、§3.2；两张模版表结构见 [`channel-login §3.1`](../14-channel-login/README.md)（`channel_login_templates`）与 [`product`](../16-product/README.md)（`channel_iap_templates`），二者**同构**：`(channel_id_ref, template_version)` + 四件套 4 个 JSONB + `enabled` + 时间戳，`UNIQUE(channel_id_ref, template_version)`。

唯一的迁移是权限码 seed：`migrations/000018_platform_channel_admin_perms.{up,down}.sql`，向 `platform.admin_permissions` 插入 4 个权限码并给 `super_admin` 补授（`ON CONFLICT DO NOTHING`，可重复执行）。

### 2.1 env 语义：跨运行环境共享

这 4 张表都在共享 schema `platform`，**不带 `env` 列、也不按环境各存一份**（`00 §2.2`）。含义：

- 系统管理员在任一环境登录后改渠道/模版，`develop`/`sandbox`/`production` **同时看到同一份定义**；不存在"把渠道从 sandbox 同步到 production"这回事，[`sync`](../21-sync/README.md) 的 `section=channels` 同步的是**渠道实例与其配置**，不是平台渠道与模版。
- 仓储 SQL 对这些表**显式写 `platform.` 前缀**（不依赖 `search_path` 的 env schema），与游戏维度业务表"不写 schema 前缀"的规则相反 —— 因为它们与当前 env 无关。
- 前端「渠道管理」页因此**不挂 `EnvironmentBadge`**（§6）。

---

## 3. 模版版本管理：不走 draft/published 双态

与 `00 §4.4.1` 完全一致，本文只把它落到操作层面：

- 简单模板表**没有 `status` 列**，`00 §3.3` 的 `VersionStatus`（draft/published/archived）三态机**只适用于 cashier 价格模板版本**，与渠道模版无关。
- 版本控制只有两个字段：`enabled` + `template_version`。
- **运行时取「`enabled=true` 的最新 `template_version`」**，即响应里的派生字段 `effective`（同渠道同 `kind` 内最多一个 `true`）。与 `channel-login §5.2` / `product` 的取版本逻辑同源，`effective` 只是把"运行时会用哪个版本"提前展示给管理员，避免"改了个没人用的旧版本"。
- **升级** = 新建更高的 `template_version`；**下线** = `enabled=false`；**不建议原地改写已在用的版本**（会让已填参的渠道实例在下次校验时突然 `invalid`）。因此 `PATCH` 明确禁止改 `templateVersion`：要改版本号就是新建版本。
- `login` / `iap` **分表同构**，版本号空间彼此独立：同一渠道下 `login/v2` 与 `iap/v2` 互不冲突，也互不影响各自的 `effective`。

### 3.1 四件套编辑约束（为什么这么严）

模版是**下游所有填参与校验的唯一事实来源**，一个自相矛盾的模版会让整条链路失效，所以写入时做整体自洽性校验（清单见父文档 §6.7.5），其中两条最关键：

- **`component=password` 的 key 必须登记进 `secretFieldsJson`**：`secret_fields_json` 是下游"加密落库 + 响应脱敏"的开关（`00 §6.1`）。只把控件画成密码框但没登记，等价于**密钥明文入库**。
- **`component=file` 的 key 必须登记进 `fileFieldsJson`**：`accept` / `maxSizeKB` 约束、以及"复制创建时清空文件字段"（父文档 §5.2）都依赖这份登记。

`PATCH` 采用**四件套整体替换**语义（省略=保留、传了=整段覆盖），并对**合并后的完整模版**重跑全部校验 —— 防止"只改 `formSchemaJson`、忘了同步 `secretFieldsJson`"留下不自洽的版本。

---

## 4. 渠道身份不可变：`channelId` / `channelType` / `region`

创建后这三个字段**不可修改**（不出现在 `PATCH` 补丁里，前端表单在编辑态置灰并给出原因）：

| 字段 | 不可改的原因 |
| --- | --- |
| `channelId` | 它是**渠道实例的引用键**：`game_channels.channel_id_ref → platform.channels(id)` 的业务语义、模版归属、支付路由选择器、快照与同步的 diff key 都以 `channelId` 表达。改它等于悄悄改写所有游戏的渠道身份。 |
| `region` | 它决定**与 market 的兼容性**（父文档 §5.1：`market=CN ⇒ domestic`、`market!=CN ⇒ overseas`）。兼容性是**实时派生、不落库**的，改 `region` 会让既有渠道实例**集体失配**（整片标红、退出快照/同步），且没有任何"迁移"语义可言。 |
| `channelType` | 渠道类型是分类事实（`store/oem/web/direct/mini_game`），下游按类型做展示与分组；改类型属于"换了一个渠道"，语义上应另建。 |

需要换身份 ⇒ **另建渠道**，把旧渠道 `enabled=false` 下线。可改的只有：`channelName`、`enabled`、`sort`、`loginMode`、`paymentMode`、`loginLocked`、`paymentLocked`。

> 提醒：`loginMode` / `paymentMode` 虽可改，但会切换该渠道走 [`channel-login`](../14-channel-login/README.md)（`channel_only`）还是 [`account-auth`](../13-account-auth/README.md)（`account_system`）、以及支付形态。改动前需确认既有渠道实例的配置去向；`login_locked` / `payment_locked` 的语义是"游戏侧不可改该策略"（父文档 §3.2、`channel-login §5.5`）。

---

## 5. 游戏侧如何消费

```text
系统管理员（本文）                            游戏运营（channel / channel-login / product）
platform.channels(channelId, region, ...)  ──┐
platform.channel_policies(loginMode, ...)  ──┤
                                             ├─► game_channels（渠道实例，按 (game, market, channel) 建）
platform.channel_login_templates(四件套) ────┤        └─► game_channel_login_configs（按模版填参、密文加密、推导 config_status）
platform.channel_iap_templates(四件套) ──────┘        └─► IAP 配置（按模版填参）
```

1. 系统管理员在「渠道管理」新增渠道（含 `region` 与策略），再为该渠道新建 `login` / `iap` 模版版本并启用。
2. 游戏运营在游戏详情页「渠道」页签新增渠道实例：候选渠道按 `region` × `market` 兼容性过滤（父文档 §5.1），只能从**已有平台渠道**里选，选不到就得回来找系统管理员建。
3. 打开实例的登录 / IAP 配置：后端取该渠道**当前 `effective` 的模版版本**下发四件套，前端用模板驱动渲染器（`01 §5.3`）渲染表单；提交后按 `validation_rules_json` 校验、按 `secret_fields_json` 加密、推导 `config_status`。
4. 只有 `config_status=valid` 且未隐藏/兼容/启用的实例才进快照、同步与客户端最终配置（`00 §9` 红线）。

**边界提醒**：渠道下没有启用的模版版本时，下游按各自模块口径处理（`channel-login §11.2` 当前为"拒绝写入 + 提示该渠道未配置登录模板"），本模块**不提供无模版兜底自由表单**。

---

## 6. 前端信息架构

- 入口：顶部菜单「渠道管理」（路由 `channels`，`meta.perm = platform_channel.read`）→ `views/channels/ChannelsView.vue`。**不需要先选游戏**，页面标题即「渠道管理」（不再是"渠道实例管理"）。
- 结构：`PageCard` + 两个页签
  - 「渠道」`components/platform/PlatformChannelsPanel.vue`：关键字 / `region` / 渠道类型 / 启用状态过滤 + 分页表格（列含类型、region、登录/支付模式、锁定位、`登录 N / IAP M` 模版数、启用状态）；抽屉承载新建/编辑，编辑态把 `channelId` / `channelType` / `region` 置灰并写明原因（§4）。
  - 「渠道模版」`components/platform/ChannelTemplatesPanel.vue`：先选渠道（模版隶属渠道，未选时给空态引导），再按 `kind` 过滤版本列表（列含版本、种类、字段数、敏感/文件字段数、启用、**是否生效**、更新时间）；抽屉内 `components/platform/TemplateQuartetEditor.vue` 编辑四件套，草稿状态与前端预校验在 `components/platform/templateDraft.ts`。
- 写操作按钮统一挂 `v-perm`（`platform_channel.write` / `channel_template.write`），无权限置灰（`01 §5`）。
- **不显示 `EnvironmentBadge`**：`platform.*` 跨环境共享（§2.1），与 `views/system/SystemView.vue` 同口径。
- API 客户端：`api/modules/platformChannels.ts`（与游戏侧 `api/modules/channels.ts` 分开，复用其枚举与类型）。

---

## 7. 测试要点

### 接口场景矩阵（→ 见 [`03-testing`](../../03-testing.md) §4）

> 维度定义见 `03-testing §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨 env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端就近测试：`internal/transport/http/platformchannel/platform_channel_http_test.go`（配 `memstore_test.go` 内存仓储）、`internal/domain/channel/platform_admin_test.go`；前端 e2e：`tests/frontend/e2e/platform-channels.spec.ts`。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET /api/admin/platform/channels | ✓ | ✓ | ✓ | ✓ | — | — | — | — | ✓ | — | keyword/region/channelType/enabled 过滤；模版版本数统计 |
| POST /api/admin/platform/channels | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | channelId 格式；主数据+策略同事务 |
| GET /api/admin/platform/channels/{channelId} | ✓ | ✓ | ✓ | — | — | — | — | — | — | — | 不存在 ⇒ 404 |
| PATCH /api/admin/platform/channels/{channelId} | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | — | — | ✓ | channelId/channelType/region 不可改；空 patch 幂等不写审计；策略 upsert |
| GET /api/admin/platform/channels/{channelId}/templates | ✓ | ✓ | ✓ | ✓ | — | — | — | — | — | — | kind 省略返两类；effective 判定 |
| POST /api/admin/platform/channels/{channelId}/templates | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | 四件套自洽性全清单；同渠道同 kind 版本号唯一 |
| GET /api/admin/platform/channel-templates/{kind}/{templateId} | ✓ | ✓ | ✓ | ✓ | — | — | — | — | — | — | kind 非法/templateId 非 int64 ⇒ 400 |
| PATCH /api/admin/platform/channel-templates/{kind}/{templateId} | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | — | — | ✓ | 整体替换 + 合并后重校验；templateVersion 不可改；空 patch 幂等不写审计 |

> S6（跨 env）对本组接口**不适用**：`platform.*` 跨环境共享（§2.1）。对应的替代用例是"在不同 env 登录读到同一份渠道/模版定义"。S8（脱敏）也不适用：模版只存**字段定义**，不存字段值，本组接口不接触任何密文。

### 补充关键用例

- 权限隔离：只有 `channel.write` 而无 `platform_channel.write` ⇒ 创建渠道 403（验证不复用 `channel.*`）。
- 四件套红线：`component=password` 未登记 secret、`component=file` 未登记 file、`select` 无 `options`、`secretFieldsJson` 出现未声明 key、`pattern` 非法正则 ⇒ 均 `VALIDATION_FAILED` 且 `details` 指到具体 `field`。
- `effective` 判定：同渠道同 kind 下建 `v1`(enabled)、`v2`(enabled) ⇒ 只有 `v2` 为 `effective`；把 `v2` 置 `enabled=false` ⇒ `v1` 变 `effective`。
- 分表独立：同渠道下 `login/v1` 与 `iap/v1` 可共存，互不冲突且各自有 `effective`。
- 整体替换：只传 `secretFieldsJson` ⇒ `formSchemaJson` 保持原值；传入与 `formSchemaJson` 不自洽的 `secretFieldsJson` ⇒ 被合并后校验拦下。
- 前端 e2e：两页签列表渲染、新建渠道 / 新建模版版本、编辑态身份字段只读、生效版本标记、无写权限时按钮置灰、页面不出现 env 徽标。

---

## 8. 显式假设与未决问题

### 8.1 假设

1. **不提供删除**：渠道与模版版本一律用 `enabled=false` 下线，保留历史可追溯（与 `00 §7.5` 的 `write` 粗粒度一致，不单列 `*.delete`）。
2. **模版只管定义、不碰值**：本组接口不读写任何配置实例，因此不涉及加密/脱敏；`secret_fields_json` 只是"声明哪些 key 是密文"。
3. **本轮只覆盖 login / iap 两类模版**：`account_auth` / `feature_plugin` / `cashier_provider` 三类简单模板表的 system 侧维护入口按同一套理念后续补齐（结构同构，可直接复用 `ValidateChannelTemplate` 的校验清单与 `TemplateQuartetEditor`）。
4. **`sort` 取值 0..9999**，仅影响列表展示顺序，无业务语义。

### 8.2 未决问题

1. **`template_version` 的排序口径**：`effective` 依赖"最新版本"，当前实现按 `template_version` **字符串降序**取首个 `enabled`。因此 `v10` 会排在 `v9` **之前**（字符串比较 `"v9" > "v10"`），语义化版本号不被识别。建议约定**定宽版本号**（如 `v001`/`2026.01`）或后续引入显式排序序号；在此之前，管理员命名需自觉保证字典序 = 时间序。
2. **改 `loginMode`/`paymentMode` 对既有渠道实例的连带影响**：目前只改平台策略，不回扫既有实例的配置有效性。是否需要"策略变更后把受影响实例标 `invalid`"待与 [`snapshot`](../20-snapshot/README.md) / [`sync`](../21-sync/README.md) 对齐。
3. **后端场景矩阵 manifest**：其它模块在 `tests/backend/scenarios/*.yaml` 有 manifest，本组接口当前只有 Go 就近测试，未补 `platform-channel.yaml`。
4. **`form_schema_json` 的 `default` 字段**：`00 §4.1` 的单字段结构含 `default`（缺省 `""`/`null`），但当前渠道模版的字段结构（后端实体与本组接口的 DTO）**未包含 `default`**，因此系统管理员暂时无法通过本入口给字段设默认值。待确认是补齐 `default` 还是从 `00 §4.1` 中收敛掉该字段。
5. **`scope` 允许空串**：`00 §4.1.1` 要求模板字段都带 `scope`；本组接口把 `''` 也视为合法（按 `00 §4.1.1` 的缺省语义等价于 `both`）。若要强制显式声明，需把 `''` 从允许集合里去掉并回填既有模版。
