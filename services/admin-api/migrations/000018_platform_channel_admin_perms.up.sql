-- 000018 · system 侧「平台渠道 + 渠道模版」管理权限码 seed
-- 背景：平台渠道主数据（platform.channels / platform.channel_policies）与渠道模版四件套
--   （platform.channel_login_templates / platform.channel_iap_templates）由系统管理员维护，与游戏无关；
--   游戏侧只在渠道实例上引用模版填参，继续用 channel.* / product.* 授权。
-- 因此新增独立权限码，避免把「改平台主数据」的能力混进游戏运营用的 channel.write。
-- 幂等：ON CONFLICT DO NOTHING，可重复执行。

INSERT INTO platform.admin_permissions (permission_code, permission_name) VALUES
  ('platform_channel.read',  '平台渠道-读'),
  ('platform_channel.write', '平台渠道-写'),
  ('channel_template.read',  '渠道模版-读'),
  ('channel_template.write', '渠道模版-写')
ON CONFLICT (permission_code) DO NOTHING;

-- super_admin 补授新权限（与 000003 §5 同口径）。
INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id
FROM platform.admin_roles r
JOIN platform.admin_permissions p
  ON p.permission_code IN (
    'platform_channel.read', 'platform_channel.write',
    'channel_template.read', 'channel_template.write'
  )
WHERE r.role_code = 'super_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;
