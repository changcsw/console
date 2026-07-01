package cashier

import (
	"testing"
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestNormalizePriceOverride(t *testing.T) {
	spec := common.CurrencySpec{
		CurrencyCode:   "USD",
		DecimalPlaces:  2,
		MinAmountMinor: 1,
		RoundingMode:   common.RoundingHalfUp,
		Enabled:        true,
	}
	in := GameCashierPriceOverride{
		CountryCode:         " us ",
		RegionCode:          "",
		Currency:            " usd ",
		PriceID:             " p1 ",
		PreTaxAmountMinor:   100,
		TaxRate:             "",
		TaxAmountMinor:      10,
		AfterTaxAmountMinor: 110,
		EffectiveAt:         time.Now(),
	}
	got, err := NormalizePriceOverride(in, spec)
	if err != nil {
		t.Fatalf("NormalizePriceOverride err=%v", err)
	}
	if got.CountryCode != "US" || got.RegionCode != "*" || got.Currency != "USD" || got.PriceID != "p1" {
		t.Fatalf("normalized key fields mismatch: %+v", got)
	}
	if got.TaxRate != "0" {
		t.Fatalf("expected default taxRate=0, got %q", got.TaxRate)
	}
}

// usdSpec 是单测共用的 USD 币种规格（2 位小数，最小 50 minor）。
func usdSpec() common.CurrencySpec {
	return common.CurrencySpec{
		CurrencyCode:   "USD",
		DecimalPlaces:  2,
		MinAmountMinor: 50,
		RoundingMode:   common.RoundingHalfUp,
		Enabled:        true,
	}
}

func baseOverride() GameCashierPriceOverride {
	return GameCashierPriceOverride{
		CountryCode:         "US",
		RegionCode:          "*",
		Currency:            "USD",
		PriceID:             "p1",
		PreTaxAmountMinor:   1000,
		TaxRate:             "0.1",
		TaxAmountMinor:      100,
		AfterTaxAmountMinor: 1100,
		EffectiveAt:         time.Now(),
	}
}

// S4 边界：金额低于 currency_specs 下限 → NormalizeMinorAmount 报错（amount below min）。
func TestNormalizePriceOverrideBelowMinimum(t *testing.T) {
	in := baseOverride()
	in.PreTaxAmountMinor = 10 // < MinAmountMinor(50)
	if _, err := NormalizePriceOverride(in, usdSpec()); err == nil {
		t.Fatal("expected error for amount below minimum, got nil")
	}
}

// S4 边界：金额为负 → 归一化报错。
func TestNormalizePriceOverrideNegativeAmount(t *testing.T) {
	in := baseOverride()
	in.TaxAmountMinor = -1
	if _, err := NormalizePriceOverride(in, usdSpec()); err == nil {
		t.Fatal("expected error for negative amount, got nil")
	}
}

// S4 边界：缺必填键字段（country/currency/priceId）→ 报错。
func TestNormalizePriceOverrideMissingKeyFields(t *testing.T) {
	for _, mut := range []func(*GameCashierPriceOverride){
		func(o *GameCashierPriceOverride) { o.CountryCode = "  " },
		func(o *GameCashierPriceOverride) { o.Currency = "" },
		func(o *GameCashierPriceOverride) { o.PriceID = "   " },
	} {
		in := baseOverride()
		mut(&in)
		if _, err := NormalizePriceOverride(in, usdSpec()); err == nil {
			t.Fatalf("expected error for missing key field, input=%+v", in)
		}
	}
}

// S4 边界：effectiveAt 为零值 → 报错。
func TestNormalizePriceOverrideMissingEffectiveAt(t *testing.T) {
	in := baseOverride()
	in.EffectiveAt = time.Time{}
	if _, err := NormalizePriceOverride(in, usdSpec()); err == nil {
		t.Fatal("expected error for zero effectiveAt, got nil")
	}
}

// 归一化：大小写与空白被规范，区域空 → '*'，税率空 → '0'。
func TestNormalizePriceOverrideNormalizesCasingAndDefaults(t *testing.T) {
	in := baseOverride()
	in.CountryCode = " us "
	in.Currency = " usd "
	in.RegionCode = ""
	in.PriceID = " p1 "
	in.TaxRate = ""
	got, err := NormalizePriceOverride(in, usdSpec())
	if err != nil {
		t.Fatalf("normalize err=%v", err)
	}
	if got.CountryCode != "US" || got.Currency != "USD" || got.RegionCode != "*" || got.PriceID != "p1" || got.TaxRate != "0" {
		t.Fatalf("normalization mismatch: %+v", got)
	}
}

func TestOverlayTemplateRows(t *testing.T) {
	snapshot := []PriceRow{
		{CountryCode: "US", RegionCode: "*", Currency: "USD", PriceID: "A", PreTaxAmountMinor: 100},
		{CountryCode: "JP", RegionCode: "*", Currency: "JPY", PriceID: "B", PreTaxAmountMinor: 200},
	}
	overrides := []GameCashierPriceOverride{
		{CountryCode: "US", RegionCode: "*", Currency: "USD", PriceID: "A", PreTaxAmountMinor: 150, EffectiveAt: time.Now()},
		{CountryCode: "KR", RegionCode: "*", Currency: "KRW", PriceID: "C", PreTaxAmountMinor: 300, EffectiveAt: time.Now()},
	}
	got := OverlayTemplateRows(snapshot, overrides)
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(got))
	}
	if got[0].PreTaxAmountMinor != 150 {
		t.Fatalf("override row should replace snapshot row")
	}
}

// 整行覆盖语义：同键覆盖替换「整行」而非字段级深合并
// （即模板行的非覆盖字段不会被保留）。
func TestOverlayTemplateRowsWholeRowReplace(t *testing.T) {
	snapshot := []PriceRow{
		{CountryCode: "US", RegionCode: "*", Currency: "USD", PriceID: "A", PreTaxAmountMinor: 100, TaxAmountMinor: 10, AfterTaxAmountMinor: 110, TaxRate: "0.1"},
	}
	overrides := []GameCashierPriceOverride{
		// 仅给 PreTax，税额留空 → 整行覆盖后税额应为 0（不沿用模板的 10/110）。
		{CountryCode: "US", RegionCode: "*", Currency: "USD", PriceID: "A", PreTaxAmountMinor: 150, EffectiveAt: time.Now()},
	}
	got := OverlayTemplateRows(snapshot, overrides)
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if got[0].PreTaxAmountMinor != 150 || got[0].TaxAmountMinor != 0 || got[0].AfterTaxAmountMinor != 0 {
		t.Fatalf("whole-row override expected (150,0,0), got (%d,%d,%d)",
			got[0].PreTaxAmountMinor, got[0].TaxAmountMinor, got[0].AfterTaxAmountMinor)
	}
}

// 空覆盖集 → 原样返回模板快照行。
func TestOverlayTemplateRowsEmptyOverrides(t *testing.T) {
	snapshot := []PriceRow{
		{CountryCode: "US", RegionCode: "*", Currency: "USD", PriceID: "A", PreTaxAmountMinor: 100},
	}
	got := OverlayTemplateRows(snapshot, nil)
	if len(got) != 1 || got[0].PreTaxAmountMinor != 100 {
		t.Fatalf("empty overrides should return snapshot unchanged, got %+v", got)
	}
}

// 覆盖键区分 region：同 country/currency/price 但不同 region 视为不同键，不互相覆盖。
func TestOverlayTemplateRowsRegionDistinguishesKey(t *testing.T) {
	snapshot := []PriceRow{
		{CountryCode: "US", RegionCode: "*", Currency: "USD", PriceID: "A", PreTaxAmountMinor: 100},
	}
	overrides := []GameCashierPriceOverride{
		{CountryCode: "US", RegionCode: "CA", Currency: "USD", PriceID: "A", PreTaxAmountMinor: 200, EffectiveAt: time.Now()},
	}
	got := OverlayTemplateRows(snapshot, overrides)
	if len(got) != 2 {
		t.Fatalf("different region must not override; expected 2 rows, got %d", len(got))
	}
}
