---
id: channel
code: "12"
title: 渠道与渠道实例（GameMarketChannel）
status: target
code_paths:
  - services/admin-api/internal/domain/channel
  - services/admin-api/internal/transport/http/channels
  - apps/admin-web/src/views/channels
depends_on: [game, common]
impacts: [account-auth, channel-login, feature-plugin, product, payment, snapshot, sync, testing]
children: []
---

# 12 · 渠道与渠道实例（GameMarketChannel）

> 本模块是整套架构的**结构性核心**：它把"渠道"从游戏级共享改为**按 market 维度拆分的实例（GameMarketChannel）**，并定义可见性、复制创建、隐藏/不兼容、运行态标识、渠道包等规则。
> 阅读前请先读 `../../00-common.md`（枚举/状态机/红线）与 `../../01-structure.md`（env/分层）。本模块强依赖 `00 §3.2 Market 语义` 与 `§3.4 ConfigStatus 状态机`。

---

## 1. 模块概述与边界

### 1.1 职责
- 维护平台级**渠道主数据** `channels` 与**渠道策略** `channel_policies`。
- 维护游戏在某个 market 下的**渠道实例** `game_channels`（即 `GameMarketChannel`）。
- 维护渠道实例下的**渠道包** `channel_packages`。
- 实现 market × 渠道的**可见性/兼容性校验**、**复制创建**、**隐藏/恢复**、**运行态标识推导**。

### 1.2 上下游
- 依赖：`11-游戏主数据`（游戏已启用的 market 集合）、`00 §3.2`（Market 语义与可见性）。
- 被依赖：`13-自有账号认证` 与 `14-渠道登录`（登录配置挂在渠道实例上）、`15-商品与IAP`（包级映射）、`18-支付路由`（按 channel/package 选择器）、`19-配置快照`（运行时合并的最小生效单元）、`20-同步`（channels/packages section）。

### 1.3 边界红线（来自 `00 §9`）
- 渠道**强制登录**（`channel-login`）与**自有账号认证**（`account-auth`）底层分开，本模块只提供"渠道实例"这一挂载点。
- 被隐藏 / 不兼容 / 无效（`config_status != valid`）的渠道实例：**不进默认列表、不进快照、不参与同步、不进客户端最终配置**。

---

## 2. 领域模型与聚合

### 2.1 GameMarketChannel 聚合（核心）
代表"某游戏 + 某 market + 某渠道"的唯一实例。落地表即 `game_channels`（D2）。

聚合拥有：
- 实例身份：`(env, gameId, market, channelId)`。
- 隐藏状态：`hidden` + `hiddenBy` + `hiddenAt`。
- 兼容性（派生，不落库）：由 `market` 与 `channel.region` 经可见性规则判定。
- 渠道实例配置入口：渠道包（本模块）、渠道登录配置（`channel-login`）、IAP 配置（`product`）。
- 配置状态：`config_status`（`empty/invalid/valid`，见 `00 §3.4`）。

不变量：
- 同一 `(env, gameId, market, channelId)` 唯一。
- 创建/迁移到某 market 时必须满足可见性：`market=CN ⇒ channel.region=domestic`；`market!=CN ⇒ channel.region=overseas`。
- `hidden=true` ⇒ `IncludedInSnapshot/Sync/RuntimeConfig` 全为 `false`。

### 2.2 值对象与纯规则
- `Market`（`00 §3`）、`ChannelRegion`（`domestic/overseas`）。
- `ValidateMarketChannelCompatibility(market, region) error`：可见性纯函数（无 IO），服务端强制调用。
- `ResolveRuntimeFlags(instance) RuntimeFlags`：根据 `hidden`、兼容性、`config_status` 推导三个只读运行态标识。

### 2.3 基础数据
- `channels`：渠道主数据 + `region`（D3）。
- `channel_policies`：渠道登录/支付策略与锁定位（决定 `account-auth`/`channel-login`/`product` 的行为）。

---

## 3. 数据模型

> 约定见 `00 §2.2`：`game_channels`、`channel_packages` **带 env**；`channels`、`channel_policies` 为平台级**不带 env**。

### 3.1 `channels`（平台级，新增 `region` —— D3）

| 列 | 类型 | 可空 | 默认 | 约束/说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `channel_id` | VARCHAR(64) | 否 | — | 业务键，UNIQUE |
| `channel_name` | VARCHAR(64) | 否 | — | |
| `channel_type` | VARCHAR(32) | 否 | — | CHECK in `store/oem/web/direct/mini_game` |
| **`region`** | VARCHAR(16) | 否 | — | **新增**，CHECK in `domestic/overseas`（D3） |
| `enabled` | BOOLEAN | 否 | `TRUE` | |
| `sort` | INT | 否 | `0` | |
| `created_at` | TIMESTAMPTZ | 否 | `NOW()` | |
| `updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(channel_id)`。索引建议：`(region)`、`(enabled, sort)`。

**迁移（追加，不改历史）**：
```sql
ALTER TABLE channels ADD COLUMN region VARCHAR(16) NOT NULL DEFAULT 'overseas'
  CHECK (region IN ('domestic','overseas'));
UPDATE channels SET region = 'domestic'
  WHERE channel_id IN ('huawei_cn','xiaomi_cn','oppo_cn','vivo_cn','wechat_mini_game','douyin_mini_game');
UPDATE channels SET region = 'overseas'
  WHERE channel_id IN ('google','apple');
-- 回填完成后可去掉 DEFAULT（可选）
```

seed region 固定值：

| channel_id | channel_type | region |
| --- | --- | --- |
| google | store | overseas |
| apple | store | overseas |
| huawei_cn | oem | domestic |
| xiaomi_cn | oem | domestic |
| oppo_cn | oem | domestic |
| vivo_cn | oem | domestic |
| wechat_mini_game | mini_game | domestic |
| douyin_mini_game | mini_game | domestic |

### 3.2 `channel_policies`（平台级）

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `channel_id_ref` | BIGINT | 否 | — | FK→channels(id)，UNIQUE |
| `login_mode` | VARCHAR(16) | 否 | — | CHECK in `channel_only/account_system` |
| `payment_mode` | VARCHAR(16) | 否 | — | CHECK in `channel_only/hybrid/cashier_only` |
| `login_locked` | BOOLEAN | 否 | `FALSE` | 锁定后游戏侧不可改登录策略 |
| `payment_locked` | BOOLEAN | 否 | `FALSE` | 锁定后游戏侧不可改支付策略 |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

seed 策略（来自现有迁移）：`huawei_cn/xiaomi_cn/oppo_cn/vivo_cn` ⇒ `login_mode=channel_only, payment_mode=channel_only, login_locked=TRUE, payment_locked=TRUE`；其余 ⇒ `login_mode=account_system, payment_mode=hybrid, locked=FALSE`。

### 3.3 `game_channels`（带 env，新增 `market_code`，即 GameMarketChannel —— D2）

| 列 | 类型 | 可空 | 默认 | 约束/说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| **`env`** | VARCHAR(16) | 否 | — | **新增**，CHECK in `develop/sandbox/production`（D1） |
| `game_id_ref` | BIGINT | 否 | — | FK→games(id) |
| **`market_code`** | VARCHAR(32) | 否 | — | **新增**，CHECK in `GLOBAL/JP/KR/SEA/HMT/CN`（D2） |
| `channel_id_ref` | BIGINT | 否 | — | FK→channels(id) |
| `enabled` | BOOLEAN | 否 | `TRUE` | |
| `hidden` | BOOLEAN | 否 | `FALSE` | **新增**，手动隐藏 |
| `hidden_by` | VARCHAR(128) | 否 | `''` | **新增**，隐藏操作人 |
| `hidden_at` | TIMESTAMPTZ | 是 | `NULL` | **新增** |
| `config_status` | VARCHAR(16) | 否 | `'empty'` | **新增**，CHECK in `empty/invalid/valid` |
| `last_check_at` | TIMESTAMPTZ | 是 | `NULL` | **新增** |
| `last_check_message` | VARCHAR(255) | 否 | `''` | **新增** |
| `copied_from_market` | VARCHAR(32) | 否 | `''` | **新增**，复制来源 market（审计/追溯，仅创建时记录） |
| `remark` | VARCHAR(255) | 否 | `''` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键（D1+D2）：`UNIQUE(env, game_id_ref, market_code, channel_id_ref)`。
索引建议：`(env, game_id_ref)`（全 market 列表默认查询）、`(env, game_id_ref, market_code)`、`(channel_id_ref)`。

> 说明：`兼容性` 不落库，按 `market_code` 与 `channels.region` 实时派生（避免规则变化导致脏数据）。`copied_from_market` 仅作来源记录，**不建立运行时联动**。

**迁移（追加）**：
```sql
ALTER TABLE game_channels
  ADD COLUMN env VARCHAR(16) NOT NULL DEFAULT 'develop' CHECK (env IN ('develop','sandbox','production')),
  ADD COLUMN market_code VARCHAR(32) NOT NULL DEFAULT 'GLOBAL' CHECK (market_code IN ('GLOBAL','JP','KR','SEA','HMT','CN')),
  ADD COLUMN hidden BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN hidden_by VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN hidden_at TIMESTAMPTZ,
  ADD COLUMN config_status VARCHAR(16) NOT NULL DEFAULT 'empty' CHECK (config_status IN ('empty','invalid','valid')),
  ADD COLUMN last_check_at TIMESTAMPTZ,
  ADD COLUMN last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN copied_from_market VARCHAR(32) NOT NULL DEFAULT '';
-- 调整唯一约束：先删旧 UNIQUE(game_id_ref, channel_id_ref)，再建新键
ALTER TABLE game_channels DROP CONSTRAINT IF EXISTS game_channels_game_id_ref_channel_id_ref_key;
ALTER TABLE game_channels ADD CONSTRAINT game_channels_env_game_market_channel_key
  UNIQUE (env, game_id_ref, market_code, channel_id_ref);
```

### 3.4 `channel_packages`（带 env）

| 列 | 类型 | 可空 | 默认 | 约束/说明 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| **`env`** | VARCHAR(16) | 否 | — | **新增**（D1） |
| `game_channel_id_ref` | BIGINT | 否 | — | FK→game_channels(id) |
| `package_code` | VARCHAR(64) | 否 | — | |
| `package_name` | VARCHAR(128) | 否 | — | |
| `market_code` | VARCHAR(32) | 否 | — | 与所属渠道实例 market 一致（应用层校验） |
| `bundle_id` | VARCHAR(128) | 否 | `''` | |
| `inherit_channel_config` | BOOLEAN | 否 | `TRUE` | 是否继承渠道实例配置 |
| `enabled` | BOOLEAN | 否 | `TRUE` | |
| `override_json` | JSONB | 否 | `{}` | 包级覆盖载荷 |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(env, game_channel_id_ref, package_code)`。索引：`(env, game_channel_id_ref)`。

**迁移（追加）**：
```sql
ALTER TABLE channel_packages
  ADD COLUMN env VARCHAR(16) NOT NULL DEFAULT 'develop' CHECK (env IN ('develop','sandbox','production'));
ALTER TABLE channel_packages DROP CONSTRAINT IF EXISTS channel_packages_game_channel_id_ref_package_code_key;
ALTER TABLE channel_packages ADD CONSTRAINT channel_packages_env_gc_code_key
  UNIQUE (env, game_channel_id_ref, package_code);
```

---

## 4. 枚举与默认值清单

| 项 | 取值 / 默认 |
| --- | --- |
| `Market` | `GLOBAL`/`JP`/`KR`/`SEA`/`HMT`/`CN`，默认 `GLOBAL` |
| `ChannelRegion` | `domestic`/`overseas`（seed 固定，无默认） |
| `ChannelType` | `store`/`oem`/`web`/`direct`/`mini_game`（无默认） |
| `LoginMode` | `channel_only`/`account_system`，默认 `account_system` |
| `PaymentMode` | `channel_only`/`hybrid`/`cashier_only`，默认 `hybrid` |
| `ConfigStatus` | `empty`/`invalid`/`valid`，新建默认 `empty`；**复制创建后强制 `invalid`** |
| `enabled` | 默认 `TRUE` |
| `hidden` | 默认 `FALSE` |
| `login_locked`/`payment_locked` | 默认 `FALSE` |
| `inherit_channel_config` | 默认 `TRUE` |
| `override_json` | 默认 `{}` |
| `bundle_id`/`remark`/`hidden_by`/`last_check_message`/`copied_from_market` | 默认 `''` |
| `env` | 取当前运行环境 |

### 运行态标识（派生，只读，三态布尔）
- `IncludedInRuntimeConfig = !hidden && compatible && config_status=='valid'`
- `IncludedInSnapshot = IncludedInRuntimeConfig`
- `IncludedInSync = !hidden && compatible`（同步以 section 数据为准，但隐藏一律排除；不兼容数据不参与同步）
- 任一为 `false` 时，前端须给原因：`hidden` / `incompatible` / `invalid_config`。

---

## 5. 业务规则与状态机

### 5.1 可见性 / 兼容性
```text
ValidateMarketChannelCompatibility(market, region):
  if market == CN and region != domestic -> error MARKET_CHANNEL_INCOMPATIBLE
  if market != CN and region != overseas -> error MARKET_CHANNEL_INCOMPATIBLE
  return ok
```
- 新增渠道时按目标 market 过滤候选渠道（前端），**服务端必须二次校验**（不能只信前端）。
- GLOBAL 仅显示 overseas。
- 派生兼容性：对已存在实例，用同一函数判定；不兼容 ⇒ 列表标红、提示"不兼容当前 market"，**不自动删除、保留配置**。

### 5.2 创建（空白 / 复制）
- 同一 `(env, game, market, channel)` 已存在 ⇒ 拒绝重复新增（`CONFLICT`）。
- 已隐藏渠道默认不出现在"新增可选列表"。
- 复制创建（从其它 market 同渠道）规则（来自 spec）：
  - 仅复制普通字段；`secret` 字段清空；`file` 字段清空；
  - 复制仅发生在创建时，创建后新旧实例不再联动；
  - 新实例 `config_status = invalid`（即使普通字段已带入，因敏感/文件字段未补齐）；
  - `last_check_message` 必须提示"缺少必填敏感字段或文件字段"；
  - 记录 `copied_from_market`。

### 5.3 隐藏 / 恢复
- `hide(operator)`：`hidden=true`、`hidden_by=operator`、`hidden_at=now`；写审计 `channel.hide`。
- `unhide()`：`hidden=false`、清 `hidden_by/hidden_at`；写审计 `channel.unhide`。
- 隐藏后：不进默认列表（除非显式"显示隐藏项"）、不进快照/同步/客户端最终配置；保留记录可恢复。
- "不兼容" vs "已隐藏" 必须在 UI 上明确区分：不兼容=规则下不可正常使用但保留；已隐藏=管理员主动移出生效集。

### 5.4 ConfigStatus 推导
- 遵循 `00 §3.4`。渠道实例自身的 `config_status` 由其模板驱动配置（登录/IAP，`channel-login`/`product`）综合判定；缺必填/敏感/文件字段 ⇒ `invalid`；全通过 ⇒ `valid`；未建任何配置 ⇒ `empty`。
- 复制创建强制 `invalid`（见 5.2）。

### 5.5 渠道包 inherit/override
- `inherit_channel_config=true`：包沿用渠道实例配置；`override_json` 仅存差异。
- `inherit_channel_config=false`：包使用自身完整配置（仍受模板校验）。
- 包 `market_code` 必须与所属渠道实例一致。

---

## 6. 后端 API

> 统一前缀 `/api/admin`，遵循 `00 §7` 包络与错误码。写操作挂权限码 `channel.write`，读 `channel.read`。

### 6.1 渠道主数据
**GET `/api/admin/games/{gameId}/channels`** —— 列出该游戏可用的渠道主数据（含 region/policy），用于新增时筛选。权限 `channel.read`。
响应：
```json
{ "data": { "items": [
  { "channelId": "google", "channelName": "Google", "channelType": "store", "region": "overseas",
    "loginMode": "account_system", "paymentMode": "hybrid", "loginLocked": false, "paymentLocked": false }
] } }
```

### 6.2 渠道实例总览（默认全 market）
**GET `/api/admin/games/{gameId}/market-channels`** —— 默认返回该游戏当前 env 下**所有 market 的所有渠道实例**。权限 `channel.read`。
Query 参数（全部可选）：
| 参数 | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `market` | string | `ALL` | `ALL` 或具体 market |
| `channelId` | string | `''` | 渠道名过滤 |
| `compatible` | bool | 不限 | 兼容状态过滤 |
| `hidden` | bool | `false` | 是否包含隐藏项；默认不含 |
| `configStatus` | enum | 不限 | `empty/invalid/valid` |
| `page`/`pageSize` | int | `1`/`20` | 见 `00 §7.3` |

响应（每行一个实例）：
```json
{ "data": { "items": [
  {
    "id": "100001:GLOBAL:google", "gameId": "100001", "market": "GLOBAL", "channelId": "google",
    "region": "overseas", "compatible": true, "hidden": false, "configStatus": "valid",
    "includedInSnapshot": true, "includedInSync": true, "includedInRuntimeConfig": true,
    "copiedFromMarket": "", "updatedAt": "2026-06-15T10:00:00Z"
  },
  {
    "id": "100001:JP:google", "gameId": "100001", "market": "JP", "channelId": "google",
    "region": "overseas", "compatible": true, "hidden": false, "configStatus": "invalid",
    "includedInSnapshot": false, "includedInSync": true, "includedInRuntimeConfig": false,
    "copiedFromMarket": "GLOBAL", "updatedAt": "2026-06-15T11:00:00Z"
  }
], "page": 1, "pageSize": 20, "total": 2 } }
```

### 6.3 创建渠道实例（空白 / 复制）
**POST `/api/admin/games/{gameId}/markets/{market}/channels`** 权限 `channel.write`。
请求 DTO：
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `channelId` | string | 是 | — | 必须存在且 `region` 与 `market` 兼容 |
| `mode` | enum | 否 | `empty` | `empty` / `copy` |
| `copyFromMarket` | string | 当 `mode=copy` 时必填 | `''` | 同游戏同渠道在该 market 必须已存在 |
| `enabled` | bool | 否 | `true` | |
| `remark` | string | 否 | `''` | maxLen 255 |
```json
// 复制创建示例
{ "channelId": "google", "mode": "copy", "copyFromMarket": "GLOBAL" }
```
成功 `201`：
```json
{ "data": { "id": "100001:JP:google", "market": "JP", "channelId": "google",
  "configStatus": "invalid", "lastCheckMessage": "缺少必填敏感字段或文件字段",
  "copiedFromMarket": "GLOBAL" } }
```
失败（不兼容）`400`：
```json
{ "error": { "code": "MARKET_CHANNEL_INCOMPATIBLE", "message": "market JP only accepts overseas channels" } }
```
失败（重复）`409`：`{ "error": { "code": "CONFLICT", "message": "channel already exists for this market" } }`

### 6.4 隐藏 / 恢复
- **POST `/api/admin/game-market-channels/{id}/hide`** 权限 `channel.write`。请求体可选 `{ "reason": "" }`。成功返回更新后的实例（`hidden=true`，运行态全 false）。
- **POST `/api/admin/game-market-channels/{id}/unhide`** 权限 `channel.write`。成功返回 `hidden=false`，运行态按 §4 重新推导。

### 6.5 渠道实例详情/编辑
- **GET `/api/admin/game-channels/{gameChannelId}`** 权限 `channel.read`。
- **PATCH `/api/admin/game-channels/{gameChannelId}`** 权限 `channel.write`。可改 `enabled`/`remark`（不改 market/channel 身份；身份变更=另起实例）。

### 6.6 渠道包
- **GET `/api/admin/game-channels/{gameChannelId}/packages`** 权限 `channel.read`。
- **POST `/api/admin/game-channels/{gameChannelId}/packages`** 权限 `channel.write`。
  请求：
  | 字段 | 类型 | 必填 | 默认 | 校验 |
  | --- | --- | --- | --- | --- |
  | `packageCode` | string | 是 | — | 同实例下唯一 |
  | `packageName` | string | 是 | — | |
  | `marketCode` | string | 是 | — | 必须等于所属实例 market |
  | `bundleId` | string | 否 | `''` | |
  | `inheritChannelConfig` | bool | 否 | `true` | |
  | `enabled` | bool | 否 | `true` | |
- **PATCH `/api/admin/channel-packages/{packageId}`** 权限 `channel.write`。可改 `packageName/bundleId/inheritChannelConfig/enabled/overrideJson`。

---

## 7. 应用服务与 command/query

- `command/create_market_channel.go`：`CreateMarketChannelCommand{ Env, GameID, Market, ChannelID, Mode, CopyFromMarket, ... }` → 校验兼容性 → 空白或复制（清 secret/file，置 invalid）。
- `command/hide_market_channel.go` / `unhide_market_channel.go`：隐藏/恢复 + 审计。
- `query/list_market_channels.go`：`ListMarketChannelsQuery{ Env, GameID, Market(ALL), ChannelID, Compatible, Hidden, ConfigStatus, Page, PageSize }` + `FilterMarketChannels` + 运行态推导。
- 领域：`domain/channel/visibility.go`（`ValidateMarketChannelCompatibility`）、`domain/channel/game_market_channel.go`（聚合、`NewCopiedMarketChannel`、`Hide/Unhide`、`ResolveRuntimeFlags`）。
- 仓储：`ChannelRepository`（channels/policies 读）、`GameChannelRepository`（实例 CRUD，按 env）、`ChannelPackageRepository`。

---

## 8. 前端信息架构

### 8.1 入口
游戏详情页 → "渠道实例" Tab（`ChannelInstancesTab.vue`）。默认展示当前游戏**所有 market 的所有实例**。

### 8.2 过滤区
- `market`（默认"全部"）、渠道名、兼容状态、隐藏状态（默认不含隐藏，提供"显示隐藏项"）、`config_status`。

### 8.3 列表（`ChannelInstanceTable.vue`）
每行一个 `GameMarketChannel`，列：`market`、渠道名称、国内/非国内、兼容状态、隐藏状态、`config_status`、是否进入最终配置、最近更新时间。
- 同一逻辑渠道在不同 market 分行显示（`GLOBAL/Google`、`JP/Google`）。
- 不兼容行**标红** + "不兼容当前 market"；隐藏行灰显并归入隐藏分组。
- 状态标签组件 `ChannelInstanceStatusTag.vue`（`empty/invalid/valid` + 兼容/隐藏）。
- 运行态徽标组件 `ChannelInstanceRuntimeFlags.vue`（`Included in Snapshot/Sync/Runtime Config`；不可生效时灰显 + tooltip 原因）。

### 8.4 新增渠道抽屉（`CreateMarketChannelDrawer.vue`）
- 先选目标 market → 再按可见性过滤候选渠道（已隐藏/已存在不出现）。
- 两种初始化：空白 / 从其它 market 同渠道复制。
- 复制模式：普通字段带入只读预览，`secret/file` 字段清空且高亮"需补填"，提交后实例为 `invalid`。

### 8.5 隐藏/恢复
- 行内操作"隐藏"/"恢复显示"，二次确认；操作后刷新列表与运行态徽标。

### 8.6 状态约束
- 不得隐藏 `invalid/empty` 状态（`00` 红线）；空/错/权限态遵循全局规范；无 `channel.write` 权限时操作置灰。

---

## 9. 与公共能力的关系

- 模板四件套：同一逻辑渠道在不同 market **复用模板定义**，但**配置实例独立**（secret/file/状态各自维护）—— 见 `00 §4`。
- currency：本模块自身不涉及金额（金额在 `product`/`cashier-template`/`game-cashier`）。
- 密文/文件：渠道实例的 secret/file 字段经 `00 §6` 加密与脱敏；复制清空。
- 审计：`channel.create/hide/unhide`、包的 `package.create/update` 等写操作写 `audit_logs`。
- env：所有实例/包按当前运行环境写入；同步在 `channels`/`packages` section 内按 env diff（`sync`）。

---

## 10. 测试要点

### 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/channel.yaml`；前端 e2e：`tests/frontend/e2e/channels.spec.ts`。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET /api/admin/games/{gameId}/channels | ✓ | ✓ | ✓ | — | — | — | — | — | — | — | 可见性过滤(CN/overseas) 候选筛选 |
| GET /api/admin/games/{gameId}/market-channels | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | — | 可见性过滤(CN/overseas)、隐藏过滤、config_status 综合判定、运行态标识推导 |
| POST /api/admin/games/{gameId}/markets/{market}/channels | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | market-channel 兼容性(MARKET_CHANNEL_INCOMPATIBLE)、复制(secret/file 清空)、config_status=invalid |
| POST /api/admin/game-market-channels/{id}/hide | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ | — | — | ✓ | 隐藏/取消隐藏（禁隐藏 invalid/empty）、运行态全 false |
| POST /api/admin/game-market-channels/{id}/unhide | ✓ | ✓ | ✓ | — | — | ✓ | ✓ | — | — | ✓ | 隐藏/取消隐藏、运行态按 §4 重新推导 |
| GET /api/admin/game-channels/{gameChannelId} | ✓ | ✓ | ✓ | — | — | ✓ | — | ✓ | — | — | config_status 综合判定 |
| PATCH /api/admin/game-channels/{gameChannelId} | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ | — | — | ✓ | market/channel 身份不可变 |
| GET /api/admin/game-channels/{gameChannelId}/packages | ✓ | ✓ | ✓ | — | — | ✓ | — | — | — | — | 按所属实例归属过滤 |
| POST /api/admin/game-channels/{gameChannelId}/packages | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | marketCode 须等于实例 market、inherit/override |
| PATCH /api/admin/channel-packages/{packageId} | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ | — | — | ✓ | override_json 仅存差异、inherit/override |

前端：Playwright e2e（`channels.spec.ts`）覆盖渠道实例列表全 market 展示、可见性过滤(CN/overseas)、隐藏/恢复二次确认、新增/复制抽屉（`secret/file` 清空且高亮需补填）、不兼容标红与运行态徽标灰显态 / vitest 组件（`ChannelInstanceStatusTag.vue`、`ChannelInstanceRuntimeFlags.vue`、`ChannelInstanceTable.vue`、`CreateMarketChannelDrawer.vue`）。

### 补充关键用例
- 可见性：`CN+overseas`、`JP+domestic`、`GLOBAL+domestic` 必须拒绝；`CN+domestic`、`JP+overseas` 通过。
- 复制创建：secret/file 被清空、`config_status=invalid`、`copiedFromMarket` 记录、与来源不联动。
- 隐藏：`hidden=true` ⇒ 运行态三标识全 false；默认列表不含隐藏。
- 全 market 默认：`market=ALL` 返回多 market 多渠道。
- 运行态推导：`invalid` 或 `incompatible` 时 `IncludedInRuntimeConfig=false` 且给出原因。
- 唯一性：同 `(env,game,market,channel)` 重复创建被拒。

---

## 11. 未决问题与显式假设
- 假设"渠道身份变更（改 market/channel）"一律视为删除旧实例 + 新建，不支持原地改身份。
- 假设兼容性不落库、实时派生；若未来 region 规则频繁变动需要历史快照，再单独引入。
- `copied_from_market` 仅用于审计/展示，不参与任何运行时联动。
- 渠道包的 `override_json` 具体结构由 `product`（商品/IAP 覆盖）与 `channel-login`（登录）各自定义，本模块只约束其为 JSONB 且默认 `{}`。
