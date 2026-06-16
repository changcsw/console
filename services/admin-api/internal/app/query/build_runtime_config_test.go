package query

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestSpecificMarketOverridesGlobalChannelInstance(t *testing.T) {
	cfg := BuildRuntimeConfig(RuntimeConfigInput{
		TargetMarket: common.MarketJP,
		GlobalChannels: []channel.GameMarketChannel{
			{
				Market:       string(common.MarketGlobal),
				ChannelID:    "google",
				ConfigStatus: common.ConfigStatusValid,
				NormalConfig: map[string]any{"value": "global"},
			},
		},
		MarketChannels: []channel.GameMarketChannel{
			{
				Market:       string(common.MarketJP),
				ChannelID:    "google",
				ConfigStatus: common.ConfigStatusValid,
				NormalConfig: map[string]any{"value": "jp"},
			},
		},
	})

	got, ok := cfg.Channels["google"]
	if !ok {
		t.Fatal("expected google channel to exist")
	}

	if got.NormalConfig["value"] != "jp" {
		t.Fatalf("expected JP override, got %#v", got.NormalConfig["value"])
	}
}
