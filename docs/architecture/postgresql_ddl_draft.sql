-- PostgreSQL 15+ draft schema for the game publishing management console.
-- This draft favors explicit business tables over aggressive normalization.

CREATE TABLE IF NOT EXISTS games (
  id BIGSERIAL PRIMARY KEY,
  game_id VARCHAR(64) NOT NULL,
  game_secret VARCHAR(128) NOT NULL,
  name VARCHAR(128) NOT NULL,
  alias VARCHAR(64) NOT NULL,
  icon_url VARCHAR(512) NOT NULL DEFAULT '',
  default_market_code VARCHAR(32) NOT NULL DEFAULT 'GLOBAL',
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id)
);

CREATE TABLE IF NOT EXISTS game_markets (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  market_code VARCHAR(32) NOT NULL,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  default_locale VARCHAR(16) NOT NULL DEFAULT 'en-US',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, market_code)
);

CREATE TABLE IF NOT EXISTS game_legal_links (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  scope_type VARCHAR(16) NOT NULL CHECK (scope_type IN ('default', 'market', 'locale')),
  scope_value VARCHAR(32) NOT NULL DEFAULT '*',
  terms_url VARCHAR(512) NOT NULL DEFAULT '',
  privacy_url VARCHAR(512) NOT NULL DEFAULT '',
  delete_account_url VARCHAR(512) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, scope_type, scope_value)
);

CREATE TABLE IF NOT EXISTS currency_specs (
  id BIGSERIAL PRIMARY KEY,
  currency_code VARCHAR(8) NOT NULL,
  currency_name VARCHAR(64) NOT NULL,
  decimal_places INT NOT NULL CHECK (decimal_places >= 0 AND decimal_places <= 6),
  min_amount_minor BIGINT NOT NULL DEFAULT 1,
  rounding_mode VARCHAR(16) NOT NULL CHECK (rounding_mode IN ('half_up', 'floor', 'ceil', 'truncate')),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (currency_code)
);

CREATE TABLE IF NOT EXISTS channels (
  id BIGSERIAL PRIMARY KEY,
  channel_id VARCHAR(64) NOT NULL,
  channel_name VARCHAR(64) NOT NULL,
  channel_type VARCHAR(32) NOT NULL CHECK (channel_type IN ('store', 'oem', 'web', 'direct', 'mini_game')),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id)
);

CREATE TABLE IF NOT EXISTS channel_policies (
  id BIGSERIAL PRIMARY KEY,
  channel_id_ref BIGINT NOT NULL REFERENCES channels(id),
  login_mode VARCHAR(16) NOT NULL CHECK (login_mode IN ('channel_only', 'account_system')),
  payment_mode VARCHAR(16) NOT NULL CHECK (payment_mode IN ('channel_only', 'hybrid', 'cashier_only')),
  login_locked BOOLEAN NOT NULL DEFAULT FALSE,
  payment_locked BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id_ref)
);

CREATE TABLE IF NOT EXISTS game_channels (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  channel_id_ref BIGINT NOT NULL REFERENCES channels(id),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  remark VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, channel_id_ref)
);

CREATE TABLE IF NOT EXISTS channel_packages (
  id BIGSERIAL PRIMARY KEY,
  game_channel_id_ref BIGINT NOT NULL REFERENCES game_channels(id),
  package_code VARCHAR(64) NOT NULL,
  package_name VARCHAR(128) NOT NULL,
  market_code VARCHAR(32) NOT NULL,
  bundle_id VARCHAR(128) NOT NULL DEFAULT '',
  inherit_channel_config BOOLEAN NOT NULL DEFAULT TRUE,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  override_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_channel_id_ref, package_code)
);

CREATE TABLE IF NOT EXISTS account_auth_types (
  id BIGSERIAL PRIMARY KEY,
  auth_type_id VARCHAR(64) NOT NULL,
  auth_type_name VARCHAR(64) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (auth_type_id)
);

CREATE TABLE IF NOT EXISTS channel_account_auth_types (
  id BIGSERIAL PRIMARY KEY,
  channel_id_ref BIGINT NOT NULL REFERENCES channels(id),
  auth_type_id_ref BIGINT NOT NULL REFERENCES account_auth_types(id),
  default_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  locked BOOLEAN NOT NULL DEFAULT FALSE,
  sort INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id_ref, auth_type_id_ref)
);

CREATE TABLE IF NOT EXISTS account_auth_templates (
  id BIGSERIAL PRIMARY KEY,
  auth_type_id_ref BIGINT NOT NULL REFERENCES account_auth_types(id),
  template_version VARCHAR(32) NOT NULL,
  form_schema_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  secret_fields_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  file_fields_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  validation_rules_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (auth_type_id_ref, template_version)
);

CREATE TABLE IF NOT EXISTS game_account_auth_configs (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  auth_type_id_ref BIGINT NOT NULL REFERENCES account_auth_types(id),
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  config_status VARCHAR(16) NOT NULL DEFAULT 'empty' CHECK (config_status IN ('empty', 'invalid', 'valid')),
  last_check_at TIMESTAMPTZ,
  last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, auth_type_id_ref)
);

CREATE TABLE IF NOT EXISTS channel_login_templates (
  id BIGSERIAL PRIMARY KEY,
  channel_id_ref BIGINT NOT NULL REFERENCES channels(id),
  template_version VARCHAR(32) NOT NULL,
  form_schema_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  secret_fields_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  file_fields_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  validation_rules_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id_ref, template_version)
);

CREATE TABLE IF NOT EXISTS game_channel_login_configs (
  id BIGSERIAL PRIMARY KEY,
  game_channel_id_ref BIGINT NOT NULL REFERENCES game_channels(id),
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  config_status VARCHAR(16) NOT NULL DEFAULT 'empty' CHECK (config_status IN ('empty', 'invalid', 'valid')),
  last_check_at TIMESTAMPTZ,
  last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_channel_id_ref)
);

CREATE TABLE IF NOT EXISTS products (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  product_id VARCHAR(128) NOT NULL,
  product_name VARCHAR(128) NOT NULL,
  base_amount_minor BIGINT NOT NULL,
  base_currency VARCHAR(8) NOT NULL,
  price_id VARCHAR(64) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, product_id)
);

CREATE TABLE IF NOT EXISTS channel_products (
  id BIGSERIAL PRIMARY KEY,
  product_id_ref BIGINT NOT NULL REFERENCES products(id),
  package_id_ref BIGINT NOT NULL REFERENCES channel_packages(id),
  product_id_mode VARCHAR(16) NOT NULL CHECK (product_id_mode IN ('default', 'override')),
  product_id_override VARCHAR(128) NOT NULL DEFAULT '',
  price_id_mode VARCHAR(16) NOT NULL CHECK (price_id_mode IN ('default', 'override')),
  price_id_override VARCHAR(64) NOT NULL DEFAULT '',
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (package_id_ref, product_id_ref)
);

CREATE TABLE IF NOT EXISTS channel_iap_templates (
  id BIGSERIAL PRIMARY KEY,
  channel_id_ref BIGINT NOT NULL REFERENCES channels(id),
  template_version VARCHAR(32) NOT NULL,
  form_schema_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  secret_fields_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  file_fields_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  validation_rules_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id_ref, template_version)
);

CREATE TABLE IF NOT EXISTS game_channel_iap_configs (
  id BIGSERIAL PRIMARY KEY,
  game_channel_id_ref BIGINT NOT NULL REFERENCES game_channels(id),
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  config_status VARCHAR(16) NOT NULL DEFAULT 'empty' CHECK (config_status IN ('empty', 'invalid', 'valid')),
  last_check_at TIMESTAMPTZ,
  last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_channel_id_ref)
);

CREATE TABLE IF NOT EXISTS channel_package_iap_overrides (
  id BIGSERIAL PRIMARY KEY,
  package_id_ref BIGINT NOT NULL REFERENCES channel_packages(id),
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  config_status VARCHAR(16) NOT NULL DEFAULT 'empty' CHECK (config_status IN ('empty', 'invalid', 'valid')),
  last_check_at TIMESTAMPTZ,
  last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (package_id_ref)
);

CREATE TABLE IF NOT EXISTS cashier_price_templates (
  id BIGSERIAL PRIMARY KEY,
  template_id VARCHAR(64) NOT NULL,
  template_name VARCHAR(128) NOT NULL,
  fx_sync_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  fx_sync_mode VARCHAR(16) NOT NULL DEFAULT 'manual_confirm' CHECK (fx_sync_mode IN ('manual_confirm', 'auto_apply')),
  fx_sync_schedule VARCHAR(16) NOT NULL DEFAULT 'monthly' CHECK (fx_sync_schedule IN ('monthly', 'quarterly')),
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (template_id)
);

CREATE TABLE IF NOT EXISTS cashier_price_template_versions (
  id BIGSERIAL PRIMARY KEY,
  template_id_ref BIGINT NOT NULL REFERENCES cashier_price_templates(id),
  version VARCHAR(32) NOT NULL,
  source_type VARCHAR(32) NOT NULL DEFAULT 'manual',
  auto_generated BOOLEAN NOT NULL DEFAULT FALSE,
  fx_base_date DATE,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  checksum VARCHAR(128) NOT NULL DEFAULT '',
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (template_id_ref, version)
);

CREATE TABLE IF NOT EXISTS cashier_price_rows (
  id BIGSERIAL PRIMARY KEY,
  template_version_id_ref BIGINT NOT NULL REFERENCES cashier_price_template_versions(id),
  country_code VARCHAR(8) NOT NULL,
  region_code VARCHAR(16) NOT NULL DEFAULT '*',
  currency VARCHAR(8) NOT NULL,
  price_id VARCHAR(64) NOT NULL,
  pre_tax_amount_minor BIGINT NOT NULL,
  tax_rate DECIMAL(8,6) NOT NULL,
  tax_amount_minor BIGINT NOT NULL,
  after_tax_amount_minor BIGINT NOT NULL,
  effective_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (template_version_id_ref, country_code, region_code, currency, price_id)
);

CREATE TABLE IF NOT EXISTS cashier_fx_sync_runs (
  id BIGSERIAL PRIMARY KEY,
  template_id_ref BIGINT NOT NULL REFERENCES cashier_price_templates(id),
  candidate_version_id_ref BIGINT NOT NULL REFERENCES cashier_price_template_versions(id),
  status VARCHAR(16) NOT NULL CHECK (status IN ('pending_review', 'approved', 'applied', 'ignored', 'failed')),
  diff_summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  reviewed_by BIGINT,
  reviewed_at TIMESTAMPTZ,
  review_note VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS game_cashier_profiles (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  template_id_ref BIGINT NOT NULL REFERENCES cashier_price_templates(id),
  applied_template_version_id BIGINT NOT NULL REFERENCES cashier_price_template_versions(id),
  snapshot_checksum VARCHAR(128) NOT NULL DEFAULT '',
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref)
);

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
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, country_code, region_code, currency, price_id)
);

CREATE TABLE IF NOT EXISTS pay_ways (
  id BIGSERIAL PRIMARY KEY,
  pay_way_id VARCHAR(64) NOT NULL,
  pay_way_name VARCHAR(64) NOT NULL,
  pay_way_type VARCHAR(32) NOT NULL CHECK (pay_way_type IN ('card', 'wallet', 'platform', 'local')),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (pay_way_id)
);

CREATE TABLE IF NOT EXISTS cashier_providers (
  id BIGSERIAL PRIMARY KEY,
  provider_id VARCHAR(64) NOT NULL,
  provider_name VARCHAR(64) NOT NULL,
  provider_kind VARCHAR(32) NOT NULL CHECK (provider_kind IN ('aggregator', 'gateway', 'wallet_direct')),
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (provider_id)
);

CREATE TABLE IF NOT EXISTS cashier_provider_templates (
  id BIGSERIAL PRIMARY KEY,
  provider_id_ref BIGINT NOT NULL REFERENCES cashier_providers(id),
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

CREATE TABLE IF NOT EXISTS billing_subjects (
  id BIGSERIAL PRIMARY KEY,
  subject_id VARCHAR(64) NOT NULL,
  subject_name VARCHAR(128) NOT NULL,
  legal_entity_name VARCHAR(255) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (subject_id)
);

CREATE TABLE IF NOT EXISTS cashier_merchant_accounts (
  id BIGSERIAL PRIMARY KEY,
  merchant_account_id VARCHAR(64) NOT NULL,
  provider_id_ref BIGINT NOT NULL REFERENCES cashier_providers(id),
  subject_id_ref BIGINT NOT NULL REFERENCES billing_subjects(id),
  merchant_id VARCHAR(128) NOT NULL,
  merchant_name VARCHAR(128) NOT NULL,
  config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  secret_ciphertext TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (merchant_account_id)
);

CREATE TABLE IF NOT EXISTS payment_routes (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  market_code VARCHAR(32) NOT NULL DEFAULT '*',
  country_code VARCHAR(8) NOT NULL DEFAULT '*',
  currency VARCHAR(8) NOT NULL DEFAULT '*',
  channel_id_ref BIGINT REFERENCES channels(id),
  package_id_ref BIGINT REFERENCES channel_packages(id),
  pay_way_id_ref BIGINT NOT NULL REFERENCES pay_ways(id),
  provider_id_ref BIGINT NOT NULL REFERENCES cashier_providers(id),
  merchant_account_id_ref BIGINT NOT NULL REFERENCES cashier_merchant_accounts(id),
  priority INT NOT NULL DEFAULT 100,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS game_config_snapshots (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  config_schema_version VARCHAR(32) NOT NULL,
  config_version VARCHAR(32) NOT NULL,
  config_json JSONB NOT NULL,
  file_name VARCHAR(255) NOT NULL,
  file_hash VARCHAR(128) NOT NULL,
  storage_key VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, config_version)
);

CREATE TABLE IF NOT EXISTS sync_jobs (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref BIGINT NOT NULL REFERENCES games(id),
  source_env VARCHAR(16) NOT NULL CHECK (source_env IN ('develop', 'sandbox', 'production')),
  target_env VARCHAR(16) NOT NULL CHECK (target_env IN ('develop', 'sandbox', 'production')),
  source_hash VARCHAR(128) NOT NULL,
  target_hash_before VARCHAR(128) NOT NULL,
  target_hash_after VARCHAR(128) NOT NULL DEFAULT '',
  include_deletes BOOLEAN NOT NULL DEFAULT FALSE,
  operator_id BIGINT NOT NULL,
  operator_note VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL CHECK (status IN ('previewed', 'succeeded', 'failed')),
  executed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sync_job_items (
  id BIGSERIAL PRIMARY KEY,
  sync_job_id_ref BIGINT NOT NULL REFERENCES sync_jobs(id),
  section VARCHAR(32) NOT NULL,
  entity_type VARCHAR(64) NOT NULL,
  entity_key VARCHAR(128) NOT NULL,
  op VARCHAR(16) NOT NULL CHECK (op IN ('add', 'update', 'delete')),
  field_name VARCHAR(64) NOT NULL,
  sandbox_value_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  production_value_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  masked BOOLEAN NOT NULL DEFAULT FALSE,
  applied BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  actor_id BIGINT NOT NULL,
  action VARCHAR(64) NOT NULL,
  resource_type VARCHAR(64) NOT NULL,
  resource_id VARCHAR(128) NOT NULL,
  env VARCHAR(16) NOT NULL CHECK (env IN ('develop', 'sandbox', 'production')),
  detail_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS admin_users (
  id BIGSERIAL PRIMARY KEY,
  user_name VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL,
  email VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_name)
);

CREATE TABLE IF NOT EXISTS admin_identities (
  id BIGSERIAL PRIMARY KEY,
  user_id_ref BIGINT NOT NULL REFERENCES admin_users(id),
  identity_type VARCHAR(16) NOT NULL CHECK (identity_type IN ('password', 'feishu')),
  identity_key VARCHAR(128) NOT NULL,
  credential_ciphertext TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (identity_type, identity_key)
);

CREATE TABLE IF NOT EXISTS admin_roles (
  id BIGSERIAL PRIMARY KEY,
  role_code VARCHAR(64) NOT NULL,
  role_name VARCHAR(128) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (role_code)
);

CREATE TABLE IF NOT EXISTS admin_permissions (
  id BIGSERIAL PRIMARY KEY,
  permission_code VARCHAR(128) NOT NULL,
  permission_name VARCHAR(128) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (permission_code)
);

CREATE TABLE IF NOT EXISTS admin_user_roles (
  id BIGSERIAL PRIMARY KEY,
  user_id_ref BIGINT NOT NULL REFERENCES admin_users(id),
  role_id_ref BIGINT NOT NULL REFERENCES admin_roles(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id_ref, role_id_ref)
);

CREATE TABLE IF NOT EXISTS admin_role_permissions (
  id BIGSERIAL PRIMARY KEY,
  role_id_ref BIGINT NOT NULL REFERENCES admin_roles(id),
  permission_id_ref BIGINT NOT NULL REFERENCES admin_permissions(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (role_id_ref, permission_id_ref)
);

CREATE INDEX IF NOT EXISTS idx_game_markets_game_id_ref ON game_markets(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_legal_links_game_id_ref ON game_legal_links(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_channels_game_id_ref ON game_channels(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_channels_channel_id_ref ON game_channels(channel_id_ref);
CREATE INDEX IF NOT EXISTS idx_channel_packages_game_channel_id_ref ON channel_packages(game_channel_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_account_auth_configs_game_id_ref ON game_account_auth_configs(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_channel_login_configs_game_channel_id_ref ON game_channel_login_configs(game_channel_id_ref);
CREATE INDEX IF NOT EXISTS idx_products_game_id_ref ON products(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_channel_products_package_id_ref ON channel_products(package_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_channel_iap_configs_game_channel_id_ref ON game_channel_iap_configs(game_channel_id_ref);
CREATE INDEX IF NOT EXISTS idx_cashier_price_template_versions_template_id_ref ON cashier_price_template_versions(template_id_ref);
CREATE INDEX IF NOT EXISTS idx_cashier_price_rows_template_version_id_ref ON cashier_price_rows(template_version_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_cashier_profiles_game_id_ref ON game_cashier_profiles(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_cashier_price_overrides_game_id_ref ON game_cashier_price_overrides(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_payment_routes_game_id_ref ON payment_routes(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_payment_routes_provider_id_ref ON payment_routes(provider_id_ref);
CREATE INDEX IF NOT EXISTS idx_payment_routes_pay_way_id_ref ON payment_routes(pay_way_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_config_snapshots_game_id_ref ON game_config_snapshots(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_sync_jobs_game_id_ref ON sync_jobs(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_sync_job_items_sync_job_id_ref ON sync_job_items(sync_job_id_ref);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_env_created_at ON audit_logs(env, created_at DESC);
