package channel

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestNewCopiedMarketChannelClearsSensitiveFields(t *testing.T) {
	source := GameMarketChannel{
		NormalConfig: map[string]any{"client_id": "jp-client"},
		SecretConfig: map[string]string{"client_secret": "secret"},
		FileConfig:   map[string]string{"keystore": "file-id"},
	}

	copied := NewCopiedMarketChannel("game-1", "JP", "google", source)

	if copied.ConfigStatus != common.ConfigStatusInvalid {
		t.Fatalf("expected invalid status, got %s", copied.ConfigStatus)
	}

	if copied.SecretConfig["client_secret"] != "" {
		t.Fatal("secret should be cleared")
	}

	if copied.FileConfig["keystore"] != "" {
		t.Fatal("file should be cleared")
	}
}

func TestHideMarksChannelExcludedFromRuntime(t *testing.T) {
	item := GameMarketChannel{
		ConfigStatus: common.ConfigStatusValid,
	}
	item.Hide("ops@example.com")

	if !item.Hidden {
		t.Fatal("channel should be hidden")
	}

	if item.IncludedInRuntimeConfig() {
		t.Fatal("hidden channel should be excluded from runtime")
	}

	if item.HiddenBy != "ops@example.com" {
		t.Fatalf("expected operator to be recorded, got %s", item.HiddenBy)
	}
}
