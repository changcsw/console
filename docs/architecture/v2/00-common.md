---
id: common
code: "00"
title: 公共部分（跨模块契约）
status: target
code_paths:
  - services/admin-api/internal/domain/common
  - apps/admin-web/src/stores/dictionary.ts
depends_on: []
impacts: [auth, game, channel, account-auth, channel-login, feature-plugin, product, cashier-template, game-cashier, payment, snapshot, sync, audit, dashboard, testing]
children: []
---

# 00 · 公共部分（跨模块契约）

> 本文件是整套 v2 架构文档的**公共契约**。所有模块文档（`modules/*/README.md`）默认遵循本文约定，不再重复说明。
> 若模块文档与本文冲突，以本文为准；模块文档只允许在本文基础上**追加**模块私有约定。

本文整合自：`docs/architecture/`（英文）、`docs/architecture/zh-CN/`（中文）、`docs/superpowers/specs/2026-06-16-market-channel-sync-design.md`、`docs/superpowers/plans/2026-06-16-market-channel-sync-implementation.md` 的并集，并固化本轮重新讨论确认的 7 项关键决策。

---

## 1. 本轮固化的关键决策（D1–D7）

| 编号 | 决策 | 结论 |
| --- | --- | --- |
| D1 | 多环境数据模型 | **单库 + 每环境独立 schema**。三个环境各有一个 PostgreSQL schema（`develop` / `sandbox` / `production`），其中放置**同名、同结构**的"游戏维度业务表"；同一逻辑对象在不同 env 下是**不同 schema 下同名表的不同物理行**。平台级共享数据放在共享 schema `platform`。运行时由 `search_path = <当前env>, platform` 路由。业务表**不带 `env` 列**。`sandbox -> production` 同步在**同库内跨 schema**（读 `sandbox.*` 与 `production.*`）做 diff / upsert。 |
| D2 | 渠道实例的 market 维度 | `game_channels` 增加 `market_code`，唯一键为 `(game_id_ref, market_code, channel_id_ref)`，**该表本身即 `GameMarketChannel` 落地表**，不再分两层。 |
| D3 | 渠道国内/非国内属性 | `channels` 增加 `region`（`domestic` / `overseas`），并在 seed 中固化。 |
| D4 | 配置快照粒度 | 快照 **per-game 一份**，`config_json` 内部按 `market` 分区，每个 market 存放"已按合并规则解析后的最终配置"。 |
| D5 | 鉴权 | JWT（access + refresh）+ RBAC，权限码格式 `resource.action`；支持密码登录与飞书回调；本地 dev 允许 mock。 |
| D6 | 同步基线一致性 | `sync/execute` 必须携带 `sync/preview` 返回的 `baseline_token`（含 `target_hash_before`）；执行前服务端复核目标 schema hash，不一致则拒绝并要求重新预览。 |
| D7 | 后端技术栈 | `chi`（路由）+ `pgx`（数据库）+ `golang-migrate`（迁移）。详见 `01-structure.md`。 |

---

## 2. 多环境模型（schema-per-env）

### 2.1 环境枚举与 schema 映射

```text
Environment = develop | sandbox | production
```

- `develop`：开发自测环境，可随意改。
- `sandbox`：预发布/联调环境，是同步的**源**。
- `production`：正式环境，是同步的**目标**，禁止盲写。

每个环境对应一个**同名 PostgreSQL schema**，"游戏维度业务表"在每个环境 schema 下各有一份同名、同结构的物理表：

```text
develop.games      sandbox.games      production.games
develop.game_channels   sandbox.game_channels   production.game_channels
...（其余业务表同理，三套同名结构）
```

平台级共享数据集中在一个共享 schema **`platform`**（全环境只有一份）。

默认值与运行时约定：

- 服务启动时由配置项 `APP_ENV` 指定当前运行环境；缺省 `develop`。
- 每个数据库连接按当前运行环境设置 `search_path = <当前env>, platform`：业务表自动落到当前环境 schema，平台表落到 `platform`。仓储层 SQL **不写 schema 前缀、不带 `env` 谓词**，环境由连接的 `search_path` 决定。
- **连接最小权限化（加固 `search_path` 风险，强约束）**：`search_path` 只是路由、不是安全边界。必须配套「**每环境独立连接池**（建连时一次性钉死 `search_path`，不在请求层逐次 `SET`）+ **每环境最小权限 DB 角色**（`app_<env>` 对其它环境 schema 连 `USAGE` 都不授）」，使误连/误查/误写在权限层直接 `permission denied`；禁止用默认 `public` / 超级用户跑业务请求。落地细节与授权脚本见 `01` §4.4。
- 写业务数据的请求一律落**当前运行环境对应的 schema**，前端不能指定/跨 schema 写（避免越权写 production）。
- 仅 `sync` 域允许**同时访问两个环境 schema**（显式声明 `source_env=sandbox`、`target_env=production`，用 schema 限定名跨 schema 读写，且用专用最小权限角色 `app_sync`，见 `01` §4.4）。

### 2.2 表的归属规则（D1）

- **"游戏维度业务表"** 放在**每个环境 schema**（`develop` / `sandbox` / `production`）下，各一份同名同结构表，**不带 `env` 列**——"行属于哪个 env"由它所在的 schema 决定。
- 受影响的表（在各模块文档中逐表标注，均为每环境一份）：`games`、`game_markets`、`game_legal_links`、`game_channels`、`channel_packages`、`game_account_auth_configs`、`game_channel_login_configs`、`game_channel_plugin_configs`、`channel_package_plugin_overrides`、`products`、`channel_products`、`game_channel_iap_configs`、`channel_package_iap_overrides`、`game_cashier_profiles`、`game_cashier_price_overrides`、`payment_routes`、`game_config_snapshots`。
- **"平台级基础数据/字典/模板表"** 放在共享 schema **`platform`**（全环境共享一套）：`channels`、`channel_policies`、`account_auth_types`、`channel_account_auth_types`、`account_auth_templates`、`channel_login_templates`、`channel_iap_templates`、`feature_plugins`、`feature_plugin_templates`、`channel_feature_plugins`、`cashier_providers`、`cashier_provider_templates`、`pay_ways`、`currency_specs`、`billing_subjects`、`cashier_merchant_accounts`、`cashier_price_templates`、`cashier_price_template_versions`、`cashier_price_rows`、`cashier_fx_sync_runs`、`admin_*`。
- **跨环境协调表也在 `platform`**（天然需要同时引用两个环境）：
  - `sync_jobs` / `sync_job_items` / `sync_consumed_tokens`：不带 `env` 列，其环境维度由 `source_env` / `target_env` 字段显式表达（值即 schema 名，见 `sync` §3）。
  - **特例 `audit_logs`：放在 `platform`，保留 `env VARCHAR(16) NOT NULL` 作为纯过滤列**。审计记录的是"事件"（不是跨环境同一对象的物理行），`env` 仅标记该操作**发生在哪个运行环境**，便于跨环境统一查询审计；它不前置唯一键、不参与同步 diff（`production` 的审计在 production 本地产生，不由 sandbox 同步而来）。
- **唯一约束不再前置 `env`**：每个环境 schema 内自成体系，唯一性天然按 schema 隔离。例：`games` 直接 `UNIQUE(game_id)`（不再 `UNIQUE(env, game_id)`）。
- **外键规则**：
  - 业务表→业务表（同一环境 schema 内）：用普通外键即可，被引用行与引用行必然同 schema（= 同 env），无需任何 env 一致性校验或复合外键。
  - 业务表→平台表（跨 schema）：用指向 `platform.<表>` 的普通外键（PostgreSQL 支持跨 schema 外键），如 `game_channels.channel_id_ref REFERENCES platform.channels(id)`。

> 备注：`cashier_merchant_accounts.secret_ciphertext` 等支付密钥属平台级基础数据，全环境共享；如果未来需要分环境密钥，再单独扩展，本期不做。

---

## 3. 全局枚举与默认值清单

> 下表是**前后端唯一事实来源**。后端 `internal/domain/common` 与前端 `dictionary` store 必须与此完全一致。

### 3.1 核心业务枚举

| 枚举 | 取值 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `Environment` | `develop` / `sandbox` / `production` | `develop` | 运行/数据环境 |
| `Market` | `GLOBAL` / `JP` / `KR` / `SEA` / `HMT` / `CN` | `GLOBAL` | 发行大区，非国家 |
| `LoginMode` | `channel_only` / `account_system` | `account_system` | 渠道登录策略 |
| `PaymentMode` | `channel_only` / `hybrid` / `cashier_only` | `hybrid` | 渠道支付策略 |
| `ConfigStatus` | `empty` / `invalid` / `valid` | `empty` | 模板驱动配置状态 |
| `OverrideMode` | `default` / `override` | `default` | 字段级覆盖模式 |
| `FXSyncMode` | `manual_confirm` / `auto_apply` | `manual_confirm` | 汇率同步模式 |
| `FXSyncSchedule` | `monthly` / `quarterly` | `monthly` | 汇率同步周期 |
| `VersionStatus` | `draft` / `published` / `archived` | `draft` | 模板版本生命周期 |
| `ChannelRegion` | `domestic` / `overseas` | （seed 固定） | 渠道国内/非国内（D3） |
| `ChannelType` | `store` / `oem` / `web` / `direct` / `mini_game` | 无默认 | 渠道类型 |
| `PayWayType` | `card` / `wallet` / `platform` / `local` | 无默认 | 支付方式类型 |
| `ProviderKind` | `aggregator` / `gateway` / `wallet_direct` | 无默认 | 支付提供商类型 |
| `RoundingMode` | `half_up` / `floor` / `ceil` / `truncate` | `half_up` | 金额舍入 |
| `LegalScopeType` | `default` / `market` / `locale` | `default` | 法务链接作用域 |
| `IdentityType` | `password` / `feishu` | 无默认 | 管理员身份类型 |
| `SyncSection` | `game` / `markets` / `legal` / `channels` / `packages` / `products` / `cashier` / `payments` / `config` | 无默认 | 同步最小单位 |
| `SyncOp` | `add` / `update` / `delete` | 无默认 | 差异操作类型 |
| `SyncJobStatus` | `previewed` / `succeeded` / `failed` | 无默认 | 同步任务状态 |
| `FXRunStatus` | `pending_review` / `approved` / `applied` / `ignored` / `failed` | `pending_review` | 汇率同步运行状态 |
| `SnapshotStatus` | `draft` / `published` | `draft` | 配置快照状态 |
| `GameStatus` | `draft` / `active` / `disabled` | `draft` | 游戏状态 |
| `AdminUserStatus` | `active` / `disabled` | `active` | 管理员状态 |

### 3.2 Market 语义补充

- `GLOBAL`：默认兜底海外市场；**不匹配 `CN`**。
- `CN`：仅中国大陆，仅允许 `domestic` 渠道。
- `JP / KR / SEA / HMT`：具体海外大区，仅允许 `overseas` 渠道。
- 具体海外 market 与 `GLOBAL` 同时存在时：**具体 market 整体覆盖 `GLOBAL`**（实例级覆盖，非字段级）。
- 渠道可见性：`market=CN` ⇒ 只显示 `domestic`；`market!=CN` ⇒ 只显示 `overseas`。

### 3.3 状态机：模板版本生命周期（VersionStatus）

允许的流转**只有**：

```text
draft --publish--> published
published --(发布新版本时自动)--> archived
```

规则：

- 同一模板任一时刻**最多一个** `published`。
- `published` 只读，不允许原地编辑；需要改动 ⇒ `copy-to-draft` 生成新 `draft` 再发布。
- 复制来源允许：空白 / 当前 `published` / 历史 `archived`；复制产物状态恒为 `draft`，复制后与来源不再联动。
- 发布新 `published` 时，旧 `published` **自动转 `archived`**。
- 禁止：`archived -> published` 直接回退；跳过 `draft` 直接生成正式版本。

### 3.4 状态机：模板驱动配置状态（ConfigStatus）

| 状态 | 含义 | 进入条件 |
| --- | --- | --- |
| `empty` | 尚未建立有效配置 | 新建实例且未填任何字段 |
| `invalid` | 已有结构但缺必填/敏感/文件字段或校验未过 | 缺字段；**复制创建后 secret/file 被清空** |
| `valid` | 完整且通过校验 | 全部必填（含 secret/file）补齐且校验通过 |

强约束：通过复制创建、且 `secret/file` 被清空的实例，**必须显示 `invalid`，不得显示 `empty`**，且 `last_check_message` 必须提示"缺少必填敏感字段或文件字段"。

---

## 4. 模板四件套（template-driven forms）

所有"模板表"统一含以下 4 个 JSONB 字段，语义全局一致：

| 字段 | 类型 | 默认值 | 含义 |
| --- | --- | --- | --- |
| `form_schema_json` | JSONB | `[]` | 前端渲染哪些字段、用什么组件、label、必填、排序 |
| `secret_fields_json` | JSONB | `[]` | 哪些字段是密文（加密存储 + 响应脱敏） |
| `file_fields_json` | JSONB | `[]` | 哪些字段是文件上传（含文件类型/大小限制） |
| `validation_rules_json` | JSONB | `{}` | 前后端共同遵循的校验规则 |

涉及模板的表：`account_auth_templates`、`channel_login_templates`、`channel_iap_templates`、`cashier_provider_templates`、`feature_plugin_templates`。

### 4.1 form_schema_json 单字段结构（约定）

```json
{
  "key": "clientId",
  "label": "Client ID",
  "component": "input",
  "required": true,
  "placeholder": "",
  "default": "",
  "order": 10,
  "group": "basic",
  "scope": "both"
}
```

- `component` 取值：`input` / `password` / `textarea` / `number` / `select` / `switch` / `file` / `json`。
- `select` 额外带 `options: [{label,value}]`。
- `default` 缺省 `""`（字符串型）/ `null`（其它）。
- `scope` 取值：`client` / `server` / `both`，**默认 `both`**（缺省按 `both` 解释，向后兼容）。标记该参数最终用于**客户端** / **仅服务端** / **两端共用**。

### 4.1.1 参数作用域 scope（全局约定）

所有使用模板四件套的模块（`account_auth_templates` / `channel_login_templates` / `channel_iap_templates` / `cashier_provider_templates` / `feature_plugin_templates`）的 `form_schema_json` 字段都必须带 `scope`：

| scope | 含义 | 是否进客户端最终配置 |
| --- | --- | --- |
| `client` | 仅客户端使用 | 是 |
| `both` | 客户端与服务端共用（默认） | 是 |
| `server` | 仅服务端使用（如服务端密钥、回调校验） | **否** |

**配置快照/客户端最终配置生成（见 `modules/20-snapshot`）只纳入 `scope ∈ {client, both}` 的参数；`server` 参数不下发到客户端配置。** 密文脱敏（§6）与 scope 过滤是两件独立的事：`server` 参数即便是密文也不下发，`client/both` 的密文参数仍按脱敏规则处理。

### 4.2 secret_fields_json / file_fields_json 结构

```json
// secret_fields_json
["clientSecret", "appSecret"]

// file_fields_json
[{"key": "keystore", "accept": [".keystore", ".jks"], "maxSizeKB": 2048}]
```

### 4.3 validation_rules_json 结构

```json
{
  "clientId":   { "minLen": 1, "maxLen": 128, "pattern": "" },
  "redirectUri":{ "format": "url" }
}
```

支持的规则键：`required` / `minLen` / `maxLen` / `min` / `max` / `pattern` / `format`（`url|email|host`）/ `enum`。

### 4.4 模板版本维护（基础数据）

- 模板本身由"基础数据/模板管理后台"维护。
- 同一逻辑渠道在不同 market 下**可复用同一套模板定义**，但**实际配置实例必须各自独立**（不共享 secret/file/状态）。

#### 4.4.1 版本状态机适用边界（重要）

模板版本的管理分为两类，**`§3.3 VersionStatus`（draft/published/archived）三态机只适用于 cashier 价格模板版本**：

- **简单模板表**（`account_auth_templates`、`channel_login_templates`、`channel_iap_templates`、`cashier_provider_templates`、`feature_plugin_templates`）：
  - **不走** §3.3 三态机，**没有 `status` 列**；只有 `enabled` + `template_version` 两个版本控制字段。
  - 运行时一律取 `enabled=true` 的**最新 `template_version`**（不存在"取 published 版本"的概念）。
  - 停用某版本通过 `enabled=false` 实现；升级通过写入更高的 `template_version` 实现。
- **cashier 价格模板版本**（`cashier_price_template_versions`，有独立的版本表 + `status` 列）：
  - **唯一**适用 §3.3 `VersionStatus`（draft/published/archived）三态机的对象。
  - 运行时取该模板当前 `published` 版本（同一模板任一时刻最多一个 `published`）。

---

## 5. 金额与币种归一化（currency）

### 5.1 currency_specs（平台级基础数据，全 env 共享）

| 字段 | 类型 | 默认值 | 约束 |
| --- | --- | --- | --- |
| `currency_code` | VARCHAR(8) | — | UNIQUE |
| `currency_name` | VARCHAR(64) | — | |
| `decimal_places` | INT | — | `0 <= x <= 6` |
| `min_amount_minor` | BIGINT | `1` | |
| `rounding_mode` | VARCHAR(16) | — | `half_up/floor/ceil/truncate` |
| `enabled` | BOOLEAN | `TRUE` | |

seed 固定值：

```text
USD  US Dollar          decimal=2 min=1 rounding=half_up
JPY  Japanese Yen       decimal=0 min=1 rounding=half_up
KRW  Korean Won         decimal=0 min=1 rounding=half_up
TWD  New Taiwan Dollar  decimal=0 min=1 rounding=half_up
EUR  Euro               decimal=2 min=1 rounding=half_up
```

### 5.2 金额写入统一流程（不可绕过）

任何涉及金额的写入路径必须按序执行：

1. 读取目标币种的 `currency_specs`；缺失 ⇒ 拒绝（错误码 `CURRENCY_NOT_SUPPORTED`）。
2. 按 `decimal_places` 校验小数精度。
3. 按 `min_amount_minor` 校验下限。
4. 按 `rounding_mode` 归一化。
5. 统一存为整数最小单位字段 `*_amount_minor`（如 `base_amount_minor`、`pre_tax_amount_minor`）。

涉及金额的表：`products`、`cashier_price_rows`、`game_cashier_price_overrides`，以及 IAP/收银台相关写入路径。

---

## 6. 密文与文件

### 6.1 密文（secret）

- `secret_fields_json` 标记的字段，落库前必须加密，存到对应 `*_ciphertext` 或 `config_json` 内的密文位（实现见 `01` 的 `infra/crypto`）。
- 任何响应中密文字段一律**脱敏**（返回 `"masked"` 或 `"******"`，绝不回明文）。
- 明文密钥**禁止落库**。
- 同步预览中密文字段必须 `masked=true`。

### 6.2 文件（file）

- `file_fields_json` 标记的字段走统一上传能力（见 `01` 的 `infra/file`），存储后保存"文件引用（storage key / hash）"，不直接存二进制内容到业务表。
- 复制创建实例时 file 字段必须清空。

---

## 7. 统一 API 约定

### 7.1 通用规则

- 统一前缀：`/api/admin`。
- 鉴权：除登录类接口外，所有接口要求 `Authorization: Bearer <accessToken>`。
- 内容类型：`application/json; charset=utf-8`。
- 时间：ISO-8601 UTC（如 `2026-06-15T10:00:00Z`）。
- 命名：请求/响应 JSON 字段统一 **camelCase**；数据库列统一 **snake_case**；URL path 段 **kebab/camel** 按现有风格（`game-channels`、`gameId`）。
- 写操作默认作用于**当前运行环境对应的 schema**（见 §2.1），前端不能指定/跨 schema 写。

### 7.2 统一响应包络

成功（单对象 / 列表）：

```json
{ "data": { /* ... */ } }
{ "data": { "items": [ /* ... */ ], "page": 1, "pageSize": 20, "total": 135 } }
```

错误：

```json
{ "error": { "code": "VALIDATION_FAILED", "message": "alias already exists", "details": [] } }
```

> 说明：现有 scaffold 直接返回裸对象/`{"error": "..."}`，v2 统一改为上述包络。各模块示例均按此包络书写。

### 7.3 分页约定

- query 参数：`page`（默认 `1`，最小 `1`）、`pageSize`（默认 `20`，最大 `100`）。
- 排序：`sort`（如 `-updatedAt` 表示降序），缺省按 `updatedAt` 降序。

### 7.4 统一错误码（节选，模块可追加）

| code | HTTP | 含义 |
| --- | --- | --- |
| `UNAUTHENTICATED` | 401 | 未登录/令牌失效 |
| `FORBIDDEN` | 403 | 无权限 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `VALIDATION_FAILED` | 400 | 入参校验失败 |
| `CONFLICT` | 409 | 唯一性/状态冲突 |
| `CURRENCY_NOT_SUPPORTED` | 400 | 币种不在 currency_specs |
| `MARKET_CHANNEL_INCOMPATIBLE` | 400 | 渠道与 market 不兼容 |
| `UNKNOWN_SECTION` | 400 | 同步 section 非法 |
| `SYNC_BASELINE_MISMATCH` | 409 | 同步基线 hash 不一致 |
| `SYNC_TOKEN_CONSUMED` | 409 | 同步 baseline_token 已被消费（重复 execute），见 `sync` §5.4 |
| `VERSION_STATE_INVALID` | 409 | 版本状态流转非法 |
| `ROUTE_CONFLICT` | 409 | 支付路由优先级/选择器冲突 |
| `INTERNAL` | 500 | 服务端内部错误 |

### 7.5 鉴权与权限码（D5）

- 令牌：`accessToken`（短期，默认 30 分钟）、`refreshToken`（默认 14 天）。
- 权限码格式：`resource.action`，如 `game.read` / `game.write` / `channel.write` / `cashier.publish` / `sync.execute` / `audit.read`。
- 每个**写/危险操作**都必须挂权限码；具体清单在各模块的 API 小节标注。
- 环境上下文：响应头或 `/api/admin/me` 返回当前 `environment`，前端常驻展示。

---

## 8. 审计（audit_logs）

- 所有**有意义的写操作**（创建/更新/删除/发布/隐藏/同步执行/审核）必须写 `audit_logs`。
- 字段：`actor_id`、`action`、`resource_type`、`resource_id`、`env`、`detail_json`、`created_at`。
- `env` 口径（见 §2.2 特例）：`audit_logs` 位于共享 `platform` schema，`env` 仅作过滤列，记录该操作**发生时的运行环境**；它不是游戏维度业务表，`env` 不前置唯一键、不参与同步 diff。`production` 的审计在 production 本地产生，不由 sandbox 同步而来。
- `action` 命名与权限码同源（`game.create` / `sync.execute` / `cashier.publish` …）。
- `detail_json` 记录关键 before/after（密文脱敏）。
- 审计查询页见 `modules/22-audit/README.md`。

---

## 9. 不可触碰的红线（全局）

- 不把"后台管理员登录"与"玩家登录配置"混在一起。
- 不把"渠道 IAP 配置"与"收银台支付路由"混在一起。
- 任何金额写入不绕过 `currency_specs`。
- `sandbox -> production` 不允许无 preview 直接写；execute 必须复核基线（D6）。
- 不存明文密钥；响应不回明文密钥。
- 不用默认 `public` / 超级用户 / 库 owner 跑业务请求；每环境用最小权限角色 + 建连时固定 `search_path`，跨环境只允许 `sync` 专用角色 + 全限定名（`01` §4.4）。
- 被隐藏 / 不兼容 / 无效的渠道实例：不进快照、不参与同步、不进客户端最终配置、不进默认列表。
- `production` 视图里不允许出现可执行的 `Sync to Production`。

---

## 10. 命名与默认值兜底约定

- 业务表的写入一律落当前运行环境对应的 schema（由 `search_path` 决定，业务行不再带 `env` 列）；`audit_logs.env` 取当前运行环境。
- 布尔默认：`enabled=TRUE`、`hidden=FALSE`、各 `*_locked=FALSE`、`is_default=FALSE`（除非模块另有说明）。
- 字符串默认：URL/备注类默认 `''`；JSONB 配置类默认 `{}`，列表类默认 `[]`。
- `default_locale` 默认 `en-US`。
- `priority` 默认 `100`（数值越小优先级越高）。
- 时间戳 `created_at/updated_at` 默认 `NOW()`。
