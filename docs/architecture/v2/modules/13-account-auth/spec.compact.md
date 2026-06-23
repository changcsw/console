---
id: account-auth
code: "13"
title: 自有账号认证（Account Auth）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [channel, game, common]
code_paths:
  - services/admin-api/internal/domain/account/auth
  - services/admin-api/internal/transport/http/games
  - apps/admin-web/src/views/games
---

# 13 · 自有账号认证 — Compact Spec

> 代码生成用精简规格。完整背景/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（模板四件套 §4、ConfigStatus §3.4、密文 §6、API 包络 §7）。

## 边界
- 仅「游戏自有账号体系」认证方式配置。不含管理员登录(`auth`)、渠道强制登录(`channel-login`)。
- `channel_policies.login_mode=account_system` 走本模块；`channel_only` 走 `channel-login`。

## 数据模型
平台级表（schema `platform`）3 张 + 游戏维度业务表 1 张（每环境 schema 各一份，**不带 env 列**，env 由 search_path 决定）。

### account_auth_types（平台级）
| 列 | 类型 | 约束 |
| --- | --- | --- |
| id | BIGSERIAL | PK |
| auth_type_id | VARCHAR(64) | UNIQUE, NOT NULL |
| auth_type_name | VARCHAR(64) | NOT NULL |
| enabled | BOOLEAN | NOT NULL DEFAULT TRUE |
| sort | INT | NOT NULL DEFAULT 0 |
| created_at/updated_at | TIMESTAMPTZ | NOT NULL DEFAULT NOW() |

seed: guest(10)/phone(20)/email(30)/google(40)/apple(50)/facebook(60)/line(70)/kakao(80)（值为 auth_type_id(sort)）。

### channel_account_auth_types（平台级）
| 列 | 类型 | 约束 |
| --- | --- | --- |
| id | BIGSERIAL | PK |
| channel_id_ref | BIGINT | NOT NULL FK→channels(id) |
| auth_type_id_ref | BIGINT | NOT NULL FK→account_auth_types(id) |
| default_enabled | BOOLEAN | NOT NULL DEFAULT FALSE（默认勾选） |
| locked | BOOLEAN | NOT NULL DEFAULT FALSE（锁定后游戏侧不可改） |
| sort | INT | NOT NULL DEFAULT 0 |
| created_at/updated_at | TIMESTAMPTZ | NOT NULL DEFAULT NOW() |

UNIQUE(channel_id_ref, auth_type_id_ref)。

### account_auth_templates（平台级，模板四件套）
| 列 | 类型 | 约束 |
| --- | --- | --- |
| id | BIGSERIAL | PK |
| auth_type_id_ref | BIGINT | NOT NULL FK→account_auth_types(id) |
| template_version | VARCHAR(32) | NOT NULL |
| form_schema_json | JSONB | NOT NULL DEFAULT '[]' |
| secret_fields_json | JSONB | NOT NULL DEFAULT '[]' |
| file_fields_json | JSONB | NOT NULL DEFAULT '[]' |
| validation_rules_json | JSONB | NOT NULL DEFAULT '{}' |
| enabled | BOOLEAN | NOT NULL DEFAULT TRUE |
| created_at/updated_at | TIMESTAMPTZ | NOT NULL DEFAULT NOW() |

UNIQUE(auth_type_id_ref, template_version)。

### game_account_auth_configs（游戏维度业务表 / 每环境 schema）
```sql
CREATE TABLE game_account_auth_configs (
  id                 BIGSERIAL PRIMARY KEY,
  game_id_ref        BIGINT NOT NULL REFERENCES games(id),                       -- 同 schema 普通 FK
  auth_type_id_ref   BIGINT NOT NULL REFERENCES platform.account_auth_types(id), -- 跨 schema 指向平台表
  enabled            BOOLEAN NOT NULL DEFAULT FALSE,
  config_json        JSONB NOT NULL DEFAULT '{}'::jsonb,                          -- 含加密后密文位
  config_status      VARCHAR(16) NOT NULL DEFAULT 'empty' CHECK (config_status IN ('empty','invalid','valid')),
  last_check_at      TIMESTAMPTZ,
  last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, auth_type_id_ref)
);
-- INDEX(game_id_ref)
```
运行时连接 `search_path = <env>, platform`；业务表仓储 SQL 不写 schema 前缀、不带 env 谓词。

## 枚举与默认
- `ConfigStatus` ∈ {empty, invalid, valid}，默认 empty。
- enabled / default_enabled / locked 默认 FALSE。
- 四件套：form/secret/file 默认 `[]`，validation 默认 `{}`；config_json 默认 `{}`；last_check_message 默认 `''`；sort 默认 0。

## 业务规则与状态机（遵循 00 §3.4）
1. enabled=false 且无配置 → `empty`。
2. 写入 config_json 后按模板 validation_rules + 必填（含 secret/file 标记必填）校验：缺必填/敏感/文件字段或校验未过 → `invalid`（last_check_message 给出具体缺失项）；全过 → `valid`（记 last_check_at）。
3. 「只启用未填参数」必须落 `invalid` 并前端警告，不得静默 empty。
4. 渠道允许范围：取自 channel_account_auth_types；default_enabled=true 接入时默认勾选；locked=true 游戏侧不可关闭/修改。
5. 密文：secret_fields_json 标记字段加密存 config_json 密文位，响应脱敏（00 §6）。
- 纯领域规则：`ValidateConfigAgainstTemplate(config_json, template) -> (config_status, last_check_message)`（无 IO）。

## 后端 API（前缀 /api/admin，包络见 00 §7；读 game.read / 写 game.write）

GET `/api/admin/account-auth/types`
→ items[]: { authTypeId, authTypeName, enabled, sort, template:{ templateVersion, formSchema[], secretFields[], fileFields[], validationRules{} } }

GET `/api/admin/channels/{channelId}/account-auth-types`
→ items[]: { authTypeId, defaultEnabled, locked }

GET `/api/admin/games/{gameId}/account-auth-configs`（按当前 env）
→ items[]: { authTypeId, enabled, configJson(脱敏), configStatus, lastCheckAt, lastCheckMessage }

PUT `/api/admin/games/{gameId}/account-auth-configs`（game.write，整体替换式）
请求 items[]:
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| authTypeId | string | 是 | — | 必须属该游戏渠道允许集合 |
| enabled | bool | 否 | false | |
| configJson | object | 否 | {} | 按模板四件套校验 |
→ 成功返回各项计算后的 configStatus 与 lastCheckMessage。
→ 失败（缺敏感字段）：该项 configStatus=invalid，lastCheckMessage="缺少必填敏感字段或文件字段: clientSecret"。

form_schema 项结构: { key, label, component, required, order, scope("client"|"server"|"both") }。
示例 google: secretFields=["clientSecret"]；validationRules={ clientId:{minLen:1}, redirectUri:{format:"url"} }。

## 应用服务 / 仓储
- `AccountAuthService`：列类型/模板、列渠道允许类型、读写游戏配置（模板校验 + 密文加密 + 状态计算 + 审计）。
- 仓储：`AccountAuthTypeRepository`、`ChannelAccountAuthTypeRepository`、`AccountAuthTemplateRepository`、`GameAccountAuthConfigRepository`（业务表仓储，SQL 不写 schema 前缀 / 不带 env 谓词）。
- 审计事件：`game.account_auth.update` 写 audit_logs。

## 前端要点（游戏详情 → "自有账号认证" Tab）
- 列出该游戏允许的认证方式（渠道允许集合并集），每项：启用开关 + 模板驱动表单（统一渲染器消费四件套）。
- config_status 标签+颜色展示 empty/invalid/valid；启用但 invalid 行内警告。
- 密文字段脱敏展示、可重填；locked 项禁用编辑；无 game.write 置灰。

## 关键假设
- 游戏可启用类型 = 已启用渠道允许集合的并集 + locked 强制。
- phone/email 等无第三方密钥类型可空四件套，启用即 valid。
