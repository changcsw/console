-- 000019 · 渠道主数据枚举收敛（12-channel platform-admin）：
--   region 由 domestic/overseas 改为发行市场（GLOBAL/CN/JP/KR/SEA/HMT，与 market 同集）；
--   channel_type 收敛为 store/domestic/mini_game（海外商店/国内渠道/小游戏）。
-- 兼容性新口径：market=CN ⇔ region=CN；market≠CN ⇔ region ∈ {GLOBAL, market}。
-- 数据映射：region domestic→CN、overseas→GLOBAL；channel_type oem→domestic、web/direct→store。
-- 幂等：DROP CONSTRAINT IF EXISTS + 按旧值 UPDATE，可重复前向执行。

ALTER TABLE platform.channels DROP CONSTRAINT IF EXISTS channels_region_check;
ALTER TABLE platform.channels DROP CONSTRAINT IF EXISTS channels_channel_type_check;

UPDATE platform.channels SET region = 'CN'     WHERE region = 'domestic';
UPDATE platform.channels SET region = 'GLOBAL' WHERE region = 'overseas';

UPDATE platform.channels SET channel_type = 'domestic' WHERE channel_type = 'oem';
UPDATE platform.channels SET channel_type = 'store'    WHERE channel_type IN ('web', 'direct');

ALTER TABLE platform.channels ALTER COLUMN region SET DEFAULT 'GLOBAL';

ALTER TABLE platform.channels
  ADD CONSTRAINT channels_region_check CHECK (region IN ('GLOBAL', 'CN', 'JP', 'KR', 'SEA', 'HMT'));

ALTER TABLE platform.channels
  ADD CONSTRAINT channels_channel_type_check CHECK (channel_type IN ('store', 'domestic', 'mini_game'));
