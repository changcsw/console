-- fixtures · 模块 auth（platform schema，全 env 共享）
-- 用于 tests/backend/scenarios/auth.yaml 中 fixture: common/auth/* 引用的连库 harness。
-- 幂等：ON CONFLICT DO NOTHING，可重复灌入。基础 super_admin / admin / 29 权限码已由
-- migrations/000003 seed，这里仅补充「测试专用」实体（禁用用户、飞书身份、限权角色、可删实体）。
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   common/auth/base                  → admin(active) + 角色齐备（迁移已 seed，足够大多数 case）
--   common/auth/disabled              → disabled_user（验「禁用即拒绝」）
--   common/auth/feishu                → 已绑定 feishu 身份的 alice（验飞书回调 + 脱敏）
--   common/auth/unreferenced_role     → 未被任何用户引用的角色（验可删 + 解绑）
--   common/auth/unreferenced_permission → 未被任何角色引用的权限码（验可删）
-- auth 角色映射（manifest auth.role → 实体）：
--   super_admin   → 迁移 seed 的超级管理员（全量权限，S1/S6/S7/S8/S9 跑通）
--   system_reader → 仅 system.read（验缺写权限 403）
--   no_perm       → 无任何权限（验缺读权限 403）

-- 1) 禁用用户（密码 = Admin@12345，bcrypt cost=10）
INSERT INTO platform.admin_users (user_name, display_name, email, status)
VALUES ('disabled_user', 'Disabled User', '', 'disabled')
ON CONFLICT (user_name) DO NOTHING;

INSERT INTO platform.admin_identities (user_id_ref, identity_type, identity_key, credential_ciphertext)
SELECT u.id, 'password', u.user_name, '$2a$10$wSKwXEKswD/tQ11fLGrP.uqVE1CuIh/Pyw7SLJfbq7D9/HHNfzMzO'
FROM platform.admin_users u
WHERE u.user_name = 'disabled_user'
ON CONFLICT (identity_type, identity_key) DO NOTHING;

-- 2) 飞书绑定用户 alice（identity_key = union_id，验回调命中 + identityKey 脱敏）
INSERT INTO platform.admin_users (user_name, display_name, email, status)
VALUES ('alice', 'Alice', 'alice@example.com', 'active')
ON CONFLICT (user_name) DO NOTHING;

INSERT INTO platform.admin_identities (user_id_ref, identity_type, identity_key, credential_ciphertext)
SELECT u.id, 'feishu', 'on_1234567890abcd', ''
FROM platform.admin_users u
WHERE u.user_name = 'alice'
ON CONFLICT (identity_type, identity_key) DO NOTHING;

-- 3) 限权角色：system_reader（仅 system.read）
INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('system_reader', '系统只读')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id
FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code = 'system.read'
WHERE r.role_code = 'system_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- 4) 限权角色：no_perm（无任何权限）
INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('no_perm', '无权限角色')
ON CONFLICT (role_code) DO NOTHING;

-- 5) 可删除（未被引用）角色 —— 验 DELETE /system/roles/{id} 成功路径
INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('temp_removable_role', '临时可删角色')
ON CONFLICT (role_code) DO NOTHING;

-- 6) 可删除（未被引用）权限码 —— 验 DELETE /system/permissions/{id} 成功路径
INSERT INTO platform.admin_permissions (permission_code, permission_name)
VALUES ('demo.removable', '可删演示权限')
ON CONFLICT (permission_code) DO NOTHING;
