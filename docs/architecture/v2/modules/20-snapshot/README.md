---
id: snapshot
code: "20"
title: 配置快照与运行时配置合并
status: target
code_paths:
  - services/admin-api/internal/domain/snapshot
depends_on: [channel, account-auth, channel-login, feature-plugin, product, cashier-template, game-cashier, payment, game, common]
impacts: [sync, testing]
children: []
---

# 20 · 配置快照与运行时配置合并

> 本文件是 v2 架构文档集的模块文档之一，默认继承 `../../00-common.md` 与 `../../01-structure.md` 的全部公共契约（env 模型、统一 API 包络、密文/文件、审计、枚举与默认值清单），冲突以 `00` 为准。
> 本模块对应后端聚合 `internal/domain/snapshot`、应用服务 `ConfigSnapshotService`，对应数据表 `game_config_snapshots`，对应前端"游戏详情 · 配置快照"区域。

本模块负责把分散在 `game`～`payment` 各模块中的"有效配置数据"，按 `market` 的合并规则解析、拼装成**客户端可直接消费的最终配置 JSON**，并以**确定性、可复现**的方式落地为版本化快照，提供生成 / 列表 / 发布 / 下载能力，作为 `sync`（`sandbox -> production` 同步）的数据基础之一。

---

## 1. 模块边界

### 1.1 一句话定义

- **配置快照（Config Snapshot）**：某个游戏在某个 `env` 下、某个时间点的"客户端最终配置 JSON"的物化版本。一条快照 = 一份完整的、per-game 的 `config_json`，其内部**按 `market` 分区**，每个 market 下存放"已按合并规则解析后的最终配置"。
- **运行时配置合并（Runtime Config Merge）**：把游戏级配置 + 各 market 的有效渠道实例，按 `00` §3.2 / market-channel-sync 规范的合并规则，解析为每个 market 的最终配置的**纯逻辑过程**。该过程**只在生成快照时执行一次**，结果固化进 `config_json`；客户端、同步、Dashboard 均消费固化结果，不再重复合并。

### 1.2 在系统中的位置（数据流）

```text
[11 游戏主数据] [12 渠道实例] [13 账号认证] [14 渠道登录]
[15 商品/IAP]  [16 收银台模板] [17 游戏收银台] [18 支付路由]
        │  （各模块只暴露"有效数据"：enabled 且 config_status=valid 且未隐藏未不兼容）
        ▼
[19 BuildRuntimeConfig 纯逻辑] ── 按 market 合并 ──► per-game config_json（按 market 分区）
        ▼
[19 ConfigSnapshotService] ── 计算 file_hash / 落库 game_config_snapshots(env=当前) ──► draft 快照
        ▼  publish
[19] published 快照  ──►  [20 同步 config section]  /  客户端拉取  /  [22 Dashboard 只读]
```

### 1.3 明确不做（红线，见 `00` §9）

- **不做实时合并**：客户端不在运行时实时合并配置；合并只发生在快照生成时刻。
- **不重新定义合并规则**：合并规则的唯一事实来源是 `00` §3.2 与 `docs/superpowers/specs/2026-06-16-market-channel-sync-design.md` 的"GLOBAL 与具体 Market 的配置合并规则"，本文只做工程化落地，不得改写语义。
- **不收纳无效数据**：被隐藏 / 不兼容 / `config_status != valid` / `enabled=false` 的任何实例，**一律不进 `config_json`**。
- **不存明文密钥**：`config_json` 内不得出现明文 secret；密文字段按 §9.2 处理。
- **不跨 env**：快照永远归属当前运行 `env`（D1）；同一 `config_version` 在不同 env 下是不同物理行。
- **不在 production 盲写**：production 下不提供"重新生成 + 直接覆盖同步"的捷径；production 快照通常由 `sync` 同步而来，本模块的 generate 在 production 下仅用于核对/补偿，受权限码约束。

---

## 2. 领域模型

### 2.1 聚合与值对象（`internal/domain/snapshot`）

```text
ConfigSnapshot (聚合根)
  ├─ id                 BIGINT
  ├─ env                Environment          // D1：快照归属环境
  ├─ gameRef            GameRef              // game_id_ref -> games.id
  ├─ configSchemaVersion string             // 配置 JSON 结构版本（代码常量，见 §4）
  ├─ configVersion      string              // 内容版本号（生成时按规则产出，见 §5.5）
  ├─ config             RuntimeConfig (值对象，最终落为 config_json)
  ├─ fileName           string              // 下载文件名
  ├─ fileHash           string              // sha256(canonical(config_json))，确定性来源
  ├─ storageKey         string              // 可选：对象存储 key（大 JSON 外置）
  ├─ status             SnapshotStatus       // draft | published
  ├─ generatedAt        time
  ├─ publishedAt        *time
  └─ timestamps         created/updated

RuntimeConfig (合并纯逻辑产出的值对象 / config_json 的内存表示)
  ├─ schemaVersion      string
  ├─ gameRef            string
  ├─ generatedAt        time
  ├─ markets            map[Market]MarketConfig   // 按 market 分区
  └─ checksum           string

MarketConfig
  ├─ market             Market
  ├─ game               GameBaseConfig            // 游戏级基础层（法务、账号认证、商品...）
  ├─ channels           []ResolvedChannel         // 合并后该 market 的最终渠道实例集合
  └─ paymentRoutes      []ResolvedRoute           // 该 market 命中的支付路由（按 18 规则）

ResolvedChannel
  ├─ channelId          string
  ├─ region             ChannelRegion             // domestic | overseas
  ├─ sourceMarket       Market                    // 该实例来源（GLOBAL 或具体 market），便于审计/调试
  ├─ login              *LoginConfig
  ├─ iap                *IapConfig
  └─ packages           []PackageConfig
```

### 2.2 领域不变量（invariants）

- **I1（环境一致性）**：`ConfigSnapshot.env` 必须等于当前运行 env；聚合内引用的所有源数据必须同 env（应用层保证）。
- **I2（有效性闭包）**：`MarketConfig.channels` 中的每个实例都满足"未隐藏 ∧ 兼容当前 market ∧ `config_status=valid` ∧ `enabled=true`"。任何不满足者在合并阶段即被剔除，绝不进入聚合。
- **I3（实例级覆盖）**：具体海外 market 覆盖 GLOBAL 时，以"完整实例"为单位替换，**禁止字段级深度合并**（见 §5.2）。
- **I4（确定性）**：给定同一份源数据集合，`BuildRuntimeConfig` 必须产出字节级一致的 canonical JSON，从而 `fileHash` 一致（见 §5.5）。
- **I5（状态单调）**：`SnapshotStatus` 仅允许 `draft -> published`；published 不可回退为 draft（见 §4.1）。
- **I6（密文不外泄）**：`config_json` 内任何 secret 位均为占位/引用，绝不含明文（见 §9.2）。

### 2.3 RuntimeConfig 合并为纯逻辑（无 IO）

`BuildRuntimeConfig` 位于 `domain/snapshot`，签名为纯函数：输入"已加载并已过滤的有效数据视图"，输出 `RuntimeConfig`。它不访问数据库、不读时钟（`generatedAt` 由调用方注入）、不做加解密（密文已在输入视图中以引用形式提供）。这样保证可单测、可复现（满足 I4）。

---

## 3. 数据模型（逐表逐字段）

本模块仅拥有一张表：`game_config_snapshots`。下表给出 **v2 目标形态**（在 `000001_init.up.sql` 现状基础上由新增迁移补 `env` 列并改唯一键，遵循 D1）。

### 3.1 `game_config_snapshots`（带 env，游戏维度业务表）

| 列 | 类型 | 默认值 | 约束 / 说明 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | — | 主键 |
| `env` | VARCHAR(16) | （无默认，写入取当前运行 env） | **新增（D1）**；`CHECK (env IN ('develop','sandbox','production'))` |
| `game_id_ref` | BIGINT | — | `REFERENCES games(id)`；被引用 `games` 行需同 env |
| `config_schema_version` | VARCHAR(32) | （由代码常量写入，如 `'1.0'`） | 配置 JSON 结构版本，见 §4 |
| `config_version` | VARCHAR(32) | （生成时产出，见 §5.5） | 内容版本号 |
| `config_json` | JSONB | — | **per-game、按 market 分区**的最终配置（结构见 §5.6 样例）；可能较大 |
| `file_name` | VARCHAR(255) | （生成时产出，如 `game_<gameId>_<configVersion>.json`） | 下载文件名 |
| `file_hash` | VARCHAR(128) | （生成时产出） | `sha256` 十六进制；对 canonical(config_json) 计算，确定性来源 |
| `storage_key` | VARCHAR(255) | `''` | 可选：JSON 外置对象存储 key；为空表示直接用 `config_json` 列 |
| `status` | VARCHAR(32) | `'draft'` | `SnapshotStatus`；本模块约束 `CHECK (status IN ('draft','published'))` |
| `generated_at` | TIMESTAMPTZ | `NOW()` | 生成时间（注入到合并逻辑，保证可复现时可显式指定） |
| `published_at` | TIMESTAMPTZ | `NULL` | 发布时间；draft 时为空 |
| `created_at` | TIMESTAMPTZ | `NOW()` | |
| `updated_at` | TIMESTAMPTZ | `NOW()` | |

### 3.2 唯一键与索引（D1 前置 env）

```sql
-- v2 目标（迁移中将现状 UNIQUE(game_id_ref, config_version) 调整为前置 env）
ALTER TABLE game_config_snapshots ADD COLUMN env VARCHAR(16) NOT NULL DEFAULT 'develop';
ALTER TABLE game_config_snapshots ADD CONSTRAINT chk_gcs_env
  CHECK (env IN ('develop','sandbox','production'));
ALTER TABLE game_config_snapshots ADD CONSTRAINT chk_gcs_status
  CHECK (status IN ('draft','published'));

-- 唯一键：前置 env（D1）
ALTER TABLE game_config_snapshots DROP CONSTRAINT IF EXISTS game_config_snapshots_game_id_ref_config_version_key;
ALTER TABLE game_config_snapshots
  ADD CONSTRAINT uq_gcs_env_game_version UNIQUE (env, game_id_ref, config_version);

-- 常用查询索引：按游戏列出快照、按 env+game 查最新 published
CREATE INDEX IF NOT EXISTS idx_gcs_env_game_generated
  ON game_config_snapshots (env, game_id_ref, generated_at DESC);
CREATE INDEX IF NOT EXISTS idx_gcs_env_game_status
  ON game_config_snapshots (env, game_id_ref, status);
```

> 说明：迁移中 `env` 加 `DEFAULT 'develop'` 仅为回填历史行；应用层写入**必须显式取当前运行 env**，不依赖默认值（见 `00` §2.1）。

### 3.3 字段默认值落地清单（呼应 §4）

| 字段 | 落地默认 | 触发时机 |
| --- | --- | --- |
| `status` | `draft` | 生成快照时 |
| `published_at` | `NULL` | 生成时；发布时置为 `NOW()` |
| `storage_key` | `''` | JSON 内联存储时 |
| `generated_at` | 注入值（默认 `NOW()`） | 生成时 |
| `config_schema_version` | 代码常量（如 `1.0`） | 生成时 |

---

## 4. 枚举与默认值清单

| 项 | 取值 / 默认 | 说明 |
| --- | --- | --- |
| `SnapshotStatus` | `draft` / `published`，默认 `draft` | 仅允许 `draft -> published`（I5） |
| `config_schema_version` | 代码常量，默认 `'1.0'` | 客户端配置 JSON 结构版本 |
| `config_version` | 生成时产出（见 §5.5） | 内容版本号 |
| `Market` 分区键 | `GLOBAL/JP/KR/SEA/HMT/CN` | 复用 `00 §3` |
| `storage_key` | 默认 `''` | 空=内联存 `config_json` |
| `file_name` | 默认 `game_<gameId>_<configVersion>.json` | |
| `env` | 当前运行环境 | D1 |
| `generated_at` | 默认 `NOW()`（可注入以复现） | |

---

## 5. 业务规则与合并算法

### 5.1 有效数据筛选（合并前置）
进入合并的每个渠道实例必须满足（`00 §9` + `12-channel` §5）：`!hidden ∧ compatible(market, region) ∧ config_status=='valid' ∧ enabled==true`。游戏级配置（法务/账号认证/商品/收银台绑定/支付路由）同样仅取有效项。渠道实例下的**功能插件实例**（`15-feature-plugin`）同样按此条件筛选，且**必接（required）插件未达 `valid` 时该渠道实例视为无效**，不进合并。

### 5.1.1 参数作用域过滤（scope，`00 §4.1.1`）
所有由模板四件套驱动的配置（账号认证 / 渠道登录 / 渠道 IAP / 收银台 provider / 功能插件）在拼装进 `config_json` 时，**逐字段按 `scope` 过滤：只纳入 `scope ∈ {client, both}` 的字段**；`scope == server` 的字段（仅服务端用，如服务端密钥、回调校验密钥）**不写入客户端最终配置**。
- 过滤在「有效数据筛选」之后、「market 合并」之前进行。
- `scope` 缺省按 `both` 解释（向后兼容）。
- 该过滤不改变 `config_status` 判定（必填校验仍含 server 字段）；它只决定**下发到客户端配置的内容**。

### 5.2 三类 market 合并规则（唯一事实来源：`00 §3.2` + spec）
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
  paymentRoutes = resolveRoutes(game, targetMarket)            # 按 `payment` market 语义命中
  return MarketConfig{ market: targetMarket, game: base, channels, paymentRoutes }
```
`mergeByInstance`：以 `channelId` 为键，先放 GLOBAL 实例，再用具体 market 实例**整体替换**同键项；具体 market 独有的追加。

### 5.3 覆盖范围（实例级，I3）
同渠道实例 / 同渠道包 / 同登录配置 / 同 IAP 配置 / 同支付路由：具体 market 覆盖 GLOBAL，**以完整实例为单位**，不得从 GLOBAL 继承部分字段再与具体 market 字段级拼接。

### 5.4 排除与失效
被隐藏 / 不兼容 / `config_status!=valid` / `enabled=false` 的实例及其下游（包/登录/IAP）一律不进 `config_json`（I2）。

### 5.5 确定性版本与 hash（I4）
- `config_json` 序列化为 **canonical JSON**（键有序、无多余空白、稳定数组序）。
- `file_hash = sha256(canonical(config_json))`（十六进制）。
- `config_version` 生成规则：`<yyyymmddHHMMSS>-<file_hash 前 8 位>`（`generated_at` 可注入以复现）。
- 同一份有效数据 ⇒ 同 `file_hash`（用于 `sync` 的 diff 基线与去重）。

### 5.6 config_json 结构样例（按 market 分区）
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
    "JP": {
      "game": { "legalLinks": [], "accountAuth": [], "products": [] },
      "channels": [
        { "channelId": "google", "region": "overseas", "sourceMarket": "JP",
          "login": {}, "iap": {}, "packages": [] }
      ],
      "paymentRoutes": []
    },
    "CN": {
      "game": { "legalLinks": [], "accountAuth": [], "products": [] },
      "channels": [
        { "channelId": "huawei_cn", "region": "domestic", "sourceMarket": "CN",
          "login": {}, "iap": {}, "packages": [] }
      ],
      "paymentRoutes": []
    }
  }
}
```
> CN 分区不含 GLOBAL 实例；JP 分区中 `google` 的 `sourceMarket=JP` 表示其覆盖了 GLOBAL 同名实例。密文位见 §9.2。

### 5.7 发布流程
- 生成 ⇒ `draft` 快照。
- 发布 ⇒ 校验当前 `draft`，置 `status=published` + `published_at=NOW()`；写审计 `snapshot.publish`。
- 同一 `(env, game)` 可有多份历史快照；"当前生效"由最近 published 决定（实现可加 `is_current` 或按 `published_at` 取最新，本期按最新 published）。

---

## 6. 后端 API

> 前缀 `/api/admin`，遵循 `00 §7` 包络。读 `game.read`，生成/发布 `snapshot.write` / `snapshot.publish`。

- **POST `/api/admin/games/{gameId}/config-snapshots/generate`** 权限 `snapshot.write`
  - 行为：按当前 env 拉取有效数据 → `BuildRuntimeConfig`（各 market）→ 计算 hash/version → 落 `draft` 快照。
  - 成功 `201`：
    ```json
    { "data": { "id": 12, "configVersion": "20260615100000-a1b2c3d4",
      "fileHash": "a1b2c3d4...", "status": "draft", "generatedAt": "2026-06-15T10:00:00Z" } }
    ```
- **GET `/api/admin/games/{gameId}/config-snapshots`** 权限 `game.read`（分页，按 `generated_at` 降序）
  ```json
  { "data": { "items": [ { "id": 12, "configVersion": "...", "status": "published",
    "fileHash": "...", "generatedAt": "...", "publishedAt": "..." } ], "page": 1, "pageSize": 20, "total": 5 } }
  ```
- **POST `/api/admin/game-config-snapshots/{snapshotId}/publish`** 权限 `snapshot.publish`
  - 校验为 `draft`；否则 `VERSION_STATE_INVALID`/`CONFLICT`。
- **GET `/api/admin/game-config-snapshots/{snapshotId}/download`** 权限 `game.read`
  - 返回 `config_json`（或经 `storage_key` 重定向），`Content-Disposition: attachment; filename=<file_name>`，密文位脱敏（§9.2）。

错误码：`NOT_FOUND`、`VALIDATION_FAILED`、`VERSION_STATE_INVALID`、`CONFLICT`。

---

## 7. 应用服务与 command/query
- `ConfigSnapshotService`：`Generate`（编排：加载有效数据 → 调 `BuildRuntimeConfig` → hash/version → 落库）、`List`、`Publish`、`Download`。
- 领域纯逻辑：`domain/snapshot/build_runtime_config.go`（`BuildRuntimeConfig`，无 IO，可单测）。
- 仓储：`ConfigSnapshotRepository`（按 env）；只读聚合各模块仓储的"有效数据视图"。

---

## 8. 前端信息架构
- 游戏详情 → "配置快照" 区域：
  - 快照列表（version/status/hash/生成时间/发布时间）。
  - "生成快照"按钮（生成 draft）。
  - JSON 预览（按 market 分区折叠展示，密文脱敏）。
  - 下载入口。
  - "发布"操作（draft → published，二次确认）。
- 空/错/权限态遵循全局；无 `snapshot.write/publish` 置灰。

---

## 9. 与公共能力的关系
### 9.1 数据来源
聚合 `game`～`payment` 各模块的有效数据（见 §1.2 数据流）。

### 9.2 密文处理
`config_json` 内 secret 字段**不落明文**：存占位（如 `"***"`）或密文引用键；下载/预览均脱敏（`00 §6`、I6）。

### 9.3 审计与 env
- 审计：`snapshot.generate`、`snapshot.publish` 写 `audit_logs`。
- env：快照归属当前运行 env；production 快照通常由 `sync` 同步而来（§1.3）。

---

## 10. 测试要点
- 具体 market 覆盖 GLOBAL（JP 的 google 整行覆盖 GLOBAL 的 google）。
- CN 不加载 GLOBAL 实例。
- 隐藏 / 不兼容 / invalid 实例被排除。
- 确定性：同一有效数据两次生成 `file_hash` 一致。
- 实例级覆盖（非字段级深合并）。
- 发布后 `status=published` + `published_at`。

---

## 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/snapshot.yaml`；前端 e2e：`tests/frontend/e2e/games.spec.ts`（快照生成/发布/下载）。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| POST `/api/admin/games/{gameId}/config-snapshots/generate` | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ | ✓ | — | ✓ | per-game 按 market 分区(D4)、scope 过滤(client/both 进、server 不进)、确定性 hash/可复现、隐藏/不兼容/无效排除、production 不盲写 |
| GET `/api/admin/games/{gameId}/config-snapshots` | ✓ | ✓ | ✓ | — | — | ✓ | — | — | ✓ | — | draft/published 状态列出（按 env+game） |
| POST `/api/admin/game-config-snapshots/{snapshotId}/publish` | ✓ | ✓ | ✓ | — | ✓ | ✓ | ✓ | — | — | ✓ | draft→published 单调（VERSION_STATE_INVALID/CONFLICT） |
| GET `/api/admin/game-config-snapshots/{snapshotId}/download` | ✓ | ✓ | ✓ | — | — | ✓ | — | ✓ | — | — | 密文脱敏(S8)、按 market 分区呈现 |

前端：`games.spec.ts` 覆盖"配置快照"区域（生成 draft / 发布二次确认 / JSON 预览按 market 折叠+脱敏 / 下载入口）、空/错/无 `snapshot.write|publish` 置灰态 / vitest 组件：快照列表（version/status/hash/时间）、JSON 预览（密文脱敏）。

---

## 11. 未决问题与显式假设
- 假设"当前生效快照"按最近 `published_at` 取最新；若需显式 `is_current` 标志，迁移可追加。
- 假设大 `config_json` 默认内联存储（`storage_key=''`），超阈值时外置对象存储由 `infra/file` 承接。
- `config_schema_version` 升级策略（客户端兼容）不在本期范围。
- canonical JSON 的具体序列化规范（键序/数字格式）在实现时统一固定。
