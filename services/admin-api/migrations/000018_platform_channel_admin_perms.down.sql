-- 000018 down · 回收 system 侧平台渠道/渠道模版权限码（先解绑角色再删码）。

DELETE FROM platform.admin_role_permissions
WHERE permission_id_ref IN (
  SELECT id FROM platform.admin_permissions
  WHERE permission_code IN (
    'platform_channel.read', 'platform_channel.write',
    'channel_template.read', 'channel_template.write'
  )
);

DELETE FROM platform.admin_permissions
WHERE permission_code IN (
  'platform_channel.read', 'platform_channel.write',
  'channel_template.read', 'channel_template.write'
);
