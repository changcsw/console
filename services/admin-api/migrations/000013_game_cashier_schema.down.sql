-- 000013 down · game-cashier 回滚（best effort）

DROP INDEX IF EXISTS idx_game_cashier_price_overrides_game_id_ref;
ALTER TABLE IF EXISTS game_cashier_price_overrides DROP CONSTRAINT IF EXISTS gcpo_currency_fk;
ALTER TABLE IF EXISTS game_cashier_price_overrides DROP CONSTRAINT IF EXISTS gcpo_key;
ALTER TABLE IF EXISTS game_cashier_price_overrides
  ADD CONSTRAINT game_cashier_price_overrides_game_id_ref_country_code_region_co_key
  UNIQUE (game_id_ref, country_code, region_code, currency, price_id);

DROP INDEX IF EXISTS idx_game_cashier_profiles_game_id_ref;
ALTER TABLE IF EXISTS game_cashier_profiles DROP CONSTRAINT IF EXISTS gcp_template_fk;
ALTER TABLE IF EXISTS game_cashier_profiles DROP CONSTRAINT IF EXISTS gcp_template_version_fk;
ALTER TABLE IF EXISTS game_cashier_profiles DROP CONSTRAINT IF EXISTS gcp_game_key;
ALTER TABLE IF EXISTS game_cashier_profiles
  ADD CONSTRAINT game_cashier_profiles_game_id_ref_key UNIQUE (game_id_ref);
