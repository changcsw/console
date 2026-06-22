---
id: game-cashier
code: "18"
title: 游戏级收银台（Game Cashier）
status: target
code_paths:
  - services/admin-api/internal/domain/cashier
  - services/admin-api/internal/transport/http/cashier
  - apps/admin-web/src/views/cashier
depends_on: [cashier-template, game, common]
impacts: [payment, snapshot, sync, testing]
children: []
---

# 18 · 游戏级收银台（Game Cashier）

> 本模块负责"游戏绑定收银台模板版本快照 + 游戏级价格覆盖"。全局模板（`cashier-template`）定义公共价格矩阵，本模块负责**绑定某个版本的快照**与**个别覆盖**。金额按 `00 §5` 归一化。
> 阅读前建议先读 `../17-cashier-template/README.md`。

---

## 1. 模块概述与边界

### 1.1 职责
- 游戏绑定收银台模板及其某个版本快照 `game_cashier_profiles`。
- 游戏级价格覆盖 `game_cashier_price_overrides`（对模板矩阵的个别覆盖）。

### 1.2 边界
- 全局模板=公共价格矩阵（`cashier-template`，平台级、版本化）。
- 游戏级=**绑定模板版本快照（非实时跟随）** + 个别覆盖。
- 不涉及支付路由（`payment`）、IAP（`product`）。

---

## 2. 领域模型与聚合
- `GameCashier` 聚合（归属 Game）：
  - `Profile`：绑定的 `templateId` + `appliedTemplateVersionId` + `snapshotChecksum`。
  - `PriceOverrides`：游戏级价格覆盖行集合。
- 纯规则：价格覆盖金额归一化；覆盖优先级（游戏级覆盖 > 模板版本快照行）。

---

## 3. 数据模型

> 两表**带 env**（`00 §2.2`，D1）。

### 3.1 `game_cashier_profiles`（带 env）

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| **`env`** | VARCHAR(16) | 否 | — | **新增**，CHECK in `develop/sandbox/production` |
| `game_id_ref` | BIGINT | 否 | — | FK→games(id) |
| `template_id_ref` | BIGINT | 否 | — | FK→cashier_price_templates(id) |
| `applied_template_version_id` | BIGINT | 否 | — | FK→cashier_price_template_versions(id)（快照版本） |
| `snapshot_checksum` | VARCHAR(128) | 否 | `''` | 绑定时刻版本校验和 |
| `applied_at` | TIMESTAMPTZ | 否 | `NOW()` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键（D1）：`UNIQUE(env, game_id_ref)`（一个游戏一份绑定）。

**迁移（追加）**：
```sql
ALTER TABLE game_cashier_profiles
  ADD COLUMN env VARCHAR(16) NOT NULL DEFAULT 'develop' CHECK (env IN ('develop','sandbox','production'));
ALTER TABLE game_cashier_profiles DROP CONSTRAINT IF EXISTS game_cashier_profiles_game_id_ref_key;
ALTER TABLE game_cashier_profiles ADD CONSTRAINT gcp_env_game_key UNIQUE (env, game_id_ref);
```

### 3.2 `game_cashier_price_overrides`（带 env）

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| **`env`** | VARCHAR(16) | 否 | — | **新增** |
| `game_id_ref` | BIGINT | 否 | — | FK→games(id) |
| `country_code` | VARCHAR(8) | 否 | — | |
| `region_code` | VARCHAR(16) | 否 | `'*'` | |
| `currency` | VARCHAR(8) | 否 | — | 必须在 currency_specs |
| `price_id` | VARCHAR(64) | 否 | — | |
| `pre_tax_amount_minor` | BIGINT | 否 | — | 归一化 |
| `tax_rate` | DECIMAL(8,6) | 否 | — | |
| `tax_amount_minor` | BIGINT | 否 | — | |
| `after_tax_amount_minor` | BIGINT | 否 | — | |
| `reason` | VARCHAR(255) | 否 | `''` | 覆盖原因 |
| `effective_at` | TIMESTAMPTZ | 否 | — | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键（D1）：`UNIQUE(env, game_id_ref, country_code, region_code, currency, price_id)`。

**迁移（追加）**：
```sql
ALTER TABLE game_cashier_price_overrides
  ADD COLUMN env VARCHAR(16) NOT NULL DEFAULT 'develop' CHECK (env IN ('develop','sandbox','production'));
ALTER TABLE game_cashier_price_overrides DROP CONSTRAINT IF EXISTS game_cashier_price_overrides_game_id_ref_country_code_region_co_key;
ALTER TABLE game_cashier_price_overrides ADD CONSTRAINT gcpo_env_key
  UNIQUE (env, game_id_ref, country_code, region_code, currency, price_id);
```

---

## 4. 枚举与默认值清单

| 项 | 取值 / 默认 |
| --- | --- |
| `region_code` | 默认 `'*'` |
| `snapshot_checksum`/`reason` | 默认 `''` |
| 金额字段 | `*_amount_minor`，归一化整数最小单位 |
| `env` | 当前运行环境 |
| `applied_at` | 默认 `NOW()` |

---

## 5. 业务规则与状态机

### 5.1 绑定模板版本快照
- 游戏绑定 `templateId` 时，必须指定一个 `published` 版本作为 `applied_template_version_id`，并记录 `snapshot_checksum`（绑定时刻该版本校验和）。
- **绑定是快照，不实时跟随**：模板后续发布新版本，不会自动影响已绑定游戏，除非游戏主动"升级绑定版本"。
- 重新绑定/升级：更新 `applied_template_version_id` + `snapshot_checksum` + `applied_at`，写审计。

### 5.2 价格覆盖
- 游戏级覆盖行覆盖模板快照中相同 `(country, region, currency, price_id)` 的价格。
- 覆盖金额按 `00 §5` 归一化。
- 最终游戏价格 = 模板版本快照行 ← 被游戏级覆盖行整行覆盖（按唯一键匹配）。

---

## 6. 后端 API

> 前缀 `/api/admin/games/{gameId}/cashier`，遵循 `00 §7`。读 `cashier.read`，写 `cashier.write`。

- **GET `/profile`**
  ```json
  { "data": { "templateId": "global_default", "appliedTemplateVersion": "7",
    "snapshotChecksum": "sha256-...", "appliedAt": "2026-06-15T10:00:00Z" } }
  ```
- **PUT `/profile`** 权限 `cashier.write`
  | 字段 | 类型 | 必填 | 默认 | 校验 |
  | --- | --- | --- | --- | --- |
  | `templateId` | string | 是 | — | 模板必须存在 |
  | `templateVersion` | string | 是 | — | 必须是该模板的 `published` 版本 |
  ```json
  { "templateId": "global_default", "templateVersion": "7" }
  ```
- **GET `/price-overrides`**
- **PUT `/price-overrides`** 权限 `cashier.write`（整体替换式，金额归一化）
  ```json
  { "items": [
    { "countryCode": "JP", "regionCode": "*", "currency": "JPY", "priceId": "price_600",
      "preTaxAmountMinor": 600, "taxRate": 0.1, "taxAmountMinor": 60, "afterTaxAmountMinor": 660,
      "reason": "JP 本地定价", "effectiveAt": "2026-07-01T00:00:00Z" } ] }
  ```
错误码：`CURRENCY_NOT_SUPPORTED`、`VALIDATION_FAILED`、`NOT_FOUND`（模板/版本）、`CONFLICT`。

---

## 7. 应用服务与 command/query
- `GameCashierService`：绑定/升级模板版本（取版本快照 checksum）、读写价格覆盖（归一化、审计）。
- 仓储：`GameCashierProfileRepository`、`GameCashierPriceOverrideRepository`（均按 env）。

---

## 8. 前端信息架构
- 游戏详情 → "收银台" Tab：
  - 已绑定模板 + 模板版本 + 绑定时间；提供"切换/升级版本"。
  - 游戏级价格覆盖列表/编辑（金额受 `currency_specs` 约束、舍入预览）。
  - 清楚区分"模板公共矩阵"与"游戏级覆盖"边界（覆盖行高亮）。
- 空/错/权限态遵循全局；无 `cashier.write` 置灰。

---

## 9. 与公共能力的关系
- 金额归一化：`00 §5`。
- 审计：`cashier.profile.bind`、`cashier.override.update`。
- env：两表按当前运行环境；同步走 `cashier` section（`sync`）。
- 与 `cashier-template`：绑定的是 `cashier-template` 的某个 `published` 版本快照。

---

## 10. 测试要点

### 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/game-cashier.yaml`；前端 e2e：`tests/frontend/e2e/cashier.spec.ts`（游戏级绑定/覆盖）。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET `/api/admin/games/{gameId}/cashier/profile` | ✓ | ✓ | ✓ | — | — | ✓ | — | — | — | — | 绑定 published 版本快照(snapshot_checksum)、跨 env(带 env 表) |
| PUT `/api/admin/games/{gameId}/cashier/profile` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | 绑定 published 版本快照(snapshot_checksum)、跨 env(带 env 表) |
| GET `/api/admin/games/{gameId}/cashier/price-overrides` | ✓ | ✓ | ✓ | — | — | ✓ | — | — | ✓ | — | 游戏级价格覆盖、currency 归一化、跨 env(带 env 表) |
| PUT `/api/admin/games/{gameId}/cashier/price-overrides` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | ✓ | 游戏级价格覆盖、currency 归一化、金额最小单位、跨 env(带 env 表) |

前端：Playwright e2e 收银台 Tab（绑定/升级版本、覆盖编辑、空/错/无权限态置灰）/ vitest 覆盖行高亮与金额舍入预览组件。

### 补充关键用例
- 绑定快照不随模板新版本自动变化（验证 snapshot 语义）。
- 仅可绑定 `published` 版本。
- 价格覆盖金额归一化（JPY 无小数）。
- 覆盖按唯一键整行覆盖模板矩阵。

---

## 11. 未决问题与显式假设
- 假设一个游戏同一 env 只绑定一个收银台模板（`UNIQUE(env, game_id)`）。
- 假设"升级绑定版本"为显式动作，不自动跟随。
- 覆盖为整行覆盖语义（与渠道实例的实例级覆盖一致，不做字段级深合并）。
