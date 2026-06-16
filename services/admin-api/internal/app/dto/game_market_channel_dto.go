package dto

import "github.com/csw/console/services/admin-api/internal/domain/common"

type GameMarketChannelListItem struct {
	GameID                  string              `json:"gameId"`
	Market                  string              `json:"market"`
	ChannelID               string              `json:"channelId"`
	ConfigStatus            common.ConfigStatus `json:"configStatus"`
	Hidden                  bool                `json:"hidden"`
	IncludedInSnapshot      bool                `json:"includedInSnapshot"`
	IncludedInSync          bool                `json:"includedInSync"`
	IncludedInRuntimeConfig bool                `json:"includedInRuntimeConfig"`
	IncompatibleWithMarket  bool                `json:"incompatibleWithMarket"`
}
