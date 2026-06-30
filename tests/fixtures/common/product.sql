-- fixtures · 模块 product（platform schema，全 env 共享）
-- 用于 tests/backend/scenarios/product.yaml 中 fixture: common/product/* 引用的连库 harness。
-- 本文件由 scripts/regression/db.sh 自动灌入（common/*.sql）。幂等：ON CONFLICT DO NOTHING，可重复灌入。
--
-- 内容：
--   1) RBAC：product 专用权限码 product.read / product.write + 角色 product_admin / product_reader。
--   2) 平台级渠道 IAP 模板 channel_iap_templates（google 渠道 v1：四件套 appId 必填 + privateKey 密文）。
--      模板内容由基础数据/模板后台维护，本模块只消费；此处为测试前置。
--
-- product.yaml auth.role → RBAC 实体：
--   product_admin  → product.read + product.write（S1/S5/S6/S7/S8/S9/S10 跑通）
--   product_reader → 仅 product.read（验缺写权限 403；可读列表/详情/配置）
--   no_perm        → 复用 common/auth.sql 的 no_perm（无任何权限）

-- ───────────────────────── 1) RBAC：product 权限码 + 角色（platform schema）
INSERT INTO platform.admin_permissions (permission_code, permission_name)
VALUES ('product.read', '商品-读'), ('product.write', '商品-写')
ON CONFLICT (permission_code) DO NOTHING;

INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('product_admin', '商品管理员'), ('product_reader', '商品只读')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code IN ('product.read', 'product.write')
WHERE r.role_code = 'product_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code = 'product.read'
WHERE r.role_code = 'product_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- no_perm 角色（与 common/auth.sql 同源，幂等补齐，避免依赖加载顺序）
INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('no_perm', '无权限角色')
ON CONFLICT (role_code) DO NOTHING;

-- ───────────────────────── 2) 平台级渠道 IAP 模板（common/product/template）
-- google 渠道 v1 模板：appId 普通必填字段；privateKey 密文必填字段（验脱敏 S8 + config_status 推导）。
-- 简单模板表（00 §4.4.1）：无 status 列，取 enabled 最新版本。
INSERT INTO platform.channel_iap_templates
  (channel_id_ref, template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json, enabled)
SELECT ch.id, 'v1',
  '[{"key":"appId","label":"App ID","component":"input","required":true,"order":10},
    {"key":"privateKey","label":"Private Key","component":"password","required":true,"order":20}]'::jsonb,
  '["privateKey"]'::jsonb,
  '[]'::jsonb,
  '{"appId":{"required":true,"minLen":1}}'::jsonb,
  TRUE
FROM platform.channels ch
WHERE ch.channel_id = 'google'
ON CONFLICT (channel_id_ref, template_version) DO NOTHING;
