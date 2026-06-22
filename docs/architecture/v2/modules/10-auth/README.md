---
id: auth
code: "10"
title: 后台鉴权与 RBAC
status: target
code_paths:
  - services/admin-api/internal/domain/admin
  - services/admin-api/internal/domain/auth
  - services/admin-api/internal/transport/http/admin
  - apps/admin-web/src/stores/auth.ts
  - apps/admin-web/src/stores/permission.ts
  - apps/admin-web/src/views/login
  - apps/admin-web/src/views/system
depends_on: [common]
impacts: [testing]
children: []
---

# 10 · 后台鉴权与 RBAC

> 本模块文档默认遵循 `../../00-common.md`（公共契约）与 `../../01-structure.md`（技术栈/分层）。
> 与公共部分冲突时以公共部分为准；本文只在其基础上**追加**本模块私有约定。
> 锁定决策 **D5**：JWT（access 默认 30 分钟 + refresh 默认 14 天）+ RBAC；权限码格式 `resource.action`；支持**密码登录（bcrypt）**与**飞书回调**；本地 dev 允许 mock。
> 红线（必须贯彻）：**后台管理员登录绝不与玩家登录配置混在一起**。本模块涉及的 `admin_*` 表是**平台级**，**全部不带 `env` 列**（全环境共享一套管理员/角色/权限）。

---

## 1. 模块概述与边界

### 1.1 模块职责

后台鉴权与 RBAC 模块是整个发行控制台的**统一入口与访问控制中枢**，负责：

1. **身份认证（Authentication）**：验证"你是谁"。支持两类身份：
   - **密码登录**：用户名 + 密码（bcrypt 校验）。
   - **飞书登录**：飞书 OAuth 授权回调，换取并绑定飞书身份。
   - 本地开发环境允许 **mock 登录**（绕过真实飞书，便于联调）。
2. **令牌签发与生命周期管理（Token）**：签发 `accessToken`（短期，默认 30 分钟）与 `refreshToken`（长期，默认 14 天）；支持刷新（refresh）与登出（logout）。
3. **授权与权限解析（Authorization / RBAC）**：用户 → 角色 → 权限码（`resource.action`）的三层模型；登录后把用户的权限码集合解析进**鉴权上下文**，供权限中间件与前端按钮级控制使用。
4. **鉴权与权限中间件**：为除登录类接口外的所有 `/api/admin/**` 接口提供 `Bearer` 令牌校验、权限码校验、当前运行环境（`environment`）上下文注入。
5. **管理员/角色/权限的管理后台**：在 `/api/admin/system` 下提供管理员、角色、权限的读写接口与对应前端页面。

### 1.2 边界（不做什么）

- **不负责玩家（游戏内用户）登录**：玩家登录策略、渠道登录配置（`game_channel_login_configs`）、自有账号认证（`game_account_auth_configs`）属于 `account-auth` / `channel-login`，与本模块**物理隔离、配置隔离、代码隔离**。这是全局红线第 1 条。
- **不负责业务数据的 `env` 维度**：`admin_*` 表全部为平台级，无 `env` 列。本模块只负责"读取当前运行环境并注入上下文 / 在 `/api/admin/me` 展示"，不参与 `sandbox -> production` 同步。
- **不负责审计存储的实现**：审计表 `audit_logs` 与查询页属于 `audit`；本模块只**调用**审计写入能力，并约定本模块的 `action` 命名。

### 1.3 上下游依赖

| 方向 | 依赖对象 | 说明 |
| --- | --- | --- |
| 依赖（上游） | `00 §6 密文` / `infra/crypto`（AES-GCM） | 飞书 `credential_ciphertext`（如刷新令牌/凭据）加密落库 |
| 依赖（上游） | `bcrypt` | 密码哈希与校验（密码身份） |
| 依赖（上游） | `github.com/golang-jwt/jwt/v5` | JWT 签发与校验 |
| 依赖（上游） | `00 §2 / 01 §2` 环境模型（`APP_ENV`） | 注入并展示当前运行 `environment` |
| 依赖（上游） | `infra/config` | 读取 JWT 密钥、TTL、飞书 OAuth 配置、mock 开关 |
| 被依赖（下游） | **所有其它业务模块（11–22）** | 全部接口经本模块的鉴权/权限中间件保护；写操作挂权限码 |
| 被依赖（下游） | `21 审计日志` | 本模块写 `audit_logs`（登录、用户/角色/权限变更） |
| 被依赖（下游） | 前端 `stores/auth`、`stores/permission`、`stores/app`、路由守卫 | 消费 `/api/admin/me` 与登录响应 |

---

## 2. 领域模型与聚合

领域包：`internal/domain/admin/`（管理员、角色、权限）与 `internal/domain/auth/`（令牌、身份）。应用服务：`AdminAuthService`、`AdminUserService`、`RoleService`、`PermissionService`。

### 2.1 聚合与实体

| 聚合 / 实体 | 落地表 | 说明 |
| --- | --- | --- |
| `AdminUser`（聚合根） | `admin_users` | 管理员主体。聚合内含其 `Identities`、`Roles` |
| `AdminIdentity`（实体） | `admin_identities` | 一个管理员可有多种登录身份（password / feishu），归属于 `AdminUser` |
| `Role`（聚合根） | `admin_roles` | 角色；聚合内含其 `Permissions` |
| `Permission`（实体/字典） | `admin_permissions` | 权限码目录（`resource.action`），平台级权限字典 |
| `UserRole`（关联） | `admin_user_roles` | 用户—角色 多对多 |
| `RolePermission`（关联） | `admin_role_permissions` | 角色—权限 多对多 |

### 2.2 值对象

| 值对象 | 说明 |
| --- | --- |
| `PermissionCode` | 形如 `resource.action` 的字符串值对象；不可变；命名规范见 §4.4 |
| `IdentityType` | 枚举 `password` / `feishu`（见 `00 §3.1`） |
| `AdminUserStatus` | 枚举 `active` / `disabled`（见 `00 §3.1`） |
| `TokenPair` | `{ accessToken, refreshToken, expiresAt }` |
| `AuthContext` | 请求级鉴权上下文：`{ userId, displayName, roles[], permissions(set), environment }` |
| `Claims` | JWT 载荷值对象（见 §5.3） |

### 2.3 不变量（业务恒等约束）

1. **身份唯一**：`admin_identities` 的 `(identity_type, identity_key)` 全局唯一；同一第三方身份不能绑定到两个管理员。
2. **用户名唯一**：`admin_users.user_name` 全局唯一。
3. **角色码唯一**：`admin_roles.role_code` 全局唯一。
4. **权限码唯一**：`admin_permissions.permission_code` 全局唯一。
5. **关联唯一**：`admin_user_roles (user_id_ref, role_id_ref)` 唯一；`admin_role_permissions (role_id_ref, permission_id_ref)` 唯一。
6. **权限解析路径恒定**：用户的有效权限 = 其所有角色所授予权限码的**并集**（去重）。用户不直接持有权限，必须经由角色。
7. **禁用即拒绝**：`status='disabled'` 的管理员，任何身份都不能完成登录；已签发的 `accessToken` 在到期前可能仍有效（见 §5 与 §11 未决问题），刷新（refresh）必须重新校验 `status`。
8. **密码身份的凭据**：`password` 身份的 `credential_ciphertext` 存储 **bcrypt 哈希**（非可逆密文）；明文密码绝不落库（红线 `00 §9`）。
9. **平台级无 env**：本模块所有表无 `env`；管理员/角色/权限对三个运行环境是同一套数据。

---

## 3. 数据模型（逐表逐字段）

> 数据来源：`services/admin-api/migrations/000001_init.up.sql`。**以下所有表均不带 `env` 列**（平台级，`00 §2.2` 已列入"不带 env 的平台级表"）。
> 命名：列 `snake_case`；时间戳 `created_at` / `updated_at` 默认 `NOW()`。

### 3.1 `admin_users`（管理员）

| 列名 | 类型 | 可空 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | 自增 | PRIMARY KEY | 主键 |
| `user_name` | VARCHAR(64) | 否 | — | `UNIQUE (user_name)` | 登录用户名（不变量 2） |
| `display_name` | VARCHAR(128) | 否 | — | — | 展示名 |
| `email` | VARCHAR(128) | 否 | `''` | — | 邮箱（可空字符串） |
| `status` | VARCHAR(16) | 否 | `'active'` | 见说明 | 管理员状态，取值 `active` / `disabled`（`AdminUserStatus`） |
| `created_at` | TIMESTAMPTZ | 否 | `NOW()` | — | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 否 | `NOW()` | — | 更新时间 |

- **索引/唯一**：`UNIQUE (user_name)`；主键索引 `id`。
- **不带 env**：是。
- **备注**：迁移 `000001` 未对 `status` 写 DB 级 `CHECK`，取值约束由应用层（`AdminUserStatus`）强制；建议后续迁移补 `CHECK (status IN ('active','disabled'))`（见 §11）。

### 3.2 `admin_identities`（管理员身份）

| 列名 | 类型 | 可空 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | 自增 | PRIMARY KEY | 主键 |
| `user_id_ref` | BIGINT | 否 | — | `REFERENCES admin_users(id)` | 归属管理员（FK） |
| `identity_type` | VARCHAR(16) | 否 | — | `CHECK (identity_type IN ('password','feishu'))` | 身份类型（`IdentityType`） |
| `identity_key` | VARCHAR(128) | 否 | — | 复合 `UNIQUE (identity_type, identity_key)` | 身份标识：password=用户名/账号；feishu=飞书 `union_id`（或 `open_id`，见 §5.4 决策） |
| `credential_ciphertext` | TEXT | 否 | `''` | — | 凭据：password=**bcrypt 哈希**；feishu=空串或加密后的第三方令牌引用（`00 §6` 脱敏） |
| `created_at` | TIMESTAMPTZ | 否 | `NOW()` | — | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 否 | `NOW()` | — | 更新时间 |

- **索引/唯一**：`UNIQUE (identity_type, identity_key)`；建议附加普通索引 `(user_id_ref)` 便于按用户查身份（见 §11）。
- **CHECK**：`identity_type IN ('password','feishu')`。
- **不带 env**：是。

### 3.3 `admin_roles`（角色）

| 列名 | 类型 | 可空 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | 自增 | PRIMARY KEY | 主键 |
| `role_code` | VARCHAR(64) | 否 | — | `UNIQUE (role_code)` | 角色码（机器可读，如 `super_admin`） |
| `role_name` | VARCHAR(128) | 否 | — | — | 角色显示名 |
| `created_at` | TIMESTAMPTZ | 否 | `NOW()` | — | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 否 | `NOW()` | — | 更新时间 |

- **索引/唯一**：`UNIQUE (role_code)`。
- **不带 env**：是。

### 3.4 `admin_permissions`（权限码目录）

| 列名 | 类型 | 可空 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | 自增 | PRIMARY KEY | 主键 |
| `permission_code` | VARCHAR(128) | 否 | — | `UNIQUE (permission_code)` | 权限码 `resource.action`（见 §4.4） |
| `permission_name` | VARCHAR(128) | 否 | — | — | 权限显示名/描述 |
| `created_at` | TIMESTAMPTZ | 否 | `NOW()` | — | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 否 | `NOW()` | — | 更新时间 |

- **索引/唯一**：`UNIQUE (permission_code)`。
- **不带 env**：是。
- **备注**：权限码目录建议由 seed 固化（§4.4 清单），运行时管理后台仅做有限维护。

### 3.5 `admin_user_roles`（用户—角色关联）

| 列名 | 类型 | 可空 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | 自增 | PRIMARY KEY | 主键 |
| `user_id_ref` | BIGINT | 否 | — | `REFERENCES admin_users(id)` | 用户（FK） |
| `role_id_ref` | BIGINT | 否 | — | `REFERENCES admin_roles(id)` | 角色（FK） |
| `created_at` | TIMESTAMPTZ | 否 | `NOW()` | 复合 `UNIQUE (user_id_ref, role_id_ref)` | 关联建立时间 |

- **索引/唯一**：`UNIQUE (user_id_ref, role_id_ref)`。
- **无 `updated_at`**（关联表仅记录建立时间，更新即"删旧建新"）。
- **不带 env**：是。

### 3.6 `admin_role_permissions`（角色—权限关联）

| 列名 | 类型 | 可空 | 默认值 | 约束 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | 自增 | PRIMARY KEY | 主键 |
| `role_id_ref` | BIGINT | 否 | — | `REFERENCES admin_roles(id)` | 角色（FK） |
| `permission_id_ref` | BIGINT | 否 | — | `REFERENCES admin_permissions(id)` | 权限（FK） |
| `created_at` | TIMESTAMPTZ | 否 | `NOW()` | 复合 `UNIQUE (role_id_ref, permission_id_ref)` | 关联建立时间 |

- **索引/唯一**：`UNIQUE (role_id_ref, permission_id_ref)`。
- **无 `updated_at`**。
- **不带 env**：是。

### 3.7 关系图（文字版）

```text
admin_users 1───* admin_identities         （一个用户多种登录身份）
admin_users *───* admin_roles  （经 admin_user_roles）
admin_roles *───* admin_permissions （经 admin_role_permissions）

有效权限(user) = ⋃ { role.permissions | role ∈ user.roles }
```

### 3.8 与本模块相关的其它表引用

- `audit_logs.actor_id`（BIGINT）= 操作者 `admin_users.id`（逻辑外键，DB 层未强制 FK）。本模块的登录/登出/用户角色变更均写此表（见 §9）。
- `cashier_fx_sync_runs.reviewed_by`、`sync_jobs.operator_id` 等也以 `admin_users.id` 作为操作者标识（跨模块，本模块不维护）。

---

## 4. 枚举与默认值清单（集中复列）

> 本节是本模块的**唯一事实来源**，与 `00 §3.1` 保持一致；前端 `dictionary` store 与后端 `domain/common`、`domain/admin` 常量必须与此完全一致。

### 4.1 `IdentityType`（身份类型）

| 取值 | 含义 | 默认 |
| --- | --- | --- |
| `password` | 用户名+密码（bcrypt） | — |
| `feishu` | 飞书 OAuth 身份 | — |

- 无默认值（创建身份时必须显式给定）。DB `CHECK` 已固化在 `admin_identities.identity_type`。

### 4.2 `AdminUserStatus`（管理员状态）

| 取值 | 含义 | 默认 |
| --- | --- | --- |
| `active` | 启用，可登录 | **是（默认）** |
| `disabled` | 禁用，拒绝登录与刷新 | — |

- 默认值 `active`（`admin_users.status DEFAULT 'active'`）。

### 4.3 令牌有效期与相关默认值（D5）

| 配置项 | 默认值 | 说明 | 配置来源（env） |
| --- | --- | --- | --- |
| access token TTL | **30 分钟** | `accessToken` 过期时长 | `ADMIN_JWT_ACCESS_TTL`（如 `30m`） |
| refresh token TTL | **14 天** | `refreshToken` 过期时长（14d = 336h） | `ADMIN_JWT_REFRESH_TTL`（如 `336h`） |
| JWT 签名算法 | `HS256` | 对称签名（密钥来自配置/KMS） | — |
| JWT 签名密钥 | — | 必填，禁止硬编码 | `ADMIN_JWT_SECRET` |
| JWT issuer | `admin-api` | `iss` 声明 | `ADMIN_JWT_ISSUER`（可选） |
| 飞书 mock 开关 | `false`（仅 dev 允许 `true`） | 本地绕过真实飞书回调 | `ADMIN_FEISHU_MOCK`（仅 `APP_ENV=develop` 生效） |
| bcrypt cost | `10` | 密码哈希代价因子 | `ADMIN_BCRYPT_COST`（可选） |

- **password / feishu / mock 优先级**：生产/沙箱禁止 mock；`ADMIN_FEISHU_MOCK=true` 仅当 `APP_ENV=develop` 才被接受，否则服务启动报错或忽略并告警（见 §11）。

### 4.4 权限码命名规范与样例清单

**格式**：`resource.action`，全小写，`resource` 为业务资源单数名（多词用 `_`），`action` 为动作动词。

**标准 action 动词集合**（建议统一收敛）：

| action | 语义 |
| --- | --- |
| `read` | 查看/列表/详情（GET） |
| `write` | 新建+更新+启停（POST/PUT/PATCH，含创建与修改） |
| `delete` | 删除/移除关联 |
| `publish` | 发布（版本/快照） |
| `approve` | 审核通过（汇率等） |
| `preview` | 同步预览 |
| `execute` | 同步执行（危险） |

> 约定：除非资源需要把"删除"与"写"分权，否则 `write` 同时覆盖新建与更新；需要更细分时再拆 `create`/`update`。本模块采用 `write` 粗粒度，管理员/角色/权限管理的删除并入 `*.write`，不再单列 `*.delete`（见 §6）。

**权限码样例清单（建议 seed 固化于 `admin_permissions`）**：

| permission_code | permission_name | 归属模块 |
| --- | --- | --- |
| `game.read` | 游戏-查看 | 11 |
| `game.write` | 游戏-编辑 | 11 |
| `channel.read` | 渠道实例-查看 | 12 |
| `channel.write` | 渠道实例-编辑 | 12 |
| `account_auth.read` | 自有账号认证-查看 | 13 |
| `account_auth.write` | 自有账号认证-编辑 | 13 |
| `channel_login.read` | 渠道登录-查看 | 14 |
| `channel_login.write` | 渠道登录-编辑 | 14 |
| `product.read` | 商品/IAP-查看 | 15 |
| `product.write` | 商品/IAP-编辑 | 15 |
| `cashier.read` | 收银台模板-查看 | 16 |
| `cashier.write` | 收银台模板-编辑 | 16 |
| `cashier.publish` | 收银台版本-发布 | 16 |
| `fx.approve` | 汇率同步-审核 | 16 |
| `payment.read` | 支付路由-查看 | 18 |
| `payment.write` | 支付路由-编辑 | 18 |
| `snapshot.read` | 配置快照-查看 | 19 |
| `snapshot.generate` | 配置快照-生成 | 19 |
| `snapshot.publish` | 配置快照-发布 | 19 |
| `sync.preview` | 同步-预览 | 20 |
| `sync.execute` | 同步-执行 | 20 |
| `audit.read` | 审计日志-查看 | 21 |
| `system.read` | 系统管理-查看（管理员/角色/权限只读） | 10 |
| `admin_user.write` | 管理员-管理（增改禁用/分配角色/重置密码） | 10 |
| `role.write` | 角色-管理（增改删/配权限） | 10 |
| `permission.write` | 权限码目录-维护 | 10 |

> 说明：`*.read` 类是否进权限码目录、是否做接口级强校验，取决于策略。本模块默认**读接口要求登录但不强制特定权限码**（除敏感读如 `audit.read`、`system.read`）；**写/危险操作必须挂权限码**（`00 §7.5`）。

### 4.5 其它默认值兜底（沿用 `00 §10`）

- 新建管理员 `status` 默认 `active`，`email` 默认 `''`。
- 时间戳默认 `NOW()`。
- 分页：`page=1`、`pageSize=20`（最大 `100`），排序缺省 `-updatedAt`（`00 §7.3`）。

---

## 5. 业务规则与状态机

### 5.1 管理员状态机（AdminUserStatus）

```text
        create
          │
          ▼
       active ──disable──▶ disabled
          ▲                   │
          └──────enable───────┘
```

- 新建管理员默认 `active`。
- `active --disable--> disabled`：禁用后立即拒绝该用户后续**登录**与**刷新**；进行中的 `accessToken` 在过期前可能仍有效（无状态 JWT 的固有特性，见 §11 加固建议）。
- `disabled --enable--> active`：恢复登录能力。
- 状态变更需 `admin_user.write` 权限并写审计。

### 5.2 登录流程（密码 / 飞书 / mock）

#### 5.2.1 密码登录（`POST /api/admin/auth/login`）

```text
1. 入参校验：userName、password 必填且非空。
2. 按 user_name 查 admin_users；不存在 -> UNAUTHENTICATED（不区分"用户不存在/密码错"，避免枚举）。
3. 校验 status == 'active'；否则 -> UNAUTHENTICATED（message 提示账号被禁用，按安全策略可模糊化）。
4. 查 admin_identities (identity_type='password', user_id_ref=该用户)；取 credential_ciphertext(bcrypt 哈希)。
5. bcrypt.CompareHashAndPassword(hash, password)；不匹配 -> UNAUTHENTICATED。
6. 解析该用户的角色与权限码集合（§5.5）。
7. 签发 TokenPair（access 30m / refresh 14d），claims 含 userId/roles/permissions（§5.3）。
8. 写审计 admin.login（detail 含 identityType=password，不含密码）。
9. 返回 200 + { accessToken, refreshToken, expiresAt, user{...} }。
```

#### 5.2.2 飞书登录回调（`POST /api/admin/auth/feishu/callback`）

```text
1. 前端引导用户跳转飞书授权页（client_id/redirect_uri/state），用户同意后飞书回调带回 code。
2. 前端把 code(+state) 提交到本接口。
3. 后端用 code 向飞书换 app_access_token / user_access_token，再换取用户信息(union_id/open_id, name, email)。
   - 本地 dev 且 ADMIN_FEISHU_MOCK=true：跳过真实换取，按 mock 映射直接得到 identity_key。
4. 按 (identity_type='feishu', identity_key=union_id) 查 admin_identities：
   - 命中 -> 取其 user_id_ref 对应 admin_user。
   - 未命中 -> 默认拒绝（UNAUTHENTICATED，需管理员先在 system 后台绑定）。是否允许"自动开户"是策略，见 §11。
5. 校验 admin_user.status == 'active'。
6. 解析角色/权限 -> 签发 TokenPair -> 写审计 admin.login(identityType=feishu) -> 返回（响应结构同密码登录）。
```

- 飞书相关密钥/令牌如需落库，走 `00 §6` 密文加密存 `credential_ciphertext`，响应一律脱敏。

#### 5.2.3 本地 mock（仅 dev）

- 仅当 `APP_ENV=develop` 且 `ADMIN_FEISHU_MOCK=true` 时，`feishu/callback` 接受约定的 mock `code`（如 `mock:alice`）直接映射到既有飞书身份，便于无飞书凭据联调。
- 生产/沙箱永不开启；红线：不得把 mock 路径泄漏到非 dev 环境。

### 5.3 Token 签发 / 刷新 / 失效

#### JWT Claims（access 与 refresh 共用结构，`typ` 区分）

```json
{
  "sub": "1",
  "typ": "access",            // 或 "refresh"
  "userName": "alice",
  "displayName": "Alice",
  "roles": ["super_admin"],
  "perms": ["game.read", "game.write", "sync.execute"],
  "iss": "admin-api",
  "iat": 1750000000,
  "exp": 1750001800,           // access: iat+30m；refresh: iat+14d
  "jti": "a1b2c3..."           // 令牌唯一 ID，用于登出/denylist
}
```

- **签发（login / feishu callback）**：同时签发 access（`typ=access`，30m）与 refresh（`typ=refresh`，14d），二者 `jti` 不同。`access` 携带完整 `roles/perms`；`refresh` 可只携带 `sub/jti/exp`（精简），刷新时再回库取最新权限。
- **刷新（`POST /api/admin/auth/refresh`）**：
  1. 校验 refresh 令牌签名与 `exp`，`typ=refresh`。
  2. 回库校验 `admin_users.status=='active'`（不变量 7）。
  3. **重新解析角色/权限**（确保权限实时性），签发新的 access（必要时滚动签发新 refresh）。
  4. 旧 refresh 是否立即作废 = 是否启用 refresh 轮换（rotation），见 §11；默认不轮换、不强制 denylist。
- **失效 / 登出（`POST /api/admin/auth/logout`）**：
  - 无状态 JWT 默认**客户端丢弃**令牌即登出。
  - 若启用服务端 denylist（按 `jti` + 过期时间，存内存/Redis），logout 把当前 access/refresh 的 `jti` 加入 denylist 直至其 `exp`。当前 schema **无 refresh 持久化表**，denylist 为可选加固项（§11）。

#### Token 状态（概念状态机）

```text
issued ──(到达 exp)──▶ expired
issued ──(logout / denylist)──▶ revoked
issued ──(refresh 成功)──▶ 派生新 access（旧 access 仍自然到期）
```

### 5.4 飞书身份的 identity_key 选择

- **决策**：`identity_key` 采用飞书 **`union_id`**（跨应用稳定，企业内唯一）；若仅单应用场景，可退化为 `open_id`。本文锁定 `union_id` 为首选，落到 `admin_identities.identity_key`（受 `UNIQUE(identity_type, identity_key)` 保护）。

### 5.5 权限解析到鉴权上下文

```text
loadAuthContext(userId):
  roles  = SELECT r.role_code FROM admin_user_roles ur
             JOIN admin_roles r ON r.id=ur.role_id_ref WHERE ur.user_id_ref=userId
  perms  = SELECT DISTINCT p.permission_code FROM admin_user_roles ur
             JOIN admin_role_permissions rp ON rp.role_id_ref=ur.role_id_ref
             JOIN admin_permissions p ON p.id=rp.permission_id_ref
             WHERE ur.user_id_ref=userId
  return AuthContext{ userId, displayName, roles, permsSet=set(perms), environment=APP_ENV }
```

- 登录/刷新时解析并写入 access claims；请求时由中间件从 access 令牌还原 `AuthContext`（不必每请求回库，权限以令牌内 `perms` 为准；权限变更通过短 TTL + 刷新生效）。
- 超管约定：可设角色 `super_admin` 拥有全部权限码；或在中间件中对 `super_admin` 角色短路放行（实现细节见 §11）。

### 5.6 鉴权中间件链（chi middleware）

请求经过的中间件顺序（`transport/http/middleware/`）：

```text
Recoverer ─▶ RequestID/Logger ─▶ EnvContext ─▶ Authn(Bearer) ─▶ Authz(require perm) ─▶ Handler ─▶ Audit(写操作)
```

| 中间件 | 职责 | 失败响应 |
| --- | --- | --- |
| `EnvContext` | 把 `APP_ENV` 注入 ctx，并在响应头 `X-Environment` 回显当前环境 | — |
| `Authn` | 校验 `Authorization: Bearer <accessToken>`：签名/exp/typ=access；还原 `AuthContext` 入 ctx | 缺/失效 -> `401 UNAUTHENTICATED` |
| `Authz(code)` | 路由级声明所需权限码，校验 `code ∈ ctx.perms` | 无权限 -> `403 FORBIDDEN` |
| `Audit` | 写操作成功后写 `audit_logs`（§9） | 不阻断主流程（失败仅告警） |

- **豁免鉴权的接口**（`Authn` 之前/旁路）：`POST /auth/login`、`POST /auth/refresh`、`POST /auth/feishu/callback`、健康检查。`refresh` 自行校验 refresh 令牌而非 access。
- `GET /api/admin/me` 需要 `Authn` 但不需要特定权限码。

### 5.7 环境上下文注入

- 当前运行环境由 `APP_ENV` 决定（`00 §2.1`），缺省 `develop`。
- 每个响应附 `X-Environment: <env>`；`GET /api/admin/me` 的 `environment` 字段亦回显；前端常驻展示 `EnvironmentBadge`（`01 §5.2`）。
- 本模块不写带 env 的数据；审计写入时 `audit_logs.env` 取当前运行环境（登录/管理操作的 env 记录为发生环境）。

---

## 6. 后端 API

> 全部遵循 `00 §7`：前缀 `/api/admin`；JSON 字段 `camelCase`；统一响应包络 `{ "data": ... }` / `{ "error": {code,message,details} }`；时间 ISO-8601 UTC。
> 除登录类外均需 `Authorization: Bearer <accessToken>`。下列"权限码"列：`—` 表示仅需登录，无需特定权限码。

### 6.0 接口总览

| # | Method | Path | 权限码 | 说明 |
| --- | --- | --- | --- | --- |
| 1 | POST | `/api/admin/auth/login` | 公开 | 密码登录 |
| 2 | POST | `/api/admin/auth/refresh` | 公开（凭 refresh） | 刷新 access |
| 3 | POST | `/api/admin/auth/logout` | 需登录 | 登出 |
| 4 | POST | `/api/admin/auth/feishu/callback` | 公开 | 飞书回调登录 |
| 5 | GET | `/api/admin/me` | — | 当前用户与权限/环境 |
| 6 | GET | `/api/admin/system/admin-users` | `system.read` | 管理员列表 |
| 7 | POST | `/api/admin/system/admin-users` | `admin_user.write` | 新建管理员 |
| 8 | GET | `/api/admin/system/admin-users/{id}` | `system.read` | 管理员详情 |
| 9 | PATCH | `/api/admin/system/admin-users/{id}` | `admin_user.write` | 更新管理员（含启停） |
| 10 | PUT | `/api/admin/system/admin-users/{id}/roles` | `admin_user.write` | 分配角色 |
| 11 | POST | `/api/admin/system/admin-users/{id}/reset-password` | `admin_user.write` | 重置/设置密码 |
| 12 | GET | `/api/admin/system/roles` | `system.read` | 角色列表 |
| 13 | POST | `/api/admin/system/roles` | `role.write` | 新建角色 |
| 14 | PATCH | `/api/admin/system/roles/{id}` | `role.write` | 更新角色 |
| 15 | DELETE | `/api/admin/system/roles/{id}` | `role.write` | 删除角色 |
| 16 | PUT | `/api/admin/system/roles/{id}/permissions` | `role.write` | 配置角色权限 |
| 17 | GET | `/api/admin/system/permissions` | `system.read` | 权限码目录列表 |
| 18 | POST | `/api/admin/system/permissions` | `permission.write` | 新建权限码 |
| 19 | DELETE | `/api/admin/system/permissions/{id}` | `permission.write` | 删除权限码 |

---

### 6.1 `POST /api/admin/auth/login`（密码登录）

- **权限**：公开（无需令牌）。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `userName` | string | 是 | — | minLen=1, maxLen=64 |
| `password` | string | 是 | — | minLen=1, maxLen=128 |

- **响应 DTO**（`data`）：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `accessToken` | string | JWT，30m |
| `refreshToken` | string | JWT，14d |
| `expiresAt` | string(ISO-8601) | accessToken 过期时刻 |
| `user` | object | `{ userId:number, userName:string, displayName:string, roles:string[], permissions:string[] }` |

- **错误码**：`VALIDATION_FAILED`(400)、`UNAUTHENTICATED`(401，凭据错误/账号禁用)、`INTERNAL`(500)。

**成功示例**：

```json
// 请求
POST /api/admin/auth/login
{ "userName": "alice", "password": "S3cret!" }

// 200
{
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1Ni␣...access",
    "refreshToken": "eyJhbGciOiJIUzI1Ni␣...refresh",
    "expiresAt": "2026-06-17T13:36:00Z",
    "user": {
      "userId": 1,
      "userName": "alice",
      "displayName": "Alice",
      "roles": ["super_admin"],
      "permissions": ["game.read", "game.write", "sync.preview", "sync.execute", "system.read", "admin_user.write", "role.write", "permission.write"]
    }
  }
}
```

**失败示例**：

```json
// 401
{ "error": { "code": "UNAUTHENTICATED", "message": "用户名或密码错误", "details": [] } }
```

---

### 6.2 `POST /api/admin/auth/refresh`（刷新令牌）

- **权限**：公开，但请求体须携带有效 `refreshToken`。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `refreshToken` | string | 是 | — | 非空，须为 `typ=refresh` 的有效 JWT |

- **响应 DTO**（`data`）：`{ accessToken, refreshToken, expiresAt }`（`refreshToken` 是否更换取决于是否启用轮换，默认原样返回或滚动签发）。
- **错误码**：`VALIDATION_FAILED`(400)、`UNAUTHENTICATED`(401，refresh 失效/过期/账号禁用)、`INTERNAL`(500)。

**成功示例**：

```json
// 请求
POST /api/admin/auth/refresh
{ "refreshToken": "eyJhbGciOiJIUzI1Ni␣...refresh" }

// 200
{
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1Ni␣...newAccess",
    "refreshToken": "eyJhbGciOiJIUzI1Ni␣...refresh",
    "expiresAt": "2026-06-17T14:06:00Z"
  }
}
```

**失败示例**：

```json
// 401
{ "error": { "code": "UNAUTHENTICATED", "message": "刷新令牌已失效，请重新登录", "details": [] } }
```

---

### 6.3 `POST /api/admin/auth/logout`（登出）

- **权限**：需登录（携带 access）。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `refreshToken` | string | 否 | `""` | 提供则一并加入 denylist（若启用） |

- **响应 DTO**（`data`）：`{ "loggedOut": true }`。
- **行为**：客户端丢弃令牌；若启用 denylist，把当前 access（与可选 refresh）`jti` 拉黑至其 `exp`。写审计 `admin.logout`。
- **错误码**：`UNAUTHENTICATED`(401)、`INTERNAL`(500)。

**成功示例**：

```json
// 200
{ "data": { "loggedOut": true } }
```

---

### 6.4 `POST /api/admin/auth/feishu/callback`（飞书回调登录）

- **权限**：公开。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `code` | string | 是 | — | 飞书授权码；dev+mock 时接受 `mock:<userName>` |
| `state` | string | 否 | `""` | CSRF 防护回显值，应与发起时一致 |
| `redirectUri` | string | 否 | `""` | 与申请授权时一致（如飞书要求） |

- **响应 DTO**（`data`）：同 §6.1 登录响应（`accessToken/refreshToken/expiresAt/user`）。
- **错误码**：`VALIDATION_FAILED`(400)、`UNAUTHENTICATED`(401，飞书校验失败/身份未绑定/账号禁用)、`INTERNAL`(500，飞书服务不可用)。

**成功示例**：

```json
// 请求
POST /api/admin/auth/feishu/callback
{ "code": "fs_auth_code_xxx", "state": "rand123" }

// 200
{
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1Ni␣...access",
    "refreshToken": "eyJhbGciOiJIUzI1Ni␣...refresh",
    "expiresAt": "2026-06-17T13:36:00Z",
    "user": {
      "userId": 2, "userName": "bob", "displayName": "Bob",
      "roles": ["operator"], "permissions": ["game.read", "channel.read"]
    }
  }
}
```

**失败示例**（身份未绑定）：

```json
// 401
{ "error": { "code": "UNAUTHENTICATED", "message": "飞书账号未绑定后台管理员，请联系管理员", "details": [] } }
```

---

### 6.5 `GET /api/admin/me`（当前用户与环境）

- **权限**：需登录，无需特定权限码（`—`）。
- **请求**：无 body。
- **响应 DTO**（`data`）：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `userId` | number | 管理员 id |
| `userName` | string | 用户名 |
| `displayName` | string | 展示名 |
| `email` | string | 邮箱 |
| `status` | string | `active`/`disabled` |
| `roles` | string[] | 角色码集合 |
| `permissions` | string[] | 权限码集合（去重） |
| `identities` | object[] | `[{ identityType, identityKey(脱敏) }]` |
| `environment` | string | 当前运行环境 `develop/sandbox/production` |

- **错误码**：`UNAUTHENTICATED`(401)、`INTERNAL`(500)。

**成功示例**：

```json
// 200
{
  "data": {
    "userId": 1,
    "userName": "alice",
    "displayName": "Alice",
    "email": "alice@example.com",
    "status": "active",
    "roles": ["super_admin"],
    "permissions": ["game.read", "game.write", "sync.execute", "system.read", "admin_user.write", "role.write", "permission.write"],
    "identities": [
      { "identityType": "password", "identityKey": "alice" },
      { "identityType": "feishu", "identityKey": "on_****1a2b" }
    ],
    "environment": "sandbox"
  }
}
```

---

### 6.6 `GET /api/admin/system/admin-users`（管理员列表）

- **权限**：`system.read`。
- **Query**：`page`(默认1)、`pageSize`(默认20，最大100)、`sort`(默认`-updatedAt`)、`keyword`(可选，匹配 userName/displayName/email)、`status`(可选 `active`/`disabled`)。
- **响应**：分页列表，`items[]` 每项 `{ id, userName, displayName, email, status, roles:string[], createdAt, updatedAt }`。
- **错误码**：`UNAUTHENTICATED`(401)、`FORBIDDEN`(403)、`VALIDATION_FAILED`(400)。

**成功示例**：

```json
// 200
{
  "data": {
    "items": [
      { "id": 1, "userName": "alice", "displayName": "Alice", "email": "alice@example.com",
        "status": "active", "roles": ["super_admin"], "createdAt": "2026-06-01T00:00:00Z", "updatedAt": "2026-06-10T00:00:00Z" }
    ],
    "page": 1, "pageSize": 20, "total": 1
  }
}
```

---

### 6.7 `POST /api/admin/system/admin-users`（新建管理员）

- **权限**：`admin_user.write`。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `userName` | string | 是 | — | minLen=1,maxLen=64，唯一 |
| `displayName` | string | 是 | — | minLen=1,maxLen=128 |
| `email` | string | 否 | `""` | maxLen=128，format=email（非空时） |
| `status` | string | 否 | `active` | enum: active/disabled |
| `password` | string | 否 | `""` | 提供则创建 password 身份；minLen=8,maxLen=128 |
| `roleIds` | number[] | 否 | `[]` | 角色 id，需存在 |
| `feishuKey` | string | 否 | `""` | 提供则创建 feishu 身份（identity_key=union_id） |

- **响应**：`data` = 新建用户详情（同 §6.8）。
- **错误码**：`VALIDATION_FAILED`(400)、`CONFLICT`(409，userName/身份已存在)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。
- **审计**：`admin_user.create`。

**成功示例**：

```json
// 请求
POST /api/admin/system/admin-users
{ "userName": "bob", "displayName": "Bob", "email": "bob@example.com",
  "password": "InitPass123", "roleIds": [2], "status": "active" }

// 200
{
  "data": {
    "id": 2, "userName": "bob", "displayName": "Bob", "email": "bob@example.com",
    "status": "active", "roles": [{ "id": 2, "roleCode": "operator", "roleName": "运营" }],
    "identities": [{ "identityType": "password", "identityKey": "bob" }],
    "createdAt": "2026-06-17T13:00:00Z", "updatedAt": "2026-06-17T13:00:00Z"
  }
}
```

**失败示例**：

```json
// 409
{ "error": { "code": "CONFLICT", "message": "userName already exists", "details": [{ "field": "userName" }] } }
```

---

### 6.8 `GET /api/admin/system/admin-users/{id}`（管理员详情）

- **权限**：`system.read`。
- **响应 DTO**（`data`）：`{ id, userName, displayName, email, status, roles:[{id,roleCode,roleName}], identities:[{identityType,identityKey(脱敏)}], permissions:string[], createdAt, updatedAt }`。
- **错误码**：`NOT_FOUND`(404)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。

**成功示例**：

```json
// 200
{
  "data": {
    "id": 2, "userName": "bob", "displayName": "Bob", "email": "bob@example.com", "status": "active",
    "roles": [{ "id": 2, "roleCode": "operator", "roleName": "运营" }],
    "identities": [{ "identityType": "password", "identityKey": "bob" }],
    "permissions": ["game.read", "channel.read"],
    "createdAt": "2026-06-17T13:00:00Z", "updatedAt": "2026-06-17T13:00:00Z"
  }
}
```

---

### 6.9 `PATCH /api/admin/system/admin-users/{id}`（更新管理员，含启停）

- **权限**：`admin_user.write`。
- **请求 DTO**（均可选，至少一项）：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `displayName` | string | 否 | — | maxLen=128 |
| `email` | string | 否 | — | maxLen=128, format=email |
| `status` | string | 否 | — | enum: active/disabled |

- **响应**：`data` = 更新后详情。
- **错误码**：`VALIDATION_FAILED`(400)、`NOT_FOUND`(404)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。
- **审计**：`admin_user.update`（status 变更 detail 记录 before/after）。

**成功示例**（禁用）：

```json
// 请求
PATCH /api/admin/system/admin-users/2
{ "status": "disabled" }

// 200
{ "data": { "id": 2, "userName": "bob", "status": "disabled", "updatedAt": "2026-06-17T13:10:00Z" } }
```

---

### 6.10 `PUT /api/admin/system/admin-users/{id}/roles`（分配角色）

- **权限**：`admin_user.write`。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `roleIds` | number[] | 是 | — | 全量覆盖；每个 id 必须存在于 admin_roles |

- **语义**：全量替换该用户的角色集合（删旧建新，写 `admin_user_roles`）。
- **响应**：`data` = `{ id, roles:[{id,roleCode,roleName}] }`。
- **错误码**：`VALIDATION_FAILED`(400，含不存在的 roleId)、`NOT_FOUND`(404，用户不存在)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。
- **审计**：`admin_user.assign_roles`（detail 记录 before/after roleIds）。

**成功示例**：

```json
// 请求
PUT /api/admin/system/admin-users/2/roles
{ "roleIds": [2, 3] }

// 200
{ "data": { "id": 2, "roles": [
  { "id": 2, "roleCode": "operator", "roleName": "运营" },
  { "id": 3, "roleCode": "auditor", "roleName": "审计员" }
] } }
```

---

### 6.11 `POST /api/admin/system/admin-users/{id}/reset-password`（重置/设置密码）

- **权限**：`admin_user.write`。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `newPassword` | string | 是 | — | minLen=8,maxLen=128 |

- **语义**：为该用户的 `password` 身份重置 bcrypt 哈希；若无 password 身份则创建之（`identity_key=userName`）。明文不落库、不入审计 detail。
- **响应**：`data` = `{ id, reset: true }`。
- **错误码**：`VALIDATION_FAILED`(400)、`NOT_FOUND`(404)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。
- **审计**：`admin_user.reset_password`（不含密码明文/哈希）。

**成功示例**：

```json
// 200
{ "data": { "id": 2, "reset": true } }
```

---

### 6.12 `GET /api/admin/system/roles`（角色列表）

- **权限**：`system.read`。
- **Query**：分页同上；`keyword`(匹配 roleCode/roleName)。
- **响应**：`items[]` = `{ id, roleCode, roleName, permissionCount, createdAt, updatedAt }`。
- **错误码**：`UNAUTHENTICATED`(401)、`FORBIDDEN`(403)。

**成功示例**：

```json
// 200
{ "data": { "items": [
  { "id": 1, "roleCode": "super_admin", "roleName": "超级管理员", "permissionCount": 26, "createdAt": "2026-06-01T00:00:00Z", "updatedAt": "2026-06-01T00:00:00Z" }
], "page": 1, "pageSize": 20, "total": 1 } }
```

---

### 6.13 `POST /api/admin/system/roles`（新建角色）

- **权限**：`role.write`。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `roleCode` | string | 是 | — | minLen=1,maxLen=64，唯一，建议 `[a-z0-9_]+` |
| `roleName` | string | 是 | — | minLen=1,maxLen=128 |
| `permissionIds` | number[] | 否 | `[]` | 初始权限集合，需存在 |

- **响应**：`data` = 新建角色详情 `{ id, roleCode, roleName, permissions:[{id,permissionCode,permissionName}] }`。
- **错误码**：`VALIDATION_FAILED`(400)、`CONFLICT`(409，roleCode 已存在)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。
- **审计**：`role.create`。

**成功示例**：

```json
// 请求
POST /api/admin/system/roles
{ "roleCode": "operator", "roleName": "运营", "permissionIds": [1, 3] }

// 200
{ "data": { "id": 2, "roleCode": "operator", "roleName": "运营",
  "permissions": [
    { "id": 1, "permissionCode": "game.read", "permissionName": "游戏-查看" },
    { "id": 3, "permissionCode": "channel.read", "permissionName": "渠道实例-查看" }
  ] } }
```

---

### 6.14 `PATCH /api/admin/system/roles/{id}`（更新角色）

- **权限**：`role.write`。
- **请求 DTO**（可选，至少一项）：`roleName`(string, maxLen=128)。`roleCode` 不可改（作为稳定标识）。
- **响应**：`data` = 更新后角色详情。
- **错误码**：`VALIDATION_FAILED`(400)、`NOT_FOUND`(404)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。
- **审计**：`role.update`。

**成功示例**：

```json
// 200
{ "data": { "id": 2, "roleCode": "operator", "roleName": "运营(海外)" } }
```

---

### 6.15 `DELETE /api/admin/system/roles/{id}`（删除角色）

- **权限**：`role.write`。
- **语义**：删除角色及其 `admin_role_permissions`、`admin_user_roles` 关联（级联或应用层先解绑）。若角色仍被用户引用，策略可选"拒绝删除（`CONFLICT`）"或"强制解绑"，本文默认**拒绝删除被占用角色**，要求先解绑。
- **响应**：`data` = `{ id, deleted: true }`。
- **错误码**：`NOT_FOUND`(404)、`CONFLICT`(409，角色仍被用户引用)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。
- **审计**：`role.delete`。

**成功示例**：

```json
// 200
{ "data": { "id": 2, "deleted": true } }
```

**失败示例**：

```json
// 409
{ "error": { "code": "CONFLICT", "message": "role is still assigned to 3 users", "details": [{ "userCount": 3 }] } }
```

---

### 6.16 `PUT /api/admin/system/roles/{id}/permissions`（配置角色权限）

- **权限**：`role.write`。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `permissionIds` | number[] | 是 | — | 全量覆盖；每个 id 须存在于 admin_permissions |

- **语义**：全量替换该角色的权限集合（写 `admin_role_permissions`）。生效：因 access 内嵌 perms，变更对已登录用户在其令牌刷新后生效（短 TTL 限制影响窗口）。
- **响应**：`data` = `{ id, permissions:[{id,permissionCode,permissionName}] }`。
- **错误码**：`VALIDATION_FAILED`(400)、`NOT_FOUND`(404)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。
- **审计**：`role.assign_permissions`（before/after）。

**成功示例**：

```json
// 请求
PUT /api/admin/system/roles/2/permissions
{ "permissionIds": [1, 2, 3, 4] }

// 200
{ "data": { "id": 2, "permissions": [
  { "id": 1, "permissionCode": "game.read", "permissionName": "游戏-查看" },
  { "id": 2, "permissionCode": "game.write", "permissionName": "游戏-编辑" },
  { "id": 3, "permissionCode": "channel.read", "permissionName": "渠道实例-查看" },
  { "id": 4, "permissionCode": "channel.write", "permissionName": "渠道实例-编辑" }
] } }
```

---

### 6.17 `GET /api/admin/system/permissions`（权限码目录列表）

- **权限**：`system.read`。
- **Query**：分页同上；`keyword`(匹配 permissionCode/permissionName)。可支持 `all=true` 返回全量（前端配权限选择器用）。
- **响应**：`items[]` = `{ id, permissionCode, permissionName, createdAt, updatedAt }`。
- **错误码**：`UNAUTHENTICATED`(401)、`FORBIDDEN`(403)。

**成功示例**：

```json
// 200
{ "data": { "items": [
  { "id": 1, "permissionCode": "game.read", "permissionName": "游戏-查看", "createdAt": "2026-06-01T00:00:00Z", "updatedAt": "2026-06-01T00:00:00Z" },
  { "id": 2, "permissionCode": "game.write", "permissionName": "游戏-编辑", "createdAt": "2026-06-01T00:00:00Z", "updatedAt": "2026-06-01T00:00:00Z" }
], "page": 1, "pageSize": 20, "total": 26 } }
```

---

### 6.18 `POST /api/admin/system/permissions`（新建权限码）

- **权限**：`permission.write`。
- **请求 DTO**：

| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `permissionCode` | string | 是 | — | maxLen=128，唯一，须匹配 `^[a-z0-9_]+\.[a-z0-9_]+$`（`resource.action`） |
| `permissionName` | string | 是 | — | minLen=1,maxLen=128 |

- **响应**：`data` = 新建权限 `{ id, permissionCode, permissionName }`。
- **错误码**：`VALIDATION_FAILED`(400，格式不符)、`CONFLICT`(409，已存在)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。
- **审计**：`permission.create`。

**成功示例**：

```json
// 请求
POST /api/admin/system/permissions
{ "permissionCode": "report.read", "permissionName": "报表-查看" }

// 200
{ "data": { "id": 27, "permissionCode": "report.read", "permissionName": "报表-查看" } }
```

**失败示例**（格式不符）：

```json
// 400
{ "error": { "code": "VALIDATION_FAILED", "message": "permissionCode must match resource.action", "details": [{ "field": "permissionCode" }] } }
```

---

### 6.19 `DELETE /api/admin/system/permissions/{id}`（删除权限码）

- **权限**：`permission.write`。
- **语义**：删除权限码及其 `admin_role_permissions` 关联（应用层先解绑）。被角色引用时默认**拒绝**（`CONFLICT`），要求先从角色移除。
- **响应**：`data` = `{ id, deleted: true }`。
- **错误码**：`NOT_FOUND`(404)、`CONFLICT`(409)、`FORBIDDEN`(403)、`UNAUTHENTICATED`(401)。
- **审计**：`permission.delete`。

**成功示例**：

```json
// 200
{ "data": { "id": 27, "deleted": true } }
```

---

## 7. 应用服务与 command/query 划分

应用层位于 `internal/app/`，按 `command`（写）/`query`（读）拆分，DTO 在 `internal/app/dto/`。领域纯逻辑在 `internal/domain/admin`、`internal/domain/auth`。

### 7.1 `AdminAuthService`（认证 + 令牌）

| 方法 | 类型 | 输入 | 输出 | 说明 |
| --- | --- | --- | --- | --- |
| `Login` | command | `LoginCmd{userName,password}` | `TokenPair + UserView` | 密码登录（§5.2.1） |
| `FeishuCallback` | command | `FeishuCallbackCmd{code,state,redirectUri}` | `TokenPair + UserView` | 飞书登录（§5.2.2） |
| `Refresh` | command | `RefreshCmd{refreshToken}` | `TokenPair` | 刷新（§5.3） |
| `Logout` | command | `LogoutCmd{accessJti,refreshToken?}` | `void` | 登出/拉黑（§5.3） |
| `Me` | query | `userId(from ctx)` | `MeView` | `/api/admin/me`（§6.5） |
| `LoadAuthContext` | query | `userId` | `AuthContext` | 角色/权限解析（§5.5），供中间件/刷新调用 |

- 依赖：`AdminUserRepository`、`AdminIdentityRepository`、`RolePermissionQuery`（读权限集合）、`crypto`（飞书凭据）、`bcrypt`、`jwt`、`config`、`FeishuClient`（含 mock 实现）。

### 7.2 `AdminUserService`

| 方法 | 类型 | 说明 |
| --- | --- | --- |
| `ListUsers` | query | 分页列表（§6.6） |
| `GetUser` | query | 详情（§6.8） |
| `CreateUser` | command | 新建 + 可选 password/feishu 身份 + 角色（§6.7） |
| `UpdateUser` | command | 更新基础信息/启停（§6.9） |
| `AssignRoles` | command | 全量覆盖角色（§6.10） |
| `ResetPassword` | command | 重置 bcrypt 哈希（§6.11） |

### 7.3 `RoleService`

| 方法 | 类型 | 说明 |
| --- | --- | --- |
| `ListRoles` / `GetRole` | query | §6.12 |
| `CreateRole` / `UpdateRole` / `DeleteRole` | command | §6.13/6.14/6.15 |
| `AssignPermissions` | command | 全量覆盖角色权限（§6.16） |

### 7.4 `PermissionService`

| 方法 | 类型 | 说明 |
| --- | --- | --- |
| `ListPermissions` | query | §6.17 |
| `CreatePermission` / `DeletePermission` | command | §6.18/6.19 |

### 7.5 仓储接口（窄，单聚合）

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

type RoleRepository interface {
    Create / Update / Delete / FindByID / List
    ReplacePermissions(ctx, roleID int64, permIDs []int64) error
}

type PermissionRepository interface {
    Create / Delete / FindByID / List
    ListCodesByUser(ctx, userID int64) ([]string, error)  // §5.5 权限解析
}
```

- 跨表编排（建用户同时建身份与分配角色、登录时联表取权限）放在 `app` 服务层，不入仓储（`01 §4.1/4.2`）。

---

## 8. 前端信息架构

技术栈：`Vue 3 + Vite + TS + Pinia + Vue Router + Element Plus`（`01 §1/§5`）。

### 8.1 登录页（`views/login/`）

- 路由 `/login`，**与玩家登录 UI 完全隔离**（红线）。
- 两种登录方式 Tab/切换：
  - **密码登录**：`userName` + `password` 表单，提交 `POST /auth/login`。
  - **飞书登录**：按钮跳转飞书授权 → 回调页提交 `code` 到 `POST /auth/feishu/callback`。
- 登录成功：写入 `auth` store（token + user），跳转 `redirect` 或 `/dashboard`。
- 状态：加载中（按钮 loading）、错误态（凭据错误/账号禁用，行内红字）、飞书未绑定提示、网络异常 toast。

### 8.2 Pinia stores

| store | 状态 | 行为 |
| --- | --- | --- |
| `auth`（`stores/auth.ts`） | `accessToken`、`refreshToken`、`user`、`expiresAt` | `login()`、`feishuLogin()`、`refresh()`、`logout()`、持久化（localStorage，谨慎）、自动续期（access 临期前调 refresh） |
| `permission`（`stores/permission.ts`） | `perms: Set<string>`、`roles: string[]` | `hasPerm(code)`、`hasAnyPerm([])`、按权限过滤动态路由/菜单 |
| `app`（`stores/app.ts`） | `environment` | 由 `/api/admin/me` 或响应头 `X-Environment` 写入，常驻 `EnvironmentBadge` |

### 8.3 API 客户端与拦截器（`api/http.ts`）

- 请求拦截：注入 `Authorization: Bearer <accessToken>`。
- 响应拦截：解包 `{ data }`；遇 `error.code`：
  - `UNAUTHENTICATED`(401)：尝试用 refresh 自动续期一次；失败则清空 auth、跳 `/login`。
  - `FORBIDDEN`(403)：toast"无权限"，不跳登录。
  - 其它：toast `error.message`。
- 读取 `X-Environment` 写入 `app` store。

### 8.4 动态菜单与路由守卫（`router/`）

- 路由分组（`01 §5.1`）：`/login`、`/dashboard`、`/games`、`/channels`、`/cashier`、`/payment`、`/audit`、`/system`。
- 路由 `meta.perm` 声明所需权限码；守卫 `beforeEach`：
  1. 未登录且非 `/login` → 重定向 `/login?redirect=...`。
  2. 已登录访问需要 `meta.perm` 的路由 → `permission.hasPerm(meta.perm)` 为假则跳 403 页/隐藏菜单。
  3. 菜单按 `hasPerm` 过滤；无权限项不渲染。
- 写/危险按钮挂权限指令 `v-perm="'role.write'"`（无权限置灰或隐藏，`01 §5.3`）。

### 8.5 `/system` 下管理页

- **管理员管理**（`views/system/admin-users/`）：列表（分页/搜索/状态筛选）+ 抽屉式新建/编辑 + 角色分配抽屉 + 重置密码弹窗 + 启用/禁用开关。写操作挂 `admin_user.write`。
- **角色管理**（`views/system/roles/`）：角色列表 + 新建/编辑抽屉 + 权限分配（权限码多选树/分组，按 `resource` 分组）。写操作挂 `role.write`。
- **权限管理**（`views/system/permissions/`）：权限码目录列表 + 新建/删除（`permission.write`）；通常只读浏览为主。
- 进入 `/system` 需 `system.read`。

### 8.6 空/错/权限/异常态

| 态 | 表现 |
| --- | --- |
| 空 | 列表无数据：展示空态插画 + "新建"引导（有写权限时） |
| 错 | 接口报错：页面级错误条 + 重试按钮；表单字段级 `VALIDATION_FAILED.details` 行内提示 |
| 权限 | 无 `system.read`：菜单不显示 `/system`；直链访问跳 403 页；无写权限的按钮置灰/隐藏 |
| 异常 | token 失效：自动 refresh，失败跳登录；网络异常：全局 toast + 局部 skeleton |

---

## 9. 与公共能力的关系

### 9.1 密文（`00 §6`）

- `admin_identities.credential_ciphertext`：
  - `password` 身份：存 **bcrypt 哈希**（单向，非 AES 密文，但同样禁止回显）。
  - `feishu` 身份：如需缓存第三方令牌/凭据，走 `infra/crypto`（AES-GCM）加密后存入，响应一律脱敏（`identityKey` 也脱敏展示，如 `on_****1a2b`）。
- 任何接口响应**绝不回明文密码/哈希/第三方密钥**；`/api/admin/me` 与用户详情仅回 `identityType` + 脱敏 `identityKey`。

### 9.2 审计（`00 §8` / `audit`）

- 本模块写 `audit_logs` 的 `action` 命名（与权限码同源）：

| action | 触发 | resource_type |
| --- | --- | --- |
| `admin.login` | 登录成功（password/feishu） | `admin_user` |
| `admin.login_failed` | 登录失败（可选，节流） | `admin_user` |
| `admin.logout` | 登出 | `admin_user` |
| `admin_user.create` | 新建管理员 | `admin_user` |
| `admin_user.update` | 更新/启停 | `admin_user` |
| `admin_user.assign_roles` | 分配角色 | `admin_user` |
| `admin_user.reset_password` | 重置密码（不含明文） | `admin_user` |
| `role.create` / `role.update` / `role.delete` | 角色增改删 | `admin_role` |
| `role.assign_permissions` | 配角色权限 | `admin_role` |
| `permission.create` / `permission.delete` | 权限码维护 | `admin_permission` |

- `audit_logs` 字段：`actor_id`(当前管理员 id)、`action`、`resource_type`、`resource_id`、`env`(当前运行环境)、`detail_json`(before/after，密文脱敏)、`created_at`。
- 登录类的 `actor_id` 即登录主体本人；登录失败若主体未知（用户名不存在）可记 `actor_id=0` 或省略（按 21 模块约定）。

### 9.3 环境上下文（`00 §2` / `01 §2`）

- `admin_*` 平台级、无 env；但每次审计写入的 `audit_logs.env` 记录"操作发生时的运行环境"。
- `/api/admin/me.environment` 与响应头 `X-Environment` 向前端透出当前运行环境，前端常驻展示。

### 9.4 统一 API 约定（`00 §7`）

- 全部走 `{ data }` / `{ error }` 包络；字段 camelCase；分页 `page/pageSize/total`；错误码采用 `00 §7.4` 全局集合（本模块未新增私有错误码，全部复用 `UNAUTHENTICATED/FORBIDDEN/NOT_FOUND/VALIDATION_FAILED/CONFLICT/INTERNAL`）。

---

## 10. 测试要点

## 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/auth.yaml`；前端 e2e：`tests/frontend/e2e/login.spec.ts`。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| POST /api/admin/auth/login | ✓ | — | — | ✓ | — | — | ✓ | ✓ | — | — | 密码 bcrypt 校验；登录本身 S2 豁免（无前置 token）；防用户枚举（不区分用户不存在/密码错） |
| POST /api/admin/auth/refresh | ✓ | — | — | ✓ | — | — | — | — | — | — | token 刷新（校验 typ=refresh/过期，回库校验账号禁用）；S2 豁免（凭 refresh 而非 access）；轮换可选 |
| POST /api/admin/auth/logout | ✓ | ✓ | — | — | — | — | ✓ | — | — | — | 登出仅需登录无特定权限；denylist（jti 拉黑）可选 |
| POST /api/admin/auth/feishu/callback | ✓ | — | — | ✓ | — | — | ✓ | ✓ | — | — | 飞书回调（code 换取/union_id 绑定/未绑定拒绝）；mock 仅 dev；登录本身 S2 豁免 |
| GET /api/admin/me | ✓ | ✓ | — | — | — | ✓ | — | ✓ | — | — | environment/X-Environment 回显；identityKey 脱敏；仅需登录无特定权限码 |
| GET /api/admin/system/admin-users | ✓ | ✓ | ✓ | ✓ | — | — | — | — | ✓ | — | 平台级无 env；status/分页 query 校验 |
| POST /api/admin/system/admin-users | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ | — | ✓ | userName/身份唯一冲突；密码 bcrypt 落库不回显；建用户+身份+角色跨表事务 |
| GET /api/admin/system/admin-users/{id} | ✓ | ✓ | ✓ | — | — | — | — | ✓ | — | — | identityKey 脱敏；404 |
| PATCH /api/admin/system/admin-users/{id} | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | — | — | — | 启停状态机；status before/after 审计 |
| PUT /api/admin/system/admin-users/{id}/roles | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | — | — | ✓ | 全量覆盖角色（删旧建新事务）；roleId 须存在 |
| POST /api/admin/system/admin-users/{id}/reset-password | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | ✓ | — | — | 密码哈希；明文不回显、不入审计 detail |
| GET /api/admin/system/roles | ✓ | ✓ | ✓ | ✓ | — | — | — | — | ✓ | — | 平台级无 env；分页 |
| POST /api/admin/system/roles | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | roleCode 唯一冲突；角色+初始权限事务 |
| PATCH /api/admin/system/roles/{id} | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | — | — | — | roleCode 不可改（稳定标识） |
| DELETE /api/admin/system/roles/{id} | ✓ | ✓ | ✓ | — | ✓ | — | ✓ | — | — | ✓ | 被用户引用拒绝删除（CONFLICT）；级联解绑 user_roles/role_permissions |
| PUT /api/admin/system/roles/{id}/permissions | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | — | — | ✓ | 全量覆盖权限（事务）；生效需令牌刷新；permissionId 须存在 |
| GET /api/admin/system/permissions | ✓ | ✓ | ✓ | ✓ | — | — | — | — | ✓ | — | all=true 全量；平台级无 env |
| POST /api/admin/system/permissions | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | — | permissionCode 正则 `resource.action` 校验；唯一冲突 |
| DELETE /api/admin/system/permissions/{id} | ✓ | ✓ | ✓ | — | ✓ | — | ✓ | — | — | ✓ | 被角色引用拒绝删除（CONFLICT）；解绑 role_permissions |

前端：登录页（`views/login/`：密码/飞书 Tab、加载/凭据错误态、飞书未绑定提示、登录后 `redirect` 跳转）与 `/system` 管理员/角色/权限页（列表分页/搜索、抽屉式新建编辑、角色分配、权限多选树、启停开关、重置密码弹窗、403 直链拦截）走 Playwright e2e（`login.spec.ts`）+ 关键状态截图 / `auth`、`permission`、`app` store、`v-perm` 权限指令与路由守卫 `beforeEach` 走 vitest 组件/单元测试。

### 补充关键用例

### 10.1 单元测试（domain / app）

- **bcrypt 校验**：正确密码通过、错误密码拒绝、空密码拒绝；哈希永不等于明文。
- **JWT 签发/解析**：access TTL=30m、refresh TTL=14d；篡改签名拒绝；过期拒绝；`typ` 混用拒绝（用 refresh 当 access 访问被拒）。
- **权限解析（§5.5）**：多角色权限并集去重；无角色 → 空权限；`super_admin` 短路（若实现）。
- **状态机**：`disabled` 用户登录被拒；`disabled` 用户 refresh 被拒；启用后恢复。
- **DTO 校验**：`permissionCode` 正则 `^[a-z0-9_]+\.[a-z0-9_]+$`；userName/password 长度边界。

### 10.2 集成测试（transport，httptest）

- **登录端到端**：seed 一个 active 管理员 → `POST /auth/login` 返回 token → 用 access 访问受保护接口 200。
- **刷新**：access 过期后用 refresh 换新 access 成功；refresh 过期失败。
- **登出**：logout 后（若启用 denylist）原 token 被拒。
- **飞书 mock**：`APP_ENV=develop` + `ADMIN_FEISHU_MOCK=true`，`code=mock:alice` 登录成功；非 dev 环境 mock 路径被拒。
- **权限中间件**：无 token → 401；有 token 无对应权限码 → 403；有权限 → 200。
- **环境上下文**：响应头 `X-Environment` 与 `/api/admin/me.environment` 等于 `APP_ENV`。
- **管理后台 CRUD**：建用户→建身份→分配角色→配角色权限→新 access 内 perms 生效；删除被占用角色/权限 → 409。
- **唯一约束**：重复 userName/roleCode/permissionCode/身份 → 409 `CONFLICT`。

### 10.3 安全测试

- 响应体不含密码明文/哈希/飞书密钥（全链路检查脱敏）。
- 登录失败不区分"用户不存在"与"密码错误"（防枚举）。
- 越权：低权限用户访问 `/system` 接口 403；篡改 JWT perms 因签名校验失败而 401。
- 审计：每个写/登录操作都产生一条 `audit_logs`，detail 不含敏感明文。

---

## 11. 未决问题与显式假设

### 11.1 显式假设

1. **refresh 令牌无持久化表**：当前 `000001` schema 无 refresh/session 表，假设 refresh 为**无状态 JWT**。logout 与"被禁用用户的存量 access 立即失效"依赖可选的 `jti` denylist（内存/Redis）；本期默认**不启用 denylist**，登出为客户端丢弃 + 短 TTL 自然过期。
2. **权限以 access 内嵌 `perms` 为准**：权限变更对已登录用户在其 access 刷新（≤30m）后生效，可接受短窗口延迟。
3. **飞书 `identity_key` 用 `union_id`**（§5.4）。
4. **飞书未绑定身份默认拒绝登录**（不自动开户），需管理员先在 `/system` 绑定。
5. **`super_admin` 角色**：作为拥有全部权限码的约定角色，由 seed 固化或中间件短路放行（二选一，实现期定）。
6. **权限码 `*.read` 多数不做接口级强校验**（仅需登录），敏感读 `audit.read`/`system.read` 例外。
7. **mock 仅 dev**：`ADMIN_FEISHU_MOCK` 仅当 `APP_ENV=develop` 生效，否则忽略并告警。

### 11.2 未决问题（待产品/安全确认）

1. **是否启用 refresh 轮换（rotation）+ denylist**：涉及是否新增 `admin_refresh_tokens` / `admin_sessions` 表（当前 schema 没有）。若需"禁用即时踢下线""单设备登录""强制下线"，必须补持久化会话表与新增迁移。
2. **`admin_users.status` 是否补 DB 级 `CHECK`**：当前迁移未对 `status` 写 CHECK；建议后续迁移补 `CHECK (status IN ('active','disabled'))` 与 `(status)` 索引。
3. **`admin_identities` 是否补 `(user_id_ref)` 普通索引**：便于按用户列身份；当前仅有 `(identity_type, identity_key)` 唯一索引。
4. **删除策略**：角色/权限被引用时拒绝删除（本文默认）还是强制级联解绑？需确认。
5. **登录失败审计与限流**：是否记录 `admin.login_failed`、是否对同一用户名/IP 做失败次数限流与锁定（防爆破），以及阈值。
6. **密码强度策略**：本文取 minLen=8；是否需要复杂度规则、历史密码、过期更换周期。
7. **多因素认证（MFA）**：当前不做；是否纳入后续。
8. **`permissions` 是否完全运行时可维护**：权限码与代码中的 `Authz` 声明耦合；建议权限码以 seed 为主，运行时新增的权限码若无代码挂载点则无实际作用，需明确治理流程。
9. **飞书"自动开户/首登自助绑定"**：是否允许；若允许，默认赋予什么角色。
