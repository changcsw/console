---
id: audit
code: "22"
title: 审计日志（Audit Logs）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [common]
code_paths:
  - services/admin-api/internal/transport/http/middleware
  - apps/admin-web/src/views/audit
---

# 22 · 审计日志 — Compact Spec

> 代码生成用精简规格。完整背景/测试矩阵/场景矩阵见 `./README.md`。前置契约见 `../../00-common.md`（§8 审计、§7 分页与包络、§7.5 权限码 `audit.read`、§6.1 脱敏、§2.1 env 模型、§3 字典同源）与 `../../01-structure.md`（目录分层、`transport/http/audit/`、`middleware/` 审计中间件）。冲突以 `00` 为准。

## 边界 / 红线
- 审计是**横切能力**（非业务聚合私有），两职责：① 统一写入规范（写侧，被 `auth`～`sync` 所有写操作模块强制调用）；② 审计查询页（读侧，只读，权限码 `audit.read`）。
- 落地表：`migrations/000001_init.up.sql` 已存在的 `audit_logs`（7 列）。**本模块不新增列**，只规范使用方式 + 追加建议索引（v2 新迁移）。
- 写入**唯一入口** = `AuditService.Write` + 审计中间件；禁止业务代码直接 `INSERT audit_logs`（除 `auditRepository` 内部）。
- 审计**只增不改**：仅 INSERT/SELECT；无 UPDATE/DELETE；无编辑/清空 API；表无 `updated_at` 列。
- 不做：审计记录改/删/清理 API、字段级版本回放（由快照承担）、读操作审计、trace/span、告警/订阅/导出（本期不做）。
- 写入审计**不单独挂权限码**，是各业务写操作的副作用；查询页统一挂 `audit.read`（无权限：菜单隐藏、接口 403）。

## 数据模型

### audit_logs（平台级，位于共享 `platform` schema，**每行带 `env` 过滤列**）
审计是事件流水，每条记录独立、无"跨环境同一对象"，故全 env 一份表、不按 env 拆 schema；`env` 仅作**过滤维度**（操作上下文标记）。
```sql
CREATE TABLE IF NOT EXISTS platform.audit_logs (
  id BIGSERIAL PRIMARY KEY,
  actor_id BIGINT NOT NULL,                          -- 逻辑指向 admin_users.id，无 FK（用户删后仍可追溯）；0=系统
  action VARCHAR(64) NOT NULL,                       -- "resource.action"，与权限码同源
  resource_type VARCHAR(64) NOT NULL,                -- 通常 = action 的 resource 段
  resource_id VARCHAR(128) NOT NULL,                 -- 业务键优先，无则 DB id 字符串
  env VARCHAR(16) NOT NULL CHECK (env IN ('develop','sandbox','production')),
  detail_json JSONB NOT NULL DEFAULT '{}'::jsonb,    -- 关键 before/after+摘要+请求上下文，密文必脱敏
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()      -- 即操作时间；无 updated_at
);
```
- 排序基于 `created_at`+`id`（二级 `id` 倒序保证稳定）。
- `action`/`resource_type` 为开放 VARCHAR，**不做 DB CHECK**；字典由 `00 §3` + 本文 §枚举 维护。

### 建议索引（v2 新迁移 `0000NN_audit_indexes.up.sql` 追加，不改 000001）
```sql
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at                ON audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id                  ON audit_logs (actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type_created_at  ON audit_logs (resource_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_env_created_at            ON audit_logs (env, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created_at         ON audit_logs (action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource                  ON audit_logs (resource_type, resource_id, created_at DESC);
```
高频等值过滤（resource_type/env/action）各配 `(<col>, created_at DESC)` 复合索引使"等值过滤+时间倒序分页"走索引；`(resource_type,resource_id,created_at DESC)` 支撑单对象操作历史。`detail_json` GIN 索引本期不做。容量：高写入只增表，运维侧按月分区/归档，API 不感知。

### 领域模型（domain/common，值对象，非聚合根，写后永不更新）
```go
type AuditLog struct {
    ID int64; ActorID int64; Action string; ResourceType string
    ResourceID string; Env Environment; Detail AuditDetail; CreatedAt time.Time
}
type AuditDetail struct {
    Summary string             `json:"summary,omitempty"` // 人类可读摘要
    Before  map[string]any     `json:"before,omitempty"`  // 关键 before（脱敏后）
    After   map[string]any     `json:"after,omitempty"`   // 关键 after（脱敏后）
    Changed []string           `json:"changed,omitempty"` // 变更字段名列表
    Extra   map[string]any     `json:"extra,omitempty"`   // 模块自定义（如 syncJobId）
    Request *AuditRequestMeta  `json:"request,omitempty"`
}
type AuditRequestMeta struct { IP, UserAgent, RequestID, Method, Path string } // 均 json omitempty
```

## 枚举与默认值清单

### action 命名规则
- 格式恒为 `resource.action`（小写+下划线），**与权限码同源**：能写该操作的权限码是什么，`action` 就用什么。
- `resource` 段 = 业务资源名（单数 snake_case），与 `resource_type` 对齐。
- 规范动词集合：`create` / `update` / `delete` / `enable` / `disable` / `hide` / `unhide` / `publish` / `archive` / `copy` / `approve` / `reject` / `ignore` / `apply` / `execute` / `preview`(默认不审计) / `login` / `logout`(可选扩展)。

### action 样例（按模块，非封闭枚举；前端下拉用本清单 + facets 兜底）
| 模块 | action 样例 |
| --- | --- |
| auth RBAC | `admin_user.create/update/disable`、`role.create/update/delete`、`role.assign_permission`、`user.assign_role` |
| auth 认证(可选) | `auth.login` / `auth.logout` / `auth.token_refresh` |
| game | `game.create/update/enable/disable/delete`、`game_market.create/update`、`game_legal_link.update` |
| channel | `game_channel.create/update/enable/disable/hide/unhide`、`channel_package.create/update/delete` |
| account-auth | `game_account_auth_config.update/enable/disable` |
| channel-login | `game_channel_login_config.update/enable` |
| product | `product.create/update/delete/enable`、`channel_product.update`、`game_channel_iap_config.update`、`channel_package_iap_override.update` |
| cashier-template | `cashier_price_template.create/update`、`cashier_price_template_version.create/copy/publish/archive`、`cashier_price_row.update` |
| cashier 汇率 | `fx.preview/approve/reject/ignore/apply`（`cashier_fx_sync_runs`） |
| game-cashier | `game_cashier_profile.apply/update`、`game_cashier_price_override.create/update/delete` |
| payment | `payment_route.create/update/delete/enable/disable`、`pay_way.update`、`cashier_provider.update`、`cashier_merchant_account.create/update` |
| snapshot | `snapshot.generate` / `snapshot.publish` |
| sync | `sync.execute`（强制审计） |
| 基础数据/字典 | `channel.update`、`account_auth_type.update`、`currency_spec.update`、`billing_subject.update` |

### resource_type 清单（通常 = action 的 resource 段；标注是否游戏维度对象=每环境 schema）
否（平台级 `platform`）：`admin_user` / `role` / `permission` / `channel` / `account_auth_type` / `cashier_price_template` / `cashier_price_template_version` / `cashier_price_row` / `cashier_fx_sync_run`(别名 `fx`) / `pay_way` / `cashier_provider` / `cashier_merchant_account` / `billing_subject` / `currency_spec`。
是（游戏维度，每环境 schema）：`game` / `game_market` / `game_legal_link` / `game_channel` / `channel_package` / `game_account_auth_config` / `game_channel_login_config` / `product` / `channel_product` / `game_channel_iap_config` / `channel_package_iap_override` / `game_cashier_profile` / `game_cashier_price_override` / `payment_route` / `game_config_snapshot`。
特殊：`sync_job`（`action=sync.execute`，`resource_type` 取 `sync_job` 或 `game`）、`auth`（可选扩展）。允许 `sync` vs `sync_job`、`fx` vs `cashier_fx_sync_run` 别名差异，模块需固定其一。

### env 取值与默认
- `develop`/`sandbox`/`production` = 当前运行环境下的写操作；**`sync.execute` 特例取目标环境 `production`**。
- `env` 不允许前端传入，统一取当前运行环境（`00 §2.1`）。平台级基础数据写操作仍记录"操作时所处运行环境"。

### 其它默认
- `detail_json` DB 默认 `'{}'`；即使无 before/after 也应尽量写 `summary` 与 `request`；完全无补充时允许保持 `{}`。
- `actor_id`：`>0`=真实 `admin_users.id`；`0`=系统/自动任务（前端展示 `System`）。

## 业务规则

### 何时写审计（§5.1）
- **必须写**：所有业务对象 CUD；`enable/disable/hide/unhide`；`publish/archive/copy`；审核 `approve/reject/ignore/apply`；`sync.execute`（`01 §8` 第 6 步硬要求）。
- **不写**：纯读（列表/详情/预览查询）；`sync.preview`/`fx.preview` 等预览类（默认不写）；幂等无变化的写（推荐不写，或写 `summary="no-op"`+`changed=[]`）。

### 写什么（字段填充 §5.2）
- `actor_id`=当前管理员 id（鉴权中间件注入），系统任务 `0`；`action`=与权限码同源；`resource_type`=action 的 resource 段；`env`=当前运行环境（`sync.execute` 取 production）；`created_at`=DB NOW()。
- `resource_id` 规则：单对象写用业务键（`game_id`/`template_id`），无则用 DB id 字符串；同步执行用 `game_id` 且 `detail.extra.syncJobId` 记 `sync_jobs.id`；批量用代表性父键 + `extra.affected` 列明细。

### detail_json / before-after 约定（§5.3）
- 只记**关键字段** before/after（非整行快照）；`changed` 列出实际变化字段名供前端高亮。
- `create` 通常只 `after`；`delete` 通常只 `before`；`update` 同时给出对应字段。
```json
{
  "summary": "更新游戏 g_1001：name、status 变更",
  "changed": ["name", "status"],
  "before": { "name": "Old Name", "status": "draft" },
  "after":  { "name": "New Name", "status": "active" },
  "request": { "ip": "10.1.2.3", "requestId": "req_8f3a", "method": "PUT", "path": "/api/admin/games/g_1001" }
}
```

### 脱敏（强约束，遵循 `00 §6.1`，§5.5）
- `detail_json` 内任何密文/密钥字段一律脱敏：写入前用统一脱敏器把 `secret_fields_json` 标记的键替换为 `"masked"`/`"******"`，**绝不写明文**；`before`/`after`/`extra` 嵌套对象递归脱敏。
- 文件字段（`file_fields_json`）只记文件引用（storage key/hash），不记内容。
- 脱敏发生在 `AuditService.Write` 内（统一入口），业务方只声明哪些键是 secret（复用各自模板 `secret_fields_json`）。

### 不可篡改（§5.4）/ 写入失败策略（§5.6）/ 统一封装（§5.7）
- `audit_logs` 仅 INSERT/SELECT，禁 UPDATE/DELETE；无编辑/清空 API；归档/清理仅 DBA 运维侧（建议收回应用账号 UPDATE/DELETE 权限）。
- **首选与业务写同事务**（显式路径）：审计 INSERT 失败 → 业务写回滚（保证"有变更必有审计"），推荐用于高合规操作（`sync.execute`/`*.publish`/`fx.approve`）。
- **兜底中间件路径**：响应已返回无法回滚，写失败记结构化错误日志（含 action/resource/actor）+ 可触发告警，不影响主响应。不允许静默吞掉无痕迹。

### 统一写入入口与中间件（§2.3 / §2.4）
两种调用路径：
1. **显式写入（首选）**：业务命令服务在写操作**同一事务**内调 `AuditService.Write`，能拿到精确 before/after。
2. **中间件兜底（防遗漏）**：对"已识别为写操作但未显式写审计"的请求，响应成功后补一条**粗粒度**审计（无 before/after，仅 action/resource/请求摘要）；显式写入存在时不重复写。

审计中间件职责：① 上下文注入（`actorID`/`env`/`requestID/ip/userAgent/method/path` 注入 ctx）；② 写操作识别（仅**非 GET/HEAD/OPTIONS** 且响应 2xx 视为成功写候选）；③ 去重（ctx 标志位记录"已写审计"则不补写）；④ 失败不阻断（按 §写入失败策略 处理）。
```text
请求 -> recover -> 鉴权(actorID) -> env 上下文(env) -> 权限校验 -> 审计中间件(注入 ctx)
     -> 业务 handler/service（显式 AuditService.Write，并打"已写"标志）
     -> 返回前：审计中间件检查标志，未写且为成功写操作则兜底补写
```

## 后端 API（前缀 `/api/admin`，包络 `00 §7`；**仅读接口，无写接口**；权限码 `audit.read`）

### GET `/api/admin/audit-logs` — 列表查询（分页，时间倒序）
Query 参数：
| 参数 | 类型 | 必填 | 默认 | 说明/校验 |
| --- | --- | --- | --- | --- |
| env | string | 否 | 不限 | develop/sandbox/production，不传=全部 |
| action | string | 否 | 不限 | 精确匹配单个 action |
| resourceType | string | 否 | 不限 | 精确 |
| resourceId | string | 否 | 不限 | 与 resourceType 组合看单对象历史 |
| operator | integer | 否 | 不限 | actor_id 精确，前端下拉提交 id |
| operatorKeyword | string | 否 | — | 按用户名/显示名模糊（join admin_users）；与 operator 共存时 operator 优先 |
| from | string(ISO-8601) | 否 | — | `created_at >= from`（含） |
| to | string(ISO-8601) | 否 | — | `created_at <= to`（含） |
| keyword | string | 否 | — | 在 resource_id / detail.summary 模糊匹配 |
| page | integer | 否 | 1 | 最小 1 |
| pageSize | integer | 否 | 20 | 最大 100，超限截断到 100 |
| sort | string | 否 | -createdAt | 仅支持 `-createdAt`/`createdAt`，其它回退默认 |

校验：env/action/resourceType 不在字典内不报错（开放）但查无结果；`from > to` ⇒ `VALIDATION_FAILED`。
响应项 `AuditLogItem`：`{ id(string,避免JS精度), actorId(string,"0"=系统), operator(object|null: {id,userName,displayName}，join admin_users，删/系统则 null 或系统占位), action, resourceType, resourceId, env, detail(脱敏后原样回传，结构见上), createdAt(ISO-8601) }`，外层列表包络 `{ data:{ items[], page, pageSize, total } }`。

错误码：`FORBIDDEN`(403 缺 audit.read)、`UNAUTHENTICATED`(401 未登录)、`VALIDATION_FAILED`(400 from>to)。

### GET `/api/admin/audit-logs/{id}` — 单条详情（可选）
- 单对象包络回传与列表项同构的对象；推荐先实现列表，本接口为深链/解耦用。不存在 ⇒ `NOT_FOUND`(404)。

### GET `/api/admin/audit-logs/facets` — 过滤器候选（可选）
- 为前端下拉提供 `{ envs[], actions[], resourceTypes[] }` 候选；也可由前端字典 store 静态内置（`00 §3`），本接口为动态兜底。
- **写侧无独立接口**：不对外暴露 `POST /audit-logs`，审计写入由各业务写接口内部完成。

## 应用服务 / 仓储

```go
// app/audit_service.go
type AuditWriteInput struct {
    ActorID int64; Action string; ResourceType string; ResourceID string
    Env common.Environment  // 缺省取当前运行环境
    Detail common.AuditDetail
}
type SecretAwareAuditInput struct {
    AuditWriteInput
    SecretKeys []string  // 需脱敏字段名（复用模块模板 secret_fields_json）
}
type AuditQuery struct {
    Env *common.Environment; Action, ResourceType, ResourceID *string
    Operator *int64; OperatorKeyword *string; From, To *time.Time; Keyword *string
    Page, PageSize int; SortDesc bool  // true=-createdAt（默认）
}
type AuditPage struct { Items []AuditLogItem; Page, PageSize int; Total int64 }

type AuditService interface {
    Write(ctx context.Context, in SecretAwareAuditInput) error  // 业务写成功后、提交前调用（同事务）
    Query(ctx context.Context, q AuditQuery) (AuditPage, error)
}
```
`Write` 职责：① 补全上下文（从 ctx 取 actorID/env/请求元，env 缺省取运行环境，`sync.execute` 由调用方显式传 production）；② 脱敏 `detail.before/after/extra`（递归）；③ 结构规整（补 summary、校验 changed）；④ 持久化 `auditRepository.Insert`（显式路径在业务事务内）；⑤ ctx 打"已写审计"标志供中间件去重。
`Query` 职责：组装 WHERE（等值 env/action/resourceType/resourceId/actorId + 范围 created_at BETWEEN + 可选 keyword）+ 分页 + 排序（created_at 倒序默认，二级 id 倒序稳定）+ join admin_users 解析 operator（actor_id=0 ⇒ 系统占位）。

调用示例（game.update，显式同事务）：
```go
func (h *UpdateGameHandler) Handle(ctx context.Context, cmd UpdateGameCommand) error {
    return h.tx.WithinTx(ctx, func(ctx context.Context) error {
        before, _ := h.games.Get(ctx, env, cmd.GameID)
        after, err := h.games.Update(ctx, env, cmd)
        if err != nil { return err }
        return h.audit.Write(ctx, SecretAwareAuditInput{
            AuditWriteInput: AuditWriteInput{
                ActorID: actor.FromCtx(ctx), Action: "game.update",
                ResourceType: "game", ResourceID: cmd.GameID, // Env 缺省取运行环境
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
仓储 `infra/persistence/postgres/audit_repository.go`：`Insert(ctx,row) error`（唯一写方法）、`Query(ctx,q) ([]row, total, error)`；**不提供 Update/Delete**（编译期杜绝篡改）。

## 前端（审计日志页，路由 `/audit`，`apps/admin-web/src/views/audit/`，API `api/modules/audit.ts`）
页面结构：顶部 页标题 + EnvironmentBadge（当前运行环境常驻）；过滤区 FilterBar；列表区 Table（createdAt 倒序）；点击行右侧滑出详情抽屉。
- 过滤器（提交式，点查询才请求，变更分页/排序保留过滤条件）：env(select，字典 Environment)、action(可搜索 select，§action 清单/facets)、resourceType(可搜索 select)、operator(可搜索 select 管理员→提交 id)、timeRange(from-to 转 ISO-8601 UTC)、keyword(input)。env 默认不限（任一环境查历史），但顶部展示当前运行环境徽标。
- 列表列：时间(本地时区+hover UTC)、操作者(operator.displayName 兜底 actorId，系统显示 System)、动作(Tag 按动词色系 create绿/delete红/publish蓝/execute橙/hide灰)、资源类型、资源标识(可复制)、环境(Tag，production 高亮警示)、摘要(detail.summary 单行省略+hover)、操作(详情按钮)。
- 详情抽屉：顶部基础信息；中部 before/after 对照（仅 after=create 单列；仅 before=delete 单列；两者=update 左右对照、changed 高亮、未变弱化/折叠；密文显示 `******` 不解密）；底部 extra（syncJobId 等）+ request（IP/requestId/method/path）折叠 + "查看原始 JSON"。
- 状态态：加载(骨架屏)、空态("当前条件下无审计记录"+重置过滤)、错误态(非 403，重试+error.message)、权限态(403/无 audit.read 整页降级 + 菜单隐藏 `hasPerm('audit.read')`)、分页(total>pageSize 切页保留过滤)。
- 全只读：无任何写/删除按钮。可选增强：业务详情页"操作记录"深链 `/audit?resourceType=game&resourceId=g_1001`。

## 与公共能力 / 下游
- 上游依赖：鉴权中间件（actor_id 写入 + audit.read 校验）、env 上下文中间件（填 env）、脱敏能力（`00 §6.1` + 各模块 secret_fields_json）、统一包络/分页（`00 §7`）、字典/枚举（`00 §3` 前端过滤候选）。
- 下游：被**所有产生有意义写操作的模块调用**（`auth`～`sync`，`01 §7` 依赖图"audit 横切所有写操作"），各模块写用例须在命令服务中调 `AuditService.Write`；`sync.execute` 强制额外写一条审计（`01 §8` 第 6 步）。
- dashboard 只读消费审计（"最近 N 条关键操作""今日同步次数"）复用 `GET /audit-logs`，不新增写路径。

## 关键假设
- 不改 `000001` 的列：`audit_logs` 维持 7 列，索引由 v2 新增迁移追加；`actor_id` 无 FK（用户删后可追溯），系统操作用 `actor_id=0`（是否建专用"系统用户行"待定）。
- `action`/`resource_type` 保持 VARCHAR 开放、不做 DB CHECK，字典由 `00 §3` + 本文维护，前端下拉静态字典 + 可选 facets。
- `env` 是操作上下文标记不参与对象身份；`sync.execute` 写目标环境 `production`。
- `detail_json` 只存关键 before/after 非整行快照，密文统一脱敏；id 以字符串回传前端避免 JS 大整数精度问题。
- 审计页只读，无任何写/删/清理 API；归档/保留期/分区由 DBA 运维侧决策、API 不感知。
- 认证审计（`auth.login/logout/token_refresh`）、预览类取证（sync/fx.preview）、detail_json GIN 检索、导出/告警、operatorKeyword 模糊匹配 均为本期默认不做的可选扩展（先实现精确 operator）。
- 写入一致性：高危操作（publish/execute/approve）建议同事务强一致，普通 CUD 可中间件兜底最终一致。
