---
id: feature-plugin
code: "15"
title: 功能插件（Feature Plugin）
status: target
code_paths:
  - services/admin-api/internal/domain/plugin
  - apps/admin-web/src/views/channels
depends_on: [channel, game, common]
impacts: [snapshot, sync, testing]
children: []
---

# 15 · 功能插件（Feature Plugin）

> 本模块与「渠道」同级：游戏在某渠道实例（GameMarketChannel）上接入若干**功能插件**（如实名、客服、推送、防沉迷等），每个插件由**参数模板**驱动配置。
> 完全复用渠道 + IAP 的成熟模式：平台级主数据 + 模板四件套 + 渠道级配置实例 + 渠道包覆盖。阅读前请先读 `00-common.md`（枚举/模板四件套/`scope`/红线）与 `01-structure.md`（env/分层），强依赖 `12-channel`（渠道实例挂载点）与 `00 §3.4 ConfigStatus`。

---

## 1. 模块概述与边界

### 1.1 职责
- 维护平台级**功能插件主数据** `feature_plugins`（含 `region` 国内/海外）与**插件参数模板** `feature_plugin_templates`（模板四件套）。
- 维护**渠道可接插件策略** `channel_feature_plugins`（必接/可勾选/默认勾选/锁定）。
- 维护游戏在某渠道实例下的**插件配置实例** `game_channel_plugin_configs`，以及**渠道包级覆盖** `channel_package_plugin_overrides`。
- 实现插件的**国内/海外可见性校验**、**必接引导**、**勾选必填校验（异常态）**、**渠道包继承/覆盖**。

### 1.2 上下游
- 依赖：`12-channel`（渠道实例与渠道包为挂载点）、`11-game`（游戏维度）、`00-common`（模板四件套/scope/ConfigStatus/红线）。
- 被依赖：`20-snapshot`（有效插件配置按 scope 进客户端最终配置）、`21-sync`（插件配置随同步集流转）。

### 1.3 边界红线（来自 `00 §9`）
- 插件参数中标记为 `scope=server` 的字段不下发客户端最终配置（`00 §4.1.1`、`20-snapshot §5.1.1`）。
- 被隐藏 / 不兼容 / 无效（`config_status != valid`）的插件实例：**不进默认列表、不进快照、不参与同步、不进客户端最终配置**。
- 必接（`required`）插件未配置为 `valid` ⇒ 所属渠道实例运行态异常，不进快照/同步。

---

## 2. 领域模型与聚合

- `FeaturePlugin`（平台主数据）：插件身份 + `region`（`domestic`/`overseas`）。
- `ChannelFeaturePlugin`（平台级策略）：某渠道下可接哪些插件、是否必接（`required`）、前端是否可勾选（`selectable`）、默认勾选（`default_enabled`）、锁定（`locked`）。
- `GameChannelPluginConfig`（游戏维度业务表，每环境独立 schema）：挂在 `game_channels` 行上的某插件配置；一个渠道实例可挂**多个**插件。
- `ChannelPackagePluginOverride`（游戏维度业务表，每环境独立 schema）：渠道包级覆盖；默认 `inherit_channel_config=true` 沿用渠道实例的插件集合与配置。
- 纯规则：
  - `ValidatePluginRegionCompatibility(market, region)`：与 `12-channel` 的渠道可见性同源（`market=CN ⇒ domestic`；`market!=CN ⇒ overseas`），服务端强制。
  - `ResolvePluginConfigStatus(enabled, template, config)`：勾选但缺必填/敏感/文件字段 ⇒ `invalid`（遵循 `00 §3.4`）。

---

## 3. 数据模型

> 约定见 `00 §2.2`：`game_channel_plugin_configs`、`channel_package_plugin_overrides` 为**游戏维度业务表**，在每个环境 schema 各一份同名同结构表（**不带 `env` 列**，行属于哪个 env 由所在 schema 决定）；`feature_plugins`、`feature_plugin_templates`、`channel_feature_plugins` 为平台级共享表，放在共享 schema `platform`。

### 3.1 `feature_plugins`（平台级，对标 `channels`）

| 列 | 类型 | 可空 | 默认 | 约束/说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `plugin_id` | VARCHAR(64) | 否 | — | 业务键，UNIQUE |
| `plugin_name` | VARCHAR(64) | 否 | — | |
| `region` | VARCHAR(16) | 否 | — | CHECK in `domestic/overseas` |
| `enabled` | BOOLEAN | 否 | `TRUE` | |
| `sort` | INT | 否 | `0` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(plugin_id)`。索引建议：`(region)`、`(enabled, sort)`。

### 3.2 `feature_plugin_templates`（平台级，模板四件套，对标 `account_auth_templates`）

| 列 | 类型 | 可空 | 默认 | 约束/说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `plugin_id_ref` | BIGINT | 否 | — | FK→feature_plugins(id) |
| `template_version` | VARCHAR(32) | 否 | — | |
| `form_schema_json` | JSONB | 否 | `[]` | 字段含 `scope`（`00 §4.1.1`） |
| `secret_fields_json` | JSONB | 否 | `[]` | |
| `file_fields_json` | JSONB | 否 | `[]` | |
| `validation_rules_json` | JSONB | 否 | `{}` | |
| `enabled` | BOOLEAN | 否 | `TRUE` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(plugin_id_ref, template_version)`。该表为**简单模板表**（`00 §4.4.1`），无 `status` 列，不走 §3.3 三态机；运行时取 `enabled=TRUE` 的最新 `template_version`。

### 3.3 `channel_feature_plugins`（平台级，对标 `channel_account_auth_types`）

| 列 | 类型 | 可空 | 默认 | 约束/说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `channel_id_ref` | BIGINT | 否 | — | FK→channels(id) |
| `plugin_id_ref` | BIGINT | 否 | — | FK→feature_plugins(id) |
| `required` | BOOLEAN | 否 | `FALSE` | **必接**：未配置 `valid` ⇒ 渠道实例异常 |
| `selectable` | BOOLEAN | 否 | `TRUE` | **前端可勾选**；必接项一般为 `FALSE`（强制接入） |
| `default_enabled` | BOOLEAN | 否 | `FALSE` | 新建渠道实例时是否默认勾选 |
| `locked` | BOOLEAN | 否 | `FALSE` | 锁定后游戏侧不可改勾选状态 |
| `sort` | INT | 否 | `0` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(channel_id_ref, plugin_id_ref)`。约束：`plugin.region` 须与渠道使用场景 market 兼容。

### 3.4 `game_channel_plugin_configs`（游戏维度业务表，每环境独立 schema，不带 env 列，对标 `game_channel_iap_configs`，多插件）

| 列 | 类型 | 可空 | 默认 | 约束/说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `game_channel_id_ref` | BIGINT | 否 | — | FK→game_channels(id)（同 schema 普通外键） |
| `plugin_id_ref` | BIGINT | 否 | — | FK→platform.feature_plugins(id)（跨 schema 指向平台表） |
| `enabled` | BOOLEAN | 否 | `FALSE` | 是否勾选接入 |
| `config_json` | JSONB | 否 | `{}` | 插件参数（密文加密、scope 标记由模板定义） |
| `config_status` | VARCHAR(16) | 否 | `'empty'` | CHECK in `empty/invalid/valid` |
| `last_check_at` | TIMESTAMPTZ | 是 | `NULL` | |
| `last_check_message` | VARCHAR(255) | 否 | `''` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(game_channel_id_ref, plugin_id_ref)`。索引：`(game_channel_id_ref)`。

### 3.5 `channel_package_plugin_overrides`（游戏维度业务表，每环境独立 schema，不带 env 列，对标 `channel_package_iap_overrides`）

| 列 | 类型 | 可空 | 默认 | 约束/说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `package_id_ref` | BIGINT | 否 | — | FK→channel_packages(id)（同 schema 普通外键） |
| `plugin_id_ref` | BIGINT | 否 | — | FK→platform.feature_plugins(id)（跨 schema 指向平台表） |
| `inherit_channel_config` | BOOLEAN | 否 | `TRUE` | **默认与渠道用同一套插件及配置** |
| `enabled` | BOOLEAN | 否 | `FALSE` | |
| `config_json` | JSONB | 否 | `{}` | 仅存与渠道的差异 |
| `config_status` | VARCHAR(16) | 否 | `'empty'` | CHECK in `empty/invalid/valid` |
| `last_check_at` | TIMESTAMPTZ | 是 | `NULL` | |
| `last_check_message` | VARCHAR(255) | 否 | `''` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(package_id_ref, plugin_id_ref)`。索引：`(package_id_ref)`。

---

## 4. 枚举与默认值

| 项 | 取值 / 默认 |
| --- | --- |
| `region` | `domestic`/`overseas`（无默认，建插件时指定） |
| `required` | 默认 `FALSE` |
| `selectable` | 默认 `TRUE` |
| `default_enabled` / `locked` | 默认 `FALSE` |
| `inherit_channel_config` | 默认 `TRUE` |
| `enabled`（实例） | 默认 `FALSE`（未勾选） |
| `config_status` | 默认 `empty`；勾选缺必填 ⇒ `invalid` |
| `config_json` | 默认 `{}` |
| 运行环境 | 业务表不带 env 列；写操作落当前运行环境对应 schema（由 `search_path` 决定，前端不可指定/跨 schema 写） |

### 运行态标识（派生，只读）
- `IncludedInRuntimeConfig = !channelHidden && compatible && enabled && config_status=='valid'`（按 scope 过滤后下发）。
- `IncludedInSnapshot = IncludedInRuntimeConfig`；`IncludedInSync = !channelHidden && compatible && enabled && config_status=='valid'`（与运行态/快照口径一致：无效插件不参与同步，落实 `00 §9` 红线）。
- 必接插件 `enabled=false` 或 `config_status!=valid` ⇒ 渠道实例标异常并提示补齐。

---

## 5. 业务规则

### 5.1 国内/海外可见性
- 与 `12-channel §5.1` 同源：`market=CN ⇒ plugin.region=domestic`；`market!=CN ⇒ overseas`。前端按 market 过滤候选插件，服务端二次校验，不兼容 ⇒ `MARKET_CHANNEL_INCOMPATIBLE`（复用错误码语义）。

### 5.2 必接 / 可勾选
- `required=true`：该渠道下此插件必须接入；前端引导补齐，未达 `valid` 时所属渠道实例异常。
- `selectable=false`：前端置为强制选中、不可取消（典型为必接项）。
- `locked=true`：游戏侧不可改勾选状态（由平台策略锁定）。

### 5.3 勾选必填校验（异常态）
- `enabled=true` 且模板必填字段（含 secret/file）缺失 ⇒ `config_status=invalid`，`last_check_message` 提示缺哪类字段（遵循 `00 §3.4`）。
- `enabled=false`：`config_status` 可为 `empty`（未配置）。

### 5.4 渠道级 vs 渠道包级
- 默认在渠道实例（GameMarketChannel）上配置插件。
- 渠道包 `inherit_channel_config=true`：沿用渠道实例的插件集合与配置；`false`：使用包级覆盖（仍受模板校验）。

### 5.5 加渠道后引导加插件
- 游戏新建/进入渠道实例后，前端引导补齐该渠道下所有 `required` 插件（流程见 `02-operation-flow.md` 第 4 步与 §8）。

---

## 6. 后端 API

> 统一前缀 `/api/admin`，遵循 `00 §7` 包络与错误码。写操作挂权限码 `plugin.write`，读 `plugin.read`。平台侧主数据/模板/`channel_feature_plugins` 走 system 基础数据后台。

- **GET `/api/admin/game-channels/{gameChannelId}/plugins`** —— 列出该渠道实例可接插件 + 必接标记 + 当前配置态。权限 `plugin.read`。
- **POST `/api/admin/game-channels/{gameChannelId}/plugins`** —— 勾选并配置某插件（`{ pluginId, enabled, config }`）。权限 `plugin.write`。
- **PATCH `/api/admin/game-channel-plugins/{id}`** —— 改配置/启停。权限 `plugin.write`。
- **GET/POST `/api/admin/channel-packages/{packageId}/plugins`** —— 渠道包级覆盖（继承/自定义）。权限 `plugin.read`/`plugin.write`。

响应示例（列表，每行一个可接插件 + 实例态）：
```json
{ "data": { "items": [
  { "pluginId": "realname", "pluginName": "实名认证", "region": "domestic",
    "required": true, "selectable": false, "enabled": true, "configStatus": "valid",
    "includedInRuntimeConfig": true },
  { "pluginId": "push", "pluginName": "推送", "region": "overseas",
    "required": false, "selectable": true, "enabled": false, "configStatus": "empty",
    "includedInRuntimeConfig": false }
] } }
```

---

## 7. 应用服务与 command/query

- `command/configure_channel_plugin.go`：勾选/配置插件 → 兼容性校验 → 模板校验 → `ResolvePluginConfigStatus` → 落库 + 审计 `plugin.configure`。
- `command/override_package_plugin.go`：渠道包级覆盖（继承/自定义）。
- `query/list_channel_plugins.go`：列出渠道实例可接插件 + 当前实例态 + 必接缺口。
- 领域：`domain/plugin/compatibility.go`、`domain/plugin/plugin_config.go`（聚合、状态推导）。
- 仓储：`FeaturePluginRepository`（主数据/模板/策略读）、`GameChannelPluginRepository`（实例 CRUD，按 env）、`ChannelPackagePluginRepository`。

---

## 8. 前端信息架构

- 入口：游戏详情 → 渠道实例详情内新增「功能插件」区（与「渠道登录」「渠道 IAP」并列）。
- 列表：可接插件（含必接标记/国内海外/可勾选）+ 勾选态 + `config_status` + 是否进入最终配置。
- 表单：模板驱动渲染器消费 `form_schema_json`（含 `scope` 标记，server 字段可提示"不下发客户端"）。
- 引导：渠道实例创建/保存后弹出「未配置必接插件」清单引导补齐；必接项不可取消勾选。
- 渠道包：提供「继承渠道插件 / 自定义覆盖」开关。
- 权限：无 `plugin.write` 时操作置灰。

---

## 9. 与公共能力的关系
- 模板四件套：插件参数模板含 `scope`（`00 §4`、§4.1.1）。
- 密文/文件：插件 secret/file 字段经 `00 §6` 加密与脱敏；复制/继承时按 `00 §3.4` 规则处理。
- 审计：`plugin.configure` / `plugin.enable` / `plugin.disable` 等写操作写 `audit_logs`。
- env：实例/覆盖的写操作落当前运行环境对应 schema（业务表不带 env 列，环境由 schema 决定）；同步随渠道相关 section 流转（`21-sync`）。
- 快照：有效插件配置按 scope 过滤后进客户端最终配置（`20-snapshot §5.1.1`）。

---

## 10. 测试要点
- 可见性：`CN+overseas 插件`、`JP+domestic 插件` 必须拒绝。
- 必接：`required=true` 未配置 `valid` ⇒ 渠道实例异常、引导补齐；`selectable=false` 不可取消。
- 勾选必填：`enabled=true` 缺必填 ⇒ `invalid` 且 message 提示。
- 渠道包继承：`inherit_channel_config=true` 沿用渠道插件及配置；`false` 用覆盖。
- scope 过滤：`server` 字段**不下发到客户端配置快照**（`client/both` 才下发），但 **DB 中 `config_json` 仍存**该字段值（仅快照生成时按 scope 过滤，见 `00 §4.1.1`、`20-snapshot §5.1.1`）。
- 多插件：同一渠道实例挂多个插件，唯一键 `(game_channel, plugin)` 在每环境 schema 内生效。

---

## 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env（schema 隔离）：写落当前环境 schema、不允许跨 schema 写 / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/feature-plugin.yaml`；前端 e2e：`tests/frontend/e2e/channels.spec.ts`（功能插件页签）。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET `/api/admin/game-channels/{gameChannelId}/plugins` | ✓ | ✓ | ✓ | — | — | ✓ | — | ✓ | — | — | 必接(required)/可勾选(selectable) 标记、国内/海外 region 过滤、config_status、secret 脱敏(S8) |
| POST `/api/admin/game-channels/{gameChannelId}/plugins` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | 参数模板四件套含 scope、region 兼容校验、config_status 推导、引导补齐、secret 脱敏(S8) |
| PATCH `/api/admin/game-channel-plugins/{id}` | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ | ✓ | — | ✓ | 启停 enabled、config_status 重算、scope 字段、secret 脱敏(S8) |
| GET `/api/admin/channel-packages/{packageId}/plugins` | ✓ | ✓ | ✓ | — | — | ✓ | — | ✓ | — | — | 渠道级+包级 override（inherit_channel_config）、region、config_status、secret 脱敏(S8) |
| POST `/api/admin/channel-packages/{packageId}/plugins` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | 包级 override 仅存差异、inherit_channel_config、参数模板四件套含 scope、引导补齐、secret 脱敏(S8) |

前端：`channels.spec.ts`（功能插件页签：必接引导补齐 / 国内海外 region 过滤 / 勾选必填异常态 config_status / 渠道级+包级 override 继承覆盖 / server scope 字段不下发提示）/ vitest 模板渲染器组件（`form_schema_json` 含 scope 标记渲染、secret 脱敏展示、config_status 徽标）。

---

## 11. 未决问题与显式假设
- 假设功能插件代码 domain 为 `internal/domain/plugin`（待落地）。
- 假设插件 `region` 兼容性与渠道完全同源；若未来插件有跨区共用需求再单独扩展。
- 假设必接（`required`）在 `channel_feature_plugins` 平台级设定；是否允许游戏级临时豁免必接，待定（当前不允许）。
- 渠道包覆盖的 `config_json` 差异结构与渠道 IAP override 风格一致（仅存差异），具体字段由模板定义。
