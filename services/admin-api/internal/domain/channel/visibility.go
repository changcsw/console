package channel

import (
	"errors"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// ChannelRegion 渠道国内/非国内属性（00 §3.1 ChannelRegion，D3）。
type ChannelRegion string

const (
	ChannelRegionDomestic ChannelRegion = "domestic"
	ChannelRegionOverseas ChannelRegion = "overseas"
)

// 纯规则错误（app 层据此包装为统一错误码）。
var (
	// ErrMarketChannelIncompatible market 与渠道 region 不兼容（→ 400 MARKET_CHANNEL_INCOMPATIBLE）。
	ErrMarketChannelIncompatible = errors.New("market and channel region are incompatible")
	// ErrUnknownMarket market 取值非法。
	ErrUnknownMarket = errors.New("unknown market")
	// ErrUnknownRegion region 取值非法。
	ErrUnknownRegion = errors.New("unknown channel region")
)

// IsKnown 校验 region 枚举。
func (r ChannelRegion) IsKnown() bool {
	switch r {
	case ChannelRegionDomestic, ChannelRegionOverseas:
		return true
	default:
		return false
	}
}

// ValidateMarketChannelCompatibility 可见性/兼容性纯规则（compact §业务规则，服务端强制二次校验）：
//   - market==CN  ⇒ 仅允许 domestic；
//   - market!=CN  ⇒ 仅允许 overseas（含 GLOBAL/JP/KR/SEA/HMT）。
func ValidateMarketChannelCompatibility(market common.Market, region ChannelRegion) error {
	if !market.IsKnown() {
		return ErrUnknownMarket
	}
	if !region.IsKnown() {
		return ErrUnknownRegion
	}
	if market.IsCN() && region != ChannelRegionDomestic {
		return ErrMarketChannelIncompatible
	}
	if !market.IsCN() && region != ChannelRegionOverseas {
		return ErrMarketChannelIncompatible
	}
	return nil
}

// IsCompatible 兼容性布尔（派生不落库）。
func IsCompatible(market common.Market, region ChannelRegion) bool {
	return ValidateMarketChannelCompatibility(market, region) == nil
}
