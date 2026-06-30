package plugin

import "github.com/csw/console/services/admin-api/internal/domain/common"

const (
	RegionDomestic = "domestic"
	RegionOverseas = "overseas"
)

// ValidatePluginRegionCompatibility 纯规则：CN 仅 domestic；非 CN 仅 overseas。
func ValidatePluginRegionCompatibility(market, region string) bool {
	m := common.Market(market)
	if !m.IsKnown() {
		return false
	}
	switch region {
	case RegionDomestic:
		return m.IsCN()
	case RegionOverseas:
		return !m.IsCN()
	default:
		return false
	}
}
