---
id: game-cashier
code: "18"
title: 游戏级收银台（Game Cashier）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [cashier-template, game, common]
code_paths:
  - services/admin-api/internal/domain/cashier
  - services/admin-api/internal/transport/http/cashier
  - apps/admin-web/src/views/cashier
---

# 18 · 游戏级收银台 — Compact Spec

> 代码生成用精简规格。完整背景/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（env 模型/业务表 schema 隔离 §2、金额归一化 §5、统一包络/错误码 §7、审计 §8）。建议先读 `../17-cashier-template/README.md`。

## 边界
- 职责：游戏绑定收银台模板某个版本的**快照** `game_cashier_profiles` + **游戏级价格覆盖** `game_cashier_price_overrides`。
- 全局模板=公共价格矩阵（`cashier-template`，平台级、版本化）；本模块=绑定版本快照（**非实时跟随**）+ 个别覆盖。
- 不涉及支付路由（`payment`）、IAP（`product`）。

## 数据模型
两表均为**游戏维度业务表**，按 D1 在每个环境 schema（develop/sandbox/production）各一份同名同结构表，**不带 env 列**（行属于哪个 env 由所在 schema 决定，00 §2.2）。公共列：`id BIGSERIAL PK`、`created_at/updated_at TIMESTAMPTZ DEFAULT NOW()`、`game_id_ref BIGINT FK→games(id)`（同 schema 普通外键）。运行时连接 `search_path=<env>,platform`；业务表仓储 SQL 不写 schema 前缀、不带 env 谓词。

### game_cashier_profiles（每环境 schema，一游戏一份绑定）
| 列 | 类型 | 默认 | 约束 |
| --- | --- | --- | --- |
| template_id_ref | BIGINT | — | NOT NULL FK→platform.cashier_price_templates(id)（跨 schema 指向平台表） |
| applied_template_version_id | BIGINT | — | NOT NULL FK→platform.cashier_price_template_versions(id)（快照版本） |
| snapshot_checksum | VARCHAR(128) | `''` | NOT NULL 绑定时刻版本校验和 |
| applied_at | TIMESTAMPTZ | `NOW()` | NOT NULL |

唯一键：`UNIQUE(game_id_ref)`（env 由 schema 隔离，不前置 env）。
```sql
-- 迁移（追加，在每个环境 schema 内执行）：唯一键不前置 env
ALTER TABLE game_cashier_profiles DROP CONSTRAINT IF EXISTS game_cashier_profiles_game_id_ref_key;
ALTER TABLE game_cashier_profiles ADD CONSTRAINT gcp_game_key UNIQUE (game_id_ref);
```

### game_cashier_price_overrides（每环境 schema）
| 列 | 类型 | 默认 | 约束 |
| --- | --- | --- | --- |
| country_code | VARCHAR(8) | — | NOT NULL |
| region_code | VARCHAR(16) | `'*'` | NOT NULL |
| currency | VARCHAR(8) | — | NOT NULL，必须在 platform.currency_specs |
| price_id | VARCHAR(64) | — | NOT NULL |
| pre_tax_amount_minor | BIGINT | — | NOT NULL 归一化最小单位 |
| tax_rate | DECIMAL(8,6) | — | NOT NULL |
| tax_amount_minor | BIGINT | — | NOT NULL 归一化 |
| after_tax_amount_minor | BIGINT | — | NOT NULL 归一化 |
| reason | VARCHAR(255) | `''` | NOT NULL 覆盖原因 |
| effective_at | TIMESTAMPTZ | — | NOT NULL |

唯一键：`UNIQUE(game_id_ref, country_code, region_code, currency, price_id)`（不前置 env）。
```sql
-- 迁移（追加，在每个环境 schema 内执行）
ALTER TABLE game_cashier_price_overrides DROP CONSTRAINT IF EXISTS game_cashier_price_overrides_game_id_ref_country_code_region_co_key;
ALTER TABLE game_cashier_price_overrides ADD CONSTRAINT gcpo_key
  UNIQUE (game_id_ref, country_code, region_code, currency, price_id);
```

## 枚举与默认
- `region_code` 默认 `'*'`；`snapshot_checksum`/`reason` 默认 `''`；`applied_at` 默认 `NOW()`。
- 金额字段统一 `*_amount_minor`，归一化整数最小单位（00 §5）。

## 领域模型与业务规则
- `GameCashier` 聚合（归属 Game）：`Profile`（绑定 templateId + appliedTemplateVersionId + snapshotChecksum）+ `PriceOverrides`（覆盖行集合）。
- 纯规则：价格覆盖金额归一化；覆盖优先级（游戏级覆盖 > 模板版本快照行）。

### 绑定模板版本快照
1. 绑定 templateId 时必须指定一个 `published` 版本作 `applied_template_version_id`，并记录 `snapshot_checksum`（绑定时刻该版本校验和）。
2. **快照不实时跟随**：模板后续发布新版本不自动影响已绑定游戏，除非游戏主动「升级绑定版本」。
3. 重新绑定/升级：更新 `applied_template_version_id` + `snapshot_checksum` + `applied_at`，写审计。

### 价格覆盖
- 游戏级覆盖行按唯一键 `(country, region, currency, price_id)` **整行覆盖**模板快照同键价格（不做字段级深合并）。
- 覆盖金额按 00 §5 归一化。
- 最终游戏价格 = 模板版本快照行 ← 被游戏级覆盖行整行覆盖。

## 后端 API（前缀 /api/admin/games/{gameId}/cashier，包络 00 §7；读 cashier.read / 写 cashier.write）

GET `/profile`
→ data: { templateId, appliedTemplateVersion, snapshotChecksum, appliedAt }

PUT `/profile`（cashier.write）
| 字段 | 类型 | 必填 | 默认 | 校验 |
| --- | --- | --- | --- | --- |
| templateId | string | 是 | — | 模板必须存在 |
| templateVersion | string | 是 | — | 必须是该模板的 `published` 版本 |

GET `/price-overrides`
→ items[]（同 PUT 字段结构）

PUT `/price-overrides`（cashier.write，整体替换式，金额归一化）
items[] DTO: countryCode, regionCode(默认*), currency(须在 currency_specs), priceId, preTaxAmountMinor, taxRate, taxAmountMinor, afterTaxAmountMinor, reason(默认''), effectiveAt。

错误码：`CURRENCY_NOT_SUPPORTED`、`VALIDATION_FAILED`、`NOT_FOUND`（模板/版本）、`CONFLICT`。

## 应用服务 / 仓储
```go
type GameCashierService interface {
    GetProfile(ctx, gameID) (dto.ProfileDTO, error)
    BindProfile(ctx, gameID, cmd) (dto.ProfileDTO, error)        // 校验 published 版本→取快照 checksum→审计
    ListPriceOverrides(ctx, gameID) ([]dto.PriceOverrideDTO, error)
    SavePriceOverrides(ctx, gameID, cmd) ([]dto.PriceOverrideDTO, error) // 归一化→全量替换→审计
}
```
- 仓储：`GameCashierProfileRepository`、`GameCashierPriceOverrideRepository`（SQL 不写 schema 前缀、不带 env 谓词；写操作落当前运行环境对应 schema，由 search_path 决定）。
- 审计事件：`cashier.profile.bind`、`cashier.override.update`。

## 前端要点（游戏详情 → "收银台" Tab）
- 展示已绑定模板 + 模板版本 + 绑定时间；提供「切换/升级版本」。
- 游戏级价格覆盖列表/编辑（金额受 currency_specs 约束、舍入预览）。
- 清楚区分「模板公共矩阵」与「游戏级覆盖」边界（覆盖行高亮）。
- 空/错/权限态遵循全局；无 cashier.write 置灰。

## 与公共能力 / 下游
- 金额归一化 00 §5；审计 00 §8。
- env：两表为业务表、每环境独立 schema，写落当前 env schema（search_path 决定）。
- 与 `cashier-template`：绑定其某个 `published` 版本快照。
- sync：走 `cashier` section（跨 schema diff/upsert）。

## 关键假设
- 一个游戏在同一环境 schema 内只绑定一个收银台模板（`UNIQUE(game_id_ref)`）。
- 「升级绑定版本」为显式动作，不自动跟随模板新版本。
- 覆盖为整行覆盖语义（与渠道实例的实例级覆盖一致，不做字段级深合并）。
