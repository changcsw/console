package command

import "github.com/csw/console/services/admin-api/internal/domain/channel"

type HideMarketChannelCommand struct {
	Operator string
}

func ApplyHideMarketChannel(cmd HideMarketChannelCommand, item *channel.GameMarketChannel) {
	if item == nil {
		return
	}

	item.Hide(cmd.Operator)
}
