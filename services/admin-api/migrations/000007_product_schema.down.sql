-- 000007 down · product 回滚（best effort）

DROP INDEX IF EXISTS idx_channel_package_iap_overrides_package;
ALTER TABLE IF EXISTS channel_package_iap_overrides DROP CONSTRAINT IF EXISTS channel_package_iap_overrides_status_check;
DROP TABLE IF EXISTS channel_package_iap_overrides;

DROP INDEX IF EXISTS idx_game_channel_iap_configs_gc;
ALTER TABLE IF EXISTS game_channel_iap_configs DROP CONSTRAINT IF EXISTS game_channel_iap_configs_status_check;
DROP TABLE IF EXISTS game_channel_iap_configs;

DROP INDEX IF EXISTS idx_channel_products_product_id_ref;
DROP INDEX IF EXISTS idx_channel_products_package_id_ref;
ALTER TABLE IF EXISTS channel_products DROP CONSTRAINT IF EXISTS channel_products_price_id_mode_check;
ALTER TABLE IF EXISTS channel_products DROP CONSTRAINT IF EXISTS channel_products_product_id_mode_check;
DROP TABLE IF EXISTS channel_products;

DROP INDEX IF EXISTS idx_products_enabled;
DROP INDEX IF EXISTS idx_products_game_id_ref;
ALTER TABLE IF EXISTS products DROP CONSTRAINT IF EXISTS products_base_currency_fk;
DROP TABLE IF EXISTS products;

DROP INDEX IF EXISTS platform.idx_channel_iap_templates_channel_enabled_version;
ALTER TABLE IF EXISTS platform.channel_iap_templates SET SCHEMA public;
