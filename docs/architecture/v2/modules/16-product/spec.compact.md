---
id: product
code: "16"
title: 商品与 IAP 映射（Product / Channel IAP）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [channel, game, common]
code_paths:
  - services/admin-api/internal/domain/product
  - apps/admin-web/src/views/games
---

# 16 · 商品与 IAP 映射 — Compact Spec

> 代码生成用精简规格。完整背景/示例/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（env 模型 D1 §2、currency 归一化 §5、模板四件套 §4、ConfigStatus §3.4、密文/文件 §6、统一包络/错误码 §7、审计 §8、红线 §9）。

## 高风险辨析（务必牢记）
`product_id`（IAP 商品 ID，商店内购 SKU，列长 128）与 `price_id`（收银台价格档标识，列长 64，引用 `cashier_price_rows.price_id`）是**两个完全不同维度**，来源不同系统、覆盖逻辑独立、长度上限不同，任何接口/前端/解析逻辑必须分别处理，**禁止互填**。

## 边界
本模块三块互不混淆的子能力：
- **商品主数据** `products`：游戏维度逻辑商品基准定义（IAP 商品 ID 基准、商品名、基准金额、价格档基准、启用）。该游戏卖哪些商品的单一事实来源。
- **包级映射/覆盖** `channel_products`：渠道包 × 商品维度，描述某包对某商品的 `product_id`/`price_id` 是否覆盖（两组独立开关）。
- **渠道 IAP 配置** `channel_iap_templates`（平台级模板）/`game_channel_iap_configs`（渠道实例）/`channel_package_iap_overrides`（包级覆盖）：模板四件套驱动的渠道侧 IAP 参数。

红线：
- 渠道 IAP 配置 ≠ 收银台支付路由(`payment`)，数据/表/领域包分离，不混用（`00` §9）。
- `product_id` 不混 `price_id`；商品金额写入不绕过 `currency_specs`；IAP 密文不落明文、响应脱敏。
- 不在本模块：价格模板/价格行(`cashier-template`)、游戏级收银台绑定与价格覆盖(`game-cashier`)、支付路由(`payment`)、渠道包创建(`channel`，本模块只引用包不创建)。`price_id` 指向的实际金额属收银台域，本模块只存字符串引用。

## 通用列说明
所有表含 `id BIGSERIAL PK`、`created_at/updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`。业务表均为**每环境独立 schema、不带 env 列**，唯一键不前置 env（D1，`00` §2.2）；同 schema 普通外键即保证父子同 env，无需 env 一致性校验。金额列均为整数最小单位 minor，写入强制走 `00` §5 归一化。

## 数据模型

### products（商品主数据 / 每环境 schema）
| 列 | 类型 | 默认 | 约束/说明 |
| --- | --- | --- | --- |
| game_id_ref | BIGINT | — | NOT NULL REFERENCES games(id) |
| product_id | VARCHAR(128) | — | NOT NULL，IAP 商品 ID 基准（商店 SKU） |
| product_name | VARCHAR(128) | — | NOT NULL |
| base_amount_minor | BIGINT | — | NOT NULL，必须按 base_currency 归一化后写入 |
| base_currency | VARCHAR(8) | — | NOT NULL，须命中 currency_specs.currency_code 且 enabled |
| price_id | VARCHAR(64) | — | NOT NULL，收银台价格档基准（引用 cashier_price_rows.price_id） |
| enabled | BOOLEAN | TRUE | NOT NULL |

UNIQUE(game_id_ref, product_id)。`base_amount_minor + base_currency` 为基准定价（展示/对账/快照基线）；玩家实付金额由 price_id 指向价格行决定，二者关联但不强一致。

### channel_products（包级映射 / 覆盖 / 每环境 schema）
| 列 | 类型 | 默认 | 约束/说明 |
| --- | --- | --- | --- |
| product_id_ref | BIGINT | — | NOT NULL REFERENCES products(id) |
| package_id_ref | BIGINT | — | NOT NULL REFERENCES channel_packages(id) |
| product_id_mode | VARCHAR(16) | `default` | NOT NULL CHECK IN('default','override')，IAP 商品 ID 覆盖模式 |
| product_id_override | VARCHAR(128) | `''` | NOT NULL，仅 override 模式有效 |
| price_id_mode | VARCHAR(16) | `default` | NOT NULL CHECK IN('default','override')，价格档覆盖模式 |
| price_id_override | VARCHAR(64) | `''` | NOT NULL，仅 override 模式有效 |
| enabled | BOOLEAN | TRUE | NOT NULL，该商品在此包内是否启用 |

UNIQUE(package_id_ref, product_id_ref)。两组 mode+override **完全正交**：可只覆盖 product_id、只覆盖 price_id、都覆盖、都不覆盖，互不影响。v2 变更：`product_id_mode`/`price_id_mode` 补 `DEFAULT 'default'`。

### channel_iap_templates（渠道 IAP 模板 / 平台级 schema `platform` / 不带 env）
| 列 | 类型 | 默认 | 约束/说明 |
| --- | --- | --- | --- |
| channel_id_ref | BIGINT | — | NOT NULL REFERENCES channels(id) |
| template_version | VARCHAR(32) | — | NOT NULL；简单模板表（`00` §4.4.1），无 status 列，取 enabled 最新版本 |
| form_schema_json | JSONB | `[]` | 四件套：渲染字段 |
| secret_fields_json | JSONB | `[]` | 四件套：密文字段 |
| file_fields_json | JSONB | `[]` | 四件套：文件字段 |
| validation_rules_json | JSONB | `{}` | 四件套：校验规则 |
| enabled | BOOLEAN | TRUE | NOT NULL |

UNIQUE(channel_id_ref, template_version)（不前置 env，平台级共享）。模板内容由基础数据/模板管理后台维护，本模块只消费。

### game_channel_iap_configs（渠道 IAP 配置实例 / 每环境 schema）
| 列 | 类型 | 默认 | 约束/说明 |
| --- | --- | --- | --- |
| game_channel_id_ref | BIGINT | — | NOT NULL REFERENCES game_channels(id) |
| enabled | BOOLEAN | FALSE | NOT NULL，注意默认 false |
| config_json | JSONB | `{}` | 按模板四件套填写；密文位脱敏存储 |
| config_status | VARCHAR(16) | `empty` | NOT NULL CHECK IN('empty','invalid','valid') |
| last_check_at | TIMESTAMPTZ | NULL | 可空 |
| last_check_message | VARCHAR(255) | `''` | NOT NULL |

UNIQUE(game_channel_id_ref)。

### channel_package_iap_overrides（包级 IAP 覆盖实例 / 每环境 schema）
结构同 `game_channel_iap_configs`，外键改为 `package_id_ref BIGINT NOT NULL REFERENCES channel_packages(id)`，`enabled` 默认 FALSE，`config_status` 默认 empty，UNIQUE(package_id_ref)。

> 模板平台级全 env 共享一套定义；配置实例（config / override）各自独立，不共享 secret/file/状态（`00` §4.4）。实例遵循 `00` §3.4 ConfigStatus 状态机与 §6 密文/文件规则。

## 枚举与默认
| 枚举 | 取值 | 默认 | 落地列 |
| --- | --- | --- | --- |
| OverrideMode | default / override | default | channel_products.product_id_mode、price_id_mode |
| ConfigStatus | empty / invalid / valid | empty | game_channel_iap_configs.config_status、channel_package_iap_overrides.config_status |
| Environment | develop / sandbox / production | develop | 决定业务表落哪个环境 schema（不带 env 列） |
| RoundingMode（引用） | half_up / floor / ceil / truncate | half_up | 来自 currency_specs.rounding_mode |

默认值要点：products.enabled=TRUE；base_currency 无 DB 默认（业务建议 USD，必填且须命中 currency_specs）；base_amount_minor/price_id 必填无默认；channel_products 两组 mode=`default`、两个 override=`''`、enabled=TRUE；两类 IAP 实例 enabled=FALSE / config_status=empty / config_json=`{}` / last_check_message=`''`；写操作落当前运行环境 schema，前端不可指定/跨 schema 写。`channel_iap_templates` 为简单模板表，无 status，不走三态机，取 enabled 最新 template_version。

## 业务规则

### 金额归一化（`00` §5，不可绕过）
仅 `products.base_amount_minor` 涉及金额写入。写入/更新商品金额严格按序：
1. 读 base_currency 的 currency_specs；缺失或 enabled=false → `CURRENCY_NOT_SUPPORTED`(400)。
2. 按 decimal_places 校验/解析入参精度（入参支持主单位 amount 或整数 minor，统一转 minor）。
3. 按 min_amount_minor 校验下限，低于 → `VALIDATION_FAILED`。
4. 按 rounding_mode 归一化到该币种精度。5. 存为 base_amount_minor。

示例：USD(decimal=2,min=1,half_up) `4.999→500`、`0.001→拒绝(<min)`；JPY(decimal=0) `120.5→121`。归一化纯逻辑放 `internal/domain/common`(currency)，ProductService 调用，不在 transport 散落。

### 生效 product_id / price_id 解析（核心，两维独立）
```text
effectiveProductId =
    (product_id_mode == "override" && product_id_override != "")
        ? product_id_override          // 包级覆盖的 IAP 商品 ID
        : products.product_id          // 否则回退商品基准

effectivePriceId =
    (price_id_mode == "override" && price_id_override != "")
        ? price_id_override            // 包级覆盖的收银台价格档
        : products.price_id            // 否则回退商品基准
```
1. 两维完全独立，各自判断互不牵连。2. mode=default 一律回退基准（忽略 override 残值，写入时已清空）。3. mode=override 但 override 空属非法（写入被 VALIDATION_FAILED 拦截）；解析时防御性回退基准并记 warning。4. 结果用于快照按 market 输出、运行时客户端配置、sync diff。5. 解析不修改 products 基准，只读取/快照时计算派生值。

### 包级覆盖写入归一化（§写 channel_products）
- mode=default ⇒ 强制 override=''；mode=override ⇒ override 必填非空否则 `VALIDATION_FAILED`（product/price 两组各自适用）。
- product_id_ref 必须属于该包所在游戏（同 game/同 schema），否则 `VALIDATION_FAILED`/`CONFLICT`。
- price_id_override 是否存在于价格行：**本期不做强外键校验**（价格档可能后建），仅格式校验（关键假设）。

### IAP 配置与包级覆盖关系
```text
effectiveIapConfig(package) =
    package 有 enabled=true 的 override(config_status=valid)
        ? merge(baseChannelIapConfig, override.config_json)   // 包级字段优先
        : baseChannelIapConfig
其中 baseChannelIapConfig = 该 game_channel 的 game_channel_iap_configs(enabled=true 且 config_status=valid)
```
- 合并粒度：按 config_json 顶层字段覆盖式 merge（包级出现字段整体替换基线同名字段）。
- 任一侧 enabled=false 或 config_status!=valid → 不进快照/同步/客户端最终配置（`00` §9）。
- 模板版本切换不自动改写已存实例值，但触发实例重校验得到新 config_status。

### config_status 推导（`00` §3.4）
未填任何字段 → empty；已填但缺必填/缺 secret/缺 file 或校验未过 → invalid（last_check_message 给具体缺失）；全部必填（含 secret/file）齐全且校验通过 → valid。复制创建清空 secret/file 的实例**必须 invalid 不得 empty**，last_check_message 提示"缺少必填敏感字段或文件字段"。

### 与 env / 同步
所有业务表写操作落当前运行环境 schema，前端不可跨 schema 写。本模块 `SyncSection=products`；sync diff 比对解析后生效值与原始行，密文 masked。禁用/无效的商品、映射、IAP 配置不进快照/同步/客户端最终配置。

## 后端 API（前缀 /api/admin，包络 `00` §7；读 product.read / 写 product.write）
> 写权限：商品/映射用 `product.write`；渠道 IAP 配置/包级覆盖共用 `product.write`（或细分 iap.write，本期沿用）。审计 action：product.create/product.update/product.package_products.update/iap.config.update/iap.override.update，detail_json 记 before/after（密文脱敏）。

**GET `/games/{gameId}/products`**（列商品，product.read）
Query: page(默认1)/pageSize(默认20,max100)/sort(默认-updatedAt)/enabled(可选)/keyword(按 product_id/product_name 模糊)。
→ items[]: { id, env, gameId, productId, productName, baseAmountMinor, baseCurrency, baseAmountDisplay, priceId, enabled, createdAt, updatedAt }, page, pageSize, total。
> baseAmountDisplay = 服务端按 decimal_places 反算的主单位字符串（只读，非存储列）。

**POST `/games/{gameId}/products`**（创建，product.write）
| 字段 | 类型 | 必填 | 校验 |
| --- | --- | --- | --- |
| productId | string | 是 | 1–128；(gameId)内唯一→`CONFLICT` |
| productName | string | 是 | 1–128 |
| baseCurrency | string | 是 | 命中 currency_specs 且 enabled，否则 `CURRENCY_NOT_SUPPORTED` |
| baseAmountMinor | integer | 二选一 | 整数 minor，>= min_amount_minor |
| baseAmount | string/number | 二选一 | 主单位金额，服务端按 §金额归一化转 minor |
| priceId | string | 是 | 1–64 |
| enabled | boolean | 否 | 默认 true |

校验顺序：基础格式 → 唯一性 → 金额归一化。productId 与 priceId 分别校验不可互填。
→ 201 返回完整商品对象（同 GET item 结构）。
错误：`{ "error": { "code": "CURRENCY_NOT_SUPPORTED", "message": "currency 'ABC' is not in currency_specs", "details": [] } }`。

**PATCH `/products/{productId}`**（更新，product.write；定位 `(gameId, productId)`，建议带 `gameId` query 避歧义）
DTO（均可选）：productName、baseAmount/baseAmountMinor、baseCurrency、priceId、enabled。
- productId 不可变（身份键），传入忽略或 VALIDATION_FAILED。
- 改 baseAmount*/baseCurrency 任一 ⇒ 重走金额归一化全流程。priceId 单独校验长度。
→ 200 返回更新后完整商品对象。

**GET `/channel-packages/{packageId}/products`**（读包级映射，product.read）
返回该包所有 channel_products 联表 products 基准 + 解析后生效值。
→ items[]: { productId, productName, enabled, base:{ productId, priceId, baseAmountMinor, baseCurrency }, productIdMode, productIdOverride, priceIdMode, priceIdOverride, effective:{ productId, priceId } }。
> effective.productId 可回退基准而 effective.priceId 取覆盖值，体现两维独立解析。

**PUT `/channel-packages/{packageId}/products`**（整表覆盖写，product.write）
语义：items 全量声明该包映射，**全量 upsert + 删除未出现项**，写审计。
| 字段 | 类型 | 必填 | 校验 |
| --- | --- | --- | --- |
| productId | string | 是 | 须为该包所属 game 下已存在商品，否则 `VALIDATION_FAILED` |
| enabled | boolean | 否 | 默认 true |
| productIdMode | enum | 否 | default/override，默认 default |
| productIdOverride | string | 条件 | mode=override 时必填非空(≤128)，否则强制 '' |
| priceIdMode | enum | 否 | default/override，默认 default |
| priceIdOverride | string | 条件 | mode=override 时必填非空(≤64)，否则强制 '' |

两组 mode/override 配套校验；两 override 各按长度上限校验不可混；items 内 productId 不可重复。
→ 200 返回同 GET 结构（含 effective）。
错误（override 缺值）：`{ "error": { "code": "VALIDATION_FAILED", "message": "priceIdOverride is required when priceIdMode=override", "details": [{ "field": "items[0].priceIdOverride" }] } }`。

**GET `/game-channels/{gameChannelId}/iap-config`**（读渠道 IAP 配置，product.read）
返回配置实例 + 该渠道 enabled 最新 template_version 四件套，密文脱敏。
→ { gameChannelId, channelId, template:{ templateVersion, formSchema[], secretFields[], fileFields[], validationRules{} }, config:{ enabled, configStatus, configJson(脱敏), lastCheckAt, lastCheckMessage } }。
formSchema 项: { key, label, component, required, order }；fileFields 项: { key, accept[], maxSizeKB }。

**PUT `/game-channels/{gameChannelId}/iap-config`**（写渠道 IAP 配置，product.write）
| 字段 | 类型 | 必填 | 校验 |
| --- | --- | --- | --- |
| enabled | boolean | 否 | 默认 false |
| configJson | object | 是 | 按渠道 IAP 模板四件套校验（必填/类型/pattern/format） |

服务端：取渠道 enabled 最新模板校验 configJson → secret 加密落库/响应脱敏(`00` §6.1)、file 存引用(§6.2) → 计算 config_status、写 last_check_at/message。
→ 200 返回 config 部分（脱敏，configStatus 重算）。
错误（缺必填敏感/文件字段）：`{ "error": { "code": "VALIDATION_FAILED", "message": "缺少必填敏感字段或文件字段", "details": [{ "field": "configJson.privateKey", "reason": "required_file_missing" }] } }`。
> 显式 PUT 缺必填 → 返回 VALIDATION_FAILED 且不改 enabled=true；复制创建产生的实例由复制流程直接落 invalid（`00` §3.4）。

**GET `/channel-packages/{packageId}/iap-override`**（读包级 IAP 覆盖，product.read）
返回 override 实例 + 模板四件套 + 渠道实例基线（baseConfig）供对比。
→ { packageId, packageCode, channelId, template{…}, baseConfig:{ enabled, configStatus }, override:{ enabled, configStatus, configJson, lastCheckAt, lastCheckMessage } }。

**PUT `/channel-packages/{packageId}/iap-override`**（写包级 IAP 覆盖，product.write）
DTO 同 iap-config 写（enabled + configJson），同一模板四件套校验、密文/文件处理、config_status 计算。包级字段优先于渠道基线（§merge）；enabled=false 时覆盖不生效，运行时回退渠道实例配置。
→ 200 返回 override 部分（脱敏，configStatus 重算）。

错误码：`CURRENCY_NOT_SUPPORTED`(400)、`VALIDATION_FAILED`、`CONFLICT`(唯一标识冲突)、`NOT_FOUND`、`FORBIDDEN`(无 product.write)。

## 应用服务 / 仓储（internal/app command+query）
- **ProductService**：商品 CRUD（调 common/currency 归一化）、包级映射读写（写入归一化 + 生效解析）、唯一性/外键校验。
  - 方法示意：`ListProducts(ctx, env, gameId, page, filters)`、`CreateProduct/UpdateProduct(...)`、`GetPackageProducts(ctx, env, packageId)`(含 effective)、`PutPackageProducts(ctx, env, packageId, items)`(全量 upsert + 删未出现项)。
  - 仓储：`ProductRepository`(products CRUD + by game)、`ChannelProductRepository`(包级 upsert/delete by package)、`ChannelPackageRepository`(只读校验包归属)、`CurrencySpecRepository`。
- **IAPConfigService**：渠道 IAP 配置 + 包级覆盖读写、模板四件套校验、密文加解密(infra/crypto)、文件引用(infra/file)、config_status 计算、生效 IAP merge 解析。
  - 仓储：`GameChannelIapConfigRepository`、`ChannelPackageIapOverrideRepository`、`ChannelIapTemplateRepository`(读 enabled 最新模板)、`CryptoService`、`FileService`。
- 业务表仓储 SQL 不写 schema 前缀、不带 env 谓词（search_path 决定）。快照合并由 snapshot 域消费生效解析结果，sync diff 由 sync 域调用，两服务不越界大编排。

## 前端要点（游戏详情"商品/IAP"Tab + 渠道包详情抽屉；Element Plus + Pinia）
- **商品列表/编辑**：游戏详情→"商品"Tab，列 productId(标注"IAP 商品 ID")/productName/基准金额(baseAmountDisplay+baseCurrency)/priceId(标注"收银台价格档(price_id)")/enabled/更新时间。编辑抽屉对应 POST/PATCH DTO。前端预判 productId≤128、priceId≤64，币种下拉来自 dictionary store。
- **包级商品映射编辑**：渠道包详情→"商品映射"面板。每行两组覆盖控件**视觉分两列**：「IAP 商品 ID」组(productIdMode+productIdOverride+展示 base.productId)、「收银台价格档」组(priceIdMode+priceIdOverride+展示 base.priceId)。实时显示 effective 两个派生值。强约束：mode=default 禁用并清空对应 override；切 override 后必填，空值阻止提交；两组开关互不联动。防混淆 placeholder/样式区分两输入框。
- **IAP 配置/覆盖面板**：消费模板四件套用统一模板渲染器渲染 configJson；密文脱敏可重填、文件走统一上传；configStatus(empty/invalid/valid)行内显示，invalid 用告警色并展示 lastCheckMessage 不得隐藏。包级覆盖面板展示渠道基线(只读对比)+覆盖开关+覆盖表单，标注"未启用时回退渠道配置"。IAP 与收银台/支付路由面板分属不同入口，UI 不混排（红线）。
- **金额输入受 currency_specs 约束**：选定 baseCurrency 后限制小数位=decimal_places、最小值提示=min_amount_minor 换算、失焦按 rounding_mode 预览"将存储为 X minor"；仅前端提示，最终以后端归一化为准。

## 与公共能力 / 下游
- env(`00` §2 D1)：四张业务表每环境 schema 不带 env，唯一键不前置 env；channel_iap_templates 平台级。currency(§5)：products.base_amount_minor 强制归一化。模板四件套(§4)：channel_iap_templates 驱动 IAP 表单。ConfigStatus(§3.4)：IAP 配置/覆盖。密文/文件(§6)：configJson 加密落库/脱敏/文件存引用。包络/错误码(§7)、审计(§8)。
- snapshot：消费 §生效解析(product_id/price_id 派生 + IAP merge)按 market 输出，禁用/无效不进快照。sync：SyncSection=products，diff 比对生效值，密文 masked。
- channel：引用 game_channels/channel_packages，不创建包。cashier-template/game-cashier：price_id 指向 cashier_price_rows.price_id，只存引用。

## 关键假设
- price_id_override/priceId 与价格行**不做强外键校验**（价格档可能后建），仅格式校验；悬空检测/告警拟在快照阶段软校验（是否阻断同步待定）。
- 包级映射 PUT 采用"全量声明 + 删除未出现项"语义（若改为仅 upsert 需带 removed 列表或单独 DELETE）。
- IAP 配置写本期复用 product.write（如需分离再加 iap.write）。
- baseAmountMinor 与 baseAmount 同时出现时以 baseAmountMinor 优先并对 baseAmount 做一致性校验，不一致 VALIDATION_FAILED。
- IAP merge 按 config_json 顶层字段覆盖（嵌套深合并需模板声明策略）。
- 进入生效集合要求 products.enabled 与 channel_products.enabled 均为 true（任一 false 即排除）。
- product_id 不可变（身份键）；改 SKU 需迁移/重建并处理已存包级映射引用。
