-- 000012 down · feature-plugin 回滚（best effort，幂等）

DROP INDEX IF EXISTS idx_channel_package_plugin_overrides_package;
ALTER TABLE IF EXISTS channel_package_plugin_overrides
  DROP CONSTRAINT IF EXISTS channel_package_plugin_overrides_status_check;
DROP TABLE IF EXISTS channel_package_plugin_overrides;

DROP INDEX IF EXISTS idx_game_channel_plugin_configs_gc;
ALTER TABLE IF EXISTS game_channel_plugin_configs
  DROP CONSTRAINT IF EXISTS game_channel_plugin_configs_status_check;
DROP TABLE IF EXISTS game_channel_plugin_configs;

DROP INDEX IF EXISTS platform.idx_channel_feature_plugins_channel_sort;
DROP TABLE IF EXISTS platform.channel_feature_plugins;

DROP INDEX IF EXISTS platform.idx_feature_plugin_templates_plugin_enabled_version;
DROP TABLE IF EXISTS platform.feature_plugin_templates;

DROP INDEX IF EXISTS platform.idx_feature_plugins_enabled_sort;
DROP INDEX IF EXISTS platform.idx_feature_plugins_region;
DROP TABLE IF EXISTS platform.feature_plugins;
