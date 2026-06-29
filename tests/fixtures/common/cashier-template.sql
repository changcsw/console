-- fixtures · 模块 cashier-template（platform schema，平台级、全 env 共享、业务行无 env 列）
-- 用于 tests/backend/scenarios/cashier-template.yaml 中 fixture: common/cashier-template/* 引用的连库 harness。
-- 平台四表（cashier_price_templates / _template_versions / _price_rows / cashier_fx_sync_runs）由
-- migrations/000007 建立；权限码 cashier.read/write/publish、fx.approve 由 migrations/000003 seed；
-- currency_specs（USD/JPY…）由 migrations/000002 seed。本文件补「测试专用」前置，幂等可重复灌入。
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   common/cashier-template/base                → RBAC 角色 + 模板 global_default（无版本）
--   common/cashier-template/with_draft          → base + global_default v1=draft（可写行/可发布/可 copy 拒绝源）
--   common/cashier-template/with_published      → base + global_default v1=published（验只读/copy-to-draft 源/fx 源）
--   common/cashier-template/publishable         → base + global_default v1=published + v2=draft（验发布归档旧 published）
--   common/cashier-template/auto_apply_published→ base + auto_tpl（fx_sync_mode=auto_apply）v1=published
--   common/cashier-template/fx_pending          → with_published + 候选 v2=draft + fx_run(pending_review,id=1)
--   common/cashier-template/fx_applied          → fx_run(applied,id=1)（验审核非 pending 冲突）
--
-- auth.role → RBAC 实体（cashier 接口权限码）：
--   cashier_admin  → cashier.read + cashier.write + cashier.publish + fx.approve
--   cashier_reader → 仅 cashier.read
--   no_perm        → 复用 common/auth.sql 的 no_perm（无任何权限）

-- ───────────────────────── 1) RBAC：收银台读写/发布/审核角色（幂等）
INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('cashier_admin', '收银台管理员'), ('cashier_reader', '收银台只读')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code IN ('cashier.read', 'cashier.write', 'cashier.publish', 'fx.approve')
WHERE r.role_code = 'cashier_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code = 'cashier.read'
WHERE r.role_code = 'cashier_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- ───────────────────────── 2) 模板 global_default（无版本）→ fixture: base
INSERT INTO platform.cashier_price_templates (template_id, template_name, fx_sync_enabled, fx_sync_mode, fx_sync_schedule, status)
VALUES ('global_default', 'Global Default', TRUE, 'manual_confirm', 'monthly', 'draft')
ON CONFLICT (template_id) DO NOTHING;

-- auto_tpl（auto_apply 模式）→ fixture: auto_apply_published 用
INSERT INTO platform.cashier_price_templates (template_id, template_name, fx_sync_enabled, fx_sync_mode, fx_sync_schedule, status)
VALUES ('auto_tpl', 'Auto Apply Template', TRUE, 'auto_apply', 'monthly', 'draft')
ON CONFLICT (template_id) DO NOTHING;

-- ───────────────────────── 3) 版本与价格行
-- 注：以下片段在「连库 harness」按所需 fixture 选择性灌入；为幂等，用 ON CONFLICT 跳过。
-- global_default v1：默认置 draft（with_draft）；with_published/fx_* 片段会将其更新为 published。
INSERT INTO platform.cashier_price_template_versions (template_id_ref, version, source_type, auto_generated, status, checksum)
SELECT t.id, '1', 'manual', FALSE, 'draft', ''
FROM platform.cashier_price_templates t WHERE t.template_id = 'global_default'
ON CONFLICT (template_id_ref, version) DO NOTHING;

-- global_default v1 价格行（USD），金额已 minor（10.00 USD = 1000，税率 0.1 → tax 100、afterTax 1100）。
INSERT INTO platform.cashier_price_rows
  (template_version_id_ref, country_code, region_code, currency, price_id, pre_tax_amount_minor, tax_rate, tax_amount_minor, after_tax_amount_minor, effective_at)
SELECT v.id, 'US', '*', 'USD', 'p_basic', 1000, 0.100000, 100, 1100, '2026-01-01T00:00:00Z'
FROM platform.cashier_price_template_versions v
JOIN platform.cashier_price_templates t ON t.id = v.template_id_ref
WHERE t.template_id = 'global_default' AND v.version = '1'
ON CONFLICT (template_version_id_ref, country_code, region_code, currency, price_id) DO NOTHING;

-- auto_tpl v1 = published（auto_apply_published）
INSERT INTO platform.cashier_price_template_versions (template_id_ref, version, source_type, auto_generated, status, checksum, published_at)
SELECT t.id, '1', 'manual', FALSE, 'published', '', NOW()
FROM platform.cashier_price_templates t WHERE t.template_id = 'auto_tpl'
ON CONFLICT (template_id_ref, version) DO NOTHING;

-- ───────────────────────── 4) 片段专属状态调整（连库 harness 按 fixture 名执行对应 UPDATE/INSERT）
-- with_published / publishable / fx_pending：global_default v1 → published
--   UPDATE platform.cashier_price_template_versions SET status='published', published_at=NOW()
--   WHERE version='1' AND template_id_ref=(SELECT id FROM platform.cashier_price_templates WHERE template_id='global_default');
-- publishable：再插入 v2=draft（待发布，验旧 published 自动归档）
--   INSERT ... version='2', status='draft' ...
-- fx_pending：插入候选 v2=draft + fx_run(status='pending_review') 关联 candidate=v2、id=1
-- fx_applied：fx_run(status='applied')
-- 上述按片段差异化执行的 SQL 由连库 harness 依据 fixture 名拼装（见 03-testing §7 fixtures 约定）；
-- 进程内 httptest（cashier_http_test.go）已通过真实 API 链式构造等价状态，无需 DB 即覆盖各维度。
