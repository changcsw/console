-- fixtures · 模块 snapshot（sandbox schema，游戏维度业务数据样本）
-- 用于 tests/backend/scenarios/snapshot.yaml 中 fixture: sandbox/snapshot/* 引用的连库 harness。
-- 单表 game_config_snapshots 每环境 schema 各一份、不带 env 列（D1）；本文件灌入 sandbox schema。
-- 幂等：ON CONFLICT DO NOTHING，可重复灌入。config_json 内 secret 位恒为占位 '***'（I6，绝不落明文）。
--
-- 前置依赖（连库 harness 须先灌入）：
--   sandbox/game.sql             → sandbox.games('100001')  ← 快照 game_id_ref 来源
--   migrations/000015            → sandbox.game_config_snapshots 建表/uq/索引
--   （generate_* 用例还需上游有效数据：channel/account-auth/channel-login/feature-plugin/
--     product/game-cashier/payment 的 sandbox fixture 组合，方能拉出可合并的有效实例）
--
-- 引用约定（manifest fixture: 名 → 本文件片段）：
--   sandbox/snapshot/base       → RBAC + 游戏 100001 + 一份 draft 快照(id=12)（列表/下载/schema 隔离）
--   sandbox/snapshot/draft      → 同 base，强制 id=12 为 draft（验 publish 成功 draft→published）
--   sandbox/snapshot/published  → 追加 id=13 为 published（验 publish 非 draft → VERSION_STATE_INVALID）
--   sandbox/snapshot/secret     → id=12 的 config_json 含 masked secret 位（验 download 脱敏 I6/S8）
--
-- auth.role → RBAC 实体（platform schema）：
--   snapshot_admin → 角色含 snapshot.generate + snapshot.publish + game.read
--   game_reader    → 复用 sandbox/game.sql 的 game_reader（仅 game.read）
--   no_perm        → 复用 common/auth.sql 的 no_perm（无任何权限）
-- 权限码 snapshot.generate / snapshot.publish 若未由 migration seed，则由本文件补齐。

SET search_path TO sandbox, platform;

-- ───────────────────────── RBAC：snapshot 权限码 + 角色（platform schema）
INSERT INTO platform.admin_permissions (permission_code, permission_name)
VALUES ('snapshot.generate', '配置快照-生成'), ('snapshot.publish', '配置快照-发布')
ON CONFLICT (permission_code) DO NOTHING;

INSERT INTO platform.admin_roles (role_code, role_name)
VALUES ('snapshot_admin', '配置快照管理员')
ON CONFLICT (role_code) DO NOTHING;

-- snapshot_admin ← snapshot.generate + snapshot.publish + game.read
INSERT INTO platform.admin_role_permissions (role_id_ref, permission_id_ref)
SELECT r.id, p.id
FROM platform.admin_roles r
JOIN platform.admin_permissions p
  ON p.permission_code IN ('snapshot.generate', 'snapshot.publish', 'game.read')
WHERE r.role_code = 'snapshot_admin'
ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING;

-- ───────────────────────── 片段：base / draft —— 游戏 100001 一份 draft 快照(id=12)
-- config_json 结构对齐 spec 样例（按 market 分区）；secret 位恒 '***'。
INSERT INTO game_config_snapshots
  (id, game_id_ref, config_schema_version, config_version, config_json,
   file_name, file_hash, storage_key, status, generated_at, published_at)
SELECT
  12,
  g.id,
  '1.0',
  '20260615100000-a1b2c3d4',
  '{"schemaVersion":"1.0","gameId":"100001","generatedAt":"2026-06-15T10:00:00Z",'
  || '"markets":{"GLOBAL":{"game":{"legalLinks":[],"accountAuth":[],"products":[]},'
  || '"channels":[{"channelId":"google","region":"overseas","sourceMarket":"GLOBAL",'
  || '"login":{"clientId":"pub-client","apiKey":"***"}}],"paymentRoutes":[]},'
  || '"CN":{"game":{"legalLinks":[],"accountAuth":[],"products":[]},'
  || '"channels":[{"channelId":"huawei_cn","region":"domestic","sourceMarket":"CN"}],'
  || '"paymentRoutes":[]}}}',
  'game_100001_20260615100000-a1b2c3d4.json',
  'a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2',
  '',
  'draft',
  TIMESTAMPTZ '2026-06-15 10:00:00+00',
  NULL
FROM games g
WHERE g.game_id = '100001'
ON CONFLICT (id) DO NOTHING;

-- ───────────────────────── 片段：published —— 追加 id=13 已发布快照（验非 draft 发布冲突）
INSERT INTO game_config_snapshots
  (id, game_id_ref, config_schema_version, config_version, config_json,
   file_name, file_hash, storage_key, status, generated_at, published_at)
SELECT
  13,
  g.id,
  '1.0',
  '20260614090000-9f8e7d6c',
  '{"schemaVersion":"1.0","gameId":"100001","generatedAt":"2026-06-14T09:00:00Z","markets":{}}',
  'game_100001_20260614090000-9f8e7d6c.json',
  '9f8e7d6c5b4a39281706f5e4d3c2b1a0f9e8d7c6b5a493827160f5e4d3c2b1a0',
  '',
  'published',
  TIMESTAMPTZ '2026-06-14 09:00:00+00',
  TIMESTAMPTZ '2026-06-14 09:05:00+00'
FROM games g
WHERE g.game_id = '100001'
ON CONFLICT (id) DO NOTHING;

-- ───────────────────────── 片段：secret —— 显式确保 id=12 config_json 含 masked secret 位
-- （base 已内置 login.apiKey='***'；此处幂等确认，供 download_masks_secret/S8 断言引用）
UPDATE game_config_snapshots
SET config_json = jsonb_set(
      config_json,
      '{markets,GLOBAL,channels,0,login,apiKey}',
      '"***"'::jsonb,
      true
    )
WHERE id = 12;

-- 序列对齐（显式 id 插入后推进 BIGSERIAL，避免后续 INSERT 主键冲突）
SELECT setval(
  pg_get_serial_sequence('game_config_snapshots', 'id'),
  GREATEST((SELECT COALESCE(MAX(id), 1) FROM game_config_snapshots), 13)
);
