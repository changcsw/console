package dto

// CurrencySpecView 平台级 currency_specs 只读视图（字典下拉来源，00 §5.1）。
// 字段为 camelCase，与前端 stores/dictionary.ts ⇄ api/modules/products.ts CurrencySpec 对齐。
type CurrencySpecView struct {
	CurrencyCode   string `json:"currencyCode"`
	CurrencyName   string `json:"currencyName"`
	DecimalPlaces  int    `json:"decimalPlaces"`
	MinAmountMinor int64  `json:"minAmountMinor"`
	RoundingMode   string `json:"roundingMode"`
	Enabled        bool   `json:"enabled"`
}
