package channel

import (
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// CopiedMissingFieldsMessage 复制创建后 config_status=invalid 的提示（00 §3.4 强约束）。
const CopiedMissingFieldsMessage = "缺少必填敏感字段或文件字段"

// 运行态不可生效原因（compact §运行态标识）。
const (
	RuntimeReasonHidden        = "hidden"
	RuntimeReasonIncompatible  = "incompatible"
	RuntimeReasonInvalidConfig = "invalid_config"
)

// GameMarketChannel 渠道实例聚合 = "某游戏 + 某 market + 某渠道"（落表 game_channels）。
// env 由所在 schema 决定，不落列。Region 为派生属性（取自 channels.region，用于兼容性实时判定，不落 game_channels）。
// NormalConfig/SecretConfig/FileConfig 为模板驱动配置载荷（由下游 channel-login/product/feature-plugin 维护），
// 本模块不持久化这些载荷，仅承载于聚合用于快照/同步合并（snapshot/sync）与复制清空语义。
type GameMarketChannel struct {
	ID               int64  // game_channels.id（对外 gameChannelId）
	GameIDRef        int64  // games.id（内部）
	GameID           string // games.game_id（对外业务键）
	Market           string
	ChannelIDRef     int64         // platform.channels.id（内部）
	ChannelID        string        // channels.channel_id（对外业务键）
	Region           ChannelRegion // 派生（channels.region），用于兼容性判定，不落库
	Enabled          bool
	Hidden           bool
	HiddenBy         string
	HiddenAt         *time.Time
	ConfigStatus     common.ConfigStatus
	LastCheckAt      *time.Time
	LastCheckMessage string
	CopiedFromMarket string
	Remark           string
	CreatedAt        time.Time
	UpdatedAt        time.Time

	NormalConfig map[string]any
	SecretConfig map[string]string
	FileConfig   map[string]string
}

// RuntimeFlags 三态只读运行标识 + 不可生效原因（compact §运行态标识）。
type RuntimeFlags struct {
	IncludedInRuntimeConfig bool
	IncludedInSnapshot      bool
	IncludedInSync          bool
	Reason                  string // "" / hidden / incompatible / invalid_config
}

// DisplayKey 列表行展示用复合串 gameId:market:channelId（仅展示，不作接口路径参数）。
func (g GameMarketChannel) DisplayKey() string {
	return g.GameID + ":" + g.Market + ":" + g.ChannelID
}

// Compatible 兼容性派生（实时按 market + region 判定，不落库）。
func (g GameMarketChannel) Compatible() bool {
	return IsCompatible(common.Market(g.Market), g.Region)
}

// ResolveRuntimeFlags 三态运行标识纯函数（compact）：
//
//	IncludedInRuntimeConfig = !hidden && compatible && config_status==valid
//	IncludedInSnapshot      = IncludedInRuntimeConfig
//	IncludedInSync          = 同口径（隐藏/不兼容/无效一律不参与）
//
// 任一为 false 时给出原因（hidden 优先于 incompatible 优先于 invalid_config）。
func (g GameMarketChannel) ResolveRuntimeFlags() RuntimeFlags {
	compatible := g.Compatible()
	included := !g.Hidden && compatible && g.ConfigStatus == common.ConfigStatusValid
	reason := ""
	switch {
	case g.Hidden:
		reason = RuntimeReasonHidden
	case !compatible:
		reason = RuntimeReasonIncompatible
	case g.ConfigStatus != common.ConfigStatusValid:
		reason = RuntimeReasonInvalidConfig
	}
	return RuntimeFlags{
		IncludedInRuntimeConfig: included,
		IncludedInSnapshot:      included,
		IncludedInSync:          included,
		Reason:                  reason,
	}
}

// IncludedInRuntimeConfig 兼容旧下游（snapshot/sync 合并）的简化判定（不含兼容性维度，仅 !hidden && valid）。
// API 响应一律用 ResolveRuntimeFlags（含兼容性）。
func (g GameMarketChannel) IncludedInRuntimeConfig() bool {
	return !g.Hidden && g.ConfigStatus == common.ConfigStatusValid
}

// Hide 隐藏实例：hidden=true、记录操作人与时间（compact §隐藏/恢复，审计 channel.hide）。
func (g *GameMarketChannel) Hide(operator string, at time.Time) {
	g.Hidden = true
	g.HiddenBy = operator
	t := at
	g.HiddenAt = &t
}

// Unhide 恢复实例：hidden=false、清隐藏操作人/时间（审计 channel.unhide）。
func (g *GameMarketChannel) Unhide() {
	g.Hidden = false
	g.HiddenBy = ""
	g.HiddenAt = nil
}

// NewBlankMarketChannel 空白创建：config_status=empty，配置载荷为空（compact §创建）。
func NewBlankMarketChannel(gameID, market, channelID string, region ChannelRegion) GameMarketChannel {
	return GameMarketChannel{
		GameID:       gameID,
		Market:       market,
		ChannelID:    channelID,
		Region:       region,
		Enabled:      true,
		ConfigStatus: common.ConfigStatusEmpty,
		NormalConfig: map[string]any{},
		SecretConfig: map[string]string{},
		FileConfig:   map[string]string{},
	}
}

// NewCopiedMarketChannel 复制创建（从其它 market 同渠道）：仅复制普通字段；secret/file 清空；
// config_status 强制 invalid；记 copied_from_market 与缺字段提示（compact §创建 / 00 §3.4）。
func NewCopiedMarketChannel(gameID, market, channelID string, region ChannelRegion, source GameMarketChannel) GameMarketChannel {
	return GameMarketChannel{
		GameID:           gameID,
		Market:           market,
		ChannelID:        channelID,
		Region:           region,
		Enabled:          true,
		ConfigStatus:     common.ConfigStatusInvalid,
		CopiedFromMarket: source.Market,
		LastCheckMessage: CopiedMissingFieldsMessage,
		NormalConfig:     cloneAnyMap(source.NormalConfig),
		SecretConfig:     map[string]string{},
		FileConfig:       map[string]string{},
	}
}

func cloneAnyMap(input map[string]any) map[string]any {
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}
