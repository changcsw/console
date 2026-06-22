---
id: dashboard
code: "23"
title: Dashboard 总览（只读聚合视图）
status: target
code_paths:
  - apps/admin-web/src/views/dashboard
depends_on: [cashier-template, snapshot, sync, common]
impacts: [testing]
children: []
---

# 23 · Dashboard 总览（只读聚合视图）

> 本模块文档默认遵循 `../../00-common.md`（env、§3 枚举、§7 API 包络与权限码）与 `../../01-structure.md`（§5 前端结构、§7 模块依赖、路由 `/dashboard`）。冲突以 `00` 为准。
> 本模块由本轮讨论新增，定位为**纯只读聚合视图**：不新增任何业务表，不写任何业务数据，只读取其它模块（`channel` / `account-auth` / `channel-login` / `product` / `cashier-template` / `game-cashier` / `snapshot` / `sync`）的现有数据，按**当前运行环境（env）**聚合出"待办 / 异常 / 状态"卡片与指标，并提供点击跳转到对应模块的入口。

---

## 1. 边界

### 1.1 模块职责（做什么）

- 提供管理后台首页 `/dashboard` 的**聚合只读视图**：把分散在各模块的"需要运营/管理员关注的事项"集中呈现为卡片与指标。
- 聚合维度严格限定在**当前运行环境（`APP_ENV`）**，即用户登录后所处的 env。Dashboard **不允许**跨 env 聚合，也不接受前端传入 env 参数（避免越权读取 production）。
- 所有指标均为**派生量**：由各来源表按既定过滤条件实时统计/查询得到，Dashboard 本身不缓存业务事实、不落库聚合结果。
- 每张卡片/每条指标都附带**跳转目标**（route + query），点击后跳到对应模块的过滤后列表，让运营"从总览直达待办处理点"。

### 1.2 明确不做什么（红线）

- **不新增业务表**：本模块零 DDL。任何聚合都来自只读 SQL。
- **不写任何数据**：无 create/update/delete/publish/审核能力；本模块的所有 API 仅 `GET`，不挂任何写权限码，不写 `audit_logs`。
- **不做业务判定与状态流转**：例如"汇率待审"只统计 `cashier_fx_sync_runs.status='pending_review'` 的数量，审核动作在 `cashier-template` 完成；Dashboard 只读不审。
- **不重新定义口径**：所有枚举/状态语义复用各模块，与 `00` §3 完全一致，不在此引入新枚举。
- **不绕过权限**：用户只能看到自己有读权限的卡片；无权限的卡片整块隐藏或置灰（见 §8）。
- **不跨 env 聚合**：production 环境的 Dashboard 只反映 production 的数据；不在此页提供任何 `Sync to Production` 可执行入口（遵循 `00` §9）。

### 1.3 与其它模块的边界关系

| 维度 | Dashboard（本模块） | 来源模块 |
| --- | --- | --- |
| 数据所有权 | 无（只读） | 各来源模块独占 |
| 写操作 | 无 | 各来源模块 |
| 口径定义 | 复用 | 各来源模块 + `00` §3 |
| 跳转 | 提供入口 | 各来源模块承接处理 |

---

## 2. 领域模型（聚合视图 DTO，无新聚合）

本模块**不引入任何领域聚合根**，不在 `internal/domain` 下新增聚合。它只是 `app/query` 层的一个只读编排服务，产出**聚合视图 DTO**。所有 DTO 仅存在于 `internal/app/dto`（或 `app/query/dashboard` 内部读模型），不映射任何物理表。

### 2.1 视图 DTO 概念结构

```text
DashboardSummary (顶层聚合视图，对应一次 /summary 调用)
├── environment            : 当前运行环境（develop|sandbox|production）
├── generatedAt            : 本次聚合的服务端时间（UTC）
├── timeRange              : 本次聚合采用的时间范围（用于"最近"类指标）
├── fxReview               : FxReviewMetric        （汇率待审，平台级，不按 env）
├── configIssues           : ConfigIssuesMetric     （配置异常 invalid，按 env）
├── recentSyncJobs         : RecentSyncJobsMetric   （最近同步任务状态，按 env）
├── pendingSnapshots       : PendingSnapshotsMetric （待发布快照 draft，按 env）
└── channelInstanceIssues  : ChannelInstanceIssuesMetric（不兼容/隐藏渠道实例，按 env）
```

每个 `*Metric` 是一个**只读数值/明细包**，包含：

- 核心计数（`count` / 分桶计数）。
- 跳转入口（`link`：目标 route + query）。
- 可选的少量"明细预览"（`topItems`，最多 N 条，仅用于卡片内快速浏览，不替代来源模块列表）。
- 该指标的 env 适用性标记（`envScoped: true|false`）。

### 2.2 指标值对象（概念字段）

```text
MetricLink
├── route   : string   # 前端路由路径，如 "/cashier"
└── query   : object    # 跳转携带的过滤条件，如 { tab: "fx-review", status: "pending_review" }

FxReviewMetric
├── pendingReviewCount : int
├── envScoped          : false        # 来源表为平台级，全 env 共享
├── topItems[]         : { runId, templateId, templateName, triggeredAt }
└── link               : MetricLink

ConfigIssuesMetric
├── invalidTotal       : int          # 各来源表 invalid 合计（当前 env）
├── bySource[]         : { source, invalidCount }   # 按来源表分桶
├── envScoped          : true
├── topItems[]         : { source, gameId, gameName, target, lastCheckMessage }
└── link               : MetricLink

RecentSyncJobsMetric
├── window             : 时间范围回显
├── total              : int
├── byStatus           : { previewed, succeeded, failed }   # 按 SyncJobStatus 分桶
├── lastFailedAt       : string|null
├── envScoped          : true         # 按 target_env = 当前 env 过滤
├── topItems[]         : { jobId, gameId, gameName, status, executedAt }
└── link               : MetricLink

PendingSnapshotsMetric
├── draftCount         : int          # status=draft（待发布）
├── envScoped          : true
├── topItems[]         : { snapshotId, gameId, gameName, configVersion, generatedAt }
└── link               : MetricLink

ChannelInstanceIssuesMetric
├── hiddenCount        : int          # 被隐藏实例数
├── incompatibleCount  : int          # 与 market 不兼容实例数
├── envScoped          : true
├── topItems[]         : { gameChannelId, gameId, gameName, channelId, marketCode, issue }
└── link               : MetricLink
```

> 说明：`topItems` 是**可选**的轻量预览（默认每指标 ≤5 条，按时间倒序），用于卡片二级展开；主数据仍以来源模块列表为准。是否返回 `topItems` 由 `/summary` 的 `withTopItems` 参数控制（默认 `false`，仅返回计数，降低首屏开销）。

---

## 3. 数据模型（不新增表 · 读取来源与过滤条件）

### 3.1 本模块新增表：无

本模块**不产生任何迁移文件、不新增任何表/列/索引**。Dashboard 完全建立在其它模块已存在的 v2 表之上。

### 3.2 读取来源表与过滤条件汇总

> 下表以 `00` §2.2 的 v2 env 落库规则为准：带 env 的业务表按 `env = 当前运行环境` 过滤；平台级表全 env 共享（不按 env 过滤，需在卡片上明确标注）。
> 列名以 `docs/architecture/postgresql_ddl_draft.sql` 为基础，叠加 v2 约定的 `env` / `market_code` / `hidden` 等新增列（详见各来源模块文档）。

| 指标 | 来源表 | 关键列 | 过滤条件（current env = E） | env 维度 |
| --- | --- | --- | --- | --- |
| 汇率待审 | `cashier_fx_sync_runs` | `status`, `template_id_ref`, `triggered_at` | `status = 'pending_review'` | **平台级**（不按 env，全环境共享一套，见 `00` §2.2） |
| 配置异常·自有账号认证 | `game_account_auth_configs` | `config_status`, `env`, `game_id_ref`, `last_check_message` | `env = E AND config_status = 'invalid'` | 按 env |
| 配置异常·渠道登录 | `game_channel_login_configs` | `config_status`, `env`, `game_channel_id_ref` | `env = E AND config_status = 'invalid'` | 按 env |
| 配置异常·渠道 IAP | `game_channel_iap_configs` | `config_status`, `env`, `game_channel_id_ref` | `env = E AND config_status = 'invalid'` | 按 env |
| 配置异常·分包 IAP 覆盖 | `channel_package_iap_overrides` | `config_status`, `env`, `package_id_ref` | `env = E AND config_status = 'invalid'` | 按 env |
| 最近同步任务状态 | `sync_jobs` | `status`, `target_env`, `game_id_ref`, `executed_at`, `created_at` | `target_env = E AND created_at >= now()-window` | 按 env（用 `target_env`） |
| 待发布快照 | `game_config_snapshots` | `status`, `env`, `game_id_ref`, `config_version`, `generated_at` | `env = E AND status = 'draft'` | 按 env |
| 不兼容/隐藏渠道实例 | `game_channels` | `hidden`, `market_code`, `env`, `game_id_ref`, `channel_id_ref` | `env = E AND (hidden = TRUE OR 与 market 不兼容)` | 按 env |
| 渠道兼容性判定辅助 | `channels` | `region`（`domestic`/`overseas`） | 关联 `game_channels.channel_id_ref` | 平台级（关联用） |
| 游戏名展示辅助 | `games` | `env`, `game_id`, `game_name` | `env = E`，按 `game_id_ref` 关联取名 | 按 env |

### 3.3 关键过滤口径补充

- **配置异常只统计 `invalid`，不统计 `empty`**：`empty` 表示尚未开始配置（正常初始态），`invalid` 表示"已动手但缺必填/敏感/文件字段或校验未过"（含复制创建后 secret/file 被清空，见 `00` §3.4），是真正需要人工修复的待办。
- **被隐藏/不兼容实例的"不进快照、不参与同步"** 已由 `channel` / `snapshot` / `sync` 保证（`00` §9 红线）。Dashboard 在此**只读统计其数量**，提醒运营这些实例游离在最终配置之外。
- **渠道兼容性判定**（与 `00` §3.2 一致）：`market_code = 'CN'` 仅允许 `region = 'domestic'`；`market_code != 'CN'`（含 `GLOBAL/JP/KR/SEA/HMT`）仅允许 `region = 'overseas'`。违反即"不兼容"。
- **同步任务按 `target_env` 而非 `env`**：`sync_jobs` 无 `env` 列，但有 `source_env`/`target_env`。Dashboard 站在"当前环境视角"统计**以当前 env 为目标**的同步记录（例如运行在 production 时看流入 production 的同步结果）。

### 3.4 索引与性能（不新增，仅利用既有）

Dashboard 仅依赖来源表既有索引（如 `idx_game_config_snapshots_game_id_ref`、`idx_sync_jobs_game_id_ref` 等）与各 `config_status` / `status` / `env` 列。聚合查询均为 `COUNT(*)` + 少量 `ORDER BY ... LIMIT N`，复杂度低。若后续单表数据量大、`config_status='invalid'` 计数变慢，可由来源模块按需补 `(env, config_status)` 部分索引——**该决策归来源模块所有，不在本模块落实**（见 §11）。

---

## 4. 枚举与默认值清单

### 4.1 复用的枚举（均来自 `00` §3，本模块零新增枚举）

| 枚举 | 取值 | 在 Dashboard 的用途 |
| --- | --- | --- |
| `Environment` | `develop` / `sandbox` / `production` | 聚合维度；回显当前 env |
| `FXRunStatus` | `pending_review` / `approved` / `applied` / `ignored` / `failed` | 汇率待审仅取 `pending_review` |
| `ConfigStatus` | `empty` / `invalid` / `valid` | 配置异常仅取 `invalid` |
| `SyncJobStatus` | `previewed` / `succeeded` / `failed` | 最近同步任务按此分桶 |
| `SnapshotStatus` | `draft` / `published` | 待发布快照仅取 `draft` |
| `Market` | `GLOBAL` / `JP` / `KR` / `SEA` / `HMT` / `CN` | 渠道兼容性判定 |
| `ChannelRegion` | `domestic` / `overseas` | 渠道兼容性判定 |

> 注意：DDL 草案里 `game_config_snapshots.status` 为 `VARCHAR(32) DEFAULT 'draft'`，v2 取值对齐 `00` §3 的 `SnapshotStatus`（`draft`/`published`）。Dashboard 的"待发布"= `status='draft'`。

### 4.2 本模块私有的展示用枚举/常量

| 常量 | 取值 | 默认 | 说明 |
| --- | --- | --- | --- |
| `ChannelIssueType` | `hidden` / `incompatible` | — | 仅用于 `channelInstanceIssues.topItems[].issue` 的展示标注，不落库 |
| `ConfigIssueSource` | `account_auth` / `channel_login` / `channel_iap` / `package_iap_override` | — | 配置异常分桶来源标识，对应 4 张来源表 |

### 4.3 时间范围（"最近"类指标）默认值

| 参数 | 取值范围 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `range` | `24h` / `7d` / `30d` / `90d` | `7d` | 控制"最近同步任务状态"统计窗口（按 `sync_jobs.created_at`） |
| `withTopItems` | `true` / `false` | `false` | 是否返回每指标的明细预览（`topItems`） |
| `topN`（每指标明细条数） | `1..20` | `5` | `withTopItems=true` 时生效；超限按 `20` 截断 |

> 仅"最近同步任务状态"受 `range` 约束。其余指标（汇率待审、配置异常、待发布快照、渠道实例问题）是**当前快照态计数**，与时间窗无关，不受 `range` 影响。

### 4.4 其它默认值兜底

- 任一指标无数据时，计数返回 `0`，`topItems` 返回 `[]`，**不返回 null**（前端空态依赖稳定结构）。
- `lastFailedAt` 无失败记录时返回 `null`。
- `environment` 恒等于服务端当前 `APP_ENV`，前端不可覆盖。

---

## 5. 业务规则（各卡片/指标的定义与计算口径）

> 统一约定：current env = E（来自服务端运行环境）。除特别标注"平台级"外，所有计数都隐含 `WHERE env = E`。所有 SQL 为口径示意（实际由 `app/query` 用 pgx 实现），不代表最终物化语句。

### 5.1 卡片 A：汇率待审（FX Pending Review）

- **定义**：当前存在多少条等待人工审核的汇率同步运行。
- **来源**：`cashier-template` 的 `cashier_fx_sync_runs`。
- **口径**：

```sql
SELECT COUNT(*) AS pending_review_count
FROM cashier_fx_sync_runs
WHERE status = 'pending_review';
```

- **env 维度**：**平台级，不按 env 过滤**（`cashier_fx_sync_runs` 属 `00` §2.2 平台级表，全环境共享一套）。卡片上必须用文案/角标标注"全环境"，避免运营误以为是当前 env 专属。
- **跳转**：`/cashier` + `{ tab: "fx-review", status: "pending_review" }`。
- **明细预览**（可选）：按 `triggered_at DESC` 取前 N 条，字段 `runId / templateId / templateName / triggeredAt`。

### 5.2 卡片 B：配置异常（Config Issues · invalid）

- **定义**：当前 env 下，模板驱动配置实例中处于 `invalid` 的总数（按来源分桶）。这些实例缺必填/敏感/文件字段或校验未过，需人工修复。
- **来源**：`account-auth` / `channel-login` / `product` 的四张配置表（自有账号认证、渠道登录、渠道 IAP、分包 IAP 覆盖）。
- **口径**（四张表分别计数后求和）：

```sql
-- 自有账号认证（account-auth）
SELECT COUNT(*) FROM game_account_auth_configs
WHERE env = :E AND config_status = 'invalid';
-- 渠道登录（channel-login）
SELECT COUNT(*) FROM game_channel_login_configs
WHERE env = :E AND config_status = 'invalid';
-- 渠道 IAP（product）
SELECT COUNT(*) FROM game_channel_iap_configs
WHERE env = :E AND config_status = 'invalid';
-- 分包 IAP 覆盖（product）
SELECT COUNT(*) FROM channel_package_iap_overrides
WHERE env = :E AND config_status = 'invalid';
```

- **`invalidTotal`** = 上述四者之和；**`bySource`** 给出每来源的分桶计数（`account_auth` / `channel_login` / `channel_iap` / `package_iap_override`）。
- **不含 `empty`**：`empty` 不计入异常（理由见 §3.3）。
- **env 维度**：按 env。
- **跳转**：分桶点击跳到对应模块列表并预置 `configStatus=invalid` 过滤；卡片整体点击默认跳 `channel` / `account-auth` 系列的"配置异常"汇总入口（约定 route 见 §8）。
- **明细预览**（可选）：合并四源后按 `last_check_at DESC NULLS LAST` 取前 N 条，回带 `source / gameId / gameName / target / lastCheckMessage`，其中 `lastCheckMessage` 直接取来源表 `last_check_message`（密文已脱敏，来源模块保证）。

### 5.3 卡片 C：最近同步任务状态（Recent Sync Jobs）

- **定义**：在 `range` 时间窗内，以当前 env 为目标（`target_env = E`）的同步任务，按 `SyncJobStatus` 分桶计数，并给出最近一次失败时间。
- **来源**：`sync` 的 `sync_jobs`。
- **口径**：

```sql
SELECT status, COUNT(*) AS cnt
FROM sync_jobs
WHERE target_env = :E
  AND created_at >= now() - :window     -- window 由 range 决定，默认 7d
GROUP BY status;                          -- status ∈ previewed|succeeded|failed

SELECT MAX(COALESCE(executed_at, created_at)) AS last_failed_at
FROM sync_jobs
WHERE target_env = :E AND status = 'failed'
  AND created_at >= now() - :window;
```

- **`byStatus`** = `{ previewed, succeeded, failed }`（缺失桶补 `0`）；**`total`** = 三者之和。
- **env 维度**：按 env（用 `target_env`，理由见 §3.3）。
- **production 安全**：本卡片只展示**历史结果**，不提供任何"重新同步/执行同步"按钮（`00` §9）。
- **跳转**：`/games/:gameId`（同步入口在游戏详情下）或 `sync` 的同步历史列表（约定 route 见 §8），预置 `status` 过滤。
- **明细预览**（可选）：按 `COALESCE(executed_at, created_at) DESC` 取前 N 条。

### 5.4 卡片 D：待发布快照（Pending Snapshots · draft）

- **定义**：当前 env 下处于 `draft` 状态、尚未发布的配置快照数量。
- **来源**：`snapshot` 的 `game_config_snapshots`。
- **口径**：

```sql
SELECT COUNT(*) AS draft_count
FROM game_config_snapshots
WHERE env = :E AND status = 'draft';
```

- **env 维度**：按 env。
- **语义**：`draft` 快照表示"已生成但未发布"，是发布/同步链路上的待办；`published` 不计入。
- **跳转**：`/games/:gameId`（快照在游戏详情）或 `snapshot` 快照列表，预置 `status=draft`。
- **明细预览**（可选）：按 `generated_at DESC` 取前 N 条，回带 `snapshotId / gameId / gameName / configVersion / generatedAt`。

### 5.5 卡片 E：不兼容 / 隐藏渠道实例（Channel Instance Issues）

- **定义**：当前 env 下，存在问题、被排除在最终配置之外的渠道实例数量，分两类：
  - **hidden**：`game_channels.hidden = TRUE`（被人工隐藏）。
  - **incompatible**：渠道 region 与其 `market_code` 不兼容（见 §3.3 判定）。
- **来源**：`channel` 的 `game_channels` 关联平台级 `channels.region`。
- **口径**：

```sql
-- 被隐藏实例
SELECT COUNT(*) AS hidden_count
FROM game_channels
WHERE env = :E AND hidden = TRUE;

-- 不兼容实例（CN 仅允许 domestic；非 CN 仅允许 overseas）
SELECT COUNT(*) AS incompatible_count
FROM game_channels gc
JOIN channels c ON c.id = gc.channel_id_ref
WHERE gc.env = :E
  AND (
        (gc.market_code = 'CN'  AND c.region <> 'domestic')
     OR (gc.market_code <> 'CN' AND c.region <> 'overseas')
      );
```

- **是否相交**：`hidden` 与 `incompatible` 在统计上**各自独立计数**（一个实例可能同时既隐藏又不兼容）。卡片分别展示两个数字；若需"去重总数"，由前端用并集说明文案处理，后端不强行去重（明细 `topItems[].issue` 会标注命中哪类，单实例命中两类时列两条标注或合并标注，见前端）。
- **env 维度**：按 env（`channels` 关联只用于取 `region`，本身平台级）。
- **跳转**：`/games/:gameId`（渠道在游戏详情的渠道 Tab）或 `channel` 渠道实例列表，预置 `issue=hidden|incompatible`。
- **明细预览**（可选）：按 `updated_at DESC` 取前 N 条。

### 5.6 全局规则

- **聚合一致性**：`/summary` 一次返回所有卡片在**同一服务端时刻 `generatedAt`**的快照；各指标内部用独立 SQL，但同处一个只读事务/同一连接的一致快照内执行（推荐 `REPEATABLE READ` 或单连接顺序查询），避免卡片之间时间错位。
- **权限可见性**：用户对某指标来源模块无读权限时，该卡片**不参与聚合**（后端跳过其查询并在响应中标 `permitted=false`），前端整块隐藏/置灰（见 §8）。
- **零写入**：本模块任何路径都不产生 `audit_logs`、不改任何来源数据。

---

## 6. 后端 API

> 统一遵循 `00` §7：前缀 `/api/admin`，`Authorization: Bearer`，camelCase，成功包络 `{ "data": ... }`，错误包络 `{ "error": { code, message, details } }`。所有接口均为只读 `GET`，无写权限码。
> 读权限：建议引入聚合读权限码 `dashboard.read`（仅控制能否进入 Dashboard 本身）；**各指标是否返回还需叠加来源模块的读权限**（如 `cashier.read` / `channel.read` / `game.read` / `sync.read` / `snapshot.read`），由 `DashboardQueryService` 逐项裁剪。

### 6.1 `GET /api/admin/dashboard/summary`

聚合返回所有卡片指标，是 Dashboard 首屏唯一必需接口。

**Query 参数**

| 参数 | 类型 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- | --- |
| `range` | enum(`24h`/`7d`/`30d`/`90d`) | 否 | `7d` | 仅作用于"最近同步任务状态" |
| `withTopItems` | boolean | 否 | `false` | 是否返回各指标明细预览 |
| `topN` | int(1..20) | 否 | `5` | 每指标明细条数（`withTopItems=true` 时生效） |

> 无 `env` 参数：env 恒由服务端运行环境决定（`00` §2.1）。

**权限**：`dashboard.read`（进入）。返回的各指标按来源模块读权限逐项裁剪（无权限项 `permitted=false` 且计数置 `0`、明细置 `[]`）。

**DTO（响应 `data`）**

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `environment` | string | 当前运行环境 |
| `generatedAt` | string(ISO-8601 UTC) | 聚合时刻 |
| `timeRange` | object | `{ range, since, until }`，`since/until` 为 UTC |
| `fxReview` | FxReviewMetric | 见下 |
| `configIssues` | ConfigIssuesMetric | 见下 |
| `recentSyncJobs` | RecentSyncJobsMetric | 见下 |
| `pendingSnapshots` | PendingSnapshotsMetric | 见下 |
| `channelInstanceIssues` | ChannelInstanceIssuesMetric | 见下 |

各 Metric 公共字段：`permitted`(boolean)、`envScoped`(boolean)、`link`(MetricLink `{route, query}`)、`topItems`(array，`withTopItems=false` 时恒为 `[]`)。

**示例请求**

```
GET /api/admin/dashboard/summary?range=7d&withTopItems=true&topN=3
Authorization: Bearer <accessToken>
```

**示例响应（200，运行环境 = production）**

```json
{
  "data": {
    "environment": "production",
    "generatedAt": "2026-06-17T13:00:00Z",
    "timeRange": {
      "range": "7d",
      "since": "2026-06-10T13:00:00Z",
      "until": "2026-06-17T13:00:00Z"
    },
    "fxReview": {
      "permitted": true,
      "envScoped": false,
      "pendingReviewCount": 2,
      "link": { "route": "/cashier", "query": { "tab": "fx-review", "status": "pending_review" } },
      "topItems": [
        { "runId": 1201, "templateId": 7, "templateName": "Global Cashier Price v3", "triggeredAt": "2026-06-17T02:00:00Z" },
        { "runId": 1198, "templateId": 9, "templateName": "JP Cashier Price v2", "triggeredAt": "2026-06-16T02:00:00Z" }
      ]
    },
    "configIssues": {
      "permitted": true,
      "envScoped": true,
      "invalidTotal": 5,
      "bySource": [
        { "source": "account_auth", "invalidCount": 1 },
        { "source": "channel_login", "invalidCount": 2 },
        { "source": "channel_iap", "invalidCount": 1 },
        { "source": "package_iap_override", "invalidCount": 1 }
      ],
      "link": { "route": "/games", "query": { "configStatus": "invalid" } },
      "topItems": [
        { "source": "channel_login", "gameId": "g_swordman", "gameName": "剑客世界", "target": "google", "lastCheckMessage": "缺少必填敏感字段或文件字段" },
        { "source": "channel_iap", "gameId": "g_swordman", "gameName": "剑客世界", "target": "apple", "lastCheckMessage": "sharedSecret 未填写" },
        { "source": "account_auth", "gameId": "g_racing", "gameName": "极速狂飙", "target": "feishu", "lastCheckMessage": "appSecret 校验未通过" }
      ]
    },
    "recentSyncJobs": {
      "permitted": true,
      "envScoped": true,
      "window": { "range": "7d", "since": "2026-06-10T13:00:00Z", "until": "2026-06-17T13:00:00Z" },
      "total": 4,
      "byStatus": { "previewed": 1, "succeeded": 2, "failed": 1 },
      "lastFailedAt": "2026-06-15T09:30:00Z",
      "link": { "route": "/games", "query": { "tab": "sync-history", "targetEnv": "production" } },
      "topItems": [
        { "jobId": 88, "gameId": "g_swordman", "gameName": "剑客世界", "status": "failed", "executedAt": "2026-06-15T09:30:00Z" },
        { "jobId": 87, "gameId": "g_racing", "gameName": "极速狂飙", "status": "succeeded", "executedAt": "2026-06-14T11:00:00Z" },
        { "jobId": 86, "gameId": "g_puzzle", "gameName": "方块谜题", "status": "succeeded", "executedAt": "2026-06-13T15:20:00Z" }
      ]
    },
    "pendingSnapshots": {
      "permitted": true,
      "envScoped": true,
      "draftCount": 3,
      "link": { "route": "/games", "query": { "tab": "snapshots", "status": "draft" } },
      "topItems": [
        { "snapshotId": 510, "gameId": "g_swordman", "gameName": "剑客世界", "configVersion": "2026.06.17-1", "generatedAt": "2026-06-17T08:00:00Z" },
        { "snapshotId": 509, "gameId": "g_racing", "gameName": "极速狂飙", "configVersion": "2026.06.16-2", "generatedAt": "2026-06-16T10:00:00Z" },
        { "snapshotId": 508, "gameId": "g_puzzle", "gameName": "方块谜题", "configVersion": "2026.06.15-1", "generatedAt": "2026-06-15T07:00:00Z" }
      ]
    },
    "channelInstanceIssues": {
      "permitted": true,
      "envScoped": true,
      "hiddenCount": 2,
      "incompatibleCount": 1,
      "link": { "route": "/games", "query": { "tab": "channels", "issue": "hidden,incompatible" } },
      "topItems": [
        { "gameChannelId": 3301, "gameId": "g_swordman", "gameName": "剑客世界", "channelId": "google", "marketCode": "CN", "issue": "incompatible" },
        { "gameChannelId": 3290, "gameId": "g_racing", "gameName": "极速狂飙", "channelId": "huawei_cn", "marketCode": "CN", "issue": "hidden" }
      ]
    }
  }
}
```

**错误示例**

```json
{ "error": { "code": "UNAUTHENTICATED", "message": "token expired", "details": [] } }
```

```json
{ "error": { "code": "VALIDATION_FAILED", "message": "range must be one of 24h/7d/30d/90d", "details": [{ "field": "range" }] } }
```

> 若用户连 `dashboard.read` 都没有：返回 `403 FORBIDDEN`。若仅缺部分来源模块读权限：`/summary` 仍 `200`，对应指标 `permitted=false`。

### 6.2 子查询接口（指标钻取，可选实现）

`/summary` 的 `topItems` 只用于卡片内快速预览（默认关闭）。当运营想"在 Dashboard 内分页查看某指标完整明细而不立即跳模块页"时，提供下列只读子接口。它们均为可选增强，**最小可用 MVP 只需 `/summary`**。

所有子接口：均 `GET`、均按 `00` §7.3 分页（`page`/`pageSize`/`sort`），均按当前 env（汇率待审除外），均叠加对应来源模块读权限。

#### 6.2.1 `GET /api/admin/dashboard/pending-fx-runs`

待审汇率运行明细（平台级，不按 env）。读权限：`dashboard.read` + `cashier.read`。

**Query**：`page`/`pageSize`/`sort`（默认 `-triggeredAt`）。

**示例响应（200）**

```json
{
  "data": {
    "items": [
      {
        "runId": 1201,
        "templateId": 7,
        "templateName": "Global Cashier Price v3",
        "candidateVersionId": 73,
        "status": "pending_review",
        "triggeredAt": "2026-06-17T02:00:00Z",
        "diffSummary": { "changedRows": 12, "currencies": ["USD", "EUR"] }
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 2
  }
}
```

#### 6.2.2 `GET /api/admin/dashboard/config-issues`

配置异常明细（当前 env，invalid）。读权限：`dashboard.read` + （`channel.read`/`game.read` 视来源）。

**Query**：`source`(可选，枚举 `account_auth`/`channel_login`/`channel_iap`/`package_iap_override`，缺省=全部合并)、`page`/`pageSize`/`sort`（默认 `-lastCheckAt`）。

**示例响应（200）**

```json
{
  "data": {
    "items": [
      {
        "source": "channel_login",
        "refId": 4501,
        "gameId": "g_swordman",
        "gameName": "剑客世界",
        "target": "google",
        "marketCode": "GLOBAL",
        "configStatus": "invalid",
        "lastCheckAt": "2026-06-17T06:00:00Z",
        "lastCheckMessage": "缺少必填敏感字段或文件字段"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 5
  }
}
```

#### 6.2.3 `GET /api/admin/dashboard/recent-sync-jobs`

最近同步任务明细（当前 env 为 target）。读权限：`dashboard.read` + `sync.read`。

**Query**：`range`(默认 `7d`)、`status`(可选 `previewed`/`succeeded`/`failed`)、`page`/`pageSize`/`sort`（默认 `-createdAt`）。

**示例响应（200）**

```json
{
  "data": {
    "items": [
      {
        "jobId": 88,
        "gameId": "g_swordman",
        "gameName": "剑客世界",
        "sourceEnv": "sandbox",
        "targetEnv": "production",
        "status": "failed",
        "includeDeletes": false,
        "createdAt": "2026-06-15T09:25:00Z",
        "executedAt": "2026-06-15T09:30:00Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 4
  }
}
```

#### 6.2.4 `GET /api/admin/dashboard/pending-snapshots`

待发布快照明细（当前 env，draft）。读权限：`dashboard.read` + `snapshot.read`。

**Query**：`page`/`pageSize`/`sort`（默认 `-generatedAt`）。

**示例响应（200）**

```json
{
  "data": {
    "items": [
      {
        "snapshotId": 510,
        "gameId": "g_swordman",
        "gameName": "剑客世界",
        "configVersion": "2026.06.17-1",
        "configSchemaVersion": "1.4",
        "status": "draft",
        "generatedAt": "2026-06-17T08:00:00Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 3
  }
}
```

#### 6.2.5 `GET /api/admin/dashboard/channel-instance-issues`

不兼容/隐藏渠道实例明细（当前 env）。读权限：`dashboard.read` + `channel.read`。

**Query**：`issue`(可选 `hidden`/`incompatible`，缺省=两者并集)、`page`/`pageSize`/`sort`（默认 `-updatedAt`）。

**示例响应（200）**

```json
{
  "data": {
    "items": [
      {
        "gameChannelId": 3301,
        "gameId": "g_swordman",
        "gameName": "剑客世界",
        "channelId": "google",
        "channelRegion": "overseas",
        "marketCode": "CN",
        "hidden": false,
        "issues": ["incompatible"]
      },
      {
        "gameChannelId": 3290,
        "gameId": "g_racing",
        "gameName": "极速狂飙",
        "channelId": "huawei_cn",
        "channelRegion": "domestic",
        "marketCode": "CN",
        "hidden": true,
        "issues": ["hidden"]
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 3
  }
}
```

### 6.3 接口与权限码总览

| 方法 | 路径 | 进入权限 | 叠加来源读权限 | env 维度 | 写权限 |
| --- | --- | --- | --- | --- | --- |
| GET | `/dashboard/summary` | `dashboard.read` | 逐项裁剪 | 各指标各自 | 无 |
| GET | `/dashboard/pending-fx-runs` | `dashboard.read` | `cashier.read` | 平台级 | 无 |
| GET | `/dashboard/config-issues` | `dashboard.read` | `channel.read`/`game.read` | 当前 env | 无 |
| GET | `/dashboard/recent-sync-jobs` | `dashboard.read` | `sync.read` | 当前 env（target） | 无 |
| GET | `/dashboard/pending-snapshots` | `dashboard.read` | `snapshot.read` | 当前 env | 无 |
| GET | `/dashboard/channel-instance-issues` | `dashboard.read` | `channel.read` | 当前 env | 无 |

> `dashboard.read` 为本模块新增的读权限码（格式遵循 `00` §7.5 `resource.action`）。若团队倾向"只要登录即可看 Dashboard"，可将 `dashboard.read` 作为所有管理员默认拥有的基础权限（见 §11 未决问题）。

---

## 7. 应用服务（DashboardQueryService · 只读）

### 7.1 定位与分层

- 位置：`internal/app/query/dashboard/`（只读用例，归属 `app/query`，符合 `01` §4 分层）。
- 名称：`DashboardQueryService`。
- 性质：**纯只读编排**。不依赖 crypto/file 写能力，不调用任何 command。它从各来源模块的**只读仓储/查询方法**取数后组装 DTO。
- 不新增领域聚合、不新增仓储写方法；如来源模块缺少所需"计数/明细"只读查询方法，则在对应来源仓储上**新增只读方法**（归来源模块所有，本模块只消费）。

### 7.2 依赖（均为只读）

```text
DashboardQueryService
├── env: 当前运行环境（来自 config/中间件注入的 env 上下文）
├── permission: 当前用户权限码集合（用于逐指标裁剪）
├── fxRunReadRepo        (cashier_fx_sync_runs 只读计数/列表)
├── configIssueReadRepos (4 张配置表只读计数/列表)
├── syncJobReadRepo      (sync_jobs 只读计数/列表)
├── snapshotReadRepo     (game_config_snapshots 只读计数/列表)
├── channelInstanceReadRepo (game_channels + channels.region 只读计数/列表)
└── gameNameLookup       (games 只读，按 game_id_ref 取 game_name)
```

### 7.3 关键方法（签名示意）

```text
Summary(ctx, params{range, withTopItems, topN}) -> DashboardSummary
PendingFxRuns(ctx, page) -> Page[FxRunItem]
ConfigIssues(ctx, source?, page) -> Page[ConfigIssueItem]
RecentSyncJobs(ctx, range, status?, page) -> Page[SyncJobItem]
PendingSnapshots(ctx, page) -> Page[SnapshotItem]
ChannelInstanceIssues(ctx, issue?, page) -> Page[ChannelIssueItem]
```

### 7.4 编排要点

1. **env 注入不可被覆盖**：`env` 来自中间件 env 上下文（`00` §2.1），方法签名不暴露 env 入参（汇率待审为平台级，本就不带 env）。
2. **权限裁剪**：`Summary` 内对每个指标先查 `permission.hasPerm(...)`；无权限则跳过该指标查询、置 `permitted=false`、计数置 0、明细置空。
3. **一致性快照**：`Summary` 在单连接/单只读事务内顺序执行各指标查询，统一取一个 `generatedAt`（服务端 `now()`）。失败的单个指标查询不应整体 500——可按指标降级（标记错误态，见 §8 错误态约定），但默认实现遇到底层错误返回 `500 INTERNAL`（最简实现），是否做"指标级降级"列入 §11。
4. **N+1 防护**：`topItems` 的 `gameName` 通过批量 `IN (game_id_ref...)` 一次性回查 `games`，不逐条查询。
5. **零副作用**：服务内禁止任何写仓储依赖；代码评审需保证不引入 command/crypto-write。

### 7.5 Transport 层

- 位置：`internal/transport/http/dashboard/`（新增 handler 包，`01` §4 目录可追加）。
- 路由在 `httpserver` 注册 `/api/admin/dashboard/*`，统一挂鉴权中间件 + env 上下文中间件 + `dashboard.read` 权限中间件；**不挂审计中间件的写分支**（只读无需审计；如需"谁查看了 Dashboard"的访问日志，另由通用访问日志承担，不写 `audit_logs`）。

---

## 8. 前端（`/dashboard`）

### 8.1 页面布局

`views/dashboard/index.vue`，采用卡片栅格（响应式，桌面 2–3 列）。顶部固定一条**环境标识**与时间范围切换器，主体为 5 张指标卡片。

```text
┌───────────────────────────────────────────────────────────────┐
│  Dashboard 总览        [EnvironmentBadge: PRODUCTION]  [range ▾] │  ← 顶部条
├───────────────────────────────┬───────────────────────────────┤
│ A 汇率待审 (全环境)            │ B 配置异常 (invalid)           │
│   2  待审核                    │   5  合计                      │
│   [查看 →]                     │   认证1 登录2 IAP1 覆盖1        │
│                               │   [去修复 →]                   │
├───────────────────────────────┼───────────────────────────────┤
│ C 最近同步任务 (近7天)         │ D 待发布快照 (draft)           │
│   成功2 / 失败1 / 预览1        │   3  待发布                    │
│   最近失败 06-15 09:30         │   [去发布 →]                   │
│   [查看历史 →]                 │                               │
├───────────────────────────────┴───────────────────────────────┤
│ E 渠道实例问题   隐藏 2 · 不兼容 1     [去处理 →]               │
└───────────────────────────────────────────────────────────────┘
```

### 8.2 卡片与小组件

- 复用 `components/page` 的 `PageCard` 容器；每卡含：标题、主指标大数字、分桶/副指标小字、跳转按钮，及可选"展开明细"（消费 `topItems`，需 `withTopItems=true`）。
- 复用 `EnvironmentBadge`（`01` §5.3，来自 `app` store 的 `environment`）常驻顶部，明确当前 env。
- 汇率待审卡片必须显示"全环境"角标（因其 `envScoped=false`），与其它"当前 env"卡片区分。
- 计数为 `0` 的卡片显示"无待办"绿色态，不报警；`>0` 时主数字用警示色（黄/红）。

### 8.3 点击跳转（直达待办处理点）

| 卡片 | 跳转 route | 携带 query | 落点 |
| --- | --- | --- | --- |
| A 汇率待审 | `/cashier` | `{ tab: "fx-review", status: "pending_review" }` | `cashier-template` 汇率审核列表 |
| B 配置异常 | `/games`（或 `channel` / `account-auth` 配置异常汇总） | `{ configStatus: "invalid", source? }` | 异常配置实例列表 |
| C 最近同步 | `/games`（同步历史） | `{ tab: "sync-history", targetEnv, status? }` | `sync` 同步历史 |
| D 待发布快照 | `/games` | `{ tab: "snapshots", status: "draft" }` | `snapshot` 快照列表 |
| E 渠道实例问题 | `/games` | `{ tab: "channels", issue }` | `channel` 渠道实例列表 |

> 跳转 route/query 为前端约定，最终以各来源模块前端文档定义的列表过滤参数为准；Dashboard 仅负责把 `link.query` 透传过去。

### 8.4 空态 / 错误态 / 权限态 / 环境标识

- **空态**：某指标计数为 0 → 卡片显示"暂无待办/异常"，绿色，无跳转高亮（跳转按钮仍可点，进入空列表）。
- **加载态**：首屏 `/summary` 加载时整页骨架屏；卡片各自不单独 loading（一次聚合）。
- **错误态**：`/summary` 整体失败 → 顶部错误条 + "重试"按钮；若后端实现"指标级降级"（§7.4），则失败指标单卡显示错误态与"重试该卡"。
- **权限态**：`permitted=false` 的卡片**整块隐藏**（推荐）或置灰并提示"无权限查看"（与 `01` §5.3 "无权限置灰或隐藏"一致）；连 `dashboard.read` 都没有 → 路由守卫直接拦截，不进入 `/dashboard`。
- **环境标识**：`EnvironmentBadge` 常驻；production 下整页**不出现**任何 `Sync to Production` 等可执行写入口（`00` §9）。

### 8.5 前端数据与 store

- API 客户端：`api/modules/dashboard.ts`，封装 `getSummary(params)` 与各子查询；走 `http.ts` 统一解包 `{data}` 与错误处理。
- 不新增专用 Pinia store；env 取自 `app` store，权限取自 `permission` store（`hasPerm`），枚举文案取自 `dictionary` store（`00` §3 同源）。
- `range` 切换在前端记忆（localStorage，默认 `7d`），仅影响卡片 C。

---

## 9. 与公共能力关系

| 公共能力（`00`/`01`） | 在本模块的体现 |
| --- | --- |
| env 模型（`00` §2） | 所有按 env 指标用当前运行环境过滤；env 不可由前端指定；汇率待审为平台级显式标注"全环境" |
| 全局枚举（`00` §3） | 复用 `FXRunStatus`/`ConfigStatus`/`SyncJobStatus`/`SnapshotStatus`/`Market`/`ChannelRegion`，零新增业务枚举 |
| API 包络（`00` §7.2） | 所有响应 `{data}`/`{error}`；列表子接口用 `{items,page,pageSize,total}` |
| 分页约定（`00` §7.3） | 子查询接口用 `page`/`pageSize`/`sort` |
| 错误码（`00` §7.4） | 复用 `UNAUTHENTICATED`/`FORBIDDEN`/`VALIDATION_FAILED`/`INTERNAL`，本模块不新增错误码 |
| 鉴权权限码（`00` §7.5） | 新增只读 `dashboard.read`；指标级叠加来源模块 `*.read` |
| 密文脱敏（`00` §6） | Dashboard 不展示任何密文；`lastCheckMessage` 由来源模块保证已脱敏；不回明文 |
| 审计（`00` §8） | 只读，不写 `audit_logs` |
| 红线（`00` §9） | production 无可执行同步入口；被隐藏/不兼容/无效实例只统计不纳入配置 |
| 前端通用 UI（`01` §5.3） | 复用 `PageCard`/`EnvironmentBadge`；权限置灰/隐藏；异常态行内可见 |
| 模块依赖（`01` §7） | 处于依赖图最末端，单向只读依赖 12/13/14/15/16/19/20，不被任何模块依赖 |

---

## 10. 测试要点

### 10.1 单元测试（`DashboardQueryService`）

- **env 过滤正确性**：注入 env=production，构造 develop/sandbox/production 三套数据，断言按 env 指标只统计 production；汇率待审跨 env 全计入。
- **配置异常口径**：构造 `empty/invalid/valid` 三态，断言只计 `invalid`；四来源分桶数正确、`invalidTotal`=分桶之和。
- **同步任务窗口**：构造不同 `created_at`，断言 `range` 边界（含/不含 `now()-window`）；`byStatus` 缺桶补 0；`lastFailedAt` 取最近失败。
- **快照口径**：只计 `draft`，`published` 不计。
- **渠道兼容性判定**：覆盖 `CN+domestic`(兼容)、`CN+overseas`(不兼容)、`GLOBAL+overseas`(兼容)、`JP+domestic`(不兼容)；`hidden` 与 `incompatible` 各自独立计数。
- **权限裁剪**：缺某来源读权限时该指标 `permitted=false`、计数 0、明细空，不抛错。
- **零写入**：用 mock 仓储断言无任何写方法被调用、无 `audit_logs` 写入。
- **topItems**：`withTopItems=false` 时恒为空数组；`true` 时按约定排序、按 `topN`（上限 20）截断；`gameName` 批量回查无 N+1。

### 10.2 接口测试（`httptest`）

- 未带 token → 401 `UNAUTHENTICATED`。
- 无 `dashboard.read` → 403 `FORBIDDEN`。
- `range` 非法 → 400 `VALIDATION_FAILED`。
- 正常 → 200，校验包络 `{data}` 与各 Metric 结构、`environment` 等于运行环境、`generatedAt` 为 UTC。
- production 运行环境下响应与前端均无任何写/同步执行入口（前端 e2e 断言不存在 `Sync to Production` 按钮）。

### 10.3 前端测试（vitest + testing-library）

- 各卡片在计数 0 / >0 / 错误 / 无权限四态渲染正确。
- 跳转按钮 route+query 与约定一致。
- `EnvironmentBadge` 显示当前 env；汇率待审卡显示"全环境"角标。
- `range` 切换仅触发卡片 C 数据变化且被本地记忆。

### 10.4 一致性/性能

- 一次 `/summary` 内各指标取同一 `generatedAt`（一致快照）。
- 大数据量下 `/summary` 仅做计数 + 限量明细，p95 在可接受阈值内（基准随来源数据量校准）。

---

## 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/dashboard.yaml`；前端 e2e：`tests/frontend/e2e/dashboard.spec.ts`。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET /api/admin/dashboard/summary | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | ✓ | — | — | 只读聚合(不做业务判定/状态流转)；分桶统计(汇率待审/配置异常/同步状态/待发布快照/渠道实例问题)；按 env 聚合(不接受前端 env)；权限逐项裁剪 permitted=false。S5/S7/S10 — 只读聚合 |
| GET /api/admin/dashboard/pending-fx-runs | ✓ | ✓ | ✓ | ✓ | — | — | — | — | ✓ | — | 只读聚合；平台级(不按 env)；卡片跳转预置过滤。S5/S7/S10 — 只读聚合 |
| GET /api/admin/dashboard/config-issues | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | ✓ | ✓ | — | 只读聚合；分桶统计(配置异常按来源)；按 env 聚合；lastCheckMessage 来源已脱敏。S5/S7/S10 — 只读聚合 |
| GET /api/admin/dashboard/recent-sync-jobs | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | — | 只读聚合；按 env 聚合(target_env)；卡片跳转预置过滤。S5/S7/S10 — 只读聚合 |
| GET /api/admin/dashboard/pending-snapshots | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | — | 只读聚合；按 env 聚合；卡片跳转预置过滤(status=draft)。S5/S7/S10 — 只读聚合 |
| GET /api/admin/dashboard/channel-instance-issues | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | — | 只读聚合；按 env 聚合；hidden/incompatible 分桶；卡片跳转预置过滤。S5/S7/S10 — 只读聚合 |

前端：Playwright e2e `/dashboard`（卡片 0 / >0 / 错误 / 无权限 四态渲染、跳转按钮 route+query 与约定一致、`EnvironmentBadge` 显示当前 env、汇率待审卡"全环境"角标、production 下无 `Sync to Production` 等写入口）/ vitest 组件（`PageCard` 各卡四态、`range` 切换仅触发卡片 C 且本地记忆）。

---

## 11. 未决问题与假设

### 11.1 假设（按现有文档合理推定，若来源模块文档更新需对齐）

- **A1**：v2 已按 `00` §2.2 给 `game_account_auth_configs`/`game_channel_login_configs`/`game_channel_iap_configs`/`channel_package_iap_overrides`/`game_config_snapshots`/`game_channels` 等业务表补齐 `env` 列；DDL 草案为 pre-env 版本，最终列以各来源模块迁移为准。
- **A2**：`game_channels` 已按 D2 增加 `market_code`，并有 `hidden BOOLEAN`（被隐藏标记）；`channels` 已按 D3 有 `region`。本模块的"隐藏/不兼容"判定依赖这两列。
- **A3**：`cashier_fx_sync_runs` 为平台级（无 env），故"汇率待审"为全环境聚合；若未来该表分 env，本卡口径需改为按 env。
- **A4**：`game_config_snapshots.status` 在 v2 取值对齐 `SnapshotStatus`(`draft`/`published`)；"待发布"= `draft`。
- **A5**：同步任务以 `target_env = 当前 env` 表达"当前环境视角"，因 `sync_jobs` 无 `env` 列。

### 11.2 未决问题（需产品/各模块确认）

- **Q1 `dashboard.read` 粒度**：是否设为所有登录管理员默认拥有（即"能登录就能看总览"），还是独立可分配权限码？默认倾向"默认拥有 + 指标级再裁剪"。
- **Q2 指标级降级 vs 整体失败**：`/summary` 中单指标查询出错时，是返回该指标错误态（其余正常）还是整体 500？MVP 默认整体 500，体验优化可改指标级降级。
- **Q3 跳转 query 契约**：各来源模块列表页对 `configStatus`/`issue`/`status`/`targetEnv` 等过滤参数的具体命名需与来源模块前端文档最终对齐；本模块先给约定值。
- **Q4 是否纳入更多指标**：如"汇率 `failed` 运行数""同步 `failed` 待重试""快照与最新配置漂移"等是否进 Dashboard，本期未纳入，作为后续增量。
- **Q5 配置异常是否含 `game_cashier_profiles` 等其它配置态**：本期口径限定四张带 `config_status` 的模板驱动表；收银台 profile 无 `config_status` 列，暂不纳入"配置异常"卡。
- **Q6 缓存**：是否对 `/summary` 加短 TTL（如 30s）缓存以降负载？本期默认实时查询，不缓存。
- **Q7 来源仓储只读方法归属**：Dashboard 所需"计数/限量明细"只读方法应由各来源模块仓储提供；若来源模块尚未提供，需在对应模块补充（不在本模块新增跨表仓储）。
