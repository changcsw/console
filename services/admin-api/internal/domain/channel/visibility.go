package channel

import (
	"errors"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// ChannelRegion 渠道发行市场属性（00 §3.1 ChannelRegion，D3）。取值与 common.Market 同集：
// GLOBAL=全球发行（不含中国大陆），CN=中国大陆，JP/KR/SEA/HMT=对应市场专属。
type ChannelRegion string

const (
	ChannelRegionGlobal ChannelRegion = "GLOBAL"
	ChannelRegionCN     ChannelRegion = "CN"
	ChannelRegionJP     ChannelRegion = "JP"
	ChannelRegionKR     ChannelRegion = "KR"
	ChannelRegionSEA    ChannelRegion = "SEA"
	ChannelRegionHMT    ChannelRegion = "HMT"
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
	case ChannelRegionGlobal, ChannelRegionCN, ChannelRegionJP, ChannelRegionKR, ChannelRegionSEA, ChannelRegionHMT:
		return true
	default:
		return false
	}
}

// ValidateMarketChannelCompatibility 可见性/兼容性纯规则（compact §业务规则，服务端强制二次校验）：
//   - market==CN  ⇒ 仅允许发行市场 CN 的渠道（中国大陆渠道体系独立）；
//   - market!=CN  ⇒ 允许发行市场 GLOBAL（全球发行）或与该 market 相同的渠道。
func ValidateMarketChannelCompatibility(market common.Market, region ChannelRegion) error {
	if !market.IsKnown() {
		return ErrUnknownMarket
	}
	if !region.IsKnown() {
		return ErrUnknownRegion
	}
	if market.IsCN() {
		if region != ChannelRegionCN {
			return ErrMarketChannelIncompatible
		}
		return nil
	}
	if region != ChannelRegionGlobal && region != ChannelRegion(market) {
		return ErrMarketChannelIncompatible
	}
	return nil
}

// IsCompatible 兼容性布尔（派生不落库）。
func IsCompatible(market common.Market, region ChannelRegion) bool {
	return ValidateMarketChannelCompatibility(market, region) == nil
}
