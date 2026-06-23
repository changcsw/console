---
id: product
code: "16"
title: 商品与 IAP 映射（Product / Channel IAP）
status: target
code_paths:
  - services/admin-api/internal/domain/product
  - apps/admin-web/src/views/games
depends_on: [channel, game, common]
impacts: [game-cashier, payment, snapshot, sync, testing]
children: []
---

# 16 · 商品与 IAP 映射（Product / Channel IAP）

> 本文件遵循 `../../00-common.md` 与 `../../01-structure.md` 的全部公共契约（env 模型、currency 归一化、模板四件套、ConfigStatus、密文、统一 API 包络、错误码、审计）。本文只在公共契约之上追加**商品主数据 / 包级映射 / 渠道 IAP 配置**三块的模块私有约定。冲突以 `00` 为准。
>
> 对应执行计划：后端 `backend_agent_execution.md` 阶段 7（商品、IAP、渠道包覆盖），前端 `frontend_agent_execution.md` 阶段 6（商品与 IAP）；DDL 指南见 `postgresql_ddl_guide.md` 第 6、7 节。
>
> **本模块最高风险点（务必牢记）**：`product_id`（IAP 商品 ID，商店内购标识）与 `price_id`（收银台价格档标识）是**两个完全不同维度的标识**，绝不可混淆。本文所有章节都会明确区分二者。

---

## 0. 名词与高风险概念辨析（先读）

| 标识 | 中文 | 含义 | 物理列 | 覆盖列 | 引用对象 |
| --- | --- | --- | --- | --- | --- |
| `product_id` | IAP 商品 ID | 渠道商店（Google Play / App Store / 各国内渠道）侧的**内购商品标识符**，玩家在商店内购时实际下单的商品 SKU。例：`gem_60`、`com.game.gem60`。 | `products.product_id` | `channel_products.product_id_override` | 渠道商店后台登记的商品 |
| `price_id` | 收银台价格档 | **自有收银台**侧的**价格档位标识符**，对应 `cashier_price_rows.price_id`（见 `cashier-template` / `game-cashier`）。它决定了该商品在收银台走哪一档价格矩阵（含税/不含税/分国家币种）。例：`price_499`、`price_jp_600`。 | `products.price_id` | `channel_products.price_id_override` | 收银台价格模板的价格行 `cashier_price_rows.price_id` |

> 一句话区分：**`product_id` 是"在商店里卖的是哪个内购商品"；`price_id` 是"在我们自己的收银台里这个商品按哪一档价格收钱"。** 两者来源不同系统、覆盖逻辑互相独立、字符串长度上限不同（`product_id` 128、`price_id` 64），任何接口/前端/解析逻辑都必须分别处理，禁止用一个值同时填充另一个。

| 实体 | 中文 | 落地表 | env |
| --- | --- | --- | --- |
| Product（逻辑商品 / 商品主数据） | 游戏维度的商品基准定义 | `products` | 每环境独立 schema（D1，不带 env 列） |
| ChannelProduct（包级商品映射 / 覆盖） | 某个渠道包对某商品的 `product_id`/`price_id` 覆盖关系 | `channel_products` | 每环境独立 schema（D1，不带 env 列） |
| ChannelIapTemplate（渠道 IAP 模板） | 平台级模板四件套，定义渠道侧 IAP 参数表单 | `channel_iap_templates` | **平台级共享 schema `platform`**（不带 env） |
| GameChannelIapConfig（渠道 IAP 配置） | 某游戏渠道实例的 IAP 参数配置实例 | `game_channel_iap_configs` | 每环境独立 schema（D1，不带 env 列） |
| ChannelPackageIapOverride（包级 IAP 覆盖） | 某渠道包对渠道 IAP 配置的覆盖实例 | `channel_package_iap_overrides` | 每环境独立 schema（D1，不带 env 列） |

---

## 1. 边界（Scope）

本模块明确划分为**三块互不混淆的子能力**：

### 1.1 商品主数据（Product master data）

- 表：`products`。
- 维度：**游戏维度**（`game_id_ref`），与渠道、market、包无关。
- 职责：定义一个游戏下的逻辑商品（IAP 商品 ID 基准 `product_id`、商品名、基准金额 `base_amount_minor + base_currency`、收银台价格档基准 `price_id`、是否启用）。
- 这是"该游戏一共卖哪些商品"的单一事实来源。所有包级映射都引用它。

### 1.2 包级映射（Per-package mapping / override）

- 表：`channel_products`。
- 维度：**渠道包维度**（`package_id_ref` → `channel_packages`）× **商品维度**（`product_id_ref` → `products`）。
- 职责：描述"某个渠道包里，这个商品的 IAP 商品 ID 与收银台价格档，相对商品基准是否需要覆盖"。
- 两组独立覆盖开关：
  - `product_id_mode` + `product_id_override`：覆盖 **IAP 商品 ID**。
  - `price_id_mode` + `price_id_override`：覆盖 **收银台价格档**。
- 解决场景：95% 默认继承商品基准；少数联运包/地区包需要用不同的商店 SKU 或不同价格档。

### 1.3 渠道 IAP 配置（Channel IAP config）

- 表：`channel_iap_templates`（平台级模板）、`game_channel_iap_configs`（渠道实例配置）、`channel_package_iap_overrides`（包级覆盖）。
- 维度：渠道 IAP **支付/校验参数**（如商店校验回调、公钥、商户号等渠道侧 IAP 参数），由模板四件套驱动。
- 职责：配置"这个游戏渠道实例做渠道内购（IAP）需要的渠道侧参数"，并支持个别包覆盖。
- **与收银台支付路由严格隔离**（红线，见 `00` §9）：渠道 IAP 配置 ≠ 收银台支付路由（`payment`），二者不在本模块混用。

### 1.4 明确不在本模块（Out of scope）

- 收银台价格模板/价格行/价格矩阵：`cashier-template`（`cashier_price_*`）。
- 游戏级收银台绑定与价格覆盖：`game-cashier`（`game_cashier_profiles`、`game_cashier_price_overrides`）。
- 支付方式/提供商/商户/路由：`payment`（`payment_routes` 等）。
- 渠道包（`channel_packages`）的创建与基础属性：`channel`（渠道实例）。本模块只**引用**包，不创建包。
- `price_id` 指向的价格行实际金额来自 `cashier-template` / `game-cashier`；本模块只保存 `price_id` 字符串引用，不保存价格档金额。

---

## 2. 领域模型（Domain Model）

领域聚合：`internal/domain/product`（见 `01` §4 目录）。聚合根为 **Product**，并以"包级映射"为其从属实体。IAP 配置在领域上更贴近渠道实例（`channel` / 模板驱动配置），但本模块文档将其一并描述，应用层落在 `IAPConfigService`。

### 2.1 Product 聚合（聚合根）

```text
Product (聚合根, 对应 products 行)
├── 逻辑商品身份: (gameId, productId)   ← env 由所在 schema 决定，业务表不带 env 列
├── 基准属性:
│     productName
│     baseAmountMinor + baseCurrency   ← 金额必须按 00 §5 归一化
│     priceId(基准收银台价格档)
│     enabled
└── 包级映射集合: []ChannelProduct (按 packageId 聚合)
      ├── 包级 IAP 商品 ID 覆盖 (product_id_mode / product_id_override)
      └── 包级收银台 price_id 覆盖 (price_id_mode / price_id_override)
```

聚合不变量（invariants）：

1. `product_id` 在 `(game_id)` 内唯一（每环境 schema 内，不前置 env）。
2. `base_amount_minor` 必须是已按 `base_currency` 的 `currency_specs` 归一化后的整数最小单位值，且 `>= min_amount_minor`。
3. 包级映射的 `product_id_ref` 与 `package_id_ref` 必然落在同一环境 schema（= 同 env）——同 schema 内普通外键即可保证一致，无需任何 env 一致性校验或跨 env 引用判断。
4. 覆盖模式为 `override` 时，对应的 `*_override` 字段不得为空字符串；为 `default` 时，对应 `*_override` 字段必须为空字符串（写入时归一化清空）。

### 2.2 ChannelProduct（包级映射实体）

- 身份：`(package_id_ref, product_id_ref)`（每环境 schema 内）。
- 表达"某包对某商品"的覆盖意图，本身不存最终生效值，只存"模式 + 覆盖候选值"。最终生效值由 §5.2 的解析逻辑在读取/快照时计算。
- 两组覆盖维度**完全正交**：可以只覆盖 `product_id`、只覆盖 `price_id`、都覆盖、都不覆盖。

### 2.3 渠道 IAP 配置（模板驱动配置实体）

```text
ChannelIapTemplate (平台级, channel_iap_templates)
        │  定义 form_schema/secret_fields/file_fields/validation_rules
        │  按 channel + template_version (简单模板表, 取 enabled 最新版本, 00 §4.4.1)
        ▼
GameChannelIapConfig (渠道实例配置, game_channel_iap_configs, 每环境独立 schema, 不带 env 列)
        │  config_json 按模板四件套填写; config_status: empty/invalid/valid
        ▼
ChannelPackageIapOverride (包级覆盖, channel_package_iap_overrides, 每环境独立 schema, 不带 env 列)
           对个别包覆盖渠道 IAP 配置(同样模板驱动, config_status)
```

- 模板（`channel_iap_templates`）为平台级、全 env 共享一套定义；**实际配置实例（config / override）各自独立**，不共享 secret/file/状态（见 `00` §4.4）。
- 渠道 IAP 配置实例与包级 IAP 覆盖实例都遵循 `00` §3.4 的 ConfigStatus 状态机与 §6 密文/文件规则。

---

## 3. 数据模型（逐表逐字段）

> 约定：以下"v2 变更"列标注本期需要的迁移动作；业务表均为**每环境独立 schema、不带 `env` 列**，唯一键不前置 env（依据 D1，见 `00` §2.2）。所有金额列均为整数最小单位（minor），写入路径强制走 `00` §5 归一化。

### 3.1 `products`（商品主数据，**每环境独立 schema，不带 env 列**）

| 列 | 类型 | 默认 | 约束/说明 | v2 变更 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | — |
| `game_id_ref` | BIGINT | — | `NOT NULL REFERENCES games(id)`（同 schema 普通外键） | — |
| `product_id` | VARCHAR(128) | — | `NOT NULL`，**IAP 商品 ID 基准**（商店内购 SKU） | — |
| `product_name` | VARCHAR(128) | — | `NOT NULL`，商品展示名 | — |
| `base_amount_minor` | BIGINT | — | `NOT NULL`，基准金额（最小单位），**必须按 `base_currency` 归一化后写入** | — |
| `base_currency` | VARCHAR(8) | — | `NOT NULL`，必须命中 `currency_specs.currency_code` 且 `enabled=TRUE` | — |
| `price_id` | VARCHAR(64) | — | `NOT NULL`，**收银台价格档基准**（引用 `cashier_price_rows.price_id`） | — |
| `enabled` | BOOLEAN | `TRUE` | `NOT NULL` | — |
| `created_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` | — |
| `updated_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` | — |
| 唯一键 | — | — | `UNIQUE(game_id_ref, product_id)`（每环境 schema 内，不前置 env） | — |

字段释义补充：

- `product_id` 与 `price_id` 是两个不同维度，长度上限不同（128 vs 64），不可互填。
- `base_amount_minor + base_currency` 是"商品基准定价"，用于展示/对账/快照基线；真正向玩家收钱的金额由 `price_id` 指向的收银台价格行决定（含税分国家），二者通过 `price_id` 关联但不强一致。

### 3.2 `channel_products`（包级映射 / 覆盖，**每环境独立 schema，不带 env 列**）

| 列 | 类型 | 默认 | 约束/说明 | v2 变更 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | — |
| `product_id_ref` | BIGINT | — | `NOT NULL REFERENCES products(id)`（同 schema 普通外键） | — |
| `package_id_ref` | BIGINT | — | `NOT NULL REFERENCES channel_packages(id)`（同 schema 普通外键） | — |
| `product_id_mode` | VARCHAR(16) | `default` | `NOT NULL`，`CHECK IN ('default','override')`，**IAP 商品 ID 覆盖模式** | 补 `DEFAULT 'default'` |
| `product_id_override` | VARCHAR(128) | `''` | `NOT NULL`，仅 `product_id_mode='override'` 时有效（IAP 商品 ID 覆盖值） | — |
| `price_id_mode` | VARCHAR(16) | `default` | `NOT NULL`，`CHECK IN ('default','override')`，**收银台价格档覆盖模式** | 补 `DEFAULT 'default'` |
| `price_id_override` | VARCHAR(64) | `''` | `NOT NULL`，仅 `price_id_mode='override'` 时有效（价格档覆盖值） | — |
| `enabled` | BOOLEAN | `TRUE` | `NOT NULL`，该商品是否在此包内启用 | — |
| `created_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` | — |
| `updated_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` | — |
| 唯一键 | — | — | `UNIQUE(package_id_ref, product_id_ref)`（每环境 schema 内，不前置 env） | — |

两组 mode + override 默认语义（强约束）：

- 两组模式默认都为 `default`（继承商品基准），两个 override 字段默认空串。
- `product_id_mode` 控制 `product_id`，`price_id_mode` 控制 `price_id`，**互不影响**。一个包对某商品可以"覆盖 IAP 商品 ID 但不覆盖价格档"，反之亦然。

### 3.3 `channel_iap_templates`（渠道 IAP 模板，**平台级、不带 env**）

| 列 | 类型 | 默认 | 约束/说明 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK |
| `channel_id_ref` | BIGINT | — | `NOT NULL REFERENCES channels(id)` |
| `template_version` | VARCHAR(32) | — | `NOT NULL`；简单模板表（`00` §4.4.1），无 `status` 列，取 `enabled` 最新版本 |
| `form_schema_json` | JSONB | `[]` | 模板四件套：渲染字段定义（`00` §4.1） |
| `secret_fields_json` | JSONB | `[]` | 模板四件套：密文字段（`00` §4.2、§6.1） |
| `file_fields_json` | JSONB | `[]` | 模板四件套：文件字段（`00` §4.2、§6.2） |
| `validation_rules_json` | JSONB | `{}` | 模板四件套：校验规则（`00` §4.3） |
| `enabled` | BOOLEAN | `TRUE` | `NOT NULL` |
| `created_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` |
| `updated_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` |
| 唯一键 | — | — | `UNIQUE(channel_id_ref, template_version)`（**不前置 env**，平台级共享） |

> 该表是平台级基础数据（见 `00` §2.2 不带 env 清单），全环境共享同一套模板定义；模板内容维护由基础数据/模板管理后台负责（模块 system），本模块只**消费**模板渲染表单与校验。

### 3.4 `game_channel_iap_configs`（渠道 IAP 配置实例，**每环境独立 schema，不带 env 列**）

| 列 | 类型 | 默认 | 约束/说明 | v2 变更 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | — |
| `game_channel_id_ref` | BIGINT | — | `NOT NULL REFERENCES game_channels(id)`（同 schema 普通外键） | — |
| `enabled` | BOOLEAN | `FALSE` | `NOT NULL`，注意默认 **false**（默认未启用） | — |
| `config_json` | JSONB | `{}` | 按 `channel_iap_templates` 四件套填写；密文位脱敏存储 | — |
| `config_status` | VARCHAR(16) | `empty` | `NOT NULL`，`CHECK IN ('empty','invalid','valid')`（`00` §3.4） | — |
| `last_check_at` | TIMESTAMPTZ | NULL | 可空，最近一次校验时间 | — |
| `last_check_message` | VARCHAR(255) | `''` | `NOT NULL`，最近校验消息 | — |
| `created_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` | — |
| `updated_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` | — |
| 唯一键 | — | — | `UNIQUE(game_channel_id_ref)`（每环境 schema 内，不前置 env） | — |

### 3.5 `channel_package_iap_overrides`（包级 IAP 覆盖实例，**每环境独立 schema，不带 env 列**）

| 列 | 类型 | 默认 | 约束/说明 | v2 变更 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | — |
| `package_id_ref` | BIGINT | — | `NOT NULL REFERENCES channel_packages(id)`（同 schema 普通外键） | — |
| `enabled` | BOOLEAN | `FALSE` | `NOT NULL`，默认 **false** | — |
| `config_json` | JSONB | `{}` | 按渠道 IAP 模板四件套填写；密文脱敏 | — |
| `config_status` | VARCHAR(16) | `empty` | `NOT NULL`，`CHECK IN ('empty','invalid','valid')` | — |
| `last_check_at` | TIMESTAMPTZ | NULL | 可空 | — |
| `last_check_message` | VARCHAR(255) | `''` | `NOT NULL` | — |
| `created_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` | — |
| `updated_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` | — |
| 唯一键 | — | — | `UNIQUE(package_id_ref)`（每环境 schema 内，不前置 env） | — |

### 3.6 IAP 配置 / 覆盖的 config_status 与四件套关系

- `config_json` 的可填字段、哪些是 secret、哪些是 file、校验规则全部来自 `channel_iap_templates` 对应渠道 `enabled=TRUE` 的最新 `template_version` 的四件套。
- `config_status` 推导（写入后由服务端统一计算，见 §5.4）：
  - 未填任何字段 → `empty`。
  - 已填部分但缺必填/缺 secret/缺 file 或校验未过 → `invalid`。
  - 全部必填（含 secret/file）齐全且校验通过 → `valid`。
- 复制创建实例并清空 secret/file 的，**必须 `invalid`，不得 `empty`**，且 `last_check_message` 提示"缺少必填敏感字段或文件字段"（`00` §3.4 强约束）。

---

## 4. 枚举与默认值清单（穷尽）

> 与 `00` §3 全局清单一致，此处列出本模块涉及到的全部枚举与默认值，作为本模块前后端落地的事实来源。

### 4.1 枚举

| 枚举 | 取值 | 默认值 | 落地列 |
| --- | --- | --- | --- |
| `OverrideMode` | `default` / `override` | `default` | `channel_products.product_id_mode`、`channel_products.price_id_mode` |
| `ConfigStatus` | `empty` / `invalid` / `valid` | `empty` | `game_channel_iap_configs.config_status`、`channel_package_iap_overrides.config_status` |
| `Environment` | `develop` / `sandbox` / `production` | `develop`（运行环境） | 决定各业务表落在哪个环境 schema（业务表不带 env 列） |
| `RoundingMode`（金额归一化引用） | `half_up` / `floor` / `ceil` / `truncate` | `half_up` | 来自 `currency_specs.rounding_mode` |

> `channel_iap_templates` 为**简单模板表**（`00` §4.4.1），无 `status` 列，不走 `VersionStatus` 三态机；运行时取 `enabled=TRUE` 的最新 `template_version`。

### 4.2 默认值清单（逐列）

| 列 | 默认值 | 备注 |
| --- | --- | --- |
| `products.enabled` | `TRUE` | 新建商品默认启用 |
| `products.base_currency` | 无 DB 默认，**业务建议 `USD`** | 必须命中 `currency_specs` |
| `products.base_amount_minor` | 无默认（必填） | 归一化后整数 minor |
| `products.price_id` | 无默认（必填） | 收银台价格档基准 |
| `channel_products.product_id_mode` | `default` | 不覆盖 IAP 商品 ID |
| `channel_products.product_id_override` | `''` | 仅 override 时填 |
| `channel_products.price_id_mode` | `default` | 不覆盖价格档 |
| `channel_products.price_id_override` | `''` | 仅 override 时填 |
| `channel_products.enabled` | `TRUE` | 该商品在包内默认启用 |
| `game_channel_iap_configs.enabled` | `FALSE` | **默认未启用** |
| `game_channel_iap_configs.config_status` | `empty` | — |
| `game_channel_iap_configs.config_json` | `{}` | — |
| `game_channel_iap_configs.last_check_message` | `''` | — |
| `channel_package_iap_overrides.enabled` | `FALSE` | **默认未启用** |
| `channel_package_iap_overrides.config_status` | `empty` | — |
| `channel_package_iap_overrides.config_json` | `{}` | — |
| 运行环境（无 env 列） | 写操作落当前运行环境对应 schema | 前端不可指定/跨 schema 写（`00` §2.1） |
| 所有 `created_at/updated_at` | `NOW()` | — |

---

## 5. 业务规则（Business Rules）

### 5.1 金额归一化全流程（不可绕过，`00` §5）

仅 `products.base_amount_minor` 涉及本模块金额写入（`channel_products`/IAP 配置不含金额；`price_id` 指向的金额属 `cashier-template` / `game-cashier`）。写入/更新商品金额时**必须严格按序**：

1. 读取 `base_currency` 对应的 `currency_specs`；缺失或 `enabled=FALSE` → 拒绝，错误码 `CURRENCY_NOT_SUPPORTED`（400）。
2. 按 `decimal_places` 校验/解析入参精度（入参建议同时支持"主单位小数 amount"或"整数 minor"两种，但最终统一转 minor；见 §6.2 DTO 约定）。
3. 按 `min_amount_minor` 校验下限，低于下限 → `VALIDATION_FAILED`。
4. 按 `rounding_mode` 归一化到该币种精度。
5. 统一存为 `base_amount_minor`（整数最小单位）。

示例：`baseCurrency=USD`（decimal=2, min=1, half_up），入参 `amount=4.999` → 归一化 minor = `500`；入参 `amount=0.001` → 低于 `min_amount_minor=1`（1 minor=0.01 USD）→ 拒绝。`baseCurrency=JPY`（decimal=0），入参 `amount=120.5` → half_up → `121`（minor=日元本位）。

> 归一化纯逻辑放在 `internal/domain/common`（currency），`ProductService` 调用，不在 transport 层散落。

### 5.2 生效 `product_id` / `price_id` 解析（核心逻辑，务必区分两维）

给定一条 `channel_products`（包 P × 商品 R）及其商品基准 `products`，**包内最终生效值**按下式独立解析两维：

```text
effectiveProductId =
    (product_id_mode == "override" && product_id_override != "")
        ? product_id_override          // 用包级覆盖的 IAP 商品 ID
        : products.product_id          // 否则回退商品基准 IAP 商品 ID

effectivePriceId =
    (price_id_mode == "override" && price_id_override != "")
        ? price_id_override            // 用包级覆盖的收银台价格档
        : products.price_id            // 否则回退商品基准价格档
```

规则细化：

1. 两维**完全独立**：`product_id` 与 `price_id` 各自判断，互不牵连。
2. `mode=default` 时一律回退基准，即便 `*_override` 字段历史上有残值也忽略（写入时已被清空，见 5.3）。
3. `mode=override` 但 `*_override` 为空：属非法状态，写入时即被 `VALIDATION_FAILED` 拦截，正常不会出现；解析时作防御性回退基准并记 warning。
4. 解析结果用于：配置快照（`snapshot`）按 market 输出最终 IAP/价格映射、运行时客户端配置、`sync` diff 比对。
5. 解析**不修改** `products` 基准，只在读取/快照时计算派生值。

### 5.3 包级覆盖写入归一化

写入 `channel_products` 时服务端统一归一化，保证存储一致：

- `product_id_mode=default` ⇒ 强制 `product_id_override=''`。
- `product_id_mode=override` ⇒ `product_id_override` 必填非空，否则 `VALIDATION_FAILED`。
- `price_id_mode=default` ⇒ 强制 `price_id_override=''`。
- `price_id_mode=override` ⇒ `price_id_override` 必填非空，否则 `VALIDATION_FAILED`。
- 引用的 `product_id_ref` 必须属于该包所在游戏（同 game；父子行天然同 schema=同 env），否则 `VALIDATION_FAILED`/`CONFLICT`。
- `price_id_override` 是否必须存在于某价格模板的价格行：**本期不做强外键校验**（价格档可能后建），列为未决问题（见 §11）；仅做格式校验。

### 5.4 IAP 配置与包级覆盖的关系

- `game_channel_iap_configs` 是渠道实例级"基线 IAP 配置"；`channel_package_iap_overrides` 是个别包对它的覆盖。
- 生效 IAP 配置解析（供快照/运行时）：

```text
effectiveIapConfig(package) =
    package 有 enabled=true 的 channel_package_iap_overrides(config_status=valid)
        ? merge(baseChannelIapConfig, packageOverride.config_json)   // 包级覆盖字段优先
        : baseChannelIapConfig                                       // 否则用渠道实例配置
其中 baseChannelIapConfig = 该 game_channel 的 game_channel_iap_configs(enabled=true 且 config_status=valid)
```

- 合并粒度：按 `config_json` 顶层字段做覆盖式 merge（包级出现的字段整体替换基线同名字段）。
- 任一侧 `enabled=false` 或 `config_status!=valid`：该侧不进入快照/同步/客户端最终配置（`00` §9 红线）。
- `channel_iap_templates` 与 `game_channel_iap_configs`/`channel_package_iap_overrides` 的关系：模板提供字段定义与校验，配置实例提供实际值；模板版本切换不自动改写已存实例值，但会触发实例重校验得到新的 `config_status`。

### 5.5 与 env / 同步的关系

- 所有业务表写操作落当前运行环境对应 schema（`00` §2.1），前端不可指定/跨 schema 写。
- `sync` 域中本模块涉及 `SyncSection=products`（`00` §3.1）。同步 diff 比对的是"解析后的生效值"（§5.2/§5.4 的派生结果）与原始行，密文字段 `masked`。
- 被禁用/无效的商品、包映射、IAP 配置不进快照、不参与同步、不进客户端最终配置。

### 5.6 红线（模块内重申）

- 不把"渠道 IAP 配置"与"收银台支付路由"混在一起（`00` §9）。
- 不把 `product_id`（IAP 商品 ID）与 `price_id`（收银台价格档）混填。
- 商品金额写入不绕过 `currency_specs`。
- IAP 配置密文不落明文、响应脱敏。

---

## 6. 后端 API（逐接口完整 DTO + 校验 + 示例 JSON）

> 全部遵循 `00` §7 统一约定：前缀 `/api/admin`、`Authorization: Bearer`、camelCase、统一响应包络 `{ "data": ... }` / `{ "error": ... }`、写操作落当前运行环境对应 schema、分页 `page/pageSize`。下列接口默认需要登录。

权限码（`resource.action`，`00` §7.5）：

| 操作 | 权限码 |
| --- | --- |
| 读商品/映射/IAP 配置 | `product.read` |
| 写商品/映射 | `product.write` |
| 写渠道 IAP 配置/包级覆盖 | `product.write`（IAP 子能力共用，或细分 `iap.write`，本期沿用 `product.write`） |

审计（`00` §8）：所有写操作写 `audit_logs`，`action` 取 `product.create` / `product.update` / `product.package_products.update` / `iap.config.update` / `iap.override.update`，`detail_json` 记 before/after（密文脱敏）。

---

### 6.1 `GET /api/admin/games/{gameId}/products`（列出游戏商品）

- 权限：`product.read`。
- Query：`page`（默认 1）、`pageSize`（默认 20，最大 100）、`sort`（默认 `-updatedAt`）、可选 `enabled`（`true/false` 过滤）、`keyword`（按 `product_id`/`product_name` 模糊）。
- 作用 env：当前运行环境。

响应（200）：

```json
{
  "data": {
    "items": [
      {
        "id": 1001,
        "env": "sandbox",
        "gameId": "honkai",
        "productId": "gem_60",
        "productName": "60 Gems",
        "baseAmountMinor": 499,
        "baseCurrency": "USD",
        "baseAmountDisplay": "4.99",
        "priceId": "price_499",
        "enabled": true,
        "createdAt": "2026-06-15T10:00:00Z",
        "updatedAt": "2026-06-16T09:30:00Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 1
  }
}
```

> `baseAmountDisplay` 为服务端按 `base_currency` 的 `decimal_places` 反算的主单位字符串（只读，便于前端展示），非存储列。

### 6.2 `POST /api/admin/games/{gameId}/products`（创建商品）

- 权限：`product.write`。
- 请求 DTO：

| 字段 | 类型 | 必填 | 校验 |
| --- | --- | --- | --- |
| `productId` | string | 是 | 1–128 字符；`(gameId)` 内唯一（每环境 schema 内），重复 → `CONFLICT` |
| `productName` | string | 是 | 1–128 字符 |
| `baseCurrency` | string | 是 | 命中 `currency_specs` 且 `enabled`，否则 `CURRENCY_NOT_SUPPORTED` |
| `baseAmountMinor` | integer | 二选一 | 与 `baseAmount` 二选一；整数 minor，`>= min_amount_minor` |
| `baseAmount` | string/number | 二选一 | 主单位金额，服务端按 §5.1 归一化为 minor |
| `priceId` | string | 是 | 1–64 字符；收银台价格档标识 |
| `enabled` | boolean | 否 | 默认 `true` |

校验顺序：基础格式 → 唯一性 → 金额归一化（§5.1）。`productId` 与 `priceId` 分别校验，不可互填。

请求示例：

```json
POST /api/admin/games/honkai/products
{
  "productId": "gem_60",
  "productName": "60 Gems",
  "baseAmount": "4.99",
  "baseCurrency": "USD",
  "priceId": "price_499",
  "enabled": true
}
```

响应（201）：

```json
{
  "data": {
    "id": 1001,
    "env": "sandbox",
    "gameId": "honkai",
    "productId": "gem_60",
    "productName": "60 Gems",
    "baseAmountMinor": 499,
    "baseCurrency": "USD",
    "baseAmountDisplay": "4.99",
    "priceId": "price_499",
    "enabled": true,
    "createdAt": "2026-06-17T12:00:00Z",
    "updatedAt": "2026-06-17T12:00:00Z"
  }
}
```

错误示例（币种不支持）：

```json
{ "error": { "code": "CURRENCY_NOT_SUPPORTED", "message": "currency 'ABC' is not in currency_specs", "details": [] } }
```

### 6.3 `PATCH /api/admin/products/{productId}`（更新商品）

> 路径 `{productId}` 为业务 `product_id`，定位行用 `(gameId, productId)`（在当前环境 schema 内）；`gameId` 由商品归属推导（或要求 query `gameId`，实现统一二选一，建议带 `gameId` query 以避免歧义）。

- 权限：`product.write`。
- 请求 DTO（均可选，部分更新）：`productName`、`baseAmount`/`baseAmountMinor`、`baseCurrency`、`priceId`、`enabled`。
- 校验：
  - 不允许改 `productId`（身份键），如需改名等同删除重建；传入则忽略或 `VALIDATION_FAILED`。
  - 改 `baseAmount*` 或 `baseCurrency` 任一 ⇒ 重新走 §5.1 全流程归一化。
  - `priceId` 单独校验长度，不与 `productId` 混。

请求示例：

```json
PATCH /api/admin/products/gem_60?gameId=honkai
{
  "productName": "60 Gems (Promo)",
  "baseAmount": "3.99",
  "baseCurrency": "USD",
  "priceId": "price_399"
}
```

响应（200）：返回更新后的完整商品对象（同 6.2 响应结构）。

### 6.4 `GET /api/admin/channel-packages/{packageId}/products`（读包级商品映射）

- 权限：`product.read`。
- 路径 `{packageId}` 为 `channel_packages` 行（业务 `package_code` 或 id，按现有风格统一用 id/code，本文用 id 语义）。
- 返回：该包下所有 `channel_products` 映射，并联表 `products` 基准，**同时返回解析后的生效值**（§5.2），方便前端直接展示。

响应（200）：

```json
{
  "data": {
    "packageId": 5001,
    "packageCode": "google-global",
    "items": [
      {
        "productId": "gem_60",
        "productName": "60 Gems",
        "enabled": true,
        "base": {
          "productId": "gem_60",
          "priceId": "price_499",
          "baseAmountMinor": 499,
          "baseCurrency": "USD"
        },
        "productIdMode": "default",
        "productIdOverride": "",
        "priceIdMode": "override",
        "priceIdOverride": "price_jp_600",
        "effective": {
          "productId": "gem_60",
          "priceId": "price_jp_600"
        }
      }
    ]
  }
}
```

> 注意 `effective.productId` 回退基准 `gem_60`（IAP 商品 ID 未覆盖），而 `effective.priceId` 取覆盖值 `price_jp_600`（价格档已覆盖）——清晰体现两维独立解析。

### 6.5 `PUT /api/admin/channel-packages/{packageId}/products`（整表覆盖写包级映射）

- 权限：`product.write`。
- 语义：以 `items` 全量声明该包的商品映射（缺省项视为移除该包内映射；具体"删除 vs 保留"策略：本期采用**全量 upsert + 删除未出现项**，并写审计）。
- 请求 DTO（每项）：

| 字段 | 类型 | 必填 | 校验 |
| --- | --- | --- | --- |
| `productId` | string | 是 | 必须是该包所属 game 下已存在商品（同 game、同 schema），否则 `VALIDATION_FAILED` |
| `enabled` | boolean | 否 | 默认 `true` |
| `productIdMode` | enum | 否 | `default`/`override`，默认 `default` |
| `productIdOverride` | string | 条件 | `productIdMode=override` 时必填非空（≤128）；否则强制 `''` |
| `priceIdMode` | enum | 否 | `default`/`override`，默认 `default` |
| `priceIdOverride` | string | 条件 | `priceIdMode=override` 时必填非空（≤64）；否则强制 `''` |

校验要点（§5.3）：两组 mode/override 配套校验；`productIdOverride` 与 `priceIdOverride` 分别按各自长度上限校验，不可混；`items` 内 `productId` 不可重复。

请求示例：

```json
PUT /api/admin/channel-packages/5001/products
{
  "items": [
    {
      "productId": "gem_60",
      "enabled": true,
      "productIdMode": "default",
      "productIdOverride": "",
      "priceIdMode": "override",
      "priceIdOverride": "price_jp_600"
    },
    {
      "productId": "gem_300",
      "enabled": true,
      "productIdMode": "override",
      "productIdOverride": "com.partner.gem300",
      "priceIdMode": "default",
      "priceIdOverride": ""
    }
  ]
}
```

响应（200）：返回与 6.4 相同结构（含 `effective`）。

错误示例（override 模式缺值）：

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "priceIdOverride is required when priceIdMode=override",
    "details": [{ "field": "items[0].priceIdOverride" }]
  }
}
```

### 6.6 `GET /api/admin/game-channels/{gameChannelId}/iap-config`（读渠道 IAP 配置）

- 权限：`product.read`。
- 返回：该游戏渠道实例的 IAP 配置实例 + 其渠道 `enabled=TRUE` 的最新 `template_version` 模板四件套（供前端渲染），密文字段脱敏。

响应（200）：

```json
{
  "data": {
    "gameChannelId": 7001,
    "channelId": "apple",
    "template": {
      "templateVersion": "v3",
      "formSchema": [
        { "key": "issuerId", "label": "Issuer ID", "component": "input", "required": true, "order": 10 },
        { "key": "keyId", "label": "Key ID", "component": "input", "required": true, "order": 20 },
        { "key": "privateKey", "label": "Private Key", "component": "file", "required": true, "order": 30 }
      ],
      "secretFields": [],
      "fileFields": [{ "key": "privateKey", "accept": [".p8"], "maxSizeKB": 64 }],
      "validationRules": { "issuerId": { "minLen": 1 } }
    },
    "config": {
      "enabled": true,
      "configStatus": "valid",
      "configJson": { "issuerId": "ABCDEF", "keyId": "XYZ123", "privateKey": "file://stored-ref" },
      "lastCheckAt": "2026-06-16T08:00:00Z",
      "lastCheckMessage": "ok"
    }
  }
}
```

### 6.7 `PUT /api/admin/game-channels/{gameChannelId}/iap-config`（写渠道 IAP 配置）

- 权限：`product.write`。
- 请求 DTO：

| 字段 | 类型 | 必填 | 校验 |
| --- | --- | --- | --- |
| `enabled` | boolean | 否 | 默认 `false` |
| `configJson` | object | 是 | 按渠道 IAP 模板四件套校验（必填/类型/pattern/format） |

服务端处理：
1. 取该渠道 `channel_iap_templates` `enabled=TRUE` 的最新 `template_version` 四件套校验 `configJson`。
2. secret 字段加密落库、响应脱敏（`00` §6.1）；file 字段存引用（§6.2）。
3. 计算 `config_status`（§5.4 / `00` §3.4），写 `last_check_at` / `last_check_message`。

请求示例：

```json
PUT /api/admin/game-channels/7001/iap-config
{
  "enabled": true,
  "configJson": {
    "issuerId": "ABCDEF",
    "keyId": "XYZ123",
    "privateKey": "file://upload-token-xyz"
  }
}
```

响应（200）：同 6.6 的 `config` 部分（密文脱敏，`configStatus` 为重算值）。

错误示例（缺必填敏感/文件字段，复制创建场景）：

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "缺少必填敏感字段或文件字段",
    "details": [{ "field": "configJson.privateKey", "reason": "required_file_missing" }]
  }
}
```

> 即使校验失败需要落库为 `invalid` 的场景（如复制创建后清空 secret/file），服务端按业务策略可选择"保存为 invalid"或"拒绝保存"；本模块约定：显式 PUT 缺必填 → 返回 `VALIDATION_FAILED` 且不改 `enabled` 为 true；复制创建产生的实例由复制流程直接落 `invalid`（`00` §3.4）。

### 6.8 `GET /api/admin/channel-packages/{packageId}/iap-override`（读包级 IAP 覆盖）

- 权限：`product.read`。
- 返回：该包的 `channel_package_iap_overrides` 实例 + 模板四件套（同 6.6 结构），并附带其渠道实例基线配置以便前端对比。

响应（200）：

```json
{
  "data": {
    "packageId": 5001,
    "packageCode": "google-global",
    "channelId": "google",
    "template": { "templateVersion": "v2", "formSchema": [], "secretFields": [], "fileFields": [], "validationRules": {} },
    "baseConfig": { "enabled": true, "configStatus": "valid" },
    "override": {
      "enabled": false,
      "configStatus": "empty",
      "configJson": {},
      "lastCheckAt": null,
      "lastCheckMessage": ""
    }
  }
}
```

### 6.9 `PUT /api/admin/channel-packages/{packageId}/iap-override`（写包级 IAP 覆盖）

- 权限：`product.write`。
- 请求 DTO：同 6.7（`enabled` + `configJson`），按同一模板四件套校验、密文/文件处理、`config_status` 计算。
- 语义：包级覆盖字段优先于渠道实例基线（§5.4 merge）。`enabled=false` 时该覆盖不生效，运行时回退渠道实例配置。

请求示例：

```json
PUT /api/admin/channel-packages/5001/iap-override
{
  "enabled": true,
  "configJson": { "merchantPublicKey": "MIIB...override..." }
}
```

响应（200）：同 6.8 的 `override` 部分（脱敏、`configStatus` 重算）。

---

## 7. 应用服务（Application Services）

落在 `internal/app/command` + `internal/app/query`，编排领域 + 仓储（`01` §4）。

### 7.1 `ProductService`

职责：
- 商品 CRUD（`products`），调用 `common/currency` 归一化（§5.1）。
- 包级映射读写（`channel_products`），含 §5.3 写入归一化、§5.2 生效解析（供查询返回 `effective` 与快照）。
- 唯一性/外键校验（父子行天然同 schema=同 env，无需 env 一致性校验）。

依赖仓储：`ProductRepository`（窄仓储：products CRUD + by game 查询）、`ChannelProductRepository`（包级映射 upsert/delete by package）、`ChannelPackageRepository`（只读校验包归属）、`CurrencySpecRepository`（读 currency_specs）。

关键方法（示意）：
- `ListProducts(ctx, env, gameId, page, filters)`
- `CreateProduct(ctx, env, gameId, cmd)` / `UpdateProduct(ctx, env, gameId, productId, cmd)`
- `GetPackageProducts(ctx, env, packageId)` → 返回含 `effective`
- `PutPackageProducts(ctx, env, packageId, items)` → 全量 upsert + 删除未出现项

### 7.2 `IAPConfigService`

职责：
- 渠道 IAP 配置读写（`game_channel_iap_configs`）。
- 包级 IAP 覆盖读写（`channel_package_iap_overrides`）。
- 模板四件套校验、密文加解密（`infra/crypto`）、文件引用（`infra/file`）、`config_status` 计算、生效 IAP 配置 merge 解析（§5.4）。

依赖：`GameChannelIapConfigRepository`、`ChannelPackageIapOverrideRepository`、`ChannelIapTemplateRepository`（读 `enabled` 最新模板）、`CryptoService`、`FileService`。

> 两个服务都不做跨表大编排越界：快照合并由 `snapshot` `ConfigSnapshotService` 消费本模块的"生效解析"结果；同步 diff 由 `sync` `SyncService` 调用。

---

## 8. 前端（Frontend）

> 技术栈 Element Plus + Pinia（`01` §5）。优先抽屉式交互，配置异常态行内可见，写操作挂权限指令。本模块前端入口位于游戏详情（`/games/:gameId`）的"商品 / IAP"相关 Tab，以及渠道包详情抽屉。

### 8.1 商品列表与编辑

- 页面：游戏详情 → "商品"Tab，表格列：`productId`(IAP 商品 ID)、`productName`、基准金额（`baseAmountDisplay` + `baseCurrency`）、`priceId`(收银台价格档)、`enabled`、更新时间。
- **列标题必须显式区分**：IAP 商品 ID 列标注"IAP 商品 ID"，价格档列标注"收银台价格档(price_id)"，避免运营混淆。
- 编辑抽屉：表单字段对应 6.2/6.3 DTO；`baseAmount` 输入框受 `currency_specs` 约束（见 8.4）。
- 校验前端预判：`productId` 长度≤128、`priceId` 长度≤64、币种下拉来自 `dictionary` store 的 currency 列表。

### 8.2 包级商品映射编辑（清楚区分四个字段）

- 入口：渠道包详情 → "商品映射"面板，加载 `GET /channel-packages/{packageId}/products`。
- 每行商品两组覆盖控件，**视觉上分两列分组**：
  - 「IAP 商品 ID」组：`productIdMode`（default/override 开关）+ `productIdOverride`（仅 override 可编辑），并展示基准 `base.productId`。
  - 「收银台价格档」组：`priceIdMode`（default/override 开关）+ `priceIdOverride`（仅 override 可编辑），并展示基准 `base.priceId`。
- 实时显示 `effective.productId` / `effective.priceId` 两个派生值（来自后端或前端按 §5.2 同样逻辑预览），帮助运营确认最终生效值。
- **强约束 UI**：`mode=default` 时禁用并清空对应 override 输入；切到 `override` 时 override 变必填，空值阻止提交。两组开关互不联动。
- 防混淆提示：override 输入框 placeholder 分别提示"填商店内购商品 SKU"与"填收银台价格档 ID"，且两个输入框样式/图标区分。

### 8.3 IAP 配置面板与包级覆盖面板

- 渠道 IAP 配置面板（游戏渠道实例详情）：消费 6.6 返回的模板四件套，统一用模板渲染器（`01` §5.3）渲染 `configJson`；密文字段显示为脱敏/可重填；文件字段走统一上传。
- `configStatus` 标签（empty/invalid/valid）行内显示，`invalid` 用告警色并展示 `lastCheckMessage`（如"缺少必填敏感字段或文件字段"），不得隐藏。
- 包级 IAP 覆盖面板：在渠道包详情，展示渠道基线配置（只读对比）+ 包级覆盖开关 `enabled` + 覆盖字段表单；明确标注"未启用时回退渠道配置"。
- 严格隔离提示：IAP 配置面板与收银台/支付路由面板分属不同模块入口，UI 不混排（红线）。

### 8.4 金额输入受 currency_specs 约束并预览舍入

- 选定 `baseCurrency` 后，金额输入框：
  - 小数位数限制 = `decimal_places`（如 JPY=0 禁止小数）。
  - 最小值提示 = `min_amount_minor` 换算的主单位下限。
  - 失焦时按 `rounding_mode` 预览归一化结果，并显示"将存储为 X minor"，与后端 §5.1 保持一致。
- 金额预览仅前端提示，最终以后端归一化为准；前端不得自行决定 minor 存储值绕过后端。

---

## 9. 与公共能力的关系

| 公共能力（00 / 01） | 本模块用法 |
| --- | --- |
| env 模型（§2，D1） | `products`/`channel_products`/`game_channel_iap_configs`/`channel_package_iap_overrides` 每环境独立 schema、不带 env 列，唯一键不前置 env；`channel_iap_templates` 平台级（共享 schema `platform`）不带 env |
| currency 归一化（§5） | `products.base_amount_minor` 写入强制走读 spec→校验精度→校验下限→舍入→存 minor |
| 模板四件套（§4） | `channel_iap_templates` 提供 form/secret/file/validation；IAP 配置实例据此渲染与校验 |
| ConfigStatus（§3.4） | IAP 配置与包级覆盖的 `config_status`，复制清空 secret/file ⇒ `invalid` |
| 模板版本（§4.4.1 简单模板表） | `channel_iap_templates` 无 `status` 列，不走 §3.3 三态机；取 `enabled` 最新 `template_version`（由模板管理后台维护） |
| 密文/文件（§6） | IAP `config_json` 密文加密落库、响应脱敏；文件存引用 |
| 统一 API/包络/错误码（§7） | 全部接口遵循 `{data}`/`{error}`、`CURRENCY_NOT_SUPPORTED`/`VALIDATION_FAILED`/`CONFLICT`/`NOT_FOUND` |
| 审计（§8） | 商品/映射/IAP 写操作写 `audit_logs`，action 与权限码同源 |
| 快照/同步（`snapshot` / `sync`） | 提供 §5.2/§5.4 生效解析结果给快照按 market 合并；`SyncSection=products` |
| 渠道实例（`channel`） | 引用 `game_channels` / `channel_packages`，不创建包 |
| 收银台（`cashier-template` / `game-cashier`） | `price_id` 指向 `cashier_price_rows.price_id`，本模块只存引用 |

---

## 10. 测试要点（Test Points）

### 10.1 金额归一化

- USD（decimal=2, min=1, half_up）：`4.999 → 500`；`4.991 → 499`；`0.004 → 拒绝(<min)`；`0.005 → 1`。
- JPY（decimal=0）：`120.5 → 121`；`120.4 → 120`；传小数被按 0 位归一化。
- 币种不在 `currency_specs` 或 `enabled=false` → `CURRENCY_NOT_SUPPORTED`。
- minor 与主单位两种入参等价性（`baseAmountMinor=499` 与 `baseAmount="4.99"`/USD 结果一致）。

### 10.2 覆盖解析（两维独立，核心）

- `product_id_mode=default, price_id_mode=default` → effective 全回退基准。
- `product_id_mode=default, price_id_mode=override` → IAP 商品 ID 回退基准、价格档取覆盖（验证两维独立，最易混淆场景）。
- `product_id_mode=override, price_id_mode=default` → 反向。
- `product_id_mode=override, price_id_mode=override` → 各取各的覆盖值。
- `mode=override` 但 override 空 → 写入被 `VALIDATION_FAILED` 拦截。
- `mode=default` 写入时强制清空 override 残值。
- `productIdOverride` 误填价格档值 / `priceIdOverride` 误填商品 SKU：长度与语义校验（长度边界 128 vs 64）。

### 10.3 IAP 配置与覆盖

- 模板四件套校验：缺必填→`invalid`；齐全→`valid`；空→`empty`。
- 复制创建清空 secret/file → 必须 `invalid` 且 `last_check_message` 含"缺少必填敏感字段或文件字段"，不得 `empty`。
- 密文响应脱敏、明文不落库。
- 包级覆盖 `enabled=true & valid` → merge 生效；`enabled=false` 或非 valid → 回退渠道实例配置。

### 10.4 env / 唯一键 / 权限 / 审计

- `(game_id, product_id)` 唯一性冲突（每环境 schema 内）→ `CONFLICT`。
- 包与商品必然落在同一环境 schema（= 同 env），同 schema 普通外键即保证一致，无需 env 一致性校验。
- 无 `product.write` 权限 → `FORBIDDEN`。
- 写操作产生 `audit_logs` 记录且密文脱敏。

---

## 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env（schema 隔离）：写落当前环境 schema、不允许跨 schema 写 / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/product.yaml`；前端 e2e：`tests/frontend/e2e/games.spec.ts`（商品/IAP 页签）。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET /api/admin/games/{gameId}/products | ✓ | ✓ | ✓ | — | — | — | — | — | ✓ | — | keyword/enabled 过滤、baseAmountDisplay 反算 |
| POST /api/admin/games/{gameId}/products | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | — | currency 归一化(CURRENCY_NOT_SUPPORTED)、base_amount_minor 写入流程 |
| PATCH /api/admin/products/{productId} | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ | — | — | — | productId 不可变、改币种/金额重走 currency 归一化、base_amount_minor 写入流程 |
| GET /api/admin/channel-packages/{packageId}/products | ✓ | ✓ | ✓ | — | — | — | — | — | — | — | override 解析(product_id/price_id default/override)、effective 两维独立派生 |
| PUT /api/admin/channel-packages/{packageId}/products | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | 全量 upsert+删除未出现项、override 解析(product_id/price_id default/override)、price_id 悬空软校验 |
| GET /api/admin/game-channels/{gameChannelId}/iap-config | ✓ | ✓ | ✓ | — | — | — | — | ✓ | — | — | 模板四件套渲染、密文/文件脱敏 |
| PUT /api/admin/game-channels/{gameChannelId}/iap-config | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ | ✓ | — | — | config_status 计算、复制清空 secret/file→invalid、与收银台支付路由隔离 |
| GET /api/admin/channel-packages/{packageId}/iap-override | ✓ | ✓ | ✓ | — | — | — | — | ✓ | — | — | 渠道基线对比、密文/文件脱敏 |
| PUT /api/admin/channel-packages/{packageId}/iap-override | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ | ✓ | — | — | 包级 merge 覆盖优先、enabled=false 回退渠道基线、与收银台支付路由隔离 |

前端：`games.spec.ts` 覆盖 商品列表/创建编辑抽屉、包级商品映射（两维 override 独立切换 + effective 预览）、IAP 配置面板（configStatus empty/invalid/valid 三态）、包级 IAP 覆盖面板（回退提示） / vitest 组件：金额输入受 currency_specs 约束与舍入预览、override 双列控件、模板渲染器脱敏字段。

---

## 11. 未决问题与假设（Open Questions & Assumptions）

1. **`price_id_override` / `priceId` 与价格行的强一致性**：本期不做"price_id 必须存在于某价格模板价格行"的强外键/存在性校验（价格档可能晚于商品创建）。假设由运营保证或在快照生成阶段（`snapshot`）做软校验告警。**待定**：是否在快照生成时对悬空 `price_id` 报警并阻断同步。
2. **包级映射 PUT 的删除语义**：本文假设 `PUT /channel-packages/{packageId}/products` 为"全量声明 + 删除未出现项"。若运营更希望"仅 upsert 不删除"，需改为带显式 `removed` 列表或单独 DELETE 接口。**待确认**。
3. **IAP 权限码粒度**：本期 IAP 配置写复用 `product.write`。若需要与商品写权限分离（如 IAP 由专人维护），应新增 `iap.write` 权限码。**待确认**。
4. **`base_amount` 入参形态**：同时支持 `baseAmountMinor`（整数）与 `baseAmount`（主单位字符串）两种，二者同时出现时以 `baseAmountMinor` 优先还是冲突报错，需统一约定。**当前假设**：同时出现则 `baseAmountMinor` 优先并对 `baseAmount` 做一致性校验，不一致 `VALIDATION_FAILED`。
5. **IAP 配置 merge 粒度**：§5.4 假设按 `config_json` 顶层字段覆盖式 merge。若模板存在嵌套对象需深合并，需在模板层声明合并策略。**待定**。
6. **`channel_products.enabled=false` 与商品 `products.enabled=false` 的关系**：假设两者均需为 true 才进入生效集合（任一为 false 即排除）。**待确认**是否需要"包内强制启用覆盖商品级禁用"的特例。
7. **商品改名/改 productId**：本文假设 `product_id` 不可变（身份键）。若业务需要改 SKU，需提供迁移/重建流程并处理已存在的包级映射引用。**待定**。
