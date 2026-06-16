package cashier

type PriceTemplate struct {
	ID             int64  `json:"id"`
	TemplateID     string `json:"templateId"`
	TemplateName   string `json:"templateName"`
	FXSyncEnabled  bool   `json:"fxSyncEnabled"`
	FXSyncMode     string `json:"fxSyncMode"`
	FXSyncSchedule string `json:"fxSyncSchedule"`
	Status         string `json:"status"`
}

type PriceRow struct {
	CountryCode         string `json:"countryCode"`
	RegionCode          string `json:"regionCode"`
	Currency            string `json:"currency"`
	PriceID             string `json:"priceId"`
	PreTaxAmountMinor   int64  `json:"preTaxAmountMinor"`
	TaxRate             string `json:"taxRate"`
	TaxAmountMinor      int64  `json:"taxAmountMinor"`
	AfterTaxAmountMinor int64  `json:"afterTaxAmountMinor"`
	EffectiveAt         string `json:"effectiveAt"`
}

