-- fixtures · 模块 account-auth（sandbox schema，业务数据样本）
-- 用于 tests/backend/scenarios/account-auth.yaml 中 fixture: sandbox/account-auth/* 引用的连库 harness。
-- game_account_auth_configs 为游戏维度业务表（每环境 schema 各一份、不带 env 列），本文件灌入 sandbox schema
-- （03-testing §7：sandbox/ = sandbox schema 业务数据样本）。幂等：ON CONFLICT DO NOTHING，可重复灌入。
-- 依赖：sandbox/game.sql 的 sandbox/game/base（游戏 100001）+ platform 平台目录（migration 000006 seed）。
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   sandbox/account-auth/base       → 游戏 100001，无任何 game_account_auth_configs 行；
--                                     读接口走「渠道允许集合并集」默认态合并（验 S1/S6/S10 起点）。
--   sandbox/account-auth/configured → 100001 的 google 已配置并启用（含密文位）；
--                                     验 S8 脱敏读 + 密文更新回归（masked/留空/仅 toggle 不改密文）。

SET search_path TO sandbox, platform;

-- ───────────────────────── sandbox/account-auth/base
-- 无业务行；仅依赖 100001 存在（来自 sandbox/game/base）。占位说明，无 INSERT。

-- ───────────────────────── sandbox/account-auth/configured：google 已配置（含密文）
-- config_json.clientSecret 存「密文位」（base64(nonce||ciphertext) 占位样本，绝非明文）。
-- 读接口恒返回 masked；密文更新回归断言此值在 masked/留空/仅 toggle 提交时保持不变。
INSERT INTO sandbox.game_account_auth_configs
  (game_id_ref, auth_type_id_ref, enabled, config_json, config_status, last_check_message)
SELECT g.id, at.id, TRUE,
       jsonb_build_object(
         'clientId', 'client-abc',
         'clientSecret', 'ZW5jOnRvcHNlY3JldC1jaXBoZXJ0ZXh0LXNhbXBsZQ==',
         'redirectUri', 'https://example.com/oauth/cb'
       ),
       'valid', ''
FROM sandbox.games g
JOIN platform.account_auth_types at ON at.auth_type_id = 'google'
WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, auth_type_id_ref) DO NOTHING;
