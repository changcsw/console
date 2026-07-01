-- 000015 · snapshot 模块：game_config_snapshots 幂等建表/约束/索引
-- 业务表 SQL 不写 schema 前缀，依赖 search_path=<env>,platform。

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
  published_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE game_config_snapshots DROP CONSTRAINT IF EXISTS chk_gcs_status;
ALTER TABLE game_config_snapshots DROP CONSTRAINT IF EXISTS uq_gcs_game_version;
ALTER TABLE game_config_snapshots DROP CONSTRAINT IF EXISTS game_config_snapshots_game_id_ref_config_version_key;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='chk_gcs_status' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_config_snapshots
      ADD CONSTRAINT chk_gcs_status
      CHECK (status IN ('draft','published'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='uq_gcs_game_version' AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_config_snapshots
      ADD CONSTRAINT uq_gcs_game_version
      UNIQUE (game_id_ref, config_version);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_gcs_game_generated
  ON game_config_snapshots (game_id_ref, generated_at DESC);

CREATE INDEX IF NOT EXISTS idx_gcs_game_status
  ON game_config_snapshots (game_id_ref, status);
