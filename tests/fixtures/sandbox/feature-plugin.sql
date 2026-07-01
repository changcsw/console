-- fixtures · 模块 feature-plugin（业务表样本 / sandbox schema）
-- 用于 tests/backend/scenarios/feature-plugin.yaml 中 fixture: sandbox/feature-plugin/* 引用的连库 harness。
-- 业务表（game_channel_plugin_configs / channel_package_plugin_overrides）位于当前 env schema：
--   仓储 SQL 不写 schema 前缀，由 search_path=<env>,platform 决定；本文件灌入 sandbox schema。
-- 前置依赖（连库 harness 负责按 search_path=sandbox 执行 + 先灌 common/feature-plugin.sql 平台目录）：
--   - game_channels 中存在 id=1 的渠道实例（huawei_cn / market=CN / 未隐藏），由 sandbox/game.sql 等前置 seed。
--   - channel_packages 中存在 id=1 的渠道包（归属 game_channel id=1），由渠道包前置 seed。
--   -密文 appSecret 应为「加密后值」（连库 harness 用真实 cipher 预置）；此处占位 enc::supersecret 表示密文位。
-- 说明：当前 scripts/regression/db.sh 仅自动灌 common/*.sql；sandbox/*.sql 为连库 harness 的前向声明，
--       待 SCENARIO_WITH_DB=1 连库 harness 落地后按 env schema 灌入。幂等可重复。
SET search_path TO sandbox, platform;

-- ───────────────────────── 1) 已配置渠道实例插件（realname：valid + 含密文，验 S1 运行态派生 / S8 脱敏）
INSERT INTO game_channel_plugin_configs (
  game_channel_id_ref, plugin_id_ref, enabled, config_json, config_status, last_check_message
)
SELECT 1, fp.id, TRUE,
  '{"appId":"app-existing","appSecret":"enc::supersecret"}'::jsonb,
  'valid', ''
FROM platform.feature_plugins fp
WHERE fp.plugin_id = 'realname'
ON CONFLICT (game_channel_id_ref, plugin_id_ref) DO NOTHING;

-- ───────────────────────── 2) 渠道包覆盖（realname：inherit=TRUE，验 inherit 运行态派生）
INSERT INTO channel_package_plugin_overrides (
  package_id_ref, plugin_id_ref, inherit_channel_config, enabled, config_json, config_status, last_check_message
)
SELECT 1, fp.id, TRUE, FALSE,
  '{}'::jsonb, 'empty', ''
FROM platform.feature_plugins fp
WHERE fp.plugin_id = 'realname'
ON CONFLICT (package_id_ref, plugin_id_ref) DO NOTHING;
