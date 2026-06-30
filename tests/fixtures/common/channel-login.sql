-- fixtures · 模块 channel-login（platform schema，全 env 共享）
-- 用于 tests/backend/scenarios/channel-login.yaml 中 fixture: common/channel-login/* 引用的连库 harness。
-- 平台前置：
--   - channels + channel_policies（login_mode/login_locked）已由 000002 seed：
--       huawei_cn/xiaomi_cn/oppo_cn/vivo_cn = channel_only（login_locked=TRUE）；其余 = account_system。
--   - channel_login_templates 的 huawei_cn v1 模板（appId 普通必填 + appSecret 密文，含 validation_rules）
--     已由 000007 seed 落 platform；xiaomi_cn 刻意「无模板」以验「无模板拒绝写」(S4)。
-- 本文件仅补充「测试专用」RBAC 角色，幂等可重复灌入。
--
-- 引用约定（manifest fixture/auth → 本文件片段）：
--   auth.role：
--     channel_reader → channel.read（验 S1/S8 读；缺写权限 S3 403 写）
--     channel_admin  → channel.read + channel.write（验 S1/S4/S5/S7/S8/S10 写）
--     no_perm        → 复用 common/auth.sql 的 no_perm（无任何权限 → 读写均 403）

-- ───────────────────────── 1) RBAC：渠道读写权限与角色（幂等）
INSERT INTO platform.admin_permissions (permission_code, permission_name)
VALUES ('channel.read', '渠道-读'), ('channel.write', '渠道-写')
ON CONFLICT (permission_code) DO NOTHING;

INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('channel_admin', '渠道管理员'), ('channel_reader', '渠道只读')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code IN ('channel.read', 'channel.write')
WHERE r.role_code = 'channel_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code = 'channel.read'
WHERE r.role_code = 'channel_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- ───────────────────────── 2) 模板四件套兜底：huawei_cn v1（若 000007 未生效时补齐，幂等）
-- 与 000007 seed 等价：appId 普通必填 + appSecret 密文必填 + validation_rules（minLen/maxLen/pattern）。
INSERT INTO platform.channel_login_templates (
  channel_id_ref, template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json, enabled
)
SELECT ch.id, 'v1',
  '[{"key":"appId","label":"App ID","component":"input","required":true,"order":10,"group":"basic","scope":"both"},
    {"key":"appSecret","label":"App Secret","component":"password","required":true,"order":20,"group":"secret","scope":"server"}]'::jsonb,
  '["appSecret"]'::jsonb,
  '[]'::jsonb,
  '{"appId":{"minLen":1,"maxLen":64,"pattern":"^[0-9A-Za-z_-]+$"},"appSecret":{"minLen":8,"maxLen":256}}'::jsonb,
  TRUE
FROM platform.channels ch
WHERE ch.channel_id = 'huawei_cn'
ON CONFLICT (channel_id_ref, template_version) DO NOTHING;
