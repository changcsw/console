-- 000020 down · 回收功能插件管理权限码与插件分类字典。
-- 顺序：先解绑角色权限，再删权限码；先摘掉 feature_plugins 的外键列，再删分类表。
-- 不动 000012 建的 feature_plugins / feature_plugin_templates 本体。

DELETE FROM platform.admin_role_permissions
WHERE permission_id_ref IN (
  SELECT id FROM platform.admin_permissions
  WHERE permission_code IN ('feature_plugin.read', 'feature_plugin.write')
);

DELETE FROM platform.admin_permissions
WHERE permission_code IN ('feature_plugin.read', 'feature_plugin.write');

DROP INDEX IF EXISTS platform.idx_feature_plugins_category;

ALTER TABLE platform.feature_plugins DROP COLUMN IF EXISTS category_id_ref;

DROP INDEX IF EXISTS platform.idx_feature_plugin_categories_enabled_sort;

DROP TABLE IF EXISTS platform.feature_plugin_categories;
