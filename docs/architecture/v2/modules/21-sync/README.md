---
id: sync
code: "21"
title: Sandbox → Production 同步
status: target
code_paths:
  - services/admin-api/internal/domain/sync
  - services/admin-api/internal/transport/http/sync
depends_on: [snapshot, channel, account-auth, channel-login, feature-plugin, product, cashier-template, game-cashier, payment, game, common]
impacts: [testing]
children: []
---

# 21 · Sandbox → Production 同步

> 本文件是 v2 架构模块文档之一，默认遵循 `../../00-common.md`（统一 API 包络、错误码、密文脱敏、审计、红线）与 `../../01-structure.md`（分层、目录、§8 sandbox→production 数据流）。
> 若本文与 `00` 冲突，以 `00` 为准；本文只在 `00` 基础上**追加**同步域的私有约定。
> 本模块对应后端 `internal/domain/sync`、`internal/app/command`（preview/execute）、`internal/transport/http/sync`，以及前端游戏详情页的「同步预览抽屉 / 同步历史 Tab」。

---

## 1. 边界（Scope）

### 1.1 模块职责（做什么）

本模块负责把同一逻辑游戏在 `sandbox` 环境下已经配置好的数据，**经过预览、人工勾选、基线复核之后，按 `section` 有序写入到 `production` 环境**。它是整个后台「配置 → 上线」链路的最后一道闸门，承载以下职责：

1. **差异预览（preview）**：以 `game` 为单位，按 9 个固定 `section` 比对 `sandbox`（源）与 `production`（目标）的有效数据，产出每个 section 下的 `add / update / delete` 差异列表；对密文字段做 `masked=true` 脱敏；同时生成 `baseline_token`（含 `target_hash_before`），作为后续执行的基线凭证。
2. **执行同步（execute）**：接收前端显式勾选的 `selected_sections`、`baseline_token`、可选 `include_deletes`；在写入前**复核 production 当前 hash 是否仍等于 `target_hash_before`**，一致才按 section 有序 upsert，不一致直接拒绝（`SYNC_BASELINE_MISMATCH`）并要求重新预览。
3. **任务与明细落库**：把每次预览/执行落库到 `sync_jobs` / `sync_job_items`，并对执行写 `audit_logs`（`action=sync.execute`）。
4. **历史查询**：提供 `GET /sync-jobs` 列出某游戏的同步任务历史（含预览与执行）。

### 1.2 非职责（不做什么）

- **不生成业务数据**：各 section 的源数据（games / channels / products / cashier / payments / config 等）由各自模块（`game`～`snapshot`）在 `sandbox` 环境内维护；本模块只读它们的「有效数据」做 diff 与 upsert，不负责校验单条业务记录是否合规（合规性由源模块在 sandbox 阶段保证）。
- **不做 develop 相关同步**：本期同步链路固定 `source_env=sandbox → target_env=production`（D1）。`sync_jobs` 的 `source_env/target_env` CHECK 虽允许三值，但本模块只产出 `sandbox→production` 的任务；其它组合属未来扩展，本期不开放入口。
- **不做无 preview 的直写**：`execute` 必须携带 preview 返回的 `baseline_token`（D6 / `00` §9 红线）。
- **不在 production 视图暴露执行入口**：`production` 运行环境下，前端隐藏一切 `Sync to Production`（`00` §9、`01` §2）。
- **不参与运行时配置合并**：运行时 `GLOBAL` 与具体 market 的合并由「配置快照 + 运行时合并（`snapshot`）」负责；本模块只把合并后/有效的数据原样搬运到 production。

### 1.3 与其它模块的边界

| 关系方 | 边界说明 |
| --- | --- |
| 配置快照（`snapshot`） | `config` section 的 diff 基于 `snapshot` 产出的 `game_config_snapshots`（per-game，按 market 合并）；本模块读取快照的 `config_json` / `file_hash` 做对比与搬运。 |
| 渠道实例（`channel`） | `channels / packages` section 的源数据来自 `GameMarketChannel` 体系；**被隐藏 / 不兼容 / 无效的实例全程排除**（不进 preview、不进 execute）。 |
| 商品与 IAP（`product`）、收银台（`cashier-template` / `game-cashier`）、支付路由（`payment`） | 对应 `products / cashier / payments` section 的源表；本模块只读取「有效」记录（`enabled` 且 `config_status` 非异常态）。 |
| 审计（`audit`） | `execute` 必须写 `audit_logs`；查询入口见 `../22-audit/README.md`。 |
| 鉴权（`auth`） | `preview` 挂 `sync.read`（或 `game.read`），`execute` 挂 `sync.execute`；详见 §6。 |

### 1.4 锁定决策回链（本模块必须贯彻）

| 决策 | 在本模块的落地 |
| --- | --- |
| D1 同库按 env diff | 同一物理库内按 `env` 列比对；`source_env=sandbox`、`target_env=production`，不跨库。 |
| D6 基线一致性 | `execute` 必携带 `baseline_token`（含 `target_hash_before`）；执行前复核 `production` 当前 hash，不一致 → `SYNC_BASELINE_MISMATCH`。 |
| section 固定枚举 | `game / markets / legal / channels / packages / products / cashier / payments / config` 共 9 种；未识别 → `UNKNOWN_SECTION`。 |
| 显式 selected_sections | `execute` 必须显式声明；未选 section **不得隐式连带写入**。 |
| 依赖校验 | 所选 section 依赖的前置数据在 production 不存在/不兼容 → 拒绝并返回明确依赖缺失。 |
| 删除 opt-in | 删除默认不执行，需 `include_deletes=true` 显式勾选。 |
| 失效数据排除 | 隐藏 / 不兼容 / 无效数据全程排除。 |
| 密文脱敏 | preview 中密文字段 `masked=true`，绝不回明文。 |

---

## 2. 领域模型（Domain Model）

后端领域层位置：`internal/domain/sync`（纯领域，无 IO）；编排位于 `internal/app/command/preview_section_sync.go`、`internal/app/command/execute_section_sync.go`。

### 2.1 聚合根：Sync Aggregate

`sync.Aggregate` 拥有三类能力（与 `go_domain_api_draft.md` 「Sync 聚合：preview / execute / audit」一致）：

```text
Sync Aggregate
├── preview  差异预览：产出 SyncPreview（按 section 的 DiffSection 集合 + baseline）
├── execute  执行同步：消费 baseline + selected_sections，产出 SyncJob（落库 + 审计）
└── audit    审计：执行后写 audit_logs（before/after，密文脱敏）
```

聚合不变量（invariants）：

1. 一个 `SyncPreview` 永远对应一对 `(source_env=sandbox, target_env=production, game)`。
2. 一个 `SyncJob` 引用一次 preview 时刻的基线（`source_hash` / `target_hash_before`）；执行成功后才写入 `target_hash_after`。
3. `SyncJobItem` 不脱离 `SyncJob` 单独存在；`section` 必须 ∈ 9 种枚举。
4. 任何进入 preview/execute 的实体必须是「有效数据」：未被隐藏、与当前 market 兼容、`config_status != invalid`、`enabled = true`。

### 2.2 值对象：DiffSection / DiffChange

```text
SyncPreview
  gameId            string
  sourceEnv         "sandbox"
  targetEnv         "production"
  sourceHash        string          // sandbox 侧有效数据规范化哈希
  targetHashBefore  string          // production 侧当前有效数据规范化哈希
  hasDiff           bool
  baselineToken     BaselineToken   // 见 §2.3
  sections          []DiffSection

DiffSection
  section           SyncSection     // 9 种之一
  summary           DiffSummary     // { add, update, delete } 计数
  dependencies      []SectionDep    // 该 section 依赖的前置 section（见 §5.5）
  changes           []DiffChange

DiffChange
  op                SyncOp          // add / update / delete
  entityType        string          // 例：product / game_channel / payment_route / config_snapshot
  entityKey         string          // 业务唯一键（非自增 id），如 product_id / channel_id+market / price_id
  fieldName         string          // 字段级 diff 的字段名；整对象增删时用 "*" 表示整行
  sandboxValue      any             // 源值（masked=true 时为脱敏占位）
  productionValue   any             // 目标当前值（masked=true 时为脱敏占位）
  masked            bool            // 密文字段恒为 true
```

约定：

- **add**：`production` 不存在该 `entityKey`，`productionValue` 为空/`null`。
- **update**：双方都存在但字段值不同；逐字段产出 `DiffChange`（`fieldName` 为具体字段）。
- **delete**：`production` 存在而 `sandbox` 不存在（或 sandbox 侧已失效）的实体；`sandboxValue` 为空/`null`。
- **masked**：当 `fieldName` 属于该实体模板的 `secret_fields_json`（或落库为 `*_ciphertext`）时，`masked=true`，`sandboxValue` / `productionValue` 一律返回 `"masked"`，**绝不回明文**（`00` §6.1）。

### 2.3 值对象：BaselineToken（基线凭证）

`baseline_token` 是 preview 与 execute 之间的「乐观锁凭证」，本质是把预览时刻的目标状态指纹打包成一个**不可变、可校验、有时效**的 token。

```text
BaselineToken（解码后的逻辑结构）
  gameId            string
  sourceEnv         "sandbox"
  targetEnv         "production"
  sourceHash        string      // 与 SyncPreview.sourceHash 一致
  targetHashBefore  string      // 与 SyncPreview.targetHashBefore 一致；execute 据此复核
  previewedAt       string      // ISO-8601 UTC
  expiresAt         string      // previewedAt + TTL（默认 30 分钟，见 §11 假设）
  nonce             string      // 幂等/防重放键：全局唯一随机串，execute 成功后落库去重（见 §5.3/§5.4）
  sig               string      // 对以上字段的 HMAC-SHA256 签名（密钥来自 infra/crypto/config）
```

生成与校验见 §5.3 / §5.4。token 对外是一个不透明字符串（建议 `base64url(payloadJSON) + "." + base64url(sig)`），前端原样回传，**不得自行解析或篡改**。

### 2.4 哈希（hash）语义

- `source_hash` / `target_hash_before` / `target_hash_after` 均为对「某 env 下该 game 全部有效数据的规范化序列化结果」的 `SHA-256`（落库列长 128，存十六进制或带算法前缀如 `sha256-...`）。
- **规范化（canonicalization）**要求确定性：固定字段顺序、固定 key 排序、排除自增 `id` / `created_at` / `updated_at` 等非语义列、排除失效数据、密文字段以密文（或其哈希）参与而非明文。
- 同一份有效数据无论计算多少次都必须得到同一 hash（用于 §5.4 的基线复核与幂等判断）。

---

## 3. 数据模型（逐表逐字段）

> 两张表均来自 `services/admin-api/migrations/000001_init.up.sql`，本节按真实列逐字段说明，并补充本模块的语义与约束。两表均为**平台级任务记录表**（不带 `env` 列），其 env 维度通过 `source_env` / `target_env` 字段显式表达（`00` §2.2：sync 域是唯一允许显式声明双 env 的域）。

### 3.1 `sync_jobs`（同步任务）

每一次「预览生成」对应一行 `sync_jobs`（`status=previewed`）；该任务被执行成功/失败时原地更新 `status` 与 `target_hash_after` / `executed_at`。

| 列 | 类型 | 默认 | 约束 | 语义 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | 任务主键。 |
| `game_id_ref` | BIGINT | — | `REFERENCES games(id)` NOT NULL | 关联游戏（引用业务 `games` 行；注意 games 自身带 env，本字段指向逻辑 game）。 |
| `source_env` | VARCHAR(16) | — | NOT NULL，`CHECK IN (develop,sandbox,production)` | 同步源环境，本模块恒为 `sandbox`。 |
| `target_env` | VARCHAR(16) | — | NOT NULL，`CHECK IN (develop,sandbox,production)` | 同步目标环境，本模块恒为 `production`。 |
| `source_hash` | VARCHAR(128) | — | NOT NULL | 预览时刻 sandbox 侧有效数据的规范化哈希。 |
| `target_hash_before` | VARCHAR(128) | — | NOT NULL | 预览时刻 production 侧有效数据的规范化哈希；execute 据此复核基线（D6）。 |
| `target_hash_after` | VARCHAR(128) | `''` | NOT NULL DEFAULT '' | 执行成功后 production 侧新的规范化哈希；未执行/失败时为空串。 |
| `include_deletes` | BOOLEAN | `FALSE` | NOT NULL | 本次执行是否纳入删除项（opt-in）；预览阶段固定记录为请求值（默认 false）。 |
| `operator_id` | BIGINT | — | NOT NULL | 操作者管理员 id（与 `audit_logs.actor_id` 同源）。 |
| `operator_note` | VARCHAR(255) | `''` | NOT NULL DEFAULT '' | 操作备注（前端可选填写）。 |
| `status` | VARCHAR(32) | — | NOT NULL，`CHECK IN (previewed,succeeded,failed)` | 任务状态机，见 §4。 |
| `executed_at` | TIMESTAMPTZ | NULL | 可空 | 执行时间；仅 `succeeded/failed` 有值。 |
| `created_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | 创建（预览）时间。 |
| `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | 末次更新时间。 |

补充约束与约定：

- **本模块写入约束**：`source_env` 只写 `sandbox`，`target_env` 只写 `production`；若收到其它组合（未来扩展），按 §6 校验拒绝。
- `status` 不在 DB 层做流转校验，由应用层状态机（§4.3）保证仅 `previewed → succeeded` / `previewed → failed`。
- 一次 preview 生成一行 `previewed` 记录；execute 复用同一 `sync_job_id`（前端不持有该 id 时，execute 也可新建任务，见 §11 未决问题 Q3，本期默认「execute 新建任务并引用基线」，preview 记录用于追溯）。

### 3.2 `sync_job_items`（同步明细，逐字段差异）

每条 `DiffChange` 落一行（整对象增删时 `field_name='*'`），与 `sync_jobs` 一对多。

| 列 | 类型 | 默认 | 约束 | 语义 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK | 明细主键。 |
| `sync_job_id_ref` | BIGINT | — | `REFERENCES sync_jobs(id)` NOT NULL | 所属任务。 |
| `section` | VARCHAR(32) | — | NOT NULL | 所属 section；应用层强制 ∈ 9 种枚举（DB 未加 CHECK，由 §4 枚举 + §6 校验兜住）。 |
| `entity_type` | VARCHAR(64) | — | NOT NULL | 实体类型（如 `product` / `game_channel` / `channel_package` / `payment_route` / `config_snapshot`）。 |
| `entity_key` | VARCHAR(128) | — | NOT NULL | 实体业务唯一键（非自增 id），需在 source/target 间可对齐（见 §5.2 对齐键）。 |
| `op` | VARCHAR(16) | — | NOT NULL，`CHECK IN (add,update,delete)` | 差异操作类型。 |
| `field_name` | VARCHAR(64) | — | NOT NULL | 字段名；整行 add/delete 用 `'*'`。 |
| `sandbox_value_json` | JSONB | `'{}'` | NOT NULL | 源值（包裹为 `{"value": ...}`）；masked 时为脱敏占位。 |
| `production_value_json` | JSONB | `'{}'` | NOT NULL | 目标当前值；masked 时为脱敏占位。 |
| `masked` | BOOLEAN | `FALSE` | NOT NULL | 是否密文脱敏字段。 |
| `applied` | BOOLEAN | `FALSE` | NOT NULL | 该明细是否在本次 execute 中实际落库（被选中且写入成功为 true；未选/被跳过/删除未勾选为 false）。 |
| `created_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | 创建时间。 |
| `updated_at` | TIMESTAMPTZ | `NOW()` | NOT NULL | 末次更新时间。 |

补充约定：

- **预览阶段**即写入全部 `DiffChange`（`applied=false`），便于 preview 结果可追溯/复现。
- **执行阶段**对被选中 section 内、且满足删除 opt-in 规则的明细，写库成功后置 `applied=true`；未选中 section 或未勾选删除项的明细保持 `applied=false`。
- `sandbox_value_json` / `production_value_json` 统一用 `{"value": <原值>}` 包裹，避免 JSONB 顶层非对象的问题；`masked=true` 时值为 `{"value":"masked"}`。

### 3.3 status CHECK 与索引建议

- `sync_jobs.status` 已有 `CHECK (status IN ('previewed','succeeded','failed'))`。
- `sync_job_items.op` 已有 `CHECK (op IN ('add','update','delete'))`。
- 建议（实现期补充迁移，非本文落实）：`sync_jobs(game_id_ref, created_at DESC)` 索引支撑历史列表分页；`sync_job_items(sync_job_id_ref, section)` 索引支撑按 section 渲染。
- **nonce 去重存储（D 决策，实现期迁移）**：二选一——(a) 新增 `sync_consumed_tokens(nonce VARCHAR(64) UNIQUE NOT NULL, sync_job_id_ref BIGINT, consumed_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`；或 (b) 给 `sync_jobs` 增 `baseline_nonce VARCHAR(64)` 并加 `UNIQUE` 部分索引（仅 `status='succeeded'`）。推荐 (a)，与任务行解耦、便于按 TTL 清理。execute 写入与 nonce 落库须同事务（§5.6）。

---

## 4. 枚举与默认值清单

> 与 `00` §3.1 全局枚举保持一致，本节为同步域的权威子集。

### 4.1 SyncSection（9 种，无默认值）

| 值 | 含义 | 主要源表（sandbox→production 搬运对象） |
| --- | --- | --- |
| `game` | 游戏基础信息 | `games`（name/alias/icon_url/default_market_code/status…） |
| `markets` | 已启用市场集合 | `game_markets` |
| `legal` | 法务链接 | `game_legal_links` |
| `channels` | 渠道实例（按 market 拆分） | `game_channels`（+ `game_channel_login_configs` / `game_channel_iap_configs`） |
| `packages` | 渠道包 | `channel_packages`（+ `channel_package_iap_overrides`） |
| `products` | 商品与渠道商品映射 | `products` / `channel_products` |
| `cashier` | 游戏级收银台绑定与价格覆盖 | `game_cashier_profiles` / `game_cashier_price_overrides` |
| `payments` | 支付路由 | `payment_routes` |
| `config` | 配置快照 | `game_config_snapshots`（per-game，按 market 合并） |

未识别的 section 一律拒绝 → 错误码 `UNKNOWN_SECTION`（HTTP 400）。

### 4.2 SyncOp（差异操作，无默认值）

| 值 | 含义 |
| --- | --- |
| `add` | 目标新增（production 无此实体）。 |
| `update` | 目标更新（字段级差异）。 |
| `delete` | 目标删除（production 有、sandbox 无/失效）；**默认不执行**。 |

### 4.3 SyncJobStatus（任务状态，无默认值）

状态机（应用层强约束）：

```text
previewed --execute 成功--> succeeded
previewed --execute 失败--> failed
```

规则：

- 不允许 `succeeded → *` 或 `failed → *` 的再次流转（一次任务终态不可逆）。
- 失败任务可重新走 preview 生成新任务，不复用旧任务行。

### 4.4 默认值清单

| 项 | 默认值 | 说明 |
| --- | --- | --- |
| `include_deletes` | `false` | 删除默认不执行（opt-in）。 |
| `masked`（item） | `false` | 仅密文字段为 `true`。 |
| `applied`（item） | `false` | 仅被选中并写入成功的明细为 `true`。 |
| `target_hash_after` | `''` | 仅 `succeeded` 时回填。 |
| `operator_note` | `''` | 备注可空。 |
| `source_env` | `sandbox` | 本模块固定。 |
| `target_env` | `production` | 本模块固定。 |
| BaselineToken TTL | `30min` | 见 §11 假设，可配置。 |

---

## 5. 业务规则与流程

### 5.1 整体时序（与 `01` §8 对齐）

```text
1. 运营在 sandbox 完成各 section 配置（env=sandbox）
2. `snapshot` 生成 config snapshot（per-game，按 market 合并，env=sandbox）
3. POST /sync/preview：
   - 收集 sandbox 与 production 同一 game 各 section 的「有效数据」
   - 排除隐藏/不兼容/无效数据
   - 按 section 产出 add/update/delete 差异，密文 masked=true
   - 计算 source_hash / target_hash_before
   - 生成 baseline_token（含 target_hash_before、过期时间、签名）
   - 落 sync_jobs(status=previewed) + sync_job_items(applied=false)
4. 运营在抽屉中按 section 勾选 selected_sections（+ 可选 include_deletes）
5. POST /sync/execute（携带 baseline_token + selected_sections + include_deletes）：
   - 校验 selected_sections 全部 ∈ 9 枚举，否则 UNKNOWN_SECTION
   - 校验/解码 baseline_token（签名、未过期、game/env 匹配）
   - 复核 production 当前 hash == baseline.target_hash_before，不一致 → SYNC_BASELINE_MISMATCH
   - 依赖校验：所选 section 的前置数据在 production 存在且兼容
   - 在事务内按 section 依赖拓扑顺序有序 upsert（删除项仅在 include_deletes=true 时执行）
   - 计算 target_hash_after，更新 sync_jobs(status=succeeded)，置相关 items.applied=true
   - 写 audit_logs(action=sync.execute, before/after 脱敏)
6. 任一步失败：sync_jobs(status=failed)，事务回滚，不部分写入
7. 隐藏/不兼容/无效数据全程排除
```

### 5.2 预览规则（preview）

**数据采集**：对每个 section，分别加载 `env=sandbox`（源）与 `env=production`（目标）下该 game 的有效记录。「有效」判定：

- 渠道实例（channels/packages 下游）：`hidden=false` 且 与当前 market 兼容 且 `config_status != 'invalid'`。
- 通用业务记录：`enabled=true`（如 `game_markets.enabled` / `game_channels.enabled` / `products.enabled` / `payment_routes.enabled`）。
- config section：仅取 `status='published'` 的最新快照参与（草稿快照不参与同步，对齐 spec「draft 不参与同步执行」）。

**对齐键（entity_key）**：每个 section 用业务唯一键对齐，而非自增 id（自增 id 在不同 env 不同）：

| section | 对齐键示例 |
| --- | --- |
| game | `game_id` |
| markets | `market_code` |
| legal | `scope_type + ':' + scope_value` |
| channels | `market_code + '/' + channel_id` |
| packages | `market_code + '/' + channel_id + '/' + package_code` |
| products | `product_id`（channel_products 用 `package_key + '/' + product_id`） |
| cashier | profile 用 `game_id`；override 用 `country+region+currency+price_id` |
| payments | 归一化选择器 `pay_way+package+channel+market+country+currency`（空=`*`） |
| config | `config_version`（或快照内容哈希） |

**差异计算**：按对齐键做集合比对：

- 仅源有 → `add`（`field_name='*'`，整行）。
- 仅目标有 → `delete`（`field_name='*'`，整行）。
- 双方都有 → 逐字段比对，值不同的字段各产出一条 `update`。

**脱敏**：字段属 `secret_fields_json` 或落 `*_ciphertext` → `masked=true`，值置 `"masked"`。

**哈希与基线**：计算 `source_hash`、`target_hash_before`，生成 `baseline_token`（§5.3），落 `sync_jobs(previewed)` + `sync_job_items(applied=false)`。

### 5.3 baseline_token 生成

```text
payload = {
  gameId, sourceEnv:"sandbox", targetEnv:"production",
  sourceHash, targetHashBefore,
  previewedAt: now(), expiresAt: now()+TTL, nonce: random()
}
sig   = HMAC_SHA256(serverSecret, canonicalJSON(payload))
token = base64url(canonicalJSON(payload)) + "." + base64url(sig)
```

- `serverSecret` 来自 `infra/config`（与 `infra/crypto` 同源密钥域）。
- token 时效默认 30 分钟（`expiresAt`）；过期 → execute 拒绝并要求重新预览。
- `nonce` 为**全局唯一随机串**（建议 128-bit，如 UUIDv4 / 随机字节 base64url），是本 token 的幂等键与防重放键。生成时即写入 baseline_token 并参与签名，**不可被前端篡改**。

> **nonce 去重 vs 基线复核（两道独立闸门，缺一不可）**
> - **基线 hash 复核（D6）**：防「目标环境在预览后已被他人改动」——目标已变更则 `SYNC_BASELINE_MISMATCH`。
> - **nonce 去重（幂等）**：防「同一份预览的 token 被重复 execute」（重试、双击、网络重发）。即使基线未变（hash 相同、复核会通过），若该 nonce 已被成功消费过，也**不得再次写入**，避免重复 upsert 产生重复审计与「这次到底改没改」的争议。

### 5.4 execute 复核流程（D6 核心）

执行入口收到请求后，**严格按序**执行，任一步失败立即返回对应错误码且不写库：

1. **section 合法性**：`selected_sections` 非空且全部 ∈ 9 枚举；存在非法值 → `UNKNOWN_SECTION`（400）。
2. **token 解码与验签**：`baseline_token` 可解码、`sig` 验签通过、`gameId/sourceEnv/targetEnv` 与 URL/上下文一致、未过期；任一不满足 → `VALIDATION_FAILED`（400，message 说明 token 非法/过期，提示重新预览）。
3. **nonce 去重（幂等闸门）**：查 `baseline.nonce` 是否已被成功消费（见 §5.6 落库）：
   - 已消费 → `SYNC_TOKEN_CONSUMED`（409），message：「该同步已执行（重复请求），请勿重复提交；如需再次同步请重新预览」。响应可回带首次执行的 `syncJobId` 供前端定位。
   - 这一步**先于基线复核**：即便基线未变（hash 相同），同一 token 也不允许第二次写入，杜绝重复 upsert/重复审计。
4. **基线复核**：实时重新计算 `production` 当前有效数据的规范化 hash `target_hash_now`；与 `baseline.target_hash_before` 比较：
   - 不一致 → `SYNC_BASELINE_MISMATCH`（409），message：「目标环境已发生变更，请重新预览后再执行」。
5. **依赖校验**（§5.5）：所选 section 的前置依赖在 production 已存在且兼容；缺失 → `VALIDATION_FAILED`（400），`details` 列出缺失依赖（含缺失 section 与具体 entityKey）。
6. **有序 upsert**（事务内，§5.6）：按依赖拓扑顺序对每个选中 section 执行 add/update；删除项仅当 `include_deletes=true` 时执行；**在同一事务内**落 `nonce` 去重记录（唯一约束冲突即视为并发重复，回滚并返回 `SYNC_TOKEN_CONSUMED`）。
7. **收尾**：计算 `target_hash_after`，更新 `sync_jobs(status=succeeded, executed_at, target_hash_after)`，置已写入明细 `applied=true`；写 `audit_logs`。
8. **异常**：任一写入失败 → 回滚事务，`sync_jobs(status=failed)`，返回 `INTERNAL`（500）或对应业务错误码；保证**不部分写入**（nonce 去重记录随事务回滚，允许修正后重试同一预览）。

### 5.5 依赖校验（section 依赖关系）

section 之间存在前置依赖，所选 section 的依赖必须在 **production 目标环境**中已经存在且兼容（依赖既可来自本次同时勾选并先行写入的 section，也可来自 production 既有数据）：

```text
game        （根，无前置）
markets      依赖 game
legal        依赖 game
channels     依赖 game, markets        // 渠道实例归属某 market，market 必须先存在
packages     依赖 channels             // 渠道包挂在 game_channel 之下
products     依赖 game                 // 商品归属 game；channel_products 额外依赖 packages
cashier      依赖 game                 // 收银台绑定/价格覆盖归属 game
payments     依赖 channels, packages, products, cashier  // 路由引用 channel/package/pay_way/provider/merchant
config       依赖 channels, packages, products, cashier, payments  // 快照是上述有效数据的合并产物
```

校验规则：

- 若勾选 `packages` 但 production 既无对应 `channels` 实例、本次又未勾选 `channels`（或勾选了但该渠道在 sandbox 已失效不会写入）→ 拒绝，`details` 指明「缺失依赖：channels/<market>/<channel_id>」。
- 依赖「不兼容」示例：`payments` 路由引用的 `channel` 与目标 market 规则不兼容 → 拒绝。
- 拓扑顺序同时用于 §5.6 的写入次序。

### 5.6 有序 upsert（写入次序与幂等）

- 按上面拓扑顺序写入：`game → markets → legal → channels → packages → products → cashier → payments → config`。
- 每个 section 内：先 `add` 后 `update`，最后（若 `include_deletes`）`delete`。
- upsert 以 `env=production` + 业务唯一键为冲突键（沿用各表 `UNIQUE(env, …)`，D1）：存在则更新有变化字段，不存在则插入。
- **nonce 去重落库**：在执行事务内向 `sync_consumed_tokens(nonce UNIQUE, sync_job_id_ref, consumed_at)` 插入一行（或在 `sync_jobs.baseline_nonce` 上加唯一约束）。唯一冲突即并发/重复执行 → 回滚并返回 `SYNC_TOKEN_CONSUMED`。该记录与本次写入同事务提交/回滚，保证「写入成功」与「nonce 已消费」原子一致。
- **幂等性（双闸门）**：基线复核（D6）防目标已变更，nonce 去重防同 token 重放。两者都满足才执行写入；因 upsert 以业务键为准，结合 nonce 去重，重复提交同一 token 不会产生重复 upsert 或重复 `sync.execute` 审计。
- 密文字段写入 production 时，从 source 取**密文**（或重新加密），绝不经过明文中转、不落明文（`00` §6.1）。

### 5.7 删除 opt-in 与失效数据排除

- **删除默认不执行**：`include_deletes` 缺省 false；为 false 时，preview 仍展示 `delete` 差异（让运营知情），但 execute 跳过所有 `delete` 明细（`applied=false`）。
- `include_deletes=true` 时，仅删除「production 有、sandbox 侧已不存在或已失效」的实体；删除前同样受依赖约束（不允许删除仍被其它 production 数据引用的前置实体，否则该删除项跳过并在响应中提示，见 §11 Q5）。
- **失效数据排除（全程）**：被隐藏 / 不兼容 / `config_status=invalid` / `enabled=false` 的记录：不进 source 有效集（既不产生 add/update，也不作为「sandbox 存在」阻止 production 的 delete 判定——即 sandbox 侧失效等价于「不存在」，可能触发 production 的 delete 候选项）。

### 5.8 落库与审计

- preview：写 `sync_jobs(status=previewed)` + 全量 `sync_job_items(applied=false)`。
- execute 成功：更新 `sync_jobs(status=succeeded, target_hash_after, executed_at)`；被写入明细 `applied=true`；写 `audit_logs(action=sync.execute, resource_type=game, resource_id=gameId, env=production, detail_json={selectedSections, includeDeletes, addCount, updateCount, deleteCount, sourceHash, targetHashBefore, targetHashAfter})`，detail 中任何密文一律脱敏。
- execute 失败：`sync_jobs(status=failed)`，可选写 `audit_logs`（记录失败原因）。

---

## 6. 后端 API（逐接口完整 DTO + 校验 + 示例）

> 统一遵循 `00` §7：前缀 `/api/admin`、Bearer 鉴权、`application/json; charset=utf-8`、JSON 字段 camelCase、时间 ISO-8601 UTC、统一响应包络 `{ "data": ... }` / `{ "error": {...} }`。
> 路由装配位于 `internal/transport/http/sync`（见 `01` §4）。

### 6.1 POST `/api/admin/games/{gameId}/sync/preview`

- **作用**：生成 sandbox→production 的按 section 差异 + baseline_token。
- **权限码**：`sync.read`（最低 `game.read`）。
- **路径参数**：`gameId`（游戏业务 id）。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `sections` | string[] | 否 | 9 种全集 | 每项 ∈ SyncSection，否则 `UNKNOWN_SECTION`；为空/缺省表示预览全部 section。 |
| `includeDeletes` | boolean | 否 | `false` | 仅影响预览中是否高亮删除项的提示态；删除是否执行以 execute 为准。 |

- **校验顺序**：游戏存在（否则 `NOT_FOUND`）→ `sections` 合法（否则 `UNKNOWN_SECTION`）→ 运行环境允许预览（production 运行环境下不应有此入口，但服务端仍以 `source_env=sandbox` 逻辑处理）。
- **成功响应（200）**：`data` 内含 `baselineToken` 与按 section 的差异。

**请求示例**：

```json
POST /api/admin/games/100001/sync/preview
Authorization: Bearer <accessToken>
Content-Type: application/json

{
  "sections": ["products", "channels", "config"],
  "includeDeletes": false
}
```

**完整响应示例（200）**：

```json
{
  "data": {
    "gameId": "100001",
    "sourceEnv": "sandbox",
    "targetEnv": "production",
    "sourceHash": "sha256-2f1d0c9a7b...e21",
    "targetHashBefore": "sha256-9a83bb1f02...c40",
    "hasDiff": true,
    "baselineToken": "eyJnYW1lSWQiOiIxMDAwMDEiLCJzb3VyY2VFbnYiOiJzYW5kYm94Iiwi...==.MEUCIQ...sig",
    "previewedAt": "2026-06-17T13:05:00Z",
    "expiresAt": "2026-06-17T13:35:00Z",
    "sections": [
      {
        "section": "channels",
        "summary": { "add": 1, "update": 1, "delete": 0 },
        "dependencies": ["game", "markets"],
        "changes": [
          {
            "op": "add",
            "entityType": "game_channel",
            "entityKey": "JP/google",
            "fieldName": "*",
            "sandboxValue": {
              "market": "JP", "channelId": "google",
              "enabled": true, "loginConfigStatus": "valid"
            },
            "productionValue": null,
            "masked": false
          },
          {
            "op": "update",
            "entityType": "game_channel_login_config",
            "entityKey": "JP/google",
            "fieldName": "clientSecret",
            "sandboxValue": "masked",
            "productionValue": "masked",
            "masked": true
          }
        ]
      },
      {
        "section": "products",
        "summary": { "add": 0, "update": 1, "delete": 1 },
        "dependencies": ["game"],
        "changes": [
          {
            "op": "update",
            "entityType": "product",
            "entityKey": "gem_60",
            "fieldName": "priceId",
            "sandboxValue": "price_499",
            "productionValue": "price_599",
            "masked": false
          },
          {
            "op": "delete",
            "entityType": "product",
            "entityKey": "gem_1",
            "fieldName": "*",
            "sandboxValue": null,
            "productionValue": { "productId": "gem_1", "enabled": true },
            "masked": false
          }
        ]
      },
      {
        "section": "config",
        "summary": { "add": 0, "update": 1, "delete": 0 },
        "dependencies": ["channels", "packages", "products", "cashier", "payments"],
        "changes": [
          {
            "op": "update",
            "entityType": "config_snapshot",
            "entityKey": "v2026.06.17",
            "fieldName": "fileHash",
            "sandboxValue": "sha256-aaa...",
            "productionValue": "sha256-bbb...",
            "masked": false
          }
        ]
      }
    ]
  }
}
```

### 6.2 POST `/api/admin/games/{gameId}/sync/execute`

- **作用**：按 `selected_sections` 执行同步，执行前复核基线。
- **权限码**：`sync.execute`（危险操作，必须挂权限并写审计）。
- **路径参数**：`gameId`。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `selectedSections` | string[] | 是 | — | 非空；每项 ∈ SyncSection，否则 `UNKNOWN_SECTION`。 |
| `baselineToken` | string | 是 | — | 必须为本游戏 preview 返回的有效 token；验签 + 未过期 + game/env 匹配，否则 `VALIDATION_FAILED`。 |
| `includeDeletes` | boolean | 否 | `false` | 为 true 才执行删除项。 |
| `operatorNote` | string | 否 | `''` | ≤255 字符。 |

- **校验顺序**（见 §5.4）：sections 合法 → token 验签/过期/匹配 → 基线 hash 复核 → 依赖校验 → 有序 upsert。
- **错误码**：`UNKNOWN_SECTION`(400) / `VALIDATION_FAILED`(400) / `SYNC_BASELINE_MISMATCH`(409) / `SYNC_TOKEN_CONSUMED`(409) / `NOT_FOUND`(404) / `FORBIDDEN`(403) / `INTERNAL`(500)。

**请求示例（完整）**：

```json
POST /api/admin/games/100001/sync/execute
Authorization: Bearer <accessToken>
Content-Type: application/json

{
  "selectedSections": ["channels", "products"],
  "baselineToken": "eyJnYW1lSWQiOiIxMDAwMDEiLCJzb3VyY2VFbnYiOiJzYW5kYm94Iiwi...==.MEUCIQ...sig",
  "includeDeletes": false,
  "operatorNote": "JP Google 渠道上线 + 商品价格修正"
}
```

**成功响应示例（200）**：

```json
{
  "data": {
    "syncJobId": "9012",
    "gameId": "100001",
    "sourceEnv": "sandbox",
    "targetEnv": "production",
    "status": "succeeded",
    "selectedSections": ["channels", "products"],
    "includeDeletes": false,
    "sourceHash": "sha256-2f1d0c9a7b...e21",
    "targetHashBefore": "sha256-9a83bb1f02...c40",
    "targetHashAfter": "sha256-77c1e0aa45...f08",
    "appliedSummary": {
      "channels": { "add": 1, "update": 1, "delete": 0 },
      "products": { "add": 0, "update": 1, "delete": 0 }
    },
    "skipped": {
      "deletes": [
        { "section": "products", "entityKey": "gem_1", "reason": "include_deletes=false" }
      ],
      "unselectedSections": ["config"]
    },
    "executedAt": "2026-06-17T13:10:42Z"
  }
}
```

**基线不一致响应示例（409）**：

```json
{
  "error": {
    "code": "SYNC_BASELINE_MISMATCH",
    "message": "目标环境已发生变更，请重新预览后再执行",
    "details": [
      { "field": "targetHashBefore", "expected": "sha256-9a83bb1f02...c40", "actual": "sha256-be77aa01...d19" }
    ]
  }
}
```

**重复执行响应示例（409，nonce 已消费）**：

```json
{
  "error": {
    "code": "SYNC_TOKEN_CONSUMED",
    "message": "该同步已执行（重复请求），请勿重复提交；如需再次同步请重新预览",
    "details": [
      { "field": "baselineToken", "consumedSyncJobId": "9012", "consumedAt": "2026-06-17T13:10:42Z" }
    ]
  }
}
```

**未识别 section 响应示例（400）**：

```json
{
  "error": {
    "code": "UNKNOWN_SECTION",
    "message": "unknown section: marketing",
    "details": [ { "field": "selectedSections", "value": "marketing" } ]
  }
}
```

**依赖缺失响应示例（400）**：

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "selected sections have missing dependencies in production",
    "details": [
      { "section": "packages", "missingDependency": "channels", "entityKey": "JP/google" }
    ]
  }
}
```

### 6.3 GET `/api/admin/games/{gameId}/sync-jobs`

- **作用**：列出某游戏的同步任务历史（含 previewed/succeeded/failed）。
- **权限码**：`sync.read`（或 `audit.read`）。
- **Query**：分页遵循 `00` §7.3（`page` 默认 1，`pageSize` 默认 20 / 最大 100，`sort` 默认 `-createdAt`）；可选 `status` 过滤（`previewed|succeeded|failed`）。
- **成功响应示例（200，列表包络）**：

```json
{
  "data": {
    "items": [
      {
        "syncJobId": "9012",
        "gameId": "100001",
        "sourceEnv": "sandbox",
        "targetEnv": "production",
        "status": "succeeded",
        "includeDeletes": false,
        "operatorId": "7",
        "operatorNote": "JP Google 渠道上线 + 商品价格修正",
        "sourceHash": "sha256-2f1d0c9a7b...e21",
        "targetHashBefore": "sha256-9a83bb1f02...c40",
        "targetHashAfter": "sha256-77c1e0aa45...f08",
        "executedAt": "2026-06-17T13:10:42Z",
        "createdAt": "2026-06-17T13:05:00Z"
      },
      {
        "syncJobId": "9008",
        "gameId": "100001",
        "status": "failed",
        "targetHashAfter": "",
        "executedAt": "2026-06-17T11:02:10Z",
        "createdAt": "2026-06-17T10:58:00Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 12
  }
}
```

- 可选明细接口（实现期可加）：`GET /api/admin/sync-jobs/{syncJobId}/items` 返回该任务的 `sync_job_items`（密文仍 masked）。本期至少保证列表接口可用。

### 6.4 权限码与审计汇总

| 接口 | 权限码 | 是否写审计 |
| --- | --- | --- |
| `POST .../sync/preview` | `sync.read` | 否（预览只读，可选记录轻量日志） |
| `POST .../sync/execute` | `sync.execute` | **是**（`action=sync.execute`） |
| `GET .../sync-jobs` | `sync.read` | 否 |

---

## 7. 应用服务（SyncService）

位置：`internal/app/command/preview_section_sync.go`、`internal/app/command/execute_section_sync.go`；仓储 `internal/infra/persistence/postgres` 实现 `SyncRepository`。

### 7.1 SyncRepository（窄仓储，对齐 go_domain_api_draft）

```go
type SyncRepository interface {
    CreateJob(ctx context.Context, job *SyncJob) (int64, error)
    UpdateJobResult(ctx context.Context, jobID int64, status string, targetHashAfter string, executedAt time.Time) error
    AddItems(ctx context.Context, jobID int64, items []SyncJobItem) error
    MarkItemsApplied(ctx context.Context, jobID int64, itemIDs []int64) error
    ListJobsByGame(ctx context.Context, gameID int64, page, pageSize int, status string) ([]SyncJob, int, error)
}
```

仓储保持窄：只做 `sync_jobs/sync_job_items` 的 CRUD 与分页查询，不放跨表 diff/upsert（`01` §4.2）。各 section 的源/目标数据读取与 upsert 由对应聚合的仓储承担，SyncService 负责编排。

### 7.2 SyncService.Preview（编排）

```text
Preview(ctx, gameID, req) -> (SyncPreview, error)
  1. 校验 game 存在；校验 req.sections 合法（UNKNOWN_SECTION）
  2. 对每个 section：loadEffective(env=sandbox) / loadEffective(env=production)
  3. diff(sandboxSet, productionSet) -> []DiffChange（脱敏 secret）
  4. canonicalize+hash -> sourceHash / targetHashBefore
  5. token = BuildBaselineToken(...)
  6. repo.CreateJob(status=previewed); repo.AddItems(applied=false)
  7. return SyncPreview{..., baselineToken: token}
```

### 7.3 SyncService.Execute（编排，事务）

```text
Execute(ctx, gameID, req) -> (SyncJobResult, error)
  1. 校验 selectedSections 合法（UNKNOWN_SECTION）
  2. token = ParseAndVerify(req.baselineToken)  // 验签/过期/匹配，失败 VALIDATION_FAILED
  3. targetHashNow = hash(loadEffective(env=production))
     if targetHashNow != token.targetHashBefore -> SYNC_BASELINE_MISMATCH
  4. checkDependencies(selectedSections, productionState)  // 缺失 -> VALIDATION_FAILED(details)
  5. BEGIN TX
     for section in topoOrder(selectedSections):
        applyAdds(section); applyUpdates(section)
        if req.includeDeletes: applyDeletes(section)
     targetHashAfter = hash(loadEffective(env=production))
     repo.UpdateJobResult(succeeded, targetHashAfter, now)
     repo.MarkItemsApplied(appliedItemIDs)
     audit.Write(sync.execute, detail脱敏)
     COMMIT
  6. on any error: ROLLBACK; repo.UpdateJobResult(failed); return error
```

要点：

- **事务边界**：步骤 5 全程单事务，保证「不部分写入」（红线）。
- **密文**：upsert 写 production 时取密文/重新加密，绝不落明文。
- **审计**：detail 记录 before/after 计数与 hash，敏感值脱敏。

---

## 8. 前端（admin-web）

> 位置：游戏详情页（`views/games/detail`），与 `01` §5 通用 UI 契约、`frontend_agent_execution.md` Phase 9 后半一致。API 客户端 `api/syncSections.ts`。

### 8.1 入口可见性（强约束）

- **仅 `sandbox` 运行环境**展示 `Sync to Production` 入口；`production` 运行环境下**绝不渲染**该入口（`00` §9、`01` §2、`frontend_agent_execution.md` Phase 9）。可见性由 `app` store 的 `environment` 驱动，并辅以权限 `sync.execute`。
- 即便误绕过前端，后端仍以 `source_env=sandbox` 逻辑兜底，production 不可作为同步源。

### 8.2 同步预览抽屉（SyncSectionDrawer）

交互流程：

1. 点击 `Sync to Production` → 调 `POST /sync/preview`（默认全 section）。
2. **按 section 分组**展示差异，每组顶部显示 `add/update/delete` 计数徽标。
3. 每条差异行：
   - `add` 绿色高亮、`update` 黄色高亮、`delete` 红色高亮（语义色与列表-差异抽屉统一组件一致）。
   - 密文字段：`masked=true` 时显示 `••••••` / `masked`，**不可展开明文**。
   - `update` 行展示「sandbox 值 → production 值」对照。
4. **勾选 selected_sections**：section 级复选框；未勾选的 section 在 execute 时不传、不写入。section 内单条差异本期不支持逐条勾选（粒度为 section，见 §11 Q2）。
5. **可选 include_deletes**：单独开关，默认关闭；关闭时 `delete` 行以「仅提示，不执行」样式呈现。
6. **确认执行**：携带 `baselineToken + selectedSections + includeDeletes + operatorNote` 调 `POST /sync/execute`。
7. **结果反馈**：
   - 成功：toast + 刷新历史 Tab，展示 appliedSummary。
   - `SYNC_BASELINE_MISMATCH`：弹窗提示「目标已变更，请重新预览」，并提供「重新预览」按钮（重新拉取 preview，刷新 token）。
   - `UNKNOWN_SECTION` / `VALIDATION_FAILED`（依赖缺失）：行内/弹窗展示 `details`，指明缺失依赖。

### 8.3 同步历史 Tab（SyncJobsTab）

- 列：任务 id、状态（previewed/succeeded/failed 状态标签）、selectedSections/include_deletes、操作者、备注、source/target hash（截断展示）、executedAt、createdAt。
- 支持按 status 过滤、分页（遵循 `00` §7.3）。
- 失败任务行可展开查看错误概要；成功任务可查看 appliedSummary（如接入 items 接口可下钻明细，密文仍 masked）。

### 8.4 production 视图禁止可执行同步（再次强调）

- 在 `production` 环境，游戏详情页**不出现** `Sync to Production` 按钮、抽屉、执行入口；最多只读展示历史（若产品需要，可在 production 只读查看 `sync-jobs` 记录）。
- 这是 `00` §9 红线与 `frontend_agent_execution.md`「Keep sandbox-only sync action impossible to trigger in production views」的硬性要求。

---

## 9. 与公共能力的关系（00 / 01 回链）

| 公共能力 | 本模块如何遵循 |
| --- | --- |
| API 包络（`00` §7.2） | 所有响应 `{ "data": ... }` / `{ "error": {...} }`；列表带 `page/pageSize/total`。 |
| 错误码（`00` §7.4） | 复用 `UNKNOWN_SECTION`(400)、`SYNC_BASELINE_MISMATCH`(409)、`VALIDATION_FAILED`(400)、`NOT_FOUND`/`FORBIDDEN`/`INTERNAL`。 |
| 密文脱敏（`00` §6.1） | preview 中密文 `masked=true`，值 `"masked"`；upsert 不经明文。 |
| 审计（`00` §8） | `execute` 写 `audit_logs(action=sync.execute, env=production, detail 脱敏)`。 |
| 鉴权（`00` §7.5 / D5） | `preview→sync.read`、`execute→sync.execute`；危险操作必挂权限。 |
| env 模型（`00` §2 / D1） | 同库按 env diff，仅 sync 域允许显式 `source_env/target_env`。 |
| 基线一致性（D6）+ 幂等 | `execute` 必携带 baseline_token，复核 `target_hash_before`（防目标已变更）+ nonce 去重（防重复执行），见 §5.3/§5.4/§5.6。 |
| 红线（`00` §9） | 无 preview 不直写；不存/不回明文；隐藏/不兼容/无效数据全程排除；production 视图无可执行同步。 |
| 数据流（`01` §8） | 完整复刻 7 步链路（§5.1）。 |

---

## 10. 接口场景矩阵与测试要点

### 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/sync.yaml`；前端 e2e：`tests/frontend/e2e/sync.spec.ts`。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `POST .../sync/preview` | ✓ | ✓ | ✓ | ✓(UNKNOWN_SECTION) | — | ✓ | —(只读) | ✓ | — | — | 按 section 差异、baseline_token 生成、密文 masked |
| `POST .../sync/execute` | ✓ | ✓ | ✓ | ✓(UNKNOWN_SECTION/token非法) | ✓(BASELINE_MISMATCH/TOKEN_CONSUMED) | ✓(sandbox→production) | ✓(sync.execute) | ✓ | — | ✓(全程单事务,不部分写入) | **基线 hash 复核(D6)、nonce 去重幂等、依赖拓扑校验、删除 opt-in、显式 selected_sections** |
| `GET .../sync-jobs` | ✓ | ✓ | ✓ | ✓(非法 status 过滤) | — | — | — | ✓(items 密文 masked) | ✓ | — | 历史列表（previewed/succeeded/failed） |

前端：Playwright e2e —— 同步预览抽屉（按 section 勾选、三色高亮、密文不可见明文）、基线不一致引导重新预览、重复提交被拦（`SYNC_TOKEN_CONSUMED`）、`production` 视图无同步入口（红线截图）；vitest —— SyncSectionDrawer 组件、SyncJobsTab 状态标签。

### 补充关键用例

> 后端：`go test`（domain/app/transport）；前端：`vitest`。下列为本模块**必须覆盖**的关键用例。

#### 后端

1. **未识别 section 拒绝**：`execute`/`preview` 收到 `selectedSections=["channels","marketing"]` → 返回 400 `UNKNOWN_SECTION`，message 含非法值（对齐 plan Task5 `TestExecuteSyncRejectsUnknownSection`）。
2. **基线不一致拒绝**：preview 后人为修改 production 数据使 hash 变化，再 execute → 409 `SYNC_BASELINE_MISMATCH`，并要求重新预览。
3. **token 过期/篡改拒绝**：过期 token 或破坏签名 → 400 `VALIDATION_FAILED`。
3b. **重复执行幂等**：同一有效 token 连续 execute 两次——首次成功；第二次（即便基线未变）→ 409 `SYNC_TOKEN_CONSUMED`，且 production 数据/审计**不被二次写入**（验证 nonce 去重与基线复核相互独立）。
4. **隐藏/不兼容/无效排除**：sandbox 侧隐藏渠道实例不出现在 preview（对齐 plan `TestHiddenChannelExcludedFromSyncPreview`）；`config_status=invalid` 不参与；draft 快照不参与 config diff。
5. **删除 opt-in**：`include_deletes=false` 时 `delete` 明细不被 `applied`；为 true 时才执行删除；preview 始终展示 delete。
6. **显式 selected_sections**：未勾选 section 不写入（执行后该 section 无 `applied=true` 明细，production 该 section 不变）。
7. **依赖校验**：勾选 `packages` 而 production 无 `channels` 且未勾选 → 400 `VALIDATION_FAILED`，details 指明缺失依赖。
8. **有序 upsert / 事务**：中途写入失败整体回滚，`sync_jobs.status=failed`，无部分写入。
9. **哈希确定性**：同一份有效数据多次计算 hash 稳定；执行成功后 `target_hash_after` 正确回填。
10. **密文不外泄**：preview/items 响应中 secret 字段恒为 `masked`，审计 detail 不含明文。

#### 前端

1. **production 视图无同步入口**：`environment=production` 时不渲染 `Sync to Production`（对齐 `frontend_agent_execution.md`）。
2. **selected_sections 只发勾选项**：勾选 `channels`，execute payload `selectedSections=["channels"]`（对齐 plan Task9 sync-drawer 测试）。
3. **add/update/delete 高亮 + masked 展示**：预览抽屉按 section 分组、三色高亮、密文不可见明文。
4. **基线不一致引导重新预览**：收到 `SYNC_BASELINE_MISMATCH` 弹出「重新预览」。
5. **include_deletes 默认关闭**：默认不勾选；delete 行呈「仅提示」态。

---

## 11. 未决问题与假设

| 编号 | 类型 | 内容 | 本期默认处理 |
| --- | --- | --- | --- |
| Q1 | 假设 | `source_hash` / `target_hash_*` 采用 SHA-256，对「有效数据规范化序列化」求值，排除自增 id/时间戳，密文以密文参与。 | 按本文 §2.4 实现；规范化算法在 `domain/sync` 内固定。 |
| Q2 | 未决 | 差异勾选粒度：section 级 vs 单条差异级。 | 本期固定 **section 级**（spec/plan 均按 section）；逐条勾选留待后续。 |
| Q3 | 未决 | preview 生成的 `sync_jobs(previewed)` 与 execute 任务是否同一行。 | 本期默认 execute **新建任务行**并引用基线（preview 行用于追溯）；如需复用 previewId 可在请求加 `previewJobId`，留待实现期定。 |
| Q4 | 已定 | baseline_token 的 `nonce` 服务端去重（幂等）。 | **本期必做**：基线 hash 复核 + nonce 去重双闸门。execute 成功在同事务落 `nonce` 唯一记录；同 token 二次 execute → `SYNC_TOKEN_CONSUMED`(409)。区分「目标已变更」(`SYNC_BASELINE_MISMATCH`) 与「重复执行」(`SYNC_TOKEN_CONSUMED`) 两类，见 §5.3/§5.4/§5.6。 |
| Q5 | 未决 | `include_deletes=true` 时，删除项若仍被 production 其它数据引用（外键/路由引用）如何处理。 | 本期默认**跳过该删除项并在响应 `skipped` 中提示**，不强制级联删除；级联策略留待后续。 |
| Q6 | 假设 | `config` section 仅同步 `status=published` 的快照；draft 快照不参与（对齐 spec「draft 不参与同步执行」）。 | 按假设实现。 |
| Q7 | 未决 | 是否提供 `GET /sync-jobs/{id}/items` 明细接口。 | 列表接口为本期必需；明细接口为可选增强（密文仍 masked）。 |
| Q8 | 假设 | baseline_token TTL 默认 30 分钟，可由配置覆盖。 | 过期即要求重新预览。 |
| Q9 | 假设 | `sync_jobs.operator_id` 来源于鉴权上下文当前管理员，与 `audit_logs.actor_id` 同源。 | 按假设实现。 |
| Q10 | 未决 | 大数据量预览的分页/截断（单 game 差异过多时）。 | 本期假设单 game 差异规模可控、全量返回；超大规模的分页留待后续。 |
