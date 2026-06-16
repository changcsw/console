package channel

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestValidateMarketChannelCompatibility(t *testing.T) {
	if err := ValidateMarketChannelCompatibility(common.MarketCN, ChannelRegionDomestic); err != nil {
		t.Fatalf("CN + domestic should pass: %v", err)
	}

	if err := ValidateMarketChannelCompatibility(common.MarketCN, ChannelRegionOverseas); err == nil {
		t.Fatal("CN + overseas should fail")
	}

	if err := ValidateMarketChannelCompatibility(common.MarketJP, ChannelRegionDomestic); err == nil {
		t.Fatal("JP + domestic should fail")
	}
}
