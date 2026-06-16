package command

import "github.com/csw/console/services/admin-api/internal/domain/channel"

type CreateMarketChannelCommand struct {
	GameID    string
	Market    string
	ChannelID string
	Source    channel.GameMarketChannel
}

func BuildCreatedMarketChannel(cmd CreateMarketChannelCommand) channel.GameMarketChannel {
	return channel.NewCopiedMarketChannel(cmd.GameID, cmd.Market, cmd.ChannelID, cmd.Source)
}
