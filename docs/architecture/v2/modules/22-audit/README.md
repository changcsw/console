---
id: audit
code: "22"
title: 审计日志（Audit Logs）
status: target
code_paths:
  - services/admin-api/internal/transport/http/middleware
  - apps/admin-web/src/views/audit
depends_on: [common]
impacts: [testing]
children: []
---

# 22 · 审计日志（Audit Logs）

> 本文是 `audit`「审计日志」的架构设计文档。它同时承担两个角色：
> 1. **横切写入规范**：定义"所有有意义写操作如何统一落审计"的全局约束，被 10–20 所有写操作模块强制调用；
> 2. **查询页**：定义后台「审计日志」页的数据模型、后端 API、应用服务与前端交互。
>
> 本文默认遵循 `../../00-common.md`（特别是 **§8 审计**、§7 分页与包络、§7.5 权限码 `audit.read`）与 `../../01-structure.md`（目录分层、`transport/http/audit/`、`middleware/` 审计中间件）。与公共契约冲突处一律以 `00` 为准；本文只在其基础上追加审计私有约定。
>
> 落地数据表：`services/admin-api/migrations/000001_init.up.sql` 中已存在的 `audit_logs`（真实列：`id / actor_id / action / resource_type / resource_id / env / detail_json / created_at`）。本文不新增列，只规范其使用方式与建议索引。

---

## 1. 边界（横切能力：写入规范 + 查询页）

### 1.1 模块职责

审计日志模块是一个**横切（cross-cutting）能力**，不是某个业务聚合的私有功能。它由两部分构成：

1. **统一写入规范（写侧）**
   - 提供整个后台**唯一**的审计写入入口（`AuditService.Write` + 审计中间件）。
   - 规定"哪些操作必须写审计""写什么字段""`action` 如何命名""`detail_json` 记录什么、如何脱敏"。
   - 被 10（鉴权 RBAC）、11（游戏主数据）、12（渠道实例）、13（自有账号认证）、14（渠道登录）、15（商品与 IAP）、16（收银台模板）、17（游戏级收银台）、18（支付路由）、19（配置快照）、20（同步）等**所有产生有意义写操作的模块调用**。
   - 同步执行（`sync`）除写 `sync_jobs / sync_job_items` 外，**必须**额外写一条 `sync.execute` 审计（见 `01` §8 数据流第 6 步）。

2. **审计查询页（读侧）**
   - 提供 `GET /api/admin/audit-logs` 列表查询接口（过滤 + 分页 + 排序），权限码 `audit.read`。
   - 提供前端「审计日志」页（路由 `/audit`，见 `01` §5.1）：过滤器、列表、详情（before/after）展开、空态/错误态/权限态。
   - 审计页**只读**：没有任何创建/编辑/删除审计记录的接口（不可篡改，见 §5）。

### 1.2 明确不做（Out of Scope）

| 不做的事 | 原因 / 归属 |
| --- | --- |
| 审计记录的修改 / 删除 / 物理清理接口 | 审计只增不改（§5.4）；归档/清理由 DBA 运维策略处理，不暴露 API |
| 业务对象的"字段级历史版本回放" | 审计只记录 before/after 关键摘要，不是 event-sourcing；完整版本史由各业务表 `updated_at` 与快照（`snapshot`）承担 |
| 登录成功/失败、令牌刷新等"认证审计" | 本期统一归到 §4 的 `auth.*` action（属可选扩展，见 §11 未决问题）；与业务写审计同表不同 action |
| 读操作审计（谁查看了什么） | 不记录普通读；仅记录"有意义的写/危险操作"（见 §5.1） |
| 跨服务/链路追踪（trace/span） | 由可观测性体系（日志/APM）负责，与业务审计区分 |
| 审计内容的告警/订阅/导出报表 | 本期不做，列入 §11 未决问题 |

### 1.3 与权限码

- 查询页统一挂权限码 `audit.read`（`00` §7.5）。无该权限：菜单隐藏、接口 403（`FORBIDDEN`）。
- 写入审计**不单独挂权限码**：审计写入是各业务写操作的"副作用"，随业务操作本身的权限校验通过后自动发生。审计写入失败的处理策略见 §5.5。

---

## 2. 领域模型（AuditLog 值对象、统一写入入口/中间件）

### 2.1 领域定位

审计在领域分层中的位置（参照 `01` §4）：

```text
domain/common/      # AuditLog 值对象、Action/ResourceType 枚举常量、脱敏接口定义
app/                # AuditService（统一写入 + 查询编排）
infra/persistence/postgres/  # auditRepository（INSERT / 分页 SELECT）
transport/http/audit/        # 查询 Handler
transport/http/middleware/   # 审计中间件（兜底捕获写操作结果）
```

审计**不是独立聚合根**，`audit_logs` 行是一个**不可变值对象（immutable value object）**：写入后永不更新，只读出。

### 2.2 AuditLog 值对象（领域内表示）

```go
// domain/common/audit.go（示意，仅领域模型，不是实现）
package common

// AuditLog 是一条不可变审计记录。
type AuditLog struct {
    ID           int64          // BIGSERIAL，DB 生成
    ActorID      int64          // 操作者 admin_users.id；系统操作约定见 §4.4
    Action       string         // "resource.action"，与权限码同源，见 §4.1
    ResourceType string         // 资源类型，见 §4.2
    ResourceID   string         // 资源业务标识（字符串，见 §5.3）
    Env          Environment    // 操作发生的环境 develop/sandbox/production
    Detail       AuditDetail    // 落库为 detail_json
    CreatedAt    time.Time      // DB 默认 NOW()
}

// AuditDetail 是 detail_json 的结构化约定（见 §5.3）。
type AuditDetail struct {
    Summary string                 `json:"summary,omitempty"` // 人类可读摘要
    Before  map[string]any         `json:"before,omitempty"`  // 关键 before（脱敏后）
    After   map[string]any         `json:"after,omitempty"`   // 关键 after（脱敏后）
    Changed []string               `json:"changed,omitempty"` // 变更字段名列表
    Extra   map[string]any         `json:"extra,omitempty"`   // 模块自定义补充（如 syncJobId）
    Request *AuditRequestMeta      `json:"request,omitempty"` // 请求上下文（IP/UA/请求ID）
}

type AuditRequestMeta struct {
    IP        string `json:"ip,omitempty"`
    UserAgent string `json:"userAgent,omitempty"`
    RequestID string `json:"requestId,omitempty"`
    Method    string `json:"method,omitempty"`
    Path      string `json:"path,omitempty"`
}
```

> 说明：`detail_json` 列在 DB 中是开放 JSONB，本文给出**推荐结构**（`summary/before/after/changed/extra/request`）。模块写入时应尽量遵循该结构，便于前端统一渲染 before/after diff。

### 2.3 统一写入入口（唯一事实来源）

审计写入**只允许**经由 `AuditService`（应用层）发生，禁止业务代码直接 `INSERT audit_logs`。两种调用路径：

1. **显式写入（首选，业务服务主动调用）**
   - 业务命令服务在写操作的**同一事务**内调用 `AuditService.Write(ctx, AuditWriteInput)`。
   - 优点：能拿到精确的 before/after、resource_id、业务语义最准确。
   - 适用：创建/更新/删除/发布/隐藏/审核/同步执行等所有结构化写用例。

2. **中间件兜底（防遗漏，见 §2.4）**
   - 对"已识别为写操作但业务未显式写审计"的请求，由审计中间件在响应成功后补一条**粗粒度**审计（无 before/after，仅 action/resource/请求摘要）。
   - 目的：保证"有意义写操作必有审计"的硬约束不被开发遗漏击穿；显式写入存在时中间件不重复写（见 §2.4 去重）。

```go
// app/audit_service.go（示意）
type AuditWriteInput struct {
    ActorID      int64
    Action       string
    ResourceType string
    ResourceID   string
    Env          common.Environment // 缺省取当前运行环境（00 §2.1）
    Detail       common.AuditDetail
}

type AuditService interface {
    // Write 在调用方事务内写入一条审计（推荐 tx 透传）。
    Write(ctx context.Context, in AuditWriteInput) error
    // Query 供查询页使用。
    Query(ctx context.Context, q AuditQuery) (AuditPage, error)
}
```

### 2.4 审计中间件（transport/http/middleware）

参照 `01` §4 目录，`middleware/` 含「鉴权、权限、env 上下文、审计、recover」。审计中间件职责：

1. **上下文注入**：把 `actorID`（来自鉴权中间件）、`env`（来自 env 上下文中间件）、`requestID/ip/userAgent/method/path` 注入 `context`，供 `AuditService.Write` 取用。
2. **写操作识别**：仅对**非 GET/HEAD/OPTIONS** 且响应 2xx 的请求视为"成功写操作候选"。
3. **去重**：若该请求生命周期内业务已调用过 `AuditService.Write`（通过 context 标志位记录"已写审计"），中间件**不再补写**；否则补一条粗粒度兜底审计。
4. **失败不阻断**：审计写入失败按 §5.5 处理（记录系统日志，不影响主业务响应）。

```text
请求 -> recover -> 鉴权(actorID) -> env 上下文(env) -> 权限校验 -> 审计中间件(注入 ctx)
     -> 业务 handler/service（显式 AuditService.Write，并打"已写"标志）
     -> 返回前：审计中间件检查标志，未写且为成功写操作则兜底补写
```

---

## 3. 数据模型（逐表逐字段）

### 3.1 表：`audit_logs`（平台级表，但每行带 `env`）

`audit_logs` 在 `00` §2.2 中归类为**平台级表**（不按 env 分物理库/不参与 env 唯一键前置），**但每一行带 `env` 字段**用于"记录该操作发生在哪个环境"。这与"业务表用 env 区分物理行"语义不同：审计的 env 是**操作上下文标记**，不是对象身份的一部分。

DDL（来自 `migrations/000001_init.up.sql`，原样，本文不改列）：

```sql
CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  actor_id BIGINT NOT NULL,
  action VARCHAR(64) NOT NULL,
  resource_type VARCHAR(64) NOT NULL,
  resource_id VARCHAR(128) NOT NULL,
  env VARCHAR(16) NOT NULL CHECK (env IN ('develop', 'sandbox', 'production')),
  detail_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

#### 3.1.1 逐字段说明

| 列名 | 类型 | 可空 | 默认值 | 约束 / 取值 | 含义与写入约定 |
| --- | --- | --- | --- | --- | --- |
| `id` | `BIGSERIAL` | 否 | 自增 | `PRIMARY KEY` | 主键，DB 生成。前端列表"主键/序号"列可用，但排序应基于 `created_at`+`id`（见 §6）。 |
| `actor_id` | `BIGINT` | 否 | — | 逻辑指向 `admin_users.id` | 操作者。**注意 DDL 未建 FK**（审计需在用户被删后仍可追溯），由应用层保证有效或写入约定值（系统操作见 §4.4）。 |
| `action` | `VARCHAR(64)` | 否 | — | 格式 `resource.action`，见 §4.1 | 操作动作，**与权限码同源**。如 `game.create`、`cashier.publish`、`sync.execute`。 |
| `resource_type` | `VARCHAR(64)` | 否 | — | 见 §4.2 枚举清单 | 资源类型，通常等于 `action` 的 `resource` 段（如 `game`、`game_channel`、`cashier_price_template`）。 |
| `resource_id` | `VARCHAR(128)` | 否 | — | 字符串 | 资源业务标识。优先用业务键（如 `game_id`、`channel_id`、`template_id`），无业务键时用 DB `id` 字符串；批量/无单一对象时约定见 §5.3。 |
| `env` | `VARCHAR(16)` | 否 | — | `CHECK (env IN ('develop','sandbox','production'))` | 操作发生的环境。取**当前运行环境**（`00` §2.1）。同步执行写 `production`（目标环境，见 §5.2）。 |
| `detail_json` | `JSONB` | 否 | `'{}'::jsonb` | 推荐结构见 §2.2 / §5.3 | 关键 before/after + 摘要 + 请求上下文，**密文字段必须脱敏**（`00` §6.1）。空对象 `{}` 表示无补充明细。 |
| `created_at` | `TIMESTAMPTZ` | 否 | `NOW()` | — | 记录生成时间（即操作时间）。**无 `updated_at`**：审计只增不改（§5.4）。 |

> 关键点：`audit_logs` **没有** `updated_at` 列（区别于业务表），这是"只增不改"约束的物理体现。任何更新该表行的代码都属于违规。

### 3.2 建议索引（本文新增建议，落地放在 v2 新迁移文件中追加，不改 000001）

原始迁移 `000001` 未对 `audit_logs` 建任何二级索引。审计查询页（§6）按 `env / action / resource_type / actor_id / created_at 范围` 过滤并按时间倒序分页，需补充索引。建议在 v2 新增迁移（如 `0000NN_audit_indexes.up.sql`）中追加：

```sql
-- 时间倒序是默认排序与最常见过滤，单列降序索引覆盖"最近 N 条""时间范围"
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at
  ON audit_logs (created_at DESC);

-- 按操作者追溯
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id
  ON audit_logs (actor_id);

-- 按资源类型过滤（常与时间范围组合）
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type_created_at
  ON audit_logs (resource_type, created_at DESC);

-- 按环境过滤（常与时间范围组合）
CREATE INDEX IF NOT EXISTS idx_audit_logs_env_created_at
  ON audit_logs (env, created_at DESC);

-- 按 action 过滤
CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created_at
  ON audit_logs (action, created_at DESC);

-- 定位某个具体资源的全部操作历史
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource
  ON audit_logs (resource_type, resource_id, created_at DESC);
```

索引设计说明：

- **默认排序索引**：`(created_at DESC)` 服务"无过滤的最近记录"与"纯时间范围"查询。
- **复合索引前缀对齐过滤器**：`resource_type / env / action` 三个高频等值过滤各配 `(<col>, created_at DESC)`，使"等值过滤 + 时间倒序分页"走索引避免额外排序。
- **资源溯源索引**：`(resource_type, resource_id, created_at DESC)` 支撑"查某个游戏/某条价格模板的全部操作历史"。
- 任务要求点名的四项（`actor_id` / `resource_type` / `created_at` / `env`）均已覆盖。
- `detail_json` 如未来需要按内部字段检索，可补 `GIN` 索引（本期不做，见 §11）。

### 3.3 容量与保留策略（说明，非建表）

- 审计为高写入量、只增表；建议运维侧按月做分区或归档（如 `created_at` 时间分区），冷数据转归档表/对象存储。**API 层不感知分区**。
- 本期不实现自动清理；保留期与归档策略列入 §11 未决问题。

---

## 4. 枚举与默认值清单

> 本节是审计写入的**取值字典**。`action` 与 `resource_type` 必须与各业务模块的权限码、资源命名保持同源一致（`00` §7.5、§8）。下列清单为"穷尽样例 + 命名规则"，模块新增写操作时按规则扩展并回填本表。

### 4.1 `action` 命名规则

- 格式恒为 `resource.action`（小写 + 下划线分词），**与权限码同源**：能写该操作的权限码是什么，审计 `action` 就用什么。
- `resource` 段：业务资源名（单数，snake_case），与 `resource_type` 对齐。
- `action` 段：动词，统一使用下列规范动词集合：

| 动词 | 含义 | 典型场景 |
| --- | --- | --- |
| `create` | 新建对象 | 新建游戏、渠道实例、模板、商品 |
| `update` | 更新字段/配置 | 修改配置、改名、调价、改优先级 |
| `delete` | 删除对象 | 删除路由、删除商品 |
| `enable` / `disable` | 启停 | 启用/停用渠道、商品、支付方式 |
| `hide` / `unhide` | 隐藏/取消隐藏 | 隐藏渠道实例（不进快照/同步） |
| `publish` | 发布（版本生命周期） | 收银台价格模板版本发布、快照发布 |
| `archive` | 归档 | 模板版本归档（通常随 publish 自动） |
| `copy` | 复制创建 | 模板 copy-to-draft、配置复制 |
| `approve` / `reject` / `ignore` | 审核决策 | 汇率同步审核、复核 |
| `apply` | 应用/落地 | 汇率审核通过后 apply、收银台 profile 应用 |
| `execute` | 执行（危险动作） | sandbox→production 同步执行 |
| `preview` | 预览（可选，默认不审计） | 同步预览（默认不写，见 §5.1） |
| `login` / `logout` | 认证（可选扩展） | 管理员登录/登出（见 §11） |

### 4.2 `action` 穷尽样例清单（按模块）

> 以下为"覆盖各模块有意义写操作"的样例集合。它**不是封闭枚举**（审计 `action` 列是 VARCHAR，不做 DB 级 CHECK），但前端过滤器与字典 store 应内置该清单作为下拉候选（`00` §3 字典同源）。

| 模块 | `action` 样例 | 说明 |
| --- | --- | --- |
| 10 鉴权/RBAC | `admin_user.create` / `admin_user.update` / `admin_user.disable` / `role.create` / `role.update` / `role.delete` / `role.assign_permission` / `user.assign_role` | 管理员、角色、权限分配 |
| 10 认证（可选） | `auth.login` / `auth.logout` / `auth.token_refresh` | 见 §11，本期可不开启 |
| 11 游戏主数据 | `game.create` / `game.update` / `game.enable` / `game.disable` / `game.delete` / `game_market.create` / `game_market.update` / `game_legal_link.update` | 游戏与 market/法务链接 |
| 12 渠道实例 | `game_channel.create` / `game_channel.update` / `game_channel.enable` / `game_channel.disable` / `game_channel.hide` / `game_channel.unhide` / `channel_package.create` / `channel_package.update` / `channel_package.delete` | 渠道实例（含隐藏） |
| 13 自有账号认证 | `game_account_auth_config.update` / `game_account_auth_config.enable` / `game_account_auth_config.disable` | 账号认证配置 |
| 14 渠道登录 | `game_channel_login_config.update` / `game_channel_login_config.enable` | 渠道登录配置 |
| 15 商品与 IAP | `product.create` / `product.update` / `product.delete` / `product.enable` / `channel_product.update` / `game_channel_iap_config.update` / `channel_package_iap_override.update` | 商品与 IAP 覆盖 |
| 16 收银台模板 | `cashier_price_template.create` / `cashier_price_template.update` / `cashier_price_template_version.create` / `cashier_price_template_version.copy` / `cashier_price_template_version.publish` / `cashier_price_template_version.archive` / `cashier_price_row.update` | 价格模板与版本生命周期 |
| 16/17 汇率 | `fx.preview` / `fx.approve` / `fx.reject` / `fx.ignore` / `fx.apply` | 汇率同步运行审核（`cashier_fx_sync_runs`） |
| 17 游戏级收银台 | `game_cashier_profile.apply` / `game_cashier_profile.update` / `game_cashier_price_override.create` / `game_cashier_price_override.update` / `game_cashier_price_override.delete` | 游戏收银台 profile 与价格覆盖 |
| 18 支付路由 | `payment_route.create` / `payment_route.update` / `payment_route.delete` / `payment_route.enable` / `payment_route.disable` / `pay_way.update` / `cashier_provider.update` / `cashier_merchant_account.create` / `cashier_merchant_account.update` | 支付路由与商户账号 |
| 19 配置快照 | `game_config_snapshot.create` / `game_config_snapshot.publish` | 快照生成与发布 |
| 20 同步 | `sync.execute` | sandbox→production 同步执行（强制审计） |
| 基础数据/字典 | `channel.update` / `account_auth_type.update` / `currency_spec.update` / `billing_subject.update` | 平台级基础数据维护 |

> 任务点名的关键样例已全部覆盖：`game.create`、`game.update`、`channel.hide`（本文落地为 `game_channel.hide`，因隐藏作用于"游戏-渠道实例"而非平台级 `channels`；若有平台级 `channel.disable` 亦写 `channel.disable`）、`cashier.publish`（本文落地为 `cashier_price_template_version.publish`）、`sync.execute`、`fx.approve`。

### 4.3 `resource_type` 穷尽清单

`resource_type` 通常等于 `action` 的 `resource` 段。完整候选（与 DB 表/聚合对齐）：

| `resource_type` | 对应业务/表 | 是否带 env 业务对象 |
| --- | --- | --- |
| `admin_user` | `admin_users` | 否（平台级） |
| `role` | `admin_roles` | 否 |
| `permission` | `admin_permissions` | 否 |
| `game` | `games` | 是 |
| `game_market` | `game_markets` | 是 |
| `game_legal_link` | `game_legal_links` | 是 |
| `channel` | `channels`（平台级基础数据） | 否 |
| `game_channel` | `game_channels` | 是 |
| `channel_package` | `channel_packages` | 是 |
| `account_auth_type` | `account_auth_types` | 否 |
| `game_account_auth_config` | `game_account_auth_configs` | 是 |
| `game_channel_login_config` | `game_channel_login_configs` | 是 |
| `product` | `products` | 是 |
| `channel_product` | `channel_products` | 是 |
| `game_channel_iap_config` | `game_channel_iap_configs` | 是 |
| `channel_package_iap_override` | `channel_package_iap_overrides` | 是 |
| `cashier_price_template` | `cashier_price_templates` | 否 |
| `cashier_price_template_version` | `cashier_price_template_versions` | 否 |
| `cashier_price_row` | `cashier_price_rows` | 否 |
| `cashier_fx_sync_run` / `fx` | `cashier_fx_sync_runs` | 否 |
| `game_cashier_profile` | `game_cashier_profiles` | 是 |
| `game_cashier_price_override` | `game_cashier_price_overrides` | 是 |
| `pay_way` | `pay_ways` | 否 |
| `cashier_provider` | `cashier_providers` | 否 |
| `cashier_merchant_account` | `cashier_merchant_accounts` | 否 |
| `billing_subject` | `billing_subjects` | 否 |
| `currency_spec` | `currency_specs` | 否 |
| `payment_route` | `payment_routes` | 是 |
| `game_config_snapshot` | `game_config_snapshots` | 是 |
| `sync_job` | `sync_jobs`（`action=sync.execute`，`resource_type=sync_job` 或 `game`） | — |
| `auth` | 认证事件（可选扩展） | — |

> `resource_type` 与 `action.resource` 段允许出现 `sync` vs `sync_job`、`fx` vs `cashier_fx_sync_run` 这类便捷别名差异；约定以本表"对应业务/表"列为准，模块实现需固定选择其一并在本表登记。

### 4.4 `env` 取值与默认值

| 取值 | 何时使用 |
| --- | --- |
| `develop` | 当前运行环境为 develop 时的所有写操作 |
| `sandbox` | 当前运行环境为 sandbox 时的所有写操作 |
| `production` | 当前运行环境为 production 时的写操作；**以及同步执行 `sync.execute`（目标环境恒为 production，见 §5.2）** |

- **默认值**：`env` 不允许前端传入，统一取当前运行环境（`00` §2.1）；同步执行特例取目标环境。
- 平台级基础数据（如 `channel.update`、`currency_spec.update`）虽对象本身全 env 共享，但审计仍记录"操作时所处的运行环境"，便于回答"谁在哪个环境下改了平台数据"。

### 4.5 `detail_json` 默认值

- DB 默认 `'{}'::jsonb`。
- 应用写入约定：即使无 before/after，也应尽量写入 `summary` 与 `request` 元信息；完全无补充信息时允许保持 `{}`。

### 4.6 特殊 `actor_id` 约定

| `actor_id` 值 | 含义 |
| --- | --- |
| `> 0` | 真实 `admin_users.id` |
| `0` | 系统/自动任务发起（如定时汇率拉取触发的自动候选生成）。约定用 `0` 表示"系统"，前端展示为 `System`。 |

> 是否引入专门的"系统用户行"而非魔法值 `0`，列入 §11 未决问题。

---

## 5. 业务规则（何时写、写什么、脱敏、不可篡改、统一封装）

### 5.1 何时必须写审计

**判定标准：凡是"有意义的写操作"（改变了系统持久状态且具有业务/合规追溯价值）必须写一条审计。** 具体包括（`00` §8）：

- **创建 / 更新 / 删除**：所有业务对象的 CUD。
- **启停 / 隐藏**：`enable / disable / hide / unhide`。
- **发布 / 归档 / 复制**：版本生命周期 `publish / archive / copy`（`00` §3.3）。
- **审核决策**：汇率同步 `approve / reject / ignore`、`apply`。
- **同步执行**：`sync.execute`（`01` §8 第 6 步硬要求）。

**不写审计**：

- 纯读操作（列表、详情、预览查询）。
- `sync.preview`、`fx.preview` 等预览类（不改状态）。**默认不写**；如需取证可作为可选扩展开启（§11）。
- 幂等无变化的写（如 PUT 提交与现状完全一致、`ON CONFLICT DO NOTHING` 未命中插入）：建议**不写**或写 `summary="no-op"` 且 `changed=[]`（模块自行决定，推荐不写以降噪）。

### 5.2 写什么（字段填充规则）

| 字段 | 填充规则 |
| --- | --- |
| `actor_id` | 当前登录管理员 id（鉴权中间件注入）；系统任务用 `0`（§4.6） |
| `action` | 与触发该写操作的权限码同源（§4.1） |
| `resource_type` | `action` 的 resource 段对应的资源类型（§4.3） |
| `resource_id` | 业务键优先；批量操作见下文 |
| `env` | 当前运行环境；`sync.execute` 取**目标环境 production** |
| `detail_json` | `summary` + `before`/`after`（脱敏）+ `changed` + `extra` + `request`（§2.2） |
| `created_at` | DB `NOW()` |

`resource_id` 规则细化（§5.3 展开）：

- 单对象写：用该对象业务键（如 `game_id="g_1001"`、`template_id="tpl_jp"`）；无业务键时用 DB `id` 字符串。
- 同步执行：`resource_id` 用 `game_id`（同步以 per-game 为单位），`detail_json.extra.syncJobId` 记录 `sync_jobs.id`。
- 批量/无单一对象（如批量调价）：`resource_id` 用代表性父键（如 `template_version_id`），`detail_json` 内用 `extra.affected` 列出受影响明细数量/键列表。

### 5.3 `detail_json` 内容与 before/after 约定

- 只记录**关键字段**的 before/after，不是整行快照（避免巨大 payload）。"关键字段"由各模块定义（一般是用户可改的业务字段 + 状态字段）。
- `changed` 列出本次实际变化的字段名，便于前端高亮。
- `create`：通常只有 `after`（`before` 省略或为 `{}`）。
- `delete`：通常只有 `before`。
- `update`：`before` 与 `after` 同时给出对应字段。

before/after 示例（`game.update` 改名 + 改状态）：

```json
{
  "summary": "更新游戏 g_1001：name、status 变更",
  "changed": ["name", "status"],
  "before": { "name": "Old Name", "status": "draft" },
  "after":  { "name": "New Name", "status": "active" },
  "request": {
    "ip": "10.1.2.3",
    "userAgent": "Mozilla/5.0 ...",
    "requestId": "req_8f3a...",
    "method": "PUT",
    "path": "/api/admin/games/g_1001"
  }
}
```

### 5.4 不可篡改 / 只增不改

- `audit_logs` **只允许 INSERT 与 SELECT**；禁止 UPDATE / DELETE（应用层无任何更新/删除审计的接口或方法）。
- 表无 `updated_at` 列，物理上不预期被更新。
- 不提供"编辑审计""清空审计"的 API（§1.2）。归档/清理仅由 DBA 运维流程在数据库侧进行，且应有独立审批与记录（不在本系统 API 范围）。
- 建议（运维侧，非本文落地）：对 `audit_logs` 收回应用账号的 `UPDATE/DELETE` 权限，仅授予 `INSERT/SELECT`，从权限层面强化不可篡改。

### 5.5 脱敏（强约束，遵循 `00` §6.1）

- `detail_json` 内**任何密文/密钥字段一律脱敏**：写入前用统一脱敏器把 `secret_fields_json` 标记的键替换为 `"masked"` / `"******"`，**绝不写明文**。
- 脱敏覆盖范围：`before`、`after`、`extra` 内嵌套对象同样递归脱敏。
- 文件字段（`file_fields_json`）只记录文件引用（storage key / hash），不记录文件内容。
- 脱敏发生在 `AuditService.Write` 内（统一入口），业务模块只需声明哪些键是 secret（复用各自模板的 `secret_fields_json`），避免每个模块各自实现导致漏脱敏。

### 5.6 审计写入失败的处理策略

- **首选：与业务写同事务**（显式写入路径）。若审计 INSERT 失败导致事务回滚，则业务写也回滚——保证"有业务变更必有审计"的强一致。这是对**高合规要求操作**（如 `sync.execute`、`*.publish`、`fx.approve`）的推荐策略。
- **兜底中间件路径**：响应已返回，无法回滚业务。审计写失败时记录结构化错误日志（含 action/resource/actor），不影响主响应，并可触发告警（§11）。
- 不允许"审计失败被静默吞掉且无任何痕迹"。

### 5.7 统一写入封装（防绕过）

- 全后台**唯一**写入路径为 `AuditService.Write`；代码评审与架构约束禁止任何地方直接 `INSERT INTO audit_logs`（除 `auditRepository` 实现内部）。
- 脱敏、env 取值、actor 注入、`detail_json` 结构规整全部集中在 `AuditService` / 中间件，业务方只提供语义化输入。

---

## 6. 后端 API（逐接口完整 DTO + 示例 JSON）

> 遵循 `00` §7：前缀 `/api/admin`、`Authorization: Bearer`、`application/json; charset=utf-8`、时间 ISO-8601 UTC、字段 camelCase、统一响应包络、统一分页（`page`/`pageSize`/`sort`）。

审计模块**只暴露读接口**（无写接口）。

### 6.1 `GET /api/admin/audit-logs` — 审计日志列表查询

- **权限码**：`audit.read`
- **用途**：按过滤条件分页查询审计记录，按时间倒序展示。

#### 6.1.1 Query 参数（请求 DTO）

| 参数 | 类型 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- | --- |
| `env` | string | 否 | 不限 | 过滤环境，取值 `develop`/`sandbox`/`production`。不传=全部环境。 |
| `action` | string | 否 | 不限 | 过滤动作，精确匹配单个 `action`（如 `game.update`）。 |
| `resourceType` | string | 否 | 不限 | 过滤资源类型（如 `game_channel`）。 |
| `resourceId` | string | 否 | 不限 | 过滤具体资源标识（与 `resourceType` 组合可看单对象历史）。 |
| `operator` | integer | 否 | 不限 | 操作者 `actor_id`（精确）。前端下拉选管理员，提交其 id。 |
| `operatorKeyword` | string | 否 | — | 可选：按操作者用户名/显示名模糊匹配（服务端 join `admin_users`）。与 `operator` 互斥时以 `operator` 优先。 |
| `from` | string(ISO-8601) | 否 | — | 时间范围起（含），按 `created_at >= from`。 |
| `to` | string(ISO-8601) | 否 | — | 时间范围止（含），按 `created_at <= to`。 |
| `keyword` | string | 否 | — | 可选：在 `resource_id` / `detail_json.summary` 上模糊匹配。 |
| `page` | integer | 否 | `1` | 页码，最小 `1`（`00` §7.3）。 |
| `pageSize` | integer | 否 | `20` | 每页条数，最大 `100`（`00` §7.3）。 |
| `sort` | string | 否 | `-createdAt` | 排序，仅支持 `-createdAt`（默认，倒序）/ `createdAt`（正序）。其它字段不支持排序，传入则回退默认。 |

校验规则：

- `env` / `action` / `resourceType` 不在已知字典内时不报错（VARCHAR 开放），但会查无结果。
- `from > to` ⇒ `VALIDATION_FAILED`。
- `pageSize > 100` ⇒ 截断为 100（或 `VALIDATION_FAILED`，按 `00` 风格本文取"截断到上限"）。

#### 6.1.2 响应 DTO（列表项 `AuditLogItem`）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | string | 审计记录 id（BIGINT 转字符串，避免 JS 精度丢失）。 |
| `actorId` | string | 操作者 id；`"0"` 表示系统。 |
| `operator` | object\|null | 操作者展开信息（服务端 join `admin_users`）：`{ "id": string, "userName": string, "displayName": string }`；用户已删/系统则为 `null` 或系统占位。 |
| `action` | string | 动作 `resource.action`。 |
| `resourceType` | string | 资源类型。 |
| `resourceId` | string | 资源标识。 |
| `env` | string | 操作环境。 |
| `detail` | object | `detail_json` 原样回传（已脱敏），结构见 §2.2 / §5.3。 |
| `createdAt` | string(ISO-8601) | 操作时间。 |

#### 6.1.3 请求示例

```http
GET /api/admin/audit-logs?env=production&action=sync.execute&from=2026-06-01T00:00:00Z&to=2026-06-17T23:59:59Z&page=1&pageSize=20&sort=-createdAt
Authorization: Bearer <accessToken>
```

#### 6.1.4 成功响应示例（200，列表包络，`00` §7.2）

```json
{
  "data": {
    "items": [
      {
        "id": "90231",
        "actorId": "12",
        "operator": { "id": "12", "userName": "alice", "displayName": "Alice Zhang" },
        "action": "sync.execute",
        "resourceType": "sync_job",
        "resourceId": "g_1001",
        "env": "production",
        "detail": {
          "summary": "同步 g_1001 sandbox→production，应用 channels/products 共 14 项变更",
          "changed": ["channels", "products"],
          "extra": {
            "syncJobId": "5567",
            "sourceEnv": "sandbox",
            "targetEnv": "production",
            "selectedSections": ["channels", "products"],
            "includeDeletes": false,
            "targetHashBefore": "ab12...",
            "targetHashAfter": "cd34...",
            "appliedItemCount": 14
          },
          "request": {
            "ip": "10.1.2.3",
            "requestId": "req_8f3a2c",
            "method": "POST",
            "path": "/api/admin/sync/execute"
          }
        },
        "createdAt": "2026-06-17T08:32:10Z"
      },
      {
        "id": "90230",
        "actorId": "12",
        "operator": { "id": "12", "userName": "alice", "displayName": "Alice Zhang" },
        "action": "cashier_price_template_version.publish",
        "resourceType": "cashier_price_template_version",
        "resourceId": "tpl_jp@v3",
        "env": "sandbox",
        "detail": {
          "summary": "发布价格模板版本 tpl_jp v3，旧版本 v2 自动归档",
          "changed": ["status"],
          "before": { "status": "draft" },
          "after": { "status": "published" },
          "extra": { "archivedVersion": "v2", "checksum": "9f1c..." }
        },
        "createdAt": "2026-06-17T07:15:44Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 2
  }
}
```

#### 6.1.5 错误响应示例

无权限（缺 `audit.read`）：

```json
{ "error": { "code": "FORBIDDEN", "message": "missing permission: audit.read", "details": [] } }
```

时间范围非法：

```json
{ "error": { "code": "VALIDATION_FAILED", "message": "from must be earlier than to", "details": [{ "field": "from" }] } }
```

未登录：

```json
{ "error": { "code": "UNAUTHENTICATED", "message": "token expired", "details": [] } }
```

### 6.2 `GET /api/admin/audit-logs/{id}` — 单条审计详情（可选）

- **权限码**：`audit.read`
- **用途**：列表已返回完整 `detail`，详情主要为深链/分享与前端解耦提供。若不实现，前端直接用列表项展开（推荐先实现 6.1，6.2 列为可选）。

请求：

```http
GET /api/admin/audit-logs/90231
Authorization: Bearer <accessToken>
```

成功响应（单对象包络）：

```json
{
  "data": {
    "id": "90231",
    "actorId": "12",
    "operator": { "id": "12", "userName": "alice", "displayName": "Alice Zhang" },
    "action": "sync.execute",
    "resourceType": "sync_job",
    "resourceId": "g_1001",
    "env": "production",
    "detail": { "summary": "..." },
    "createdAt": "2026-06-17T08:32:10Z"
  }
}
```

不存在：

```json
{ "error": { "code": "NOT_FOUND", "message": "audit log not found", "details": [] } }
```

### 6.3 `GET /api/admin/audit-logs/facets` — 过滤器候选（可选）

- **权限码**：`audit.read`
- **用途**：为前端过滤下拉提供候选集合（`action` / `resourceType` 列表、可选 operators）。也可由前端字典 store 静态内置（`00` §3），本接口为动态兜底。

成功响应示例：

```json
{
  "data": {
    "envs": ["develop", "sandbox", "production"],
    "actions": ["game.create", "game.update", "game_channel.hide", "cashier_price_template_version.publish", "sync.execute", "fx.approve"],
    "resourceTypes": ["game", "game_channel", "cashier_price_template_version", "sync_job"]
  }
}
```

> 写侧无独立接口：审计写入由各业务模块的写接口在内部完成，不对外暴露 `POST /audit-logs`。

---

## 7. 应用服务（AuditService 统一写入 + 查询）

### 7.1 写入：`AuditService.Write`

职责（集中所有横切逻辑，业务方零散心智负担）：

1. **补全上下文**：从 `ctx` 取 `actorID` / `env` / 请求元信息；`env` 缺省取当前运行环境，`sync.execute` 由调用方显式传 `production`。
2. **脱敏**：对 `detail.before/after/extra` 按调用方声明的 secret keys 递归脱敏（§5.5）。
3. **结构规整**：补 `summary`（缺省可由 action+resourceId 生成）、`changed` 校验。
4. **持久化**：调用 `auditRepository.Insert`；显式路径在业务事务内执行（接收同一 `tx`/`ctx`）。
5. **标记**：在 `ctx` 打"已写审计"标志，供中间件去重（§2.4）。

```go
type SecretAwareAuditInput struct {
    AuditWriteInput
    SecretKeys []string // 需脱敏的字段名（复用模块模板 secret_fields_json）
}

// Write：业务命令服务在写成功后、提交前调用。
func (s *auditService) Write(ctx context.Context, in SecretAwareAuditInput) error
```

调用示例（伪代码，游戏更新用例）：

```go
// app/command/update_game.go（示意）
func (h *UpdateGameHandler) Handle(ctx context.Context, cmd UpdateGameCommand) error {
    return h.tx.WithinTx(ctx, func(ctx context.Context) error {
        before, _ := h.games.Get(ctx, env, cmd.GameID)
        after, err := h.games.Update(ctx, env, cmd)
        if err != nil { return err }
        return h.audit.Write(ctx, SecretAwareAuditInput{
            AuditWriteInput: AuditWriteInput{
                ActorID:      actor.FromCtx(ctx),
                Action:       "game.update",
                ResourceType: "game",
                ResourceID:   cmd.GameID,
                // Env 缺省取运行环境
                Detail: common.AuditDetail{
                    Changed: diffKeys(before, after),
                    Before:  pick(before, changedKeys),
                    After:   pick(after, changedKeys),
                },
            },
            SecretKeys: nil, // game 表无 secret 字段
        })
    })
}
```

### 7.2 查询：`AuditService.Query`

职责：

1. 解析 `AuditQuery`（来自 §6.1 query 参数）。
2. 组装 WHERE（等值过滤 `env/action/resourceType/resourceId/actorId` + 范围 `created_at BETWEEN from AND to` + 可选 keyword）。
3. 分页（`page/pageSize`）+ 排序（`created_at` 倒序为默认，二级排序 `id` 倒序保证稳定）。
4. join `admin_users` 解析 `operator`（`actor_id=0` ⇒ 系统占位）。
5. 返回 `AuditPage{ Items, Page, PageSize, Total }`。

```go
type AuditQuery struct {
    Env             *common.Environment
    Action          *string
    ResourceType    *string
    ResourceID      *string
    Operator        *int64
    OperatorKeyword *string
    From            *time.Time
    To              *time.Time
    Keyword         *string
    Page            int
    PageSize        int
    SortDesc        bool // true=-createdAt（默认）
}

type AuditPage struct {
    Items    []AuditLogItem
    Page     int
    PageSize int
    Total    int64
}
```

仓储层（`infra/persistence/postgres/audit_repository.go`）：

- `Insert(ctx, row) error`：唯一写方法。
- `Query(ctx, q) ([]row, total, error)`：分页查询 + count。
- **不提供** `Update` / `Delete` 方法（物理上杜绝篡改，§5.4）。

---

## 8. 前端（审计日志页）

> 路由 `/audit`（`01` §5.1），视图目录 `apps/admin-web/src/views/audit/`，API 客户端 `api/modules/audit.ts`。遵循 `01` §5.3 通用 UI 契约（列表 + 抽屉式详情、权限指令、统一状态态）。

### 8.1 页面结构

```text
/audit
  ├─ 顶部：页标题 + EnvironmentBadge（当前运行环境，常驻，01 §2）
  ├─ 过滤区（FilterBar）
  │    env(select) | action(select/可搜索) | resourceType(select) |
  │    operator(select 管理员/可搜索) | timeRange(from-to 日期时间范围) |
  │    keyword(input，可选) | [查询] [重置]
  ├─ 列表区（Table，按 createdAt 倒序）
  └─ 详情抽屉（点击行右侧滑出，展示 detail before/after diff）
```

### 8.2 过滤器（与 §6.1 query 对齐）

| 过滤器 | 控件 | 取值来源 | 映射参数 |
| --- | --- | --- | --- |
| 环境 env | select | 字典 `Environment`（`00` §3.1） | `env` |
| 动作 action | 可搜索 select | 字典 action 清单（§4.2）/ facets 接口 | `action` |
| 资源类型 resource type | 可搜索 select | 字典 `resource_type`（§4.3）/ facets | `resourceType` |
| 操作者 operator | 可搜索 select | 管理员列表（用户名/显示名）→ 提交 id | `operator` |
| 时间范围 time range | 日期时间范围选择器 | 用户选择 | `from` / `to`（转 ISO-8601 UTC） |
| 关键字 keyword | input（可选） | 用户输入 | `keyword` |

- 过滤器为"提交式"（点查询才请求），变更分页/排序保留当前过滤条件。
- env 过滤默认**不限**（不强制锁当前运行环境），便于在任一环境查看历史；但页面顶部仍展示当前运行环境徽标避免误解。

### 8.3 列表列

| 列 | 来源字段 | 渲染 |
| --- | --- | --- |
| 时间 | `createdAt` | 本地时区展示 + hover 显示 UTC 原值 |
| 操作者 | `operator.displayName`（兜底 `actorId`） | 系统操作显示 `System` 标签 |
| 动作 | `action` | Tag，按动词色系（create/绿、delete/红、publish/蓝、execute/橙、hide/灰） |
| 资源类型 | `resourceType` | 文本 |
| 资源标识 | `resourceId` | 文本，可点击复制 |
| 环境 | `env` | Tag（production 高亮警示色） |
| 摘要 | `detail.summary` | 单行省略，hover 全文 |
| 操作 | — | 「详情」按钮 → 打开详情抽屉 |

### 8.4 详情抽屉（before/after 展开）

- 顶部：基础信息（id / 操作者 / action / resourceType / resourceId / env / createdAt）。
- 中部：**before / after 对照视图**
  - 仅 `after`（create）：单列展示新建内容。
  - 仅 `before`（delete）：单列展示被删内容。
  - 两者都有（update）：左右对照，`changed` 字段高亮，未变字段弱化或折叠。
  - 密文字段统一显示 `******`（已由后端脱敏，前端不做解密尝试）。
- 底部：`extra`（如 syncJobId、selectedSections）+ `request`（IP / requestId / method / path），折叠展示。
- `detail_json` 提供"查看原始 JSON"折叠区，便于排查。

### 8.5 状态态（空态 / 错误态 / 权限态）

| 状态 | 触发 | 展示 |
| --- | --- | --- |
| 加载态 | 请求中 | 表格骨架屏 / loading |
| 空态 | `items=[]` 且无错误 | 居中空插画 + "当前条件下无审计记录"，提供「重置过滤」按钮 |
| 错误态 | 接口非 2xx（非 403） | 错误提示 + 「重试」按钮，展示 `error.message` |
| 权限态 | 403 / 无 `audit.read` | 整页降级为"无权限查看审计日志"占位；侧边菜单项对无权限用户**隐藏**（`hasPerm('audit.read')`，`01` §5.2） |
| 分页 | `total > pageSize` | 底部分页器，切页保留过滤条件 |

### 8.6 权限与交互约束

- 审计页所有交互均为只读：**无任何写/删除按钮**（符合 §1.2 / §5.4）。
- 菜单与路由守卫：`permission` store 的 `hasPerm('audit.read')` 决定菜单可见与路由可进入。
- 跨模块跳转（可选增强）：从某游戏详情页"操作记录"入口跳到 `/audit?resourceType=game&resourceId=g_1001`，复用同一查询。

---

## 9. 与公共能力关系（被所有模块写操作调用）

### 9.1 上游（审计依赖什么）

| 依赖 | 来源 | 用途 |
| --- | --- | --- |
| 鉴权中间件 | `auth` / `01` middleware | 提供 `actor_id`（写入）、`audit.read` 校验（查询） |
| env 上下文中间件 | `00` §2.1 / `01` middleware | 提供当前运行环境，填 `env` |
| 脱敏能力 | `00` §6.1 + 各模块 `secret_fields_json` | `detail_json` 密文脱敏 |
| 统一包络/分页 | `00` §7.2 / §7.3 | 查询接口响应格式 |
| 字典/枚举 | `00` §3 | 前端过滤器候选（Environment / action / resourceType） |

### 9.2 下游（谁依赖审计）

审计被**所有产生有意义写操作的模块**调用（10–20），关系如 `01` §7 依赖图所示"[审计 21] 横切所有写操作"。每个模块的写用例都必须在其命令服务中调用 `AuditService.Write`：

| 调用方模块 | 触发审计的典型写操作 |
| --- | --- |
| 10 鉴权/RBAC | 管理员/角色/权限变更、角色分配 |
| 11 游戏主数据 | game / market / legal link 的 CUD |
| 12 渠道实例 | game_channel / package 的 CUD、hide/unhide |
| 13/14 登录认证配置 | account_auth / login config 更新 |
| 15 商品与 IAP | product / channel_product / iap config 更新 |
| 16 收银台模板 | 模板/版本 publish/copy/archive、price row 更新 |
| 17 游戏级收银台 | profile apply、price override CUD、汇率 approve/apply |
| 18 支付路由 | route / merchant account 的 CUD |
| 19 配置快照 | snapshot create / publish |
| 20 同步 | **sync.execute（强制，`01` §8 第 6 步）** |
| 22 Dashboard | 只读聚合，不写审计（可消费审计做"最近操作"卡片） |

### 9.3 与 `dashboard`

- Dashboard 可只读消费审计（如"最近 N 条关键操作""今日同步执行次数"），通过 `GET /api/admin/audit-logs` 复用查询，不新增写路径。

---

## 10. 测试要点

### 10.1 单元测试（domain / app）

- **脱敏器**：含 secret key 的 `before/after/extra`（含嵌套）写入后，断言对应键被替换为 `masked`/`******`，明文不出现在落库 payload。
- **env 取值**：普通写取当前运行环境；`sync.execute` 取 `production`；前端不可覆盖 env。
- **detail 结构**：create 仅 after、delete 仅 before、update 同时含 before/after 且 `changed` 与实际 diff 一致。
- **去重标志**：显式 `Write` 后 ctx 标志置位，中间件不重复补写。
- **actor=0**：系统任务写入 `actor_id=0`，查询时 operator 解析为系统占位。

### 10.2 仓储测试（infra/postgres）

- `Insert` 正常落库，`detail_json` 默认 `{}`。
- `Query` 各过滤组合（env / action / resourceType / resourceId / operator / from-to / keyword）正确命中。
- 分页：`page/pageSize` 边界（page=1、超界页返回空 items、pageSize>100 截断）。
- 排序：默认 `-createdAt`，同一时间戳用 `id` 倒序稳定。
- count 与 items 一致性（`total` 准确）。
- **不可变**：仓储不暴露 Update/Delete（编译期保证 / 接口无该方法）。

### 10.3 集成测试（transport）

- `GET /audit-logs` 无 `audit.read` ⇒ 403 `FORBIDDEN`；未登录 ⇒ 401 `UNAUTHENTICATED`。
- `from>to` ⇒ 400 `VALIDATION_FAILED`。
- 完整链路：执行一次 `game.update` 写操作 → 查询审计能查到对应记录且字段正确。
- `sync.execute` 链路：执行同步后，断言除 `sync_jobs/sync_job_items` 外存在一条 `action=sync.execute`、`env=production` 的审计。
- 中间件兜底：构造一个"业务未显式写审计"的成功写请求，断言兜底审计被补写且仅一条。

### 10.4 写入失败策略测试

- 同事务路径：模拟审计 INSERT 失败 ⇒ 业务写回滚（数据库无业务变更、无审计）。
- 兜底路径：审计写失败 ⇒ 主响应仍成功，错误进结构化日志（可断言日志输出）。

### 10.5 前端测试（vitest）

- 过滤器参数正确映射为 query（含时间转 UTC）。
- 空态 / 错误态 / 权限态渲染正确；无 `audit.read` 菜单隐藏。
- 详情抽屉 before/after diff 渲染：create/delete/update 三种情形；密文显示 `******`。
- 切页保留过滤条件。

---

## 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/audit.yaml`；前端 e2e：`tests/frontend/e2e/audit.spec.ts`。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET /api/admin/audit-logs | ✓ | ✓ | ✓ | ✓ | — | ✓ | —* | ✓ | ✓ | — | operator join（含 `actor_id=0` 系统占位）；时间范围 from/to；keyword 模糊 |
| GET /api/admin/audit-logs/{id} | ✓ | ✓ | ✓ | — | — | — | —* | ✓ | — | — | NOT_FOUND 404；单条按 id，无分页/过滤 |
| GET /api/admin/audit-logs/facets | ✓ | ✓ | ✓ | — | — | — | —* | — | — | — | 候选集合（action/resourceType），可由前端字典 store 静态兜底（`00` §3） |

> 维度说明：
> - **S4**：`GET /audit-logs` 校验非法过滤参数（`from > to` ⇒ `VALIDATION_FAILED`，见 §6.1）；`{id}`/`facets` 无可校验入参，标 —。
> - **S5 冲突 / S10 事务回滚**：审计读侧均为单次查询，无并发冲突、无写事务，全部标 —。
> - **S6 跨env**：`audit_logs` 带 `env` 列作过滤维度（见 `00` §2.2 特例），`GET /audit-logs` 支持按 `env` 过滤 ⇒ 标 ✓ 并注明「env 仅过滤维度，非游戏维度业务表」；`{id}`/`facets` 不涉及 env 过滤，标 —。
> - **S7 审计（—*）**：audit 是**被写方**——审计写入由各模块的写操作触发，本模块只提供写入能力（`AuditService.Write` + 中间件，见 §2.3 / §9.2），不在本模块只读接口的自测维度内；写入链路覆盖见 §10.3（`game.update` / `sync.execute` 端到端断言）。
> - **S8 脱敏**：`GET /audit-logs` 与 `{id}` 回传 `detail` 时密文字段已脱敏（`******`，§5.5）⇒ ✓；`facets` 仅返回枚举候选，无敏感字段，标 —。
> - **S9 分页**：仅 `GET /audit-logs` 列表分页（`page/pageSize`，默认 `-createdAt`，二级 `id` 倒序）⇒ ✓；单条/facets 无分页。

前端：`audit.spec.ts` 覆盖 `/audit` 列表查询、过滤器提交、详情抽屉 before/after diff、空态/错误态/权限态（无 `audit.read` 菜单隐藏）；vitest 组件覆盖过滤器→query 映射（时间转 UTC）、三种状态态渲染、before/after diff（create/delete/update、密文显示 `******`）、切页保留过滤条件。

---

## 11. 未决问题与假设

### 11.1 假设（本文采用的默认决策）

1. **不改 `000001` 的列**：`audit_logs` 维持 7 列原状；索引通过 v2 新增迁移追加（§3.2）。
2. **`actor_id` 无 FK**：为在用户删除后仍可追溯，应用层保证有效性；系统操作用 `actor_id=0`（§4.6）。
3. **`action`/`resource_type` 不做 DB CHECK**：保持 VARCHAR 开放，字典由 `00` §3 + 本文 §4 维护，前端下拉用静态字典 + 可选 facets 接口。
4. **`env` 是操作上下文标记**，不参与对象身份；同步执行写目标环境 `production`（§4.4 / §5.2）。
5. **`detail_json` 只存关键 before/after**，非整行快照；密文统一脱敏（§5.3 / §5.5）。
6. **审计页只读**，无任何写/删/清理 API（§1.2 / §5.4）。
7. **id 以字符串回传**前端，避免 JS 大整数精度问题（§6.1.2）。

### 11.2 未决问题（待产品/技术决策）

1. **认证审计是否纳入**：`auth.login/logout/token_refresh` 是否写 `audit_logs`（同表不同 action）还是独立机制？本期默认**不开启**，预留 action 命名。
2. **预览类是否取证**：`sync.preview` / `fx.preview` 是否需要审计留痕（合规取证）？默认不写。
3. **保留期与归档**：审计保留多久、是否分区、冷数据归档到哪里、清理是否需独立审批流？运维侧决策，API 不感知。
4. **系统操作主体**：用魔法值 `actor_id=0` 还是建一行专用"系统用户"？影响 join 与展示。
5. **写入一致性级别**：哪些操作必须"同事务强一致"（失败回滚业务），哪些可"中间件兜底最终一致"？建议对 publish/execute/approve 等高危操作强一致，普通 CUD 视性能权衡。
6. **`detail_json` 检索**：是否需要按 `detail_json` 内部字段查询（GIN 索引）？本期不做。
7. **导出与告警**：审计是否需要导出 CSV、关键 action（如生产同步）实时告警/订阅？本期不做，列为增强。
8. **operator 模糊匹配**：`operatorKeyword` 是否实现（需 join `admin_users` 模糊查），还是仅支持精确 `operator`(id)？默认先实现精确，模糊为可选。
9. **跨模块"操作记录"入口**：各业务详情页是否提供"该对象操作历史"快捷入口（深链 `/audit?resourceType=..&resourceId=..`）？默认作为增强项。
