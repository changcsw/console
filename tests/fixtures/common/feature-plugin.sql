-- fixtures · 模块 feature-plugin（功能插件 / platform schema，全 env 共享）
-- 用于 tests/backend/scenarios/feature-plugin.yaml 中 fixture: common/feature-plugin/* 引用的连库 harness。
-- 平台前置：
--   - 平台三表（feature_plugins / feature_plugin_templates / channel_feature_plugins）由 000012 建表。
--   - channels（含 region/market 场景）由 000002 seed：huawei_cn 为 CN（domestic）渠道。
-- 本文件补「测试专用」前置：RBAC 角色 + 插件主数据/模板/渠道策略，幂等可重复灌入（ON CONFLICT DO NOTHING）。
--
-- 引用约定（manifest fixture/auth → 本文件片段）：
--   auth.role：
--     plugin_reader → plugin.read（验 S1/S8 读；缺写权限 S3 403 写）
--     plugin_admin  → plugin.read + plugin.write（验 S1/S4/S5/S7/S8/S10 写）
--     no_perm       → 复用 common/auth.sql 的 no_perm（无任何权限 → 读写均 403）
--   fixture: common/feature-plugin/catalog → 插件目录 + 渠道策略就绪（必接/locked/海外不兼容样本）

-- ───────────────────────── 1) RBAC：插件读写权限与角色（幂等）
INSERT INTO platform.admin_permissions (permission_code, permission_name)
VALUES ('plugin.read', '功能插件-读'), ('plugin.write', '功能插件-写')
ON CONFLICT (permission_code) DO NOTHING;

INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('plugin_admin', '插件管理员'), ('plugin_reader', '插件只读')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code IN ('plugin.read', 'plugin.write')
WHERE r.role_code = 'plugin_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code = 'plugin.read'
WHERE r.role_code = 'plugin_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- ───────────────────────── 2) 插件主数据（feature_plugins）
--   realname            : 国内实名插件（必接样本）
--   customer_service    : 国内客服插件（可选样本）
--   locked_plugin       : 国内锁定插件（locked=true 样本）
--   overseas_only_plugin: 海外插件（CN 渠道下不兼容样本，验 MARKET_CHANNEL_INCOMPATIBLE）
INSERT INTO platform.feature_plugins (plugin_id, plugin_name, region, enabled, sort)
VALUES
  ('realname',             '实名认证',   'domestic', TRUE, 10),
  ('customer_service',     '客服',       'domestic', TRUE, 20),
  ('locked_plugin',        '锁定插件',   'domestic', TRUE, 30),
  ('overseas_only_plugin', '海外专用插件','overseas', TRUE, 40)
ON CONFLICT (plugin_id) DO NOTHING;

-- ───────────────────────── 3) 插件模板四件套（feature_plugin_templates，enabled 最新版本生效）
--   realname/customer_service v1：appId 普通必填（含 validation_rules）+ appSecret 密文必填（scope=server）
INSERT INTO platform.feature_plugin_templates (
  plugin_id_ref, template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json, enabled
)
SELECT fp.id, 'v1',
  '[{"key":"appId","label":"App ID","component":"input","required":true,"order":10,"group":"basic","scope":"both"},
    {"key":"appSecret","label":"App Secret","component":"password","required":true,"order":20,"group":"secret","scope":"server"}]'::jsonb,
  '["appSecret"]'::jsonb,
  '[]'::jsonb,
  '{"appId":{"minLen":1,"maxLen":64,"pattern":"^[0-9A-Za-z_-]+$"}}'::jsonb,
  TRUE
FROM platform.feature_plugins fp
WHERE fp.plugin_id IN ('realname', 'customer_service', 'locked_plugin', 'overseas_only_plugin')
ON CONFLICT (plugin_id_ref, template_version) DO NOTHING;

-- ───────────────────────── 4) 渠道策略（channel_feature_plugins），挂到 huawei_cn（CN/domestic 渠道）
--   realname        : required=TRUE, selectable=FALSE（必接不可取消勾选 → 必接缺口/不可取消用例）
--   customer_service: 普通可选（default_enabled=FALSE）
--   locked_plugin   : locked=TRUE（游戏侧不可改）
--   overseas_only_plugin: 刻意挂到 CN 渠道以构造 region 不兼容（运行态/写校验拒绝）
INSERT INTO platform.channel_feature_plugins (channel_id_ref, plugin_id_ref, required, selectable, default_enabled, locked, sort, enabled)
SELECT ch.id, fp.id, TRUE, FALSE, TRUE, FALSE, 10, TRUE
FROM platform.channels ch JOIN platform.feature_plugins fp ON fp.plugin_id = 'realname'
WHERE ch.channel_id = 'huawei_cn'
ON CONFLICT (channel_id_ref, plugin_id_ref) DO NOTHING;

INSERT INTO platform.channel_feature_plugins (channel_id_ref, plugin_id_ref, required, selectable, default_enabled, locked, sort, enabled)
SELECT ch.id, fp.id, FALSE, TRUE, FALSE, FALSE, 20, TRUE
FROM platform.channels ch JOIN platform.feature_plugins fp ON fp.plugin_id = 'customer_service'
WHERE ch.channel_id = 'huawei_cn'
ON CONFLICT (channel_id_ref, plugin_id_ref) DO NOTHING;

INSERT INTO platform.channel_feature_plugins (channel_id_ref, plugin_id_ref, required, selectable, default_enabled, locked, sort, enabled)
SELECT ch.id, fp.id, FALSE, TRUE, FALSE, TRUE, 30, TRUE
FROM platform.channels ch JOIN platform.feature_plugins fp ON fp.plugin_id = 'locked_plugin'
WHERE ch.channel_id = 'huawei_cn'
ON CONFLICT (channel_id_ref, plugin_id_ref) DO NOTHING;

INSERT INTO platform.channel_feature_plugins (channel_id_ref, plugin_id_ref, required, selectable, default_enabled, locked, sort, enabled)
SELECT ch.id, fp.id, FALSE, TRUE, FALSE, FALSE, 40, TRUE
FROM platform.channels ch JOIN platform.feature_plugins fp ON fp.plugin_id = 'overseas_only_plugin'
WHERE ch.channel_id = 'huawei_cn'
ON CONFLICT (channel_id_ref, plugin_id_ref) DO NOTHING;
