-- fixtures · 模块 dashboard（platform schema + RBAC）
-- 用于 tests/backend/scenarios/dashboard.yaml 中 common/dashboard/* 引用的连库 harness。
-- 当前文件仅提供权限角色种子，业务样本由上游模块 fixtures 组合提供。

INSERT INTO platform.admin_permissions (permission_code, permission_name)
VALUES ('dashboard.read', 'Dashboard-读')
ON CONFLICT (permission_code) DO NOTHING;

INSERT INTO platform.admin_roles (role_code, role_name)
VALUES
  ('dashboard_reader', 'Dashboard只读'),
  ('dashboard_partial_reader', 'Dashboard受限只读')
ON CONFLICT (role_code) DO NOTHING;

-- dashboard_reader：拥有 dashboard 所有读侧依赖权限（用于 S1/S6/S7/S10）
INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id
FROM platform.admin_roles r
JOIN platform.admin_permissions p
  ON p.permission_code IN ('dashboard.read', 'cashier.read', 'channel.read', 'game.read', 'sync.preview', 'snapshot.read')
WHERE r.role_code = 'dashboard_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- dashboard_partial_reader：故意缺少 cashier.read/sync.preview/snapshot.read，验证指标级权限裁剪
INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id
FROM platform.admin_roles r
JOIN platform.admin_permissions p
  ON p.permission_code IN ('dashboard.read', 'channel.read', 'game.read')
WHERE r.role_code = 'dashboard_partial_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;
