package channel

import (
	"testing"
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestNewCopiedMarketChannelClearsSensitiveFields(t *testing.T) {
	source := GameMarketChannel{
		Market:       string(common.MarketGlobal),
		NormalConfig: map[string]any{"client_id": "global-client"},
		SecretConfig: map[string]string{"client_secret": "secret"},
		FileConfig:   map[string]string{"keystore": "file-id"},
	}

	copied := NewCopiedMarketChannel("100001", "JP", "google", ChannelRegionOverseas, source)

	if copied.ConfigStatus != common.ConfigStatusInvalid {
		t.Fatalf("expected invalid status, got %s", copied.ConfigStatus)
	}
	if copied.NormalConfig["client_id"] != "global-client" {
		t.Fatalf("expected copied normal config, got %#v", copied.NormalConfig)
	}
	if copied.SecretConfig["client_secret"] != "" {
		t.Fatal("secret should be cleared")
	}
	if copied.FileConfig["keystore"] != "" {
		t.Fatal("file should be cleared")
	}
	if copied.CopiedFromMarket != string(common.MarketGlobal) {
		t.Fatalf("expected copiedFromMarket=GLOBAL, got %s", copied.CopiedFromMarket)
	}
	if copied.LastCheckMessage != CopiedMissingFieldsMessage {
		t.Fatalf("expected copy hint message, got %s", copied.LastCheckMessage)
	}
}

func TestHideRecordsOperatorAndExcludesFromRuntime(t *testing.T) {
	item := GameMarketChannel{
		Market:       string(common.MarketJP),
		Region:       ChannelRegionOverseas,
		ConfigStatus: common.ConfigStatusValid,
	}
	now := time.Now()
	item.Hide("ops@example.com", now)

	if !item.Hidden || item.HiddenBy != "ops@example.com" || item.HiddenAt == nil {
		t.Fatalf("expected hidden state recorded, got %#v", item)
	}
	flags := item.ResolveRuntimeFlags()
	if flags.IncludedInRuntimeConfig || flags.IncludedInSnapshot || flags.IncludedInSync {
		t.Fatal("hidden channel must be excluded from runtime/snapshot/sync")
	}
	if flags.Reason != RuntimeReasonHidden {
		t.Fatalf("expected hidden reason, got %s", flags.Reason)
	}
}

func TestUnhideRestoresRuntimeEligibility(t *testing.T) {
	now := time.Now()
	item := GameMarketChannel{
		Market:       string(common.MarketJP),
		Region:       ChannelRegionOverseas,
		Hidden:       true,
		HiddenBy:     "ops@example.com",
		HiddenAt:     &now,
		ConfigStatus: common.ConfigStatusValid,
	}
	item.Unhide()

	if item.Hidden || item.HiddenBy != "" || item.HiddenAt != nil {
		t.Fatalf("expected hidden state cleared, got %#v", item)
	}
	if !item.ResolveRuntimeFlags().IncludedInRuntimeConfig {
		t.Fatal("visible valid compatible channel should be runtime-eligible")
	}
}

func TestResolveRuntimeFlagsReasons(t *testing.T) {
	// 不兼容：JP + domestic region。
	incompatible := GameMarketChannel{Market: string(common.MarketJP), Region: ChannelRegionDomestic, ConfigStatus: common.ConfigStatusValid}
	if got := incompatible.ResolveRuntimeFlags(); got.IncludedInRuntimeConfig || got.Reason != RuntimeReasonIncompatible {
		t.Fatalf("expected incompatible reason, got %#v", got)
	}
	// 无效配置。
	invalid := GameMarketChannel{Market: string(common.MarketJP), Region: ChannelRegionOverseas, ConfigStatus: common.ConfigStatusInvalid}
	if got := invalid.ResolveRuntimeFlags(); got.IncludedInRuntimeConfig || got.Reason != RuntimeReasonInvalidConfig {
		t.Fatalf("expected invalid_config reason, got %#v", got)
	}
	// 全过。
	ok := GameMarketChannel{Market: string(common.MarketGlobal), Region: ChannelRegionOverseas, ConfigStatus: common.ConfigStatusValid}
	if got := ok.ResolveRuntimeFlags(); !got.IncludedInRuntimeConfig || got.Reason != "" {
		t.Fatalf("expected runtime-eligible with no reason, got %#v", got)
	}
}

func TestCanHideRejectsUnhealthy(t *testing.T) {
	if err := CanHide(common.ConfigStatusValid); err != nil {
		t.Fatalf("valid should be hideable: %v", err)
	}
	if err := CanHide(common.ConfigStatusInvalid); err == nil {
		t.Fatal("invalid must not be hideable")
	}
	if err := CanHide(common.ConfigStatusEmpty); err == nil {
		t.Fatal("empty must not be hideable")
	}
}
