---
id: game
code: "11"
title: 游戏主数据（Game Core）
status: target
code_paths:
  - services/admin-api/internal/domain/game
  - services/admin-api/internal/transport/http/games
  - apps/admin-web/src/views/games
depends_on: [common]
impacts: [channel, account-auth, channel-login, feature-plugin, product, cashier-template, game-cashier, payment, snapshot, sync, testing]
children: []
---

# 11 · 游戏主数据（Game Core）

> 本文件是 v2 架构文档集的模块文档之一，默认继承 `../../00-common.md`（以下简称「00 公共」）与 `../../01-structure.md`（以下简称「01 结构」）的全部约定。
> 凡 00 公共已定义的：env 模型（schema-per-env）、统一响应包络、统一错误码、分页、鉴权与权限码、审计、命名与默认值兜底，本文不再重复，仅在需要落地时引用。
> 若本文与 00 公共冲突，以 00 公共为准；本文只在公共基座上**追加**游戏主数据模块的私有约定。

涉及的真实数据表：`games`、`game_markets`、`game_legal_links`（DDL 见 `services/admin-api/migrations/000001_init.up.sql`）。按已锁定决策 **D1（单库 + 每环境独立 schema）**，这三张游戏维度业务表在 `develop` / `sandbox` / `production` 三个环境 schema 下各有一份**同名、同结构、不带 `env` 列**的表，行属于哪个 env 由其所在 schema 决定；运行时由 `search_path = <当前env>, platform` 路由。

涉及的后端 6 个接口：

```text
GET   /api/admin/games
POST  /api/admin/games
GET   /api/admin/games/{gameId}
PATCH /api/admin/games/{gameId}
PUT   /api/admin/games/{gameId}/markets
PUT   /api/admin/games/{gameId}/legal-links
```

---

## 1. 模块概述与边界

### 1.1 模块定位

「游戏主数据」是整个发行后台的**根聚合**与一切下游配置的挂载点。它对应领域层 `internal/domain/game`（见 01 结构 §4）的 `Game` 聚合根，承载一个游戏在**某个 env（对应一个环境 schema）** 下的：

- **基础信息**：`name` / `alias` / `iconUrl` / `status`、自动生成的 `gameId` / `gameSecret`、默认市场 `defaultMarketCode`。
- **发行市场集合（markets）**：该游戏启用了哪些 `Market`（`GLOBAL/JP/KR/SEA/HMT/CN`），以及每个市场的默认语言、是否默认市场、是否启用。
- **法务链接（legalLinks）**：按 `default/market/locale` 三种作用域维护的服务条款 / 隐私政策 / 账号注销链接。

### 1.2 在依赖图中的位置（见 01 结构 §7）

```text
公共(00) ──被依赖──> [游戏主数据 11] ──被依赖──> 渠道实例(12) / 账号认证(13) / 渠道登录(14)
                          │                       商品与IAP(15) / 游戏级收银台(17) / 支付路由(18)
                          └──────────────────────> 配置快照(19) ──> Sandbox→Production 同步(20)
```

- **上游**：仅依赖 00 公共（枚举、env、密文脱敏、审计、包络）。不依赖任何业务模块。
- **下游**：渠道实例（`channel`）依赖本模块产出的 **market 集合**；几乎所有 per-game 业务表都以 `games.id` 为外键根。`gameId` 是同步（`sync`）、快照（`snapshot`）的聚合主键。
- 因此本模块是**最先要落地**的业务模块；它的 schema-per-env 模型、唯一键、ID 生成规则会被所有下游沿用。

### 1.3 模块职责（做什么）

1. 在**当前运行环境（env，对应一个 schema）**下创建 / 列表 / 查看 / 编辑游戏基础信息。
2. 自动生成不可变的对外标识 `game_id` 与对外密钥 `game_secret`（响应脱敏）。
3. 维护游戏的发行市场集合（多 market，默认 `GLOBAL`），保证「恰好一个默认市场」。
4. 维护游戏的法务链接（三种 scope），保证作用域唯一。

### 1.4 模块边界（不做什么 / 红线）

- **不**管理渠道实例、渠道包、登录/IAP 配置（属 `channel`～`feature-plugin`，仅提供 market 集合给它们消费）。
- **不**管理收银台模板、价格矩阵、支付路由（属 `cashier-template` / `game-cashier` / `payment`，仅提供 `gameId` 根）。
- **不**生成配置快照、不执行同步（属 `snapshot` / `sync`）。
- **不**允许前端指定写入的 `env` / schema：写操作一律落当前运行环境对应的 schema（由 `search_path` 决定，00 公共 §2.1），不允许跨 schema 写。
- **不**回明文 `game_secret`：任何响应一律脱敏（00 公共 §6.1）。
- **不**允许修改 `game_id` / `game_secret`（创建后不可变；密钥仅支持后续「重置」类操作，本期不实现）。

---

## 2. 领域模型与聚合

### 2.1 Game 聚合（聚合根）

`Game` 聚合根聚合三类实体/值对象，聚合边界 = 一个游戏在一个 env（一个环境 schema）下的全部主数据：

```text
Game (聚合根, 对应当前环境 schema 下的 games 行)
 ├─ 基础信息 (值对象集合)
 │    gameId / gameSecret / name / alias / iconUrl
 │    defaultMarketCode / status / createdAt / updatedAt
 ├─ markets: []GameMarket          (对应 game_markets 行集合)
 │    marketCode / isDefault / enabled / defaultLocale
 └─ legalLinks: []GameLegalLink    (对应 game_legal_links 行集合)
      scopeType / scopeValue / termsUrl / privacyUrl / deleteAccountUrl
```

> 业务表不带 `env` 列；聚合所属的 env 由运行时 `search_path`（当前环境 schema）隐式决定，不作为字段落库。

聚合一致性规则（由 `domain/game` 纯逻辑 + `app` 服务共同保证）：

- 聚合内所有行天然同属一个环境 schema（无 `env` 列），父子必然同 env，无需任何 env 一致性校验。
- `markets` 中**有且仅有一个** `isDefault = true`，且其 `marketCode == games.default_market_code`。
- `markets` 中 `marketCode` 不重复（受 DB 唯一键兜底）。
- `legalLinks` 中 `(scopeType, scopeValue)` 不重复（受 DB 唯一键兜底）。
- `gameId` / `gameSecret` 创建时生成且不可变。

### 2.2 子实体：GameMarket

表示「游戏在某个 market 下的发行视角开关」。它是渠道实例（`channel` 的 `GameMarketChannel`）能否挂载到该 market 的**前置开关**：只有 `game_markets` 里存在且 `enabled=true` 的 market，渠道实例才允许在该 market 下创建。

> 注意：`GameMarket` 只承载「市场是否启用 + 默认语言 + 是否默认」这层游戏级开关；market 下的具体渠道实例配置属于 `channel`，不在本聚合内。

### 2.3 子实体：GameLegalLink

表示游戏的法务链接，按作用域分三层（`LegalScopeType`）：

- `default`：兜底，`scopeValue` 固定 `*`，每个游戏**至多一条**。
- `market`：按发行大区覆盖，`scopeValue` 取某个 `Market` 值（如 `JP`）。
- `locale`：按语言覆盖，`scopeValue` 取语言标签（如 `ja-JP`）。

运行时取用优先级（客户端最终配置，由 `snapshot` 合并，本模块仅定义数据）：`locale` > `market` > `default`。本模块只负责存储与唯一性，不负责合并。

### 2.4 与下游聚合的关系

| 下游 | 依赖本模块的什么 | 关系 |
| --- | --- | --- |
| `GameMarketChannel`（`channel`） | `game_markets` 启用的 market 集合 | 新增渠道实例时校验目标 market 已在本游戏启用 |
| 账号认证 / 登录 / IAP（`account-auth` / `channel-login` / `product`） | `games.id` + market | 以游戏根 + market 维度挂载 |
| 商品（`product`） | `games.id` | `products.game_id_ref` |
| 收银台 / 支付路由（`game-cashier` / `payment`） | `games.id` + `defaultMarketCode` | 路由 market 匹配兜底 |
| 配置快照（`snapshot`） | 整聚合 + market 合并规则 | 按 market 生成最终配置 |
| 同步（`sync`） | `gameId` + section `game/markets/legal` | 按 section diff |

---

## 3. 数据模型（逐表逐字段）

> 说明：以下 3 张表的「原始 DDL」来自 `migrations/000001_init.up.sql`；本文按 **D1（单库 + 每环境独立 schema）** 给出「v2 目标结构」。这三张游戏维度业务表在 `develop` / `sandbox` / `production` 三个环境 schema 下**各建一份同名、同结构、不带 `env` 列**的表（通过新增迁移文件在各 schema 内创建，不改历史迁移语义，见 01 结构 §6）。唯一键**不再前置 `env`**，唯一性天然按 schema 隔离；运行时由 `search_path = <当前env>, platform` 决定落到哪个 schema，仓储 SQL 不写 schema 前缀、不带 `env` 谓词。下方 DDL 省略 schema 限定名（实际在每个环境 schema 内重复建一次）。

### 3.1 表 `games`（游戏基础信息）

v2 目标结构：

```sql
-- 在每个环境 schema（develop/sandbox/production）内各建一份同名同结构表
CREATE TABLE games (
  id                  BIGSERIAL PRIMARY KEY,
  game_id             VARCHAR(64)  NOT NULL,
  game_secret         VARCHAR(128) NOT NULL,
  name                VARCHAR(128) NOT NULL,
  alias               VARCHAR(64)  NOT NULL,
  icon_url            VARCHAR(512) NOT NULL DEFAULT '',
  default_market_code VARCHAR(32)  NOT NULL DEFAULT 'GLOBAL',
  status              VARCHAR(32)  NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft','active','disabled')), -- 见 §4，建议补 CHECK
  created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (game_id),    -- schema 内唯一（每环境一份，天然按 env 隔离）
  UNIQUE (alias)       -- alias 在所属环境 schema 内唯一（见 §5.4）
);
```

逐字段说明：

| 列 | 类型 | 默认 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 自增 | PK | 内部主键，仅用于外键引用，**不对外暴露**；各环境 schema 内独立自增 |
| `game_id` | VARCHAR(64) | 无（自动生成） | NOT NULL, `UNIQUE(game_id)` | 对外游戏标识，创建时生成，**不可变**（生成规则见 §5.1） |
| `game_secret` | VARCHAR(128) | 无（自动生成） | NOT NULL | 对外密钥，创建时生成，**响应一律脱敏**（§5.2、§6.1） |
| `name` | VARCHAR(128) | 无 | NOT NULL | 游戏展示名，可改 |
| `alias` | VARCHAR(64) | 无 | NOT NULL, `UNIQUE(alias)` | 简称/代号，在所属环境 schema 内唯一（§5.4），可改但需校验唯一 |
| `icon_url` | VARCHAR(512) | `''` | NOT NULL | 图标 URL，可空字符串 |
| `default_market_code` | VARCHAR(32) | `'GLOBAL'` | NOT NULL | 默认市场，必须 ∈ 该游戏已启用 markets，且对应行 `isDefault=true`（§5.6） |
| `status` | VARCHAR(32) | `'draft'` | NOT NULL, CHECK in `draft/active/disabled` | 游戏状态（§4）；建议新增迁移补 CHECK（原 DDL 无 CHECK） |
| `created_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | 创建时间 |
| `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | 更新时间，任意写操作刷新 |

唯一键（D1，每环境 schema 内一份，唯一性按 schema 天然隔离）：

- `UNIQUE (game_id)`：不再前置 `env`；不同环境 schema 下可存在同名 `game_id`（即同步对齐的同一逻辑游戏）。
- `UNIQUE (alias)`：落地 §5.4 的「alias 在所属环境 schema 内唯一」业务规则；DDL 原表无此键，由新增迁移补齐。

### 3.2 表 `game_markets`（游戏发行市场）

v2 目标结构：

```sql
-- 在每个环境 schema（develop/sandbox/production）内各建一份同名同结构表
CREATE TABLE game_markets (
  id             BIGSERIAL PRIMARY KEY,
  game_id_ref    BIGINT      NOT NULL REFERENCES games(id),  -- 同 schema 内普通外键
  market_code    VARCHAR(32) NOT NULL
                   CHECK (market_code IN ('GLOBAL','JP','KR','SEA','HMT','CN')), -- 见 §4，建议补 CHECK
  is_default     BOOLEAN     NOT NULL DEFAULT FALSE,
  enabled        BOOLEAN     NOT NULL DEFAULT TRUE,
  default_locale VARCHAR(16) NOT NULL DEFAULT 'en-US',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, market_code)   -- schema 内唯一（不再前置 env）
);
```

逐字段说明：

| 列 | 类型 | 默认 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 自增 | PK | 内部主键 |
| `game_id_ref` | BIGINT | 无 | NOT NULL, FK→`games(id)` | 所属游戏；同 schema 内普通外键，父子必然同 env，无需 env 一致性校验（00 §2.2） |
| `market_code` | VARCHAR(32) | 无 | NOT NULL, CHECK in `Market` 枚举 | 市场代码，∈ `GLOBAL/JP/KR/SEA/HMT/CN`；建议新增迁移补 CHECK |
| `is_default` | BOOLEAN | `FALSE` | NOT NULL | 是否默认市场；每游戏恰好一条为 `true`（§5.6） |
| `enabled` | BOOLEAN | `TRUE` | NOT NULL | 是否启用该市场；禁用后下游不可在此 market 建渠道实例 |
| `default_locale` | VARCHAR(16) | `'en-US'` | NOT NULL | 该市场默认语言（00 §10 兜底 `en-US`） |
| `created_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | 创建时间 |
| `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | 更新时间 |

唯一键（D1）：`UNIQUE (game_id_ref, market_code)`，不再前置 `env`；每环境 schema 内一份，唯一性按 schema 天然隔离。

### 3.3 表 `game_legal_links`（游戏法务链接）

v2 目标结构：

```sql
-- 在每个环境 schema（develop/sandbox/production）内各建一份同名同结构表
CREATE TABLE game_legal_links (
  id                 BIGSERIAL PRIMARY KEY,
  game_id_ref        BIGINT       NOT NULL REFERENCES games(id),  -- 同 schema 内普通外键
  scope_type         VARCHAR(16)  NOT NULL
                       CHECK (scope_type IN ('default','market','locale')),
  scope_value        VARCHAR(32)  NOT NULL DEFAULT '*',
  terms_url          VARCHAR(512) NOT NULL DEFAULT '',
  privacy_url        VARCHAR(512) NOT NULL DEFAULT '',
  delete_account_url VARCHAR(512) NOT NULL DEFAULT '',
  created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, scope_type, scope_value)  -- schema 内唯一（不再前置 env）
);
```

逐字段说明：

| 列 | 类型 | 默认 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 自增 | PK | 内部主键 |
| `game_id_ref` | BIGINT | 无 | NOT NULL, FK→`games(id)` | 所属游戏；同 schema 内普通外键，父子必然同 env，无需 env 一致性校验（00 §2.2） |
| `scope_type` | VARCHAR(16) | 无 | NOT NULL, CHECK in `default/market/locale` | 作用域类型（`LegalScopeType`，§4） |
| `scope_value` | VARCHAR(32) | `'*'` | NOT NULL | 作用域取值：`default`→`*`；`market`→Market 值；`locale`→语言标签（§5.5） |
| `terms_url` | VARCHAR(512) | `''` | NOT NULL | 服务条款 URL |
| `privacy_url` | VARCHAR(512) | `''` | NOT NULL | 隐私政策 URL |
| `delete_account_url` | VARCHAR(512) | `''` | NOT NULL | 账号注销 URL |
| `created_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | 创建时间 |
| `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | 更新时间 |

唯一键（D1）：`UNIQUE (game_id_ref, scope_type, scope_value)`，不再前置 `env`；每环境 schema 内一份，唯一性按 schema 天然隔离。

### 3.4 跨表关系与 env 隔离（D1 通用规则）

- 三表均**不带 `env` 列**，在每个环境 schema 内各一份；`game_markets`、`game_legal_links` 与其 `game_id_ref` 指向的 `games` 行**必然在同一 schema**（同一 env），父子天然同 env。
- 业务表→业务表用**同一 schema 内的普通外键**即可，**无需复合 env 外键，也无需应用层 env 一致性校验**。
- 不再需要 `WHERE env = $currentEnv` 谓词：运行时 `search_path = <当前env>, platform` 已将查询定位到当前环境 schema，仓储 SQL 不写 schema 前缀、不带 env 谓词，天然不跨环境串读。
- `sandbox → production` 的跨 schema 复制由 `sync` 域负责（显式跨 schema 读写），本模块不直接处理跨 schema 写。

---

## 4. 枚举与默认值清单（穷尽）

> 本清单以 00 公共 §3 为唯一事实来源，下表是本模块**实际用到的子集**及其落库默认值。后端 `internal/domain/common` 与前端 `dictionary` store 必须与此一致。

### 4.1 枚举

| 枚举名 | 取值（穷尽） | 默认值 | 落库列 | 说明 |
| --- | --- | --- | --- | --- |
| `Environment` | `develop` / `sandbox` / `production` | `develop`（由 `APP_ENV` 决定，缺省 develop） | 不落列（由所在 schema 表达，业务表无 `env` 列） | 运行/数据环境，对应 `develop`/`sandbox`/`production` 三个 schema；由 `search_path` 路由，写入不可由前端指定 |
| `Market` | `GLOBAL` / `JP` / `KR` / `SEA` / `HMT` / `CN` | `GLOBAL` | `games.default_market_code` / `game_markets.market_code` | 发行大区，非国家；一个游戏可多 market |
| `GameStatus` | `draft` / `active` / `disabled` | `draft` | `games.status` | 游戏生命周期状态 |
| `LegalScopeType` | `default` / `market` / `locale` | `default` | `game_legal_links.scope_type` | 法务链接作用域 |

### 4.2 Market 语义补充（继承 00 §3.2）

- `GLOBAL`：默认兜底海外市场；**不匹配 `CN`**；仅显示 overseas 渠道（下游 12 用）。
- `CN`：仅中国大陆，仅允许 domestic 渠道。
- `JP / KR / SEA / HMT`：具体海外大区，仅允许 overseas 渠道；与 `GLOBAL` 并存时**整体覆盖** `GLOBAL`（实例级，`snapshot` 合并）。
- 本模块只负责「启用哪些 market」，不负责渠道可见性过滤（属 `channel`）。

### 4.3 GameStatus 语义与建议状态机

| 状态 | 含义 | 进入条件 |
| --- | --- | --- |
| `draft` | 草稿（新建默认） | `POST /games` 创建后默认 |
| `active` | 已激活/正常发行 | 运营在详情页显式激活 |
| `disabled` | 停用 | 运营显式停用 |

建议流转（本模块内宽松，不强制单向）：`draft ⇄ active`、`active ⇄ disabled`、`draft → disabled`。本期通过 `PATCH /games/{gameId}` 的 `status` 字段直接改写，不强制状态机校验；若后续需要锁死，再在 `domain/game` 增加流转校验。

### 4.4 字段级默认值（穷尽，落库时生效）

| 字段 | 默认值 | 来源 |
| --- | --- | --- |
| `games.icon_url` | `''` | 00 §10「URL 类默认空串」 |
| `games.default_market_code` | `'GLOBAL'` | 本模块 + Market 默认 |
| `games.status` | `'draft'` | GameStatus 默认 |
| `game_markets.is_default` | `FALSE` | 00 §10（除创建默认市场时显式置 `TRUE`） |
| `game_markets.enabled` | `TRUE` | 00 §10「`enabled=TRUE`」 |
| `game_markets.default_locale` | `'en-US'` | 00 §10「`default_locale=en-US`」 |
| `game_legal_links.scope_value` | `'*'` | DDL 默认（`default` scope 用） |
| `game_legal_links.terms_url` / `privacy_url` / `delete_account_url` | `''` | 00 §10 URL 类默认空串 |
| `created_at` / `updated_at`（三表） | `NOW()` | 00 §10 |

---

## 5. 业务规则

### 5.1 `game_id` 自动生成规则

- 由服务端在 `POST /games` 时生成，**前端不可传入**；若前端传入 `gameId` 一律忽略。
- 规则：在当前环境 schema 内，从基数 `100000` 起的**自增数字字符串**（与 `go_domain_api_draft.md` 示例 `"100001"` 一致），保证 schema 内 `UNIQUE(game_id)`。
  - 实现建议：每个环境 schema 维护一个序列（独立 sequence 或在 `domain/game` 内基于「当前 schema 内最大 game_id + 1」生成并在事务内校验唯一），冲突则重试。
  - 不同环境 schema 各自从 `100000` 起独立计数（唯一性按 schema 隔离）。
- `game_id` 创建后**不可变**：`PATCH /games/{gameId}` 不接受修改 `gameId`。

### 5.2 `game_secret` 自动生成与脱敏规则

- 由服务端在 `POST /games` 时生成：长度 ≤ 128 的高熵随机串（建议 base62/hex，≥ 32 字节熵）。
- **写入**：可直接存明文于 `game_secret`（它是「对外下发给游戏客户端/SDK 的标识密钥」，非平台内部加密密钥，因此 DDL 用普通 `VARCHAR(128)` 而非 `*_ciphertext`）。若安全策略要求，可在新增迁移中改为密文列；本期沿用 DDL 原样存储。
- **响应**：**一律脱敏**（00 §6.1），统一返回 `"masked"`（与 `go_domain_api_draft.md` 示例一致），绝不回明文。仅创建成功时可一次性返回明文供运营记录（见 §6.2 的 `POST /games` 响应约定）。
- `game_secret` 创建后**不可变**（本期不提供重置接口；后续如需「重置密钥」单开接口 + `game.write` 权限 + 审计）。

### 5.3 默认创建 GLOBAL 市场规则

- `POST /games` 时若请求未显式提供 `markets`，服务端**自动创建一条 `GLOBAL` 市场**：`marketCode='GLOBAL'`、`isDefault=true`、`enabled=true`、`defaultLocale='en-US'`。
- 若请求提供了 `markets` 但**未包含** `defaultMarketCode` 指定的市场，服务端必须自动补入该默认市场行（保证默认市场必然存在）。
- 若请求既未提供 `markets` 也未提供 `defaultMarketCode`，则 `defaultMarketCode` 取 `GLOBAL` 并据此建默认市场。
- 创建出的 markets 中，`marketCode == defaultMarketCode` 的那条 `isDefault=true`，其余 `false`。

### 5.4 alias 唯一规则

- `alias` 在**所属环境 schema 内全局唯一**（落地为 `UNIQUE(alias)`，§3.1）。
- `POST` / `PATCH` 写入 alias 时，服务端先查重；冲突返回 `409 CONFLICT`（`message: "alias already exists"`，对齐 00 §7.2 示例）。
- 不同环境 schema 间 alias 可重复（唯一性按 schema 隔离）。

### 5.5 legal scope 唯一与取值规则

- `(scopeType, scopeValue)` 在「同一游戏」内唯一（落地 `UNIQUE(game_id_ref, scope_type, scope_value)`，§3.3；同一游戏行天然只在一个环境 schema 内）。
- 取值约束（服务端校验）：
  - `scopeType=default` ⇒ `scopeValue` 必须为 `'*'`，且每游戏至多一条。
  - `scopeType=market` ⇒ `scopeValue` 必须 ∈ `Market` 枚举（`GLOBAL/JP/KR/SEA/HMT/CN`）。
  - `scopeType=locale` ⇒ `scopeValue` 必须是合法语言标签（如 `ja-JP`、`en-US`；格式校验 `xx-XX` 或 `xx`）。
- 违反取值约束返回 `400 VALIDATION_FAILED`；重复返回 `409 CONFLICT`。

### 5.6 market 唯一与默认市场规则

- 同一游戏同一 env 下 `marketCode` 不重复（§3.2 唯一键）。
- **恰好一个默认市场**：`game_markets` 中 `isDefault=true` 的行**有且仅有一条**，且其 `marketCode == games.default_market_code`。
- 通过 `PUT /games/{gameId}/markets` 全量覆盖 markets 时，服务端校验：
  - 列表非空；
  - `marketCode` 取值合法且不重复；
  - 恰好一条 `isDefault=true`（或由 `defaultMarketCode` 推导）；
  - 同步回写 `games.default_market_code`。
- 一个游戏可启用多个 market（多 market），无数量上限（受枚举 6 值约束）。

### 5.7 env 行为规则（D1，schema-per-env）

- 所有写操作落**当前运行环境对应的 schema**（由 `search_path` 决定，源自 `APP_ENV`），前端**不可指定/跨 schema 写**。
- `game_markets` / `game_legal_links` 与父 `games` 行天然同 schema（同 env），普通外键即可，无需 env 注入与一致性校验。
- 所有读取（列表/详情）由 `search_path` 自动定位到当前环境 schema，仓储 SQL 不写 schema 前缀、不带 env 谓词，不跨 env 返回。
- `sandbox → production` 的跨 schema 复制由 `sync`「同步」负责（section = `game/markets/legal`，同库内跨 schema diff/upsert），本模块不直接处理跨 schema 写。

### 5.8 多 market 管理规则

- 新增 market：通过 `PUT /games/{gameId}/markets` 全量提交期望的 market 集合（含新加项）。
- 移除 market：从 `PUT` 提交的集合中剔除即可；但若该 market 下已存在渠道实例（`channel`）或被设为默认市场，则**拒绝移除**（返回 `409 CONFLICT`，提示存在下游依赖或不可移除默认市场）。
- 切换默认市场：在 `PUT` 提交中改变 `isDefault` 指向，服务端同步 `games.default_market_code`。

### 5.9 审计（继承 00 §8）

本模块所有写操作写 `audit_logs`：

| 操作 | `action` | `resource_type` | `resource_id` |
| --- | --- | --- | --- |
| 创建游戏 | `game.create` | `game` | `gameId` |
| 编辑基础信息 | `game.update` | `game` | `gameId` |
| 覆盖 markets | `game.markets.update` | `game` | `gameId` |
| 覆盖 legalLinks | `game.legal.update` | `game` | `gameId` |

`detail_json` 记录关键 before/after；`game_secret` 在 detail 中也必须脱敏。

---

## 6. 后端 API

> 统一遵循 00 公共 §7：前缀 `/api/admin`、`Authorization: Bearer <accessToken>`、`application/json; charset=utf-8`、字段 camelCase、时间 ISO-8601 UTC、统一响应包络、统一错误码、写操作落当前运行环境对应 schema（由 `search_path` 决定）。下列每个写接口都标注权限码（00 §7.5）。

### 6.0 权限码与错误码总表

| 接口 | 方法 | 权限码 | 主要错误码 |
| --- | --- | --- | --- |
| 列表 | GET `/games` | `game.read` | `UNAUTHENTICATED` / `FORBIDDEN` |
| 创建 | POST `/games` | `game.write` | `VALIDATION_FAILED` / `CONFLICT` |
| 详情 | GET `/games/{gameId}` | `game.read` | `NOT_FOUND` |
| 编辑 | PATCH `/games/{gameId}` | `game.write` | `VALIDATION_FAILED` / `CONFLICT` / `NOT_FOUND` |
| 覆盖市场 | PUT `/games/{gameId}/markets` | `game.write` | `VALIDATION_FAILED` / `CONFLICT` / `NOT_FOUND` |
| 覆盖法务 | PUT `/games/{gameId}/legal-links` | `game.write` | `VALIDATION_FAILED` / `CONFLICT` / `NOT_FOUND` |

通用错误响应（00 §7.2）：

```json
{ "error": { "code": "VALIDATION_FAILED", "message": "alias already exists", "details": [] } }
```

> 路径参数 `{gameId}` 指对外 `game_id`（如 `100001`），**不是**内部 `id`。服务端在当前环境 schema 内按 `game_id` 定位（schema 由 `search_path` 决定）。

### 6.1 GET `/api/admin/games` — 游戏列表

- 权限：`game.read`。
- Query（继承 00 §7.3 分页）：

| 参数 | 类型 | 默认 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| `page` | int | `1` | ≥ 1 | 页码 |
| `pageSize` | int | `20` | ≤ 100 | 每页条数 |
| `sort` | string | `-updatedAt` | 见 00 §7.3 | 排序，缺省按 `updatedAt` 降序 |
| `keyword` | string | `''` | ≤ 64 | 模糊匹配 `name` / `alias` / `gameId` |
| `status` | enum | 无 | ∈ GameStatus | 按状态过滤，不传=全部 |
| `marketCode` | enum | 无 | ∈ Market | 仅返回启用了该 market 的游戏 |

- 隐式作用域：当前环境 schema（由 `search_path` 决定），不跨 env。
- 列表项为「轻量摘要」（不展开 legalLinks，markets 仅给计数与默认市场）。

请求示例：

```text
GET /api/admin/games?page=1&pageSize=20&keyword=project&status=active&sort=-updatedAt
```

成功响应（200）：

```json
{
  "data": {
    "items": [
      {
        "gameId": "100001",
        "name": "Project A",
        "alias": "pa",
        "iconUrl": "https://cdn.example.com/icon/pa.png",
        "status": "active",
        "defaultMarketCode": "GLOBAL",
        "marketCodes": ["GLOBAL", "JP"],
        "marketCount": 2,
        "createdAt": "2026-06-10T08:00:00Z",
        "updatedAt": "2026-06-15T10:00:00Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 1
  }
}
```

> 列表项**不返回** `gameSecret`（即便脱敏也不在列表带出，降低暴露面）。

### 6.2 POST `/api/admin/games` — 创建游戏

- 权限：`game.write`。
- 请求体 DTO（`CreateGameRequest`）：

| 字段 | 类型 | 必填 | 默认 | 校验 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `name` | string | 是 | — | 1–128 字符 | 展示名 |
| `alias` | string | 是 | — | 1–64 字符，`^[a-zA-Z0-9_-]+$`，同 env 唯一 | 简称 |
| `iconUrl` | string | 否 | `''` | ≤ 512，format=url（非空时） | 图标 |
| `defaultMarketCode` | enum | 否 | `GLOBAL` | ∈ Market | 默认市场 |
| `status` | enum | 否 | `draft` | ∈ GameStatus | 初始状态 |
| `markets` | string[] | 否 | `["GLOBAL"]` | 每项 ∈ Market，去重 | 启用的市场集合（§5.3） |

- 服务端行为：
  1. 校验 `alias` 唯一（§5.4）、`name` 非空、`markets`/`defaultMarketCode` 合法。
  2. 生成 `gameId`（§5.1）、`gameSecret`（§5.2）。
  3. 落到当前运行环境对应 schema（由 `search_path` 决定，前端不可指定/跨 schema 写）。
  4. 建 markets：未传则 `["GLOBAL"]`；保证含 `defaultMarketCode`；该市场 `isDefault=true`、`enabled=true`、`defaultLocale='en-US'`（§5.3、§5.6）。
  5. 写 `audit_logs`（`game.create`）。
- **忽略**前端传入的 `gameId` / `gameSecret` / 任何环境/schema 指定（一律落当前运行环境 schema）。

请求示例（对齐 `go_domain_api_draft.md`）：

```json
{
  "name": "Project A",
  "alias": "pa",
  "defaultMarketCode": "GLOBAL",
  "iconUrl": "",
  "markets": ["GLOBAL", "JP"]
}
```

成功响应（201）——**仅创建时**一次性返回明文 `gameSecret` 供运营记录，之后任何接口均脱敏：

```json
{
  "data": {
    "gameId": "100001",
    "name": "Project A",
    "alias": "pa",
    "gameSecret": "sk_live_8f3c9a1b7d2e4f60a1b2c3d4e5f60718",
    "secretMasked": false,
    "iconUrl": "",
    "status": "draft",
    "defaultMarketCode": "GLOBAL",
    "markets": [
      { "marketCode": "GLOBAL", "isDefault": true, "enabled": true, "defaultLocale": "en-US" },
      { "marketCode": "JP", "isDefault": false, "enabled": true, "defaultLocale": "en-US" }
    ],
    "legalLinks": [],
    "createdAt": "2026-06-15T10:00:00Z",
    "updatedAt": "2026-06-15T10:00:00Z"
  }
}
```

错误示例（alias 冲突，409）：

```json
{ "error": { "code": "CONFLICT", "message": "alias already exists", "details": [{ "field": "alias", "value": "pa" }] } }
```

校验失败示例（400）：

```json
{ "error": { "code": "VALIDATION_FAILED", "message": "invalid request", "details": [
  { "field": "name", "reason": "required" },
  { "field": "markets[1]", "reason": "must be one of GLOBAL/JP/KR/SEA/HMT/CN" }
] } }
```

### 6.3 GET `/api/admin/games/{gameId}` — 游戏详情

- 权限：`game.read`。
- 路径参数：`gameId`（对外 game_id）。
- 行为：在当前环境 schema 内按 `gameId` 查询（schema 由 `search_path` 决定）；不存在返回 `404 NOT_FOUND`。
- 响应包含完整聚合：基础信息 + markets + legalLinks；`gameSecret` 脱敏。

成功响应（200）（对齐 `go_domain_api_draft.md` 示例并补全 env/脱敏标记）：

```json
{
  "data": {
    "gameId": "100001",
    "name": "Project A",
    "alias": "pa",
    "gameSecret": "masked",
    "secretMasked": true,
    "iconUrl": "https://cdn.example.com/icon/pa.png",
    "defaultMarketCode": "GLOBAL",
    "status": "active",
    "env": "develop",
    "markets": [
      { "marketCode": "GLOBAL", "isDefault": true, "enabled": true, "defaultLocale": "en-US" },
      { "marketCode": "JP", "isDefault": false, "enabled": true, "defaultLocale": "ja-JP" }
    ],
    "legalLinks": [
      {
        "scopeType": "default",
        "scopeValue": "*",
        "termsUrl": "https://example.com/terms",
        "privacyUrl": "https://example.com/privacy",
        "deleteAccountUrl": "https://example.com/delete-account"
      },
      {
        "scopeType": "market",
        "scopeValue": "JP",
        "termsUrl": "https://example.com/jp/terms",
        "privacyUrl": "https://example.com/jp/privacy",
        "deleteAccountUrl": "https://example.com/jp/delete-account"
      }
    ],
    "createdAt": "2026-06-10T08:00:00Z",
    "updatedAt": "2026-06-15T10:00:00Z"
  }
}
```

未找到（404）：

```json
{ "error": { "code": "NOT_FOUND", "message": "game not found", "details": [] } }
```

### 6.4 PATCH `/api/admin/games/{gameId}` — 编辑基础信息

- 权限：`game.write`。
- 仅允许修改基础信息字段；**不可改** `gameId` / `gameSecret`；游戏所属 env 由其所在 schema 决定，PATCH 不接受切换环境（不允许跨 schema 迁移）；markets 与 legalLinks 走各自专用接口。
- 请求体 DTO（`UpdateGameRequest`，部分更新，字段均可选；缺省=不改）：

| 字段 | 类型 | 校验 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 1–128 | 展示名 |
| `alias` | string | 1–64，`^[a-zA-Z0-9_-]+$`，同 env 唯一 | 改 alias 需查重（§5.4） |
| `iconUrl` | string | ≤ 512，format=url（非空时） | 图标 |
| `status` | enum | ∈ GameStatus | 状态变更（§4.3） |
| `defaultMarketCode` | enum | ∈ Market，且必须 ∈ 已启用 markets | 改默认市场；服务端同步 `game_markets.is_default`（§5.6） |

- 行为：在当前环境 schema 内按 `gameId` 定位；逐字段 patch；`alias` 变更查重；`defaultMarketCode` 变更校验其已启用并同步 is_default；刷新 `updated_at`；写 `audit_logs`（`game.update`）。

请求示例：

```json
{
  "name": "Project A (Global)",
  "status": "active",
  "defaultMarketCode": "JP"
}
```

成功响应（200）：返回与 §6.3 同结构的完整详情（`gameSecret` 脱敏）。

冲突示例（改默认市场到未启用 market，409/400）：

```json
{ "error": { "code": "VALIDATION_FAILED", "message": "defaultMarketCode must be an enabled market", "details": [{ "field": "defaultMarketCode", "value": "KR" }] } }
```

### 6.5 PUT `/api/admin/games/{gameId}/markets` — 全量覆盖市场集合

- 权限：`game.write`。
- 语义：**全量覆盖**（PUT），以请求体为最终期望集合做 diff（新增/更新/删除）。
- 请求体 DTO（`ReplaceMarketsRequest`）：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `markets` | object[] | 是 | — | 非空，`marketCode` 不重复 |
| `markets[].marketCode` | enum | 是 | — | ∈ Market |
| `markets[].isDefault` | bool | 否 | `false` | 整列表恰好一条 `true` |
| `markets[].enabled` | bool | 否 | `true` | — |
| `markets[].defaultLocale` | string | 否 | `en-US` | 语言标签格式 |

- 服务端行为（§5.6、§5.8）：
  1. 校验非空、`marketCode` 合法不重复、恰好一条 `isDefault=true`。
  2. 与现有 markets diff：新增插入、存在则更新、缺失则删除。
  3. **删除保护**：若被删除的 market 下已存在渠道实例（`channel`）或为当前默认市场，拒绝（`409 CONFLICT`）。
  4. 回写 `games.default_market_code` = `isDefault=true` 的 marketCode。
  5. 写 `audit_logs`（`game.markets.update`）。

请求示例：

```json
{
  "markets": [
    { "marketCode": "GLOBAL", "isDefault": true,  "enabled": true, "defaultLocale": "en-US" },
    { "marketCode": "JP",     "isDefault": false, "enabled": true, "defaultLocale": "ja-JP" },
    { "marketCode": "KR",     "isDefault": false, "enabled": true, "defaultLocale": "ko-KR" }
  ]
}
```

成功响应（200）：

```json
{
  "data": {
    "gameId": "100001",
    "defaultMarketCode": "GLOBAL",
    "markets": [
      { "marketCode": "GLOBAL", "isDefault": true,  "enabled": true, "defaultLocale": "en-US" },
      { "marketCode": "JP",     "isDefault": false, "enabled": true, "defaultLocale": "ja-JP" },
      { "marketCode": "KR",     "isDefault": false, "enabled": true, "defaultLocale": "ko-KR" }
    ]
  }
}
```

删除受阻示例（409）：

```json
{ "error": { "code": "CONFLICT", "message": "cannot remove market with existing channels", "details": [{ "field": "markets", "marketCode": "JP" }] } }
```

多默认市场示例（400）：

```json
{ "error": { "code": "VALIDATION_FAILED", "message": "exactly one default market is required", "details": [] } }
```

### 6.6 PUT `/api/admin/games/{gameId}/legal-links` — 全量覆盖法务链接

- 权限：`game.write`。
- 语义：**全量覆盖**（PUT），以请求体为最终期望集合做 diff。
- 请求体 DTO（`ReplaceLegalLinksRequest`）：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `legalLinks` | object[] | 是 | — | `(scopeType, scopeValue)` 不重复 |
| `legalLinks[].scopeType` | enum | 是 | — | ∈ `default/market/locale` |
| `legalLinks[].scopeValue` | string | 否 | `*` | 按 scopeType 取值约束（§5.5） |
| `legalLinks[].termsUrl` | string | 否 | `''` | ≤ 512，format=url（非空时） |
| `legalLinks[].privacyUrl` | string | 否 | `''` | ≤ 512，format=url（非空时） |
| `legalLinks[].deleteAccountUrl` | string | 否 | `''` | ≤ 512，format=url（非空时） |

- 服务端行为（§5.5）：
  1. 逐项校验 scope 取值：`default`→`scopeValue='*'` 且至多一条；`market`→∈ Market；`locale`→语言标签格式。
  2. 校验 `(scopeType, scopeValue)` 不重复。
  3. diff 写库（新增/更新/删除）。
  4. 写 `audit_logs`（`game.legal.update`）。

请求示例：

```json
{
  "legalLinks": [
    {
      "scopeType": "default",
      "scopeValue": "*",
      "termsUrl": "https://example.com/terms",
      "privacyUrl": "https://example.com/privacy",
      "deleteAccountUrl": "https://example.com/delete-account"
    },
    {
      "scopeType": "market",
      "scopeValue": "JP",
      "termsUrl": "https://example.com/jp/terms",
      "privacyUrl": "https://example.com/jp/privacy",
      "deleteAccountUrl": ""
    },
    {
      "scopeType": "locale",
      "scopeValue": "ko-KR",
      "termsUrl": "https://example.com/ko/terms",
      "privacyUrl": "https://example.com/ko/privacy",
      "deleteAccountUrl": ""
    }
  ]
}
```

成功响应（200）：返回写入后的完整 `legalLinks` 数组（结构同请求项）。

scope 取值非法示例（400）：

```json
{ "error": { "code": "VALIDATION_FAILED", "message": "invalid scopeValue for scopeType", "details": [
  { "field": "legalLinks[0].scopeValue", "reason": "scopeType=default requires scopeValue '*'" }
] } }
```

---

## 7. 应用服务与 command/query

> 遵循 01 结构 §4 分层：`domain/game`（纯逻辑）→ `app/command` `app/query`（编排）→ `infra/persistence/postgres`（pgx 仓储）→ `transport/http/games`（chi handler）。仓储保持窄（单聚合 CRUD + 必要查询），跨表编排放 app 层；仓储方法接收 `ctx`（其连接已按当前运行环境设置 `search_path = <当前env>, platform`），SQL **不写 schema 前缀、不带 `env` 谓词**，环境由连接决定。

### 7.1 领域层 `internal/domain/game`

纯逻辑（无 IO），承载聚合一致性：

- `Game` 聚合根、`GameMarket` / `GameLegalLink` 值对象。
- `GenerateGameID(env, lastSeq)` / `GenerateGameSecret()`：ID 与密钥生成纯函数（随机源由调用方注入）。
- `ValidateMarkets(markets, defaultMarketCode)`：恰好一个默认、枚举合法、不重复。
- `ValidateLegalScope(scopeType, scopeValue)`：scope 取值约束（§5.5）。
- `ApplyDefaultMarket(req)`：缺省补 `GLOBAL` 默认市场（§5.3）。
- 不做任何 DB / 网络调用。

### 7.2 应用服务 `GameService`（app 层）

按 `go_domain_api_draft.md`「按领域拆分、避免超大 GameService」原则，本模块对应 `GameService`，方法只覆盖游戏主数据：

| 方法 | 类型 | 对应接口 | 说明 |
| --- | --- | --- | --- |
| `ListGames(ctx, ListGamesQuery)` | query | GET `/games` | 分页 + 过滤，返回摘要 |
| `GetGame(ctx, gameId)` | query | GET `/games/{gameId}` | 聚合详情，secret 脱敏 |
| `CreateGame(ctx, CreateGameCmd)` | command | POST `/games` | 生成 id/secret、建默认市场、审计 |
| `UpdateGame(ctx, UpdateGameCmd)` | command | PATCH `/games/{gameId}` | 部分更新基础信息 |
| `ReplaceMarkets(ctx, ReplaceMarketsCmd)` | command | PUT `/markets` | 全量覆盖 markets |
| `ReplaceLegalLinks(ctx, ReplaceLegalLinksCmd)` | command | PUT `/legal-links` | 全量覆盖 legalLinks |

> `ctx` 携带按当前运行环境设置好 `search_path` 的连接/事务；env 由连接隐式决定，不再作为方法参数显式传入。

依赖注入：`GameRepository`（仓储）、随机源（secret/id）、`AuditLogger`、`Clock`。

### 7.3 command / query 对象（`app/dto`）

- command（写）：`CreateGameCmd` / `UpdateGameCmd` / `ReplaceMarketsCmd` / `ReplaceLegalLinksCmd`，由 transport 层从请求 DTO 映射，携带 `actorId`（当前运行环境由 `ctx` 连接的 `search_path` 决定，不在命令对象内重复携带）。
- query（读）：`ListGamesQuery`（page/pageSize/sort/keyword/status/marketCode）；返回 `GameSummaryDTO` / `GameDetailDTO`（后者含 markets+legalLinks，secret 脱敏）。
- DTO ↔ 领域对象在 app 层映射；transport 层只做 JSON ↔ DTO + 包络封装。

### 7.4 仓储接口 `GameRepository`（infra/persistence/postgres）

窄接口（单聚合），全部带 `ctx`（连接已按当前运行环境设置 `search_path`，SQL 不写 schema 前缀、不带 env 谓词）：

```text
NextGameIDSeq(ctx) (int64, error)            // 当前环境 schema 内序列
ExistsAlias(ctx, alias, excludeGameID) (bool, error)
InsertGame(ctx, game) (Game, error)          // 同事务插 games + game_markets
GetGameByGameID(ctx, gameId) (Game, error)   // 聚合装配
ListGames(ctx, query) (items, total, error)
UpdateGame(ctx, gameId, patch) (Game, error)
ReplaceMarkets(ctx, gameId, markets) error   // 事务内 diff
ReplaceLegalLinks(ctx, gameId, links) error  // 事务内 diff
CountChannelsByMarket(ctx, gameId, marketCode) (int, error) // 删除保护（跨模块只读查询，同 schema）
```

事务边界：`CreateGame`（games + 默认 market）、`ReplaceMarkets`、`ReplaceLegalLinks` 均在单事务内完成，并在事务内写审计。

### 7.5 transport 层 `transport/http/games`

- chi 子路由挂 `/api/admin/games`。
- 中间件链（01 §4 + 00 §7.5）：`recover` → 鉴权（Bearer）→ 权限（`game.read`/`game.write`）→ env 上下文注入（把当前 env 注入 ctx；`search_path` 由每环境连接池建连时钉死，中间件不逐请求 `SET`，见 `01 §4.4`）→ handler → 审计。
- handler 仅负责：解析/校验请求 DTO → 调 `GameService` → 包络化响应（`{data}` / `{error}`）。

---

## 8. 前端

> 遵循 01 结构 §5：Vue3 + Pinia + Element Plus，抽屉优先，列表/详情/抽屉/状态标签统一走 `components/page`。API 客户端在 `api/modules/games.ts`，消费统一包络解包（`http.ts`）。路由 `/games` → `/games/:gameId`（含多 Tab）。

### 8.1 游戏列表页 `/games`

- 顶部常驻 `EnvironmentBadge`（当前 env，来自 `app` store）。
- 工具栏：关键字搜索（name/alias/gameId）、`status` 筛选、`marketCode` 筛选、「新建游戏」按钮（挂 `game.write` 权限指令，无权限隐藏/置灰）。
- 表格列：`gameId`、`name`、`alias`、`status`（状态标签）、`defaultMarketCode`、market 数量/标签、`updatedAt`、操作（查看详情）。
- **不展示** `gameSecret`。
- 分页走 00 §7.3（page/pageSize/total）。
- 行点击进入详情。

### 8.2 新建游戏抽屉

- 抽屉表单字段：`name`（必填）、`alias`（必填，前端即时查重提示）、`iconUrl`（上传 wrapper）、`defaultMarketCode`（默认 `GLOBAL`）、`markets`（多选，默认含 `GLOBAL`）、`status`（默认 `draft`）。
- 默认市场处理（对齐 `frontend_agent_execution.md` Phase 3「Default market handling should be clear in the UI」）：UI 明确标注哪个 market 是默认；切换默认市场时自动确保其在已选 markets 内。
- 提交成功后：**一次性弹窗展示明文 `gameSecret`** 并提示「仅此一次，请妥善保存」，关闭后列表刷新。

### 8.3 游戏详情页（多 Tab 入口）`/games/:gameId`

详情页头部展示：`gameId`（只读，可复制）、`name`、`alias`、`status` 标签、`defaultMarketCode`、`gameSecret`（**脱敏展示** `••••••` / `masked`，提供「复制」按钮但复制的是脱敏占位或触发受控展示；本期仅展示脱敏值）。

Tab 入口（本模块负责前两个，其余为 `channel`～`sync` 等下游模块的占位入口，对齐 `frontend_agent_execution.md` Phase 4）：

```text
[基础信息] [市场] [法务链接] | [渠道] [包] [商品] [账号认证] [渠道登录] [IAP] [收银台] [支付路由] [配置快照] [同步记录]
```

- 基础信息 Tab：展示/编辑（抽屉或行内）`name`/`alias`/`iconUrl`/`status`/`defaultMarketCode` → 调 PATCH。

### 8.4 市场 Tab

- 表格：`marketCode`、`isDefault`（默认市场标记）、`enabled`、`defaultLocale`、操作。
- 「编辑市场集合」抽屉：多选 market + 设默认 + 每 market 的 `defaultLocale` + `enabled` → 提交走 PUT `/markets`（全量覆盖）。
- 交互约束：恰好一个默认市场（单选 radio）；移除被下游占用的 market 时前端提示后端会拒绝（展示 409 错误信息）。

### 8.5 法务链接 Tab

- 表格：`scopeType`、`scopeValue`、`termsUrl`、`privacyUrl`、`deleteAccountUrl`、操作。
- 「编辑法务链接」抽屉：可增删多行；`scopeType` 选择联动 `scopeValue`（`default`→锁定 `*`；`market`→Market 下拉；`locale`→语言标签输入/下拉）→ 提交走 PUT `/legal-links`（全量覆盖）。
- 三种 scope 取值约束在前端先校验，后端二次校验。

### 8.6 展示 game_id / 脱敏 game_secret / 默认市场

- `gameId`：列表与详情均展示，详情可复制。
- `gameSecret`：**任何页面只展示脱敏值**（`masked`），仅创建成功弹窗一次性显示明文。
- 默认市场：列表与市场 Tab 用高亮标签标识 `defaultMarketCode`。

### 8.7 空 / 错 / 权限态

| 态 | 处理 |
| --- | --- |
| 空列表 | 列表区展示空状态插画 + 「新建游戏」引导（有 `game.write` 才显示按钮） |
| 空 markets/legalLinks | Tab 内空状态 + 「编辑」引导 |
| 加载中 | 表格/抽屉骨架屏 |
| 接口错误 | 按 00 §7.2 解包 `error.code/message`，toast 或表单字段级错误（用 `details`） |
| 404 | 详情页展示「游戏不存在或已切换环境」并回列表 |
| 无 `game.read` | 菜单/路由隐藏（permission store 守卫） |
| 无 `game.write` | 新建/编辑/保存按钮置灰或隐藏（权限指令） |

---

## 9. 与公共能力关系

| 公共能力（00/01） | 本模块如何落地 |
| --- | --- |
| env 模型（00 §2，D1） | 三表在每个环境 schema 各一份（不带 `env` 列）；唯一键不前置 `env`；写落当前运行环境 schema；读由 `search_path` 定位当前 env，不跨 env |
| 全局枚举（00 §3） | 用 `Environment` / `Market` / `GameStatus` / `LegalScopeType`，默认值对齐 §4 |
| 统一包络（00 §7.2） | 所有响应 `{data}` / 列表 `{data:{items,page,pageSize,total}}` / 错误 `{error}` |
| 分页（00 §7.3） | GET `/games` 用 page/pageSize/sort |
| 鉴权与权限码（00 §7.5，D5） | `game.read` / `game.write` 挂在读/写接口 |
| 密文与脱敏（00 §6） | `gameSecret` 响应脱敏，仅创建一次性返明文；审计 detail 也脱敏 |
| 审计（00 §8） | 四类写操作写 `audit_logs`，action 与权限码同源（§5.9） |
| 命名与默认值兜底（00 §10） | URL 默认 `''`、`enabled=TRUE`、`is_default=FALSE`、`default_locale=en-US`、时间戳 `NOW()` |
| 同步（00 §8 数据流 / `sync`） | 提供 section `game/markets/legal` 的可 diff 数据；跨 schema（`sandbox`→`production`）复制由 `sync` 负责，本模块不执行跨 schema 写 |
| 错误码（00 §7.4） | 复用 `UNAUTHENTICATED/FORBIDDEN/NOT_FOUND/VALIDATION_FAILED/CONFLICT`，本模块不新增错误码 |

---

## 10. 测试要点

### 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨 env（schema 隔离）：写落当前环境 schema、不允许跨 schema 写 / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/game.yaml`；前端 e2e：`tests/frontend/e2e/games.spec.ts`。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET `/api/admin/games` | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | ✓ | ✓ | — | market 过滤/env 隔离（列表不带 gameSecret） |
| POST `/api/admin/games` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | game_id/secret 生成不可变、默认市场补全（GLOBAL）、创建一次性返明文 |
| GET `/api/admin/games/{gameId}` | ✓ | ✓ | ✓ | — | — | ✓ | — | ✓ | — | — | game_secret 脱敏、game_id 跨 env 对齐 |
| PATCH `/api/admin/games/{gameId}` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | 默认市场必须已启用、alias 在所属环境 schema 内唯一、gameId/secret 不可改 |
| PUT `/api/admin/games/{gameId}/markets` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | market 删除保护（409 存在下游渠道实例）、默认市场不可移除 |
| PUT `/api/admin/games/{gameId}/legal-links` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | legal scope_type 唯一（default 仅 `*` 且至多一条） |

前端：`games.spec.ts`（列表筛选/分页、新建抽屉一次性 secret 弹窗、市场 Tab 单默认 radio + 移除受阻 409、法务 Tab scopeType↔scopeValue 联动）/ vitest 组件（列表无权限隐藏新建、详情 gameSecret 脱敏展示、alias 即时校验）。

### 补充关键用例

#### 10.1 单元测试（domain/game，纯逻辑）

- `GenerateGameID`：per-env 从 100000 起自增、不同 env 独立、冲突重试。
- `GenerateGameSecret`：长度 ≤ 128、熵充足、每次不同。
- `ValidateMarkets`：恰好一个默认 / 多默认报错 / 零默认报错 / 重复 marketCode 报错 / 非法枚举报错。
- `ValidateLegalScope`：`default` 仅 `*` 且至多一条 / `market` 必须 ∈ 枚举 / `locale` 语言标签格式 / 重复 (scopeType,scopeValue) 报错。
- `ApplyDefaultMarket`：未传 markets 补 GLOBAL / 传了但缺默认市场时补入。

#### 10.2 服务与仓储测试（app + infra，httptest/pgx）

- 创建：忽略前端传入的 gameId/gameSecret/任何 env 指定；落当前运行环境对应 schema；建默认市场；写审计。
- alias 唯一：同环境 schema 内冲突 409；不同环境 schema 可重复。
- 详情：secret 脱敏（`masked`、`secretMasked=true`）；当前环境 schema 内查不到 → 404。
- PATCH：不可改 gameId/gameSecret；改 defaultMarketCode 到未启用 market 报错；alias 改名查重。
- PUT markets：全量覆盖 diff；恰好一默认；删除被渠道占用的 market → 409；回写 default_market_code。
- PUT legal-links：scope 取值校验；重复 409；全量覆盖 diff。
- env 隔离：子表与父 `games` 行天然同 schema（同 env），普通外键即可，无需 env 一致性校验；不同环境 schema 数据互不串读。

#### 10.3 接口契约测试（transport）

- 响应包络：成功 `{data}`、列表带 `page/pageSize/total`、错误 `{error:{code,message,details}}`。
- 权限：缺 token → 401 `UNAUTHENTICATED`；无 `game.write` 写接口 → 403 `FORBIDDEN`。
- 列表过滤/排序/分页参数边界（pageSize>100 截断、page<1 纠正）。
- `gameSecret` 不出现在列表项；详情/PATCH 永远脱敏；仅 POST 201 返明文一次。

#### 10.4 前端测试（vitest + testing-library）

- 列表：分页/筛选/空态/无权限隐藏新建按钮。
- 新建抽屉：alias 即时校验、默认市场联动、提交后一次性 secret 弹窗。
- 市场 Tab：单默认 radio、移除受阻 409 提示。
- 法务 Tab：scopeType↔scopeValue 联动校验。
- 详情：gameSecret 始终脱敏展示。

---

## 11. 未决问题与假设

### 11.1 假设（本文采用的默认决定，未与上游再确认）

1. **`game_id` 生成策略**：假设为「每个环境 schema 内从 100000 起自增数字串」（依据 `go_domain_api_draft.md` 示例 `100001`）。若需全局唯一/雪花/带前缀，需上游确认。
2. **`game_secret` 存储**：假设按 DDL 原样以明文 `VARCHAR(128)` 存（它是对外下发密钥，非平台内部加密密钥），仅响应脱敏。若安全合规要求落库加密，需新增迁移改密文列。
3. **创建时一次性返回明文 secret**：假设 `POST /games` 201 返回一次明文供运营记录，之后永久脱敏。若禁止任何明文出网，则需改为「仅在专门的『查看/重置密钥』受控接口返回」。
4. **`status` / `market_code` 的 CHECK 约束**：原 DDL `games.status`、`game_markets.market_code` 无 CHECK，本文建议在 D1 迁移中补 CHECK；若上游希望保持宽松（仅应用层校验），可不加。
5. **`UNIQUE(alias)`**：原 DDL 无 alias 唯一键，本文按业务规则「alias 在所属环境 schema 内唯一」补为 `UNIQUE(alias)`（每环境 schema 各一份）。若 alias 仅要求展示用途、允许重复，需上游确认去掉。
6. **market 移除的删除保护**：假设「market 下存在渠道实例（`channel`）即拒绝移除」。跨模块只读查询 `CountChannelsByMarket` 依赖 `channel` 落地后才能生效；`channel` 未实现前，删除保护可降级为仅「不可移除默认市场」。

### 11.2 未决问题（需上游 / 跨模块拍板）

1. **密钥重置**：是否需要 `POST /games/{gameId}/secret/reset`（`game.write` + 审计 + 一次性返明文）？本期未纳入 6 接口范围。
2. **游戏删除/归档**：是否支持删除游戏？当前仅 `status=disabled` 软停用，无物理删除接口；若需删除需定义级联策略（下游大量外键）。
3. **markets 的 `enabled=false` 语义**：禁用 market 时，其下已有渠道实例如何处理（标红不兼容 / 隐藏 / 报错）？与 `channel` 的「不兼容/隐藏」规则需对齐（参见 market-channel-sync-design 的不兼容处理）。
4. **`locale` scope 的语言标签白名单**：是否限定为各 market 的 `default_locale` 集合，还是任意 BCP-47 标签？目前仅做格式校验。
5. **跨 env 的 game_id 对应关系**：sandbox 与 production 同一逻辑游戏是否必须共享同一 `game_id`（便于同步按 gameId 对齐）？本文假设「同步以 gameId 作为跨 env 对齐键」，需 `sync` 确认生成策略与之兼容（即同步创建 production 行时沿用源 env 的 game_id，而非重新自增）。


