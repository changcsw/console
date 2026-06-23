---
id: dashboard
code: "23"
title: Dashboard 总览（只读聚合视图）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [cashier-template, snapshot, sync, common]
code_paths:
  - apps/admin-web/src/views/dashboard
---

# 23 · Dashboard 总览 — Compact Spec

> 代码生成用精简规格。完整背景/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（env 模型 §2、枚举 §3、API 包络/错误码/分页/权限码 §7、密文脱敏 §6、审计 §8、红线 §9）与 `../../01-structure.md`（前端结构 §5、模块依赖 §7、路由 `/dashboard`）。

## 边界 / 红线
- 纯**只读聚合视图**：管理后台首页 `/dashboard`，把各模块"待办/异常/状态"集中为卡片+指标，提供跳转入口。
- 聚合维度严格限定**当前运行环境（`APP_ENV`）**；不接受前端传入 env（避免越权读 production），不跨 env 聚合。
- **零 DDL / 零写**：不新增任何表/列/索引；所有 API 仅 `GET`，不挂写权限码、不写 `audit_logs`。
- **不做业务判定/状态流转/重定义口径**：只统计，不审核；枚举语义复用 `00` §3，不引入新业务枚举。
- 无权限的卡片整块隐藏/置灰；production 页面**不出现**任何 `Sync to Production` 等可执行写入口（`00` §9）。
- 所有指标均为**派生量**：实时只读 SQL 统计，不缓存业务事实、不落库聚合结果。

## 数据模型（聚合视图 DTO，无新聚合 / 无物理表）
不新增领域聚合根；位于 `app/query` 只读读模型 / `internal/app/dto`，不映射物理表。

```text
DashboardSummary
├── environment   : develop|sandbox|production（当前运行环境）
├── generatedAt   : 聚合服务端时刻（ISO-8601 UTC）
├── timeRange     : { range, since, until }（UTC，"最近"类指标用）
├── fxReview              : FxReviewMetric          （汇率待审，平台级，不按 env）
├── configIssues          : ConfigIssuesMetric      （配置异常 invalid，按 env）
├── recentSyncJobs        : RecentSyncJobsMetric     （最近同步任务状态，按 env）
├── pendingSnapshots      : PendingSnapshotsMetric   （待发布快照 draft，按 env）
└── channelInstanceIssues : ChannelInstanceIssuesMetric（隐藏/不兼容渠道实例，按 env）
```

各 `*Metric` 公共字段（通用说明）：`permitted`(bool，无来源读权限时 false 且计数置 0/明细置 [])、`envScoped`(bool)、`link`(MetricLink `{route, query}`)、`topItems`(array，`withTopItems=false` 时恒 `[]`)。`MetricLink = { route:string, query:object }`。

各 Metric 私有字段：
- `FxReviewMetric`: `pendingReviewCount:int`，`envScoped=false`；topItems `{runId, templateId, templateName, triggeredAt}`。
- `ConfigIssuesMetric`: `invalidTotal:int`，`bySource[]:{source, invalidCount}`，`envScoped=true`；topItems `{source, gameId, gameName, target, lastCheckMessage}`。
- `RecentSyncJobsMetric`: `window`(时间范围回显)，`total:int`，`byStatus:{previewed, succeeded, failed}`，`lastFailedAt:string|null`，`envScoped=true`；topItems `{jobId, gameId, gameName, status, executedAt}`。
- `PendingSnapshotsMetric`: `draftCount:int`，`envScoped=true`；topItems `{snapshotId, gameId, gameName, configVersion, generatedAt}`。
- `ChannelInstanceIssuesMetric`: `hiddenCount:int`，`incompatibleCount:int`，`envScoped=true`；topItems `{gameChannelId, gameId, gameName, channelId, marketCode, issue}`。
- `topItems` 可选（默认每指标 ≤5 条，按时间倒序），由 `withTopItems` 控制（默认 false 仅返回计数）。

## 读取来源表与聚合口径（核心，不新增表）
统一约定：连接 `search_path = <当前env>, platform`（`00` §2.1）；当前环境业务表 **SQL 不带 `env` 谓词**；平台级表在 `platform` schema，全环境共享一套（卡片须标注"全环境"）。所有 SQL 为口径示意（实际 `app/query` 用 pgx 实现）。聚合均为 `COUNT(*)` + 少量 `ORDER BY ... LIMIT N`，仅依赖来源表既有索引，不新增索引。

| 指标 | 来源表 | 过滤条件 | schema 维度 |
| --- | --- | --- | --- |
| 汇率待审 | `cashier_fx_sync_runs` | `status='pending_review'` | **平台级**（全环境共享） |
| 配置异常·自有账号认证 | `game_account_auth_configs` | `config_status='invalid'` | 当前 env |
| 配置异常·渠道登录 | `game_channel_login_configs` | `config_status='invalid'` | 当前 env |
| 配置异常·渠道 IAP | `game_channel_iap_configs` | `config_status='invalid'` | 当前 env |
| 配置异常·分包 IAP 覆盖 | `channel_package_iap_overrides` | `config_status='invalid'` | 当前 env |
| 配置异常·功能插件 | `game_channel_plugin_configs` | `config_status='invalid'` | 当前 env |
| 配置异常·分包插件覆盖 | `channel_package_plugin_overrides` | `config_status='invalid'` | 当前 env |
| 最近同步任务 | `sync_jobs`（platform） | `target_env=E AND created_at >= now()-window` | 平台表，按 `target_env` 过滤 |
| 待发布快照 | `game_config_snapshots` | `status='draft'` | 当前 env |
| 隐藏/不兼容渠道实例 | `game_channels` | `hidden=TRUE OR 与 market 不兼容` | 当前 env |
| 兼容性判定辅助 | `channels`（platform） | 关联 `game_channels.channel_id_ref → platform.channels`，取 `region`(domestic/overseas) | 平台级（关联用） |
| 游戏名展示辅助 | `games` | 按 `game_id_ref` 关联取 `name` | 当前 env |

关键口径补充：
- **配置异常只计 `invalid`，不计 `empty`**：`empty`=未开始配置（正常初始态）；`invalid`=已动手但缺必填/敏感/文件字段或校验未过（含复制创建后 secret/file 被清空，`00` §3.4），是真正待办。
- **渠道兼容性判定**（同 `00` §3.2）：`market_code='CN'` 仅允许 `region='domestic'`；`market_code!='CN'`（含 `GLOBAL/JP/KR/SEA/HMT`）仅允许 `region='overseas'`。违反即"不兼容"。
- **同步任务按 `target_env`**：`sync_jobs` 无 `env` 列，站在"当前环境视角"统计以当前 env 为目标的同步记录。
- 隐藏/不兼容实例"不进快照、不参与同步"由 `channel`/`snapshot`/`sync` 保证（`00` §9）；Dashboard 只读统计数量。

## 枚举与默认值
复用枚举（均来自 `00` §3，零新增业务枚举）：
- `Environment`: develop/sandbox/production（聚合维度+回显）。
- `FXRunStatus`: pending_review/approved/applied/ignored/failed（汇率待审仅取 pending_review）。
- `ConfigStatus`: empty/invalid/valid（配置异常仅取 invalid）。
- `SyncJobStatus`: previewed/succeeded/failed（最近同步分桶）。
- `SnapshotStatus`: draft/published（待发布仅取 draft）。
- `Market`: GLOBAL/JP/KR/SEA/HMT/CN；`ChannelRegion`: domestic/overseas（兼容性判定）。

本模块私有展示常量（不落库）：
- `ChannelIssueType`: hidden/incompatible（仅 `channelInstanceIssues.topItems[].issue` 标注）。
- `ConfigIssueSource`: account_auth/channel_login/channel_iap/package_iap_override/plugin_config/package_plugin_override（配置异常分桶来源，对应 6 表）。

默认值：
- `range`: 24h/7d/30d/90d，默认 `7d`（仅作用于"最近同步任务"，按 `sync_jobs.created_at`；其余指标为快照态计数，不受 range 影响）。
- `withTopItems`: 默认 `false`；`topN`: 1..20，默认 `5`（withTopItems=true 时生效，超限按 20 截断）。
- 兜底：任一指标无数据 count 返回 `0`、`topItems` 返回 `[]`（不返回 null）；`lastFailedAt` 无失败返回 `null`；`environment` 恒等服务端 `APP_ENV`，前端不可覆盖。

## 业务规则与聚合算法（各卡片口径）

### 卡片 A：汇率待审（平台级，不按 env）
```sql
SELECT COUNT(*) AS pending_review_count
FROM cashier_fx_sync_runs
WHERE status = 'pending_review';
```
卡片须标"全环境"角标（`envScoped=false`）。跳转 `/cashier` + `{tab:"fx-review", status:"pending_review"}`。明细按 `triggered_at DESC` 取前 N（runId/templateId/templateName/triggeredAt）。

### 卡片 B：配置异常（invalid，按 env）
六张表分别计数后求和（`search_path` 已设，无 env 谓词）：
```sql
SELECT COUNT(*) FROM game_account_auth_configs    WHERE config_status='invalid'; -- account_auth
SELECT COUNT(*) FROM game_channel_login_configs    WHERE config_status='invalid'; -- channel_login
SELECT COUNT(*) FROM game_channel_iap_configs      WHERE config_status='invalid'; -- channel_iap
SELECT COUNT(*) FROM channel_package_iap_overrides WHERE config_status='invalid'; -- package_iap_override
SELECT COUNT(*) FROM game_channel_plugin_configs   WHERE config_status='invalid'; -- plugin_config
SELECT COUNT(*) FROM channel_package_plugin_overrides WHERE config_status='invalid'; -- package_plugin_override
```
`invalidTotal`=六者之和；`bySource[]` 给每来源分桶计数。不含 empty。跳转：分桶点击跳对应模块列表预置 `configStatus=invalid`；整体点击跳配置异常汇总。明细：合并各源按 `last_check_at DESC NULLS LAST` 取前 N（source/gameId/gameName/target/lastCheckMessage，message 取来源表已脱敏值）。

### 卡片 C：最近同步任务（按 env=target_env）
```sql
SELECT status, COUNT(*) AS cnt FROM sync_jobs
WHERE target_env = :E AND created_at >= now() - :window  -- window 由 range 决定，默认 7d
GROUP BY status;  -- status ∈ previewed|succeeded|failed

SELECT MAX(COALESCE(executed_at, created_at)) AS last_failed_at FROM sync_jobs
WHERE target_env = :E AND status='failed' AND created_at >= now() - :window;
```
`byStatus={previewed,succeeded,failed}`（缺桶补 0），`total`=三者之和。只展示历史结果，无"重新/执行同步"按钮（`00` §9）。跳转 `/games`（同步历史）预置 `status`。明细按 `COALESCE(executed_at, created_at) DESC` 取前 N。

### 卡片 D：待发布快照（draft，按 env）
```sql
SELECT COUNT(*) AS draft_count FROM game_config_snapshots WHERE status='draft';
```
`published` 不计。跳转 `/games` + `{tab:"snapshots", status:"draft"}`。明细按 `generated_at DESC` 取前 N（snapshotId/gameId/gameName/configVersion/generatedAt）。

### 卡片 E：隐藏/不兼容渠道实例（按 env）
```sql
SELECT COUNT(*) AS hidden_count FROM game_channels WHERE hidden = TRUE;

SELECT COUNT(*) AS incompatible_count
FROM game_channels gc
JOIN platform.channels c ON c.id = gc.channel_id_ref
WHERE (gc.market_code='CN'  AND c.region <> 'domestic')
   OR (gc.market_code<>'CN' AND c.region <> 'overseas');
```
`hidden` 与 `incompatible` **各自独立计数**（单实例可同时命中两类）；后端不去重，明细 `topItems[].issue` 标注命中类型。跳转 `/games` + `{tab:"channels", issue:"hidden|incompatible"}`。明细按 `updated_at DESC` 取前 N。

### 全局规则
- **一致性快照**：`/summary` 各指标在单连接/单只读事务内顺序执行，统一取一个 `generatedAt`（服务端 `now()`），避免卡片时间错位（推荐 REPEATABLE READ 或单连接顺序查询）。
- **权限可见性**：无来源读权限的指标不参与聚合（跳过查询、`permitted=false`、计数 0、明细空），前端整块隐藏/置灰。
- **零写入**：任何路径不产生 `audit_logs`、不改来源数据。
- **N+1 防护**：`topItems` 的 `gameName` 用批量 `IN(game_id_ref...)` 一次回查 `games`。

## 后端 API（前缀 `/api/admin`，包络 `00` §7，全部只读 GET，无写权限码）
进入权限 `dashboard.read`（本模块新增只读权限码，`00` §7.5）；各指标返回还需叠加来源模块读权限，由 `DashboardQueryService` 逐项裁剪（无权限项 `permitted=false`、计数 0、明细 []）。无 `env` 参数（env 由服务端决定）。

### GET `/dashboard/summary`（首屏唯一必需）
Query：`range`(enum 24h/7d/30d/90d，默认 7d，仅作用于最近同步)、`withTopItems`(bool，默认 false)、`topN`(int 1..20，默认 5)。
权限：`dashboard.read`。
响应 `data`：`environment`、`generatedAt`(ISO-8601 UTC)、`timeRange{range,since,until}`、`fxReview`、`configIssues`、`recentSyncJobs`、`pendingSnapshots`、`channelInstanceIssues`（结构见数据模型）。
错误：`UNAUTHENTICATED`(401)、`FORBIDDEN`(403，缺 dashboard.read)、`VALIDATION_FAILED`(400，range 非法)；仅缺部分来源读权限仍 200，对应指标 `permitted=false`。

成功响应示例（200，运行环境=production，withTopItems=true&topN=3）：
```json
{
  "data": {
    "environment": "production",
    "generatedAt": "2026-06-17T13:00:00Z",
    "timeRange": { "range": "7d", "since": "2026-06-10T13:00:00Z", "until": "2026-06-17T13:00:00Z" },
    "fxReview": {
      "permitted": true, "envScoped": false, "pendingReviewCount": 2,
      "link": { "route": "/cashier", "query": { "tab": "fx-review", "status": "pending_review" } },
      "topItems": [ { "runId": 1201, "templateId": 7, "templateName": "Global Cashier Price v3", "triggeredAt": "2026-06-17T02:00:00Z" } ]
    },
    "configIssues": {
      "permitted": true, "envScoped": true, "invalidTotal": 6,
      "bySource": [
        { "source": "account_auth", "invalidCount": 1 },
        { "source": "channel_login", "invalidCount": 2 },
        { "source": "channel_iap", "invalidCount": 1 },
        { "source": "package_iap_override", "invalidCount": 1 },
        { "source": "plugin_config", "invalidCount": 1 },
        { "source": "package_plugin_override", "invalidCount": 0 }
      ],
      "link": { "route": "/games", "query": { "configStatus": "invalid" } },
      "topItems": [ { "source": "channel_login", "gameId": "g_swordman", "gameName": "剑客世界", "target": "google", "lastCheckMessage": "缺少必填敏感字段或文件字段" } ]
    },
    "recentSyncJobs": {
      "permitted": true, "envScoped": true,
      "window": { "range": "7d", "since": "2026-06-10T13:00:00Z", "until": "2026-06-17T13:00:00Z" },
      "total": 4, "byStatus": { "previewed": 1, "succeeded": 2, "failed": 1 }, "lastFailedAt": "2026-06-15T09:30:00Z",
      "link": { "route": "/games", "query": { "tab": "sync-history", "targetEnv": "production" } },
      "topItems": [ { "jobId": 88, "gameId": "g_swordman", "gameName": "剑客世界", "status": "failed", "executedAt": "2026-06-15T09:30:00Z" } ]
    },
    "pendingSnapshots": {
      "permitted": true, "envScoped": true, "draftCount": 3,
      "link": { "route": "/games", "query": { "tab": "snapshots", "status": "draft" } },
      "topItems": [ { "snapshotId": 510, "gameId": "g_swordman", "gameName": "剑客世界", "configVersion": "20260617080000-a1b2c3d4", "generatedAt": "2026-06-17T08:00:00Z" } ]
    },
    "channelInstanceIssues": {
      "permitted": true, "envScoped": true, "hiddenCount": 2, "incompatibleCount": 1,
      "link": { "route": "/games", "query": { "tab": "channels", "issue": "hidden,incompatible" } },
      "topItems": [ { "gameChannelId": 3301, "gameId": "g_swordman", "gameName": "剑客世界", "channelId": "google", "marketCode": "CN", "issue": "incompatible" } ]
    }
  }
}
```

### 子查询接口（指标钻取，可选实现；MVP 仅需 `/summary`）
均 GET、按 `00` §7.3 分页（`page`/`pageSize`/`sort`）、按当前 env（汇率待审除外）、叠加来源模块读权限。响应包络 `{items,page,pageSize,total}`。

| 路径 | 说明 | 进入+叠加权限 | env | 默认 sort | 主要字段/过滤 |
| --- | --- | --- | --- | --- | --- |
| `GET /dashboard/pending-fx-runs` | 待审汇率运行明细 | `dashboard.read`+`cashier.read` | 平台级 | `-triggeredAt` | runId/templateId/templateName/candidateVersionId/status/triggeredAt/diffSummary |
| `GET /dashboard/config-issues` | 配置异常明细 | `dashboard.read`+`channel.read`/`game.read` | 当前 env | `-lastCheckAt` | `source`(可选枚举,缺省合并)；source/refId/gameId/gameName/target/marketCode/configStatus/lastCheckAt/lastCheckMessage |
| `GET /dashboard/recent-sync-jobs` | 最近同步明细 | `dashboard.read`+`sync.preview` | 当前 env(target) | `-createdAt` | `range`(默认7d)/`status`(可选)；jobId/gameId/gameName/sourceEnv/targetEnv/status/includeDeletes/createdAt/executedAt |
| `GET /dashboard/pending-snapshots` | 待发布快照明细 | `dashboard.read`+`snapshot.read` | 当前 env | `-generatedAt` | snapshotId/gameId/gameName/configVersion/configSchemaVersion/status/generatedAt |
| `GET /dashboard/channel-instance-issues` | 隐藏/不兼容实例明细 | `dashboard.read`+`channel.read` | 当前 env | `-updatedAt` | `issue`(可选 hidden/incompatible,缺省并集)；gameChannelId/gameId/gameName/channelId/channelRegion/marketCode/hidden/issues[] |

## 应用服务（DashboardQueryService · 只读）
- 位置 `internal/app/query/dashboard/`（只读用例，`01` §4）；Transport `internal/transport/http/dashboard/`，路由 `/api/admin/dashboard/*` 挂鉴权+env 上下文+`dashboard.read` 中间件，**不挂审计写分支**。
- 性质：纯只读编排，不依赖 crypto/file 写、不调用 command；从来源模块只读仓储/查询取数组装 DTO。如来源仓储缺只读"计数/明细"方法，在对应来源模块仓储新增只读方法（归来源模块所有，本模块只消费）。

```text
// 依赖均只读
DashboardQueryService {
  env                       // 当前运行环境（中间件 env 上下文注入，方法签名不暴露 env 入参）
  permission                // 当前用户权限码集合（逐指标裁剪）
  fxRunReadRepo             // cashier_fx_sync_runs 只读计数/列表
  configIssueReadRepos      // 6 张配置表只读计数/列表
  syncJobReadRepo           // sync_jobs 只读计数/列表
  snapshotReadRepo          // game_config_snapshots 只读计数/列表
  channelInstanceReadRepo   // game_channels + channels.region 只读计数/列表
  gameNameLookup            // games 只读，按 game_id_ref 取 name
}

// 方法签名示意
Summary(ctx, params{range, withTopItems, topN}) -> DashboardSummary
PendingFxRuns(ctx, page) -> Page[FxRunItem]
ConfigIssues(ctx, source?, page) -> Page[ConfigIssueItem]
RecentSyncJobs(ctx, range, status?, page) -> Page[SyncJobItem]
PendingSnapshots(ctx, page) -> Page[SnapshotItem]
ChannelInstanceIssues(ctx, issue?, page) -> Page[ChannelIssueItem]
```
编排要点：env 不可被覆盖；`Summary` 内逐指标 `permission.hasPerm(...)` 裁剪；单连接/单只读事务取统一 `generatedAt`；topItems gameName 批量回查防 N+1；零写副作用（评审保证不引入 command/crypto-write）。

## 前端（`/dashboard`）
- `views/dashboard/index.vue`：卡片栅格（桌面 2–3 列），顶部固定 `EnvironmentBadge`（来自 `app` store environment）+ `range` 切换器；主体 5 张卡片（A 汇率待审/B 配置异常/C 最近同步/D 待发布快照/E 渠道实例问题）。
- 卡片复用 `components/page` 的 `PageCard`：标题+主指标大数字+分桶/副指标小字+跳转按钮+可选"展开明细"（消费 topItems，需 withTopItems=true）。
- 汇率待审卡必须显示"全环境"角标（`envScoped=false`）以区分其它"当前 env"卡片。
- 计数为 0 卡片显示"无待办"绿色态；`>0` 主数字用警示色。
- 跳转（直达待办处理点，query 透传 `link.query`）：A `/cashier`{tab:fx-review,status:pending_review}；B `/games`{configStatus:invalid,source?}；C `/games`{tab:sync-history,targetEnv,status?}；D `/games`{tab:snapshots,status:draft}；E `/games`{tab:channels,issue}。
- 四态：空态绿色"暂无待办"；加载态首屏整页骨架屏（一次聚合）；错误态顶部错误条+重试（若做指标级降级则失败单卡错误态）；权限态 `permitted=false` 整块隐藏（或置灰），缺 `dashboard.read` 路由守卫拦截。
- 数据层：`api/modules/dashboard.ts` 封装 `getSummary(params)` 与子查询，走 `http.ts` 统一解包；不新增专用 Pinia store（env 取 `app` store、权限取 `permission` store `hasPerm`、枚举文案取 `dictionary` store）；`range` 前端记忆（localStorage，默认 7d，仅影响卡片 C）。

## 与公共能力 / 上游关系
- env(`00` §2)：按 env 指标用当前运行环境过滤，env 不可前端指定；汇率待审平台级标"全环境"。
- 枚举(`00` §3)：复用 FXRunStatus/ConfigStatus/SyncJobStatus/SnapshotStatus/Market/ChannelRegion，零新增。
- API 包络/分页/错误码(`00` §7)：`{data}`/`{error}`，列表 `{items,page,pageSize,total}`，复用 UNAUTHENTICATED/FORBIDDEN/VALIDATION_FAILED/INTERNAL，不新增错误码。
- 权限码(`00` §7.5)：新增只读 `dashboard.read`；指标级叠加来源 `*.read`。
- 密文(`00` §6)：不展示任何密文，`lastCheckMessage` 由来源模块保证已脱敏。审计(`00` §8)：只读不写 `audit_logs`。
- 红线(`00` §9)：production 无可执行同步入口；隐藏/不兼容/无效实例只统计不纳入配置。
- 依赖图(`01` §7)：处最末端，单向只读依赖上游（channel/account-auth/channel-login/product/feature-plugin/cashier-template/snapshot/sync/game），不被任何模块依赖。

## 关键假设
- v2 已按 `00` §2.2 将各业务表（game_account_auth_configs / game_channel_login_configs / game_channel_iap_configs / channel_package_iap_overrides / game_channel_plugin_configs / channel_package_plugin_overrides / game_config_snapshots / game_channels）置于每环境 schema；`game_channels` 已有 `market_code` 与 `hidden`，`channels` 已有 `region`。
- `cashier_fx_sync_runs` 为平台级（无 env），故汇率待审为全环境聚合；若未来分 env 需改按 env。
- `sync_jobs` 无 `env` 列，以 `target_env=当前 env` 表达"当前环境视角"；`game_config_snapshots.status` 取值对齐 SnapshotStatus（draft/published），"待发布"=draft。
- `dashboard.read` 粒度（默认拥有 vs 独立可分配）、`/summary` 指标级降级 vs 整体 500、跳转 query 命名（configStatus/issue/status/targetEnv 等）需与来源模块前端文档最终对齐；MVP 默认"默认拥有+指标级裁剪 / 整体 500 / 先用约定值"。
- 配置异常本期限定 6 张带 `config_status` 的模板驱动表；无 `config_status` 列的收银台 profile 暂不纳入。本模块不缓存（默认实时查询）。
