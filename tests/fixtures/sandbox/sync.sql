-- fixtures · 模块 sync（#21 · Sandbox→Production 同步）— sandbox schema「同步源」样本
-- 用于 tests/backend/scenarios/sync.yaml 中 fixture: sandbox/sync/* 引用的连库 harness。
-- sync 是唯一显式跨 schema 域：本文件灌 sandbox schema（源），production/sync.sql 灌 production schema（目标基线）。
-- 幂等：ON CONFLICT DO NOTHING，可重复灌入。密文列存密文，preview 恒 masked（03-testing §7 / 00 §6）。
--
-- 前置依赖（连库 harness 须先灌入）：
--   sandbox/game.sql             → sandbox.games('100001') + GLOBAL/JP 市场
--   production/sync.sql          → production baseline（构造 add/update/delete 差异）
--   migrations/000016            → platform.sync_jobs / sync_job_items / sync_consumed_tokens
--
-- 引用约定（manifest fixture: 名 → 片段）：
--   sandbox/sync/base    → RBAC + 游戏 100001 有效数据（channels/products 相对 production 存在 add/update）
--   sandbox/sync/secret  → 含密文字段（login clientSecret / *_ciphertext）→ preview masked（S8）
--
-- sync.yaml auth.role → RBAC 实体（platform schema）：
--   sync_operator              → sync.preview（可 preview + 查 sync-jobs）
--   sync_executor              → sync.preview + sync.execute（可执行同步）
--   sync_operator_preview_only → sync.preview（用于 execute 缺 execute 权限 → 403）
--   sync_reader_noperm         → 无任何 sync 权限（preview/list → 403）

SET search_path TO sandbox, platform;

-- ───────────────────────── RBAC：sync 权限码 + 角色（platform schema）
INSERT INTO platform.admin_permissions (permission_code, permission_name)
VALUES ('sync.preview', '同步-预览'), ('sync.execute', '同步-执行')
ON CONFLICT (permission_code) DO NOTHING;

INSERT INTO platform.admin_roles (role_code, role_name)
VALUES
  ('sync_operator', '同步操作员(预览)'),
  ('sync_executor', '同步执行员'),
  ('sync_operator_preview_only', '同步仅预览'),
  ('sync_reader_noperm', '同步无权限')
ON CONFLICT (role_code) DO NOTHING;

-- sync_operator ← sync.preview
INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code = 'sync.preview'
WHERE r.role_code IN ('sync_operator', 'sync_operator_preview_only')
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- sync_executor ← sync.preview + sync.execute
INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id FROM platform.admin_roles r
JOIN platform.admin_permissions p ON p.permission_code IN ('sync.preview', 'sync.execute')
WHERE r.role_code = 'sync_executor'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- ───────────────────────── sandbox/sync/base：游戏 100001 有效数据（依赖 sandbox/game.sql 的 games/markets）
-- 渠道实例：JP/google（有效）。production 无 → 触发 channels.add。
INSERT INTO sandbox.game_channels (game_id_ref, channel_id_ref, market_code, enabled, hidden, config_status, remark)
SELECT g.id, ch.id, 'JP', TRUE, FALSE, 'valid', 'sandbox-only'
FROM sandbox.games g, platform.channels ch
WHERE g.game_id = '100001' AND ch.channel_id = 'google'
ON CONFLICT DO NOTHING;

-- 商品：gem_1（sandbox 改名 → 相对 production 触发 products.update: product_name 字段）
INSERT INTO sandbox.products (game_id_ref, product_id, product_name, base_amount_minor, base_currency, price_id, enabled)
SELECT g.id, 'gem_1', 'Gems Pack A (v2)', 100, 'USD', 'price_gem_1', TRUE
FROM sandbox.games g WHERE g.game_id = '100001'
ON CONFLICT DO NOTHING;

-- 失效数据全程排除：hidden/config_status!=valid/enabled=false 不进有效集（不产生 add/update）
INSERT INTO sandbox.products (game_id_ref, product_id, product_name, base_amount_minor, base_currency, price_id, enabled)
SELECT g.id, 'gem_disabled', 'Disabled Pack', 50, 'USD', 'price_disabled', FALSE
FROM sandbox.games g WHERE g.game_id = '100001'
ON CONFLICT DO NOTHING;

-- ───────────────────────── sandbox/sync/secret：含密文字段（验 S8 preview masked）
-- 说明：当前 loader 尚未全覆盖 login/iap/plugin 子表（见 module open_issues）；
-- 待 loader 补齐后，此处密文字段应以 masked=true、值 'masked' 出现在 preview 差异中。
-- 明文/密文列结构随 channel-login 模块（configJson.clientSecret / *_ciphertext）。
