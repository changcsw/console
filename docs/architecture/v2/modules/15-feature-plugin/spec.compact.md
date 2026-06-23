---
id: feature-plugin
code: "15"
title: 功能插件（Feature Plugin）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [channel, game, common]
code_paths:
  - services/admin-api/internal/domain/plugin
  - apps/admin-web/src/views/channels
---

# 15 · 功能插件 — Compact Spec

> 代码生成用精简规格。完整背景/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（模板四件套 §4、scope §4.1.1、ConfigStatus §3.4、密文 §6、审计 §8、红线 §9、API 包络 §7）与 `01-structure.md`（env/分层）。强依赖 `12-channel`（渠道实例/渠道包为挂载点）。

## 边界
- 与「渠道」同级：游戏在某渠道实例（GameMarketChannel）接入若干功能插件（实名/客服/推送/防沉迷等），每插件由参数模板驱动。
- 复用渠道 + IAP 模式：平台级主数据 + 模板四件套 + 渠道级配置实例 + 渠道包覆盖。
- 红线（来自 00 §9）：
  - `scope=server` 字段不下发客户端最终配置。
  - 隐藏/不兼容/`config_status!=valid` 的插件实例：不进默认列表、不进快照、不参与同步、不进客户端最终配置。
  - 必接（`required`）插件未配 `valid` ⇒ 所属渠道实例运行态异常，不进快照/同步。

## 数据模型
平台级共享表（schema `platform`）3 张：`feature_plugins`、`feature_plugin_templates`、`channel_feature_plugins`。游戏维度业务表 2 张（每环境 schema 各一份同结构，**不带 env 列**，env 由 search_path 决定）：`game_channel_plugin_configs`、`channel_package_plugin_overrides`。

公共列约定（各表均含）：`id BIGSERIAL PK`、`enabled BOOLEAN`、`created_at/updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`；多数含 `sort INT NOT NULL DEFAULT 0`。下文仅列业务字段与关键约束。

### feature_plugins（平台级，对标 channels）
| 列 | 类型 | 约束 |
| --- | --- | --- |
| plugin_id | VARCHAR(64) | UNIQUE, NOT NULL（业务键） |
| plugin_name | VARCHAR(64) | NOT NULL |
| region | VARCHAR(16) | NOT NULL CHECK IN('domestic','overseas') |
| enabled | BOOLEAN | NOT NULL DEFAULT TRUE |
| sort | INT | NOT NULL DEFAULT 0 |

UNIQUE(plugin_id)。索引建议 `(region)`、`(enabled, sort)`。

### feature_plugin_templates（平台级，模板四件套，对标 account_auth_templates）
| 列 | 类型 | 约束 |
| --- | --- | --- |
| plugin_id_ref | BIGINT | NOT NULL FK→feature_plugins(id) |
| template_version | VARCHAR(32) | NOT NULL |
| form_schema_json | JSONB | NOT NULL DEFAULT '[]'（字段含 scope） |
| secret_fields_json | JSONB | NOT NULL DEFAULT '[]' |
| file_fields_json | JSONB | NOT NULL DEFAULT '[]' |
| validation_rules_json | JSONB | NOT NULL DEFAULT '{}' |
| enabled | BOOLEAN | NOT NULL DEFAULT TRUE |

UNIQUE(plugin_id_ref, template_version)。**简单模板表**（00 §4.4.1）：无 status 列、不走三态机；运行时取 `enabled=TRUE` 的最新 template_version。

### channel_feature_plugins（平台级，对标 channel_account_auth_types）
| 列 | 类型 | 约束 |
| --- | --- | --- |
| channel_id_ref | BIGINT | NOT NULL FK→channels(id) |
| plugin_id_ref | BIGINT | NOT NULL FK→feature_plugins(id) |
| required | BOOLEAN | NOT NULL DEFAULT FALSE（必接：未配 valid ⇒ 渠道实例异常） |
| selectable | BOOLEAN | NOT NULL DEFAULT TRUE（前端可勾选；必接项一般 FALSE） |
| default_enabled | BOOLEAN | NOT NULL DEFAULT FALSE（新建实例默认勾选） |
| locked | BOOLEAN | NOT NULL DEFAULT FALSE（锁定后游戏侧不可改） |
| sort | INT | NOT NULL DEFAULT 0 |

UNIQUE(channel_id_ref, plugin_id_ref)。约束：plugin.region 须与渠道使用场景 market 兼容。

### game_channel_plugin_configs（游戏维度业务表 / 每环境 schema / 多插件，对标 game_channel_iap_configs）
| 列 | 类型 | 约束 |
| --- | --- | --- |
| game_channel_id_ref | BIGINT | NOT NULL FK→game_channels(id)（同 schema 普通 FK） |
| plugin_id_ref | BIGINT | NOT NULL FK→platform.feature_plugins(id)（跨 schema 指向平台表） |
| enabled | BOOLEAN | NOT NULL DEFAULT FALSE（是否勾选接入） |
| config_json | JSONB | NOT NULL DEFAULT '{}'（含密文位、scope 由模板定义） |
| config_status | VARCHAR(16) | NOT NULL DEFAULT 'empty' CHECK IN('empty','invalid','valid') |
| last_check_at | TIMESTAMPTZ | NULL |
| last_check_message | VARCHAR(255) | NOT NULL DEFAULT '' |

UNIQUE(game_channel_id_ref, plugin_id_ref)。索引 `(game_channel_id_ref)`。

### channel_package_plugin_overrides（游戏维度业务表 / 每环境 schema，对标 channel_package_iap_overrides）
| 列 | 类型 | 约束 |
| --- | --- | --- |
| package_id_ref | BIGINT | NOT NULL FK→channel_packages(id)（同 schema 普通 FK） |
| plugin_id_ref | BIGINT | NOT NULL FK→platform.feature_plugins(id)（跨 schema 指向平台表） |
| inherit_channel_config | BOOLEAN | NOT NULL DEFAULT TRUE（默认与渠道用同一套插件及配置） |
| enabled | BOOLEAN | NOT NULL DEFAULT FALSE |
| config_json | JSONB | NOT NULL DEFAULT '{}'（仅存与渠道的差异） |
| config_status | VARCHAR(16) | NOT NULL DEFAULT 'empty' CHECK IN('empty','invalid','valid') |
| last_check_at | TIMESTAMPTZ | NULL |
| last_check_message | VARCHAR(255) | NOT NULL DEFAULT '' |

UNIQUE(package_id_ref, plugin_id_ref)。索引 `(package_id_ref)`。

运行时连接 `search_path = <env>, platform`；业务表仓储 SQL 不写 schema 前缀、不带 env 谓词、不允许跨 schema 写。

## 枚举与默认
- `region` ∈ {domestic, overseas}（无默认，建插件时指定）。
- `config_status` ∈ {empty, invalid, valid}，默认 empty。
- required 默认 FALSE；selectable 默认 TRUE；default_enabled / locked 默认 FALSE；inherit_channel_config 默认 TRUE。
- enabled（实例）默认 FALSE（未勾选）；config_json 默认 `{}`；四件套 `[]`/`{}`；last_check_message 默认 `''`；sort 默认 0。

### 运行态标识（派生，只读）
- `IncludedInRuntimeConfig = !channelHidden && compatible && enabled && config_status=='valid'`（再按 scope 过滤下发）。
- `IncludedInSnapshot = IncludedInRuntimeConfig`；`IncludedInSync` 同口径（无效插件不参与同步，落实 00 §9）。
- 必接插件 `enabled=false` 或 `config_status!=valid` ⇒ 渠道实例标异常并提示补齐。

## 业务规则与状态机

### 国内/海外可见性（与 12-channel §5.1 同源，服务端强制）
- `market=CN ⇒ plugin.region=domestic`；`market!=CN ⇒ overseas`。
- 前端按 market 过滤候选插件，服务端二次校验，不兼容 ⇒ `MARKET_CHANNEL_INCOMPATIBLE`。
- 纯规则：`ValidatePluginRegionCompatibility(market, region) -> bool`（无 IO）。

### 必接 / 可勾选 / 锁定
- `required=true`：渠道下此插件必须接入；未达 valid 时所属渠道实例异常、前端引导补齐。
- `selectable=false`：前端强制选中、不可取消（典型为必接项）。
- `locked=true`：游戏侧不可改勾选状态。

### 勾选必填校验（异常态，遵循 00 §3.4）
- `enabled=true` 且模板必填字段（含 secret/file）缺失 ⇒ `config_status=invalid`，`last_check_message` 提示缺哪类字段。
- `enabled=false`：`config_status` 可为 `empty`（未配置）。不得「只勾选未填」静默为 empty。
- 纯领域规则：`ResolvePluginConfigStatus(enabled, template, config) -> (config_status, last_check_message)`（无 IO）。

### 渠道级 vs 渠道包级
- 默认在渠道实例（GameMarketChannel）上配置插件，一个实例可挂多个插件。
- 渠道包 `inherit_channel_config=true`：沿用渠道实例插件集合与配置；`false`：用包级覆盖（仅存差异，仍受模板校验）。

## 后端 API（前缀 /api/admin，包络见 00 §7；读 plugin.read / 写 plugin.write；平台主数据/模板/channel_feature_plugins 走 system 基础数据后台）

GET `/api/admin/game-channels/{gameChannelId}/plugins`（plugin.read，按当前 env）
→ items[]: { pluginId, pluginName, region, required, selectable, enabled, configStatus, includedInRuntimeConfig }（列出该渠道实例可接插件 + 必接标记 + 当前配置态；secret 脱敏）

POST `/api/admin/game-channels/{gameChannelId}/plugins`（plugin.write，审计 plugin.configure，按当前 env）
请求 DTO:
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| pluginId | string | 是 | — | 须属该渠道允许集合且 region 与 market 兼容 |
| enabled | bool | 否 | false | |
| config | object | 否 | {} | 按模板四件套校验 |
→ 兼容性校验 → 模板校验 → ResolvePluginConfigStatus → 落库 + 审计。返回计算后的 configStatus 与 lastCheckMessage。

PATCH `/api/admin/game-channel-plugins/{id}`（plugin.write，审计）—— 改配置/启停，config_status 重算。

GET / POST `/api/admin/channel-packages/{packageId}/plugins`（plugin.read / plugin.write）—— 渠道包级覆盖（继承/自定义，POST 仅存差异）。

错误码：`MARKET_CHANNEL_INCOMPATIBLE`（region 不兼容）、`VALIDATION_FAILED`、`CONFLICT`（唯一键冲突）。

## 应用服务 / 仓储（command/query）
```go
// command/configure_channel_plugin.go：勾选/配置 → 兼容性校验 → 模板校验 → ResolvePluginConfigStatus → 落库 + 审计 plugin.configure
// command/override_package_plugin.go：渠道包级覆盖（继承/自定义）
// query/list_channel_plugins.go：列渠道实例可接插件 + 实例态 + 必接缺口
// domain/plugin/compatibility.go、domain/plugin/plugin_config.go：聚合、状态推导（纯规则）
```
- 仓储：`FeaturePluginRepository`（主数据/模板/策略读）、`GameChannelPluginRepository`（实例 CRUD，按 env）、`ChannelPackagePluginRepository`。业务表仓储 SQL 不写 schema 前缀 / 不带 env 谓词。
- 审计事件：`plugin.configure` / `plugin.enable` / `plugin.disable` 写 audit_logs。

## 前端要点（游戏详情 → 渠道实例详情内「功能插件」区，与「渠道登录」「渠道 IAP」并列）
- 列出可接插件（必接标记/国内海外/可勾选）+ 勾选态 + config_status + 是否进入最终配置。
- 表单：模板驱动渲染器消费 form_schema_json（含 scope，server 字段提示"不下发客户端"）；secret 脱敏可重填、file 走统一上传。
- 引导：渠道实例创建/保存后弹「未配置必接插件」清单引导补齐；必接项不可取消勾选；locked 项禁用编辑。
- 渠道包：提供「继承渠道插件 / 自定义覆盖」开关。无 plugin.write 置灰。

## 与公共能力 / 下游
- 模板四件套(00 §4/§4.1.1)：插件参数模板含 scope。密文/文件(00 §6)：secret/file 字段加密落库/响应脱敏。审计(00 §8)：插件配置/启停写操作。
- 快照(20-snapshot §5.1.1)：有效插件配置按 scope 过滤后进客户端最终配置（server 字段 DB 仍存，仅快照按 scope 过滤）。
- 同步(21-sync)：插件配置随渠道相关 section 流转。

## 关键假设
- 插件 domain 代码为 `internal/domain/plugin`（待落地）。
- 插件 region 兼容性与渠道完全同源；跨区共用未来再扩展。
- required 在 channel_feature_plugins 平台级设定；当前不允许游戏级临时豁免必接。
- 渠道包覆盖 config_json 与渠道 IAP override 风格一致（仅存差异），具体字段由模板定义。
