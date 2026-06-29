package dto

import "time"

// ===== Commands（写用例输入；指针表示可选，nil 不改）=====

// CreateGameCmd 创建游戏命令（POST /games）。gameId/gameSecret 服务端生成，忽略前端传入。
type CreateGameCmd struct {
	Name              string
	Alias             string
	IconURL           string
	DefaultMarketCode string
	Status            string
	Markets           []string
}

// UpdateGameCmd 编辑游戏基础信息命令（PATCH /games/{gameId}）。
type UpdateGameCmd struct {
	GameID            string
	Name              *string
	Alias             *string
	IconURL           *string
	Status            *string
	DefaultMarketCode *string
}

// MarketInput 全量覆盖市场的单项输入（PUT /games/{gameId}/markets）。
type MarketInput struct {
	MarketCode    string
	IsDefault     bool
	Enabled       bool
	DefaultLocale string
}

// ReplaceMarketsCmd 全量覆盖市场命令。
type ReplaceMarketsCmd struct {
	GameID  string
	Markets []MarketInput
}

// LegalLinkInput 全量覆盖法务链接的单项输入（PUT /games/{gameId}/legal-links）。
type LegalLinkInput struct {
	ScopeType        string
	ScopeValue       string
	TermsURL         string
	PrivacyURL       string
	DeleteAccountURL string
}

// ReplaceLegalLinksCmd 全量覆盖法务链接命令。
type ReplaceLegalLinksCmd struct {
	GameID     string
	LegalLinks []LegalLinkInput
}

// ListGamesQuery 游戏列表查询（GET /games）。
type ListGamesQuery struct {
	Keyword    string
	Status     string
	MarketCode string
	Page       int
	PageSize   int
	Sort       string
}

// ===== Views（响应输出；camelCase 由 JSON tag 决定）=====

// GameMarketView 市场视图（详情/创建响应）。
type GameMarketView struct {
	MarketCode    string `json:"marketCode"`
	IsDefault     bool   `json:"isDefault"`
	Enabled       bool   `json:"enabled"`
	DefaultLocale string `json:"defaultLocale"`
}

// GameLegalLinkView 法务链接视图。
type GameLegalLinkView struct {
	ScopeType        string `json:"scopeType"`
	ScopeValue       string `json:"scopeValue"`
	TermsURL         string `json:"termsUrl"`
	PrivacyURL       string `json:"privacyUrl"`
	DeleteAccountURL string `json:"deleteAccountUrl"`
}

// GameListItem 游戏列表项（轻量摘要，不返回 gameSecret）。
type GameListItem struct {
	GameID            string    `json:"gameId"`
	Name              string    `json:"name"`
	Alias             string    `json:"alias"`
	IconURL           string    `json:"iconUrl"`
	Status            string    `json:"status"`
	DefaultMarketCode string    `json:"defaultMarketCode"`
	MarketCodes       []string  `json:"marketCodes"`
	MarketCount       int       `json:"marketCount"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// GameDetail 游戏完整聚合视图（详情/创建响应）。
// 创建响应一次性返回明文 gameSecret（SecretMasked=false）；其余接口恒脱敏（gameSecret="masked"）。
type GameDetail struct {
	GameID            string              `json:"gameId"`
	Name              string              `json:"name"`
	Alias             string              `json:"alias"`
	IconURL           string              `json:"iconUrl"`
	Status            string              `json:"status"`
	DefaultMarketCode string              `json:"defaultMarketCode"`
	GameSecret        string              `json:"gameSecret"`
	SecretMasked      bool                `json:"secretMasked"`
	Environment       string              `json:"environment"`
	Markets           []GameMarketView    `json:"markets"`
	LegalLinks        []GameLegalLinkView `json:"legalLinks"`
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         time.Time           `json:"updatedAt"`
}
