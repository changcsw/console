package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	gameapp "github.com/csw/console/services/admin-api/internal/app/game"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domaingame "github.com/csw/console/services/admin-api/internal/domain/game"
)

// GameRepo games / game_markets / game_legal_links 仓储（业务表，SQL 不带 schema 前缀，靠 search_path）。
type GameRepo struct{ db DBTX }

// NextGameIDSeq 返回当前 schema 内 game_id 的最大数字序号（空表/无数字串返回 0）。
func (r *GameRepo) NextGameIDSeq(ctx context.Context) (int64, error) {
	var seq int64
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(game_id::bigint), 0) FROM games WHERE game_id ~ '^[0-9]+$'`,
	).Scan(&seq)
	return seq, mapErr(err)
}

// ExistsAlias 判断 alias 是否已存在；excludeGameID 非空时排除该游戏。
func (r *GameRepo) ExistsAlias(ctx context.Context, alias, excludeGameID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM games WHERE alias=$1 AND ($2='' OR game_id<>$2))`,
		alias, excludeGameID,
	).Scan(&exists)
	return exists, mapErr(err)
}

// InsertGame 插入 games + 市场集合，返回装配后的聚合（含 ID/时间戳）。
func (r *GameRepo) InsertGame(ctx context.Context, g domaingame.Game) (domaingame.Game, error) {
	err := r.db.QueryRow(ctx,
		`INSERT INTO games (game_id, game_secret, name, alias, icon_url, default_market_code, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)
		 RETURNING id, created_at, updated_at`,
		g.GameID, g.GameSecret, g.Name, g.Alias, g.IconURL, g.DefaultMarketCode, string(g.Status),
	).Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return domaingame.Game{}, mapErr(err)
	}
	if err := r.insertMarkets(ctx, g.ID, g.Markets); err != nil {
		return domaingame.Game{}, err
	}
	g.LegalLinks = []domaingame.GameLegalLink{}
	return g, nil
}

func (r *GameRepo) insertMarkets(ctx context.Context, gameRowID int64, markets []domaingame.GameMarket) error {
	for _, m := range markets {
		if _, err := r.db.Exec(ctx,
			`INSERT INTO game_markets (game_id_ref, market_code, is_default, enabled, default_locale)
			 VALUES ($1,$2,$3,$4,$5)`,
			gameRowID, m.MarketCode, m.IsDefault, m.Enabled, m.DefaultLocale,
		); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// GetGameByGameID 按对外 game_id 装配完整聚合。
func (r *GameRepo) GetGameByGameID(ctx context.Context, gameID string) (domaingame.Game, error) {
	var g domaingame.Game
	var status string
	err := r.db.QueryRow(ctx,
		`SELECT id, game_id, game_secret, name, alias, icon_url, default_market_code, status, created_at, updated_at
		 FROM games WHERE game_id=$1`, gameID,
	).Scan(&g.ID, &g.GameID, &g.GameSecret, &g.Name, &g.Alias, &g.IconURL, &g.DefaultMarketCode, &status, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return domaingame.Game{}, mapErr(err)
	}
	g.Status = common.GameStatus(status)

	markets, err := r.marketsByGameRowID(ctx, g.ID)
	if err != nil {
		return domaingame.Game{}, err
	}
	g.Markets = markets

	links, err := r.legalLinksByGameRowID(ctx, g.ID)
	if err != nil {
		return domaingame.Game{}, err
	}
	g.LegalLinks = links
	return g, nil
}

func (r *GameRepo) marketsByGameRowID(ctx context.Context, gameRowID int64) ([]domaingame.GameMarket, error) {
	rows, err := r.db.Query(ctx,
		`SELECT market_code, is_default, enabled, default_locale
		 FROM game_markets WHERE game_id_ref=$1 ORDER BY market_code`, gameRowID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	markets := []domaingame.GameMarket{}
	for rows.Next() {
		var m domaingame.GameMarket
		if err := rows.Scan(&m.MarketCode, &m.IsDefault, &m.Enabled, &m.DefaultLocale); err != nil {
			return nil, mapErr(err)
		}
		markets = append(markets, m)
	}
	return markets, mapErr(rows.Err())
}

func (r *GameRepo) legalLinksByGameRowID(ctx context.Context, gameRowID int64) ([]domaingame.GameLegalLink, error) {
	rows, err := r.db.Query(ctx,
		`SELECT scope_type, scope_value, terms_url, privacy_url, delete_account_url
		 FROM game_legal_links WHERE game_id_ref=$1 ORDER BY scope_type, scope_value`, gameRowID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	links := []domaingame.GameLegalLink{}
	for rows.Next() {
		var l domaingame.GameLegalLink
		if err := rows.Scan(&l.ScopeType, &l.ScopeValue, &l.TermsURL, &l.PrivacyURL, &l.DeleteAccountURL); err != nil {
			return nil, mapErr(err)
		}
		links = append(links, l)
	}
	return links, mapErr(rows.Err())
}

// ListGames 分页列表（每项含市场集合用于摘要）。
func (r *GameRepo) ListGames(ctx context.Context, q dto.ListGamesQuery) ([]domaingame.Game, int, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR alias ILIKE $%d OR game_id ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+kw+"%")
		idx++
	}
	if q.Status != "" {
		where = append(where, fmt.Sprintf("status=$%d", idx))
		args = append(args, q.Status)
		idx++
	}
	if q.MarketCode != "" {
		where = append(where, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM game_markets gm WHERE gm.game_id_ref=games.id AND gm.market_code=$%d AND gm.enabled=TRUE)", idx))
		args = append(args, q.MarketCode)
		idx++
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM games WHERE "+cond, args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}

	page, pageSize := q.Page, q.PageSize
	order := orderBy(q.Sort, map[string]string{"updatedAt": "updated_at", "createdAt": "created_at", "name": "name"}, "updated_at DESC")
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(ctx, fmt.Sprintf(
		`SELECT id, game_id, name, alias, icon_url, default_market_code, status, created_at, updated_at
		 FROM games WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`, cond, order, idx, idx+1), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()

	games := []domaingame.Game{}
	for rows.Next() {
		var g domaingame.Game
		var status string
		if err := rows.Scan(&g.ID, &g.GameID, &g.Name, &g.Alias, &g.IconURL, &g.DefaultMarketCode, &status, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, 0, mapErr(err)
		}
		g.Status = common.GameStatus(status)
		games = append(games, g)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, mapErr(err)
	}

	for i := range games {
		markets, err := r.marketsByGameRowID(ctx, games[i].ID)
		if err != nil {
			return nil, 0, err
		}
		games[i].Markets = markets
	}
	return games, total, nil
}

// UpdateGame 更新 games 列级字段；patch.DefaultMarketCode 非空时同步 game_markets.is_default。
func (r *GameRepo) UpdateGame(ctx context.Context, gameID string, patch gameapp.GamePatch) error {
	sets := []string{}
	args := []any{}
	idx := 1
	if patch.Name != nil {
		sets = append(sets, fmt.Sprintf("name=$%d", idx))
		args = append(args, *patch.Name)
		idx++
	}
	if patch.Alias != nil {
		sets = append(sets, fmt.Sprintf("alias=$%d", idx))
		args = append(args, *patch.Alias)
		idx++
	}
	if patch.IconURL != nil {
		sets = append(sets, fmt.Sprintf("icon_url=$%d", idx))
		args = append(args, *patch.IconURL)
		idx++
	}
	if patch.Status != nil {
		sets = append(sets, fmt.Sprintf("status=$%d", idx))
		args = append(args, *patch.Status)
		idx++
	}
	if patch.DefaultMarketCode != nil {
		sets = append(sets, fmt.Sprintf("default_market_code=$%d", idx))
		args = append(args, *patch.DefaultMarketCode)
		idx++
	}

	if len(sets) > 0 {
		sets = append(sets, "updated_at=NOW()")
		args = append(args, gameID)
		tag, err := r.db.Exec(ctx,
			fmt.Sprintf("UPDATE games SET %s WHERE game_id=$%d", strings.Join(sets, ", "), idx), args...)
		if err != nil {
			return mapErr(err)
		}
		if tag.RowsAffected() == 0 {
			return mapErr(errNoRows())
		}
	}

	if patch.DefaultMarketCode != nil {
		if _, err := r.db.Exec(ctx,
			`UPDATE game_markets SET is_default=(market_code=$2), updated_at=NOW()
			 WHERE game_id_ref=(SELECT id FROM games WHERE game_id=$1)`,
			gameID, *patch.DefaultMarketCode,
		); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// ReplaceMarkets 全量覆盖市场并回写 games.default_market_code（事务内 delete + insert）。
func (r *GameRepo) ReplaceMarkets(ctx context.Context, gameID string, markets []domaingame.GameMarket, defaultMarketCode string) error {
	rowID, err := r.gameRowID(ctx, gameID)
	if err != nil {
		return err
	}
	if _, err := r.db.Exec(ctx, `DELETE FROM game_markets WHERE game_id_ref=$1`, rowID); err != nil {
		return mapErr(err)
	}
	if err := r.insertMarkets(ctx, rowID, markets); err != nil {
		return err
	}
	if _, err := r.db.Exec(ctx,
		`UPDATE games SET default_market_code=$2, updated_at=NOW() WHERE id=$1`, rowID, defaultMarketCode); err != nil {
		return mapErr(err)
	}
	return nil
}

// ReplaceLegalLinks 全量覆盖法务链接（事务内 delete + insert）。
func (r *GameRepo) ReplaceLegalLinks(ctx context.Context, gameID string, links []domaingame.GameLegalLink) error {
	rowID, err := r.gameRowID(ctx, gameID)
	if err != nil {
		return err
	}
	if _, err := r.db.Exec(ctx, `DELETE FROM game_legal_links WHERE game_id_ref=$1`, rowID); err != nil {
		return mapErr(err)
	}
	for _, l := range links {
		if _, err := r.db.Exec(ctx,
			`INSERT INTO game_legal_links (game_id_ref, scope_type, scope_value, terms_url, privacy_url, delete_account_url)
			 VALUES ($1,$2,$3,$4,$5,$6)`,
			rowID, l.ScopeType, l.ScopeValue, l.TermsURL, l.PrivacyURL, l.DeleteAccountURL,
		); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

// CountChannelsByMarket 删除保护用计数：统计某 game 某 market 下的渠道实例数（同 schema，channel 模块落地后回填）。
func (r *GameRepo) CountChannelsByMarket(ctx context.Context, gameID, marketCode string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM game_channels gc
		 JOIN games g ON g.id = gc.game_id_ref
		 WHERE g.game_id = $1 AND gc.market_code = $2`,
		gameID, marketCode,
	).Scan(&n)
	return n, mapErr(err)
}

func (r *GameRepo) gameRowID(ctx context.Context, gameID string) (int64, error) {
	var id int64
	if err := r.db.QueryRow(ctx, `SELECT id FROM games WHERE game_id=$1`, gameID).Scan(&id); err != nil {
		return 0, mapErr(err)
	}
	return id, nil
}
