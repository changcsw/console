package command

import "github.com/csw/console/services/admin-api/internal/domain/channel"

type UnhideMarketChannelCommand struct {
	ID string
}

func ApplyUnhideMarketChannel(item *channel.GameMarketChannel) {
	if item == nil {
		return
	}

	item.Unhide()
}
