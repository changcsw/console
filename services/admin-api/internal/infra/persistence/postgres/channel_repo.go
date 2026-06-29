package postgres

import (
	"context"

	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// ChannelRepo 平台级渠道主数据/策略读仓储（platform.channels / platform.channel_policies）。
// SQL 不写 schema 前缀，靠连接的 search_path（含 platform）。
type ChannelRepo struct{ db DBTX }

const channelSelect = `
SELECT c.id, c.channel_id, c.channel_name, c.channel_type, c.region, c.enabled, c.sort,
       COALESCE(p.login_mode, 'account_system'), COALESCE(p.payment_mode, 'hybrid'),
       COALESCE(p.login_locked, FALSE), COALESCE(p.payment_locked, FALSE)
FROM channels c
LEFT JOIN channel_policies p ON p.channel_id_ref = c.id`

// ListChannelsWithPolicy 列出全部渠道主数据 + 策略（按 sort 升序）。
func (r *ChannelRepo) ListChannelsWithPolicy(ctx context.Context) ([]domainchannel.ChannelWithPolicy, error) {
	rows, err := r.db.Query(ctx, channelSelect+` ORDER BY c.sort ASC, c.id ASC`)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainchannel.ChannelWithPolicy{}
	for rows.Next() {
		cwp, err := scanChannelWithPolicy(rows)
		if err != nil {
			return nil, mapErr(err)
		}
		out = append(out, cwp)
	}
	return out, mapErr(rows.Err())
}

// GetChannelByChannelID 按业务键取单个渠道主数据 + 策略。
func (r *ChannelRepo) GetChannelByChannelID(ctx context.Context, channelID string) (domainchannel.ChannelWithPolicy, error) {
	row := r.db.QueryRow(ctx, channelSelect+` WHERE c.channel_id = $1`, channelID)
	cwp, err := scanChannelWithPolicy(row)
	if err != nil {
		return domainchannel.ChannelWithPolicy{}, mapErr(err)
	}
	return cwp, nil
}

func scanChannelWithPolicy(row interface{ Scan(...any) error }) (domainchannel.ChannelWithPolicy, error) {
	var (
		c                              domainchannel.Channel
		region, loginMode, paymentMode string
		loginLocked, paymentLocked     bool
	)
	if err := row.Scan(
		&c.ID, &c.ChannelID, &c.ChannelName, &c.ChannelType, &region, &c.Enabled, &c.Sort,
		&loginMode, &paymentMode, &loginLocked, &paymentLocked,
	); err != nil {
		return domainchannel.ChannelWithPolicy{}, err
	}
	c.Region = domainchannel.ChannelRegion(region)
	return domainchannel.ChannelWithPolicy{
		Channel: c,
		Policy: domainchannel.ChannelPolicy{
			ChannelIDRef:  c.ID,
			LoginMode:     common.LoginMode(loginMode),
			PaymentMode:   common.PaymentMode(paymentMode),
			LoginLocked:   loginLocked,
			PaymentLocked: paymentLocked,
		},
	}, nil
}
