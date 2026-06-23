---
id: channel
code: "12"
title: 渠道与渠道实例（GameMarketChannel）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [game, common]
code_paths:
  - services/admin-api/internal/domain/channel
  - services/admin-api/internal/transport/http/channels
  - apps/admin-web/src/views/channels
---

# 12 · 渠道与渠道实例 — Compact Spec

> 代码生成用精简规格。完整背景/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（env 模型 §2.2、Market 语义 §3.2、ConfigStatus 状态机 §3.4、模板四件套 §4、密文 §6、API 包络/错误码 §7、审计 §8、红线 §9）与 `../../01-structure.md`（env/分层）。
> 结构性核心：把"渠道"从游戏级共享改为**按 market 拆分的实例 GameMarketChannel**，定义可见性/复制创建/隐藏/运行态标识/渠道包。

## 边界 / 红线
- 维护平台级渠道主数据 `channels` 与策略 `channel_policies`；游戏维度渠道实例 `game_channels`（GameMarketChannel）与渠道包 `channel_packages`。
- 渠道**强制登录**(`channel-login`) 与**自有账号认证**(`account-auth`) 底层分开，本模块只提供"渠道实例"挂载点。
- 被隐藏 / 不兼容 / 无效(`config_status!=valid`) 的实例：**不进默认列表、不进快照、不参与同步、不进客户端最终配置**（00 §9）。
- 可见性/兼容性是纯函数（`domain/channel`，无 IO），服务端强制二次校验，不能只信前端。

## 领域模型（internal/domain/channel）
聚合 `GameMarketChannel` = "某游戏 + 某 market + 某渠道" 唯一实例（落表 `game_channels`）。env 由所在环境 schema 决定，不落列。
- 实例身份：`(gameId, market, channelId)`；隐藏态 `hidden/hiddenBy/hiddenAt`；配置状态 `config_status`。
- 兼容性**派生不落库**：由 `market` 与 `channels.region` 经可见性规则实时判定（规则变更不留脏数据）。
- 不变量：同 `(gameId, market, channelId)` 在所属 schema 内唯一；创建/迁移须满足可见性；`hidden=true ⇒ IncludedInSnapshot/Sync/RuntimeConfig 全 false`。
- 值对象/纯规则：`Market`(00 §3)、`ChannelRegion`(domestic/overseas)、`ValidateMarketChannelCompatibility(market, region) error`、`ResolveRuntimeFlags(instance) RuntimeFlags`（三态只读标识）。

## 数据模型
平台级表（schema `platform`）2 张：`channels`、`channel_policies`；游戏维度业务表 2 张（每环境 schema 各一份、**不带 env 列**，env 由 search_path 决定）：`game_channels`、`channel_packages`。
公共列约定：`id BIGSERIAL PK`、`enabled BOOLEAN DEFAULT TRUE`、`sort INT DEFAULT 0`（仅 channels）、`created_at/updated_at TIMESTAMPTZ DEFAULT NOW()`；下表只列业务字段。

### channels（平台级，新增 region — D3）
| 列 | 类型 | 约束 |
| --- | --- | --- |
| channel_id | VARCHAR(64) | UNIQUE, NOT NULL 业务键 |
| channel_name | VARCHAR(64) | NOT NULL |
| channel_type | VARCHAR(32) | CHECK in store/oem/web/direct/mini_game |
| region | VARCHAR(16) | **新增** CHECK in domestic/overseas（D3） |

UNIQUE(channel_id)。索引建议 (region)、(enabled, sort)。
迁移：`ALTER TABLE channels ADD COLUMN region VARCHAR(16) NOT NULL DEFAULT 'overseas' CHECK (region IN ('domestic','overseas'));` 回填后可去 DEFAULT。
seed region：google/apple=overseas；huawei_cn/xiaomi_cn/oppo_cn/vivo_cn/wechat_mini_game/douyin_mini_game=domestic。

### channel_policies（平台级）
| 列 | 类型 | 约束 |
| --- | --- | --- |
| channel_id_ref | BIGINT | NOT NULL FK→channels(id), UNIQUE |
| login_mode | VARCHAR(16) | CHECK in channel_only/account_system |
| payment_mode | VARCHAR(16) | CHECK in channel_only/hybrid/cashier_only |
| login_locked | BOOLEAN | NOT NULL DEFAULT FALSE 锁后游戏侧不可改登录策略 |
| payment_locked | BOOLEAN | NOT NULL DEFAULT FALSE 锁后游戏侧不可改支付策略 |

seed：huawei_cn/xiaomi_cn/oppo_cn/vivo_cn ⇒ login_mode=channel_only, payment_mode=channel_only, login_locked=payment_locked=TRUE；其余 ⇒ login_mode=account_system, payment_mode=hybrid, locked=FALSE。

### game_channels（游戏维度业务表 / 每环境 schema / 不带 env 列 — D2）
| 列 | 类型 | 默认 | 约束/说明 |
| --- | --- | --- | --- |
| game_id_ref | BIGINT | — | NOT NULL FK→games(id)（同 schema 普通 FK） |
| market_code | VARCHAR(32) | — | **新增** NOT NULL CHECK in GLOBAL/JP/KR/SEA/HMT/CN（D2） |
| channel_id_ref | BIGINT | — | NOT NULL FK→platform.channels(id)（跨 schema 普通 FK） |
| hidden | BOOLEAN | FALSE | **新增** 手动隐藏 |
| hidden_by | VARCHAR(128) | `''` | **新增** 隐藏操作人 |
| hidden_at | TIMESTAMPTZ | NULL | **新增** |
| config_status | VARCHAR(16) | `'empty'` | **新增** CHECK in empty/invalid/valid |
| last_check_at | TIMESTAMPTZ | NULL | **新增** |
| last_check_message | VARCHAR(255) | `''` | **新增** |
| copied_from_market | VARCHAR(32) | `''` | **新增** 复制来源 market（仅创建时记录，无运行时联动） |
| remark | VARCHAR(255) | `''` | |

唯一键：`UNIQUE(game_id_ref, market_code, channel_id_ref)`（不前置 env）。索引 (game_id_ref)、(game_id_ref, market_code)、(channel_id_ref)。
迁移：ADD COLUMN market_code/hidden/hidden_by/hidden_at/config_status/last_check_at/last_check_message/copied_from_market（默认同上）；`DROP CONSTRAINT IF EXISTS game_channels_game_id_ref_channel_id_ref_key` 后 `ADD CONSTRAINT ... UNIQUE (game_id_ref, market_code, channel_id_ref)`。每个环境 schema 各执行一次。

### channel_packages（游戏维度业务表 / 每环境 schema / 不带 env 列）
| 列 | 类型 | 默认 | 约束/说明 |
| --- | --- | --- | --- |
| game_channel_id_ref | BIGINT | — | NOT NULL FK→game_channels(id)（同 schema 普通 FK） |
| package_code | VARCHAR(64) | — | NOT NULL |
| package_name | VARCHAR(128) | — | NOT NULL |
| market_code | VARCHAR(32) | — | 须与所属渠道实例 market 一致（应用层校验） |
| bundle_id | VARCHAR(128) | `''` | |
| inherit_channel_config | BOOLEAN | TRUE | 是否继承渠道实例配置 |
| override_json | JSONB | `{}` | 包级覆盖载荷（仅存差异） |

唯一键：`UNIQUE(game_channel_id_ref, package_code)`（不前置 env）。索引 (game_channel_id_ref)。

### env 隔离（00 §2.2 / 01 §4.3）
- 父子业务表行必然同 schema（同 env）：`game_channels.game_id_ref→games(id)`、`channel_packages.game_channel_id_ref→game_channels(id)` 用同 schema 普通 FK，**无需复合 env 外键/应用层 env 一致性校验**。
- `game_channels.channel_id_ref→platform.channels(id)` 用跨 schema 普通 FK。
- 运行时 `search_path = <env>, platform`；业务表仓储 SQL 不写 schema 前缀、不带 env 谓词、不允许跨 schema 写。

## 枚举与默认
| 项 | 取值 / 默认 |
| --- | --- |
| Market | GLOBAL/JP/KR/SEA/HMT/CN，默认 GLOBAL |
| ChannelRegion | domestic/overseas（seed 固定，无默认） |
| ChannelType | store/oem/web/direct/mini_game（无默认） |
| LoginMode | channel_only/account_system，默认 account_system |
| PaymentMode | channel_only/hybrid/cashier_only，默认 hybrid |
| ConfigStatus | empty/invalid/valid，新建默认 empty；**复制创建后强制 invalid** |
| enabled | 默认 TRUE |
| hidden / login_locked / payment_locked | 默认 FALSE |
| inherit_channel_config | 默认 TRUE |
| override_json | 默认 {} |
| bundle_id/remark/hidden_by/last_check_message/copied_from_market | 默认 '' |

### 运行态标识（派生，只读，三态布尔）
- `IncludedInRuntimeConfig = !hidden && compatible && config_status=='valid'`
- `IncludedInSnapshot = IncludedInRuntimeConfig`
- `IncludedInSync = !hidden && compatible && config_status=='valid'`（口径一致：隐藏/不兼容/无效一律不参与同步，落实 00 §9）
- 任一为 false 时前端须给原因：`hidden` / `incompatible` / `invalid_config`。

## 业务规则与状态机

### 可见性 / 兼容性（纯函数，服务端强制）
```text
ValidateMarketChannelCompatibility(market, region):
  if market == CN and region != domestic -> error MARKET_CHANNEL_INCOMPATIBLE
  if market != CN and region != overseas -> error MARKET_CHANNEL_INCOMPATIBLE
  return ok
```
- 新增时按目标 market 过滤候选渠道（前端），服务端必须二次校验。GLOBAL 仅显示 overseas。
- 已存在实例用同一函数派生兼容性；不兼容 ⇒ 列表标红、提示"不兼容当前 market"，**不自动删除、保留配置**。

### 创建（空白 / 复制）
- 同 `(game, market, channel)` 在当前 schema 已存在 ⇒ 拒绝（`CONFLICT`）。已隐藏渠道默认不进"新增可选列表"。
- 复制创建（从其它 market 同渠道，`NewCopiedMarketChannel`）：仅复制普通字段；`secret`/`file` 字段清空；仅创建时复制，创建后新旧不联动；新实例 `config_status=invalid`（普通字段已带入但敏感/文件字段未补齐）；`last_check_message` 提示"缺少必填敏感字段或文件字段"；记 `copied_from_market`。

### 隐藏 / 恢复
- `hide(operator)`：hidden=true、hidden_by=operator、hidden_at=now；审计 `channel.hide`。
- `unhide()`：hidden=false、清 hidden_by/hidden_at；审计 `channel.unhide`。
- 隐藏后不进默认列表（除非显式"显示隐藏项"）、不进快照/同步/客户端最终配置；保留记录可恢复。
- UI 须区分"不兼容"（规则下不可用但保留）vs"已隐藏"（管理员主动移出生效集）。**不得隐藏 invalid/empty 状态**（00 红线）。

### ConfigStatus 推导（遵循 00 §3.4）
- 渠道实例 config_status 由模板驱动配置综合判定，覆盖三类来源：渠道登录(`channel-login`)、IAP(`product`)、功能插件(`feature-plugin`，含所有 `required=true` 必接插件)。缺必填/敏感/文件字段 ⇒ invalid；全过 ⇒ valid；未建任何配置 ⇒ empty。
- **必接插件口径**：任一 `required=true` 插件 `enabled=false` 或 `config_status!=valid`，渠道实例 config_status 即不得为 valid（与 feature-plugin §1.3、snapshot §5.1 一致）。
- 复制创建强制 invalid。

### 渠道包 inherit/override
- `inherit_channel_config=true`：沿用渠道实例配置，`override_json` 仅存差异；`=false`：用自身完整配置（仍受模板校验）。
- 包 `market_code` 必须与所属渠道实例一致。

## 后端 API（前缀 /api/admin，包络见 00 §7；读 channel.read / 写 channel.write）

### 资源 ID 口径（统一）
- 路径参数一律用 int64 `gameChannelId`(= game_channels.id)；前端从列表行 `gameChannelId` 字段回传，不解析复合串。
- 复合串 `gameId:market:channelId`（如 `100001:GLOBAL:google`）仅作展示 `displayKey`/列表行 key，不作任何接口路径参数。
- 列表每行同时返回 `gameChannelId`(int64) 与 `displayKey`(复合串)。

GET `/games/{gameId}/channels`（列该游戏可用渠道主数据，新增时筛选）
→ items[]: { channelId, channelName, channelType, region, loginMode, paymentMode, loginLocked, paymentLocked }

GET `/games/{gameId}/market-channels`（默认当前 schema 全 market 所有实例）
Query（全可选）: market(默认 ALL), channelId(默认 ''), compatible(bool 不限), hidden(bool 默认 false), configStatus(empty/invalid/valid 不限), page/pageSize(默认 1/20)
→ items[]: { gameChannelId, displayKey, gameId, market, channelId, region, compatible, hidden, configStatus, includedInSnapshot, includedInSync, includedInRuntimeConfig, copiedFromMarket, updatedAt } + page/pageSize/total

POST `/games/{gameId}/markets/{market}/channels`（创建空白/复制，channel.write）
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| channelId | string | 是 | — | 存在且 region 与 market 兼容 |
| mode | enum | 否 | empty | empty / copy |
| copyFromMarket | string | mode=copy 时必填 | '' | 同游戏同渠道在该 market 须已存在 |
| enabled | bool | 否 | true | |
| remark | string | 否 | '' | maxLen 255 |
→ 成功 201: { gameChannelId, displayKey, market, channelId, configStatus, lastCheckMessage, copiedFromMarket }（复制时 configStatus=invalid, lastCheckMessage="缺少必填敏感字段或文件字段"）
→ 失败：不兼容 400 `MARKET_CHANNEL_INCOMPATIBLE`；重复 409 `CONFLICT`。

POST `/game-channels/{gameChannelId}/hide`（channel.write）请求体可选 `{reason}` → 返回更新后实例（hidden=true，运行态全 false）
POST `/game-channels/{gameChannelId}/unhide`（channel.write）→ hidden=false，运行态按 §运行态标识 重新推导

GET `/game-channels/{gameChannelId}`（channel.read）实例详情
PATCH `/game-channels/{gameChannelId}`（channel.write）可改 enabled/remark（不改 market/channel 身份；身份变更=另起实例）

GET `/game-channels/{gameChannelId}/packages`（channel.read）
POST `/game-channels/{gameChannelId}/packages`（channel.write）
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| packageCode | string | 是 | — | 同实例下唯一 |
| packageName | string | 是 | — | |
| marketCode | string | 是 | — | 须等于所属实例 market |
| bundleId | string | 否 | '' | |
| inheritChannelConfig | bool | 否 | true | |
| enabled | bool | 否 | true | |
PATCH `/channel-packages/{packageId}`（channel.write）可改 packageName/bundleId/inheritChannelConfig/enabled/overrideJson

错误码：`MARKET_CHANNEL_INCOMPATIBLE`(400)、`CONFLICT`(409 唯一标识冲突)、`VALIDATION_FAILED`。

## 应用服务 / 仓储
- command：`CreateMarketChannelCommand{ Env, GameID, Market, ChannelID, Mode, CopyFromMarket, ... }`（校验兼容性→空白或复制清 secret/file 置 invalid）、`HideMarketChannel`/`UnhideMarketChannel`（+审计）。
- query：`ListMarketChannelsQuery{ Env, GameID, Market(ALL), ChannelID, Compatible, Hidden, ConfigStatus, Page, PageSize }` + `FilterMarketChannels` + 运行态推导。
- 领域：`domain/channel/visibility.go`(`ValidateMarketChannelCompatibility`)、`domain/channel/game_market_channel.go`（聚合、`NewCopiedMarketChannel`、`Hide/Unhide`、`ResolveRuntimeFlags`）。
```go
type ChannelRepository interface { /* 读 platform.channels / platform.channel_policies */ }
type GameChannelRepository interface { /* 实例 CRUD；连接按当前 env 设 search_path，SQL 不写 schema 前缀/不带 env 谓词 */ }
type ChannelPackageRepository interface { /* 包 CRUD */ }
```
- 审计：`channel.create/hide/unhide`、`package.create/update` 写 audit_logs。

## 前端要点（游戏详情 → "渠道实例" Tab）
- `ChannelInstancesTab.vue` 默认展示该游戏全 market 所有实例；过滤区：market(默认全部)、渠道名、兼容状态、隐藏状态(默认不含，可"显示隐藏项")、config_status。
- `ChannelInstanceTable.vue` 每行一个 GameMarketChannel：market、渠道名、国内/非国内、兼容状态、隐藏状态、config_status、是否进最终配置、最近更新。同逻辑渠道不同 market 分行（GLOBAL/Google、JP/Google）；不兼容行标红+提示；隐藏行灰显归隐藏分组。
- 组件：`ChannelInstanceStatusTag.vue`（empty/invalid/valid + 兼容/隐藏）、`ChannelInstanceRuntimeFlags.vue`（Snapshot/Sync/Runtime Config 徽标，不可生效灰显+tooltip 原因）。
- `CreateMarketChannelDrawer.vue`：先选 market→按可见性过滤候选（已隐藏/已存在不出现）；空白或从其它 market 同渠道复制；复制模式普通字段只读预览、secret/file 清空高亮"需补填"，提交后 invalid。
- 隐藏/恢复行内操作+二次确认；无 channel.write 置灰；不得隐藏 invalid/empty。

## 与公共能力 / 下游
- 模板四件套(00 §4)：同逻辑渠道跨 market 复用模板定义，但配置实例独立（secret/file/状态各自维护）。密文/文件(00 §6)：实例 secret/file 加密脱敏、复制清空。审计(00 §8)：channel.create/hide/unhide、package.create/update。
- 被依赖：`account-auth`/`channel-login`（登录配置挂渠道实例）、`product`（IAP 包级映射）、`payment`（按 channel/package 选择器）、`snapshot`（最小生效单元）、`sync`（channels/packages section，sandbox→production 同库跨 schema diff/upsert）。
- 本模块不涉及金额（金额在 product/cashier-template/game-cashier）。

## 关键假设
- 渠道身份变更（改 market/channel）一律视为删除旧实例+新建，不支持原地改身份。
- 兼容性不落库、实时派生；若未来 region 规则频繁变动需历史快照再单独引入。
- `copied_from_market` 仅用于审计/展示，不参与任何运行时联动。
- 渠道包 `override_json` 具体结构由 product（IAP 覆盖）与 channel-login（登录）各自定义，本模块只约束其为 JSONB 且默认 `{}`。
