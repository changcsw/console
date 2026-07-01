package cashier

import (
	"fmt"
	"strings"
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type GameCashierProfile struct {
	ID                       int64
	GameIDRef                int64
	TemplateIDRef            int64
	AppliedTemplateVersionID int64
	SnapshotChecksum         string
	AppliedAt                time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type GameCashierPriceOverride struct {
	ID                  int64
	GameIDRef           int64
	CountryCode         string
	RegionCode          string
	Currency            string
	PriceID             string
	PreTaxAmountMinor   int64
	TaxRate             string
	TaxAmountMinor      int64
	AfterTaxAmountMinor int64
	Reason              string
	EffectiveAt         time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type GameCashier struct {
	Profile        *GameCashierProfile
	PriceOverrides []GameCashierPriceOverride
}

func NormalizePriceOverride(in GameCashierPriceOverride, spec common.CurrencySpec) (GameCashierPriceOverride, error) {
	norm := in
	norm.CountryCode = strings.ToUpper(strings.TrimSpace(in.CountryCode))
	norm.RegionCode = strings.ToUpper(strings.TrimSpace(in.RegionCode))
	if norm.RegionCode == "" {
		norm.RegionCode = "*"
	}
	norm.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	norm.PriceID = strings.TrimSpace(in.PriceID)
	norm.Reason = strings.TrimSpace(in.Reason)
	norm.TaxRate = strings.TrimSpace(in.TaxRate)
	if norm.TaxRate == "" {
		norm.TaxRate = "0"
	}

	if norm.CountryCode == "" || norm.Currency == "" || norm.PriceID == "" {
		return GameCashierPriceOverride{}, fmt.Errorf("countryCode/currency/priceId required")
	}
	if norm.EffectiveAt.IsZero() {
		return GameCashierPriceOverride{}, fmt.Errorf("effectiveAt required")
	}
	if _, err := common.NormalizeMinorAmount(norm.PreTaxAmountMinor, spec); err != nil {
		return GameCashierPriceOverride{}, err
	}
	if _, err := common.NormalizeMinorAmount(norm.TaxAmountMinor, spec); err != nil {
		return GameCashierPriceOverride{}, err
	}
	if _, err := common.NormalizeMinorAmount(norm.AfterTaxAmountMinor, spec); err != nil {
		return GameCashierPriceOverride{}, err
	}
	return norm, nil
}

func OverlayTemplateRows(snapshot []PriceRow, overrides []GameCashierPriceOverride) []PriceRow {
	merged := make([]PriceRow, 0, len(snapshot)+len(overrides))
	idx := make(map[string]int, len(snapshot))
	for _, row := range snapshot {
		key := row.CountryCode + "|" + row.RegionCode + "|" + row.Currency + "|" + row.PriceID
		idx[key] = len(merged)
		merged = append(merged, row)
	}
	for _, ov := range overrides {
		row := PriceRow{
			CountryCode:         ov.CountryCode,
			RegionCode:          ov.RegionCode,
			Currency:            ov.Currency,
			PriceID:             ov.PriceID,
			PreTaxAmountMinor:   ov.PreTaxAmountMinor,
			TaxRate:             ov.TaxRate,
			TaxAmountMinor:      ov.TaxAmountMinor,
			AfterTaxAmountMinor: ov.AfterTaxAmountMinor,
			EffectiveAt:         ov.EffectiveAt.UTC().Format(time.RFC3339),
		}
		key := ov.CountryCode + "|" + ov.RegionCode + "|" + ov.Currency + "|" + ov.PriceID
		if pos, ok := idx[key]; ok {
			merged[pos] = row
			continue
		}
		idx[key] = len(merged)
		merged = append(merged, row)
	}
	return merged
}
