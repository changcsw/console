-- 000003 · auth 模块：platform schema 归位 + 约束/索引补齐 + RBAC seed
-- 范围：仅处理 auth 的 admin_* 平台级表（00 §2.2）。业务表的每环境 schema 拆分不在本迁移内
--       （属跨模块 bootstrap 职责，见 handoff 偏差说明）。
-- 幂等：全部使用 IF EXISTS / IF NOT EXISTS / ON CONFLICT，可重复执行。
-- 假设：000001 在默认 schema（public）建了 admin_* 六张表；本迁移把它们归位到 platform。

CREATE SCHEMA IF NOT EXISTS platform;

-- 1) admin_* 平台级表归位到 platform（幂等：移动后 public.* 不再存在则跳过）
ALTER TABLE IF EXISTS public.admin_users            SET SCHEMA platform;
ALTER TABLE IF EXISTS public.admin_identities       SET SCHEMA platform;
ALTER TABLE IF EXISTS public.admin_roles            SET SCHEMA platform;
ALTER TABLE IF EXISTS public.admin_permissions      SET SCHEMA platform;
ALTER TABLE IF EXISTS public.admin_user_roles       SET SCHEMA platform;
ALTER TABLE IF EXISTS public.admin_role_permissions SET SCHEMA platform;

-- 2) admin_users.status CHECK(active/disabled)
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'admin_users_status_check'
  ) THEN
    ALTER TABLE platform.admin_users
      ADD CONSTRAINT admin_users_status_check CHECK (status IN ('active', 'disabled'));
  END IF;
END $$;

-- 3) 建议索引（FK / 权限解析连接热路径）
CREATE INDEX IF NOT EXISTS idx_admin_identities_user_id_ref       ON platform.admin_identities(user_id_ref);
CREATE INDEX IF NOT EXISTS idx_admin_user_roles_user_id_ref       ON platform.admin_user_roles(user_id_ref);
CREATE INDEX IF NOT EXISTS idx_admin_user_roles_role_id_ref       ON platform.admin_user_roles(role_id_ref);
CREATE INDEX IF NOT EXISTS idx_admin_role_permissions_role_id_ref ON platform.admin_role_permissions(role_id_ref);
CREATE INDEX IF NOT EXISTS idx_admin_role_permissions_perm_id_ref ON platform.admin_role_permissions(permission_id_ref);

-- 4) 权限码目录 seed（compact「权限码命名与 seed 清单」）
INSERT INTO platform.admin_permissions (permission_code, permission_name) VALUES
  ('game.read',           '游戏-读'),
  ('game.write',          '游戏-写'),
  ('channel.read',        '渠道-读'),
  ('channel.write',       '渠道-写'),
  ('account_auth.read',   '账号认证-读'),
  ('account_auth.write',  '账号认证-写'),
  ('channel_login.read',  '渠道登录-读'),
  ('channel_login.write', '渠道登录-写'),
  ('plugin.read',         '功能插件-读'),
  ('plugin.write',        '功能插件-写'),
  ('product.read',        '商品-读'),
  ('product.write',       '商品-写'),
  ('cashier.read',        '收银台-读'),
  ('cashier.write',       '收银台-写'),
  ('cashier.publish',     '收银台-发布'),
  ('fx.approve',          '汇率-审核'),
  ('payment.read',        '支付路由-读'),
  ('payment.write',       '支付路由-写'),
  ('snapshot.read',       '配置快照-读'),
  ('snapshot.generate',   '配置快照-生成'),
  ('snapshot.publish',    '配置快照-发布'),
  ('sync.preview',        '同步-预览'),
  ('sync.execute',        '同步-执行'),
  ('audit.read',          '审计-读'),
  ('dashboard.read',      'Dashboard-读'),
  ('system.read',         '系统管理-读'),
  ('admin_user.write',    '管理员-写'),
  ('role.write',          '角色-写'),
  ('permission.write',    '权限码-写')
ON CONFLICT (permission_code) DO NOTHING;

-- 5) super_admin 角色 + 授全量权限
INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('super_admin', '超级管理员')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id
FROM platform.admin_roles r
CROSS JOIN platform.admin_permissions p
WHERE r.role_code = 'super_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- 6) 初始管理员 admin（明文绝不入库；credential 为 bcrypt 哈希占位，对应明文 ChangeMe_123!，上线须立即重置）
INSERT INTO platform.admin_users (user_name, display_name, email, status)
VALUES ('admin', 'Administrator', '', 'active')
ON CONFLICT (user_name) DO NOTHING;

INSERT INTO platform.admin_identities (user_id_ref, identity_type, identity_key, credential_ciphertext)
SELECT u.id, 'password', u.user_name, '$2a$10$mMSFj36823l6DawT5cGY7effWmFA1zNG0v9mzew3TlcsUkDcN98Z.'
FROM platform.admin_users u
WHERE u.user_name = 'admin'
ON CONFLICT (identity_type, identity_key) DO NOTHING;

INSERT INTO platform.admin_user_roles (user_id_ref, role_id_ref)
SELECT u.id, r.id
FROM platform.admin_users u, platform.admin_roles r
WHERE u.user_name = 'admin' AND r.role_code = 'super_admin'
ON CONFLICT (user_id_ref, role_id_ref) DO NOTHING;
