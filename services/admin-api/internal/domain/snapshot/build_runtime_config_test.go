package snapshot

import (
	"testing"
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestBuildRuntimeConfigMergeByInstance(t *testing.T) {
	view := ValidDataView{
		GameID:      "g1",
		GeneratedAt: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		Channels: []ChannelInput{
			{
				ChannelID:    "google",
				Region:       "overseas",
				Market:       common.MarketGlobal,
				Enabled:      true,
				ConfigStatus: common.ConfigStatusValid,
			},
			{
				ChannelID:    "google",
				Region:       "overseas",
				Market:       common.MarketJP,
				Enabled:      true,
				ConfigStatus: common.ConfigStatusValid,
			},
		},
	}

	out := BuildRuntimeConfig(view)
	jp := out.Markets["JP"].Channels
	if len(jp) != 1 {
		t.Fatalf("expected 1 JP channel, got %d", len(jp))
	}
	if jp[0].SourceMarket != "JP" {
		t.Fatalf("expected JP override source, got %s", jp[0].SourceMarket)
	}
}

func TestBuildRuntimeConfigCNNoGlobalFallback(t *testing.T) {
	view := ValidDataView{
		GameID:      "g1",
		GeneratedAt: time.Now().UTC(),
		Channels: []ChannelInput{
			{
				ChannelID:    "google",
				Region:       "overseas",
				Market:       common.MarketGlobal,
				Enabled:      true,
				ConfigStatus: common.ConfigStatusValid,
			},
		},
	}

	out := BuildRuntimeConfig(view)
	if got := len(out.Markets["CN"].Channels); got != 0 {
		t.Fatalf("expected CN empty channels, got %d", got)
	}
}

func TestBuildRuntimeConfigScopeAndRequiredPlugin(t *testing.T) {
	view := ValidDataView{
		GameID:      "g1",
		GeneratedAt: time.Now().UTC(),
		Channels: []ChannelInput{
			{
				ChannelID:    "huawei_cn",
				Region:       "domestic",
				Market:       common.MarketCN,
				Enabled:      true,
				ConfigStatus: common.ConfigStatusValid,
				Login: &TemplateConfig{
					Enabled:      true,
					ConfigStatus: common.ConfigStatusValid,
					Config: map[string]any{
						"clientId":     "abc",
						"serverSecret": "s1",
					},
					FormSchema: []ScopeField{
						{Key: "clientId", Scope: "client"},
						{Key: "serverSecret", Scope: "server"},
					},
					SecretFields: []string{"clientId"},
				},
				Plugins: []PluginConfig{
					{
						PluginID:     "must",
						Required:     true,
						Region:       "domestic",
						Enabled:      false,
						ConfigStatus: common.ConfigStatusInvalid,
					},
				},
			},
		},
	}

	out := BuildRuntimeConfig(view)
	if got := len(out.Markets["CN"].Channels); got != 0 {
		t.Fatalf("expected channel removed by required plugin invalid, got %d", got)
	}
}
