-- fixtures · 模块 sync（#21）— production schema「同步目标基线」样本
-- 用于 tests/backend/scenarios/sync.yaml 中 fixture: production/sync/* 引用的连库 harness。
-- production/ = production schema（03-testing §7：同步目标基线，用于 diff/baseline/nonce 测试）。
-- 幂等：ON CONFLICT DO NOTHING，可重复灌入。
--
-- 前置依赖（连库 harness 须先灌入）：
--   production 三环境 schema 已建 + migrations 全量（含 000016 platform.sync_jobs/items/consumed_tokens）
--   sandbox/sync.sql（源侧差异样本）配套灌入
--
-- 引用约定（manifest fixture: 名 → 片段）：
--   production/sync/base     → 游戏 100001 + 基线数据（sandbox 存在 add/update 差异，供 execute 成功）
--   production/sync/drifted  → 在 base 基础上改动 production（模拟预览后被他人改动 → S5 SYNC_BASELINE_MISMATCH）
--   production/sync/secret   → 含密文列基线（验 execute 密文重加密不经明文）
--   production/sync/empty    → production 无任何 section 数据（验依赖缺失 VALIDATION_FAILED）
--   production/sync/jobs     → platform.sync_jobs 历史行（previewed/succeeded/failed，验 sync-jobs 列表/分页/过滤）

SET search_path TO production, platform;

-- ───────────────────────── production/sync/base：游戏 100001 目标基线
INSERT INTO production.games (game_id, game_secret, name, alias, icon_url, default_market_code, status)
VALUES ('100001', 'prd0000000000000000000000000000000000000000000000000000000000base', 'Demo Game', 'demo-game', '', 'GLOBAL', 'active')
ON CONFLICT (game_id) DO NOTHING;

INSERT INTO production.game_markets (game_id_ref, market_code, is_default, enabled, default_locale)
SELECT g.id, 'GLOBAL', TRUE, TRUE, 'en-US' FROM production.games g WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, market_code) DO NOTHING;
INSERT INTO production.game_markets (game_id_ref, market_code, is_default, enabled, default_locale)
SELECT g.id, 'JP', FALSE, TRUE, 'ja-JP' FROM production.games g WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, market_code) DO NOTHING;

-- 商品 gem_1：production 旧名（sandbox 为 'Gems Pack A (v2)' → products.update 差异）
INSERT INTO production.products (game_id_ref, product_id, product_name, base_amount_minor, base_currency, price_id, enabled)
SELECT g.id, 'gem_1', 'Gems Pack A', 100, 'USD', 'price_gem_1', TRUE
FROM production.games g WHERE g.game_id = '100001'
ON CONFLICT DO NOTHING;

-- 注：production 无 JP/google 渠道实例 → 相对 sandbox 触发 channels.add（execute 需先/同批选 game+markets 满足依赖）。

-- ───────────────────────── production/sync/jobs：sync-jobs 历史（platform schema，无 env 列）
-- source_env/target_env 显式表达环境维度；covers 列表/分页/status 过滤/schema 隔离。
INSERT INTO platform.sync_jobs
  (id, game_id_ref, source_env, target_env, source_hash, target_hash_before, target_hash_after,
   include_deletes, operator_id, operator_note, status, executed_at)
VALUES
  (7001, '100001', 'sandbox', 'production', 'src-h-1', 'tgt-h-1', '', FALSE, 1, 'preview only', 'previewed', NULL),
  (7002, '100001', 'sandbox', 'production', 'src-h-2', 'tgt-h-2', 'tgt-h-2b', FALSE, 1, 'apply channels', 'succeeded', TIMESTAMPTZ '2026-06-30 08:00:00+00'),
  (7003, '100001', 'sandbox', 'production', 'src-h-3', 'tgt-h-3', '', TRUE, 1, 'failed run', 'failed', TIMESTAMPTZ '2026-06-30 09:00:00+00')
ON CONFLICT (id) DO NOTHING;

INSERT INTO platform.sync_job_items
  (sync_job_id_ref, section, entity_type, entity_key, op, field_name, sandbox_value_json, production_value_json, masked, applied)
VALUES
  (7002, 'channels', 'game_channel', 'JP/google', 'add', '*', '{"value":{"enabled":true}}', '{}', FALSE, TRUE),
  (7002, 'products', 'product', 'gem_1', 'update', 'product_name', '{"value":"Gems Pack A (v2)"}', '{"value":"Gems Pack A"}', FALSE, TRUE)
ON CONFLICT DO NOTHING;

-- 序列对齐（显式 id 插入后推进 BIGSERIAL）
SELECT setval(
  pg_get_serial_sequence('platform.sync_jobs', 'id'),
  GREATEST((SELECT COALESCE(MAX(id), 1) FROM platform.sync_jobs), 7003)
);

-- ───────────────────────── production/sync/drifted：预览后被改动（S5 基线复核失败）
-- 语义：preview 生成 token 后，此片段改动 production 有效数据（如商品改名/新增），
-- 使实时 target_hash_now ≠ token.target_hash_before → SYNC_BASELINE_MISMATCH。
UPDATE production.products SET product_name = 'Gems Pack A (drifted)'
WHERE product_id = 'gem_1'
  AND game_id_ref = (SELECT id FROM production.games WHERE game_id = '100001');

-- ───────────────────────── production/sync/empty
-- 语义：清空/不灌 production 目标数据，仅保留 games 行不足以满足 channels 依赖（缺 markets 时）；
-- 用于 execute_dependency_missing：选 channels 而 production 无 game/markets → VALIDATION_FAILED。
-- 连库 harness 用独立 game_id（如 200002，仅建空 games 行）承载该场景，避免污染 base。

-- ───────────────────────── production/sync/secret
-- 含密文列基线（结构随 channel-login 模块）；验 execute 密文取密文/重加密，绝不经明文中转。
