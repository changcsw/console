package command

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestBuildCreatedMarketChannelCopiesSafeFields(t *testing.T) {
	cmd := CreateMarketChannelCommand{
		GameID:    "game-1",
		Market:    "JP",
		ChannelID: "google",
		Source: channel.GameMarketChannel{
			NormalConfig: map[string]any{"client_id": "jp-client"},
			SecretConfig: map[string]string{"client_secret": "secret"},
			FileConfig:   map[string]string{"keystore": "file-id"},
		},
	}

	got := BuildCreatedMarketChannel(cmd)

	if got.ConfigStatus != common.ConfigStatusInvalid {
		t.Fatalf("expected invalid status, got %s", got.ConfigStatus)
	}

	if got.NormalConfig["client_id"] != "jp-client" {
		t.Fatalf("expected copied normal config, got %#v", got.NormalConfig)
	}

	if got.SecretConfig["client_secret"] != "" {
		t.Fatal("secret should be cleared")
	}

	if got.FileConfig["keystore"] != "" {
		t.Fatal("file should be cleared")
	}
}

func TestApplyHideMarketChannelMarksItemHidden(t *testing.T) {
	item := &channel.GameMarketChannel{
		ConfigStatus: common.ConfigStatusValid,
	}

	ApplyHideMarketChannel(HideMarketChannelCommand{Operator: "ops@example.com"}, item)

	if !item.Hidden {
		t.Fatal("expected item to be hidden")
	}

	if item.HiddenBy != "ops@example.com" {
		t.Fatalf("expected operator to be recorded, got %s", item.HiddenBy)
	}

	if item.IncludedInRuntimeConfig() {
		t.Fatal("hidden item should be excluded from runtime config")
	}
}

func TestApplyUnhideMarketChannelRestoresRuntimeEligibility(t *testing.T) {
	item := &channel.GameMarketChannel{
		Hidden:       true,
		HiddenBy:     "ops@example.com",
		ConfigStatus: common.ConfigStatusValid,
	}

	ApplyUnhideMarketChannel(item)

	if item.Hidden {
		t.Fatal("expected item to be visible again")
	}

	if item.HiddenBy != "" {
		t.Fatalf("expected hidden operator to be cleared, got %s", item.HiddenBy)
	}

	if !item.IncludedInRuntimeConfig() {
		t.Fatal("visible valid item should return to runtime config")
	}
}
