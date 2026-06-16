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
	if market.IsCN() && region != ChannelRegionDomestic {
		return fmt.Errorf("market %s only accepts domestic channels", market)
	}

	if !market.IsCN() && region != ChannelRegionOverseas {
		return fmt.Errorf("market %s only accepts overseas channels", market)
	}

	return nil
}
