-- 000007 · product 模块：商品与 IAP 映射 schema 归位 + 约束补齐（compact 全量落地）
-- 业务表（products/channel_products/game_channel_iap_configs/channel_package_iap_overrides）在每环境 schema 执行；
-- 平台表 channel_iap_templates 归位到 platform（可重复执行）。

CREATE SCHEMA IF NOT EXISTS platform;

-- 1) 平台级模板表归位（public -> platform）
ALTER TABLE IF EXISTS public.channel_iap_templates SET SCHEMA platform;

CREATE TABLE IF NOT EXISTS platform.channel_iap_templates (
  id                    BIGSERIAL PRIMARY KEY,
  channel_id_ref        BIGINT      NOT NULL REFERENCES platform.channels(id),
  template_version      VARCHAR(32) NOT NULL,
  form_schema_json      JSONB       NOT NULL DEFAULT '[]'::jsonb,
  secret_fields_json    JSONB       NOT NULL DEFAULT '[]'::jsonb,
  file_fields_json      JSONB       NOT NULL DEFAULT '[]'::jsonb,
  validation_rules_json JSONB       NOT NULL DEFAULT '{}'::jsonb,
  enabled               BOOLEAN     NOT NULL DEFAULT TRUE,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id_ref, template_version)
);

CREATE INDEX IF NOT EXISTS idx_channel_iap_templates_channel_enabled_version
  ON platform.channel_iap_templates(channel_id_ref, enabled, template_version DESC);

-- 2) products（业务表）
CREATE TABLE IF NOT EXISTS products (
  id                BIGSERIAL PRIMARY KEY,
  game_id_ref       BIGINT      NOT NULL REFERENCES games(id),
  product_id        VARCHAR(128) NOT NULL,
  product_name      VARCHAR(128) NOT NULL,
  base_amount_minor BIGINT      NOT NULL,
  base_currency     VARCHAR(8)  NOT NULL,
  price_id          VARCHAR(64) NOT NULL,
  enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, product_id)
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='products_base_currency_fk' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE products
      ADD CONSTRAINT products_base_currency_fk
      FOREIGN KEY (base_currency) REFERENCES platform.currency_specs(currency_code);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_products_game_id_ref ON products(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_products_enabled ON products(enabled);

-- 3) channel_products（业务表）
CREATE TABLE IF NOT EXISTS channel_products (
  id                  BIGSERIAL PRIMARY KEY,
  product_id_ref      BIGINT       NOT NULL REFERENCES products(id),
  package_id_ref      BIGINT       NOT NULL REFERENCES channel_packages(id),
  product_id_mode     VARCHAR(16)  NOT NULL DEFAULT 'default',
  product_id_override VARCHAR(128) NOT NULL DEFAULT '',
  price_id_mode       VARCHAR(16)  NOT NULL DEFAULT 'default',
  price_id_override   VARCHAR(64)  NOT NULL DEFAULT '',
  enabled             BOOLEAN      NOT NULL DEFAULT TRUE,
  created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (package_id_ref, product_id_ref)
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='channel_products_product_id_mode_check' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE channel_products
      ADD CONSTRAINT channel_products_product_id_mode_check
      CHECK (product_id_mode IN ('default', 'override'));
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='channel_products_price_id_mode_check' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE channel_products
      ADD CONSTRAINT channel_products_price_id_mode_check
      CHECK (price_id_mode IN ('default', 'override'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_channel_products_package_id_ref ON channel_products(package_id_ref);
CREATE INDEX IF NOT EXISTS idx_channel_products_product_id_ref ON channel_products(product_id_ref);

-- 4) game_channel_iap_configs（业务表）
CREATE TABLE IF NOT EXISTS game_channel_iap_configs (
  id                 BIGSERIAL PRIMARY KEY,
  game_channel_id_ref BIGINT      NOT NULL REFERENCES game_channels(id),
  enabled            BOOLEAN      NOT NULL DEFAULT FALSE,
  config_json        JSONB        NOT NULL DEFAULT '{}'::jsonb,
  config_status      VARCHAR(16)  NOT NULL DEFAULT 'empty',
  last_check_at      TIMESTAMPTZ,
  last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (game_channel_id_ref)
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='game_channel_iap_configs_status_check' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_channel_iap_configs
      ADD CONSTRAINT game_channel_iap_configs_status_check
      CHECK (config_status IN ('empty', 'invalid', 'valid'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_game_channel_iap_configs_gc ON game_channel_iap_configs(game_channel_id_ref);

-- 5) channel_package_iap_overrides（业务表）
CREATE TABLE IF NOT EXISTS channel_package_iap_overrides (
  id                 BIGSERIAL PRIMARY KEY,
  package_id_ref     BIGINT       NOT NULL REFERENCES channel_packages(id),
  enabled            BOOLEAN      NOT NULL DEFAULT FALSE,
  config_json        JSONB        NOT NULL DEFAULT '{}'::jsonb,
  config_status      VARCHAR(16)  NOT NULL DEFAULT 'empty',
  last_check_at      TIMESTAMPTZ,
  last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (package_id_ref)
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='channel_package_iap_overrides_status_check' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE channel_package_iap_overrides
      ADD CONSTRAINT channel_package_iap_overrides_status_check
      CHECK (config_status IN ('empty', 'invalid', 'valid'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_channel_package_iap_overrides_package ON channel_package_iap_overrides(package_id_ref);
