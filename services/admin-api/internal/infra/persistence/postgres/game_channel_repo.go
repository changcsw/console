package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/csw/console/services/admin-api/internal/app/channel"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// GameChannelRepo 渠道实例仓储（game_channels，业务表，SQL 不带 schema 前缀，靠 search_path）。
// region 通过 JOIN platform.channels 实时取出（不落 game_channels）。
type GameChannelRepo struct{ db DBTX }

// gameChannelSelect 实例聚合装配列（JOIN games 取 game_id、JOIN channels 取 channel_id/region）。
const gameChannelSelect = `
SELECT gc.id, gc.game_id_ref, g.game_id, gc.market_code, gc.channel_id_ref, c.channel_id, c.region,
       gc.enabled, gc.hidden, gc.hidden_by, gc.hidden_at, gc.config_status,
       gc.last_check_at, gc.last_check_message, gc.copied_from_market, gc.remark,
       gc.created_at, gc.updated_at
FROM game_channels gc
JOIN games g ON g.id = gc.game_id_ref
JOIN channels c ON c.id = gc.channel_id_ref`

// ResolveGameRowID 把对外 game_id 解析为 games.id。
func (r *GameChannelRepo) ResolveGameRowID(ctx context.Context, gameID string) (int64, error) {
	var id int64
	if err := r.db.QueryRow(ctx, `SELECT id FROM games WHERE game_id = $1`, gameID).Scan(&id); err != nil {
		return 0, mapErr(err)
	}
	return id, nil
}

// ExistsInstance 判断 (gameIDRef, market, channelID) 实例是否已存在。
func (r *GameChannelRepo) ExistsInstance(ctx context.Context, gameIDRef int64, market, channelID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM game_channels gc JOIN channels c ON c.id = gc.channel_id_ref
		   WHERE gc.game_id_ref = $1 AND gc.market_code = $2 AND c.channel_id = $3)`,
		gameIDRef, market, channelID,
	).Scan(&exists)
	return exists, mapErr(err)
}

// FindInstance 取某 (gameIDRef, market, channelID) 实例（复制来源校验）。
func (r *GameChannelRepo) FindInstance(ctx context.Context, gameIDRef int64, market, channelID string) (domainchannel.GameMarketChannel, error) {
	row := r.db.QueryRow(ctx, gameChannelSelect+
		` WHERE gc.game_id_ref = $1 AND gc.market_code = $2 AND c.channel_id = $3`,
		gameIDRef, market, channelID)
	inst, err := scanGameMarketChannel(row)
	if err != nil {
		return domainchannel.GameMarketChannel{}, mapErr(err)
	}
	return inst, nil
}

// Insert 落库渠道实例，返回装配后的聚合（含 id/时间戳）。inst 须已带 GameIDRef/ChannelIDRef/GameID/ChannelID/Region。
func (r *GameChannelRepo) Insert(ctx context.Context, inst domainchannel.GameMarketChannel) (domainchannel.GameMarketChannel, error) {
	err := r.db.QueryRow(ctx,
		`INSERT INTO game_channels
		   (game_id_ref, channel_id_ref, market_code, enabled, remark, hidden, hidden_by,
		    config_status, last_check_message, copied_from_market)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, created_at, updated_at`,
		inst.GameIDRef, inst.ChannelIDRef, inst.Market, inst.Enabled, inst.Remark, inst.Hidden, inst.HiddenBy,
		string(inst.ConfigStatus), inst.LastCheckMessage, inst.CopiedFromMarket,
	).Scan(&inst.ID, &inst.CreatedAt, &inst.UpdatedAt)
	if err != nil {
		return domainchannel.GameMarketChannel{}, mapErr(err)
	}
	return inst, nil
}

// GetByID 按 game_channels.id 取实例聚合。
func (r *GameChannelRepo) GetByID(ctx context.Context, id int64) (domainchannel.GameMarketChannel, error) {
	row := r.db.QueryRow(ctx, gameChannelSelect+` WHERE gc.id = $1`, id)
	inst, err := scanGameMarketChannel(row)
	if err != nil {
		return domainchannel.GameMarketChannel{}, mapErr(err)
	}
	return inst, nil
}

// List 分页/过滤实例列表。compatible 在 SQL 内按 (market=='CN') == (region=='domestic') 派生过滤。
func (r *GameChannelRepo) List(ctx context.Context, q dto.ListMarketChannelsQuery) ([]domainchannel.GameMarketChannel, int, error) {
	where := []string{"g.game_id = $1"}
	args := []any{q.GameID}
	idx := 2
	if q.Market != "" && !strings.EqualFold(q.Market, "ALL") {
		where = append(where, fmt.Sprintf("gc.market_code = $%d", idx))
		args = append(args, q.Market)
		idx++
	}
	if q.ChannelID != "" {
		where = append(where, fmt.Sprintf("c.channel_id = $%d", idx))
		args = append(args, q.ChannelID)
		idx++
	}
	if q.ConfigStatus != "" {
		where = append(where, fmt.Sprintf("gc.config_status = $%d", idx))
		args = append(args, q.ConfigStatus)
		idx++
	}
	if !q.Hidden {
		where = append(where, "gc.hidden = FALSE")
	}
	if q.Compatible != nil {
		// compatible 派生：market=CN ⇔ region=domestic。
		if *q.Compatible {
			where = append(where, "((gc.market_code = 'CN') = (c.region = 'domestic'))")
		} else {
			where = append(where, "((gc.market_code = 'CN') <> (c.region = 'domestic'))")
		}
	}
	cond := strings.Join(where, " AND ")

	var total int
	countSQL := `SELECT COUNT(*) FROM game_channels gc
	             JOIN games g ON g.id = gc.game_id_ref
	             JOIN channels c ON c.id = gc.channel_id_ref WHERE ` + cond
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}

	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)
	listSQL := gameChannelSelect + " WHERE " + cond +
		fmt.Sprintf(" ORDER BY gc.updated_at DESC, gc.id DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	rows, err := r.db.Query(ctx, listSQL, args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	items := []domainchannel.GameMarketChannel{}
	for rows.Next() {
		inst, err := scanGameMarketChannel(rows)
		if err != nil {
			return nil, 0, mapErr(err)
		}
		items = append(items, inst)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, mapErr(err)
	}
	return items, total, nil
}

// UpdateBasics 更新 enabled/remark（nil 不改）。
func (r *GameChannelRepo) UpdateBasics(ctx context.Context, id int64, patch channel.GameChannelPatch) error {
	sets := []string{}
	args := []any{}
	idx := 1
	if patch.Enabled != nil {
		sets = append(sets, fmt.Sprintf("enabled = $%d", idx))
		args = append(args, *patch.Enabled)
		idx++
	}
	if patch.Remark != nil {
		sets = append(sets, fmt.Sprintf("remark = $%d", idx))
		args = append(args, *patch.Remark)
		idx++
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, id)
	tag, err := r.db.Exec(ctx,
		fmt.Sprintf("UPDATE game_channels SET %s WHERE id = $%d", strings.Join(sets, ", "), idx), args...)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(errNoRows())
	}
	return nil
}

// Hide 置 hidden=true 并记录操作人/时间。
func (r *GameChannelRepo) Hide(ctx context.Context, id int64, by string, at time.Time) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE game_channels SET hidden = TRUE, hidden_by = $2, hidden_at = $3, updated_at = NOW() WHERE id = $1`,
		id, by, at)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(errNoRows())
	}
	return nil
}

// Unhide 置 hidden=false 并清隐藏操作人/时间。
func (r *GameChannelRepo) Unhide(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE game_channels SET hidden = FALSE, hidden_by = '', hidden_at = NULL, updated_at = NOW() WHERE id = $1`,
		id)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(errNoRows())
	}
	return nil
}

func scanGameMarketChannel(row interface{ Scan(...any) error }) (domainchannel.GameMarketChannel, error) {
	var (
		inst         domainchannel.GameMarketChannel
		region       string
		configStatus string
	)
	if err := row.Scan(
		&inst.ID, &inst.GameIDRef, &inst.GameID, &inst.Market, &inst.ChannelIDRef, &inst.ChannelID, &region,
		&inst.Enabled, &inst.Hidden, &inst.HiddenBy, &inst.HiddenAt, &configStatus,
		&inst.LastCheckAt, &inst.LastCheckMessage, &inst.CopiedFromMarket, &inst.Remark,
		&inst.CreatedAt, &inst.UpdatedAt,
	); err != nil {
		return domainchannel.GameMarketChannel{}, err
	}
	inst.Region = domainchannel.ChannelRegion(region)
	inst.ConfigStatus = common.ConfigStatus(configStatus)
	return inst, nil
}
