---
id: cashier-template
code: "17"
title: 收银台模板与汇率同步（Cashier Template & FX Sync）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [common]
code_paths:
  - services/admin-api/internal/domain/cashier
  - services/admin-api/internal/transport/http/cashier
  - apps/admin-web/src/views/cashier
---

# 17 · 收银台模板与汇率同步 — Compact Spec

> 代码生成用精简规格。完整背景/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（版本生命周期 §3.3、金额归一化 §5、API 包络/错误码 §7、审计 §8、env 模型 §2.2）。
> 平台级"收银台价格模板"的版本化管理 + 汇率同步（FX，默认人工确认）。

## 边界
- 本模块是"公共价格矩阵"的定义方；游戏绑定的是**某版本的快照**（见 `game-cashier`），非实时跟随。
- 不涉及支付路由（`payment`）与渠道 IAP（`product`）。
- 所有表均为**平台级、放共享 schema `platform`、不带 env**，无需 env 迁移。

## 领域模型与聚合（internal/domain/cashier）
`CashierTemplate` 聚合：模板（fx 开关）+ 多版本（状态机）+ 版本下价格行集合 + 汇率同步运行。
值对象/纯规则（无 IO）：
- `TemplateVersion{ Version, Status }`。
- `CopyToDraft(nextVersion) TemplateVersion`：产物状态恒 `draft`。
- `CanTransition(from, to)`：仅允许 `draft→published`、`published→archived`。
- 金额归一化复用 `common` currency 工具。

## 数据模型（平台级，schema `platform`，均不带 env）
公共列约定：`id BIGSERIAL PK`、`created_at/updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`（下表不再重复列出）。

### cashier_price_templates
| 列 | 类型 | 默认 | 约束 |
| --- | --- | --- | --- |
| template_id | VARCHAR(64) | — | UNIQUE NOT NULL |
| template_name | VARCHAR(128) | — | NOT NULL |
| fx_sync_enabled | BOOLEAN | `TRUE` | NOT NULL |
| fx_sync_mode | VARCHAR(16) | `'manual_confirm'` | CHECK IN(`manual_confirm`,`auto_apply`) |
| fx_sync_schedule | VARCHAR(16) | `'monthly'` | CHECK IN(`monthly`,`quarterly`) |
| status | VARCHAR(32) | `'draft'` | 模板自身状态（启用/归档，**非版本状态**） |

### cashier_price_template_versions
| 列 | 类型 | 默认 | 约束 |
| --- | --- | --- | --- |
| template_id_ref | BIGINT | — | NOT NULL FK→cashier_price_templates(id) |
| version | VARCHAR(32) | — | NOT NULL |
| source_type | VARCHAR(32) | `'manual'` | `manual`/`copy_published`/`copy_archived`/`fx_auto` |
| auto_generated | BOOLEAN | `FALSE` | 汇率同步自动产生 |
| fx_base_date | DATE | `NULL` | 汇率基准日（可空） |
| status | VARCHAR(32) | `'draft'` | `draft`/`published`/`archived`（00 §3.3） |
| checksum | VARCHAR(128) | `''` | NOT NULL |
| published_at | TIMESTAMPTZ | `NULL` | 可空 |

UNIQUE(template_id_ref, version)。约束（应用层强制）：同一模板最多一个 `status=published`。

### cashier_price_rows
| 列 | 类型 | 默认 | 约束 |
| --- | --- | --- | --- |
| template_version_id_ref | BIGINT | — | NOT NULL FK→versions(id) |
| country_code | VARCHAR(8) | — | NOT NULL |
| region_code | VARCHAR(16) | `'*'` | NOT NULL |
| currency | VARCHAR(8) | — | NOT NULL，必须在 currency_specs |
| price_id | VARCHAR(64) | — | NOT NULL |
| pre_tax_amount_minor | BIGINT | — | 归一化后整数最小单位 |
| tax_rate | DECIMAL(8,6) | — | NOT NULL |
| tax_amount_minor | BIGINT | — | NOT NULL |
| after_tax_amount_minor | BIGINT | — | NOT NULL |
| effective_at | TIMESTAMPTZ | — | NOT NULL |

UNIQUE(template_version_id_ref, country_code, region_code, currency, price_id)。

### cashier_fx_sync_runs
| 列 | 类型 | 默认 | 约束 |
| --- | --- | --- | --- |
| template_id_ref | BIGINT | — | NOT NULL FK→templates(id) |
| candidate_version_id_ref | BIGINT | — | NOT NULL FK→versions(id)（候选 draft） |
| status | VARCHAR(16) | `'pending_review'` | CHECK IN(`pending_review`,`approved`,`applied`,`ignored`,`failed`) |
| diff_summary_json | JSONB | `{}` | NOT NULL 差异摘要 |
| triggered_at | TIMESTAMPTZ | `NOW()` | NOT NULL |
| reviewed_by | BIGINT | `NULL` | 可空 |
| reviewed_at | TIMESTAMPTZ | `NULL` | 可空 |
| review_note | VARCHAR(255) | `''` | NOT NULL |

## 枚举与默认
- `VersionStatus`: draft/published/archived，默认 draft。
- `FXSyncMode`: manual_confirm/auto_apply，默认 manual_confirm。
- `FXSyncSchedule`: monthly/quarterly，默认 monthly。
- `FXRunStatus`: pending_review/approved/applied/ignored/failed，默认 pending_review。
- `source_type`: manual/copy_published/copy_archived/fx_auto，默认 manual。
- auto_generated 默认 FALSE；fx_sync_enabled 默认 TRUE；region_code 默认 `'*'`；checksum/review_note 默认 `''`；diff_summary_json 默认 `{}`。

## 业务规则与状态机

### 版本生命周期（00 §3.3）
```text
draft --publish--> published --(发布新版本时自动)--> archived
允许：draft→published、published→archived
禁止：archived→published（需 copy-to-draft）、跳过 draft 直接生成正式版本
```
- 同一模板任一时刻最多一个 `published`。
- `published` 只读；改动需 `copy-to-draft`。
- 发布新 `published` 时，旧 `published` 自动转 `archived`（同事务）。
- 非法流转 → `VERSION_STATE_INVALID`。

### copy-to-draft
- 来源：空白 / 当前 `published` / 历史 `archived`。
- 产物状态恒 `draft`；复制价格行；复制后与来源不联动；`source_type` 标记来源。
- 领域实现：`command/copy_template_version.go` 的 `BuildDraftFromTemplateVersion(source, next)`。

### 汇率同步（FX，默认人工确认）
- 默认 `manual_confirm`：同步仅生成**候选 draft 版本** + `cashier_fx_sync_runs(status=pending_review)` + 差异摘要，**不自动应用**。
- 仅当 `fx_sync_mode=auto_apply`（显式开启）才允许自动 approve→apply。
- 审核流程：`pending_review` →(approve)→ `applied`；**`manual_confirm` 下 approve 即完成 apply（同一事务内发布候选版本并置 `applied`），不单设 apply 端点**（中间 `approved` 态仅在 `auto_apply` 或未来异步发布场景短暂出现）；或 `ignored`；异常 `failed`。
- `fx_sync_schedule` 控制触发周期（monthly/quarterly）。

### 金额归一化（00 §5）
- 价格行所有 `*_amount_minor` 写入：读 `currency_specs` → 校验精度/下限 → 舍入 → 存 minor。币种不支持 → `CURRENCY_NOT_SUPPORTED`。

## 后端 API（前缀 /api/admin/cashier，包络 00 §7；读 cashier.read / 写 cashier.write / 发布 cashier.publish / 汇率审核 fx.approve）

GET `/templates`（分页）→ items: {templateId, templateName, fxSyncEnabled, fxSyncMode, fxSyncSchedule, status}

POST `/templates`（cashier.write，审计 cashier.template.create）
```json
{ "templateId": "global_default", "templateName": "Global Default",
  "fxSyncEnabled": true, "fxSyncMode": "manual_confirm", "fxSyncSchedule": "monthly" }
```
templateId 唯一 → 冲突 `CONFLICT`。

GET `/templates/{templateId}`（含版本列表概要）

POST `/templates/{templateId}/versions`（cashier.write）产物 `status=draft`
| 字段 | 类型 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- | --- |
| sourceType | enum | 否 | `manual` | manual/copy_published/copy_archived |
| sourceVersion | string | copy 时必填 | `''` | 来源版本 |

POST `/templates/{templateId}/versions/{version}/copy-to-draft`（cashier.write）从 published/archived 复制为新 draft。
→ 201: `{ "data": { "version": "8", "status": "draft", "sourceType": "copy_published" } }`

GET / PUT `/templates/{templateId}/versions/{version}/rows`（cashier.write，PUT 批量 upsert + 金额归一化；published 只读 → `VERSION_STATE_INVALID`）

POST `/templates/{templateId}/versions/{version}/publish`（cashier.publish）校验当前为 draft；发布后旧 published 自动 archived；非法 → `VERSION_STATE_INVALID`。

POST `/templates/{templateId}/fx-sync/runs`（cashier.write）触发一次汇率同步，生成候选 draft + run(pending_review)。

POST `/fx-sync-runs/{runId}/approve`（fx.approve）审核通过；`manual_confirm` 下 approve 即完成 apply（发布候选版本），不单设 apply 端点。

错误码：`VERSION_STATE_INVALID`、`CURRENCY_NOT_SUPPORTED`、`VALIDATION_FAILED`、`CONFLICT`。

## 应用服务 / 仓储
- `CashierTemplateService`：模板/版本/价格行/发布/复制/汇率同步编排。
- 领域：`domain/cashier/template_version.go`（状态机、CopyToDraft）。
- 仓储：`CashierTemplateRepository`（模板/版本/行/run）。
- 审计事件：`cashier.template.create`、`cashier.version.publish`、`fx.approve` 写 audit_logs。

## 前端要点（顶层"收银台"路由）
- 页面：模板列表 / 模板详情 / 版本列表 / 价格矩阵编辑器 / 汇率同步审核列表。
- 版本列表 `TemplateVersionsTab.vue`：显示各版本状态；`published` 行提供"复制为 draft"入口（`CopyPublishedToDraftDialog.vue`）。
- 价格矩阵编辑器：金额输入受 `currency_specs` 约束（精度/下限/舍入预览）。
- 汇率同步审核：默认人工确认；展示差异摘要；approve/ignore 操作。
- 状态/空/错/权限态遵循全局；无对应权限置灰。

## 与公共能力 / 下游
- 版本生命周期 00 §3.3；金额归一化 00 §5；审计 00 §8。
- env：本模块平台级、共享 schema `platform`、不带 env；游戏绑定快照在 `game-cashier`（业务表，每环境独立 schema）。
- 下游 impacts：game-cashier / payment / snapshot / sync。

## 关键假设
- `version` 为字符串型自增（如 "1","2"…）；具体生成策略实现时统一。
- 汇率源（fx provider）抽象在 `infra/fx`，不限定具体数据源。
- 模板自身 status（templates.status，启用/归档）与版本 status（版本生命周期）是两层概念。
