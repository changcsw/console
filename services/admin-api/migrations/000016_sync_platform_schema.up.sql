-- 000016 · sync 模块：platform 同步任务与 token 去重表
-- 按 compact：sync_jobs / sync_job_items / sync_consumed_tokens 均落 platform。

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.sync_jobs (
  id BIGSERIAL PRIMARY KEY,
  game_id_ref VARCHAR(64) NOT NULL,
  source_env VARCHAR(16) NOT NULL CHECK (source_env IN ('develop','sandbox','production')),
  target_env VARCHAR(16) NOT NULL CHECK (target_env IN ('develop','sandbox','production')),
  source_hash VARCHAR(128) NOT NULL,
  target_hash_before VARCHAR(128) NOT NULL,
  target_hash_after VARCHAR(128) NOT NULL DEFAULT '',
  include_deletes BOOLEAN NOT NULL DEFAULT FALSE,
  operator_id BIGINT NOT NULL,
  operator_note VARCHAR(255) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL CHECK (status IN ('previewed','succeeded','failed')),
  executed_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.sync_job_items (
  id BIGSERIAL PRIMARY KEY,
  sync_job_id_ref BIGINT NOT NULL REFERENCES platform.sync_jobs(id),
  section VARCHAR(32) NOT NULL,
  entity_type VARCHAR(64) NOT NULL,
  entity_key VARCHAR(128) NOT NULL,
  op VARCHAR(16) NOT NULL CHECK (op IN ('add','update','delete')),
  field_name VARCHAR(64) NOT NULL,
  sandbox_value_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  production_value_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  masked BOOLEAN NOT NULL DEFAULT FALSE,
  applied BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.sync_consumed_tokens (
  id BIGSERIAL PRIMARY KEY,
  nonce VARCHAR(64) NOT NULL UNIQUE,
  sync_job_id_ref BIGINT NOT NULL REFERENCES platform.sync_jobs(id),
  consumed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sync_jobs_game_created
  ON platform.sync_jobs(game_id_ref, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_sync_job_items_job_section
  ON platform.sync_job_items(sync_job_id_ref, section);
