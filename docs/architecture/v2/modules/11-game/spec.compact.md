---
id: game
code: "11"
title: 游戏主数据（Game Core）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [common]
code_paths:
  - services/admin-api/internal/domain/game
  - services/admin-api/internal/transport/http/games
  - apps/admin-web/src/views/games
---

# 11 · 游戏主数据 — Compact Spec

> 代码生成用精简规格。完整背景/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（env 模型 D1 §2、统一包络/错误码/分页 §7、密文脱敏 §6、审计 §8、枚举 §3、默认值兜底 §10）与 `../../01-structure.md`（分层 §4、前端 §5）。

## 边界 / 红线
- 「游戏主数据」是发行后台**根聚合**、一切下游配置的挂载点（领域 `internal/domain/game` 的 `Game` 聚合根）；最先落地，下游沿用其 schema-per-env、唯一键、ID 生成规则。
- **只管** 游戏基础信息 + 发行市场集合(markets) + 法务链接(legalLinks)。
- **不管** 渠道实例/包/登录/IAP（`channel`~`feature-plugin`，仅提供 market 集合）、收银台模板/价格/支付路由（`cashier-template`/`game-cashier`/`payment`，仅提供 `games.id` 根）、快照/同步（`snapshot`/`sync`）。
- 不允许前端指定写入 env/schema：一律落当前运行环境 schema（`search_path` 决定，不跨 schema 写）。
- 不回明文 `game_secret`（响应恒脱敏）；`game_id`/`game_secret` 创建后不可变（本期无重置接口）。

## 数据模型
3 张游戏维度业务表（`games`/`game_markets`/`game_legal_links`），**每环境 schema 各一份、同名同结构、不带 `env` 列**；env 由 `search_path = <env>, platform` 决定。业务表→业务表用同 schema 普通 FK，父子必然同 env，无需复合 env 外键 / env 一致性校验 / `WHERE env` 谓词；仓储 SQL 不写 schema 前缀。下游跨 schema 复制由 `sync` 负责。

公共列约定（三表通用）：`id BIGSERIAL PK`（内部主键，不对外暴露）、`created_at/updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`。唯一键**不前置 `env`**，唯一性按 schema 天然隔离。

### games（游戏基础信息）
```sql
CREATE TABLE games (
  id                  BIGSERIAL PRIMARY KEY,
  game_id             VARCHAR(64)  NOT NULL,                   -- 对外标识，创建生成，不可变
  game_secret         VARCHAR(128) NOT NULL,                   -- 对外密钥，响应脱敏；本期明文存
  name                VARCHAR(128) NOT NULL,
  alias               VARCHAR(64)  NOT NULL,
  icon_url            VARCHAR(512) NOT NULL DEFAULT '',
  default_market_code VARCHAR(32)  NOT NULL DEFAULT 'GLOBAL',  -- 必须 ∈ 已启用 markets 且对应行 isDefault=true
  status              VARCHAR(32)  NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft','active','disabled')),  -- 建议补 CHECK
  created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (game_id),                                            -- schema 内唯一，不前置 env
  UNIQUE (alias)                                               -- alias 所属 schema 内唯一（新增迁移补齐）
);
```

### game_markets（游戏发行市场）
```sql
CREATE TABLE game_markets (
  id             BIGSERIAL PRIMARY KEY,
  game_id_ref    BIGINT      NOT NULL REFERENCES games(id),    -- 同 schema 普通 FK
  market_code    VARCHAR(32) NOT NULL
                   CHECK (market_code IN ('GLOBAL','JP','KR','SEA','HMT','CN')),  -- 建议补 CHECK
  is_default     BOOLEAN     NOT NULL DEFAULT FALSE,           -- 每游戏恰好一条 true
  enabled        BOOLEAN     NOT NULL DEFAULT TRUE,            -- 禁用后下游不可在此 market 建渠道实例
  default_locale VARCHAR(16) NOT NULL DEFAULT 'en-US',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, market_code)
);
```

### game_legal_links（游戏法务链接）
```sql
CREATE TABLE game_legal_links (
  id                 BIGSERIAL PRIMARY KEY,
  game_id_ref        BIGINT       NOT NULL REFERENCES games(id),  -- 同 schema 普通 FK
  scope_type         VARCHAR(16)  NOT NULL
                       CHECK (scope_type IN ('default','market','locale')),
  scope_value        VARCHAR(32)  NOT NULL DEFAULT '*',           -- default→'*'；market→Market 值；locale→语言标签
  terms_url          VARCHAR(512) NOT NULL DEFAULT '',
  privacy_url        VARCHAR(512) NOT NULL DEFAULT '',
  delete_account_url VARCHAR(512) NOT NULL DEFAULT '',
  created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, scope_type, scope_value)
);
```

## 枚举与默认
- `Environment`: develop/sandbox/production（不落列，由 schema 表达；`APP_ENV` 决定，缺省 develop）。
- `Market`: GLOBAL/JP/KR/SEA/HMT/CN，默认 GLOBAL（发行大区非国家）。
- `GameStatus`: draft/active/disabled，默认 draft。
- `LegalScopeType`: default/market/locale，默认 default。
- 字段默认：icon_url/terms_url/privacy_url/delete_account_url=`''`；default_market_code=`'GLOBAL'`；status=`'draft'`；is_default=`FALSE`；enabled=`TRUE`；default_locale=`'en-US'`；scope_value=`'*'`；时间戳=`NOW()`。
- Market 语义（继承 00 §3.2）：GLOBAL 兜底海外、**不匹配 CN**；CN 仅中国大陆；JP/KR/SEA/HMT 与 GLOBAL 并存时整体覆盖 GLOBAL（实例级，snapshot 合并）。本模块只管「启用哪些 market」，不做渠道可见性过滤。

## 聚合一致性规则
聚合边界 = 一个游戏在一个 env（一个 schema）下的全部主数据（`Game` 根 + `[]GameMarket` + `[]GameLegalLink`）。
- markets 中**有且仅有一个** `isDefault=true`，且其 `marketCode == games.default_market_code`。
- markets 的 `marketCode` 不重复；legalLinks 的 `(scopeType, scopeValue)` 不重复（DB 唯一键兜底）。
- 父子天然同 schema（同 env），无需 env 一致性校验。
- `game_id`/`game_secret` 创建时生成且不可变。

## 业务规则
1. **game_id 生成**（§5.1）：服务端在 POST 时生成，前端传入一律忽略；当前 schema 内从 `100000` 起自增数字串（示例 `100001`），保证 `UNIQUE(game_id)`，冲突重试；各环境 schema 独立计数。创建后不可变。
2. **game_secret 生成与脱敏**（§5.2）：长度 ≤128 高熵随机串（base62/hex，≥32 字节熵）；本期明文存 `VARCHAR(128)`；响应恒脱敏返 `"masked"`，仅 POST 201 一次性返明文供运营记录；创建后不可变。
3. **默认创建 GLOBAL 市场**（§5.3）：未传 markets → 自动建 `GLOBAL`(isDefault=true/enabled=true/defaultLocale=en-US)；传了 markets 但缺 defaultMarketCode 指定市场 → 自动补入；既未传 markets 也未传 defaultMarketCode → 取 GLOBAL；`marketCode==defaultMarketCode` 那条 isDefault=true 其余 false。
4. **alias 唯一**（§5.4）：所属 schema 内全局唯一；POST/PATCH 写入先查重，冲突 `409 CONFLICT`（message `"alias already exists"`）；不同 schema 可重复。
5. **legal scope 唯一与取值**（§5.5）：`(scopeType, scopeValue)` 同游戏内唯一；default⇒scopeValue 必为 `'*'` 且每游戏至多一条；market⇒∈ Market 枚举；locale⇒合法语言标签（`xx-XX` 或 `xx`）。违反取值 `400 VALIDATION_FAILED`，重复 `409 CONFLICT`。
6. **market 唯一与默认**（§5.6）：同游戏 marketCode 不重复；恰好一条 isDefault=true 且 ==games.default_market_code；PUT 全量覆盖时校验非空/合法不重复/恰好一默认，并同步回写 games.default_market_code。
7. **多 market 管理**（§5.8）：经 PUT 全量提交期望集合增删；移除时若该 market 下已有渠道实例(`channel`)或为默认市场 → 拒绝 `409 CONFLICT`；切默认市场改 isDefault 指向并同步 default_market_code。
8. **env 行为**（§5.7）：写落当前 schema（不可跨 schema 写）；读由 search_path 定位当前 env，不跨 env；sandbox→production 复制由 `sync` 负责（section `game/markets/legal`）。

### GameStatus 状态机
draft（新建默认）⇄ active（显式激活）⇄ disabled（显式停用），draft→disabled。本期经 PATCH 的 status 直接改写，不强制单向流转校验。

### 审计（继承 00 §8，detail_json 记关键 before/after，secret 脱敏）
| 操作 | action | resource_type | resource_id |
| --- | --- | --- | --- |
| 创建游戏 | `game.create` | game | gameId |
| 编辑基础信息 | `game.update` | game | gameId |
| 覆盖 markets | `game.markets.update` | game | gameId |
| 覆盖 legalLinks | `game.legal.update` | game | gameId |

## 后端 API（前缀 /api/admin，包络/错误码 00 §7；路径 `{gameId}` 指对外 game_id 非内部 id；读 game.read / 写 game.write）

通用错误响应（00 §7.2）：
```json
{ "error": { "code": "VALIDATION_FAILED", "message": "alias already exists", "details": [] } }
```

### GET `/games` — 列表（game.read）
Query（继承 00 §7.3 分页）：page(默认1,≥1)、pageSize(默认20,≤100)、sort(默认`-updatedAt`)、keyword(≤64，模糊匹配 name/alias/gameId)、status(∈GameStatus，过滤)、marketCode(∈Market，仅返回启用该 market 的游戏)。隐式作用域=当前 env schema。列表项为轻量摘要（不展开 legalLinks，markets 仅计数+默认），**不返回 gameSecret**。
→ items[]: { gameId, name, alias, iconUrl, status, defaultMarketCode, marketCodes[], marketCount, createdAt, updatedAt }；外层 { items, page, pageSize, total }。

### POST `/games` — 创建（game.write，审计 game.create）
请求体 `CreateGameRequest`：
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| name | string | 是 | — | 1–128 |
| alias | string | 是 | — | 1–64，`^[a-zA-Z0-9_-]+$`，同 env 唯一 |
| iconUrl | string | 否 | `''` | ≤512，format=url（非空时） |
| defaultMarketCode | enum | 否 | GLOBAL | ∈ Market |
| status | enum | 否 | draft | ∈ GameStatus |
| markets | string[] | 否 | `["GLOBAL"]` | 每项 ∈ Market，去重 |

行为：校验 alias 唯一/name 非空/markets 合法 → 生成 gameId/gameSecret → 落当前 schema → 建 markets（保证含 defaultMarketCode，该条 isDefault=true/enabled=true/defaultLocale=en-US）→ 写审计；忽略前端传入的 gameId/gameSecret/env。
→ 201 **仅创建时**一次性返明文 gameSecret（`secretMasked:false`）+ 完整聚合（含 markets、legalLinks:[]）；之后任何接口脱敏。
→ alias 冲突 409 `CONFLICT`；校验失败 400 `VALIDATION_FAILED`（details 给 field/reason）。

### GET `/games/{gameId}` — 详情（game.read）
当前 schema 内按 gameId 查；不存在 `404 NOT_FOUND`。返回完整聚合：基础信息 + env + markets[] + legalLinks[]，`gameSecret:"masked"`/`secretMasked:true`。
- markets 项: { marketCode, isDefault, enabled, defaultLocale }。
- legalLinks 项: { scopeType, scopeValue, termsUrl, privacyUrl, deleteAccountUrl }。

### PATCH `/games/{gameId}` — 编辑基础信息（game.write，审计 game.update）
部分更新，字段均可选（缺省=不改）；**不可改** gameId/gameSecret，不接受切换 env；markets/legalLinks 走专用接口。
| 字段 | 类型 | 校验 |
| --- | --- | --- |
| name | string | 1–128 |
| alias | string | 1–64，`^[a-zA-Z0-9_-]+$`，同 env 唯一（变更查重） |
| iconUrl | string | ≤512，format=url（非空时） |
| status | enum | ∈ GameStatus |
| defaultMarketCode | enum | ∈ Market 且必须 ∈ 已启用 markets（变更时同步 game_markets.is_default） |
→ 200 返回同详情结构；改默认市场到未启用 market → `VALIDATION_FAILED`（"defaultMarketCode must be an enabled market"）。

### PUT `/games/{gameId}/markets` — 全量覆盖市场（game.write，审计 game.markets.update）
请求体 `ReplaceMarketsRequest`：
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| markets | object[] | 是 | — | 非空，marketCode 不重复 |
| markets[].marketCode | enum | 是 | — | ∈ Market |
| markets[].isDefault | bool | 否 | false | 整列表恰好一条 true |
| markets[].enabled | bool | 否 | true | — |
| markets[].defaultLocale | string | 否 | en-US | 语言标签格式 |

行为：校验非空/合法不重复/恰好一默认 → 与现有 diff（增/改/删）→ 删除保护（被删 market 下有渠道实例或为默认 → `409 CONFLICT`）→ 回写 default_market_code → 写审计。
→ 删除受阻 `CONFLICT`（"cannot remove market with existing channels"）；多默认 `VALIDATION_FAILED`（"exactly one default market is required"）。

### PUT `/games/{gameId}/legal-links` — 全量覆盖法务链接（game.write，审计 game.legal.update）
请求体 `ReplaceLegalLinksRequest`：
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| legalLinks | object[] | 是 | — | (scopeType, scopeValue) 不重复 |
| legalLinks[].scopeType | enum | 是 | — | ∈ default/market/locale |
| legalLinks[].scopeValue | string | 否 | `*` | 按 scopeType 取值约束（规则 5） |
| legalLinks[].termsUrl | string | 否 | `''` | ≤512，format=url（非空时） |
| legalLinks[].privacyUrl | string | 否 | `''` | 同上 |
| legalLinks[].deleteAccountUrl | string | 否 | `''` | 同上 |

行为：逐项校验 scope 取值 → 校验 (scopeType, scopeValue) 不重复 → diff 写库 → 写审计。
→ 200 返回写入后完整 legalLinks 数组；scope 取值非法 `400 VALIDATION_FAILED`。

## 应用服务 / 仓储（01 §4 分层：domain → app command/query → infra/persistence/postgres(pgx) → transport/http/games(chi)）

### 领域层 `internal/domain/game`（纯逻辑，无 IO）
- 聚合 `Game` + 值对象 `GameMarket`/`GameLegalLink`。
- `GenerateGameID(env, lastSeq)` / `GenerateGameSecret()`（随机源注入）。
- `ValidateMarkets(markets, defaultMarketCode)`：恰好一默认 + 枚举合法 + 不重复。
- `ValidateLegalScope(scopeType, scopeValue)`：scope 取值约束。
- `ApplyDefaultMarket(req)`：缺省补 GLOBAL 默认市场。

### 应用服务 `GameService`（app 层，依赖 GameRepository/随机源/AuditLogger/Clock；ctx 携带按当前 env 钉死 search_path 的连接，env 不作显式参数）
| 方法 | 类型 | 接口 |
| --- | --- | --- |
| `ListGames(ctx, ListGamesQuery)` | query | GET /games |
| `GetGame(ctx, gameId)` | query | GET /games/{gameId} |
| `CreateGame(ctx, CreateGameCmd)` | command | POST /games |
| `UpdateGame(ctx, UpdateGameCmd)` | command | PATCH /games/{gameId} |
| `ReplaceMarkets(ctx, ReplaceMarketsCmd)` | command | PUT /markets |
| `ReplaceLegalLinks(ctx, ReplaceLegalLinksCmd)` | command | PUT /legal-links |

DTO ↔ 领域映射在 app 层；transport 只做 JSON↔DTO + 包络。

### 仓储 `GameRepository`（窄接口，全带 ctx，SQL 不写 schema 前缀/不带 env 谓词）
```text
NextGameIDSeq(ctx) (int64, error)                      // 当前 schema 内序列
ExistsAlias(ctx, alias, excludeGameID) (bool, error)
InsertGame(ctx, game) (Game, error)                    // 同事务插 games + 默认 market
GetGameByGameID(ctx, gameId) (Game, error)             // 聚合装配
ListGames(ctx, query) (items, total, error)
UpdateGame(ctx, gameId, patch) (Game, error)
ReplaceMarkets(ctx, gameId, markets) error             // 事务内 diff
ReplaceLegalLinks(ctx, gameId, links) error            // 事务内 diff
CountChannelsByMarket(ctx, gameId, marketCode) (int, error) // 删除保护（跨模块只读，同 schema）
```
事务边界：CreateGame（games + 默认 market）、ReplaceMarkets、ReplaceLegalLinks 单事务内完成并写审计。

### transport `transport/http/games`
chi 子路由挂 `/api/admin/games`；中间件链：recover → 鉴权(Bearer) → 权限(game.read/game.write) → env 上下文注入（search_path 由每环境连接池建连时钉死，非逐请求 SET）→ handler → 审计。

## 前端要点（Vue3 + Pinia + Element Plus，抽屉优先；路由 /games → /games/:gameId 多 Tab）
- **列表页 `/games`**：常驻 EnvironmentBadge；工具栏 keyword/status/marketCode 筛选 + 「新建游戏」（game.write 守卫）；表格列 gameId/name/alias/status 标签/defaultMarketCode/market 标签/updatedAt/操作；**不展示 gameSecret**；分页走 00 §7.3。
- **新建抽屉**：name/alias(即时查重)/iconUrl(上传)/defaultMarketCode(默认 GLOBAL)/markets(多选默认含 GLOBAL)/status(默认 draft)；UI 明确标默认市场、切换时确保其在已选内；提交成功后**一次性弹窗展示明文 gameSecret**（"仅此一次"）后刷新。
- **详情页 `/games/:gameId`**：头部 gameId(只读可复制)/name/alias/status/defaultMarketCode/gameSecret(脱敏 `masked`)；Tab：[基础信息][市场][法务链接] + 下游占位 [渠道][包][商品][账号认证][渠道登录][IAP][收银台][支付路由][配置快照][同步记录]。
- **市场 Tab**：表格 marketCode/isDefault/enabled/defaultLocale；编辑抽屉多选 market + 单默认 radio + 每 market defaultLocale/enabled → PUT 全量覆盖；移除被占用 market 展示 409。
- **法务链接 Tab**：表格 scopeType/scopeValue/三 URL；编辑抽屉增删多行，scopeType 联动 scopeValue（default 锁 `*`、market 下拉、locale 输入）→ PUT 全量覆盖；前端先校验后端二次校验。
- **空/错/权限态**：空列表插画 + 引导；接口错误按 00 §7.2 解包 error.code/message（details 做字段级）；404 提示"游戏不存在或已切换环境"；无 game.read 隐藏菜单/路由；无 game.write 写按钮置灰/隐藏。

## 与公共能力 / 下游
- env(00 §2,D1)：三表每环境 schema 各一份不带 env 列；统一包络/分页(00 §7)；密文脱敏(00 §6) gameSecret 仅创建一次性返明文；审计(00 §8) 四类写操作；默认值兜底(00 §10)；不新增错误码。
- 下游消费：`channel` 依赖 markets 集合（建渠道实例校验目标 market 已启用）；几乎所有 per-game 表以 games.id 为 FK 根；`gameId` 是 snapshot/sync 聚合主键；`sync` section = game/markets/legal。

## 关键假设
- game_id：每环境 schema 内从 100000 起自增数字串；同步以 gameId 作跨 env 对齐键（production 行沿用源 env game_id，不重新自增）。
- game_secret：按 DDL 明文 VARCHAR(128) 存（对外下发密钥非内部加密密钥），仅响应脱敏；创建时一次性返明文；如需落库加密另开迁移。
- status/market_code 的 CHECK、UNIQUE(alias) 由新增 D1 迁移补齐（原 DDL 无），保持不改历史迁移语义。
- market 删除保护依赖 `channel` 落地后的 CountChannelsByMarket；`channel` 未实现前降级为仅「不可移除默认市场」。
- 本期不提供密钥重置 / 游戏物理删除（仅 status=disabled 软停用）；locale scope 仅做格式校验，不限白名单。
