---
id: snapshot
code: "20"
title: 配置快照与运行时配置合并 — 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [channel, account-auth, channel-login, feature-plugin, product, cashier-template, game-cashier, payment, game, common]
code_paths:
  - services/admin-api/internal/domain/snapshot
---

# 20 · 配置快照与运行时配置合并 — Compact Spec

> 代码生成用精简规格。完整背景/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（env 模型 D1 §2、统一包络/错误码 §7、密文 §6、审计 §8、Market 语义 §3.2、模板四件套 §4、scope §4.1.1）与 `../../01-structure.md`。
> 职责：把 `game`～`payment` 各模块的「有效配置」按 `market` 合并规则解析、拼装为**客户端可直接消费的最终配置 JSON**，以**确定性、可复现**方式落为版本化快照（生成/列表/发布/下载），作为 `sync` 的数据基础。合并只在生成快照时执行一次，结果固化进 `config_json`。

## 边界 / 红线
- **不做实时合并**：合并只发生在快照生成时刻，客户端/同步/Dashboard 均消费固化结果。
- **不重定义合并规则**：唯一事实来源是 `00 §3.2` 与 `docs/superpowers/specs/2026-06-16-market-channel-sync-design.md`，本模块只做工程化落地。
- **不收纳无效数据**：被隐藏/不兼容/`config_status!=valid`/`enabled=false` 的任何实例一律不进 `config_json`。
- **不存明文密钥**：`config_json` 内 secret 位为占位/引用，绝不含明文。
- **不跨 env**：快照永远落在当前运行环境 schema（D1）；同一 `config_version` 在不同环境 schema 下是不同物理行。
- **不在 production 盲写**：production 不提供「重新生成+直接覆盖同步」捷径；其 generate 仅用于核对/补偿，受权限码约束。

## 数据模型
本模块仅 1 张表 `game_config_snapshots`：**游戏维度业务表**，每环境 schema（develop/sandbox/production）各一份同名同结构，**不带 env 列**（D1）。
公共列约定：`id BIGSERIAL PK`、`created_at/updated_at TIMESTAMPTZ DEFAULT NOW()`。业务表 SQL 不写 schema 前缀、不带 env 谓词，目标 schema 由连接 `search_path` 决定。

### game_config_snapshots（游戏维度业务表 / 每环境 schema）
| 列 | 类型 | 默认 | 约束 / 说明 |
| --- | --- | --- | --- |
| game_id_ref | BIGINT | — | `REFERENCES games(id)`（同 schema 普通 FK，天然同 env） |
| config_schema_version | VARCHAR(32) | 代码常量（如 `'1.0'`） | 配置 JSON 结构版本 |
| config_version | VARCHAR(32) | 生成时产出（见算法） | 内容版本号 |
| config_json | JSONB | — | per-game、按 market 分区的最终配置（结构见样例） |
| file_name | VARCHAR(255) | `game_<gameId>_<configVersion>.json` | 下载文件名 |
| file_hash | VARCHAR(128) | 生成时产出 | `sha256` 十六进制，对 canonical(config_json) 计算 |
| storage_key | VARCHAR(255) | `''` | 可选 JSON 外置对象存储 key；空=内联 config_json |
| status | VARCHAR(32) | `'draft'` | `CHECK (status IN ('draft','published'))` |
| generated_at | TIMESTAMPTZ | `NOW()` | 生成时间（可注入以复现） |
| published_at | TIMESTAMPTZ | `NULL` | 发布时置 `NOW()`；draft 时空 |

```sql
-- 每环境 schema 各应用一份；不写 schema 前缀、不带 env 谓词
ALTER TABLE game_config_snapshots ADD CONSTRAINT chk_gcs_status
  CHECK (status IN ('draft','published'));
ALTER TABLE game_config_snapshots
  ADD CONSTRAINT uq_gcs_game_version UNIQUE (game_id_ref, config_version);
CREATE INDEX IF NOT EXISTS idx_gcs_game_generated
  ON game_config_snapshots (game_id_ref, generated_at DESC);
CREATE INDEX IF NOT EXISTS idx_gcs_game_status
  ON game_config_snapshots (game_id_ref, status);
```

## 枚举与默认
- `SnapshotStatus` ∈ {draft, published}，默认 draft；仅允许 `draft -> published`（I5，published 不可回退）。
- `config_schema_version` 代码常量，默认 `'1.0'`。
- `Market` 分区键: GLOBAL/JP/KR/SEA/HMT/CN（复用 `00 §3`）。
- `storage_key` 默认 `''`（空=内联）；`file_name` 默认 `game_<gameId>_<configVersion>.json`；`generated_at` 默认 `NOW()`（可注入复现）。
- 所在 schema = 当前运行环境 schema（行内不带 env 列）。

## 领域模型（internal/domain/snapshot）
聚合根 `ConfigSnapshot`：归属环境由所在 schema 决定（行内无 env）。值对象 `RuntimeConfig`（= config_json 内存表示）。
```text
ConfigSnapshot: id, gameRef, configSchemaVersion, configVersion,
  config(RuntimeConfig), fileName, fileHash, storageKey,
  status(SnapshotStatus), generatedAt, publishedAt*, timestamps
RuntimeConfig: schemaVersion, gameRef, generatedAt,
  markets map[Market]MarketConfig, checksum
MarketConfig: market, game(GameBaseConfig 法务/账号认证/商品...),
  channels []ResolvedChannel, paymentRoutes []ResolvedRoute
ResolvedChannel: channelId, region(domestic|overseas),
  sourceMarket(GLOBAL 或具体 market，便于审计/调试),
  login *LoginConfig, iap *IapConfig, packages []PackageConfig
```

### 领域不变量
- **I1 环境一致性**：快照与其引用源数据必在同一环境 schema（search_path 决定，无需额外校验）。
- **I2 有效性闭包**：`MarketConfig.channels` 每实例满足「未隐藏 ∧ 兼容当前 market ∧ config_status=valid ∧ enabled=true」，不满足者合并阶段即剔除。
- **I3 实例级覆盖**：具体海外 market 覆盖 GLOBAL 时以「完整实例」为单位替换，**禁止字段级深度合并**。
- **I4 确定性**：同一份源数据 → `BuildRuntimeConfig` 产出字节级一致 canonical JSON → `fileHash` 一致。
- **I5 状态单调**：仅 `draft -> published`。
- **I6 密文不外泄**：`config_json` 内 secret 位均为占位/引用，绝不含明文。

`BuildRuntimeConfig` 位于 `domain/snapshot`，**纯函数**：输入「已加载并已过滤的有效数据视图」，输出 `RuntimeConfig`；不访问 DB、不读时钟（`generatedAt` 由调用方注入）、不做加解密（密文以引用形式在输入视图中提供）。保证可单测、可复现（满足 I4）。

## 业务规则与合并算法（核心，完整保留）

### 有效数据筛选（合并前置）
进入合并的每个渠道实例必须满足（`00 §9` + `12-channel §5`）：`!hidden ∧ compatible(market, region) ∧ config_status=='valid' ∧ enabled==true`。游戏级配置（法务/账号认证/商品/收银台绑定/支付路由）同样仅取有效项。渠道实例下的**功能插件实例**（`15`）同样按此筛选，且**必接（required）插件未达 valid 时该渠道实例视为无效**，不进合并。

### 参数作用域过滤（scope，`00 §4.1.1`）
所有模板四件套驱动的配置（账号认证/渠道登录/渠道 IAP/收银台 provider/功能插件）拼装进 `config_json` 时，**逐字段按 scope 过滤：只纳入 `scope ∈ {client, both}` 的字段**；`scope == server`（服务端密钥、回调校验密钥等）**不写入客户端最终配置**。
- 过滤在「有效数据筛选」之后、「market 合并」之前进行。
- `scope` 缺省按 `both` 解释（向后兼容）。
- 不改变 `config_status` 判定（必填校验仍含 server 字段）；只决定下发到客户端配置的内容。

### 三类 market 合并规则（唯一事实来源 `00 §3.2` + spec）
```text
BuildRuntimeConfig(game, targetMarket, validData):
  base = game 级有效配置(法务/账号认证/商品/收银台快照/...)
  switch targetMarket:
    case CN:
      channels = validData.channels[CN]                       # 不加载 GLOBAL
    case GLOBAL:
      channels = validData.channels[GLOBAL]
    case JP/KR/SEA/HMT:
      channels = mergeByInstance(
                   globalChannels = validData.channels[GLOBAL],
                   marketChannels = validData.channels[targetMarket])
                 # 具体 market 实例【整行覆盖】GLOBAL 同 channelId 实例（I3，禁止字段级深合并）
  paymentRoutes = resolveRoutes(game, targetMarket)            # 按 payment market 语义命中
  return MarketConfig{ market: targetMarket, game: base, channels, paymentRoutes }
```
`mergeByInstance`：以 `channelId` 为键，先放 GLOBAL 实例，再用具体 market 实例**整体替换**同键项；具体 market 独有的追加。
覆盖范围（I3）：同渠道实例/包/登录配置/IAP 配置/支付路由，具体 market 覆盖 GLOBAL **以完整实例为单位**，不得字段级拼接。
排除（I2）：被隐藏/不兼容/`config_status!=valid`/`enabled=false` 的实例及其下游（包/登录/IAP）一律不进 `config_json`。

### 确定性版本与 hash（I4）
- `config_json` 序列化为 **canonical JSON**（键有序、无多余空白、稳定数组序）。
- `file_hash = sha256(canonical(config_json))`（十六进制）。
- `config_version` 生成规则：`<yyyymmddHHMMSS>-<file_hash 前 8 位>`（`generated_at` 可注入以复现）。
- 同一份有效数据 ⇒ 同 `file_hash`（用于 `sync` 的 diff 基线与去重）。

### config_json 结构样例（按 market 分区）
```json
{
  "schemaVersion": "1.0",
  "gameId": "100001",
  "generatedAt": "2026-06-15T10:00:00Z",
  "markets": {
    "GLOBAL": {
      "game": { "legalLinks": [], "accountAuth": [], "products": [] },
      "channels": [
        { "channelId": "google", "region": "overseas", "sourceMarket": "GLOBAL",
          "login": {}, "iap": {}, "packages": [] }
      ],
      "paymentRoutes": []
    },
    "JP": { "game": {}, "channels": [
        { "channelId": "google", "region": "overseas", "sourceMarket": "JP" } ],
      "paymentRoutes": [] },
    "CN": { "game": {}, "channels": [
        { "channelId": "huawei_cn", "region": "domestic", "sourceMarket": "CN" } ],
      "paymentRoutes": [] }
  }
}
```
> CN 分区不含 GLOBAL 实例；JP 分区中 `google` 的 `sourceMarket=JP` 表示其覆盖了 GLOBAL 同名实例。secret 位脱敏（I6）。

### 发布流程
- 生成 ⇒ `draft` 快照。
- 发布 ⇒ 校验当前为 `draft`，置 `status=published` + `published_at=NOW()`，写审计 `snapshot.publish`。
- 同环境 schema 内同 game 可有多份历史快照；「当前生效」按最近 `published_at` 取最新（本期）。

## 后端 API（前缀 /api/admin，包络 00 §7；读 game.read / 生成 snapshot.generate / 发布 snapshot.publish）

POST `/api/admin/games/{gameId}/config-snapshots/generate`（snapshot.generate）
- 行为：当前环境 schema 拉有效数据 → `BuildRuntimeConfig`（各 market）→ 算 hash/version → 落 draft 快照。
- 成功 201：
```json
{ "data": { "id": 12, "configVersion": "20260615100000-a1b2c3d4",
  "fileHash": "a1b2c3d4...", "status": "draft", "generatedAt": "2026-06-15T10:00:00Z" } }
```

GET `/api/admin/games/{gameId}/config-snapshots`（game.read，分页，`generated_at` 降序）
→ items[]: { id, configVersion, status, fileHash, generatedAt, publishedAt }

POST `/api/admin/game-config-snapshots/{snapshotId}/publish`（snapshot.publish）
- 校验为 draft，否则 `VERSION_STATE_INVALID` / `CONFLICT`。

GET `/api/admin/game-config-snapshots/{snapshotId}/download`（game.read）
- 返回 config_json（或经 storage_key 重定向），`Content-Disposition: attachment; filename=<file_name>`，密文位脱敏（I6）。

错误码：`NOT_FOUND`、`VALIDATION_FAILED`、`VERSION_STATE_INVALID`、`CONFLICT`。

## 应用服务 / 仓储
- `ConfigSnapshotService`：`Generate`（编排：加载有效数据 → `BuildRuntimeConfig` → hash/version → 落库 + 审计）、`List`、`Publish`、`Download`。
- 领域纯逻辑：`domain/snapshot/build_runtime_config.go`（`BuildRuntimeConfig`，无 IO，可单测）。
- 仓储：`ConfigSnapshotRepository`（SQL 不带 env 谓词，目标 schema 由 search_path 决定）；只读聚合各模块仓储的「有效数据视图」。
- 审计：`snapshot.generate` / `snapshot.publish` 写 `platform.audit_logs`（env 记当前运行环境）。

## 前端要点（游戏详情 → "配置快照" 区域）
- 快照列表（version/status/hash/生成时间/发布时间）。
- 「生成快照」按钮（生成 draft）。
- JSON 预览（按 market 分区折叠展示，密文脱敏）。
- 下载入口。
- 「发布」操作（draft → published，二次确认）。
- 空/错/权限态遵循全局；无 `snapshot.generate/publish` 置灰。

## 与公共能力 / 下游
- 数据来源：聚合 `game`～`payment` 各模块的有效数据（见数据流）。
- 密文（00 §6）：config_json 内 secret 字段不落明文，存占位 `"***"` 或密文引用键；下载/预览均脱敏。
- payment：按 per-game per-market 调 `ResolveRoute` 写运行时配置；禁用/无效不进快照。
- sync：published 快照作为 config section 的数据基础；`file_hash` 用作 diff 基线/去重。

## 关键假设
- 「当前生效快照」按最近 `published_at` 取最新；若需显式 `is_current` 标志，迁移可追加。
- 大 `config_json` 默认内联存储（`storage_key=''`），超阈值时外置对象存储由 `infra/file` 承接。
- `config_schema_version` 升级策略（客户端兼容）不在本期范围。
- canonical JSON 的具体序列化规范（键序/数字格式）在实现时统一固定。
