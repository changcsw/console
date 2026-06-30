-- fixtures · 模块 product（sandbox schema，业务数据样本）
-- 用于 tests/backend/scenarios/product.yaml 中 fixture: sandbox/product/* 引用的连库 harness。
-- 业务四表（products / channel_products / game_channel_iap_configs / channel_package_iap_overrides）
-- 每环境 schema 各一份、不带 env 列；本文件灌入 sandbox schema（03-testing §7）。
-- 幂等：ON CONFLICT DO NOTHING，可重复灌入。
-- 依赖：sandbox/game.sql 的 sandbox/game/base（游戏 100001）+ 渠道 google（migration 000001 seed）
--       + common/product.sql 的 google v1 IAP 模板。channel 模块未提供 fixture 时，本文件就近补 game_channel/package。
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   sandbox/product/base           → 游戏 100001 下两个商品 com.demo.coin / com.demo.gem（验列表/分页/创建冲突/更新）
--   sandbox/product/mapping        → 包 7001(pkg-a) × 100001 商品的 channel_products 映射（验包级读写 + effective）
--   sandbox/product/iap            → game_channel 9001 + 包 7001 + google IAP 模板，配置实例为空（验读模板 / 写配置）
--   sandbox/product/iap_configured → 9001 渠道 IAP 已配置并启用（含密文位），验 S8 脱敏 + 密文更新回归
--
-- 关键假设：price_id_override / price_id 与价格行不做强外键校验（仅格式/长度校验）。
-- product_id(≤128) 与 price_id(≤64) 两维独立、禁止互填。base_amount_minor 必须按 currency_specs 归一化后写入。

SET search_path TO sandbox, platform;

-- ───────────────────────── 前置：渠道实例 / 渠道包（channel 模块未提供 fixture 时就近补齐）
-- game_channel 9001 = 游戏 100001 × 渠道 google（启用）
INSERT INTO sandbox.game_channels (id, game_id_ref, channel_id_ref, enabled, remark)
SELECT 9001, g.id, ch.id, TRUE, 'product fixtures'
FROM sandbox.games g JOIN platform.channels ch ON ch.channel_id = 'google'
WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, channel_id_ref) DO NOTHING;

-- channel_package 7001 = game_channel 9001 下的包 pkg-a（GLOBAL market）
INSERT INTO sandbox.channel_packages (id, game_channel_id_ref, package_code, package_name, market_code, bundle_id, enabled)
VALUES (7001, 9001, 'pkg-a', 'Package A', 'GLOBAL', 'com.demo.bundle', TRUE)
ON CONFLICT (game_channel_id_ref, package_code) DO NOTHING;

-- ───────────────────────── sandbox/product/base：游戏 100001 的两个商品
-- com.demo.coin：USD 5.00 = 500 minor（half_up 归一化结果）
INSERT INTO sandbox.products (game_id_ref, product_id, product_name, base_amount_minor, base_currency, price_id, enabled)
SELECT g.id, 'com.demo.coin', 'Coin Pack', 500, 'USD', 'price_coin', TRUE
FROM sandbox.games g WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, product_id) DO NOTHING;

-- com.demo.gem：JPY 120 = 120 minor（decimal=0）
INSERT INTO sandbox.products (game_id_ref, product_id, product_name, base_amount_minor, base_currency, price_id, enabled)
SELECT g.id, 'com.demo.gem', 'Gem Pack', 120, 'JPY', 'price_gem', TRUE
FROM sandbox.games g WHERE g.game_id = '100001'
ON CONFLICT (game_id_ref, product_id) DO NOTHING;

-- ───────────────────────── sandbox/product/mapping：包 7001 × 商品的 channel_products 映射
-- coin：product_id 覆盖（store.coin.id），price_id 回退基准（两维独立解析）
INSERT INTO sandbox.channel_products
  (product_id_ref, package_id_ref, product_id_mode, product_id_override, price_id_mode, price_id_override, enabled)
SELECT p.id, 7001, 'override', 'store.coin.id', 'default', '', TRUE
FROM sandbox.products p
JOIN sandbox.games g ON g.id = p.game_id_ref
WHERE g.game_id = '100001' AND p.product_id = 'com.demo.coin'
ON CONFLICT (package_id_ref, product_id_ref) DO NOTHING;

-- gem：两维均 default（全回退基准）
INSERT INTO sandbox.channel_products
  (product_id_ref, package_id_ref, product_id_mode, product_id_override, price_id_mode, price_id_override, enabled)
SELECT p.id, 7001, 'default', '', 'default', '', TRUE
FROM sandbox.products p
JOIN sandbox.games g ON g.id = p.game_id_ref
WHERE g.game_id = '100001' AND p.product_id = 'com.demo.gem'
ON CONFLICT (package_id_ref, product_id_ref) DO NOTHING;

-- ───────────────────────── sandbox/product/iap：配置实例占位（空配置 = empty 默认）
-- 无 INSERT：读接口在无行时回退 enabled=false / config_status=empty / config_json={}（服务端默认视图）。
-- 写接口（PUT iap-config / iap-override）首次写入即 upsert。

-- ───────────────────────── sandbox/product/iap_configured：9001 渠道 IAP 已配置（含密文位）
-- config_json.privateKey 存「密文位」（base64 占位样本，绝非明文）；读接口恒返回 masked，
-- 密文更新回归断言此值在 masked/留空/仅 toggle 提交时保持不变。
INSERT INTO sandbox.game_channel_iap_configs
  (game_channel_id_ref, enabled, config_json, config_status, last_check_message)
VALUES (9001, TRUE,
  jsonb_build_object(
    'appId', 'app-123',
    'privateKey', 'ZW5jOnByaXZhdGUta2V5LWNpcGhlcnRleHQtc2FtcGxl'
  ),
  'valid', '')
ON CONFLICT (game_channel_id_ref) DO NOTHING;
