package common

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// CurrencySpec 是 currency_specs 的领域只读投影（00 §5）。
type CurrencySpec struct {
	CurrencyCode   string
	CurrencyName   string
	DecimalPlaces  int
	MinAmountMinor int64
	RoundingMode   string
	Enabled        bool
}

var (
	ErrCurrencyFormat   = errors.New("invalid currency amount format")
	ErrCurrencyNegative = errors.New("currency amount must be non-negative")
)

// NormalizeMajorAmount 把主单位金额字符串按币种精度与舍入规则归一化为 minor 整数。
func NormalizeMajorAmount(amount string, spec CurrencySpec) (int64, error) {
	trimmed := strings.TrimSpace(amount)
	if trimmed == "" {
		return 0, ErrCurrencyFormat
	}
	r := new(big.Rat)
	if _, ok := r.SetString(trimmed); !ok {
		return 0, ErrCurrencyFormat
	}
	if r.Sign() < 0 {
		return 0, ErrCurrencyNegative
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(spec.DecimalPlaces)), nil)
	scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(scale))
	return roundRatToInt64(scaled, spec.RoundingMode)
}

// NormalizeMinorAmount 校验 minor 金额并执行最小值校验。
func NormalizeMinorAmount(amountMinor int64, spec CurrencySpec) (int64, error) {
	if amountMinor < 0 {
		return 0, ErrCurrencyNegative
	}
	if amountMinor < spec.MinAmountMinor {
		return 0, fmt.Errorf("amount below min_amount_minor(%d)", spec.MinAmountMinor)
	}
	return amountMinor, nil
}

// FormatMinorAmount 把 minor 整数按 decimal_places 格式化为主单位字符串。
func FormatMinorAmount(amountMinor int64, decimalPlaces int) string {
	if decimalPlaces <= 0 {
		return fmt.Sprintf("%d", amountMinor)
	}
	sign := ""
	if amountMinor < 0 {
		sign = "-"
		amountMinor = -amountMinor
	}
	scale := int64(1)
	for i := 0; i < decimalPlaces; i++ {
		scale *= 10
	}
	intPart := amountMinor / scale
	fracPart := amountMinor % scale
	return fmt.Sprintf("%s%d.%0*d", sign, intPart, decimalPlaces, fracPart)
}

func roundRatToInt64(v *big.Rat, mode string) (int64, error) {
	if v.Sign() < 0 {
		return 0, ErrCurrencyNegative
	}
	num := new(big.Int).Set(v.Num())
	den := new(big.Int).Set(v.Denom())
	q := new(big.Int)
	r := new(big.Int)
	q.QuoRem(num, den, r)
	switch mode {
	case "half_up":
		twiceR := new(big.Int).Lsh(r, 1)
		if twiceR.Cmp(den) >= 0 {
			q = q.Add(q, big.NewInt(1))
		}
	case "ceil":
		if r.Sign() > 0 {
			q = q.Add(q, big.NewInt(1))
		}
	case "floor", "truncate", "":
		// 正数下 floor/truncate 等价。
	default:
		return 0, fmt.Errorf("unsupported rounding mode: %s", mode)
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("amount overflow")
	}
	return q.Int64(), nil
}
