// Package game 持有游戏主数据（Game Core）的领域实体、值对象与纯规则（无 IO）。
// Game 是发行后台根聚合：基础信息 + 发行市场集合 + 法务链接。env 由所在 schema 决定，实体不带 env 列。
package game

import (
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// Game 游戏聚合根（业务表 games，每环境 schema 各一份）。
// GameSecret 仅创建时一次性下发明文，其余响应恒脱敏（00 §6）。
type Game struct {
	ID                int64 // 内部主键，不对外暴露
	GameID            string
	GameSecret        string
	Name              string
	Alias             string
	IconURL           string
	DefaultMarketCode string
	Status            common.GameStatus
	Markets           []GameMarket
	LegalLinks        []GameLegalLink
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// GameMarket 游戏发行市场（业务表 game_markets）。每游戏恰好一条 IsDefault=true。
type GameMarket struct {
	MarketCode    string
	IsDefault     bool
	Enabled       bool
	DefaultLocale string
}

// GameLegalLink 游戏法务链接（业务表 game_legal_links）。(ScopeType, ScopeValue) 同游戏内唯一。
type GameLegalLink struct {
	ScopeType        string
	ScopeValue       string
	TermsURL         string
	PrivacyURL       string
	DeleteAccountURL string
}
