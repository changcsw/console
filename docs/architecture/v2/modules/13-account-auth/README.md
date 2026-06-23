---
id: account-auth
code: "13"
title: 自有账号认证（Account Auth）
status: target
code_paths:
  - services/admin-api/internal/domain/accountauth
  - services/admin-api/internal/transport/http/games
  - apps/admin-web/src/views/games
depends_on: [channel, game, common]
impacts: [snapshot, sync, testing]
children: []
---

# 13 · 自有账号认证（Account Auth）

> 本模块负责"游戏自有账号体系"的认证方式配置（游客/手机/邮箱/第三方登录等），与 `channel-login`（渠道强制登录）和 `auth`（后台鉴权，管理员登录）**底层完全分开**（`00 §9` 红线）。
> 阅读前请先读 `../../00-common.md`（模板四件套 §4、ConfigStatus 状态机 §3.4、密文 §6、API 约定 §7）。

---

## 1. 模块概述与边界

### 1.1 职责
- 平台级维护**认证方式主数据** `account_auth_types`（游客、手机、邮箱、Google、Apple…）。
- 平台级维护**渠道允许的认证方式** `channel_account_auth_types`（某渠道允许哪些、默认勾选哪些）。
- 平台级维护**认证方式模板** `account_auth_templates`（模板四件套）。
- 游戏级维护**实际启用与配置** `game_account_auth_configs`（游戏维度业务表，每环境独立 schema）。

### 1.2 边界
- 仅面向"游戏自有账号系统"的认证方式配置。
- 不包含：管理员登录（`auth`）、渠道强制登录（`channel-login`）。
- 与渠道策略关系：`channel_policies.login_mode=account_system` 的渠道走自有账号体系；`channel_only` 渠道走 `channel-login`。

---

## 2. 领域模型与聚合

- 归属 `Game` 聚合的子区域：游戏级"自有账号认证配置集合"。
- 值对象：`AccountAuthType`（主数据）、`AccountAuthTemplate`（模板元数据，四件套）。
- 配置实体：`GameAccountAuthConfig`（`enabled` + `config_json` + `config_status` + 校验结果）。
- 纯规则：`ValidateConfigAgainstTemplate(config_json, template)` → 返回 `config_status` 与 `last_check_message`（无 IO）。

---

## 3. 数据模型

> `game_account_auth_configs` 为**游戏维度业务表**，在每个环境 schema（`develop`/`sandbox`/`production`）各一份同名同结构表（**不带 `env` 列**，行属于哪个 env 由其所在 schema 决定）；其余三表为平台级共享表，放在共享 schema `platform`（`00 §2.2`）。

### 3.1 `account_auth_types`（平台级）

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `auth_type_id` | VARCHAR(64) | 否 | — | UNIQUE |
| `auth_type_name` | VARCHAR(64) | 否 | — | |
| `enabled` | BOOLEAN | 否 | `TRUE` | |
| `sort` | INT | 否 | `0` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

seed（来自现有迁移）：

| auth_type_id | name | sort |
| --- | --- | --- |
| guest | 游客 | 10 |
| phone | 手机号 | 20 |
| email | 邮箱 | 30 |
| google | Google 登录 | 40 |
| apple | Apple 登录 | 50 |
| facebook | Facebook 登录 | 60 |
| line | LINE 登录 | 70 |
| kakao | Kakao 登录 | 80 |

### 3.2 `channel_account_auth_types`（平台级）

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `channel_id_ref` | BIGINT | 否 | — | FK→channels(id) |
| `auth_type_id_ref` | BIGINT | 否 | — | FK→account_auth_types(id) |
| `default_enabled` | BOOLEAN | 否 | `FALSE` | 该渠道下是否默认勾选 |
| `locked` | BOOLEAN | 否 | `FALSE` | 锁定后游戏侧不可改 |
| `sort` | INT | 否 | `0` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(channel_id_ref, auth_type_id_ref)`。

### 3.3 `account_auth_templates`（平台级，模板四件套）

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `auth_type_id_ref` | BIGINT | 否 | — | FK→account_auth_types(id) |
| `template_version` | VARCHAR(32) | 否 | — | |
| `form_schema_json` | JSONB | 否 | `[]` | 见 `00 §4` |
| `secret_fields_json` | JSONB | 否 | `[]` | |
| `file_fields_json` | JSONB | 否 | `[]` | |
| `validation_rules_json` | JSONB | 否 | `{}` | |
| `enabled` | BOOLEAN | 否 | `TRUE` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(auth_type_id_ref, template_version)`。

### 3.4 `game_account_auth_configs`（游戏维度业务表，每环境独立 schema，不带 env 列）

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `game_id_ref` | BIGINT | 否 | — | FK→games(id)（同 schema 普通外键） |
| `auth_type_id_ref` | BIGINT | 否 | — | FK→platform.account_auth_types(id)（跨 schema 指向平台表） |
| `enabled` | BOOLEAN | 否 | `FALSE` | |
| `config_json` | JSONB | 否 | `{}` | 含密文位（加密后） |
| `config_status` | VARCHAR(16) | 否 | `'empty'` | CHECK in `empty/invalid/valid` |
| `last_check_at` | TIMESTAMPTZ | 是 | `NULL` | |
| `last_check_message` | VARCHAR(255) | 否 | `''` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(game_id_ref, auth_type_id_ref)`。索引：`(game_id_ref)`。

**迁移（每环境 schema 各建一份，DDL 同结构）**：
```sql
-- 在每个环境 schema（develop/sandbox/production）下建同名同结构表
CREATE TABLE game_account_auth_configs (
  id                 BIGSERIAL PRIMARY KEY,
  game_id_ref        BIGINT NOT NULL REFERENCES games(id),
  auth_type_id_ref   BIGINT NOT NULL REFERENCES platform.account_auth_types(id),
  enabled            BOOLEAN NOT NULL DEFAULT FALSE,
  config_json        JSONB NOT NULL DEFAULT '{}'::jsonb,
  config_status      VARCHAR(16) NOT NULL DEFAULT 'empty' CHECK (config_status IN ('empty','invalid','valid')),
  last_check_at      TIMESTAMPTZ,
  last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, auth_type_id_ref)
);
```
> 业务表→业务表（`games`）用同 schema 普通外键；业务表→平台表（`account_auth_types`）跨 schema 指向 `platform.*`。运行时连接设 `search_path = <当前env>, platform`，仓储 SQL 不写 schema 前缀、不带 env 谓词，环境由 search_path 决定。

---

## 4. 枚举与默认值清单

| 项 | 取值 / 默认 |
| --- | --- |
| `ConfigStatus` | `empty`/`invalid`/`valid`，默认 `empty` |
| `enabled`（game config） | 默认 `FALSE` |
| `default_enabled` | 默认 `FALSE` |
| `locked` | 默认 `FALSE` |
| 模板四件套 | `form_schema_json/secret_fields_json/file_fields_json` 默认 `[]`；`validation_rules_json` 默认 `{}` |
| `config_json` | 默认 `{}` |
| `last_check_message` | 默认 `''` |
| `sort` | 默认 `0` |
| seed auth types | guest/phone/email/google/apple/facebook/line/kakao（含上表 sort） |

---

## 5. 业务规则与状态机

### 5.1 模板驱动校验 → config_status
遵循 `00 §3.4`：
1. `enabled=false` 且无配置 → `empty`。
2. 写入 `config_json` 后，按模板 `validation_rules_json` + 必填（含 `secret_fields_json`/`file_fields_json` 标记的必填）校验：
   - 缺必填/敏感/文件字段或校验未过 → `invalid`，`last_check_message` 给出具体缺失项；
   - 全部通过 → `valid`，记录 `last_check_at`。
3. "只启用未填参数"必须落 `invalid` 并前端直接警告（不得静默 `empty`）。

### 5.2 渠道允许范围
- 某渠道（`account_system`）允许的认证方式取自 `channel_account_auth_types`；`default_enabled=true` 的在游戏接入时默认勾选；`locked=true` 的游戏侧不可关闭/修改。

### 5.3 密文
- `secret_fields_json` 标记字段加密存入 `config_json` 的密文位；响应脱敏（`00 §6`）。

---

## 6. 后端 API

> 前缀 `/api/admin`，遵循 `00 §7` 包络。读权限 `game.read`，写权限 `game.write`（账号认证属游戏配置范畴）。

### 6.1 认证方式主数据
**GET `/api/admin/account-auth/types`**
```json
{ "data": { "items": [
  { "authTypeId": "guest", "authTypeName": "游客", "enabled": true, "sort": 10,
    "template": { "templateVersion": "v1", "formSchema": [], "secretFields": [], "fileFields": [], "validationRules": {} } }
] } }
```

### 6.2 渠道允许的认证方式
**GET `/api/admin/channels/{channelId}/account-auth-types`**
```json
{ "data": { "items": [
  { "authTypeId": "guest", "defaultEnabled": true, "locked": false },
  { "authTypeId": "google", "defaultEnabled": true, "locked": false }
] } }
```

### 6.3 游戏级配置读写
**GET `/api/admin/games/{gameId}/account-auth-configs`**（按当前 env）
```json
{ "data": { "items": [
  { "authTypeId": "google", "enabled": true,
    "configJson": { "clientId": "xxx", "clientSecret": "masked", "redirectUri": "https://..." },
    "configStatus": "valid", "lastCheckAt": "2026-06-15T10:00:00Z", "lastCheckMessage": "" }
] } }
```
**PUT `/api/admin/games/{gameId}/account-auth-configs`** 权限 `game.write`。
请求 DTO（整体替换式）：
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| `items[].authTypeId` | string | 是 | — | 必须是该游戏渠道允许的类型 |
| `items[].enabled` | bool | 否 | `false` | |
| `items[].configJson` | object | 否 | `{}` | 按模板四件套校验 |
```json
{ "items": [
  { "authTypeId": "guest", "enabled": true, "configJson": {} },
  { "authTypeId": "google", "enabled": true,
    "configJson": { "clientId": "xxx", "clientSecret": "xxx", "redirectUri": "https://..." } }
] }
```
成功响应：返回各项计算后的 `configStatus` 与 `lastCheckMessage`。
失败示例（缺敏感字段）：每项 `configStatus=invalid`，`lastCheckMessage="缺少必填敏感字段或文件字段: clientSecret"`。

### 6.4 form_schema_json 样例（google）
```json
[
  { "key": "clientId", "label": "Client ID", "component": "input", "required": true, "order": 10, "scope": "both" },
  { "key": "clientSecret", "label": "Client Secret", "component": "password", "required": true, "order": 20, "scope": "server" },
  { "key": "redirectUri", "label": "Redirect URI", "component": "input", "required": true, "order": 30, "scope": "both" }
]
// secret_fields_json: ["clientSecret"]
// validation_rules_json: { "clientId": {"minLen":1}, "redirectUri": {"format":"url"} }
```

---

## 7. 应用服务与 command/query
- `AccountAuthService`：列类型/模板、列渠道允许类型、读写游戏配置（含模板校验、密文加密、状态计算、审计）。
- 仓储：`AccountAuthTypeRepository`、`ChannelAccountAuthTypeRepository`、`AccountAuthTemplateRepository`、`GameAccountAuthConfigRepository`（业务表仓储，SQL 不写 schema 前缀、不带 env 谓词，环境由 `search_path` 决定）。
- 领域纯规则：`ValidateConfigAgainstTemplate`。

---

## 8. 前端信息架构
- 游戏详情 → "自有账号认证" Tab。
- 列出该游戏允许的认证方式（来自渠道允许集合并集），每项：启用开关 + 模板驱动表单（统一渲染器消费四件套）。
- `config_status` 明确展示 `empty/invalid/valid`（标签 + 颜色）；启用但 `invalid` 行内警告。
- 密文字段脱敏展示、可重填；`locked` 项禁用编辑。
- 空/错/权限态遵循全局规范；无 `game.write` 置灰。

---

## 9. 与公共能力的关系
- 模板四件套（`00 §4`）：表单渲染与校验来源。
- 密文（`00 §6`）：secret 字段加密与脱敏。
- 审计：`game.account_auth.update` 写 `audit_logs`。
- env：游戏级配置的写操作落当前运行环境对应 schema（业务表不带 env 列，环境由 schema 决定）；同步在 `channels` 之外的 game section（本模块归 `channels`/`config` 相关 section，见 `sync` 约定）。

---

## 10. 测试要点

## 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env（schema 隔离）：写落当前环境 schema、不允许跨 schema 写 / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/account-auth.yaml`；前端 e2e：`tests/frontend/e2e/games.spec.ts`（自有账号认证页签）。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET /api/admin/account-auth/types | ✓ | ✓ | ✓ | — | — | — | — | — | — | — | 模板驱动校验（四件套结构） |
| GET /api/admin/channels/{channelId}/account-auth-types | ✓ | ✓ | ✓ | — | — | — | — | — | — | — | 按渠道允许集合 |
| GET /api/admin/games/{gameId}/account-auth-configs | ✓ | ✓ | ✓ | — | — | ✓ | — | ✓ | — | — | config_status(empty/invalid/valid)、secret 脱敏 |
| PUT /api/admin/games/{gameId}/account-auth-configs | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | 模板驱动校验、按渠道允许集合、复制清空 secret/file |

前端：`games.spec.ts` 自有账号认证页签（启用开关 + 模板驱动表单渲染、`empty/invalid/valid` 标签与行内警告、`locked` 禁用、密文脱敏可重填） / vitest 模板渲染器组件（四件套消费、secret/file 字段渲染与校验）。

### 补充关键用例
- 启用但缺敏感字段 → `invalid` + 明确 message。
- 全字段通过 → `valid` + `lastCheckAt`。
- `locked` 类型游戏侧不可改。
- 密文响应脱敏、不回明文。
- 唯一性：同一环境 schema 内同 `(game, authType)` 仅一条。

---

## 11. 未决问题与显式假设
- 假设游戏可启用的认证方式 = 该游戏已启用渠道允许集合的并集（具体并集口径如有歧义，按"并集 + locked 强制"处理）。
- 假设 `phone/email` 等无需第三方密钥的类型，模板可能为空四件套，启用即 `valid`。
- 多语言文案不在本模块，统一走前端 i18n。
