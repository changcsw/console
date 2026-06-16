package channel

import "github.com/csw/console/services/admin-api/internal/domain/common"

type GameMarketChannel struct {
	ID           string
	GameID       string
	Market       string
	ChannelID    string
	Hidden       bool
	HiddenBy     string
	NormalConfig map[string]any
	SecretConfig map[string]string
	FileConfig   map[string]string
	ConfigStatus common.ConfigStatus
}

func NewCopiedMarketChannel(gameID, market, channelID string, source GameMarketChannel) GameMarketChannel {
	return GameMarketChannel{
		ID:           BuildGameMarketChannelID(gameID, market, channelID),
		GameID:       gameID,
		Market:       market,
		ChannelID:    channelID,
		NormalConfig: cloneAnyMap(source.NormalConfig),
		SecretConfig: map[string]string{},
		FileConfig:   map[string]string{},
		ConfigStatus: common.ConfigStatusInvalid,
	}
}

func (g *GameMarketChannel) Hide(operator string) {
	g.Hidden = true
	g.HiddenBy = operator
}

func (g *GameMarketChannel) Unhide() {
	g.Hidden = false
	g.HiddenBy = ""
}

func (g GameMarketChannel) IncludedInRuntimeConfig() bool {
	return !g.Hidden && g.ConfigStatus == common.ConfigStatusValid
}

func BuildGameMarketChannelID(gameID, market, channelID string) string {
	return gameID + ":" + market + ":" + channelID
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}

	return cloned
}
