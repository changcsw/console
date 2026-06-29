-- 000004 · game 模块：业务表约束/索引补齐（D1 game compact）
-- 范围：仅 game 维度业务表 games / game_markets / game_legal_links（00 §2.2，不带 env 列）。
-- 这些表在每环境 schema 各一份；本迁移不写 schema 前缀，靠迁移连接的 search_path 路由（01 §4.2）。
-- 追加新文件、不改历史；全部 IF EXISTS / IF NOT EXISTS / 存在性判断，可重复前向执行。
-- 000001 已建三表，但缺：games.alias 唯一、games.status CHECK、game_markets.market_code CHECK、FK 索引。

-- 约束名在 PostgreSQL 中按 namespace（schema）隔离；schema-per-env 下每环境 schema 各有一份 games/game_markets。
-- 存在性判断必须按当前 schema（search_path 首项）限定 connamespace，否则首个 schema 建好后其余 schema 会被误跳过。

-- 1) games.status CHECK(draft/active/disabled)
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'games_status_check' AND connamespace = current_schema()::regnamespace
  ) THEN
    ALTER TABLE games
      ADD CONSTRAINT games_status_check CHECK (status IN ('draft', 'active', 'disabled'));
  END IF;
END $$;

-- 2) games.alias schema 内唯一（compact §5.4；原 DDL 仅 UNIQUE(game_id)）
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'games_alias_key' AND connamespace = current_schema()::regnamespace
  ) THEN
    ALTER TABLE games
      ADD CONSTRAINT games_alias_key UNIQUE (alias);
  END IF;
END $$;

-- 3) game_markets.market_code CHECK(∈ Market 枚举)
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'game_markets_market_code_check' AND connamespace = current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_markets
      ADD CONSTRAINT game_markets_market_code_check
      CHECK (market_code IN ('GLOBAL', 'JP', 'KR', 'SEA', 'HMT', 'CN'));
  END IF;
END $$;

-- 4) FK / 聚合装配热路径索引
CREATE INDEX IF NOT EXISTS idx_game_markets_game_id_ref     ON game_markets(game_id_ref);
CREATE INDEX IF NOT EXISTS idx_game_legal_links_game_id_ref ON game_legal_links(game_id_ref);
