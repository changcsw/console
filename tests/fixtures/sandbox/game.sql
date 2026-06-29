-- fixtures · 模块 game（sandbox schema，业务数据样本）
-- 用于 tests/backend/scenarios/game.yaml 中 fixture: sandbox/game/* 引用的连库 harness。
-- 游戏维度三表（games / game_markets / game_legal_links）每环境 schema 各一份、不带 env 列；
-- 本文件灌入 sandbox schema（03-testing §7：sandbox/ = sandbox schema 业务数据样本）。
-- 幂等：ON CONFLICT DO NOTHING，可重复灌入。game_secret 本期明文存（响应恒脱敏）。
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   sandbox/game/base              → 游戏 100001（alias=demo-game）+ GLOBAL(默认)/JP 市场 + 一条 default 法务链接
--   sandbox/game/list              → 100001 + 100002（second-game），验列表/分页/改名冲突
--   sandbox/game/referenced_market → 100001 的 JP 市场被渠道实例引用（验删除保护 409）
--                                    channel 模块落地前 CountChannelsByMarket 恒 0 → 连库 harness 暂跳过该断言，
--                                    进程内 httptest 用注入计数等价覆盖。
--
-- game.yaml auth.role → RBAC 实体（平台级，落 platform schema；随 auth 角色/权限码体系）：
--   game_admin  → 角色含 game.read + game.write
--   game_reader → 角色仅含 game.read
--   no_perm     → 复用 common/auth.sql 的 no_perm（无任何权限）
-- 权限码 game.read / game.write 若未由 migration seed，则由连库 harness 装配期补齐。

SET search_path TO sandbox, platform;

-- ───────────────────────── RBAC：game 专用角色（platform schema）
INSERT INTO platform.admin_permissions (permission_code, permission_name)
VALUES ('game.read', '游戏-读'), ('game.write', '游戏-写')
ON CONFLICT (permission_code) DO NOTHING;

INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('game_admin', '游戏管理员'), ('game_reader', '游戏只读')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code IN ('game.read', 'game.write')
WHERE r.role_code = 'game_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code = 'game.read'
WHERE r.role_code = 'game_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- ───────────────────────── sandbox/game/base：游戏 100001
INSERT INTO sandbox.games (game_id, game_secret, name, alias, icon_url, default_market_code, status)
VALUES ('100001', 'sbx0000000000000000000000000000000000000000000000000000000000base', 'Demo Game', 'demo-game', '', 'GLOBAL', 'draft')
ON CONFLICT (game_id) DO NOTHING;

-- GLOBAL（默认，启用）+ JP（启用，非默认）
INSERT INTO sandbox.game_markets (game_id_ref, market_code, is_default, enabled, default_locale)
SELECT g.id, 'GLOBAL', TRUE, TRUE, 'en-US' FROM sandbox.games g WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, market_code) DO NOTHING;

INSERT INTO sandbox.game_markets (game_id_ref, market_code, is_default, enabled, default_locale)
SELECT g.id, 'JP', FALSE, TRUE, 'ja-JP' FROM sandbox.games g WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, market_code) DO NOTHING;

-- 一条 default 法务链接
INSERT INTO sandbox.game_legal_links (game_id_ref, scope_type, scope_value, terms_url, privacy_url, delete_account_url)
SELECT g.id, 'default', '*', 'https://example.com/terms', 'https://example.com/privacy', ''
FROM sandbox.games g WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, scope_type, scope_value) DO NOTHING;

-- ───────────────────────── sandbox/game/list：追加游戏 100002（second-game）
INSERT INTO sandbox.games (game_id, game_secret, name, alias, icon_url, default_market_code, status)
VALUES ('100002', 'sbx000000000000000000000000000000000000000000000000000000000list2', 'Second Game', 'second-game', '', 'GLOBAL', 'active')
ON CONFLICT (game_id) DO NOTHING;

INSERT INTO sandbox.game_markets (game_id_ref, market_code, is_default, enabled, default_locale)
SELECT g.id, 'GLOBAL', TRUE, TRUE, 'en-US' FROM sandbox.games g WHERE g.game_id = '100002'
ON CONFLICT (game_id_ref, market_code) DO NOTHING;

-- ───────────────────────── sandbox/game/referenced_market
-- 语义：100001 的 JP 市场下存在渠道实例（验删除保护 409）。
-- channel 表/计数 由 channel 模块落地后补；本 fixture 现仅复用 base 的 JP 市场，
-- 真实渠道引用计数待 channel 表就绪后在此追加 INSERT。
