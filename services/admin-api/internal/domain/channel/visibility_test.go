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

	if err := ValidateMarketChannelCompatibility(common.MarketGlobal, ChannelRegionOverseas); err != nil {
		t.Fatalf("GLOBAL + overseas should pass: %v", err)
	}

	if err := ValidateMarketChannelCompatibility(common.MarketKR, ChannelRegionOverseas); err != nil {
		t.Fatalf("KR + overseas should pass: %v", err)
	}

	if err := ValidateMarketChannelCompatibility(common.MarketJP, ChannelRegionDomestic); err == nil {
		t.Fatal("JP + domestic should fail")
	}

	if err := ValidateMarketChannelCompatibility(common.Market(""), ChannelRegionOverseas); err == nil {
		t.Fatal("empty market should fail")
	}

	if err := ValidateMarketChannelCompatibility(common.Market("US"), ChannelRegionOverseas); err == nil {
		t.Fatal("US market should fail")
	}

	if err := ValidateMarketChannelCompatibility(common.MarketGlobal, ChannelRegion("mars")); err == nil {
		t.Fatal("invalid channel region should fail")
	}
}
