package snapshot

import (
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

const (
	ConfigSchemaVersion = "1.0"
	SecretMaskedValue   = "***"
)

type SnapshotStatus string

const (
	StatusDraft     SnapshotStatus = "draft"
	StatusPublished SnapshotStatus = "published"
)

type ConfigSnapshot struct {
	ID                  int64
	GameIDRef           int64
	ConfigSchemaVersion string
	ConfigVersion       string
	ConfigJSON          map[string]any
	FileName            string
	FileHash            string
	StorageKey          string
	Status              SnapshotStatus
	GeneratedAt         time.Time
	PublishedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ScopeField struct {
	Key   string `json:"key"`
	Scope string `json:"scope"`
}

type LegalLink struct {
	ScopeType        string `json:"scopeType"`
	ScopeValue       string `json:"scopeValue"`
	TermsURL         string `json:"termsUrl"`
	PrivacyURL       string `json:"privacyUrl"`
	DeleteAccountURL string `json:"deleteAccountUrl"`
}

type AccountAuthItem struct {
	AuthTypeID   string
	Config       map[string]any
	FormSchema   []ScopeField
	SecretFields []string
}

type ProductItem struct {
	ProductID        string `json:"productId"`
	EffectivePriceID string `json:"priceId"`
	Currency         string `json:"currency"`
	AmountMinor      int64  `json:"amountMinor"`
}

type CashierProfile struct {
	TemplateID       string `json:"templateId"`
	TemplateVersion  int    `json:"templateVersion"`
	SnapshotChecksum string `json:"snapshotChecksum"`
}

type TemplateConfig struct {
	Enabled      bool
	ConfigStatus common.ConfigStatus
	Config       map[string]any
	FormSchema   []ScopeField
	SecretFields []string
}

type PluginConfig struct {
	PluginID      string
	PluginName    string
	Region        string
	Required      bool
	Enabled       bool
	ConfigStatus  common.ConfigStatus
	Config        map[string]any
	FormSchema    []ScopeField
	SecretFields  []string
	UpdatedAtUnix int64
}

type PackageConfig struct {
	PackageCode string `json:"packageCode"`
	BundleID    string `json:"bundleId"`
	Enabled     bool   `json:"enabled"`
}

type ChannelInput struct {
	ChannelID    string
	Region       string
	Market       common.Market
	Hidden       bool
	Enabled      bool
	ConfigStatus common.ConfigStatus
	Login        *TemplateConfig
	IAP          *TemplateConfig
	Packages     []PackageConfig
	Plugins      []PluginConfig
}

type ResolvedRoute struct {
	PayWay          string `json:"payWay"`
	Provider        string `json:"provider"`
	MerchantAccount string `json:"merchantAccount"`
}

type ValidDataView struct {
	GameID        string
	GameIDRef     int64
	GeneratedAt   time.Time
	LegalLinks    []LegalLink
	AccountAuth   []AccountAuthItem
	Products      []ProductItem
	Cashier       *CashierProfile
	Channels      []ChannelInput
	PaymentRoutes map[common.Market][]ResolvedRoute
}

type RuntimeConfig struct {
	SchemaVersion string                  `json:"schemaVersion"`
	GameID        string                  `json:"gameId"`
	GeneratedAt   string                  `json:"generatedAt"`
	Markets       map[string]MarketConfig `json:"markets"`
}

type MarketConfig struct {
	Game          GameBaseConfig    `json:"game"`
	Channels      []ResolvedChannel `json:"channels"`
	PaymentRoutes []ResolvedRoute   `json:"paymentRoutes"`
}

type GameBaseConfig struct {
	LegalLinks  []LegalLink      `json:"legalLinks"`
	AccountAuth []map[string]any `json:"accountAuth"`
	Products    []ProductItem    `json:"products"`
	Cashier     map[string]any   `json:"cashier,omitempty"`
	Extras      map[string]any   `json:"extras,omitempty"`
}

type ResolvedChannel struct {
	ChannelID    string           `json:"channelId"`
	Region       string           `json:"region"`
	SourceMarket string           `json:"sourceMarket"`
	Login        map[string]any   `json:"login,omitempty"`
	IAP          map[string]any   `json:"iap,omitempty"`
	Plugins      []map[string]any `json:"plugins,omitempty"`
	Packages     []PackageConfig  `json:"packages,omitempty"`
}
