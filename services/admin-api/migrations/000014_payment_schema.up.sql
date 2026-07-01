-- 000014 · payment 模块：平台级 5 表归位 + payment_routes 约束/唯一索引（v2）
-- 说明：
--   - 平台级：pay_ways / cashier_providers / cashier_provider_templates / billing_subjects / cashier_merchant_accounts
--   - 业务表：payment_routes（当前 env schema，不写 schema 前缀）
--   - 幂等：IF NOT EXISTS + DROP CONSTRAINT IF EXISTS + DO 守卫

CREATE SCHEMA IF NOT EXISTS platform;

ALTER TABLE IF EXISTS public.pay_ways SET SCHEMA platform;
ALTER TABLE IF EXISTS public.cashier_providers SET SCHEMA platform;
ALTER TABLE IF EXISTS public.cashier_provider_templates SET SCHEMA platform;
ALTER TABLE IF EXISTS public.billing_subjects SET SCHEMA platform;
ALTER TABLE IF EXISTS public.cashier_merchant_accounts SET SCHEMA platform;

CREATE TABLE IF NOT EXISTS platform.pay_ways (
  id BIGSERIAL PRIMARY KEY,
  pay_way_id VARCHAR(64) NOT NULL UNIQUE,
  pay_way_name VARCHAR(64) NOT NULL,
  pay_way_type VARCHAR(32) NOT NULL CHECK (pay_way_type IN ('card', 'wallet', 'platform', 'local')),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.cashier_providers (
  id BIGSERIAL PRIMARY KEY,
  provider_id VARCHAR(64) NOT NULL UNIQUE,
  provider_name VARCHAR(64) NOT NULL,
  provider_kind VARCHAR(32) NOT NULL CHECK (provider_kind IN ('aggregator', 'gateway', 'wallet_direct')),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.cashier_provider_templates (
  id BIGSERIAL PRIMARY KEY,
  provider_id_ref BIGINT NOT NULL REFERENCES platform.cashier_providers(id),
  template_version VARCHAR(32) NOT NULL,
  form_schema_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  secret_fields_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  file_fields_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  validation_rules_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (provider_id_ref, template_version)
);

CREATE TABLE IF NOT EXISTS platform.billing_subjects (
  id BIGSERIAL PRIMARY KEY,
  subject_id VARCHAR(64) NOT NULL UNIQUE,
  subject_name VARCHAR(128) NOT NULL,
  legal_entity_name VARCHAR(255) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.cashier_merchant_accounts (
  id BIGSERIAL PRIMARY KEY,
  merchant_account_id VARCHAR(64) NOT NULL UNIQUE,
  provider_id_ref BIGINT NOT NULL REFERENCES platform.cashier_providers(id),
  subject_id_ref BIGINT NOT NULL REFERENCES platform.billing_subjects(id),
  merchant_id VARCHAR(128) NOT NULL,
  merchant_name VARCHAR(128) NOT NULL,
  config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  secret_ciphertext TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payment_routes (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  market_code VARCHAR(32) NOT NULL DEFAULT '*',
  country_code VARCHAR(8) NOT NULL DEFAULT '*',
  currency VARCHAR(8) NOT NULL DEFAULT '*',
  channel_id_ref BIGINT NULL REFERENCES platform.channels(id),
  package_id_ref BIGINT NULL REFERENCES channel_packages(id),
  pay_way_id_ref BIGINT NOT NULL REFERENCES platform.pay_ways(id),
  provider_id_ref BIGINT NOT NULL REFERENCES platform.cashier_providers(id),
  merchant_account_id_ref BIGINT NOT NULL REFERENCES platform.cashier_merchant_accounts(id),
  priority INT NOT NULL DEFAULT 100,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE payment_routes DROP CONSTRAINT IF EXISTS payment_routes_channel_id_ref_fkey;
ALTER TABLE payment_routes DROP CONSTRAINT IF EXISTS payment_routes_pay_way_id_ref_fkey;
ALTER TABLE payment_routes DROP CONSTRAINT IF EXISTS payment_routes_provider_id_ref_fkey;
ALTER TABLE payment_routes DROP CONSTRAINT IF EXISTS payment_routes_merchant_account_id_ref_fkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='payment_routes_channel_fk' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE payment_routes
      ADD CONSTRAINT payment_routes_channel_fk
      FOREIGN KEY (channel_id_ref) REFERENCES platform.channels(id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='payment_routes_pay_way_fk' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE payment_routes
      ADD CONSTRAINT payment_routes_pay_way_fk
      FOREIGN KEY (pay_way_id_ref) REFERENCES platform.pay_ways(id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='payment_routes_provider_fk' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE payment_routes
      ADD CONSTRAINT payment_routes_provider_fk
      FOREIGN KEY (provider_id_ref) REFERENCES platform.cashier_providers(id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='payment_routes_merchant_fk' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE payment_routes
      ADD CONSTRAINT payment_routes_merchant_fk
      FOREIGN KEY (merchant_account_id_ref) REFERENCES platform.cashier_merchant_accounts(id);
  END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_routes_priority
  ON payment_routes (game_id_ref, pay_way_id_ref, priority) WHERE enabled;

CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_routes_selector
  ON payment_routes (game_id_ref, pay_way_id_ref,
    COALESCE(package_id_ref, -1), COALESCE(channel_id_ref, -1),
    market_code, country_code, currency) WHERE enabled;
