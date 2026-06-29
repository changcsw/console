-- 000004 down · 撤销 game 约束/索引补齐（幂等，best-effort）

DROP INDEX IF EXISTS idx_game_legal_links_game_id_ref;
DROP INDEX IF EXISTS idx_game_markets_game_id_ref;

ALTER TABLE IF EXISTS game_markets DROP CONSTRAINT IF EXISTS game_markets_market_code_check;
ALTER TABLE IF EXISTS games        DROP CONSTRAINT IF EXISTS games_alias_key;
ALTER TABLE IF EXISTS games        DROP CONSTRAINT IF EXISTS games_status_check;
