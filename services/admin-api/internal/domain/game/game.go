package game

type Game struct {
	ID                int64           `json:"id"`
	GameID            string          `json:"gameId"`
	GameSecret        string          `json:"gameSecret,omitempty"`
	Name              string          `json:"name"`
	Alias             string          `json:"alias"`
	IconURL           string          `json:"iconUrl"`
	DefaultMarketCode string          `json:"defaultMarketCode"`
	Status            string          `json:"status"`
	Markets           []Market        `json:"markets,omitempty"`
	LegalLinks        []LegalLinkRule `json:"legalLinks,omitempty"`
}

type Market struct {
	MarketCode    string `json:"marketCode"`
	IsDefault     bool   `json:"isDefault"`
	Enabled       bool   `json:"enabled"`
	DefaultLocale string `json:"defaultLocale"`
}

type LegalLinkRule struct {
	ScopeType        string `json:"scopeType"`
	ScopeValue       string `json:"scopeValue"`
	TermsURL         string `json:"termsUrl"`
	PrivacyURL       string `json:"privacyUrl"`
	DeleteAccountURL string `json:"deleteAccountUrl"`
}

