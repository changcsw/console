---
id: auth
code: "10"
title: 后台鉴权与 RBAC — 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [common]
code_paths:
  - services/admin-api/internal/domain/admin
  - services/admin-api/internal/domain/auth
  - services/admin-api/internal/transport/http/admin
  - apps/admin-web/src/stores/auth.ts
  - apps/admin-web/src/stores/permission.ts
  - apps/admin-web/src/views/login
  - apps/admin-web/src/views/system
---

# 10 · 后台鉴权与 RBAC — Compact Spec

> 代码生成用精简规格。完整背景/测试矩阵/未决问题见 `./README.md`。前置契约见 `../../00-common.md`（环境模型 §2、身份枚举 §3.1、密文 §6、统一包络/错误码/权限 §7、审计 §8、密码红线 §9）。
> 锁定决策 **D5**：JWT（access 默认 30m + refresh 默认 14d）+ RBAC；权限码格式 `resource.action`；密码登录（bcrypt）+ 飞书回调；本地 dev 允许 mock。

## 边界 / 红线
- 三套登录体系**物理/配置/代码隔离**：本模块=后台管理员登录；玩家自有账号认证(`account-auth`)、渠道强制登录(`channel-login`)与本模块**绝不混用**。
- 本模块所有 `admin_*` 表**平台级、不带 `env` 列**，置于共享 schema `platform`，全环境共享一套管理员/角色/权限。
- 不负责审计存储实现（仅调用 `audit` 写入并约定 `action` 命名）；不参与 sandbox→production 同步。
- 下游：所有业务模块(11–22) 接口经本模块鉴权/权限中间件保护，写操作挂权限码。

## 领域模型
领域包 `internal/domain/admin/`（管理员/角色/权限）+ `internal/domain/auth/`（令牌/身份）。应用服务 `AdminAuthService`、`AdminUserService`、`RoleService`、`PermissionService`。

| 聚合/实体 | 表 | 说明 |
| --- | --- | --- |
| `AdminUser`（根） | admin_users | 含 Identities、Roles |
| `AdminIdentity` | admin_identities | 一用户多身份(password/feishu) |
| `Role`（根） | admin_roles | 含 Permissions |
| `Permission`（字典） | admin_permissions | 权限码目录 |
| `UserRole` | admin_user_roles | 用户—角色 M:N |
| `RolePermission` | admin_role_permissions | 角色—权限 M:N |

值对象：`PermissionCode`(resource.action 不可变)、`IdentityType`(password/feishu)、`AdminUserStatus`(active/disabled)、`TokenPair{accessToken,refreshToken,expiresAt}`、`AuthContext{userId,displayName,roles[],permissions(set),environment}`、`Claims`(§Token)。

不变量：
1. `(identity_type, identity_key)` 全局唯一（同一第三方身份不可绑两个管理员）。
2. user_name / role_code / permission_code 各自全局唯一。
3. 关联唯一：`(user_id_ref,role_id_ref)`、`(role_id_ref,permission_id_ref)`。
4. **有效权限 = 用户所有角色授予权限码的并集（去重）**；用户不直接持权限，必经角色。
5. **禁用即拒绝**：status=disabled 任何身份不能登录；refresh 必须回库重校验 status。
6. password 身份 `credential_ciphertext` 存 **bcrypt 哈希**（非可逆），明文绝不落库。

## 数据模型（平台级 schema `platform`，全部不带 env 列）
公共列（所有表）：`id BIGSERIAL PK`、`created_at/updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`（关联表无 `updated_at`，仅 created_at）。来源 `migrations/000001_init.up.sql`。

### admin_users
| 列 | 类型 | 默认 | 约束 |
| --- | --- | --- | --- |
| user_name | VARCHAR(64) | — | NOT NULL, UNIQUE |
| display_name | VARCHAR(128) | — | NOT NULL |
| email | VARCHAR(128) | `''` | NOT NULL |
| status | VARCHAR(16) | `'active'` | NOT NULL；取值 active/disabled（应用层强制，DB 暂无 CHECK，建议补 `CHECK(status IN('active','disabled'))`） |

### admin_identities
| 列 | 类型 | 默认 | 约束 |
| --- | --- | --- | --- |
| user_id_ref | BIGINT | — | NOT NULL FK→admin_users(id) |
| identity_type | VARCHAR(16) | — | NOT NULL, `CHECK IN('password','feishu')` |
| identity_key | VARCHAR(128) | — | NOT NULL；password=用户名，feishu=`union_id` |
| credential_ciphertext | TEXT | `''` | NOT NULL；password=bcrypt 哈希，feishu=空或加密令牌 |

UNIQUE(identity_type, identity_key)；建议附 `(user_id_ref)` 索引。

### admin_roles
| 列 | 类型 | 约束 |
| --- | --- | --- |
| role_code | VARCHAR(64) | NOT NULL, UNIQUE（机器可读，如 super_admin） |
| role_name | VARCHAR(128) | NOT NULL |

### admin_permissions
| 列 | 类型 | 约束 |
| --- | --- | --- |
| permission_code | VARCHAR(128) | NOT NULL, UNIQUE（`resource.action`） |
| permission_name | VARCHAR(128) | NOT NULL |

权限码目录建议 seed 固化（见枚举节清单），运行时仅有限维护。

### admin_user_roles（关联，无 updated_at）
`user_id_ref BIGINT NOT NULL FK→admin_users(id)`、`role_id_ref BIGINT NOT NULL FK→admin_roles(id)`；UNIQUE(user_id_ref, role_id_ref)。

### admin_role_permissions（关联，无 updated_at）
`role_id_ref BIGINT NOT NULL FK→admin_roles(id)`、`permission_id_ref BIGINT NOT NULL FK→admin_permissions(id)`；UNIQUE(role_id_ref, permission_id_ref)。

关系：`admin_users 1─* admin_identities`；`admin_users *─* admin_roles`（经 user_roles）；`admin_roles *─* admin_permissions`（经 role_permissions）。`audit_logs.actor_id` = `admin_users.id`（逻辑外键，DB 未强制）。

## 枚举与默认
- `IdentityType`: password/feishu（无默认，DB CHECK 固化）。
- `AdminUserStatus`: active(默认)/disabled。
- 令牌默认（D5）：access TTL=30m(`ADMIN_JWT_ACCESS_TTL`)，refresh TTL=14d=336h(`ADMIN_JWT_REFRESH_TTL`)，签名 `HS256`，密钥 `ADMIN_JWT_SECRET`(必填禁硬编码)，issuer `admin-api`(`ADMIN_JWT_ISSUER`)，bcrypt cost=10(`ADMIN_BCRYPT_COST`)，飞书 mock=false(`ADMIN_FEISHU_MOCK`，仅 `APP_ENV=develop` 生效)。
- 兜底：新建管理员 status=active、email=`''`；时间戳 NOW()；分页 page=1/pageSize=20(max100)/sort=`-updatedAt`(00 §7.3)。

### 权限码命名与 seed 清单
格式 `resource.action`（全小写，resource 单数多词用 `_`，action 动词）。正则 `^[a-z0-9_]+\.[a-z0-9_]+$`。标准 action：read/write/delete/publish/approve/preview/execute。约定 `write` 覆盖新建+更新+启停（含删除，本模块不单列 `*.delete`）。

seed 固化清单（permission_code → 归属模块）：
game.read/write(game)、channel.read/write(channel)、account_auth.read/write(account-auth)、channel_login.read/write(channel-login)、plugin.read/write(feature-plugin)、product.read/write(product)、cashier.read/write/publish + fx.approve(cashier-template)、payment.read/write(payment)、snapshot.read/generate/publish(snapshot)、sync.preview/execute(sync)、audit.read(audit)、dashboard.read(dashboard)、system.read + admin_user.write + role.write + permission.write(auth)。

策略：读接口默认仅需登录、不强校验权限码；敏感读(`audit.read`/`system.read`)与写/危险操作必须挂权限码(00 §7.5)。

## 业务规则与状态机

### 管理员状态机
`create→active`；`active --disable--> disabled`（立即拒绝后续登录与刷新，进行中 access 自然到期前可能仍有效）；`disabled --enable--> active`。变更需 `admin_user.write` + 写审计。

### 密码登录（`POST /auth/login`）
```text
1. 校验 userName/password 必填非空。
2. 按 user_name 查 admin_users；不存在 → UNAUTHENTICATED（不区分用户不存在/密码错，防枚举）。
3. status=='active' 否则 UNAUTHENTICATED。
4. 查 admin_identities(type=password,user_id_ref)，取 bcrypt 哈希。
5. bcrypt.CompareHashAndPassword(hash,password) 不匹配 → UNAUTHENTICATED。
6. 解析角色/权限码集合（见权限解析）。
7. 签发 TokenPair(access 30m/refresh 14d)，access claims 含 userId/roles/perms。
8. 写审计 admin.login(identityType=password，不含密码)。
9. 返回 { accessToken, refreshToken, expiresAt, user{...} }。
```

### 飞书登录回调（`POST /auth/feishu/callback`）
```text
1-3. 前端引导授权→回调带 code 提交；后端用 code 换 app/user_access_token 再换用户信息(union_id/open_id,name,email)。
     dev+ADMIN_FEISHU_MOCK=true：接受 mock code(如 mock:alice) 直接映射 identity_key，跳过真实换取。
4. 按 (type=feishu, identity_key=union_id) 查 admin_identities：命中取 user_id_ref；未命中 → UNAUTHENTICATED（需先在 system 绑定，不自动开户）。
5. status=='active' 校验。
6. 解析角色/权限 → 签发 TokenPair → 写审计 admin.login(feishu) → 返回（同密码登录响应）。
```
飞书凭据如需落库走 00 §6 加密存 credential_ciphertext，响应脱敏。`identity_key` 锁定 `union_id`（单应用可退化 open_id）。mock 生产/沙箱永不开启。

### Token（签发/刷新/失效）
JWT Claims（access/refresh 共用，`typ` 区分）：
```json
{ "sub":"1", "typ":"access", "userName":"alice", "displayName":"Alice",
  "roles":["super_admin"], "perms":["game.read","sync.execute"],
  "iss":"admin-api", "iat":1750000000, "exp":1750001800, "jti":"a1b2c3" }
```
- 签发：同时发 access(typ=access,30m,含完整 roles/perms) 与 refresh(typ=refresh,14d,可仅 sub/jti/exp)，jti 不同。
- 刷新(`POST /auth/refresh`)：校验 refresh 签名/exp/typ=refresh → 回库校验 status=='active' → **重新解析角色/权限**签发新 access（必要时滚动新 refresh）。默认不轮换、不强制 denylist。
- 登出(`POST /auth/logout`)：无状态 JWT 默认客户端丢弃；若启用 denylist 把当前 access(+可选 refresh) jti 拉黑至 exp。当前 schema 无 refresh 持久化表，denylist 为可选加固。
- Token 状态：`issued →(exp) expired`；`issued →(logout/denylist) revoked`；`issued →(refresh) 派生新 access`。

### 权限解析到鉴权上下文
```text
loadAuthContext(userId):
  roles = admin_user_roles ⋈ admin_roles → role_code WHERE user_id_ref=userId
  perms = DISTINCT admin_user_roles ⋈ admin_role_permissions ⋈ admin_permissions
            → permission_code WHERE user_id_ref=userId
  return AuthContext{ userId, displayName, roles, permsSet=set(perms), environment=APP_ENV }
```
登录/刷新时解析写入 access claims；请求时中间件从 access 还原 AuthContext（不每请求回库，权限以令牌内 perms 为准，变更靠短 TTL+刷新生效）。`super_admin` 角色可 seed 全量权限或中间件短路放行（实现期定）。

### 鉴权中间件链（chi）
`Recoverer → RequestID/Logger → EnvContext → Authn(Bearer) → Authz(require perm) → Handler → Audit(写操作)`

| 中间件 | 职责 | 失败 |
| --- | --- | --- |
| EnvContext | 把 APP_ENV 注入 ctx + 响应头 `X-Environment`。**不逐请求 SET search_path**（由每环境独立连接池建连时钉死，01 §4.4） | — |
| Authn | 校验 Bearer access：签名/exp/typ=access；还原 AuthContext | 401 UNAUTHENTICATED |
| Authz(code) | 路由级声明所需权限码，校验 `code ∈ ctx.perms` | 403 FORBIDDEN |
| Audit | 写操作成功后写 audit_logs，不阻断主流程 | 仅告警 |

豁免鉴权：`POST /auth/login`、`/auth/refresh`、`/auth/feishu/callback`、健康检查。`GET /me` 需 Authn 但无需特定权限码。

环境：APP_ENV 决定当前环境(缺省 develop)，`search_path=<env>,platform` 建连钉死；每响应附 `X-Environment`；audit_logs.env 取当前运行环境（platform 中保留 env 过滤列的特例）。

## 后端 API（前缀 `/api/admin`，包络/错误码 00 §7；`—`=仅需登录）

| # | Method | Path | 权限码 | 说明 |
| --- | --- | --- | --- | --- |
| 1 | POST | `/auth/login` | 公开 | 密码登录 |
| 2 | POST | `/auth/refresh` | 公开(凭 refresh) | 刷新 access |
| 3 | POST | `/auth/logout` | 需登录 | 登出 |
| 4 | POST | `/auth/feishu/callback` | 公开 | 飞书回调登录 |
| 5 | GET | `/me` | — | 当前用户/权限/环境 |
| 6 | GET | `/system/admin-users` | system.read | 管理员列表 |
| 7 | POST | `/system/admin-users` | admin_user.write | 新建管理员 |
| 8 | GET | `/system/admin-users/{id}` | system.read | 管理员详情 |
| 9 | PATCH | `/system/admin-users/{id}` | admin_user.write | 更新(含启停) |
| 10 | PUT | `/system/admin-users/{id}/roles` | admin_user.write | 分配角色 |
| 11 | POST | `/system/admin-users/{id}/reset-password` | admin_user.write | 重置密码 |
| 12 | GET | `/system/roles` | system.read | 角色列表 |
| 13 | POST | `/system/roles` | role.write | 新建角色 |
| 14 | PATCH | `/system/roles/{id}` | role.write | 更新角色 |
| 15 | DELETE | `/system/roles/{id}` | role.write | 删除角色 |
| 16 | PUT | `/system/roles/{id}/permissions` | role.write | 配置角色权限 |
| 17 | GET | `/system/permissions` | system.read | 权限码目录 |
| 18 | POST | `/system/permissions` | permission.write | 新建权限码 |
| 19 | DELETE | `/system/permissions/{id}` | permission.write | 删除权限码 |

错误码全复用全局集：`UNAUTHENTICATED`(401)、`FORBIDDEN`(403)、`NOT_FOUND`(404)、`VALIDATION_FAILED`(400)、`CONFLICT`(409)、`INTERNAL`(500)。

### DTO 细节（含必填/默认/校验）

**login** 请求 `userName`(必填,1–64)、`password`(必填,1–128)；响应 `{accessToken, refreshToken, expiresAt(ISO), user{userId,userName,displayName,roles[],permissions[]}}`。错误 VALIDATION_FAILED/UNAUTHENTICATED(凭据错或账号禁用)/INTERNAL。

**refresh** 请求 `refreshToken`(必填,有效 typ=refresh JWT)；响应 `{accessToken, refreshToken, expiresAt}`（refreshToken 是否更换取决于轮换）。错误 VALIDATION_FAILED/UNAUTHENTICATED/INTERNAL。

**logout** 请求 `refreshToken`(可选,默认`""`,提供则一并拉黑)；响应 `{loggedOut:true}`；审计 admin.logout。

**feishu/callback** 请求 `code`(必填,dev+mock 接受 `mock:<userName>`)、`state`(可选`""` CSRF 回显)、`redirectUri`(可选`""`)；响应同 login；错误含 UNAUTHENTICATED(校验失败/未绑定/禁用)、INTERNAL(飞书不可用)。

**GET /me** 响应 `{userId,userName,displayName,email,status,roles[],permissions[],identities[{identityType,identityKey(脱敏)}],environment}`。

**GET /system/admin-users** Query `page/pageSize/sort/keyword(userName/displayName/email)/status(active|disabled)`；items `{id,userName,displayName,email,status,roles[],createdAt,updatedAt}` + page/pageSize/total。

**POST /system/admin-users** DTO：
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| userName | string | 是 | — | 1–64，唯一 |
| displayName | string | 是 | — | 1–128 |
| email | string | 否 | `""` | ≤128，非空时 format=email |
| status | string | 否 | active | enum active/disabled |
| password | string | 否 | `""` | 提供则建 password 身份；8–128 |
| roleIds | number[] | 否 | `[]` | 须存在 |
| feishuKey | string | 否 | `""` | 提供则建 feishu 身份(identity_key=union_id) |

响应=用户详情；错误 CONFLICT(userName/身份已存在)；审计 admin_user.create。

**GET /system/admin-users/{id}** 响应 `{id,userName,displayName,email,status,roles[{id,roleCode,roleName}],identities[{identityType,identityKey(脱敏)}],permissions[],createdAt,updatedAt}`；NOT_FOUND。

**PATCH /system/admin-users/{id}** 可选(≥1)：`displayName`(≤128)、`email`(≤128,email)、`status`(enum)；审计 admin_user.update(status before/after)。

**PUT /system/admin-users/{id}/roles** `roleIds`(必填,全量覆盖,每个须存在)；语义删旧建新写 admin_user_roles；响应 `{id,roles[]}`；审计 admin_user.assign_roles(before/after)。

**POST /system/admin-users/{id}/reset-password** `newPassword`(必填,8–128)；为 password 身份重置 bcrypt 哈希(无则建,key=userName)；明文不落库不入审计；响应 `{id,reset:true}`；审计 admin_user.reset_password。

**GET /system/roles** Query 分页 + `keyword(roleCode/roleName)`；items `{id,roleCode,roleName,permissionCount,createdAt,updatedAt}`。

**POST /system/roles** DTO：`roleCode`(必填,1–64,唯一,建议 `[a-z0-9_]+`)、`roleName`(必填,1–128)、`permissionIds`(可选`[]`,须存在)；响应角色详情含 permissions[]；CONFLICT(roleCode 已存在)；审计 role.create。

**PATCH /system/roles/{id}** 可选 `roleName`(≤128)；`roleCode` 不可改；审计 role.update。

**DELETE /system/roles/{id}** 默认**拒绝删除被用户引用的角色**(CONFLICT，details.userCount)，要求先解绑；否则删除并解绑 role_permissions/user_roles；响应 `{id,deleted:true}`；审计 role.delete。

**PUT /system/roles/{id}/permissions** `permissionIds`(必填,全量覆盖,须存在)；写 admin_role_permissions；生效需令牌刷新；响应 `{id,permissions[]}`；审计 role.assign_permissions(before/after)。

**GET /system/permissions** Query 分页 + `keyword` + `all=true`(全量)；items `{id,permissionCode,permissionName,createdAt,updatedAt}`。

**POST /system/permissions** `permissionCode`(必填,≤128,唯一,匹配 `^[a-z0-9_]+\.[a-z0-9_]+$`)、`permissionName`(必填,1–128)；CONFLICT/VALIDATION_FAILED(格式)；审计 permission.create。

**DELETE /system/permissions/{id}** 被角色引用默认拒绝(CONFLICT)，先解绑；响应 `{id,deleted:true}`；审计 permission.delete。

代表性错误响应包络：`{ "error": { "code":"UNAUTHENTICATED", "message":"用户名或密码错误", "details":[] } }`；`{ "error": { "code":"CONFLICT", "message":"userName already exists", "details":[{"field":"userName"}] } }`。

## 应用服务 / 仓储（internal/app，command/query 分；DTO 在 internal/app/dto）

`AdminAuthService`：`Login(LoginCmd{userName,password})→TokenPair+UserView`、`FeishuCallback(FeishuCallbackCmd{code,state,redirectUri})→TokenPair+UserView`、`Refresh(RefreshCmd{refreshToken})→TokenPair`、`Logout(LogoutCmd{accessJti,refreshToken?})`、`Me(userId)→MeView`、`LoadAuthContext(userId)→AuthContext`。依赖 AdminUserRepository/AdminIdentityRepository/RolePermissionQuery/crypto/bcrypt/jwt/config/FeishuClient(含 mock)。

`AdminUserService`：ListUsers/GetUser(query)、CreateUser/UpdateUser/AssignRoles/ResetPassword(command)。
`RoleService`：ListRoles/GetRole(query)、CreateRole/UpdateRole/DeleteRole/AssignPermissions(command)。
`PermissionService`：ListPermissions(query)、CreatePermission/DeletePermission(command)。

仓储（窄，单聚合；跨表编排放 app 层）：
```go
type AdminUserRepository interface {
    Create(ctx, *AdminUser) error
    Update(ctx, *AdminUser) error
    FindByID(ctx, id int64) (*AdminUser, error)
    FindByUserName(ctx, userName string) (*AdminUser, error)
    List(ctx, filter AdminUserFilter) ([]AdminUser, int, error)
    ReplaceRoles(ctx, userID int64, roleIDs []int64) error
}
type AdminIdentityRepository interface {
    FindByTypeKey(ctx, t IdentityType, key string) (*AdminIdentity, error)
    ListByUser(ctx, userID int64) ([]AdminIdentity, error)
    Upsert(ctx, *AdminIdentity) error
}
type RoleRepository interface { // Create/Update/Delete/FindByID/List
    ReplacePermissions(ctx, roleID int64, permIDs []int64) error
}
type PermissionRepository interface { // Create/Delete/FindByID/List
    ListCodesByUser(ctx, userID int64) ([]string, error) // 权限解析
}
```

## 前端信息架构（Vue3+Vite+TS+Pinia+Router+Element Plus）
- **登录页**(`views/login/`，`/login`)：与玩家登录 UI 完全隔离；密码/飞书 Tab；密码登录提交 `/auth/login`，飞书跳授权→回调提交 `/auth/feishu/callback`；成功写 auth store 跳 redirect 或 `/dashboard`；态：loading/凭据错误行内红字/飞书未绑定提示/网络 toast。
- **Pinia**：`auth`(accessToken/refreshToken/user/expiresAt；login/feishuLogin/refresh/logout/持久化/access 临期自动续期)、`permission`(perms:Set/roles[]；hasPerm/hasAnyPerm/按权限过滤路由菜单)、`app`(environment，由 /me 或 X-Environment 写入，常驻 EnvironmentBadge)。
- **API 客户端**(`api/http.ts`)：请求注入 Bearer；响应解包 `{data}`；401→用 refresh 自动续期一次，失败清 auth 跳 /login；403→toast 无权限；读 X-Environment 写 app store。
- **路由守卫**(`router/`)：meta.perm 声明权限码；beforeEach 未登录跳 `/login?redirect`，有 meta.perm 且 hasPerm 假跳 403/隐藏菜单；菜单按 hasPerm 过滤；写按钮指令 `v-perm="'role.write'"`。
- **`/system`**(需 system.read)：管理员管理(列表/抽屉新建编辑/角色分配/重置密码弹窗/启停，挂 admin_user.write)、角色管理(列表/抽屉/权限多选树按 resource 分组，挂 role.write)、权限管理(目录列表/新建删除，挂 permission.write)。

## 与公共能力的关系
- 密文(00 §6)：password 存 bcrypt 哈希、feishu 第三方令牌 AES-GCM 加密；响应**绝不回明文密码/哈希/密钥**，identityKey 脱敏(如 `on_****1a2b`)。
- 审计(00 §8)：action 与权限码同源——`admin.login`/`admin.login_failed`(可选)/`admin.logout`/`admin_user.{create,update,assign_roles,reset_password}`/`role.{create,update,delete,assign_permissions}`/`permission.{create,delete}`；audit_logs 字段 `actor_id/action/resource_type/resource_id/env/detail_json(before/after,脱敏)/created_at`；登录类 actor_id=登录主体本人，用户不存在可记 0/省略。
- 环境(00 §2)：admin_* 无 env；audit_logs.env 记录操作发生环境；/me.environment 与 X-Environment 透出当前环境。

## 关键假设
- refresh 为**无状态 JWT**（无持久化表）；本期默认**不启用 denylist/轮换**，登出=客户端丢弃 + 短 TTL 自然过期；"禁用即时踢下线/强制下线"需另补会话表与迁移。
- 权限以 access 内嵌 `perms` 为准，变更对已登录用户在刷新(≤30m)后生效（可接受短窗口延迟）。
- 飞书 identity_key=`union_id`；未绑定身份默认拒绝登录（不自动开户），需先在 /system 绑定。
- `super_admin` 为约定全权角色（seed 固化或中间件短路，二选一实现期定）。
- mock 仅 `APP_ENV=develop` 生效，否则忽略并告警；生产/沙箱永不开启。
- 角色/权限被引用时默认拒绝删除（CONFLICT），要求先解绑。
