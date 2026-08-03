-- 000020 · system 侧「功能插件管理」：插件分类字典 + 插件主数据归属分类 + 管理权限码
-- 背景：000012 已建 platform.feature_plugins / feature_plugin_templates（平台级主数据与模板四件套），
--   但插件分类此前只在前端硬编码。分类需由系统管理员自行增删改，故新建字典表 feature_plugin_categories，
--   并给 feature_plugins 增加可空外键 category_id_ref（可空以兼容既有未归类数据）。
-- 权限：feature_plugin.read / feature_plugin.write 是 system 侧插件主数据维护权限，
--   与游戏侧渠道实例插件配置用的 plugin.read / plugin.write 相互独立，不可混用。
-- 幂等：IF NOT EXISTS + ON CONFLICT DO NOTHING，可重复执行。

CREATE TABLE IF NOT EXISTS platform.feature_plugin_categories (
  id            BIGSERIAL PRIMARY KEY,
  category_code VARCHAR(64) NOT NULL UNIQUE,
  category_name VARCHAR(64) NOT NULL,
  enabled       BOOLEAN     NOT NULL DEFAULT TRUE,
  sort          INT         NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_feature_plugin_categories_enabled_sort
  ON platform.feature_plugin_categories(enabled, sort);

ALTER TABLE platform.feature_plugins
  ADD COLUMN IF NOT EXISTS category_id_ref BIGINT NULL
  REFERENCES platform.feature_plugin_categories(id);

CREATE INDEX IF NOT EXISTS idx_feature_plugins_category
  ON platform.feature_plugins(category_id_ref);

-- 初始分类字典（管理员后续可自行增删改）。
INSERT INTO platform.feature_plugin_categories (category_code, category_name, sort) VALUES
  ('login',   '登录类', 10),
  ('payment', '支付类', 20),
  ('push',    '推送类', 30),
  ('ad',      '广告类', 40)
ON CONFLICT (category_code) DO NOTHING;

INSERT INTO platform.admin_permissions (permission_code, permission_name) VALUES
  ('feature_plugin.read',  '功能插件管理-读'),
  ('feature_plugin.write', '功能插件管理-写')
ON CONFLICT (permission_code) DO NOTHING;

-- super_admin 补授新权限（与 000003 §5 / 000018 同口径）。
INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id
FROM platform.admin_roles r
JOIN platform.admin_permissions p
  ON p.permission_code IN ('feature_plugin.read', 'feature_plugin.write')
WHERE r.role_code = 'super_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;
