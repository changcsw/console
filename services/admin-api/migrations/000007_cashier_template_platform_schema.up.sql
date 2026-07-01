-- 000007 · cashier-template 模块：platform schema 归位 + 约束/索引补齐
-- 目标：cashier_price_templates / cashier_price_template_versions / cashier_price_rows / cashier_fx_sync_runs
-- 全部平台级，固定在 platform schema，不带 env（00 §2.2 / 模块 17 compact）

CREATE SCHEMA IF NOT EXISTS platform;

-- 1) 若历史仍在 public，迁移到 platform（幂等）
ALTER TABLE IF EXISTS public.cashier_price_templates         SET SCHEMA platform;
ALTER TABLE IF EXISTS public.cashier_price_template_versions SET SCHEMA platform;
ALTER TABLE IF EXISTS public.cashier_price_rows              SET SCHEMA platform;
ALTER TABLE IF EXISTS public.cashier_fx_sync_runs            SET SCHEMA platform;
ALTER TABLE IF EXISTS public.currency_specs                  SET SCHEMA platform;

-- 2) 目标表兜底
CREATE TABLE IF NOT EXISTS platform.cashier_price_templates (
  id BIGSERIAL PRIMARY KEY,
  template_id VARCHAR(64) NOT NULL,
  template_name VARCHAR(128) NOT NULL,
  fx_sync_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  fx_sync_mode VARCHAR(16) NOT NULL DEFAULT 'manual_confirm',
  fx_sync_schedule VARCHAR(16) NOT NULL DEFAULT 'monthly',
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (template_id)
);

CREATE TABLE IF NOT EXISTS platform.cashier_price_template_versions (
  id BIGSERIAL PRIMARY KEY,
  template_id_ref BIGINT NOT NULL REFERENCES platform.cashier_price_templates(id),
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

CREATE TABLE IF NOT EXISTS platform.cashier_price_rows (
  id BIGSERIAL PRIMARY KEY,
  template_version_id_ref BIGINT NOT NULL REFERENCES platform.cashier_price_template_versions(id),
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

CREATE TABLE IF NOT EXISTS platform.cashier_fx_sync_runs (
  id BIGSERIAL PRIMARY KEY,
  template_id_ref BIGINT NOT NULL REFERENCES platform.cashier_price_templates(id),
  candidate_version_id_ref BIGINT NOT NULL REFERENCES platform.cashier_price_template_versions(id),
  status VARCHAR(16) NOT NULL DEFAULT 'pending_review',
  diff_summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  reviewed_by BIGINT,
  reviewed_at TIMESTAMPTZ,
  review_note VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3) CHECK 约束补齐
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cashier_price_templates_fx_sync_mode_check') THEN
    ALTER TABLE platform.cashier_price_templates
      ADD CONSTRAINT cashier_price_templates_fx_sync_mode_check
      CHECK (fx_sync_mode IN ('manual_confirm', 'auto_apply'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cashier_price_templates_fx_sync_schedule_check') THEN
    ALTER TABLE platform.cashier_price_templates
      ADD CONSTRAINT cashier_price_templates_fx_sync_schedule_check
      CHECK (fx_sync_schedule IN ('monthly', 'quarterly'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cashier_price_template_versions_status_check') THEN
    ALTER TABLE platform.cashier_price_template_versions
      ADD CONSTRAINT cashier_price_template_versions_status_check
      CHECK (status IN ('draft', 'published', 'archived'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cashier_price_template_versions_source_type_check') THEN
    ALTER TABLE platform.cashier_price_template_versions
      ADD CONSTRAINT cashier_price_template_versions_source_type_check
      CHECK (source_type IN ('manual', 'copy_published', 'copy_archived', 'fx_auto'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cashier_fx_sync_runs_status_check') THEN
    ALTER TABLE platform.cashier_fx_sync_runs
      ADD CONSTRAINT cashier_fx_sync_runs_status_check
      CHECK (status IN ('pending_review', 'approved', 'applied', 'ignored', 'failed'));
  END IF;
END $$;

-- 4) 货币引用（平台级）
ALTER TABLE platform.cashier_price_rows
  DROP CONSTRAINT IF EXISTS cashier_price_rows_currency_fkey;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'cashier_price_rows_currency_fkey') THEN
    ALTER TABLE platform.cashier_price_rows
      ADD CONSTRAINT cashier_price_rows_currency_fkey
      FOREIGN KEY (currency) REFERENCES platform.currency_specs(currency_code);
  END IF;
END $$;

-- 5) 索引与唯一 published 约束
CREATE INDEX IF NOT EXISTS idx_cashier_template_versions_template ON platform.cashier_price_template_versions(template_id_ref);
CREATE INDEX IF NOT EXISTS idx_cashier_template_versions_status ON platform.cashier_price_template_versions(status);
CREATE INDEX IF NOT EXISTS idx_cashier_rows_version ON platform.cashier_price_rows(template_version_id_ref);
CREATE INDEX IF NOT EXISTS idx_cashier_fx_runs_template ON platform.cashier_fx_sync_runs(template_id_ref);
CREATE INDEX IF NOT EXISTS idx_cashier_fx_runs_status ON platform.cashier_fx_sync_runs(status);

-- 同模板最多一个 published（compact 规则，数据库层兜底）
CREATE UNIQUE INDEX IF NOT EXISTS uq_cashier_versions_one_published
  ON platform.cashier_price_template_versions(template_id_ref)
  WHERE status = 'published';
