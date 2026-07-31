-- 000019 down · 还原 region / channel_type 旧枚举与数据映射。
-- 注意：JP/KR/SEA/HMT 一律回落为 overseas（信息有损，down 仅作回滚兜底）。

ALTER TABLE platform.channels DROP CONSTRAINT IF EXISTS channels_region_check;
ALTER TABLE platform.channels DROP CONSTRAINT IF EXISTS channels_channel_type_check;

UPDATE platform.channels SET region = 'domestic' WHERE region = 'CN';
UPDATE platform.channels SET region = 'overseas' WHERE region IN ('GLOBAL', 'JP', 'KR', 'SEA', 'HMT');

UPDATE platform.channels SET channel_type = 'oem' WHERE channel_type = 'domestic';

ALTER TABLE platform.channels ALTER COLUMN region SET DEFAULT 'overseas';

ALTER TABLE platform.channels
  ADD CONSTRAINT channels_region_check CHECK (region IN ('domestic', 'overseas'));

ALTER TABLE platform.channels
  ADD CONSTRAINT channels_channel_type_check CHECK (channel_type IN ('store', 'oem', 'web', 'direct', 'mini_game'));
