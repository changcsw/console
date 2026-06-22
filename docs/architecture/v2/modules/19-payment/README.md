---
id: payment
code: "19"
title: 支付路由（Payment Routing）
status: target
code_paths:
  - services/admin-api/internal/domain/payment
  - apps/admin-web/src/views/payment
depends_on: [channel, product, cashier-template, game-cashier, game, common]
impacts: [snapshot, sync, testing]
children: []
---

# 19 · 支付路由（Payment Routing）

> 本模块文档默认遵循 `../../00-common.md`（以下简称 `00`）与 `../../01-structure.md`（以下简称 `01`）的全部公共契约：env 模型（D1）、统一 API 包络、统一错误码（含 `ROUTE_CONFLICT`）、密文脱敏、审计、权限码、命名规范等。本文只在公共契约之上**追加**支付路由模块的私有约定；与 `00` 冲突时以 `00` 为准。
>
> 本模块对应后端领域包 `internal/domain/payment`、应用服务 `PaymentRouteService`，前端 `/payment` 路由分组，落地表 `pay_ways`、`cashier_providers`、`cashier_provider_templates`、`billing_subjects`、`cashier_merchant_accounts`、`payment_routes`。
>
> **本模块要解决的核心业务诉求：让"信用卡支付"这一玩家可见支付方式，可以在不改变客户端、不改变玩家体验的前提下，从一个真实 PSP（如 Airwallex）无感切换到另一个 PSP（如 PayerMax）。** 实现手段是把"玩家可见支付方式"与"真实清算通道（商户账户）"解耦，并用一张可排序、可通配、可校验唯一性的 `payment_routes` 表把二者按游戏维度连接起来。

---

## 1. 模块边界（Bounded Context）

支付路由模块横跨"平台级基础数据"与"游戏级业务数据"两层。务必先把以下 5 个概念在语义上彻底分开，它们是本模块所有规则的地基。

### 1.1 五个核心概念逐一界定

| 概念 | 表 | env 维度 | 语义 | 谁能看见 |
| --- | --- | --- | --- | --- |
| **支付方式 `pay_way`** | `pay_ways` | 平台级，**不带 env** | **玩家在收银台看到的支付选项**，如"信用卡 / PayPal / GCash / 支付宝"。是面向玩家的展示与归类单位，不含任何密钥与清算细节。 | 玩家可见（经路由解析后） |
| **支付提供商 `provider`** | `cashier_providers` | 平台级，**不带 env** | **真实 PSP（Payment Service Provider）**，如 Airwallex、PayerMax、Stripe。是技术对接对象，决定走哪套 SDK/网关协议。玩家**不感知** provider 名称。 | 仅后台可见 |
| **公司主体 `billing_subject`** | `billing_subjects` | 平台级，**不带 env** | **签约/清算法律实体**，如"XX 香港有限公司""XX 新加坡 Pte Ltd"。决定资金最终归集到哪个法人、哪个发票主体。 | 仅后台可见 |
| **商户账户 `merchant_account`** | `cashier_merchant_accounts` | 平台级，**不带 env** | **某个主体在某个 provider 下开立的具体商户号 + 接入密钥**。是真正可用于发起交易/清算的最小可执行单元，`secret_ciphertext` 密文存储。 | 仅后台可见，密钥脱敏 |
| **支付路由 `payment_route`** | `payment_routes` | **游戏级，带 env（D1）** | **按"游戏 + 支付方式"维度，把一组匹配条件（选择器）映射到一个 `(provider, merchant_account)` 的优先级规则。** 是连接"玩家可见支付方式"与"真实清算通道"的桥梁。 | 后台维护，结果进入客户端运行时配置 |

### 1.2 概念之间的连接关系

```text
玩家视角:            credit_card (pay_way)
                          │
                          │  payment_routes（游戏级 + env）
                          │  选择器: package/channel/market/country/currency + priority
                          ▼
后台视角:     ┌──────────────────────────────────────────┐
              │ provider (Airwallex)  ──  merchant_account │
              │                            （含 subject + 密钥） │
              └──────────────────────────────────────────┘
```

一句话：**`pay_way` 是"玩家点的按钮"，`merchant_account` 是"钱实际从哪条通道走"，`payment_route` 是"在什么条件下，这个按钮该走哪条通道"。**

### 1.3 红线与不可混淆项

- **不把 `pay_way` 与 `provider` 混为一谈。** 一个 `pay_way=credit_card` 可以由多个 provider 承载；切换 provider 对玩家无感，这正是本模块的价值。
- **不把"渠道 IAP 配置（`product`）"与"收银台支付路由（本模块）"混在一起**（`00` §9 红线、`backend_agent_execution.md` 阶段 9）。渠道 IAP 走渠道自身计费（Google/Apple/华为内购），支付路由走自有收银台/PSP 清算，二者数据、表、领域包均独立。
- **不把"渠道 `channel`"与"提供商 `provider`"混为一谈。** `channel` 是发行/分发渠道（Google Play、华为），`provider` 是支付清算服务商。路由选择器里的 `channel` 仅用于"在某渠道包场景下走哪个 PSP"的细分，不等于 PSP。
- **路由匹配是纯函数。** 匹配/排序/唯一性判定必须是不依赖 IO 的纯领域逻辑（`internal/domain/payment`），便于单测与复用（`01` §4 分层）。
- **平台级基础数据全 env 共享。** `pay_ways/cashier_providers/cashier_provider_templates/billing_subjects/cashier_merchant_accounts` 不带 env（`00` §2.2、D1 备注）；只有 `payment_routes` 带 env。

---

## 2. 领域模型（Domain Model）

领域包：`services/admin-api/internal/domain/payment`。核心聚合 `PaymentRouting`，配套若干值对象与纯规则函数（`route_matcher.go` / `route_validator.go`，见 `01` 与实现计划 Task 3）。

### 2.1 聚合根：PaymentRouting

`PaymentRouting` 聚合按 **`(env, gameID, payWayID)`** 为一致性边界，持有该游戏该支付方式下的全部路由条目集合。所有"唯一性校验、优先级冲突检测、最佳路由选择"都在聚合边界内完成。

```go
// 概念形态（非最终实现）
type PaymentRouting struct {
    Env      common.Environment // develop/sandbox/production
    GameID   string             // 业务游戏号
    PayWayID string             // 玩家可见支付方式，如 credit_card
    Routes   []Route            // 同 pay_way 下的所有路由条目
}
```

> 说明：跨 `pay_way` 的唯一性互不影响——`credit_card` 与 `paypal` 的 priority 可以重复，因为它们是不同支付方式的独立优先级链路。唯一性边界严格是"同游戏 + 同 pay_way + 同 env"。

### 2.2 值对象：Route（路由条目）

`Route` 是不可变值对象，描述"一条匹配规则 + 命中后的目标通道"。

```go
type Route struct {
    ID               int64  // 物理行 id（持久层用）
    Env              string // 冗余以便校验一致性
    GameID           string

    // —— 选择器（5 维匹配条件，空 = "*" 通配）——
    Package  string // 渠道包 code，"" => "*"
    Channel  string // 渠道 channel_id，"" => "*"
    Market   string // GLOBAL/JP/KR/SEA/HMT/CN，"" => "*"
    Country  string // ISO 国家码，"" => "*"
    Currency string // 币种码，"" => "*"

    // —— 路由目标 ——
    PayWay          string // 玩家可见支付方式（聚合维度，冗余）
    Provider        string // 真实 PSP
    MerchantAccount string // 具体商户账户（含 subject + 密钥引用）

    // —— 排序与状态 ——
    Priority int  // 默认 100，越小越优先
    Enabled  bool // 仅 enabled=true 参与生效集合与唯一性校验
}
```

### 2.3 值对象：MatchInput（运行时匹配输入）

客户端/快照生成时给定一次具体支付场景，求"该走哪条通道"。

```go
type MatchInput struct {
    PayWay   string // 必填，玩家选择的支付方式
    Package  string // 当前渠道包，可空
    Channel  string // 当前渠道，可空
    Market   string // 当前发行大区，可空（建议必填以正确触发 GLOBAL 兜底）
    Country  string // 玩家所在国家，可空
    Currency string // 结算币种，可空
}
```

### 2.4 归一化（Normalization）

归一化是把"空字段"统一折叠为字面量 `"*"`，作为匹配、排序、唯一性比较的统一前置步骤。**归一化是纯函数，无副作用。**

```go
func normalize(v string) string {
    if v == "" {
        return "*"
    }
    return v
}
```

- DB 层 `payment_routes` 中 `market_code/country_code/currency` 默认值即 `'*'`（见 §3），而 `channel_id_ref/package_id_ref` 用 **NULL 表示通配 `*`**（外键不能存字面量 `"*"`）。
- 应用层把 NULL 外键映射为领域层的 `""`，再经 `normalize` 折叠为 `"*"`。即：DB NULL ⇔ 领域 `""` ⇔ 归一化 `"*"`，三态等价。

### 2.5 匹配纯规则的职责划分

| 文件 | 纯函数 | 职责 |
| --- | --- | --- |
| `route_matcher.go` | `MatchedRoutes(routes, input) []Route` | 过滤出所有"候选命中"路由（§5.1） |
| `route_matcher.go` | `PickBestRoute(routes, input) (Route, error)` | 在候选中按排序规则选出唯一最佳（§5.2） |
| `route_validator.go` | `ValidateRouteSet(routes) error` | 同 pay_way 下 priority 唯一 + 归一化选择器唯一（§5.4），冲突返回 `ROUTE_CONFLICT` |
| `route_matcher.go` | `marketMatches(routeMarket, inputMarket) bool` | market 语义匹配（§5 表） |

---

## 3. 数据模型（Data Model）

逐表逐字段说明。**带 env 的仅 `payment_routes`（D1）；其余 5 张均为平台级基础数据表，不带 env，全环境共享一套。**

### 3.1 `pay_ways`（支付方式，平台级，不带 env）

玩家可见支付方式主数据。真实列来自 `migrations/000001_init.up.sql`。

| 列 | 类型 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | 物理主键 |
| `pay_way_id` | VARCHAR(64) | — | `UNIQUE` | 业务标识，如 `credit_card`、`paypal`、`gcash` |
| `pay_way_name` | VARCHAR(64) | — | NOT NULL | 展示名，如"信用卡" |
| `pay_way_type` | VARCHAR(32) | — | `CHECK IN ('card','wallet','platform','local')` | 支付方式类型，见 §4 `PayWayType` |
| `enabled` | BOOLEAN | `TRUE` | NOT NULL | 是否启用 |
| `sort` | INT | `0` | NOT NULL | 列表排序 |
| `created_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | |
| `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | |

唯一键：`UNIQUE (pay_way_id)`（平台级，不前置 env）。

### 3.2 `cashier_providers`（支付提供商/真实 PSP，平台级，不带 env）

| 列 | 类型 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | |
| `provider_id` | VARCHAR(64) | — | `UNIQUE` | 业务标识，如 `airwallex`、`payermax`、`stripe` |
| `provider_name` | VARCHAR(64) | — | NOT NULL | 展示名 |
| `provider_kind` | VARCHAR(32) | — | `CHECK IN ('aggregator','gateway','wallet_direct')` | 提供商类型，见 §4 `ProviderKind` |
| `enabled` | BOOLEAN | `TRUE` | NOT NULL | |
| `sort` | INT | `0` | NOT NULL | |
| `created_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | |
| `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | |

唯一键：`UNIQUE (provider_id)`。

### 3.3 `cashier_provider_templates`（提供商模板四件套，平台级，不带 env）

模板驱动表单定义，语义遵循 `00` §4 模板四件套。用于商户账户配置表单渲染与前后端校验。受 `00` §3.3 版本生命周期约束。

| 列 | 类型 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | |
| `provider_id_ref` | BIGINT | — | `REFERENCES cashier_providers(id)` | 所属 provider |
| `template_version` | VARCHAR(32) | — | NOT NULL | 模板版本号 |
| `form_schema_json` | JSONB | `[]` | NOT NULL | 渲染字段定义（`00` §4.1） |
| `secret_fields_json` | JSONB | `[]` | NOT NULL | 密文字段清单（`00` §4.2/§6.1） |
| `file_fields_json` | JSONB | `[]` | NOT NULL | 文件字段清单 |
| `validation_rules_json` | JSONB | `{}` | NOT NULL | 校验规则（`00` §4.3） |
| `enabled` | BOOLEAN | `TRUE` | NOT NULL | |
| `created_at` / `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | |

唯一键：`UNIQUE (provider_id_ref, template_version)`。

### 3.4 `billing_subjects`（公司主体，平台级，不带 env）

| 列 | 类型 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | |
| `subject_id` | VARCHAR(64) | — | `UNIQUE` | 业务标识，如 `hk_entity` |
| `subject_name` | VARCHAR(128) | — | NOT NULL | 主体展示名 |
| `legal_entity_name` | VARCHAR(255) | — | NOT NULL | 法律实体全称（发票/合同用） |
| `enabled` | BOOLEAN | `TRUE` | NOT NULL | |
| `created_at` / `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | |

唯一键：`UNIQUE (subject_id)`。

### 3.5 `cashier_merchant_accounts`（商户账户，平台级，不带 env，密文）

某主体在某 provider 下的具体商户账户。**`secret_ciphertext` 为密文字段，落库前加密、响应脱敏（`00` §6.1，D1 备注：支付密钥属平台级基础数据，全 env 共享；本期不分环境密钥）。**

| 列 | 类型 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | |
| `merchant_account_id` | VARCHAR(64) | — | `UNIQUE` | 业务标识，如 `merchant_aw_main` |
| `provider_id_ref` | BIGINT | — | `REFERENCES cashier_providers(id)` | 归属 provider |
| `subject_id_ref` | BIGINT | — | `REFERENCES billing_subjects(id)` | 归属主体 |
| `merchant_id` | VARCHAR(128) | — | NOT NULL | PSP 侧商户号 |
| `merchant_name` | VARCHAR(128) | — | NOT NULL | 商户展示名 |
| `config_json` | JSONB | `{}` | NOT NULL | 非敏感接入参数（按 provider 模板渲染） |
| `secret_ciphertext` | TEXT | — | NOT NULL | **密文**：密钥/证书等敏感接入凭证，AES-GCM 加密（`01` `infra/crypto`） |
| `enabled` | BOOLEAN | `TRUE` | NOT NULL | |
| `created_at` / `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | |

唯一键：`UNIQUE (merchant_account_id)`。

> 强约束：任何响应中 `secret_ciphertext` 及 `config_json` 内被 `secret_fields_json` 标记的字段一律返回 `"masked"`，绝不回明文（`00` §6.1）；同步预览中这些字段 `masked=true`。

### 3.6 `payment_routes`（支付路由，**游戏级，带 env，D1**）

这是本模块唯一带 env 的表。**按 D1 锁定决策：新增 `env` 列，且所有唯一约束前置 `env`。**

> 现状（`000001_init.up.sql`）：`payment_routes` **没有 env 列、没有任何唯一约束**。v2 需新增迁移补齐 env 列与唯一约束（不改历史迁移，追加新文件，见 §3.7）。

逐字段（v2 目标形态）：

| 列 | 类型 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | |
| `env` | VARCHAR(16) | — | `NOT NULL CHECK (env IN ('develop','sandbox','production'))` | **D1 新增**：环境维度 |
| `game_id_ref` | BIGINT | — | `REFERENCES games(id)` | 所属游戏（被引用行 env 必须一致，应用层保证，`00` §2.2） |
| `market_code` | VARCHAR(32) | `'*'` | NOT NULL | 选择器：发行大区，`'*'` 通配 |
| `country_code` | VARCHAR(8) | `'*'` | NOT NULL | 选择器：国家码，`'*'` 通配 |
| `currency` | VARCHAR(8) | `'*'` | NOT NULL | 选择器：币种，`'*'` 通配 |
| `channel_id_ref` | BIGINT | `NULL` | `REFERENCES channels(id)`，**可空** | 选择器：渠道，**NULL 表示 `*` 通配** |
| `package_id_ref` | BIGINT | `NULL` | `REFERENCES channel_packages(id)`，**可空** | 选择器：渠道包，**NULL 表示 `*` 通配** |
| `pay_way_id_ref` | BIGINT | — | `REFERENCES pay_ways(id)` | 玩家可见支付方式（唯一性聚合维度） |
| `provider_id_ref` | BIGINT | — | `REFERENCES cashier_providers(id)` | 路由目标：真实 PSP |
| `merchant_account_id_ref` | BIGINT | — | `REFERENCES cashier_merchant_accounts(id)` | 路由目标：商户账户 |
| `priority` | INT | `100` | NOT NULL | 优先级，**越小越优先**（`00` §10） |
| `enabled` | BOOLEAN | `TRUE` | NOT NULL | 仅 `TRUE` 参与生效集合与唯一性校验 |
| `created_at` / `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | |

字段约定要点：
- `channel_id_ref` / `package_id_ref` 可空表示 `*`；`market_code/country_code/currency` 默认 `'*'`（不可 NULL，用字面量）。这是因为前两者是外键引用业务/基础数据 id，无法存字面量，只能用 NULL 编码通配。
- `priority` 默认 `100`，数值越小优先级越高（与 `00` §10 一致）。
- `provider_id_ref` 与 `merchant_account_id_ref` 必须自洽：所选 `merchant_account` 的 `provider_id_ref` 必须等于路由的 `provider_id_ref`（应用层校验，§5.5）。

### 3.7 v2 迁移：env 列与唯一性约束（D1 前置 env）

新增迁移文件（示意，遵循 `01` §6 不改历史迁移、追加新文件、幂等）：

```sql
-- 000003_payment_routes_env.up.sql（示意）

-- 1) 增加 env 列；存量行回填为当前运行环境（约定回填 'develop'，由运维确认）
ALTER TABLE payment_routes
  ADD COLUMN IF NOT EXISTS env VARCHAR(16) NOT NULL DEFAULT 'develop'
  CHECK (env IN ('develop','sandbox','production'));

-- 2) DB 层硬约束：同 env+game+pay_way 下 priority 不重复（仅约束生效行靠应用层补充，见说明）
--    注意：DB 唯一索引无法直接表达"仅 enabled=true"，故用部分唯一索引 WHERE enabled。
CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_routes_priority
  ON payment_routes (env, game_id_ref, pay_way_id_ref, priority)
  WHERE enabled;

-- 3) 归一化选择器唯一：channel/package 用 COALESCE(-1) 折叠 NULL，market/country/currency 已是字面量 '*'
CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_routes_selector
  ON payment_routes (
    env, game_id_ref, pay_way_id_ref,
    COALESCE(package_id_ref, -1),
    COALESCE(channel_id_ref, -1),
    market_code, country_code, currency
  )
  WHERE enabled;
```

> 说明：DB 部分唯一索引提供"兜底硬约束"，但**权威唯一性校验仍在应用层**（`ValidateRouteSet`，§5.4），原因：① 唯一性只针对 `enabled=true` 的生效行，部分索引可表达但需谨慎；② 应用层能返回带明确冲突信息的 `ROUTE_CONFLICT`（HTTP 409），优于裸 DB 唯一约束错误。两层并存，应用层先校验、DB 索引兜底。

---

## 4. 枚举与默认值清单

> 与 `00` §3 全局枚举保持一致；本节穷尽本模块涉及的枚举、默认值与 seed 清单。

### 4.1 枚举

| 枚举 | 取值 | 默认值 | 出处 |
| --- | --- | --- | --- |
| `PayWayType` | `card` / `wallet` / `platform` / `local` | **无默认**（建表必填） | `pay_ways.pay_way_type`，`00` §3.1 |
| `ProviderKind` | `aggregator` / `gateway` / `wallet_direct` | **无默认** | `cashier_providers.provider_kind`，`00` §3.1 |
| `Market`（选择器取值域） | `GLOBAL` / `JP` / `KR` / `SEA` / `HMT` / `CN` / `*` | 路由 `market_code` 默认 `'*'` | `00` §3.1 + 通配 `*` |
| `Environment` | `develop` / `sandbox` / `production` | `develop`（当前运行环境） | `payment_routes.env`，`00` §2.1 |

`PayWayType` 语义：
- `card`：卡类支付（信用卡/借记卡），如 `credit_card`。**本模块"信用卡无感切 PSP"的主战场。**
- `wallet`：电子钱包，如 PayPal、GCash、支付宝。
- `platform`：平台级支付聚合通道。
- `local`：本地化支付方式（如各国本地银行转账/便利店支付）。

`ProviderKind` 语义：
- `aggregator`：聚合型 PSP（一个对接覆盖多支付方式/多国），如 PayerMax、Airwallex。
- `gateway`：单一网关型 PSP。
- `wallet_direct`：直连钱包通道。

### 4.2 默认值清单（穷尽）

| 字段 | 默认值 | 来源 |
| --- | --- | --- |
| `payment_routes.market_code` | `'*'` | DDL + `00` §10 通配默认 |
| `payment_routes.country_code` | `'*'` | DDL |
| `payment_routes.currency` | `'*'` | DDL |
| `payment_routes.channel_id_ref` | `NULL`（= `*`） | DDL 可空 |
| `payment_routes.package_id_ref` | `NULL`（= `*`） | DDL 可空 |
| `payment_routes.priority` | `100`（越小越优先） | DDL + `00` §10 |
| `payment_routes.enabled` | `TRUE` | DDL + `00` §10 |
| `payment_routes.env` | 当前运行环境（迁移回填 `develop`） | D1 + `00` §10 |
| `*.enabled`（5 张平台表） | `TRUE` | DDL + `00` §10 |
| `pay_ways.sort` / `cashier_providers.sort` | `0` | DDL |
| `cashier_merchant_accounts.config_json` | `{}` | DDL + `00` §10 |
| `cashier_provider_templates.form_schema_json` / `secret_fields_json` / `file_fields_json` | `[]` | DDL + `00` §4 |
| `cashier_provider_templates.validation_rules_json` | `{}` | DDL + `00` §4 |
| `created_at` / `updated_at`（全表） | `NOW()` | DDL + `00` §10 |

### 4.3 seed 清单（基础数据，幂等 `ON CONFLICT DO NOTHING`，`01` §6）

> 以下为建议 seed，覆盖"信用卡无感切 PSP"的最小可用集合。具体值以基础数据后台维护为准。

`pay_ways` seed（示例）：

```text
credit_card  信用卡    type=card     enabled=true  sort=10
paypal       PayPal    type=wallet   enabled=true  sort=20
gcash        GCash     type=wallet   enabled=true  sort=30
alipay       支付宝     type=wallet   enabled=true  sort=40
```

`cashier_providers` seed（示例）：

```text
airwallex   Airwallex   kind=aggregator   enabled=true  sort=10
payermax    PayerMax    kind=aggregator   enabled=true  sort=20
stripe      Stripe      kind=gateway      enabled=true  sort=30
```

`billing_subjects` seed（示例）：

```text
hk_entity   XX 香港有限公司   legal=XX (HK) Co., Limited      enabled=true
sg_entity   XX 新加坡主体     legal=XX Singapore Pte. Ltd.    enabled=true
```

`cashier_merchant_accounts` seed（示例，密钥占位由运维补齐，secret 不入 seed 明文）：

```text
merchant_aw_main   provider=airwallex  subject=hk_entity   merchant_id=AW-001   secret=<encrypted>
merchant_pm_main   provider=payermax   subject=sg_entity   merchant_id=PM-001   secret=<encrypted>
```

`cashier_provider_templates`：每个 provider 至少一套 `published` 模板版本（四件套），用于商户账户表单渲染。

---

## 5. 业务规则与算法

本节是模块核心。给出完整匹配算法伪代码、market 语义表、排序决策步骤、唯一性归一化键、冲突校验。所有规则严格对齐 `docs/superpowers/specs/2026-06-16-market-channel-sync-design.md`「支付路由匹配与唯一性规则」全文。

### 5.1 候选命中规则（MatchedRoutes）

给定 `MatchInput`，一条 `Route` 成为"候选"当且仅当**同时满足**：

1. `route.PayWay == input.PayWay`（pay_way 必须精确相等，不通配）。
2. `route.Enabled == true`（隐藏/禁用不参与）。
3. `package` 命中：`norm(route.Package) == "*"` 或 `route.Package == input.Package`。
4. `channel` 命中：`norm(route.Channel) == "*"` 或 `route.Channel == input.Channel`。
5. `market` 命中：按 §5.2 market 语义表 `marketMatches(route.Market, input.Market)` 为真。
6. `country` 命中：`norm(route.Country) == "*"` 或 `route.Country == input.Country`。
7. `currency` 命中：`norm(route.Currency) == "*"` 或 `route.Currency == input.Currency`。

### 5.2 Market 匹配语义表（marketMatches）

| 目标 market（input.Market） | 允许命中的 route.Market 取值 | 不允许 |
| --- | --- | --- |
| `CN` | `CN`、`*` | `GLOBAL`、`JP/KR/SEA/HMT` |
| `JP` | `JP`、`GLOBAL`、`*` | `CN`、其他具体海外 |
| `KR` | `KR`、`GLOBAL`、`*` | `CN`、其他具体海外 |
| `SEA` | `SEA`、`GLOBAL`、`*` | `CN`、其他具体海外 |
| `HMT` | `HMT`、`GLOBAL`、`*` | `CN`、其他具体海外 |
| `GLOBAL` | `GLOBAL`、`*` | `CN`、`JP/KR/SEA/HMT` |

语义口诀（与 spec / `00` §3.2 一致）：
- **CN 只匹配 CN 或 `*`**（GLOBAL 永不兜底 CN）。
- **JP/KR/SEA/HMT 匹配 具体自身 或 GLOBAL 或 `*`**（GLOBAL 作为海外默认兜底）。
- **GLOBAL 只匹配 GLOBAL 或 `*`**。

```go
func marketMatches(routeMarket, inputMarket string) bool {
    rm := normalize(routeMarket) // 空 => "*"
    if rm == "*" {
        return true // 通配匹配任何 market
    }
    if inputMarket == "CN" {
        return rm == "CN"
    }
    // input 为 JP/KR/SEA/HMT：允许自身或 GLOBAL 兜底
    if inputMarket == "JP" || inputMarket == "KR" ||
        inputMarket == "SEA" || inputMarket == "HMT" {
        return rm == inputMarket || rm == "GLOBAL"
    }
    if inputMarket == "GLOBAL" {
        return rm == "GLOBAL"
    }
    // 兜底：input 为空/未知时，仅精确相等（保守）
    return rm == inputMarket
}
```

### 5.3 候选排序规则（PickBestRoute 的比较器）

命中多条时，按以下**有序决策步骤**逐级比较，前一级能分出胜负就不看后一级（字典序优先级，对齐 spec「候选路由排序规则」1→5）：

1. **package 精确 > package=`*`**：`route.Package != "*"` 的优先。
2. **具体 market > GLOBAL**：`route.Market ∈ {CN,JP,KR,SEA,HMT}` 优于 `route.Market == "GLOBAL"`。
3. **GLOBAL > market=`*`**：`route.Market == "GLOBAL"` 优于 `route.Market == "*"`。
4. **显式条件越多越优先**：统计 `{package, channel, market, country, currency}` 中 `!= "*"` 的个数（specificity 计数），多者优先。
5. **priority 越小越优先**：以上全部并列时，`priority` 数值更小者胜出。

> 说明：步骤 2、3 是 market 维度的细粒度排序，单独前置于"显式条件计数"，因为 spec 明确要求"具体 market 优先于 GLOBAL，GLOBAL 优先于 `*`"先于通用 specificity 比较。步骤 4 处理 package/market 之外其余维度的显式程度差异。步骤 5 的 priority 是运营可控的最终决胜手段——**"信用卡无感切 PSP"正是通过把目标 PSP 路由的 priority 调到比旧 PSP 更小来实现的**（在选择器完全相同的两条路由中，priority 更小者胜出；但注意同选择器组合不允许共存，见 §5.4，故实际切换是"新增更优条件路由"或"改写目标 merchant"）。

```go
// 返回 <0 表示 a 优先于 b
func compareRouteSpecificity(a, b Route) int {
    // 1) package 精确优先
    if rank := boolRank(a.Package != "*", b.Package != "*"); rank != 0 {
        return rank
    }
    // 2) 具体 market > GLOBAL
    if rank := boolRank(isConcreteMarket(a.Market), isConcreteMarket(b.Market)); rank != 0 {
        return rank
    }
    // 3) GLOBAL > "*"
    if rank := boolRank(a.Market == "GLOBAL", b.Market == "GLOBAL"); rank != 0 {
        return rank
    }
    // 4) 显式条件越多越优先
    if ca, cb := specificityCount(a), specificityCount(b); ca != cb {
        if ca > cb {
            return -1
        }
        return 1
    }
    // 5) priority 越小越优先
    if a.Priority != b.Priority {
        if a.Priority < b.Priority {
            return -1
        }
        return 1
    }
    return 0
}

func boolRank(a, b bool) int { // true 优先
    if a == b {
        return 0
    }
    if a {
        return -1
    }
    return 1
}

func isConcreteMarket(m string) bool {
    switch normalize(m) {
    case "CN", "JP", "KR", "SEA", "HMT":
        return true
    }
    return false
}

func specificityCount(r Route) int {
    n := 0
    for _, v := range []string{r.Package, r.Channel, r.Market, r.Country, r.Currency} {
        if normalize(v) != "*" {
            n++
        }
    }
    return n
}
```

### 5.4 唯一性归一化键与冲突校验（ValidateRouteSet）

**唯一性边界：同一 `env` + 同一 `gameID` + 同一 `payWayID` 下，仅对 `enabled=true` 的生效路由。** 两类约束并行：

1. **priority 唯一**：生效路由的 `priority` 不允许重复。归一化键 = `priorityKey = payWay + ":" + priority`。
2. **归一化选择器组合唯一**：`{package, channel, market, country, currency}` 经 `normalize` 折叠 `*` 后的组合不允许重复。归一化键 = `selectorKey = payWay|norm(package)|norm(channel)|norm(market)|norm(country)|norm(currency)`。

任一冲突 ⇒ 拒绝保存，返回 **`ROUTE_CONFLICT`（HTTP 409，`00` §7.4）**，并在 `message`/`details` 标注冲突的具体路由与冲突类型。

```go
func ValidateRouteSet(routes []Route) error {
    seenPriority := map[string]int{} // key -> 首次出现的 index
    seenSelector := map[string]int{}

    for i, r := range routes {
        if !r.Enabled {
            continue // 仅生效路由参与唯一性
        }

        priorityKey := fmt.Sprintf("%s:%d", r.PayWay, r.Priority)
        if j, ok := seenPriority[priorityKey]; ok {
            return &ConflictError{
                Code:    "ROUTE_CONFLICT",
                Kind:    "duplicate_priority",
                Message: fmt.Sprintf("pay_way=%s priority=%d 重复（与第 %d 条冲突）", r.PayWay, r.Priority, j),
            }
        }
        seenPriority[priorityKey] = i

        selectorKey := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
            r.PayWay,
            normalize(r.Package),
            normalize(r.Channel),
            normalize(r.Market),
            normalize(r.Country),
            normalize(r.Currency),
        )
        if j, ok := seenSelector[selectorKey]; ok {
            return &ConflictError{
                Code:    "ROUTE_CONFLICT",
                Kind:    "duplicate_selector",
                Message: fmt.Sprintf("pay_way=%s 归一化选择器 [%s] 重复（与第 %d 条冲突）", r.PayWay, selectorKey, j),
            }
        }
        seenSelector[selectorKey] = i
    }
    return nil
}
```

> 重要语义：唯一性比较中"空字段必须按 `*` 参与"（spec 明确）。因此 `(channel=NULL)` 与 `(channel="*")` 视为同一选择器，会触发 `duplicate_selector`。

### 5.5 目标自洽性校验（保存时）

除唯一性外，保存每条路由还需校验：

- `merchant_account.provider_id_ref == route.provider_id_ref`（商户账户必须属于所选 provider），否则 `VALIDATION_FAILED`。
- `pay_way` / `provider` / `merchant_account` / `channel` / `package` 引用必须存在且 `enabled`（被禁用的基础数据不可新挂路由）。
- `market_code` 取值必须 ∈ `{GLOBAL,JP,KR,SEA,HMT,CN,*}`；`currency` 若非 `*` 建议校验存在于 `currency_specs`（与 `00` §5 一致，非强制，因币种此处仅作选择器而非金额）。
- 被引用的 `game`、`channel_packages` 与本路由 `env` 一致（`00` §2.2 跨表 env 一致性）。

### 5.6 完整匹配算法（端到端伪代码）

```text
function ResolveRoute(env, gameID, input MatchInput) -> (provider, merchant_account) or NOT_FOUND:
    # 1. 取该 env+game+pay_way 下所有 enabled 路由
    routes = repo.ListEnabledRoutes(env, gameID, input.PayWay)

    # 2. 归一化已在持久层->领域层映射时完成（NULL/'' -> '*'）

    # 3. 候选过滤（§5.1）
    candidates = []
    for r in routes:
        if matchPackage(r, input) and matchChannel(r, input)
           and marketMatches(r.Market, input.Market)
           and matchCountry(r, input) and matchCurrency(r, input):
            candidates.append(r)

    if candidates is empty:
        return NOT_FOUND   # 客户端回退到渠道/默认处理（由收银台模块决定）

    # 4. 排序（§5.3），稳定排序保证可重现
    stableSort(candidates, by compareRouteSpecificity)

    # 5. 取第一名
    best = candidates[0]
    return (best.Provider, best.MerchantAccount)
```

### 5.7 "信用卡无感切 PSP" 的标准操作流（运营视角）

目标：把 `credit_card` 在 `GLOBAL/US/USD` 场景从 Airwallex 切到 PayerMax，玩家无感。

做法（任选其一，推荐 A）：
- **A. 改写目标（推荐，最干净）**：编辑命中该场景的那条路由，把 `provider/merchant_account` 从 `airwallex/merchant_aw_main` 改为 `payermax/merchant_pm_main`。选择器与 priority 不变，唯一性不受影响。客户端下次拉取快照即生效。
- **B. 加更精确的新路由**：新增一条 `package/channel/market/country/currency` 更具体（specificity 更高）或同选择器下的路由——但**同选择器组合不允许共存**（§5.4），所以"加新路由"只有在选择器确实更精确时才合法，否则会 `ROUTE_CONFLICT`。

> 这正体现解耦价值：玩家始终点"信用卡"，后台改的是它背后的 `merchant_account`，玩家完全无感。

---

## 6. 后端 API

统一前缀 `/api/admin`，统一包络（`00` §7.2），camelCase 字段，Bearer 鉴权，写操作挂权限码（`00` §7.5）。以下逐接口给出完整 DTO + 校验 + 示例 JSON。

权限码约定（本模块）：
- 读：`payment.read`
- 写商户/主体/路由：`payment.write`
- 路由发布到运行时（如有）：复用快照/同步模块权限。

### 6.1 `GET /api/admin/pay-ways` — 支付方式列表

- 权限：`payment.read`
- query：`page`/`pageSize`/`sort`（`00` §7.3）、可选 `enabled`、`type`（PayWayType 过滤）
- 平台级，无 env 维度。

响应：

```json
{
  "data": {
    "items": [
      { "payWayId": "credit_card", "payWayName": "信用卡", "payWayType": "card", "enabled": true, "sort": 10 },
      { "payWayId": "paypal", "payWayName": "PayPal", "payWayType": "wallet", "enabled": true, "sort": 20 }
    ],
    "page": 1, "pageSize": 20, "total": 4
  }
}
```

### 6.2 `GET /api/admin/cashier/providers` — 提供商列表

- 权限：`payment.read`
- query：`page`/`pageSize`/`sort`、可选 `enabled`、`kind`（ProviderKind 过滤）

响应：

```json
{
  "data": {
    "items": [
      { "providerId": "airwallex", "providerName": "Airwallex", "providerKind": "aggregator", "enabled": true, "sort": 10 },
      { "providerId": "payermax", "providerName": "PayerMax", "providerKind": "aggregator", "enabled": true, "sort": 20 }
    ],
    "page": 1, "pageSize": 20, "total": 3
  }
}
```

### 6.3 `GET /api/admin/billing-subjects` — 主体列表

- 权限：`payment.read`

```json
{
  "data": {
    "items": [
      { "subjectId": "hk_entity", "subjectName": "XX 香港有限公司", "legalEntityName": "XX (HK) Co., Limited", "enabled": true }
    ],
    "page": 1, "pageSize": 20, "total": 2
  }
}
```

### 6.4 `POST /api/admin/billing-subjects` — 新建主体

- 权限：`payment.write`，审计 `action=billing_subject.create`
- 请求 DTO：

| 字段 | 类型 | 必填 | 校验 |
| --- | --- | --- | --- |
| `subjectId` | string | 是 | 1–64，`[a-z0-9_]`，全局唯一（冲突 `CONFLICT`） |
| `subjectName` | string | 是 | 1–128 |
| `legalEntityName` | string | 是 | 1–255 |
| `enabled` | bool | 否 | 默认 `true` |

请求：

```json
{ "subjectId": "sg_entity", "subjectName": "XX 新加坡主体", "legalEntityName": "XX Singapore Pte. Ltd.", "enabled": true }
```

成功 `201`：

```json
{ "data": { "subjectId": "sg_entity", "subjectName": "XX 新加坡主体", "legalEntityName": "XX Singapore Pte. Ltd.", "enabled": true } }
```

冲突：

```json
{ "error": { "code": "CONFLICT", "message": "subjectId already exists", "details": [] } }
```

### 6.5 `GET /api/admin/cashier/merchant-accounts` — 商户账户列表

- 权限：`payment.read`
- query：可选 `providerId`、`subjectId`、`enabled`
- **`secretCiphertext` 及密文字段一律脱敏返回 `"masked"`（`00` §6.1）。**

```json
{
  "data": {
    "items": [
      {
        "merchantAccountId": "merchant_aw_main",
        "providerId": "airwallex",
        "subjectId": "hk_entity",
        "merchantId": "AW-001",
        "merchantName": "Airwallex HK Main",
        "configJson": { "apiBase": "https://api.airwallex.com" },
        "secret": "masked",
        "enabled": true
      }
    ],
    "page": 1, "pageSize": 20, "total": 2
  }
}
```

### 6.6 `POST /api/admin/cashier/merchant-accounts` — 新建商户账户

- 权限：`payment.write`，审计 `action=merchant_account.create`（detail 脱敏，`00` §8）
- 请求 DTO：

| 字段 | 类型 | 必填 | 校验 |
| --- | --- | --- | --- |
| `merchantAccountId` | string | 是 | 1–64，唯一（`CONFLICT`） |
| `providerId` | string | 是 | 必须存在且 `enabled` |
| `subjectId` | string | 是 | 必须存在且 `enabled` |
| `merchantId` | string | 是 | 1–128 |
| `merchantName` | string | 是 | 1–128 |
| `configJson` | object | 否 | 按该 provider 的 `cashier_provider_templates` 的 `validation_rules_json` 校验 |
| `secrets` | object | 是* | 由 `secret_fields_json` 决定的密文字段；落库前加密为 `secret_ciphertext`，绝不回明文 |
| `enabled` | bool | 否 | 默认 `true` |

请求：

```json
{
  "merchantAccountId": "merchant_pm_main",
  "providerId": "payermax",
  "subjectId": "sg_entity",
  "merchantId": "PM-001",
  "merchantName": "PayerMax SG Main",
  "configJson": { "apiBase": "https://api.payermax.com" },
  "secrets": { "apiKey": "REAL_KEY", "signKey": "REAL_SIGN" },
  "enabled": true
}
```

成功 `201`（密文已脱敏）：

```json
{
  "data": {
    "merchantAccountId": "merchant_pm_main",
    "providerId": "payermax",
    "subjectId": "sg_entity",
    "merchantId": "PM-001",
    "merchantName": "PayerMax SG Main",
    "configJson": { "apiBase": "https://api.payermax.com" },
    "secret": "masked",
    "enabled": true
  }
}
```

### 6.7 `GET /api/admin/games/{gameId}/payment-routes` — 游戏支付路由（按 env）

- 权限：`payment.read`
- 作用 env = 当前运行环境（`00` §2.1，不接受前端指定）。
- 返回按 `payWay` 分组，组内按生效优先级（即 §5.3 排序）呈现，便于前端"优先级列表 + 兜底关系"展示。

响应：

```json
{
  "data": {
    "gameId": "100001",
    "env": "sandbox",
    "groups": [
      {
        "payWayId": "credit_card",
        "payWayName": "信用卡",
        "payWayType": "card",
        "routes": [
          {
            "id": 9001,
            "selector": { "package": "*", "channel": "*", "market": "JP", "country": "*", "currency": "JPY" },
            "providerId": "payermax",
            "merchantAccountId": "merchant_pm_main",
            "priority": 10,
            "enabled": true
          },
          {
            "id": 9002,
            "selector": { "package": "*", "channel": "*", "market": "GLOBAL", "country": "*", "currency": "*" },
            "providerId": "airwallex",
            "merchantAccountId": "merchant_aw_main",
            "priority": 100,
            "enabled": true
          }
        ]
      }
    ]
  }
}
```

> 选择器中 `"*"` 即领域层 `normalize` 后的通配值；前端展示时把 `*` 渲染为"任意/兜底"。

### 6.8 `PUT /api/admin/games/{gameId}/payment-routes` — 全量保存游戏路由

- 权限：`payment.write`，审计 `action=payment_route.update`
- 语义：**整组替换**（按 game + env 全量覆盖；也可按 payWay 分组提交，见 §7）。保存前服务端执行 §5.4 唯一性校验 + §5.5 自洽校验；任一失败整体拒绝（事务回滚），返回 `ROUTE_CONFLICT` 或 `VALIDATION_FAILED`。
- 请求 DTO（`items[]`）：

| 字段 | 类型 | 必填 | 校验 |
| --- | --- | --- | --- |
| `marketCode` | string | 否 | ∈ `{GLOBAL,JP,KR,SEA,HMT,CN,*}`，缺省 `*` |
| `countryCode` | string | 否 | 缺省 `*` |
| `currency` | string | 否 | 缺省 `*`；非 `*` 建议存在于 `currency_specs` |
| `channelId` | string\|null | 否 | null/缺省 = `*`；非空必须存在且 enabled |
| `packageCode` | string\|null | 否 | null/缺省 = `*`；非空必须属于本 game 且 env 一致 |
| `payWayId` | string | 是 | 必须存在且 enabled |
| `providerId` | string | 是 | 必须存在且 enabled |
| `merchantAccountId` | string | 是 | 必须存在且 enabled，且其 provider == `providerId`（§5.5） |
| `priority` | int | 否 | 缺省 `100`；同 payWay 生效集合内唯一（§5.4） |
| `enabled` | bool | 否 | 缺省 `true` |

请求：

```json
{
  "items": [
    {
      "marketCode": "JP", "countryCode": "*", "currency": "JPY",
      "channelId": null, "packageCode": null,
      "payWayId": "credit_card", "providerId": "payermax",
      "merchantAccountId": "merchant_pm_main", "priority": 10, "enabled": true
    },
    {
      "marketCode": "GLOBAL", "countryCode": "*", "currency": "*",
      "channelId": null, "packageCode": null,
      "payWayId": "credit_card", "providerId": "airwallex",
      "merchantAccountId": "merchant_aw_main", "priority": 100, "enabled": true
    }
  ]
}
```

成功 `200`：返回与 §6.7 相同结构的最新路由分组。

冲突示例（同 payWay priority 重复）：

```json
{
  "error": {
    "code": "ROUTE_CONFLICT",
    "message": "pay_way=credit_card priority=10 重复（与第 0 条冲突）",
    "details": [{ "kind": "duplicate_priority", "payWayId": "credit_card", "priority": 10 }]
  }
}
```

冲突示例（归一化选择器重复）：

```json
{
  "error": {
    "code": "ROUTE_CONFLICT",
    "message": "pay_way=credit_card 归一化选择器 [credit_card|*|*|GLOBAL|*|*] 重复（与第 1 条冲突）",
    "details": [{ "kind": "duplicate_selector", "payWayId": "credit_card", "selector": "credit_card|*|*|GLOBAL|*|*" }]
  }
}
```

自洽校验失败示例：

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "merchantAccount merchant_pm_main 不属于 provider airwallex",
    "details": []
  }
}
```

---

## 7. 应用服务（PaymentRouteService）

应用层 `PaymentRouteService`（`internal/app`，编排，不放纯规则；`01` §4）。依赖窄仓储 + `payment` 领域纯函数 + `crypto`（商户密钥）。

职责：
1. **读编排**：`ListPayWays` / `ListProviders` / `ListBillingSubjects` / `ListMerchantAccounts`（脱敏）/ `GetGameRoutes`（按 env，分组 + 组内排序）。
2. **写编排（事务内）**：
   - `CreateBillingSubject` / `CreateMerchantAccount`（加密 secret → `secret_ciphertext`；按 provider 模板 `validation_rules_json` 校验 config）。
   - `SaveGameRoutes`：装载提交 items → 映射 NULL/`""`/`*` 三态 → `ValidateRouteSet`（§5.4）→ 逐条自洽校验（§5.5）→ 全量替换 `payment_routes`（限定 game + env）→ 写 `audit_logs`。
3. **运行时解析**：`ResolveRoute(env, gameID, MatchInput)`（§5.6），供配置快照模块（`snapshot`）生成 per-game per-market 运行时配置时调用。
4. **唯一性与冲突**：全部委托 `route_validator.ValidateRouteSet`，冲突统一抛 `ROUTE_CONFLICT`。

服务接口（示意）：

```go
type PaymentRouteService interface {
    // 读
    ListPayWays(ctx, filter) ([]dto.PayWayDTO, error)
    ListProviders(ctx, filter) ([]dto.ProviderDTO, error)
    ListBillingSubjects(ctx, filter) ([]dto.BillingSubjectDTO, error)
    ListMerchantAccounts(ctx, filter) ([]dto.MerchantAccountDTO, error) // secret 脱敏
    GetGameRoutes(ctx, env, gameID) (dto.GameRoutesDTO, error)          // 分组+排序

    // 写（事务）
    CreateBillingSubject(ctx, cmd) (dto.BillingSubjectDTO, error)
    CreateMerchantAccount(ctx, cmd) (dto.MerchantAccountDTO, error)     // 加密 secret
    SaveGameRoutes(ctx, env, gameID, cmd) (dto.GameRoutesDTO, error)    // 校验+全量替换+审计

    // 运行时
    ResolveRoute(ctx, env, gameID, input payment.MatchInput) (payment.RouteTarget, error)
}
```

与仓储边界（`01` §4.2）：仓储窄，仅单聚合 CRUD + `ListEnabledRoutes(env, gameID, payWayID)` 等必要查询；跨表自洽校验、加密、唯一性都在 service。所有带 env 方法接收 `ctx` 与 `env`。

---

## 8. 前端

前端 `/payment` 路由分组（`01` §5.1），抽屉式交互（`01` §5.3）。子页面：支付方式、提供商、主体、商户账户、游戏支付路由。对齐 `frontend_agent_execution.md` 阶段 8 完成标准——"能在页面里把某个支付方式切到另一个 PSP"。

### 8.1 页面结构

```text
/payment
  ├─ /pay-ways            支付方式列表（只读基础数据，展示 type/enabled）
  ├─ /providers           提供商列表（只读基础数据，展示 kind/enabled）
  ├─ /billing-subjects    主体列表 + 新建/编辑抽屉
  ├─ /merchant-accounts   商户账户列表 + 新建/编辑抽屉（模板驱动表单 + 密文输入）
  └─（游戏支付路由位于 /games/:gameId 详情页的"支付路由"Tab，游戏级 + env）
```

> 路由是游戏级 + env 数据，归属游戏详情页"支付路由"Tab（`spec` 前端信息架构：支付路由属游戏级区域）；前 4 个是平台级基础数据，归属 `/payment`。

### 8.2 商户账户抽屉（模板驱动 + 密文）

- 选择 `provider` → 拉取该 provider 的 `published` 模板四件套 → 用统一模板渲染器渲染 `form_schema_json`（`01` §5.3）。
- `secret_fields_json` 字段用 `password` 组件，回显恒为 `masked`，留空表示不修改（`00` §6.1）。
- `file_fields_json` 字段走统一上传（`00` §6.2）。
- 选择 `billing_subject` 下拉。
- 校验同后端 `validation_rules_json`。

### 8.3 游戏支付路由编辑器（核心交互）

设计目标（对齐用户诉求）：**按 pay_way 呈现为优先级列表；兜底关系直观；一个编辑器统一支持 package/channel/market/country/currency 作用域而不混乱；能把 credit_card 从一个 PSP 切到另一个。**

布局：

```text
[支付路由 Tab]   当前环境: sandbox（EnvironmentBadge）

▼ 信用卡 (credit_card · card)                         [+ 新增路由]
   ┌ 优先级链路（从上到下 = 命中优先级从高到低）──────────────────────┐
   │ #1  JP · *·* · JPY     → PayerMax / merchant_pm_main   prio 10  ⋯ │
   │ #2  GLOBAL · *·*·*      → Airwallex / merchant_aw_main  prio 100 ⋯ │
   │      ↑ 兜底（GLOBAL 通配，海外默认）                                │
   └──────────────────────────────────────────────────────────────────┘

▼ PayPal (paypal · wallet)                            [+ 新增路由]
   │ #1  GLOBAL · *·*·*      → PayPal / merchant_pp_main    prio 100  ⋯ │
```

交互要点：
- **按 pay_way 分组折叠**，每组是一条"优先级链路"。组内顺序即后端 §5.3 排序结果（前端不自行猜测排序，直接用后端返回顺序），保证"看到的优先级 = 实际命中优先级"。
- **兜底关系直观**：选择器全 `*`（或仅 market=GLOBAL/`*`）的路由用"兜底"徽标标注，并固定排在组内末尾视觉上"垫底"。
- **一个编辑器统一作用域**：新增/编辑路由抽屉用单一表单，5 个作用域字段（package/channel/market/country/currency）各一行，每行支持"任意(`*`) / 指定"二选一；指定时给下拉（market 固定枚举；channel/package 联动本游戏可选项；country/currency 输入或选择）。避免多套表单造成混乱。
- **切 PSP**：在某条路由的 `⋯` 菜单选"切换通道" → 抽屉只暴露 `provider + merchant_account` 两个字段（merchant 列表按所选 provider 过滤），保存即完成"信用卡无感切 PSP"。
- **冲突即时反馈**：保存触发 `ROUTE_CONFLICT` 时，前端高亮冲突的两条路由并提示是 `duplicate_priority` 还是 `duplicate_selector`。
- **env 常驻**：`production` 运行环境下隐藏一切"Sync to Production"入口（`00` §9 红线、`01` §2）；路由本身仍可在 production 查看。
- 写/危险操作挂权限指令 `payment.write`（无权限置灰）。

### 8.4 状态与可见性

- 禁用（`enabled=false`）的 pay_way/provider/merchant 在新增路由的下拉中默认不可选。
- 引用了已禁用基础数据的存量路由：行内标红提示"引用对象已禁用"，但不自动删除（保留记录，类比渠道"不兼容"处理思路）。

---

## 9. 与公共能力关系

| 公共能力（`00`/`01`） | 本模块如何遵循 |
| --- | --- |
| env 模型（D1，`00` §2） | 仅 `payment_routes` 带 env，唯一约束前置 env；5 张平台表不带 env。写操作 env 取当前运行环境。 |
| 全局枚举（`00` §3） | `PayWayType`、`ProviderKind`、`Market`、`Environment` 全部以 `00` 为唯一事实来源。 |
| Market 语义（`00` §3.2） | §5.2 marketMatches 与 `00`/spec 完全一致（CN 不被 GLOBAL 兜底等）。 |
| 模板四件套（`00` §4） | `cashier_provider_templates` 驱动商户账户表单；遵循版本生命周期 §3.3。 |
| 币种归一化（`00` §5） | 路由 `currency` 仅作选择器，不涉及金额写入；如校验存在性则查 `currency_specs`。真实金额在收银台模块。 |
| 密文与文件（`00` §6） | `cashier_merchant_accounts.secret_ciphertext` 加密落库、响应脱敏、同步预览 `masked=true`、复制时清空。 |
| 统一 API 约定（`00` §7） | 前缀、包络、分页、错误码（含 `ROUTE_CONFLICT`）、camelCase 全部遵循。 |
| 权限码（`00` §7.5） | `payment.read` / `payment.write`。 |
| 审计（`00` §8） | 主体/商户/路由的写操作写 `audit_logs`，`action` 与权限同源，detail 脱敏。 |
| 配置快照（`snapshot`） | 快照生成时按 per-game per-market 调用 `ResolveRoute`，把"该 market 下各 pay_way 的最佳通道"写入运行时配置。隐藏/禁用/无效数据不进快照（`00` §9）。 |
| 同步（`sync`） | 路由属 `payments` section；`sync/preview` 按 section 输出 add/update/delete，密文 masked；`sync/execute` 携带 baseline_token 复核 hash（D6）。 |
| 红线（`00` §9） | 不混 IAP 与支付路由；不存明文密钥；production 不出现可执行 Sync to Production。 |

---

## 10. 测试要点

> 对齐 `backend_agent_execution.md` 阶段 12「支付路由解析」单测要求与实现计划 Task 3。所有匹配/排序/唯一性测试针对纯函数，无需 IO。

### 10.1 匹配与排序（route_matcher）

1. **具体 market 击败 GLOBAL**：路由 `{market:GLOBAL,prio:20}` 与 `{market:JP,prio:30}`，输入 `market=JP` → 命中 `JP`（即便其 priority 更大）。验证 §5.3 步骤 2 优先于步骤 5。
2. **GLOBAL 兜底但不兜 CN**：输入 `market=CN`，仅有 `{market:GLOBAL}` 路由 → 不命中（`NOT_FOUND`）；输入 `market=JP` 同条件 → 命中 GLOBAL。
3. **GLOBAL > market=`*`**：`{market:GLOBAL}` 与 `{market:*}` 同时命中 JP → 选 GLOBAL。
4. **package 精确 > 通配**：`{package:google-jp}` 与 `{package:*}` 命中同输入 → 选精确。验证步骤 1 先于 market 之外的 specificity。
5. **显式条件越多越优先**：`{country:US}` 与 `{country:*,currency:USD}` 在 package/market 同级时，比较 specificityCount。
6. **priority 决胜**：所有维度并列时 priority 小者胜；用于"无感切 PSP"语义验证（改 priority 影响命中）。
7. **稳定性**：相同输入多次解析结果一致（stableSort）。
8. **pay_way 隔离**：不同 pay_way 的路由互不干扰（`credit_card` 输入不会命中 `paypal` 路由）。

### 10.2 唯一性（route_validator）

9. **重复 priority 拒绝**：同 game+pay_way+env 下两条生效路由 priority 相同 → `ROUTE_CONFLICT/duplicate_priority`。
10. **选择器去重（空=`*`）**：一条 `channel=NULL`、一条 `channel="*"`，其余相同 → 归一化后同键 → `ROUTE_CONFLICT/duplicate_selector`。
11. **disabled 不参与唯一性**：把其中一条置 `enabled=false`，则 priority/selector 冲突消失，校验通过。
12. **跨 pay_way 不冲突**：`credit_card` 与 `paypal` 各有 priority=10 的路由 → 通过。
13. **自洽校验**：merchant_account 的 provider ≠ route.provider → `VALIDATION_FAILED`。

### 10.3 API / 集成

14. `PUT payment-routes` 提交冲突集合 → 整体回滚，DB 不留部分写入。
15. 商户账户 `GET/POST` 响应 secret 恒为 `masked`，DB 存密文。
16. 路由读取响应按 pay_way 分组且组内顺序 == §5.3 排序（前端可直接渲染）。

---

## 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/payment.yaml`；前端 e2e：`tests/frontend/e2e/payment.spec.ts`。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET /api/admin/pay-ways | ✓ | ✓ | ✓ | — | — | — | — | — | ✓ | — | 平台级无 env（S6 —）；只读基础数据 |
| GET /api/admin/cashier/providers | ✓ | ✓ | ✓ | — | — | — | — | — | ✓ | — | 平台级无 env（S6 —）；只读基础数据 |
| GET /api/admin/billing-subjects | ✓ | ✓ | ✓ | — | — | — | — | — | ✓ | — | 平台级无 env（S6 —） |
| POST /api/admin/billing-subjects | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | — | subjectId 唯一(CONFLICT)；平台级无 env（S6 —）；审计 billing_subject.create |
| GET /api/admin/cashier/merchant-accounts | ✓ | ✓ | ✓ | — | — | — | — | ✓ | ✓ | — | 商户密钥脱敏(merchant_accounts.secret_ciphertext, S8)；平台级无 env（S6 —） |
| POST /api/admin/cashier/merchant-accounts | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ | — | — | 密钥加密落库+脱敏(secret_ciphertext, S8)；平台级无 env（S6 —）；审计 merchant_account.create |
| GET /api/admin/games/{gameId}/payment-routes | ✓ | ✓ | ✓ | — | — | ✓ | — | — | — | — | payment_routes 带 env（S6 ✓）；ResolveRoute 命中、分组+排序展示 |
| PUT /api/admin/games/{gameId}/payment-routes | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | 路由匹配/排序/唯一性(ROUTE_CONFLICT)、选择器归一化(*)、PSP 无感切换、与渠道 IAP 隔离(红线)；payment_routes 带 env（S6 ✓）；全量替换事务回滚(S10) |

前端：`payment.spec.ts` 覆盖 `/payment`（支付方式/提供商只读列表、主体与商户账户抽屉创建、密文输入恒显 `masked`）与游戏详情「支付路由」Tab（按 pay_way 优先级链路渲染、`⋯` 切换通道实现 PSP 无感切、`ROUTE_CONFLICT` 双行高亮、production 隐藏 Sync 入口） / vitest 覆盖模板驱动商户表单渲染、路由编辑器 5 维作用域 `*` 归一化与兜底徽标。

---

## 11. 未决问题与假设

### 11.1 假设（本文已按此书写，若不成立需回改）

- **A1**：`payment_routes` 的 env 列与唯一约束由 v2 新迁移补齐（§3.7）；存量行回填 `develop`，具体回填策略待运维确认。
- **A2**：商户密钥本期平台级、全 env 共享（D1 备注）；未来若需分环境密钥再扩展，不影响本模块路由结构。
- **A3**：路由的 `country/currency` 仅作选择器字符串匹配，不强制存在于 `currency_specs`（仅建议校验）；金额相关校验属收银台模块。
- **A4**：`PUT payment-routes` 采用"按 game+env 全量替换"语义；若改为"按 pay_way 分组增量替换"，唯一性边界不变，仅接口粒度变化。
- **A5**：运行时 `ResolveRoute` 在"无候选命中"时返回 `NOT_FOUND`，由收银台/客户端决定回退（如禁用该支付方式或回退渠道计费）；本模块不定义回退兜底通道。
- **A6**：DB 部分唯一索引作为兜底，权威唯一性校验在应用层（§3.7 说明）。

### 11.2 未决问题（需产品/架构后续拍板）

- **Q1**：选择器排序步骤 4"显式条件越多越优先"中，`channel` 与 `country` 等不同维度是否需要带权重（而非简单计数）？当前按 spec 用等权计数；若出现"channel 应比 country 更重要"的诉求需再定义维度权重表。
- **Q2**：是否允许同一 pay_way 下两条**完全相同选择器**但不同 priority 的"灰度/AB"路由？当前 §5.4 明确禁止（duplicate_selector），故灰度切换需借助更精确选择器或直接改写目标 merchant；若产品要按比例分流，需要引入新的"权重/分流"字段，超出本期范围。
- **Q3**：路由是否需要"生效时间窗（effective_at / expire_at）"以支持定时切换 PSP？当前表无时间窗字段，切换为即时生效；如需定时，需扩展 `payment_routes` 字段与解析逻辑。
- **Q4**：`provider` 或 `merchant_account` 被禁用时，已引用它的存量路由的运行时行为——当前假设"前端标红 + 解析时跳过禁用目标"，但"解析时是否跳过该候选继续找下一条"需确认（本文 §5.1 仅以 route.enabled 过滤，未对 target 禁用做候选剔除，建议补充：target 禁用的路由解析时视为不可用并继续找次优）。
- **Q5**：`payment-routes` 写操作的权限是否需要比一般 `payment.write` 更高（涉及真实资金通道）？当前统一 `payment.write`，是否拆出 `payment.route.publish` 待定。
