package channel

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestValidateMarketChannelCompatibility(t *testing.T) {
	if err := ValidateMarketChannelCompatibility(common.MarketCN, ChannelRegionCN); err != nil {
		t.Fatalf("CN + CN should pass: %v", err)
	}

	if err := ValidateMarketChannelCompatibility(common.MarketCN, ChannelRegionGlobal); err == nil {
		t.Fatal("CN + GLOBAL should fail")
	}

	if err := ValidateMarketChannelCompatibility(common.MarketGlobal, ChannelRegionGlobal); err != nil {
		t.Fatalf("GLOBAL + GLOBAL should pass: %v", err)
	}

	if err := ValidateMarketChannelCompatibility(common.MarketKR, ChannelRegionGlobal); err != nil {
		t.Fatalf("KR + GLOBAL should pass: %v", err)
	}

	if err := ValidateMarketChannelCompatibility(common.MarketKR, ChannelRegionKR); err != nil {
		t.Fatalf("KR + KR should pass: %v", err)
	}

	if err := ValidateMarketChannelCompatibility(common.MarketJP, ChannelRegionCN); err == nil {
		t.Fatal("JP + CN should fail")
	}

	if err := ValidateMarketChannelCompatibility(common.MarketJP, ChannelRegionKR); err == nil {
		t.Fatal("JP + KR should fail")
	}

	if err := ValidateMarketChannelCompatibility(common.Market(""), ChannelRegionGlobal); err == nil {
		t.Fatal("empty market should fail")
	}

	if err := ValidateMarketChannelCompatibility(common.Market("US"), ChannelRegionGlobal); err == nil {
		t.Fatal("US market should fail")
	}

	if err := ValidateMarketChannelCompatibility(common.MarketGlobal, ChannelRegion("mars")); err == nil {
		t.Fatal("invalid channel region should fail")
	}
}
