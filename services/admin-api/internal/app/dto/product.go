package dto

import "time"

type ListProductsQuery struct {
	GameID   string
	Keyword  string
	Enabled  *bool
	Page     int
	PageSize int
	Sort     string
}

type CreateProductCmd struct {
	GameID          string
	ProductID       string
	ProductName     string
	BaseCurrency    string
	BaseAmount      *string
	BaseAmountMinor *int64
	PriceID         string
	Enabled         *bool
}

type UpdateProductCmd struct {
	GameID          string
	ProductID       string
	ProductName     *string
	BaseCurrency    *string
	BaseAmount      *string
	BaseAmountMinor *int64
	PriceID         *string
	Enabled         *bool
}

type PutPackageProductsCmd struct {
	PackageID int64
	Items     []PutPackageProductItem
}

type PutPackageProductItem struct {
	ProductID         string
	Enabled           *bool
	ProductIDMode     string
	ProductIDOverride string
	PriceIDMode       string
	PriceIDOverride   string
}

type UpsertIAPConfigCmd struct {
	GameChannelID int64
	Enabled       *bool
	ConfigJSON    map[string]any
}

type UpsertPackageIAPOverrideCmd struct {
	PackageID  int64
	Enabled    *bool
	ConfigJSON map[string]any
}

type ProductView struct {
	ID                int64     `json:"id"`
	Env               string    `json:"env"`
	GameID            string    `json:"gameId"`
	ProductID         string    `json:"productId"`
	ProductName       string    `json:"productName"`
	BaseAmountMinor   int64     `json:"baseAmountMinor"`
	BaseCurrency      string    `json:"baseCurrency"`
	BaseAmountDisplay string    `json:"baseAmountDisplay"`
	PriceID           string    `json:"priceId"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type PackageProductView struct {
	ProductID         string            `json:"productId"`
	ProductName       string            `json:"productName"`
	Enabled           bool              `json:"enabled"`
	Base              PackageBaseView   `json:"base"`
	ProductIDMode     string            `json:"productIdMode"`
	ProductIDOverride string            `json:"productIdOverride"`
	PriceIDMode       string            `json:"priceIdMode"`
	PriceIDOverride   string            `json:"priceIdOverride"`
	Effective         PackageIDPairView `json:"effective"`
}

type PackageBaseView struct {
	ProductID       string `json:"productId"`
	PriceID         string `json:"priceId"`
	BaseAmountMinor int64  `json:"baseAmountMinor"`
	BaseCurrency    string `json:"baseCurrency"`
}

type PackageIDPairView struct {
	ProductID string `json:"productId"`
	PriceID   string `json:"priceId"`
}

type TemplateView struct {
	TemplateVersion string         `json:"templateVersion"`
	FormSchema      []any          `json:"formSchema"`
	SecretFields    []string       `json:"secretFields"`
	FileFields      []any          `json:"fileFields"`
	ValidationRules map[string]any `json:"validationRules"`
}

type IAPConfigView struct {
	Enabled          bool           `json:"enabled"`
	ConfigStatus     string         `json:"configStatus"`
	ConfigJSON       map[string]any `json:"configJson"`
	LastCheckAt      *time.Time     `json:"lastCheckAt"`
	LastCheckMessage string         `json:"lastCheckMessage"`
}

type GameChannelIAPConfigView struct {
	GameChannelID int64         `json:"gameChannelId"`
	ChannelID     string        `json:"channelId"`
	Template      TemplateView  `json:"template"`
	Config        IAPConfigView `json:"config"`
}

type PackageIAPOverrideView struct {
	PackageID   int64         `json:"packageId"`
	PackageCode string        `json:"packageCode"`
	ChannelID   string        `json:"channelId"`
	Template    TemplateView  `json:"template"`
	BaseConfig  IAPConfigView `json:"baseConfig"`
	Override    IAPConfigView `json:"override"`
}
