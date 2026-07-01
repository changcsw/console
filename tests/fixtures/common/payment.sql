-- fixtures · 模块 payment（platform schema，共享基础数据）
-- 对应 tests/backend/scenarios/payment.yaml 的 fixture: common/payment/base
-- 幂等：ON CONFLICT DO NOTHING，可重复灌入。
--
-- 说明：
-- 1) 本文件只放 platform 共享数据（pay_ways/providers/subjects/merchant_accounts + RBAC）。
-- 2) payment_routes 属业务表（sandbox/production schema），连库场景由对应环境 fixture 负责。

-- ───────────────────────── 1) RBAC：payment_admin / payment_reader
INSERT INTO platform.admin_roles (role_code, role_name)
VALUES
  ('payment_admin', '支付路由管理员'),
  ('payment_reader', '支付路由只读')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id
FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code IN ('payment.read', 'payment.write')
WHERE r.role_code = 'payment_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id
FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code = 'payment.read'
WHERE r.role_code = 'payment_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- ───────────────────────── 2) pay_ways / providers / subjects
INSERT INTO platform.pay_ways (pay_way_id, pay_way_name, pay_way_type, enabled, sort)
VALUES
  ('credit_card', '信用卡', 'card', TRUE, 10),
  ('paypal', 'PayPal', 'wallet', TRUE, 20)
ON CONFLICT (pay_way_id) DO NOTHING;

INSERT INTO platform.cashier_providers (provider_id, provider_name, provider_kind, enabled, sort)
VALUES
  ('airwallex', 'Airwallex', 'aggregator', TRUE, 10),
  ('payermax', 'PayerMax', 'aggregator', TRUE, 20)
ON CONFLICT (provider_id) DO NOTHING;

INSERT INTO platform.billing_subjects (subject_id, subject_name, legal_entity_name, enabled)
VALUES
  ('hk_entity', 'HK Entity', 'HK Entity Limited', TRUE)
ON CONFLICT (subject_id) DO NOTHING;

-- ───────────────────────── 3) merchant accounts（secret_ciphertext 仅占位）
INSERT INTO platform.cashier_merchant_accounts
  (merchant_account_id, provider_id_ref, subject_id_ref, merchant_id, merchant_name, config_json, secret_ciphertext, enabled)
SELECT
  'merchant_aw_main',
  p.id,
  s.id,
  'AW-001',
  'Airwallex Main',
  '{}'::jsonb,
  'enc:fixture-aw',
  TRUE
FROM platform.cashier_providers p
JOIN platform.billing_subjects s ON s.subject_id = 'hk_entity'
WHERE p.provider_id = 'airwallex'
ON CONFLICT (merchant_account_id) DO NOTHING;

INSERT INTO platform.cashier_merchant_accounts
  (merchant_account_id, provider_id_ref, subject_id_ref, merchant_id, merchant_name, config_json, secret_ciphertext, enabled)
SELECT
  'merchant_pm_main',
  p.id,
  s.id,
  'PM-001',
  'PayerMax Main',
  '{}'::jsonb,
  'enc:fixture-pm',
  TRUE
FROM platform.cashier_providers p
JOIN platform.billing_subjects s ON s.subject_id = 'hk_entity'
WHERE p.provider_id = 'payermax'
ON CONFLICT (merchant_account_id) DO NOTHING;
