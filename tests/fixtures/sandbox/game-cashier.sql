-- fixtures · 模块 game-cashier（sandbox schema，游戏维度业务数据样本）
-- 用于 tests/backend/scenarios/game-cashier.yaml 中 fixture: sandbox/game-cashier/* 引用的连库 harness。
-- 两表（game_cashier_profiles / game_cashier_price_overrides）每环境 schema 各一份、不带 env 列；
-- 本文件灌入 sandbox schema（03-testing §7：sandbox/ = sandbox schema 业务数据样本）。
-- 幂等：ON CONFLICT DO NOTHING，可重复灌入。
--
-- 前置依赖（连库 harness 须先灌入）：
--   common/cashier-template.sql → platform.cashier_price_templates('global_default') + 版本 v1
--   sandbox/game.sql            → sandbox.games('100001')  ← profile/override 的 game_id_ref 来源
--   migrations/000002           → platform.currency_specs(USD/JPY…)  ← override.currency FK 来源
--   migrations/000012           → sandbox.game_cashier_profiles / game_cashier_price_overrides 建表
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   sandbox/game-cashier/base                      → 仅 RBAC + 游戏 100001（无 profile、无 override）
--   sandbox/game-cashier/with_draft                → global_default v1=draft（验绑定 draft → CONFLICT）
--   sandbox/game-cashier/publishable_with_checksum → global_default v1=published 且 checksum 非空
--                                                    （验绑定成功；规避「发布流程不计算 checksum」缺陷）
--   sandbox/game-cashier/bound                     → publishable_with_checksum + 100001 已绑定 v1
--   sandbox/game-cashier/with_overrides            → bound + 100001 一条 USD 覆盖行（验列表/替换/回滚）
--
-- auth.role → RBAC 实体（复用 common/cashier-template.sql 的 cashier_admin/cashier_reader；
--   no_perm 复用 common/auth.sql）：cashier_admin=read+write+publish+fx.approve、cashier_reader=read。
--   本模块仅用到 cashier.read / cashier.write。

SET search_path TO sandbox, platform;

-- ───────────────────────── 片段：with_draft —— global_default v1 = draft
-- （cashier-template.sql 已以 ON CONFLICT 插入 v1=draft；此处显式确保 draft 态）
UPDATE platform.cashier_price_template_versions v
SET status = 'draft', published_at = NULL
FROM platform.cashier_price_templates t
WHERE t.id = v.template_id_ref AND t.template_id = 'global_default' AND v.version = '1';

-- ───────────────────────── 片段：publishable_with_checksum —— global_default v1 = published + 非空 checksum
-- 注：当前 cashier-template 发布流程不计算 checksum（service 恒置 ''），BindProfile 要求非空 checksum，
-- 故 seed 一个确定性 checksum 以验证「绑定成功」语义。生产路径缺陷见模块 handoff（回退 🟦后端开发）。
UPDATE platform.cashier_price_template_versions v
SET status = 'published', published_at = NOW(), checksum = 'fixture-checksum-gd-v1'
FROM platform.cashier_price_templates t
WHERE t.id = v.template_id_ref AND t.template_id = 'global_default' AND v.version = '1';

-- ───────────────────────── 片段：bound —— 游戏 100001 绑定 global_default v1
INSERT INTO sandbox.game_cashier_profiles
  (game_id_ref, template_id_ref, applied_template_version_id, snapshot_checksum, applied_at)
SELECT g.id, t.id, v.id, 'fixture-checksum-gd-v1', NOW()
FROM sandbox.games g
JOIN platform.cashier_price_templates t ON t.template_id = 'global_default'
JOIN platform.cashier_price_template_versions v ON v.template_id_ref = t.id AND v.version = '1'
WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref) DO NOTHING;

-- ───────────────────────── 片段：with_overrides —— 游戏 100001 一条 USD 覆盖行
-- 金额已 minor（10.00 USD = 1000，税率 0.1 → tax 100、afterTax 1100）。
INSERT INTO sandbox.game_cashier_price_overrides
  (game_id_ref, country_code, region_code, currency, price_id,
   pre_tax_amount_minor, tax_rate, tax_amount_minor, after_tax_amount_minor, reason, effective_at)
SELECT g.id, 'US', '*', 'USD', 'p_basic', 1000, 0.100000, 100, 1100, 'fixture', '2026-01-01T00:00:00Z'
FROM sandbox.games g WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, country_code, region_code, currency, price_id) DO NOTHING;

-- ───────────────────────── 说明（连库 harness 按 fixture 名差异化执行）
-- base：不执行上述 profile/override/published 片段（仅依赖 game.sql + cashier-template.sql 的基线）。
-- with_draft：执行「with_draft」UPDATE（v1=draft）。
-- publishable_with_checksum：执行「publishable_with_checksum」UPDATE（v1=published+checksum）。
-- bound：publishable_with_checksum + 「bound」INSERT。
-- with_overrides：bound + 「with_overrides」INSERT。
-- 进程内 httptest（game_cashier_http_test.go）已通过真实 API 链式构造 + 白盒注入 checksum 等价覆盖各维度，无需 DB。
