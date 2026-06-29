-- 000005 down · 撤销 channel 渠道实例 market 维度/隐藏态/region（幂等，best-effort）
-- 不删除 000002 写入的 channel_policies seed。

DROP INDEX IF EXISTS idx_channel_packages_game_channel_id_ref;

DROP INDEX IF EXISTS idx_game_channels_channel_id_ref;
DROP INDEX IF EXISTS idx_game_channels_game_id_ref_market;
DROP INDEX IF EXISTS idx_game_channels_game_id_ref;

ALTER TABLE IF EXISTS game_channels DROP CONSTRAINT IF EXISTS game_channels_game_market_channel_key;
ALTER TABLE IF EXISTS game_channels DROP CONSTRAINT IF EXISTS game_channels_config_status_check;
ALTER TABLE IF EXISTS game_channels DROP CONSTRAINT IF EXISTS game_channels_market_code_check;

ALTER TABLE IF EXISTS game_channels DROP COLUMN IF EXISTS copied_from_market;
ALTER TABLE IF EXISTS game_channels DROP COLUMN IF EXISTS last_check_message;
ALTER TABLE IF EXISTS game_channels DROP COLUMN IF EXISTS last_check_at;
ALTER TABLE IF EXISTS game_channels DROP COLUMN IF EXISTS config_status;
ALTER TABLE IF EXISTS game_channels DROP COLUMN IF EXISTS hidden_at;
ALTER TABLE IF EXISTS game_channels DROP COLUMN IF EXISTS hidden_by;
ALTER TABLE IF EXISTS game_channels DROP COLUMN IF EXISTS hidden;
ALTER TABLE IF EXISTS game_channels DROP COLUMN IF EXISTS market_code;

DROP INDEX IF EXISTS idx_channels_enabled_sort;
DROP INDEX IF EXISTS idx_channels_region;
ALTER TABLE IF EXISTS channels DROP CONSTRAINT IF EXISTS channels_region_check;
ALTER TABLE IF EXISTS channels DROP COLUMN IF EXISTS region;
