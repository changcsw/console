---
id: cashier-template
code: "17"
title: 收银台模板与汇率同步（Cashier Template & FX Sync）
status: target
code_paths:
  - services/admin-api/internal/domain/cashier
  - services/admin-api/internal/transport/http/cashier
  - apps/admin-web/src/views/cashier
depends_on: [common]
impacts: [game-cashier, payment, snapshot, sync, testing]
children: []
---

# 17 · 收银台模板与汇率同步（Cashier Template & FX Sync）

> 本模块定义平台级"收银台价格模板"的版本化管理与汇率同步（FX）流程。版本生命周期严格遵循 `00 §3.3`（`draft/published/archived`）。金额一律按 `00 §5` 归一化。
> 这些表是**平台级、不带 env**；游戏如何绑定模板见 `17-游戏级收银台`。

---

## 1. 模块概述与边界

### 1.1 职责
- 维护收银台价格模板 `cashier_price_templates`。
- 维护模板版本 `cashier_price_template_versions`（版本生命周期）。
- 维护版本下的价格行 `cashier_price_rows`（国家/地区/币种/价格档矩阵）。
- 维护汇率同步运行记录 `cashier_fx_sync_runs`（默认人工确认）。

### 1.2 边界
- 本模块是"公共价格矩阵"的定义方；游戏绑定的是**某个版本的快照**（`game-cashier`），非实时跟随。
- 不涉及支付路由（`payment`）与渠道 IAP（`product`）。

---

## 2. 领域模型与聚合

`CashierTemplate` 聚合：
- 模板（`fx_sync_enabled/mode/schedule` 等开关）。
- 多个版本（`TemplateVersion`，状态机）。
- 版本下的价格行集合。
- 汇率同步运行。

值对象/纯规则：
- `TemplateVersion{ Version, Status }` + `CopyToDraft(nextVersion) TemplateVersion`（产物恒 `draft`）。
- `CanTransition(from, to)`：仅允许 `draft→published`、`published→archived`。
- 金额归一化复用 `common` 的 currency 工具。

---

## 3. 数据模型（平台级，均不带 env）

### 3.1 `cashier_price_templates`

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `template_id` | VARCHAR(64) | 否 | — | UNIQUE |
| `template_name` | VARCHAR(128) | 否 | — | |
| `fx_sync_enabled` | BOOLEAN | 否 | `TRUE` | |
| `fx_sync_mode` | VARCHAR(16) | 否 | `'manual_confirm'` | CHECK in `manual_confirm/auto_apply` |
| `fx_sync_schedule` | VARCHAR(16) | 否 | `'monthly'` | CHECK in `monthly/quarterly` |
| `status` | VARCHAR(32) | 否 | `'draft'` | 模板自身状态（模板可被禁用/归档，非版本状态） |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

### 3.2 `cashier_price_template_versions`

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `template_id_ref` | BIGINT | 否 | — | FK→cashier_price_templates(id) |
| `version` | VARCHAR(32) | 否 | — | |
| `source_type` | VARCHAR(32) | 否 | `'manual'` | `manual`/`copy_published`/`copy_archived`/`fx_auto` |
| `auto_generated` | BOOLEAN | 否 | `FALSE` | 汇率同步自动产生 |
| `fx_base_date` | DATE | 是 | `NULL` | 汇率基准日 |
| `status` | VARCHAR(32) | 否 | `'draft'` | `draft/published/archived`（`00 §3.3`） |
| `checksum` | VARCHAR(128) | 否 | `''` | |
| `published_at` | TIMESTAMPTZ | 是 | `NULL` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(template_id_ref, version)`。约束（应用层强制）：同一模板最多一个 `status=published`。

### 3.3 `cashier_price_rows`

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `template_version_id_ref` | BIGINT | 否 | — | FK→versions(id) |
| `country_code` | VARCHAR(8) | 否 | — | |
| `region_code` | VARCHAR(16) | 否 | `'*'` | |
| `currency` | VARCHAR(8) | 否 | — | 必须在 currency_specs |
| `price_id` | VARCHAR(64) | 否 | — | |
| `pre_tax_amount_minor` | BIGINT | 否 | — | 归一化后整数最小单位 |
| `tax_rate` | DECIMAL(8,6) | 否 | — | |
| `tax_amount_minor` | BIGINT | 否 | — | |
| `after_tax_amount_minor` | BIGINT | 否 | — | |
| `effective_at` | TIMESTAMPTZ | 否 | — | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

唯一键：`UNIQUE(template_version_id_ref, country_code, region_code, currency, price_id)`。

### 3.4 `cashier_fx_sync_runs`

| 列 | 类型 | 可空 | 默认 | 约束 |
| --- | --- | --- | --- | --- |
| `id` | BIGSERIAL | 否 | — | PK |
| `template_id_ref` | BIGINT | 否 | — | FK→templates(id) |
| `candidate_version_id_ref` | BIGINT | 否 | — | FK→versions(id)（候选 draft） |
| `status` | VARCHAR(16) | 否 | — | CHECK in `pending_review/approved/applied/ignored/failed` |
| `diff_summary_json` | JSONB | 否 | `{}` | 差异摘要 |
| `triggered_at` | TIMESTAMPTZ | 否 | `NOW()` | |
| `reviewed_by` | BIGINT | 是 | `NULL` | |
| `reviewed_at` | TIMESTAMPTZ | 是 | `NULL` | |
| `review_note` | VARCHAR(255) | 否 | `''` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | 否 | `NOW()` | |

> 这些表不带 env（`00 §2.2`）。无需新增 env 迁移。

---

## 4. 枚举与默认值清单

| 项 | 取值 / 默认 |
| --- | --- |
| `VersionStatus` | `draft`/`published`/`archived`，默认 `draft` |
| `FXSyncMode` | `manual_confirm`/`auto_apply`，默认 `manual_confirm` |
| `FXSyncSchedule` | `monthly`/`quarterly`，默认 `monthly` |
| `FXRunStatus` | `pending_review`/`approved`/`applied`/`ignored`/`failed`，默认 `pending_review` |
| `source_type` | `manual`/`copy_published`/`copy_archived`/`fx_auto`，默认 `manual` |
| `auto_generated` | 默认 `FALSE` |
| `fx_sync_enabled` | 默认 `TRUE` |
| `region_code` | 默认 `'*'` |
| `checksum`/`review_note` | 默认 `''` |
| `diff_summary_json` | 默认 `{}` |

---

## 5. 业务规则与状态机

### 5.1 版本生命周期（`00 §3.3`）
```text
draft --publish--> published --(发布新版本时自动)--> archived

允许：draft→published、published→archived
禁止：archived→published（需 copy-to-draft）、跳过 draft 直接生成正式版本
```
- 同一模板任一时刻最多一个 `published`。
- `published` 只读；改动需 `copy-to-draft`。
- 发布新 `published` 时，旧 `published` 自动转 `archived`。
- 非法流转 → `VERSION_STATE_INVALID`。

### 5.2 copy-to-draft
- 来源：空白 / 当前 `published` / 历史 `archived`。
- 产物状态恒 `draft`；复制价格行；复制后与来源不联动；`source_type` 标记来源。

### 5.3 汇率同步（FX）
- 默认 `manual_confirm`：同步仅生成**候选 draft 版本** + `cashier_fx_sync_runs(status=pending_review)` + 差异摘要，**不自动应用**。
- 仅当 `fx_sync_mode=auto_apply`（显式开启）才允许自动 approve→apply。
- 审核流程：`pending_review` →（approve）`approved` →（apply 即发布候选版本）`applied`；或 `ignored`；异常 `failed`。
- `fx_sync_schedule` 控制触发周期（`monthly/quarterly`）。

### 5.4 金额归一化
- 价格行所有 `*_amount_minor` 写入按 `00 §5`：读 `currency_specs` → 校验精度/下限 → 舍入 → 存 minor。币种不支持 → `CURRENCY_NOT_SUPPORTED`。

---

## 6. 后端 API

> 前缀 `/api/admin/cashier`，遵循 `00 §7` 包络。读 `cashier.read`，写 `cashier.write`，发布 `cashier.publish`，审核 `cashier.approve`。

- **GET `/templates`** / **POST `/templates`**（创建模板）
  ```json
  // POST /templates
  { "templateId": "global_default", "templateName": "Global Default",
    "fxSyncEnabled": true, "fxSyncMode": "manual_confirm", "fxSyncSchedule": "monthly" }
  ```
- **GET `/templates/{templateId}`**（含版本列表概要）
- **POST `/templates/{templateId}/versions`** 权限 `cashier.write`
  | 字段 | 类型 | 必填 | 默认 | 说明 |
  | --- | --- | --- | --- | --- |
  | `sourceType` | enum | 否 | `manual` | `manual`/`copy_published`/`copy_archived` |
  | `sourceVersion` | string | 当 copy 时必填 | `''` | 来源版本 |
  产物 `status=draft`。
- **POST `/templates/{templateId}/versions/{version}/copy-to-draft`** 权限 `cashier.write`。从 `published`/`archived` 复制为新 `draft`。
  成功 `201`：`{ "data": { "version": "8", "status": "draft", "sourceType": "copy_published" } }`
- **GET `/templates/{templateId}/versions/{version}/rows`** / **PUT .../rows**（批量 upsert，金额归一化）权限 `cashier.write`
- **POST `/templates/{templateId}/versions/{version}/publish`** 权限 `cashier.publish`
  - 校验当前为 `draft`；发布后旧 `published` 自动 `archived`；非法 → `VERSION_STATE_INVALID`。
- **POST `/templates/{templateId}/fx-sync/runs`** 权限 `cashier.write`：触发一次汇率同步，生成候选 draft + run(pending_review)。
- **POST `/fx-sync-runs/{runId}/approve`** 权限 `cashier.approve`：审核通过；`manual_confirm` 下 approve 后再 apply（发布候选版本）。

错误码：`VERSION_STATE_INVALID`、`CURRENCY_NOT_SUPPORTED`、`VALIDATION_FAILED`、`CONFLICT`。

---

## 7. 应用服务与 command/query
- `CashierTemplateService`：模板/版本/价格行/发布/复制/汇率同步。
- `command/copy_template_version.go`：`BuildDraftFromTemplateVersion(source, next)`。
- 领域：`domain/cashier/template_version.go`（状态机、CopyToDraft）。
- 仓储：`CashierTemplateRepository`（模板/版本/行/run）。

---

## 8. 前端信息架构
- 顶层"收银台"路由：模板列表 / 模板详情 / 版本列表 / 价格矩阵编辑器 / 汇率同步审核列表。
- 版本列表 `TemplateVersionsTab.vue`：显示各版本状态；`published` 行提供"复制为 draft"直接入口（`CopyPublishedToDraftDialog.vue`）。
- 价格矩阵编辑器：金额输入受 `currency_specs` 约束（精度/下限/舍入预览）。
- 汇率同步审核：默认人工确认；展示差异摘要；approve/ignore 操作。
- 状态/空/错/权限态遵循全局；无对应权限置灰。

---

## 9. 与公共能力的关系
- 版本生命周期：`00 §3.3`。
- 金额归一化：`00 §5`。
- 审计：`cashier.template.create`、`cashier.version.publish`、`cashier.fx.approve` 等写 `audit_logs`。
- env：本模块平台级不带 env；游戏绑定快照在 `game-cashier`（带 env）。

---

## 10. 测试要点
- 版本流转：`draft→published` 通过；`archived→published`、跳过 draft 被拒（`VERSION_STATE_INVALID`）。
- 发布新版本旧 published 自动 archived；同模板仅一个 published。
- copy-to-draft 产物为 draft、复制行、不联动。
- 金额归一化：JPY 不允许小数；低于 min 被拒。
- FX 默认 manual_confirm：仅生成候选 + pending_review，不自动应用。

---

## 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。本模块为**平台级表、无 env**，故 S6 多数标 `—`（平台级无 env）。后端 manifest：`tests/backend/scenarios/cashier-template.yaml`；前端 e2e：`tests/frontend/e2e/cashier.spec.ts`。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET `/templates` | ✓ | ✓ | ✓ | — | — | — | — | — | ✓ | — | — |
| POST `/templates` | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | — | `template_id` 唯一冲突 |
| GET `/templates/{templateId}` | ✓ | ✓ | ✓ | ✓ | — | — | — | — | — | — | — |
| POST `/templates/{templateId}/versions` | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | `UNIQUE(template,version)`；创建+复制行事务 |
| POST `/templates/{templateId}/versions/{version}/copy-to-draft` | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | copy-to-draft 产物恒 draft、复制行事务 |
| GET `/templates/{templateId}/versions/{version}/rows` | ✓ | ✓ | ✓ | — | — | — | — | — | ✓ | — | — |
| PUT `/templates/{templateId}/versions/{version}/rows` | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | currency 归一化（`CURRENCY_NOT_SUPPORTED`）；published 只读（`VERSION_STATE_INVALID`）；批量 upsert 事务 |
| POST `/templates/{templateId}/versions/{version}/publish` | ✓ | ✓ | ✓ | — | ✓ | — | ✓ | — | — | ✓ | 版本生命周期 `draft→published`（`VERSION_STATE_INVALID`）；发布唯一 published、旧 published 自动 archived 事务 |
| POST `/templates/{templateId}/fx-sync/runs` | ✓ | ✓ | ✓ | — | — | — | ✓ | — | — | ✓ | FX 默认 manual_confirm：仅生成候选 draft + run(pending_review) 事务 |
| POST `/fx-sync-runs/{runId}/approve` | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | — | ✓ | FX 人工确认/审核 `pending_review→approved/applied`、apply 发布候选事务 |

前端：Playwright e2e 覆盖模板列表/详情、版本列表 `TemplateVersionsTab.vue`（published 行「复制为 draft」`CopyPublishedToDraftDialog.vue`）、价格矩阵编辑器（currency 精度/下限/舍入预览）、汇率同步审核列表（差异摘要 + approve/ignore）状态/空/错/权限态 / vitest 组件覆盖价格矩阵编辑器与汇率审核组件。

---

## 11. 未决问题与显式假设
- 假设 `version` 为字符串型自增（如 "1","2"…）；具体生成策略实现时统一。
- 假设汇率源（fx provider）抽象在 `infra/fx`，本文不限定具体数据源。
- 模板自身 `status`（templates.status）与版本 `status` 是两层概念：前者表示模板是否启用/归档，后者是版本生命周期。
