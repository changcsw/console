-- 000015 down · snapshot 约束与索引回滚（best effort）

DROP INDEX IF EXISTS idx_gcs_game_status;
DROP INDEX IF EXISTS idx_gcs_game_generated;

ALTER TABLE IF EXISTS game_config_snapshots DROP CONSTRAINT IF EXISTS uq_gcs_game_version;
ALTER TABLE IF EXISTS game_config_snapshots DROP CONSTRAINT IF EXISTS chk_gcs_status;
