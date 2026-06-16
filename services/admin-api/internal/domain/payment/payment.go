package payment

type PayWay struct {
	ID         int64  `json:"id"`
	PayWayID   string `json:"payWayId"`
	PayWayName string `json:"payWayName"`
	PayWayType string `json:"payWayType"`
	Enabled    bool   `json:"enabled"`
	Sort       int    `json:"sort"`
}

type Route struct {
	ID                 int64  `json:"id"`
	GameIDRef          int64  `json:"gameIdRef"`
	MarketCode         string `json:"marketCode"`
	CountryCode        string `json:"countryCode"`
	Currency           string `json:"currency"`
	ChannelIDRef       *int64 `json:"channelIdRef,omitempty"`
	PackageIDRef       *int64 `json:"packageIdRef,omitempty"`
	PayWayIDRef        int64  `json:"payWayIdRef"`
	ProviderIDRef      int64  `json:"providerIDRef"`
	MerchantAccountRef int64  `json:"merchantAccountIdRef"`
	Priority           int    `json:"priority"`
	Enabled            bool   `json:"enabled"`
}

