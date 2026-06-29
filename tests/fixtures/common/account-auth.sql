-- fixtures · 模块 account-auth（platform schema，全 env 共享）
-- 用于 tests/backend/scenarios/account-auth.yaml 中 fixture: common/account-auth/* 引用的连库 harness。
-- 平台三表（account_auth_types / channel_account_auth_types / account_auth_templates）+ seed 已由
-- migrations/000006 写入（guest/phone/email/google/apple/line/kakao 类型、account_system 渠道默认映射、google v1 模板）。
-- 本文件仅补充「测试专用」前置：RBAC 游戏角色、locked 渠道策略、缺模板类型，幂等可重复灌入。
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   common/account-auth/catalog → 平台目录就绪（migration seed）+ 本文件 RBAC/locked/缺模板补充
-- auth.role → RBAC 实体（account-auth 接口复用 game.read/game.write 权限码）：
--   game_admin  → game.read + game.write（S1/S5/S7/S8/S10 跑通）
--   game_reader → 仅 game.read（验缺写权限 403；可读三个 GET）
--   no_perm     → 复用 common/auth.sql 的 no_perm（无任何权限）

-- ───────────────────────── 1) RBAC：游戏读写角色（与 sandbox/game.sql 同源，幂等）
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

-- ───────────────────────── 2) locked 渠道策略：line 在 'google' 渠道下锁定（验 locked 游戏侧不可改）
-- 仅当 'google' 渠道存在该映射时补 locked=true；不存在则插入（default_enabled=false, locked=true）。
INSERT INTO platform.channel_account_auth_types (channel_id_ref, auth_type_id_ref, default_enabled, locked, sort)
SELECT ch.id, at.id, FALSE, TRUE, at.sort
FROM platform.channels ch
JOIN platform.account_auth_types at ON at.auth_type_id = 'line'
WHERE ch.channel_id = 'google'
ON CONFLICT (channel_id_ref, auth_type_id_ref) DO UPDATE SET locked = TRUE, updated_at = NOW();

-- ───────────────────────── 3) 缺模板类型：apple 无可用模板（验 ACCOUNT_AUTH_TEMPLATE_NOT_FOUND）
-- migration seed 为所有类型建了 v1 空模板；本前置删除 apple 的模板，制造「启用即缺模板」场景。
DELETE FROM platform.account_auth_templates t
USING platform.account_auth_types at
WHERE t.auth_type_id_ref = at.id AND at.auth_type_id = 'apple';
