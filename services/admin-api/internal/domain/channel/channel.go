package channel

type Channel struct {
	ID          int64  `json:"id"`
	ChannelID   string `json:"channelId"`
	ChannelName string `json:"channelName"`
	ChannelType string `json:"channelType"`
	Enabled     bool   `json:"enabled"`
	Sort        int    `json:"sort"`
}

type Policy struct {
	ChannelIDRef int64  `json:"channelIdRef"`
	LoginMode    string `json:"loginMode"`
	PaymentMode  string `json:"paymentMode"`
	LoginLocked  bool   `json:"loginLocked"`
	PaymentLocked bool  `json:"paymentLocked"`
}

type Package struct {
	ID                   int64  `json:"id"`
	PackageCode          string `json:"packageCode"`
	PackageName          string `json:"packageName"`
	MarketCode           string `json:"marketCode"`
	BundleID             string `json:"bundleId"`
	InheritChannelConfig bool   `json:"inheritChannelConfig"`
	Enabled              bool   `json:"enabled"`
}

