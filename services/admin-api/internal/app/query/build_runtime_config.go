package query

import (
	"github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type RuntimeConfigInput struct {
	TargetMarket   common.Market
	GlobalChannels []channel.GameMarketChannel
	MarketChannels []channel.GameMarketChannel
}

type RuntimeConfig struct {
	Channels map[string]channel.GameMarketChannel
}

func BuildRuntimeConfig(input RuntimeConfigInput) RuntimeConfig {
	result := RuntimeConfig{Channels: map[string]channel.GameMarketChannel{}}

	if shouldLoadGlobalChannels(input.TargetMarket) {
		mergeRuntimeChannels(result.Channels, input.GlobalChannels)
	}

	mergeRuntimeChannels(result.Channels, input.MarketChannels)
	return result
}

func shouldLoadGlobalChannels(targetMarket common.Market) bool {
	return targetMarket == common.MarketGlobal || targetMarket.UsesGlobalFallback()
}

func mergeRuntimeChannels(target map[string]channel.GameMarketChannel, items []channel.GameMarketChannel) {
	for _, item := range items {
		if !item.IncludedInRuntimeConfig() {
			continue
		}

		target[item.ChannelID] = item
	}
}
