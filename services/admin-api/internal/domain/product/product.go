package product

type Product struct {
	ID              int64  `json:"id"`
	GameIDRef       int64  `json:"gameIdRef"`
	ProductID       string `json:"productId"`
	ProductName     string `json:"productName"`
	BaseAmountMinor int64  `json:"baseAmountMinor"`
	BaseCurrency    string `json:"baseCurrency"`
	PriceID         string `json:"priceId"`
	Enabled         bool   `json:"enabled"`
}

type ChannelProduct struct {
	ID                int64  `json:"id"`
	ProductIDRef      int64  `json:"productIdRef"`
	PackageIDRef      int64  `json:"packageIdRef"`
	ProductIDMode     string `json:"productIdMode"`
	ProductIDOverride string `json:"productIdOverride"`
	PriceIDMode       string `json:"priceIdMode"`
	PriceIDOverride   string `json:"priceIdOverride"`
	Enabled           bool   `json:"enabled"`
}

