-- fixtures · 模块 audit（审计日志）
-- 用于 tests/backend/scenarios/audit.yaml 中 fixture: common/audit/* 引用的连库 harness。
-- 幂等：ON CONFLICT DO NOTHING / WHERE NOT EXISTS，可重复灌入。
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   common/audit/base    → 跨 env 多条审计行（验列表/分页/排序/env 过滤/按 id 详情）
--   common/audit/secret  → detail 已脱敏（masked）的行（验读侧 S8：密文恒 masked）
--   common/audit/system  → actor_id=0 系统行（验 operator 系统占位）
-- audit 角色映射（manifest auth.role → 实体）：
--   audit_reader → 仅 audit.read（验 S1/S6/S8/S9 读侧）
--   no_perm      → 无任何权限（验缺 audit.read → 403，见 common/auth.sql）
--   super_admin  → 迁移 seed 超管（验写侧 S7/S8/S10 触发入口）
--
-- 说明（schema 观察）：audit_logs 由 000001 建于默认（public）schema，000003 仅迁移 admin_* 至
-- platform；仓储以未限定名 `audit_logs` 查询，依赖 search_path 含 public。compact 文档将其
-- 归为 platform schema 表，存在文档↔迁移命名差异（详见 audit.log.md「疑似实现缺陷」）。
-- 下列 INSERT 用未限定名，与仓储一致。

-- 0) 限权角色：audit_reader（仅 audit.read）
INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('audit_reader', '审计只读')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id
FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code = 'audit.read'
WHERE r.role_code = 'audit_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- 1) common/audit/base —— 跨 env 多条审计行；actor_id=1 复用迁移 seed 的超管
--    覆盖：sandbox/production 两 env、不同 action/resource_type、相同 created_at 验二级 id DESC 稳定。
INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, env, detail_json, created_at)
SELECT 1, 'game.update', 'game', 'g_1001', 'sandbox',
       '{"summary":"更新游戏 g_1001：name 变更","changed":["name"],"before":{"name":"Old"},"after":{"name":"New"}}'::jsonb,
       TIMESTAMPTZ '2026-06-17 08:00:00+00'
WHERE NOT EXISTS (SELECT 1 FROM audit_logs WHERE action='game.update' AND resource_id='g_1001');

INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, env, detail_json, created_at)
SELECT 1, 'game_channel.hide', 'game_channel', 'gc_2002', 'sandbox',
       '{"summary":"隐藏渠道实例 gc_2002"}'::jsonb,
       TIMESTAMPTZ '2026-06-17 08:00:00+00'
WHERE NOT EXISTS (SELECT 1 FROM audit_logs WHERE action='game_channel.hide' AND resource_id='gc_2002');

INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, env, detail_json, created_at)
SELECT 1, 'sync.execute', 'sync_job', 'g_1001', 'production',
       '{"summary":"同步 g_1001 sandbox→production","extra":{"syncJobId":"5567","appliedItemCount":14}}'::jsonb,
       TIMESTAMPTZ '2026-06-17 09:30:00+00'
WHERE NOT EXISTS (SELECT 1 FROM audit_logs WHERE action='sync.execute' AND resource_id='g_1001' AND env='production');

-- 2) common/audit/secret —— detail 内密文已脱敏为 masked（读侧 S8：绝不回明文）
INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, env, detail_json, created_at)
SELECT 1, 'payment_route.update', 'payment_route', 'pr_3003', 'production',
       '{"summary":"更新支付路由 pr_3003","changed":["secret"],"before":{"secret":"masked"},"after":{"secret":"masked","name":"new"}}'::jsonb,
       TIMESTAMPTZ '2026-06-17 10:00:00+00'
WHERE NOT EXISTS (SELECT 1 FROM audit_logs WHERE action='payment_route.update' AND resource_id='pr_3003');

-- 3) common/audit/system —— actor_id=0 系统占位（operator 解析为 System）
INSERT INTO audit_logs (actor_id, action, resource_type, resource_id, env, detail_json, created_at)
SELECT 0, 'fx.apply', 'cashier_fx_sync_run', 'run_4004', 'production',
       '{"summary":"系统自动应用汇率 run_4004"}'::jsonb,
       TIMESTAMPTZ '2026-06-17 11:00:00+00'
WHERE NOT EXISTS (SELECT 1 FROM audit_logs WHERE action='fx.apply' AND resource_id='run_4004');
