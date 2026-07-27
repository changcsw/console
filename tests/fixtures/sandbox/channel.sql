-- fixtures · 模块 channel（sandbox schema，业务数据样本）
-- 用于 tests/backend/scenarios/channel.yaml 中 fixture: sandbox/channel/* 引用的连库 harness。
-- game_channels / channel_packages 为游戏维度业务表（每环境 schema 各一份、不带 env 列），灌入 sandbox schema
-- （03-testing §7）。幂等：ON CONFLICT DO NOTHING，可重复灌入。
-- 依赖：sandbox/game.sql 的 sandbox/game/base（游戏 100001 + JP 市场）+ platform 渠道目录（migration 000002 seed）。
-- RBAC：channel_admin / channel_reader / no_perm 由 common/channel-login.sql + common/auth.sql 提供。
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   sandbox/channel/base → 游戏 100001 的渠道实例与包（固定 id 供 scenario 路径引用）：
--       id=1  JP/google  config_status=empty  + 包 pkg.a（验 hide 409 / 包级读写 / create 冲突）
--       id=2  JP/apple   config_status=valid   未隐藏（验 hide/unhide 200 + 审计）
--
-- auth.role → RBAC 实体（见 common/channel-login.sql）：
--   channel_reader → channel.read
--   channel_admin  → channel.read + channel.write
--   no_perm        → 无任何权限

SET search_path TO sandbox, platform;

-- ───────────────────────── sandbox/channel/base：固定 id 的渠道实例（JP 市场）
INSERT INTO sandbox.game_channels (
  id, game_id_ref, channel_id_ref, market_code, enabled, hidden, config_status, remark
)
SELECT 1, g.id, ch.id, 'JP', TRUE, FALSE, 'empty', 'channel fixture base'
FROM sandbox.games g
JOIN platform.channels ch ON ch.channel_id = 'google'
WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, market_code, channel_id_ref) DO NOTHING;

INSERT INTO sandbox.game_channels (
  id, game_id_ref, channel_id_ref, market_code, enabled, hidden, config_status, remark
)
SELECT 2, g.id, ch.id, 'JP', TRUE, FALSE, 'valid', 'channel fixture hide target'
FROM sandbox.games g
JOIN platform.channels ch ON ch.channel_id = 'apple'
WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, market_code, channel_id_ref) DO NOTHING;

-- 包 id=1：归属 game_channel 1，market 与实例一致（JP）
INSERT INTO sandbox.channel_packages (
  id, game_channel_id_ref, package_code, package_name, market_code, bundle_id, inherit_channel_config, enabled
)
VALUES (1, 1, 'pkg.a', 'Pkg A', 'JP', 'com.demo.jp.a', TRUE, TRUE)
ON CONFLICT (game_channel_id_ref, package_code) DO NOTHING;

-- 对齐 BIGSERIAL（序列表在 public，各 env schema 的 id 列共用）
SELECT setval(
  'public.game_channels_id_seq',
  (SELECT GREATEST(COALESCE(MAX(id), 1), 2) FROM sandbox.game_channels),
  true
);
SELECT setval(
  'public.channel_packages_id_seq',
  (SELECT GREATEST(COALESCE(MAX(id), 1), 1) FROM sandbox.channel_packages),
  true
);
