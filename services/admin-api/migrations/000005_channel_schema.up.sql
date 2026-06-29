-- 000005 · channel 模块：渠道主数据 region 列 + 渠道实例 market 维度/隐藏态/配置状态（D2/D3，channel compact §数据模型）
-- 范围：platform 级 channels（新增 region）；游戏维度业务表 game_channels（market 维度/隐藏/config_status）与 channel_packages 索引。
-- channel_policies 的 login/payment 模式 seed 已在 000002 写入（取值与 compact 一致），本迁移不重复。
-- 追加新文件、不改历史；全部 IF EXISTS / IF NOT EXISTS / 存在性判断，可重复前向执行。
-- 约束名在 PostgreSQL 中按 namespace（schema）隔离；schema-per-env 下每环境 schema 各有一份业务表，
-- 存在性判断按 current_schema()（search_path 首项）限定 connamespace（对齐 000004）。
-- 偏差：channels/channel_policies 的 platform schema 归位属跨模块 bootstrap，未在本迁移内处理（见 handoff）。

-- ============ 1) channels：新增 region（D3） ============
ALTER TABLE channels
  ADD COLUMN IF NOT EXISTS region VARCHAR(16) NOT NULL DEFAULT 'overseas';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'channels_region_check' AND connamespace = current_schema()::regnamespace
  ) THEN
    ALTER TABLE channels
      ADD CONSTRAINT channels_region_check CHECK (region IN ('domestic', 'overseas'));
  END IF;
END $$;

-- region seed 回填：海外/国内（compact §channels seed）。幂等：仅按 channel_id 业务键覆盖固定取值。
UPDATE channels SET region = 'overseas' WHERE channel_id IN ('google', 'apple');
UPDATE channels SET region = 'domestic'
  WHERE channel_id IN ('huawei_cn', 'xiaomi_cn', 'oppo_cn', 'vivo_cn', 'wechat_mini_game', 'douyin_mini_game');

CREATE INDEX IF NOT EXISTS idx_channels_region        ON channels(region);
CREATE INDEX IF NOT EXISTS idx_channels_enabled_sort  ON channels(enabled, sort);

-- ============ 2) game_channels：market 维度 + 隐藏态 + config_status（D2） ============
ALTER TABLE game_channels ADD COLUMN IF NOT EXISTS market_code        VARCHAR(32)  NOT NULL DEFAULT 'GLOBAL';
ALTER TABLE game_channels ADD COLUMN IF NOT EXISTS hidden             BOOLEAN      NOT NULL DEFAULT FALSE;
ALTER TABLE game_channels ADD COLUMN IF NOT EXISTS hidden_by          VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE game_channels ADD COLUMN IF NOT EXISTS hidden_at          TIMESTAMPTZ;
ALTER TABLE game_channels ADD COLUMN IF NOT EXISTS config_status      VARCHAR(16)  NOT NULL DEFAULT 'empty';
ALTER TABLE game_channels ADD COLUMN IF NOT EXISTS last_check_at      TIMESTAMPTZ;
ALTER TABLE game_channels ADD COLUMN IF NOT EXISTS last_check_message VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE game_channels ADD COLUMN IF NOT EXISTS copied_from_market VARCHAR(32)  NOT NULL DEFAULT '';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'game_channels_market_code_check' AND connamespace = current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_channels
      ADD CONSTRAINT game_channels_market_code_check
      CHECK (market_code IN ('GLOBAL', 'JP', 'KR', 'SEA', 'HMT', 'CN'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'game_channels_config_status_check' AND connamespace = current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_channels
      ADD CONSTRAINT game_channels_config_status_check
      CHECK (config_status IN ('empty', 'invalid', 'valid'));
  END IF;
END $$;

-- 唯一键由 (game_id_ref, channel_id_ref) 改为 (game_id_ref, market_code, channel_id_ref)（D2）。
ALTER TABLE game_channels DROP CONSTRAINT IF EXISTS game_channels_game_id_ref_channel_id_ref_key;
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'game_channels_game_market_channel_key' AND connamespace = current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_channels
      ADD CONSTRAINT game_channels_game_market_channel_key
      UNIQUE (game_id_ref, market_code, channel_id_ref);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_game_channels_game_id_ref         ON game_channels(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_channels_game_id_ref_market  ON game_channels(game_id_ref, market_code);
CREATE INDEX IF NOT EXISTS idx_game_channels_channel_id_ref      ON game_channels(channel_id_ref);

-- ============ 3) channel_packages：FK 装配热路径索引 ============
CREATE INDEX IF NOT EXISTS idx_channel_packages_game_channel_id_ref ON channel_packages(game_channel_id_ref);
