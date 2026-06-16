package channel

import (
	"fmt"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type ChannelRegion string

const (
	ChannelRegionDomestic ChannelRegion = "domestic"
	ChannelRegionOverseas ChannelRegion = "overseas"
)

func ValidateMarketChannelCompatibility(market common.Market, region ChannelRegion) error {
	if !market.IsKnown() {
		return fmt.Errorf("market %s is not supported", market)
	}

	if !region.IsKnown() {
		return fmt.Errorf("channel region %s is not supported", region)
	}

	if market.IsCN() && region != ChannelRegionDomestic {
		return fmt.Errorf("market %s only accepts domestic channels", market)
	}

	if !market.IsCN() && region != ChannelRegionOverseas {
		return fmt.Errorf("market %s only accepts overseas channels", market)
	}

	return nil
}

func (r ChannelRegion) IsKnown() bool {
	switch r {
	case ChannelRegionDomestic, ChannelRegionOverseas:
		return true
	default:
		return false
	}
}
