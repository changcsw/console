-- 000003 down · 撤销 auth platform 归位与 seed（best-effort，幂等）

-- 删除 seed（按业务键）
DELETE FROM platform.admin_user_roles
WHERE user_id_ref IN (SELECT id FROM platform.admin_users WHERE user_name = 'admin');
DELETE FROM platform.admin_identities
WHERE identity_type = 'password' AND identity_key = 'admin';
DELETE FROM platform.admin_role_permissions
WHERE role_id_ref IN (SELECT id FROM platform.admin_roles WHERE role_code = 'super_admin');
DELETE FROM platform.admin_users WHERE user_name = 'admin';
DELETE FROM platform.admin_roles WHERE role_code = 'super_admin';
DELETE FROM platform.admin_permissions WHERE permission_code IN (
  'game.read','game.write','channel.read','channel.write','account_auth.read','account_auth.write',
  'channel_login.read','channel_login.write','plugin.read','plugin.write','product.read','product.write',
  'cashier.read','cashier.write','cashier.publish','fx.approve','payment.read','payment.write',
  'snapshot.read','snapshot.generate','snapshot.publish','sync.preview','sync.execute',
  'audit.read','dashboard.read','system.read','admin_user.write','role.write','permission.write'
);

-- 索引
DROP INDEX IF EXISTS platform.idx_admin_identities_user_id_ref;
DROP INDEX IF EXISTS platform.idx_admin_user_roles_user_id_ref;
DROP INDEX IF EXISTS platform.idx_admin_user_roles_role_id_ref;
DROP INDEX IF EXISTS platform.idx_admin_role_permissions_role_id_ref;
DROP INDEX IF EXISTS platform.idx_admin_role_permissions_perm_id_ref;

-- status CHECK
ALTER TABLE IF EXISTS platform.admin_users DROP CONSTRAINT IF EXISTS admin_users_status_check;

-- 归位回 public（与 000001 历史一致）
ALTER TABLE IF EXISTS platform.admin_role_permissions SET SCHEMA public;
ALTER TABLE IF EXISTS platform.admin_user_roles       SET SCHEMA public;
ALTER TABLE IF EXISTS platform.admin_permissions      SET SCHEMA public;
ALTER TABLE IF EXISTS platform.admin_roles            SET SCHEMA public;
ALTER TABLE IF EXISTS platform.admin_identities       SET SCHEMA public;
ALTER TABLE IF EXISTS platform.admin_users            SET SCHEMA public;
