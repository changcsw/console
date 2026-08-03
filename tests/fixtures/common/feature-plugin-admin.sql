-- fixtures · 模块 feature-plugin-admin（platform schema，全 env 共享）
-- 用于 tests/backend/scenarios/feature-plugin-admin.yaml 中 fixture: common/feature-plugin-admin/*
-- 引用的连库 harness。这是「系统管理员维护插件分类字典/主数据/参数模板」的后台管理接口
-- （权限码 feature_plugin.read / feature_plugin.write，由 migrations/000020 建表 + seed），
-- 与游戏侧渠道实例插件配置（plugin.read/plugin.write，tests/fixtures/common/feature-plugin.sql）
-- 是两套完全独立的权限与数据，互不复用、互不干扰。
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   common/feature-plugin-admin/base → RBAC 角色 + 分类/插件/模板基线样本
--
-- auth.role → RBAC 实体：
--   feature_plugin_admin  → feature_plugin.read + feature_plugin.write（S1/S5/S7 跑通）
--   feature_plugin_reader → 仅 feature_plugin.read（验缺写权限 403）
--   no_perm                → 复用 common/auth.sql 的 no_perm（无任何权限 → 读写均 403）
--
-- id 显式固定为 9001+（与 tests/fixtures/sandbox/product.sql 的约定一致）：这三张表的主键是
-- BIGSERIAL 自增列，多模块共享同一批 fixture 灌入顺序不固定、且随其它模块测试活动增长，
-- 若不显式定 id，scenario yaml 里按数字路径引用（如 /feature-plugin-categories/{id}）会跟着漂移。
-- 幂等：先清理上一轮 scenario 创建的实体（见 §0），再 ON CONFLICT DO NOTHING 按业务键补齐基线，
-- 可重复灌入。业务键（category_code/plugin_id/
-- template_version）均加 qa_ 前缀，与 migrations/000020 seed（login/payment/push/ad）及模块 15 的
-- common/feature-plugin.sql（realname/customer_service/…）互不冲突。

-- ───────────────────────── 0) 清理上一轮 scenario 亲手创建的实体（保证连库回归可重复跑）
-- create_*_success 三个 case 会真建分类/插件/模板，这些行不属于 fixture 基线；若不清掉，第二轮
-- 连库回归会因唯一键冲突拿到 409 而假失败。删除顺序：模板 → 插件 → 分类（外键依赖自内向外）。
DELETE FROM platform.feature_plugin_templates
WHERE plugin_id_ref IN (
  SELECT id FROM platform.feature_plugins
  WHERE plugin_id IN ('qa_sample_plugin', 'qa_scenario_created_plugin')
);

DELETE FROM platform.feature_plugins WHERE plugin_id = 'qa_scenario_created_plugin';

DELETE FROM platform.feature_plugin_categories WHERE category_code = 'qa_scenario_created_cat';

-- ───────────────────────── 1) RBAC：功能插件管理读写角色（幂等）
INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('feature_plugin_admin', '功能插件管理员'), ('feature_plugin_reader', '功能插件只读')
ON CONFLICT (role_code) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code IN ('feature_plugin.read', 'feature_plugin.write')
WHERE r.role_code = 'feature_plugin_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code = 'feature_plugin.read'
WHERE r.role_code = 'feature_plugin_reader'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- ───────────────────────── 2) 分类字典样本（固定 id 9001/9002）
--   9001 qa_sample_cat    : 有插件挂载（qa_sample_plugin）→ 删除应 409（分类删除保护）
--   9002 qa_removable_cat : 无插件挂载 → 删除应 204（成功路径）
INSERT INTO platform.feature_plugin_categories (id, category_code, category_name, enabled, sort)
VALUES
  (9001, 'qa_sample_cat',    'QA 示例分类', TRUE, 910),
  (9002, 'qa_removable_cat', 'QA 可删分类', TRUE, 920)
ON CONFLICT (category_code) DO NOTHING;

-- ───────────────────────── 3) 插件主数据样本（固定 id 9001/9002/9003）
--   9001 qa_sample_plugin    : 归类 qa_sample_cat(9001)，无模板 → 详情/列表/改字段/创建模板成功样本；
--                              同时是 qa_sample_cat 删除保护（409）样本的挂载插件，测试不应改动其分类归属。
--   9002 qa_template_plugin  : 无分类，挂 1 个模板版本 v1（固定 id 9001）→
--                              模板列表/详情/重复版本冲突样本；同时用于「删除被模板引用 → 409」样本。
--   9003 qa_deletable_plugin : 无分类/无模板/无引用 → DELETE 成功样本（一次性消耗，见 yaml 注释）。
--   9004 qa_cascade_plugin   : 无分类，挂 1 个模板版本 v1（固定 id 9002）、无渠道侧引用 →
--                              「删除插件时级联删除模板版本」成功样本（一次性消耗，见 yaml 注释）。
--                              与 9002 分开是为了不破坏本文件其它模板用例依赖的 qa_template_plugin。
INSERT INTO platform.feature_plugins (id, plugin_id, plugin_name, category_id_ref, region, enabled, sort)
VALUES (9001, 'qa_sample_plugin', 'QA 示例插件', 9001, 'domestic', TRUE, 910)
ON CONFLICT (plugin_id) DO NOTHING;

INSERT INTO platform.feature_plugins (id, plugin_id, plugin_name, category_id_ref, region, enabled, sort)
VALUES (9002, 'qa_template_plugin', 'QA 模板插件', NULL, 'overseas', TRUE, 920)
ON CONFLICT (plugin_id) DO NOTHING;

INSERT INTO platform.feature_plugins (id, plugin_id, plugin_name, category_id_ref, region, enabled, sort)
VALUES (9003, 'qa_deletable_plugin', 'QA 可删插件', NULL, 'domestic', TRUE, 930)
ON CONFLICT (plugin_id) DO NOTHING;

INSERT INTO platform.feature_plugins (id, plugin_id, plugin_name, category_id_ref, region, enabled, sort)
VALUES (9004, 'qa_cascade_plugin', 'QA 级联删除插件', NULL, 'domestic', TRUE, 940)
ON CONFLICT (plugin_id) DO NOTHING;

-- ───────────────────────── 4) 插件参数模板样本
--   9001（qa_template_plugin / v1，enabled=TRUE 生效）：模板列表/详情/重复版本冲突样本。
--   9002（qa_cascade_plugin / v1）：随 9004 一起被 delete_plugin_cascade_deletes_templates 级联删掉；
--     该 case 是一次性用例，重灌 fixture 时这两行会被上面的 INSERT 一并补回。
INSERT INTO platform.feature_plugin_templates (
  id, plugin_id_ref, template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json, enabled
)
VALUES (
  9001, 9002, 'v1',
  '[{"key":"appId","label":"App ID","component":"input","required":true,"order":10,"group":"basic","scope":"both"}]'::jsonb,
  '[]'::jsonb,
  '[]'::jsonb,
  '{"appId":{"minLen":1,"maxLen":64}}'::jsonb,
  TRUE
)
ON CONFLICT (plugin_id_ref, template_version) DO NOTHING;

INSERT INTO platform.feature_plugin_templates (
  id, plugin_id_ref, template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json, enabled
)
VALUES (
  9002, 9004, 'v1',
  '[{"key":"appId","label":"App ID","component":"input","required":true,"order":10,"group":"basic","scope":"both"}]'::jsonb,
  '[]'::jsonb,
  '[]'::jsonb,
  '{}'::jsonb,
  TRUE
)
ON CONFLICT (plugin_id_ref, template_version) DO NOTHING;
