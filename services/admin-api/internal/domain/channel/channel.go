// Package channel 持有渠道与渠道实例（GameMarketChannel）的领域实体、值对象与纯规则（无 IO）。
// 平台级主数据：Channel（channels）+ ChannelPolicy（channel_policies）；游戏维度实例：GameMarketChannel（game_channels）+ ChannelPackage（channel_packages）。
// env 由所在 schema 决定，业务实体不带 env 列（00 §2.2）。
package channel

import (
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// Channel 平台级渠道主数据（platform.channels）。region 为国内/非国内属性（D3）。
type Channel struct {
	ID          int64
	ChannelID   string
	ChannelName string
	ChannelType string
	Region      ChannelRegion
	Enabled     bool
	Sort        int
}

// ChannelPolicy 渠道策略（platform.channel_policies），与 Channel 一对一。
type ChannelPolicy struct {
	ChannelIDRef  int64
	LoginMode     common.LoginMode
	PaymentMode   common.PaymentMode
	LoginLocked   bool
	PaymentLocked bool
}

// ChannelWithPolicy 渠道主数据 + 策略（GET /games/{gameId}/channels 候选列表用）。
type ChannelWithPolicy struct {
	Channel Channel
	Policy  ChannelPolicy
}

// ChannelType 取值集合（00 §3.1，无默认）。
const (
	ChannelTypeStore    = "store"
	ChannelTypeOEM      = "oem"
	ChannelTypeWeb      = "web"
	ChannelTypeDirect   = "direct"
	ChannelTypeMiniGame = "mini_game"
)

// IsValidChannelType 校验渠道类型枚举（00 §3.1）。
func IsValidChannelType(t string) bool {
	switch t {
	case ChannelTypeStore, ChannelTypeOEM, ChannelTypeWeb, ChannelTypeDirect, ChannelTypeMiniGame:
		return true
	default:
		return false
	}
}

// ChannelPackage 渠道包（channel_packages，游戏维度业务表，每环境 schema 各一份）。
// (game_channel_id_ref, package_code) 同实例内唯一。override_json 默认 {}（仅存差异）。
type ChannelPackage struct {
	ID                   int64
	GameChannelIDRef     int64
	PackageCode          string
	PackageName          string
	MarketCode           string
	BundleID             string
	InheritChannelConfig bool
	Enabled              bool
	OverrideJSON         map[string]any
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
