package dto

import "time"

// ===== Commands（写用例输入；指针表示可选，nil 不改）=====

// CreateMarketChannelCmd 创建渠道实例（空白/复制）命令
// （POST /games/{gameId}/markets/{market}/channels）。
type CreateMarketChannelCmd struct {
	GameID         string
	Market         string
	ChannelID      string
	Mode           string // empty | copy（默认 empty）
	CopyFromMarket string // mode=copy 时必填
	Enabled        *bool  // 缺省 true
	Remark         string
}

// UpdateMarketChannelCmd 编辑渠道实例命令（PATCH /game-channels/{gameChannelId}）。
// 仅可改 enabled/remark；身份（market/channel）不可改。
type UpdateMarketChannelCmd struct {
	GameChannelID int64
	Enabled       *bool
	Remark        *string
}

// HideMarketChannelCmd 隐藏命令（POST /game-channels/{gameChannelId}/hide）。
type HideMarketChannelCmd struct {
	GameChannelID int64
	Reason        string
}

// UnhideMarketChannelCmd 恢复命令（POST /game-channels/{gameChannelId}/unhide）。
type UnhideMarketChannelCmd struct {
	GameChannelID int64
}

// ListMarketChannelsQuery 渠道实例列表查询（GET /games/{gameId}/market-channels）。
// Market 默认 "ALL"（全 market）；Hidden 默认 false（不含隐藏项）；Compatible/ConfigStatus 不限时为零值。
type ListMarketChannelsQuery struct {
	GameID       string
	Market       string // ALL 或具体 market
	ChannelID    string
	Compatible   *bool
	Hidden       bool
	ConfigStatus string // empty/invalid/valid 或空（不限）
	Page         int
	PageSize     int
}

// CreatePackageCmd 创建渠道包命令（POST /game-channels/{gameChannelId}/packages）。
type CreatePackageCmd struct {
	GameChannelID        int64
	PackageCode          string
	PackageName          string
	MarketCode           string
	BundleID             string
	InheritChannelConfig *bool // 缺省 true
	Enabled              *bool // 缺省 true
}

// UpdatePackageCmd 编辑渠道包命令（PATCH /channel-packages/{packageId}）。
type UpdatePackageCmd struct {
	PackageID            int64
	PackageName          *string
	BundleID             *string
	InheritChannelConfig *bool
	Enabled              *bool
	OverrideJSON         map[string]any // nil 不改
}

// ===== Views（响应输出；camelCase 由 JSON tag 决定）=====

// ChannelOptionView 候选渠道主数据 + 策略（GET /games/{gameId}/channels）。
type ChannelOptionView struct {
	ChannelID     string `json:"channelId"`
	ChannelName   string `json:"channelName"`
	ChannelType   string `json:"channelType"`
	Region        string `json:"region"`
	LoginMode     string `json:"loginMode"`
	PaymentMode   string `json:"paymentMode"`
	LoginLocked   bool   `json:"loginLocked"`
	PaymentLocked bool   `json:"paymentLocked"`
}

// MarketChannelListItem 渠道实例列表行（GET /games/{gameId}/market-channels）。
type MarketChannelListItem struct {
	GameChannelID           int64     `json:"gameChannelId"`
	DisplayKey              string    `json:"displayKey"`
	GameID                  string    `json:"gameId"`
	Market                  string    `json:"market"`
	ChannelID               string    `json:"channelId"`
	Region                  string    `json:"region"`
	Compatible              bool      `json:"compatible"`
	Hidden                  bool      `json:"hidden"`
	ConfigStatus            string    `json:"configStatus"`
	IncludedInSnapshot      bool      `json:"includedInSnapshot"`
	IncludedInSync          bool      `json:"includedInSync"`
	IncludedInRuntimeConfig bool      `json:"includedInRuntimeConfig"`
	RuntimeReason           string    `json:"runtimeReason"`
	CopiedFromMarket        string    `json:"copiedFromMarket"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

// MarketChannelDetail 渠道实例详情（GET /game-channels/{gameChannelId}）。
type MarketChannelDetail struct {
	GameChannelID           int64      `json:"gameChannelId"`
	DisplayKey              string     `json:"displayKey"`
	GameID                  string     `json:"gameId"`
	Market                  string     `json:"market"`
	ChannelID               string     `json:"channelId"`
	Region                  string     `json:"region"`
	Compatible              bool       `json:"compatible"`
	Enabled                 bool       `json:"enabled"`
	Hidden                  bool       `json:"hidden"`
	HiddenBy                string     `json:"hiddenBy"`
	HiddenAt                *time.Time `json:"hiddenAt"`
	ConfigStatus            string     `json:"configStatus"`
	LastCheckAt             *time.Time `json:"lastCheckAt"`
	LastCheckMessage        string     `json:"lastCheckMessage"`
	CopiedFromMarket        string     `json:"copiedFromMarket"`
	Remark                  string     `json:"remark"`
	IncludedInSnapshot      bool       `json:"includedInSnapshot"`
	IncludedInSync          bool       `json:"includedInSync"`
	IncludedInRuntimeConfig bool       `json:"includedInRuntimeConfig"`
	RuntimeReason           string     `json:"runtimeReason"`
	Environment             string     `json:"environment"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

// CreateMarketChannelResult 创建渠道实例响应（201）。
type CreateMarketChannelResult struct {
	GameChannelID    int64  `json:"gameChannelId"`
	DisplayKey       string `json:"displayKey"`
	Market           string `json:"market"`
	ChannelID        string `json:"channelId"`
	ConfigStatus     string `json:"configStatus"`
	LastCheckMessage string `json:"lastCheckMessage"`
	CopiedFromMarket string `json:"copiedFromMarket"`
}

// ChannelPackageView 渠道包视图。
type ChannelPackageView struct {
	PackageID            int64          `json:"packageId"`
	GameChannelID        int64          `json:"gameChannelId"`
	PackageCode          string         `json:"packageCode"`
	PackageName          string         `json:"packageName"`
	MarketCode           string         `json:"marketCode"`
	BundleID             string         `json:"bundleId"`
	InheritChannelConfig bool           `json:"inheritChannelConfig"`
	Enabled              bool           `json:"enabled"`
	OverrideJSON         map[string]any `json:"overrideJson"`
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
}
