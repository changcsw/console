-- fixtures · 模块 channel-login（sandbox schema，业务数据样本）
-- 用于 tests/backend/scenarios/channel-login.yaml 中 fixture: sandbox/channel-login/* 引用的连库 harness。
-- game_channel_login_configs 为游戏维度业务表（每环境 schema 各一份、不带 env 列），灌入 sandbox schema
-- （03-testing §7）。幂等：ON CONFLICT DO NOTHING，可重复灌入。
-- 依赖：sandbox/game.sql 的 sandbox/game/base（游戏 100001）+ platform 渠道目录/策略/模板（migration + common/channel-login.sql）。
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   sandbox/channel-login/base       → 游戏 100001 的渠道实例：
--                                        huawei_cn@CN（channel_only，有模板，未配置 → GET 空占位/PUT 起点）
--                                        google@JP   （account_system → 验非 channel_only 拒绝 S4）
--                                        xiaomi_cn@CN（channel_only，无模板 → 验无模板拒绝 S4）
--   sandbox/channel-login/configured → huawei_cn@CN 已配置并启用（含密文位）：验 S8 脱敏读。

SET search_path TO sandbox, platform;

-- ───────────────────────── sandbox/channel-login/base：渠道实例（market 维度）
INSERT INTO sandbox.game_channels (game_id_ref, channel_id_ref, market_code, enabled, config_status, remark)
SELECT g.id, ch.id, 'CN', TRUE, 'empty', 'channel-login fixture'
FROM sandbox.games g
JOIN platform.channels ch ON ch.channel_id = 'huawei_cn'
WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, market_code, channel_id_ref) DO NOTHING;

INSERT INTO sandbox.game_channels (game_id_ref, channel_id_ref, market_code, enabled, config_status, remark)
SELECT g.id, ch.id, 'JP', TRUE, 'empty', 'channel-login fixture (account_system)'
FROM sandbox.games g
JOIN platform.channels ch ON ch.channel_id = 'google'
WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, market_code, channel_id_ref) DO NOTHING;

INSERT INTO sandbox.game_channels (game_id_ref, channel_id_ref, market_code, enabled, config_status, remark)
SELECT g.id, ch.id, 'CN', TRUE, 'empty', 'channel-login fixture (no template)'
FROM sandbox.games g
JOIN platform.channels ch ON ch.channel_id = 'xiaomi_cn'
WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, market_code, channel_id_ref) DO NOTHING;

-- ───────────────────────── sandbox/channel-login/configured：huawei_cn@CN 已配置（含密文）
-- config_json.appSecret 存「密文位」（base64 占位样本，绝非明文）；读接口恒返回 ******。
INSERT INTO sandbox.game_channel_login_configs
  (game_channel_id_ref, enabled, config_json, config_status, last_check_message)
SELECT gc.id, TRUE,
       jsonb_build_object(
         'appId', 'app-123_AZ',
         'appSecret', 'ZW5jOmFwcHNlY3JldC1jaXBoZXJ0ZXh0LXNhbXBsZQ=='
       ),
       'valid', ''
FROM sandbox.game_channels gc
JOIN sandbox.games g ON g.id = gc.game_id_ref
JOIN platform.channels ch ON ch.id = gc.channel_id_ref
WHERE g.game_id = '100001' AND ch.channel_id = 'huawei_cn' AND gc.market_code = 'CN'
ON CONFLICT (game_channel_id_ref) DO NOTHING;
