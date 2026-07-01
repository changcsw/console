-- 000012 · game-cashier：游戏级收银台绑定与价格覆盖（业务表，按环境 schema）
-- 业务表 SQL 不写 schema 前缀，依赖 search_path=<env>,platform；
-- 平台表引用使用显式 platform. 前缀（00 §2 / §5）。

CREATE TABLE IF NOT EXISTS game_cashier_profiles (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  template_id_ref BIGINT NOT NULL REFERENCES platform.cashier_price_templates(id),
  applied_template_version_id BIGINT NOT NULL REFERENCES platform.cashier_price_template_versions(id),
  snapshot_checksum VARCHAR(128) NOT NULL DEFAULT '',
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE game_cashier_profiles DROP CONSTRAINT IF EXISTS game_cashier_profiles_template_id_ref_fkey;
ALTER TABLE game_cashier_profiles DROP CONSTRAINT IF EXISTS game_cashier_profiles_applied_template_version_id_fkey;
ALTER TABLE game_cashier_profiles DROP CONSTRAINT IF EXISTS game_cashier_profiles_game_id_ref_key;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='gcp_game_key' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_cashier_profiles
      ADD CONSTRAINT gcp_game_key UNIQUE (game_id_ref);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='gcp_template_fk' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_cashier_profiles
      ADD CONSTRAINT gcp_template_fk
      FOREIGN KEY (template_id_ref) REFERENCES platform.cashier_price_templates(id);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='gcp_template_version_fk' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_cashier_profiles
      ADD CONSTRAINT gcp_template_version_fk
      FOREIGN KEY (applied_template_version_id) REFERENCES platform.cashier_price_template_versions(id);
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS game_cashier_price_overrides (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  country_code VARCHAR(8) NOT NULL,
  region_code VARCHAR(16) NOT NULL DEFAULT '*',
  currency VARCHAR(8) NOT NULL,
  price_id VARCHAR(64) NOT NULL,
  pre_tax_amount_minor BIGINT NOT NULL,
  tax_rate DECIMAL(8,6) NOT NULL,
  tax_amount_minor BIGINT NOT NULL,
  after_tax_amount_minor BIGINT NOT NULL,
  reason VARCHAR(255) NOT NULL DEFAULT '',
  effective_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE game_cashier_price_overrides DROP CONSTRAINT IF EXISTS game_cashier_price_overrides_currency_fkey;
ALTER TABLE game_cashier_price_overrides DROP CONSTRAINT IF EXISTS game_cashier_price_overrides_game_id_ref_country_code_region_co_key;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='gcpo_key' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_cashier_price_overrides
      ADD CONSTRAINT gcpo_key UNIQUE (game_id_ref, country_code, region_code, currency, price_id);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='gcpo_currency_fk' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_cashier_price_overrides
      ADD CONSTRAINT gcpo_currency_fk
      FOREIGN KEY (currency) REFERENCES platform.currency_specs(currency_code);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_game_cashier_profiles_game_id_ref
  ON game_cashier_profiles(game_id_ref);

CREATE INDEX IF NOT EXISTS idx_game_cashier_price_overrides_game_id_ref
  ON game_cashier_price_overrides(game_id_ref);
