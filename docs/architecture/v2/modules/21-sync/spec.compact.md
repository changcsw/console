---
id: sync
code: "21"
title: Sandbox → Production 同步 — 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [snapshot, channel, account-auth, channel-login, feature-plugin, product, cashier-template, game-cashier, payment, game, common]
code_paths:
  - services/admin-api/internal/domain/sync
  - services/admin-api/internal/transport/http/sync
---

# 21 · Sandbox → Production 同步 — Compact Spec

> 代码生成用精简规格。完整背景/示例/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（env 模型 D1 §2、同库跨 schema §2.2、统一包络/错误码 §7、密文脱敏 §6、审计 §8、红线 §9）与 `../../01-structure.md`（分层、§8 sandbox→production 数据流）。
> 核心：把 sandbox 已配好的某 game 数据，经 **preview → 人工勾选 selected_sections → baseline 复核 → 按 section 有序写入 production**。后端 `internal/domain/sync`（纯领域）+ `internal/app/command`（preview/execute）+ `internal/transport/http/sync`。

## 边界 / 红线
- 仅做 `source_env=sandbox → target_env=production`（D1，同物理库跨 schema 比对/搬运，不跨库）；CHECK 虽允许三值，本期只产出此组合。
- 只读各源模块「有效数据」做 diff + upsert，**不生成业务数据**、不校验单条业务合规（合规由源模块在 sandbox 阶段保证）。
- 无 preview 不直写：`execute` 必携 `baseline_token`（D6 / `00` §9）。
- `production` 运行视图**绝不渲染**任何 `Sync to Production` 入口（`00` §9、`01` §2）。
- 隐藏 / 不兼容 / 无效（`config_status != valid` 或 `enabled=false`）数据**全程排除**（preview 与 execute）。
- 密文字段 preview 恒 `masked=true`，绝不回明文；upsert 取密文/重新加密，不经明文中转。
- sync 域是唯一允许同时访问两个环境 schema 的域（`00` §2.2）。

## 锁定决策回链
- **D1** 同库跨 schema diff（`sandbox.*` ↔ `production.*`）。
- **D6** 基线一致性：execute 前复核 production 当前 hash == `target_hash_before`，不一致 → `SYNC_BASELINE_MISMATCH`。
- section 固定 9 枚举，未识别 → `UNKNOWN_SECTION`。
- `selected_sections` 必须显式声明，未选不得隐式连带写入。
- 依赖校验：所选 section 前置数据在 production 不存在/不兼容 → 拒绝。
- 删除 opt-in：默认不执行，需 `include_deletes=true`。

## 领域模型（internal/domain/sync，纯领域无 IO）
聚合 `sync.Aggregate` 三能力：`preview`（产出 `SyncPreview`）/ `execute`（消费 baseline+selected_sections 产出 `SyncJob`，落库+审计）/ `audit`（执行后写 audit_logs，before/after 脱敏）。

不变量：① 一个 `SyncPreview` 对应一对 `(sandbox, production, game)`；② 一个 `SyncJob` 引用 preview 时刻基线（`source_hash`/`target_hash_before`），执行成功才写 `target_hash_after`；③ `SyncJobItem` 不脱离 `SyncJob`，`section` ∈ 9 枚举；④ 进入 preview/execute 的实体必须有效（未隐藏、market 兼容、`config_status != invalid`、`enabled=true`）。

```text
SyncPreview { gameId, sourceEnv:"sandbox", targetEnv:"production",
  sourceHash, targetHashBefore, hasDiff, baselineToken, sections []DiffSection }
DiffSection { section SyncSection, summary{add,update,delete}, dependencies []SectionDep, changes []DiffChange }
DiffChange  { op SyncOp(add/update/delete), entityType, entityKey(业务唯一键非自增id),
  fieldName(字段级diff;整行增删用"*"), sandboxValue, productionValue, masked }
```
- **add**：production 无此 entityKey，productionValue=null。
- **update**：双方都有、逐字段产出 DiffChange（fieldName 为具体字段）。
- **delete**：production 有、sandbox 无/失效，sandboxValue=null。
- **masked**：fieldName ∈ 模板 `secret_fields_json` 或落 `*_ciphertext` → masked=true，两值恒 `"masked"`。

### BaselineToken（preview↔execute 乐观锁 + 防重放凭证）
```text
payload { gameId, syncJobId(=preview落库的sync_jobs.id,execute据此原地更新同一行),
  sourceEnv:"sandbox", targetEnv:"production", sourceHash, targetHashBefore,
  previewedAt(ISO8601 UTC), expiresAt(=previewedAt+TTL,默认30min),
  nonce(全局唯一随机串,128bit,幂等/防重放键), sig(HMAC-SHA256, 密钥来自 infra/config 同 crypto 密钥域) }
token = base64url(canonicalJSON(payload)) + "." + base64url(sig)
```
对外不透明，前端原样回传，不得解析/篡改。

### hash 语义
- `source_hash`/`target_hash_before`/`target_hash_after` 均为「某 env 下该 game 全部有效数据规范化序列化」的 SHA-256（列长 128，存十六进制或 `sha256-` 前缀）。
- 规范化确定性：固定字段顺序 + key 排序、排除自增 id/created_at/updated_at 等非语义列、排除失效数据、密文以密文（或其哈希）参与而非明文。同一份有效数据多次计算 hash 恒等（供基线复核与幂等判断）。

## 数据模型
两表均为**平台级任务记录表**，位于共享 `platform` schema（**不带 env 列**），环境维度由 `source_env`/`target_env` 字段（值即源/目标 schema 名）显式表达。公共列：`id BIGSERIAL PK`、`created_at/updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`。

### sync_jobs（同步任务）
每次 preview 建一行（`status=previewed`）；execute 成功/失败**原地更新同一行**（写回 `status`/`target_hash_after`/`executed_at`），不另建行。

| 列 | 类型 | 默认 | 约束/语义 |
| --- | --- | --- | --- |
| game_id_ref | VARCHAR(64) | — | NOT NULL；**业务 game_id（字符串）标识逻辑游戏**，不设跨 schema 数值 FK（games 按 env 分多 schema、自增 id 不通用）；存在性由应用层对 `sandbox.games` 校验。 |
| source_env | VARCHAR(16) | — | NOT NULL，CHECK IN(develop,sandbox,production)；本模块恒 `sandbox`。 |
| target_env | VARCHAR(16) | — | NOT NULL，CHECK IN(develop,sandbox,production)；本模块恒 `production`。 |
| source_hash | VARCHAR(128) | — | NOT NULL，预览时刻 sandbox 有效数据规范化哈希。 |
| target_hash_before | VARCHAR(128) | — | NOT NULL，预览时刻 production 哈希；execute 据此复核（D6）。 |
| target_hash_after | VARCHAR(128) | `''` | NOT NULL DEFAULT ''，执行成功后 production 新哈希；未执行/失败为空串。 |
| include_deletes | BOOLEAN | `FALSE` | NOT NULL，本次是否纳入删除（opt-in）。 |
| operator_id | BIGINT | — | NOT NULL，操作者管理员 id（同 `audit_logs.actor_id`）。 |
| operator_note | VARCHAR(255) | `''` | NOT NULL DEFAULT ''。 |
| status | VARCHAR(32) | — | NOT NULL，CHECK IN(previewed,succeeded,failed)；流转校验在应用层（§状态机）。 |
| executed_at | TIMESTAMPTZ | NULL | 可空，仅 succeeded/failed 有值。 |

### sync_job_items（同步明细，逐字段差异，与 sync_jobs 一对多）
每条 DiffChange 落一行（整对象增删 `field_name='*'`）；preview 即写全部（`applied=false`）便于追溯。

| 列 | 类型 | 默认 | 约束/语义 |
| --- | --- | --- | --- |
| sync_job_id_ref | BIGINT | — | REFERENCES sync_jobs(id) NOT NULL。 |
| section | VARCHAR(32) | — | NOT NULL，应用层强制 ∈ 9 枚举（DB 无 CHECK）。 |
| entity_type | VARCHAR(64) | — | NOT NULL（如 product/game_channel/channel_package/game_channel_plugin_config/channel_package_plugin_override/payment_route/config_snapshot）。 |
| entity_key | VARCHAR(128) | — | NOT NULL，业务唯一键（非自增 id），source/target 可对齐。 |
| op | VARCHAR(16) | — | NOT NULL，CHECK IN(add,update,delete)。 |
| field_name | VARCHAR(64) | — | NOT NULL，整行 add/delete 用 `'*'`。 |
| sandbox_value_json | JSONB | `'{}'` | NOT NULL，统一 `{"value":<原值>}` 包裹；masked 时 `{"value":"masked"}`。 |
| production_value_json | JSONB | `'{}'` | NOT NULL，同上。 |
| masked | BOOLEAN | `FALSE` | NOT NULL，仅密文字段 true。 |
| applied | BOOLEAN | `FALSE` | NOT NULL，本次 execute 实际写库成功为 true；未选/跳过/删除未勾选为 false。 |

### nonce 去重存储（D 决策，实现期迁移；execute 写入与落库须同事务）
推荐 (a) `sync_consumed_tokens(nonce VARCHAR(64) UNIQUE NOT NULL, sync_job_id_ref BIGINT, consumed_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`；或 (b) `sync_jobs.baseline_nonce VARCHAR(64)` + UNIQUE 部分索引（仅 `status='succeeded'`）。推荐 (a)（与任务行解耦、便于按 TTL 清理）。
索引建议：`sync_jobs(game_id_ref, created_at DESC)`、`sync_job_items(sync_job_id_ref, section)`。

## 枚举与默认值
### SyncSection（9 种，无默认）→ 主要源表
| 值 | 源表 |
| --- | --- |
| game | games |
| markets | game_markets |
| legal | game_legal_links |
| channels | game_channels(+ login/iap/plugin configs) |
| packages | channel_packages(+ iap/plugin overrides) |
| products | products / channel_products |
| cashier | game_cashier_profiles / game_cashier_price_overrides |
| payments | payment_routes |
| config | game_config_snapshots（per-game 按 market 合并） |

未识别 section → `UNKNOWN_SECTION`(400)。
- **SyncOp**：add/update/delete（无默认；delete 默认不执行）。
- **SyncJobStatus**：previewed/succeeded/failed（无默认）。状态机（应用层强约束）：`previewed --成功--> succeeded`、`previewed --失败--> failed`；终态不可逆；失败任务重走 preview 生成新任务，不复用旧行。
- 默认值：`include_deletes=false`、`masked=false`、`applied=false`、`target_hash_after=''`、`operator_note=''`、`source_env=sandbox`、`target_env=production`、BaselineToken TTL=30min（可配）。

## 业务规则与流程

### 整体时序（与 01 §8 对齐）
```text
1. 运营在 sandbox 完成各 section 配置
2. snapshot 生成 config snapshot（per-game 按 market 合并，存 sandbox.game_config_snapshots）
3. POST /sync/preview：收集 sandbox/production 各 section 有效数据 → 排除隐藏/不兼容/无效
   → 按 section 产出 add/update/delete（密文 masked=true）→ 计算 source_hash/target_hash_before
   → 生成 baseline_token → 落 sync_jobs(previewed) + sync_job_items(applied=false)
4. 运营按 section 勾选 selected_sections（+ 可选 include_deletes）
5. POST /sync/execute（baseline_token + selected_sections + include_deletes）
6. 任一步失败：sync_jobs(failed)、事务回滚、不部分写入
7. 隐藏/不兼容/无效数据全程排除
```

### preview 规则
- **数据采集**：每 section 用 schema 限定名分别从 sandbox/production 读该 game 有效记录。「有效」：渠道实例类 `hidden=false` 且 market 兼容 且 `config_status=='valid'`；通用业务记录 `enabled=true`；config section 仅取 `status='published'` 最新快照（draft 不参与）。
- **对齐键 entity_key（业务唯一键，非自增 id）**：game→`game_id`；markets→`market_code`；legal→`scope_type+':'+scope_value`；channels→`market_code+'/'+channel_id`；packages→`market_code+'/'+channel_id+'/'+package_code`；products→`product_id`（channel_products 用 `package_key+'/'+product_id`）；cashier→profile 用 `game_id`、override 用 `country+region+currency+price_id`；payments→归一化选择器 `pay_way+package+channel+market+country+currency`（空=`*`）；config→`config_version`（或快照内容哈希）。
- **差异计算**：按对齐键集合比对——仅源有→add（`*`整行）；仅目标有→delete（`*`整行）；双方有→逐字段比对，值不同各产出一条 update。
- **脱敏**：字段属 `secret_fields_json` 或落 `*_ciphertext` → masked=true、值 `"masked"`。
- **哈希与基线**：计算 source_hash/target_hash_before，生成 baseline_token，落 previewed + items(applied=false)。

### baseline_token 生成
```text
payload = { gameId, sourceEnv:"sandbox", targetEnv:"production", sourceHash, targetHashBefore,
            previewedAt: now(), expiresAt: now()+TTL, nonce: random() }
sig   = HMAC_SHA256(serverSecret, canonicalJSON(payload))   // serverSecret 来自 infra/config
token = base64url(canonicalJSON(payload)) + "." + base64url(sig)
```
nonce 为全局唯一随机串，生成即参与签名、不可前端篡改。

> **两道独立闸门，缺一不可**：
> - **基线 hash 复核（D6）**：防「目标在预览后被他人改动」——目标已变 → `SYNC_BASELINE_MISMATCH`。
> - **nonce 去重（幂等）**：防「同一 token 被重复 execute」（重试/双击/重发）。即使 hash 相同（复核会过），nonce 已消费也不得再写，避免重复 upsert/审计。

### execute 复核流程（D6 核心，严格按序，任一步失败立即返回且不写库）
1. **section 合法性**：`selected_sections` 非空且全 ∈ 9 枚举，否则 `UNKNOWN_SECTION`(400)。
2. **token 解码与验签**：可解码、sig 验签过、`gameId/sourceEnv/targetEnv` 与上下文一致、未过期，否则 `VALIDATION_FAILED`(400)。
3. **nonce 去重（幂等闸门，先于基线复核）**：`baseline.nonce` 已被成功消费 → `SYNC_TOKEN_CONSUMED`(409)（响应可回带首次 `syncJobId`）。即便基线未变也不允许二次写入。
4. **基线复核**：实时重算 production 当前有效数据规范化 hash `target_hash_now`，与 `baseline.target_hash_before` 比较，不一致 → `SYNC_BASELINE_MISMATCH`(409)。
5. **依赖校验**：所选 section 前置依赖在 production 已存在且兼容，缺失 → `VALIDATION_FAILED`(400)，details 列出缺失 section+entityKey。
6. **有序 upsert（事务内）**：按依赖拓扑顺序对每个选中 section 执行 add/update；删除仅当 `include_deletes=true`；**同事务**落 nonce 去重记录（唯一冲突=并发重复，回滚并返回 `SYNC_TOKEN_CONSUMED`）。
7. **收尾**：计算 `target_hash_after`，更新 `sync_jobs(succeeded, executed_at, target_hash_after)`，写入明细置 `applied=true`，写 `audit_logs`。
8. **异常**：任一写入失败 → 回滚，`sync_jobs(failed)`，返回 `INTERNAL`(500) 或对应业务码；保证**不部分写入**（nonce 记录随事务回滚，允许修正后重试同一预览）。

### 依赖校验（section 拓扑）
```text
game     （根，无前置）
markets   依赖 game
legal     依赖 game
channels  依赖 game, markets          // 渠道实例归属某 market
packages  依赖 channels               // 渠道包挂在 game_channel 之下
products  依赖 game                   // channel_products 额外依赖 packages
cashier   依赖 game
payments  依赖 channels, packages, products, cashier   // 路由引用 channel/package/pay_way/provider/merchant
config    依赖 channels, packages, products, cashier, payments   // 快照是上述有效数据合并产物
```
依赖既可来自本次同时勾选并先行写入的 section，也可来自 production 既有数据。缺失 → 拒绝，details 指明「缺失依赖：channels/<market>/<channel_id>」。拓扑顺序同时作为写入次序。

### 有序 upsert（写入次序与幂等）
- 写入顺序：`game → markets → legal → channels → packages → products → cashier → payments → config`。
- 每 section 内：先 add 后 update，最后（若 include_deletes）delete。
- upsert 写 production schema（schema 限定名），以各表业务唯一键为冲突键（唯一键不含 env，D1）：存在则更新有变化字段，不存在则插入。
- **nonce 去重落库**：执行事务内向 `sync_consumed_tokens` 插一行（或 `sync_jobs.baseline_nonce` 唯一约束），唯一冲突 → 回滚并返回 `SYNC_TOKEN_CONSUMED`，与本次写入同事务提交/回滚（「写入成功」与「nonce 已消费」原子一致）。
- 密文写 production 时取密文/重新加密，绝不经明文中转（`00` §6.1）。

### 删除 opt-in 与失效数据排除
- `include_deletes=false`：preview 仍展示 delete 差异（让运营知情），但 execute 跳过所有 delete 明细（`applied=false`）。
- `include_deletes=true`：仅删「production 有、sandbox 已不存在或失效」的实体；删除前受依赖约束（仍被其它 production 数据引用则跳过并在响应 `skipped` 提示，不级联）。
- **失效数据排除（全程）**：隐藏 / 不兼容 / `config_status=invalid` / `enabled=false` 的记录不进 source 有效集（既不产生 add/update，也不作为「sandbox 存在」阻止 production 的 delete 判定——sandbox 侧失效 ≡ 不存在，可触发 production delete 候选）。

### 落库与审计
- preview：写 `sync_jobs(previewed)` + 全量 `sync_job_items(applied=false)`。
- execute 成功：更新 `sync_jobs(succeeded, target_hash_after, executed_at)`，写入明细 `applied=true`；写 `audit_logs(action=sync.execute, resource_type=game, resource_id=gameId, env=production, detail_json={selectedSections, includeDeletes, addCount, updateCount, deleteCount, sourceHash, targetHashBefore, targetHashAfter})`，detail 密文脱敏。
- execute 失败：`sync_jobs(failed)`，可选写 audit_logs（失败原因）。

## 后端 API（前缀 /api/admin，包络 00 §7，路由 internal/transport/http/sync）

### POST `/api/admin/games/{gameId}/sync/preview`（权限 `sync.preview`）
生成 sandbox→production 按 section 差异 + baseline_token。
请求 DTO：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| sections | string[] | 否 | 9 种全集 | 每项 ∈ SyncSection 否则 `UNKNOWN_SECTION`；为空/缺省=预览全部。 |
| includeDeletes | boolean | 否 | false | 仅影响预览删除项提示态；是否执行以 execute 为准。 |

校验顺序：游戏存在（否则 `NOT_FOUND`）→ sections 合法（否则 `UNKNOWN_SECTION`）。
成功响应（200）`data`：`{ gameId, sourceEnv, targetEnv, sourceHash, targetHashBefore, hasDiff, baselineToken, previewedAt, expiresAt, sections[] }`；每个 section：
```json
{
  "section": "channels",
  "summary": { "add": 1, "update": 1, "delete": 0 },
  "dependencies": ["game", "markets"],
  "changes": [
    { "op": "add", "entityType": "game_channel", "entityKey": "JP/google", "fieldName": "*",
      "sandboxValue": { "market": "JP", "channelId": "google", "enabled": true, "loginConfigStatus": "valid" },
      "productionValue": null, "masked": false },
    { "op": "update", "entityType": "game_channel_login_config", "entityKey": "JP/google",
      "fieldName": "clientSecret", "sandboxValue": "masked", "productionValue": "masked", "masked": true }
  ]
}
```

### POST `/api/admin/games/{gameId}/sync/execute`（权限 `sync.execute`，危险操作必写审计）
按 selectedSections 执行同步，执行前复核基线。
请求 DTO：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| selectedSections | string[] | 是 | — | 非空；每项 ∈ SyncSection 否则 `UNKNOWN_SECTION`。 |
| baselineToken | string | 是 | — | 本游戏 preview 返回的有效 token；验签+未过期+game/env 匹配，否则 `VALIDATION_FAILED`。 |
| includeDeletes | boolean | 否 | false | 为 true 才执行删除项。 |
| operatorNote | string | 否 | `''` | ≤255 字符。 |

校验顺序（§execute 复核流程）：sections 合法 → token 验签/过期/匹配 → nonce 去重 → 基线 hash 复核 → 依赖校验 → 有序 upsert。
错误码：`UNKNOWN_SECTION`(400) / `VALIDATION_FAILED`(400) / `SYNC_BASELINE_MISMATCH`(409) / `SYNC_TOKEN_CONSUMED`(409) / `NOT_FOUND`(404) / `FORBIDDEN`(403) / `INTERNAL`(500)。
成功响应（200）`data`：
```json
{
  "syncJobId": "9012", "gameId": "100001", "sourceEnv": "sandbox", "targetEnv": "production",
  "status": "succeeded", "selectedSections": ["channels", "products"], "includeDeletes": false,
  "sourceHash": "sha256-...", "targetHashBefore": "sha256-...", "targetHashAfter": "sha256-...",
  "appliedSummary": { "channels": { "add": 1, "update": 1, "delete": 0 }, "products": { "add": 0, "update": 1, "delete": 0 } },
  "skipped": { "deletes": [ { "section": "products", "entityKey": "gem_1", "reason": "include_deletes=false" } ], "unselectedSections": ["config"] },
  "executedAt": "2026-06-17T13:10:42Z"
}
```
错误响应：`SYNC_BASELINE_MISMATCH`(409) details 含 `{field:"targetHashBefore", expected, actual}`；`SYNC_TOKEN_CONSUMED`(409) details 含 `{field:"baselineToken", consumedSyncJobId, consumedAt}`；`UNKNOWN_SECTION`(400)/`VALIDATION_FAILED`(400, 依赖缺失 details 含 `{section, missingDependency, entityKey}`)。

### GET `/api/admin/games/{gameId}/sync-jobs`（权限 `sync.preview`）
列出某游戏同步任务历史（previewed/succeeded/failed）。Query：分页 `00` §7.3（page=1，pageSize=20/max100，sort=`-createdAt`）+ 可选 `status` 过滤。
响应列表包络 `data.{items[], page, pageSize, total}`，item：`{ syncJobId, gameId, sourceEnv, targetEnv, status, includeDeletes, operatorId, operatorNote, sourceHash, targetHashBefore, targetHashAfter, executedAt, createdAt }`。
可选增强：`GET /api/admin/sync-jobs/{syncJobId}/items`（密文仍 masked）；本期至少保证列表接口。

## 应用服务（SyncService）
位置：`internal/app/command/preview_section_sync.go` / `execute_section_sync.go`；仓储 `internal/infra/persistence/postgres` 实现 `SyncRepository`。

```go
type SyncRepository interface {
    CreateJob(ctx context.Context, job *SyncJob) (int64, error)
    UpdateJobResult(ctx context.Context, jobID int64, status string, targetHashAfter string, executedAt time.Time) error
    AddItems(ctx context.Context, jobID int64, items []SyncJobItem) error
    MarkItemsApplied(ctx context.Context, jobID int64, itemIDs []int64) error
    ListJobsByGame(ctx context.Context, gameID int64, page, pageSize int, status string) ([]SyncJob, int, error)
}
```
仓储窄：仅 `sync_jobs/sync_job_items` 的 CRUD + 分页，不放跨表 diff/upsert（`01` §4.2）。各 section 源/目标读取与 upsert 由对应聚合仓储承担，SyncService 负责编排。

```text
Preview(ctx, gameID, req) -> (SyncPreview, error)
  1. 校验 game 存在；req.sections 合法（UNKNOWN_SECTION）
  2. 每 section：loadEffective(schema=sandbox) / loadEffective(schema=production)   // 跨 schema 读
  3. diff(sandboxSet, productionSet) -> []DiffChange（脱敏 secret）
  4. canonicalize+hash -> sourceHash / targetHashBefore
  5. syncJobId = repo.CreateJob(previewed); repo.AddItems(applied=false)
  6. token = BuildBaselineToken(..., syncJobId)
  7. return SyncPreview{..., baselineToken: token}

Execute(ctx, gameID, req) -> (SyncJobResult, error)
  1. 校验 selectedSections 合法（UNKNOWN_SECTION）
  2. token = ParseAndVerify(req.baselineToken)            // 验签/过期/匹配，失败 VALIDATION_FAILED
  // 审定顺序：nonce 去重 → baseline hash 复核 → 依赖拓扑校验 → 单事务有序 upsert
  3. if nonceAlreadyConsumed(token.nonce) -> SYNC_TOKEN_CONSUMED(409)
  4. targetHashNow = hash(loadEffective(schema=production))
     if targetHashNow != token.targetHashBefore -> SYNC_BASELINE_MISMATCH
  5. checkDependencies(selectedSections, productionState)  // 缺失 -> VALIDATION_FAILED(details)
  6. BEGIN TX（单事务；原地更新 token.syncJobId 对应的 previewed 行）
       for section in topoOrder(selectedSections):
          applyAdds(section); applyUpdates(section)
          if req.includeDeletes: applyDeletes(section)
       consumeNonce(token.nonce, token.syncJobId)          // 唯一冲突 -> 回滚并返回 SYNC_TOKEN_CONSUMED
       targetHashAfter = hash(loadEffective(schema=production))
       repo.UpdateJobResult(token.syncJobId, succeeded, targetHashAfter, now)   // 不另建行
       repo.MarkItemsApplied(token.syncJobId)
       audit.Write(sync.execute, detail脱敏)
     COMMIT
  7. on any error: ROLLBACK; repo.UpdateJobResult(token.syncJobId, failed); return error
```
要点：步骤 6 全程单事务保证「不部分写入」；密文取密文/重新加密绝不落明文；审计 detail 记 before/after 计数与 hash、敏感值脱敏。

## 前端要点（游戏详情页 views/games/detail；API 客户端 api/syncSections.ts）
- **入口可见性（强约束）**：仅 `sandbox` 运行环境渲染 `Sync to Production`；production 环境绝不渲染，由 app store `environment` 驱动 + 权限 `sync.execute`。即便绕过前端，后端仍以 `source_env=sandbox` 兜底。
- **同步预览抽屉 SyncSectionDrawer**：点击 → `POST /sync/preview`（默认全 section）→ 按 section 分组、组顶 add/update/delete 计数徽标；差异行 add 绿 / update 黄 / delete 红，update 展示「sandbox→production」对照，密文 masked 显示 `••••••`/`masked` 不可展开明文；section 级复选框选 selected_sections（本期粒度为 section，不支持逐条）；include_deletes 单独开关默认关，关闭时 delete 行「仅提示，不执行」样式；确认携带 `baselineToken+selectedSections+includeDeletes+operatorNote` 调 execute。
- **结果反馈**：成功 toast + 刷新历史 Tab 展示 appliedSummary；`SYNC_BASELINE_MISMATCH` 弹窗「目标已变更，请重新预览」并提供「重新预览」按钮刷新 token；`UNKNOWN_SECTION`/`VALIDATION_FAILED` 行内/弹窗展示 details 指明缺失依赖。
- **同步历史 Tab SyncJobsTab**：列 任务 id/状态标签/selectedSections+include_deletes/操作者/备注/source-target hash(截断)/executedAt/createdAt；按 status 过滤、分页；失败行展开错误概要，成功行查看 appliedSummary。

## 与公共能力 / 下游
- API 包络/错误码（00 §7）：复用 `UNKNOWN_SECTION`/`SYNC_BASELINE_MISMATCH`/`SYNC_TOKEN_CONSUMED`/`VALIDATION_FAILED`/`NOT_FOUND`/`FORBIDDEN`/`INTERNAL`。
- 密文脱敏（00 §6.1）：preview masked、upsert 不经明文。审计（00 §8）：execute 写 `sync.execute`。鉴权（00 §7.5/D5）：preview→`sync.preview`、execute→`sync.execute`。
- env 模型（00 §2/D1）：同库跨 schema diff，仅 sync 域显式 `source_env/target_env`。
- 上游：各 section 源数据由 game~snapshot 模块在 sandbox 维护（snapshot 产出 `game_config_snapshots` 供 config section）；本模块只读有效数据搬运。

## 关键假设
- hash 采用 SHA-256，对「有效数据规范化序列化」求值（排除自增 id/时间戳，密文以密文参与），算法在 `domain/sync` 内固定确定。
- 差异勾选粒度本期固定 **section 级**；逐条勾选留待后续。
- preview/execute 共用同一 `sync_jobs` 行（preview 建 previewed、execute 原地更新到 succeeded/failed，不另建行；baseline_token 内含 syncJobId 定位）。
- baseline_token 必做「基线 hash 复核 + nonce 去重」双闸门，区分「目标已变更」(`SYNC_BASELINE_MISMATCH`) 与「重复执行」(`SYNC_TOKEN_CONSUMED`)。
- `include_deletes=true` 且删除项仍被引用 → 跳过并在 `skipped` 提示，不级联删除。
- config section 仅同步 `status=published` 快照；draft 不参与。
- baseline_token TTL 默认 30 分钟可配；过期即要求重新预览。
- `operator_id` 来自鉴权上下文当前管理员，与 `audit_logs.actor_id` 同源。
- 本期假设单 game 差异规模可控、全量返回；超大规模分页留待后续。
