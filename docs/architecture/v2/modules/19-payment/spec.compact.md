---
id: payment
code: "19"
title: 支付路由（Payment Routing）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [channel, product, cashier-template, game-cashier, game, common]
code_paths:
  - services/admin-api/internal/domain/payment
  - apps/admin-web/src/views/payment
---

# 19 · 支付路由 — Compact Spec

> 代码生成用精简规格。完整背景/示例/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（env 模型 D1 §2、统一包络/错误码 §7、密文 §6、审计 §8、Market 语义 §3.2、模板四件套 §4）。
> 核心诉求：让玩家可见支付方式（如信用卡）在不改客户端、玩家无感的前提下从一个 PSP 切到另一个 PSP。手段：解耦「玩家可见支付方式」与「真实清算通道（商户账户）」，用可排序/可通配/可校验唯一性的 `payment_routes` 按游戏维度连接。

## 五个核心概念
| 概念 | 表 | env | 语义 |
| --- | --- | --- | --- |
| pay_way 支付方式 | pay_ways | 平台级无 env | 玩家可见支付选项（信用卡/PayPal/GCash） |
| provider 提供商 | cashier_providers | 平台级无 env | 真实 PSP（Airwallex/PayerMax/Stripe），玩家不感知 |
| billing_subject 公司主体 | billing_subjects | 平台级无 env | 签约/清算法律实体 |
| merchant_account 商户账户 | cashier_merchant_accounts | 平台级无 env | 某主体在某 provider 下的商户号+密钥（密文） |
| payment_route 支付路由 | payment_routes | **游戏级业务表/每环境 schema/不带 env 列** | 按「游戏+pay_way」把选择器映射到 (provider, merchant_account) 的优先级规则 |

## 红线
- pay_way ≠ provider（一个 pay_way 可多 provider 承载；切 provider 玩家无感）。
- 渠道 IAP 配置(`product`) 与 收银台支付路由(本模块) 完全独立（数据/表/领域包分离）。
- channel（分发渠道）≠ provider（清算服务商）；选择器里的 channel 仅作细分。
- 路由匹配/排序/唯一性是**纯函数**（`internal/domain/payment`，无 IO）。
- 平台级 5 表放共享 schema `platform`、不带 env；仅 `payment_routes` 每环境独立 schema。

## 领域模型（internal/domain/payment）
聚合 `PaymentRouting` 一致性边界 = 当前环境 schema 内 `(gameID, payWayID)`。跨 pay_way 的 priority 互不影响。

```go
type Route struct {
    ID int64; GameID string
    // 选择器（空 = "*" 通配）
    Package, Channel, Market, Country, Currency string
    // 目标
    PayWay, Provider, MerchantAccount string
    Priority int   // 默认 100，越小越优先
    Enabled  bool  // 仅 true 参与生效集合与唯一性
}
type MatchInput struct { PayWay, Package, Channel, Market, Country, Currency string } // PayWay 必填
```

三态等价：**DB NULL ⇔ 领域 "" ⇔ 归一化 "*"**。
- DB 中 `market_code/country_code/currency` 默认字面量 `'*'`；`channel_id_ref/package_id_ref` 用 NULL 表示通配（外键不能存 `"*"`）。
- 应用层把 NULL 外键映射为领域 `""`，再 `normalize` 折叠为 `"*"`。

纯函数划分：`route_matcher.go`（`MatchedRoutes`、`PickBestRoute`、`marketMatches`）、`route_validator.go`（`ValidateRouteSet`，冲突返回 `ROUTE_CONFLICT`）。

## 数据模型
平台级 5 表（schema `platform`，不带 env）+ 游戏级 `payment_routes`（每环境 schema）。所有表含 `id BIGSERIAL PK`、`enabled BOOLEAN DEFAULT TRUE`、`created_at/updated_at TIMESTAMPTZ DEFAULT NOW()`。

- **pay_ways**: `pay_way_id VARCHAR(64) UNIQUE`, `pay_way_name`, `pay_way_type CHECK IN('card','wallet','platform','local')`, `sort INT DEFAULT 0`。
- **cashier_providers**: `provider_id VARCHAR(64) UNIQUE`, `provider_name`, `provider_kind CHECK IN('aggregator','gateway','wallet_direct')`, `sort`。
- **cashier_provider_templates**（简单模板表，无 status，取 enabled 最新 template_version）: `provider_id_ref FK→cashier_providers(id)`, `template_version`, 四件套 `form_schema_json/secret_fields_json/file_fields_json DEFAULT '[]'`、`validation_rules_json DEFAULT '{}'`；UNIQUE(provider_id_ref, template_version)。
- **billing_subjects**: `subject_id VARCHAR(64) UNIQUE`, `subject_name`, `legal_entity_name`。
- **cashier_merchant_accounts**（密文）: `merchant_account_id VARCHAR(64) UNIQUE`, `provider_id_ref FK`, `subject_id_ref FK`, `merchant_id`, `merchant_name`, `config_json JSONB DEFAULT '{}'`, `secret_ciphertext TEXT NOT NULL`（AES-GCM 加密，响应恒 `"masked"`）。

### payment_routes（游戏级业务表 / 每环境 schema / 不带 env 列）
| 列 | 类型 | 默认 | 约束 |
| --- | --- | --- | --- |
| game_id_ref | BIGINT | — | FK→games(id)（同 schema 普通 FK） |
| market_code | VARCHAR(32) | `'*'` | NOT NULL 选择器 |
| country_code | VARCHAR(8) | `'*'` | NOT NULL 选择器 |
| currency | VARCHAR(8) | `'*'` | NOT NULL 选择器 |
| channel_id_ref | BIGINT | NULL | FK→platform.channels(id) 可空，NULL=通配 |
| package_id_ref | BIGINT | NULL | FK→channel_packages(id) 可空，NULL=通配 |
| pay_way_id_ref | BIGINT | — | FK→platform.pay_ways(id)（唯一性聚合维度） |
| provider_id_ref | BIGINT | — | FK→platform.cashier_providers(id) |
| merchant_account_id_ref | BIGINT | — | FK→platform.cashier_merchant_accounts(id) |
| priority | INT | `100` | NOT NULL 越小越优先 |
| enabled | BOOLEAN | `TRUE` | NOT NULL 仅 true 参与生效/唯一性 |

约束：merchant_account 的 provider 必须 == 路由 provider（应用层校验）。

### v2 迁移：唯一索引（每环境 schema 内，不前置 env；DB 兜底，权威校验在应用层）
```sql
CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_routes_priority
  ON payment_routes (game_id_ref, pay_way_id_ref, priority) WHERE enabled;
CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_routes_selector
  ON payment_routes (game_id_ref, pay_way_id_ref,
    COALESCE(package_id_ref,-1), COALESCE(channel_id_ref,-1),
    market_code, country_code, currency) WHERE enabled;
```

## 枚举与默认
- `PayWayType`: card/wallet/platform/local（无默认，建表必填）。
- `ProviderKind`: aggregator/gateway/wallet_direct（无默认）。
- `Market`（选择器域）: GLOBAL/JP/KR/SEA/HMT/CN/`*`（默认 `*`）。
- `Environment`: develop/sandbox/production（运行时由 search_path 决定，非表列）。
- 默认值：route market/country/currency=`'*'`，channel/package=NULL(=`*`)，priority=100，enabled=TRUE；模板四件套 `[]`/`{}`；config_json=`{}`。

## 业务规则与算法（核心）

### 候选命中 MatchedRoutes（全部满足才候选）
1. `route.PayWay == input.PayWay`（精确，不通配）；2. `route.Enabled==true`；
3. package: `norm(route.Package)=="*"` 或 ==input.Package；4. channel 同理；
5. market: `marketMatches(route.Market, input.Market)`；6. country 同理；7. currency 同理。

### Market 语义（marketMatches）
- CN 只匹配 CN 或 `*`（GLOBAL 永不兜底 CN）。
- JP/KR/SEA/HMT 匹配 自身 或 GLOBAL 或 `*`（GLOBAL 海外兜底）。
- GLOBAL 只匹配 GLOBAL 或 `*`。`*` 路由匹配任意 market。
```go
func marketMatches(routeMarket, inputMarket string) bool {
    rm := normalize(routeMarket)
    if rm == "*" { return true }
    if inputMarket == "CN" { return rm == "CN" }
    if inputMarket == "JP" || inputMarket == "KR" || inputMarket == "SEA" || inputMarket == "HMT" {
        return rm == inputMarket || rm == "GLOBAL"
    }
    if inputMarket == "GLOBAL" { return rm == "GLOBAL" }
    return rm == inputMarket
}
```

### 排序 PickBestRoute（有序决策，前级分胜负即止）
1. package 精确(`!="*"`) > package=`*`。
2. 具体 market(CN/JP/KR/SEA/HMT) > GLOBAL。
3. GLOBAL > market=`*`。
4. 显式条件计数（{package,channel,market,country,currency} 中 `!="*"` 个数）多者优先。
5. priority 越小越优先（最终决胜；"无感切 PSP"靠此或改写目标 merchant）。
> 步骤 2/3 是 market 维度细粒度排序，前置于通用 specificity 计数（步骤 4）。

### 唯一性 ValidateRouteSet（仅 enabled=true，当前 schema 内同 game+payWay）
两类约束并行，冲突 → `ROUTE_CONFLICT`（HTTP 409，含冲突类型 details）：
1. priority 唯一：key = `payWay:priority` → 冲突 `duplicate_priority`。
2. 归一化选择器唯一：key = `payWay|norm(package)|norm(channel)|norm(market)|norm(country)|norm(currency)` → 冲突 `duplicate_selector`（`channel=NULL` 与 `channel="*"` 视为同键）。

### 目标自洽性校验（保存时，违反 → VALIDATION_FAILED）
- merchant_account.provider == route.provider。
- pay_way/provider/merchant_account/channel/package 引用必须存在且 enabled（被禁用基础数据不可新挂路由）。
- market_code ∈ {GLOBAL,JP,KR,SEA,HMT,CN,*}；currency 非 `*` 建议校验存在于 currency_specs（非强制）。

### 运行时解析 ResolveRoute（供 snapshot 调用）
```text
ResolveRoute(gameID, input) -> (provider, merchant_account) | NOT_FOUND:
  routes = repo.ListEnabledRoutes(gameID, input.PayWay)   # 当前 schema, SQL 不写 schema 前缀
  candidates = filter(routes):
     skip if providerDisabled(r) or merchantAccountDisabled(r)   # 运行时剔除被禁用目标
     keep if matchPackage & matchChannel & marketMatches & matchCountry & matchCurrency
  if empty: return NOT_FOUND        # 由收银台/客户端决定回退
  stableSort(candidates, compareRouteSpecificity)   # 稳定排序，可重现
  return candidates[0].(Provider, MerchantAccount)
```
> 运行时剔除：即便 route.enabled=true，若其 provider 或 merchant_account 被禁用，则该候选剔除、不进客户端配置（与保存期校验对称）。

## 后端 API（前缀 /api/admin，包络 00 §7；读 payment.read / 写 payment.write）
- `GET /pay-ways`（平台级，分页+enabled/type 过滤）→ items: {payWayId, payWayName, payWayType, enabled, sort}
- `GET /cashier/providers`（分页+enabled/kind）→ items: {providerId, providerName, providerKind, enabled, sort}
- `GET /billing-subjects` → items: {subjectId, subjectName, legalEntityName, enabled}
- `POST /billing-subjects`（payment.write，审计 billing_subject.create）DTO: subjectId(1–64 `[a-z0-9_]` 唯一→CONFLICT), subjectName(1–128), legalEntityName(1–255), enabled(默认true)。
- `GET /cashier/merchant-accounts`（可选 providerId/subjectId/enabled）→ secret 恒 `"masked"`。
- `POST /cashier/merchant-accounts`（payment.write，审计 merchant_account.create，detail 脱敏）DTO: merchantAccountId(唯一), providerId(存在且enabled), subjectId(存在且enabled), merchantId, merchantName, configJson(按 provider 模板 validation_rules 校验), secrets(按 secret_fields_json，加密为 secret_ciphertext，不回明文), enabled。
- `GET /games/{gameId}/payment-routes`（按当前 env）→ { gameId, env, groups[]: { payWayId, payWayName, payWayType, routes[]: {id, selector{package,channel,market,country,currency}, providerId, merchantAccountId, priority, enabled} } }，组内按 §排序 顺序返回（前端不自行排序）。
- `PUT /games/{gameId}/payment-routes`（payment.write，审计 payment_route.update）**整组替换**（当前 env schema 内按 game 全量覆盖，不允许跨 schema 写）。保存前执行唯一性+自洽校验，任一失败整体回滚。
  - items[] DTO: marketCode(∈枚举,默认*), countryCode(默认*), currency(默认*), channelId(string|null,null=*,非空须存在且enabled), packageCode(string|null,null=*,非空须属本game同schema), payWayId(存在且enabled), providerId(存在且enabled), merchantAccountId(存在且enabled且provider==providerId), priority(默认100,同payWay生效集唯一), enabled(默认true)。

错误码：`ROUTE_CONFLICT`(409，details.kind=duplicate_priority|duplicate_selector)、`VALIDATION_FAILED`、`CONFLICT`(唯一标识冲突)。

## 应用服务 PaymentRouteService（internal/app，编排，不放纯规则）
```go
type PaymentRouteService interface {
    ListPayWays(ctx, filter) ([]dto.PayWayDTO, error)
    ListProviders(ctx, filter) ([]dto.ProviderDTO, error)
    ListBillingSubjects(ctx, filter) ([]dto.BillingSubjectDTO, error)
    ListMerchantAccounts(ctx, filter) ([]dto.MerchantAccountDTO, error)   // secret 脱敏
    GetGameRoutes(ctx, gameID) (dto.GameRoutesDTO, error)                 // 当前 schema，分组+排序
    CreateBillingSubject(ctx, cmd) (dto.BillingSubjectDTO, error)
    CreateMerchantAccount(ctx, cmd) (dto.MerchantAccountDTO, error)        // 加密 secret
    SaveGameRoutes(ctx, gameID, cmd) (dto.GameRoutesDTO, error)           // 三态映射→ValidateRouteSet→自洽校验→全量替换→审计
    ResolveRoute(ctx, gameID, input payment.MatchInput) (payment.RouteTarget, error)
}
```
仓储窄（单聚合 CRUD + `ListEnabledRoutes(gameID, payWayID)`）；跨表自洽校验/加密/唯一性都在 service。env 由 ctx 设定的 search_path 决定，业务表 SQL 不写 schema 前缀/不带 env 谓词。

## 前端（/payment 路由分组，抽屉式）
页面：`/pay-ways`（只读）、`/providers`（只读）、`/billing-subjects`（列表+抽屉）、`/merchant-accounts`（列表+模板驱动抽屉+密文）；**游戏支付路由在 `/games/:gameId` 详情"支付路由"Tab（游戏级+env）**。
- 商户账户抽屉：选 provider → 拉该 provider enabled 最新 template_version 四件套 → 统一模板渲染器渲染；secret 字段 password 组件、回显 `masked`、留空=不修改；file 字段走统一上传。
- 路由编辑器：按 pay_way 分组折叠成"优先级链路"，组内顺序直接用后端返回顺序；兜底路由（全 `*` 或仅 GLOBAL/`*`）打"兜底"徽标垫底；单一抽屉表单 5 个作用域字段各一行（任意`*`/指定二选一）；切 PSP 用 `⋯`→"切换通道"仅暴露 provider+merchant_account（merchant 按 provider 过滤）；`ROUTE_CONFLICT` 高亮两条冲突行并区分类型；production 隐藏 Sync 入口；无 payment.write 置灰。
- 引用了已禁用基础数据的存量路由：行内标红"引用对象已禁用"，不自动删除。

## 与公共能力 / 下游
- 模板四件套(00 §4)：cashier_provider_templates 驱动商户表单。密文(00 §6)：secret_ciphertext 加密落库/响应脱敏/同步预览 masked/复制清空。审计(00 §8)：主体/商户/路由写操作。
- snapshot：按 per-game per-market 调 ResolveRoute 写运行时配置；禁用/无效不进快照。
- sync：路由属 `payments` section；execute 携带 baseline 复核 hash（D6）。

## 关键假设
- payment_routes 唯一约束由 v2 新迁移在各环境 schema 补齐；不带 env 列。
- 商户密钥本期平台级全 env 共享。
- 同 pay_way 下完全相同选择器禁止共存（无按比例分流/AB）；无生效时间窗，切换即时生效。
- ResolveRoute 无候选 → NOT_FOUND，回退由收银台/客户端决定。
