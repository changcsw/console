package channel

import "github.com/csw/console/services/admin-api/internal/domain/common"

type GameMarketChannel struct {
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

func (g GameMarketChannel) IncludedInRuntimeConfig() bool {
	return !g.Hidden && g.ConfigStatus == common.ConfigStatusValid
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
