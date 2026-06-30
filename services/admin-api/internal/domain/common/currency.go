package common

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
)

type RoundingMode string

const (
	RoundingHalfUp   RoundingMode = "half_up"
	RoundingFloor    RoundingMode = "floor"
	RoundingCeil     RoundingMode = "ceil"
	RoundingTruncate RoundingMode = "truncate"
)

// CurrencySpec 是 currency_specs 的领域只读投影（00 §5）。
type CurrencySpec struct {
	CurrencyCode   string
	CurrencyName   string
	DecimalPlaces  int
	MinAmountMinor int64
	RoundingMode   RoundingMode
	Enabled        bool
}

var (
	ErrCurrencyFormat   = errors.New("invalid currency amount format")
	ErrCurrencyNegative = errors.New("currency amount must be non-negative")
)

// NormalizeAmountToMinor 把 major 金额归一化为 minor（00 §5），含下限校验；支持负数（cashier）。
func NormalizeAmountToMinor(majorAmount string, spec CurrencySpec) (int64, error) {
	amount := strings.TrimSpace(majorAmount)
	if amount == "" {
		return 0, fmt.Errorf("amount is required")
	}
	r := new(big.Rat)
	if _, ok := r.SetString(amount); !ok {
		return 0, fmt.Errorf("invalid amount: %s", amount)
	}

	scale := big.NewRat(int64(math.Pow10(spec.DecimalPlaces)), 1)
	scaled := new(big.Rat).Mul(r, scale)
	minor, err := roundRatToInt64(scaled, spec.RoundingMode)
	if err != nil {
		return 0, err
	}
	if minor < spec.MinAmountMinor {
		return 0, fmt.Errorf("amount below minimum")
	}
	return minor, nil
}

// NormalizeMajorAmount 把主单位金额字符串按币种精度与舍入规则归一化为 minor 整数（product）。
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
	scale := big.NewRat(int64(math.Pow10(spec.DecimalPlaces)), 1)
	scaled := new(big.Rat).Mul(r, scale)
	return roundRatToInt64Positive(scaled, spec.RoundingMode)
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

func roundRatToInt64(v *big.Rat, mode RoundingMode) (int64, error) {
	if v == nil {
		return 0, fmt.Errorf("nil amount")
	}
	n := new(big.Int).Set(v.Num())
	d := new(big.Int).Set(v.Denom())
	q := new(big.Int)
	r := new(big.Int)
	q.QuoRem(n, d, r)
	if r.Sign() == 0 {
		return q.Int64(), nil
	}

	absR := new(big.Int).Abs(r)
	absD := new(big.Int).Abs(d)
	sign := n.Sign()
	switch mode {
	case RoundingFloor:
		if sign < 0 {
			q.Sub(q, big.NewInt(1))
		}
	case RoundingCeil:
		if sign > 0 {
			q.Add(q, big.NewInt(1))
		}
	case RoundingTruncate:
		// toward zero: keep q
	case RoundingHalfUp:
		doubleR := new(big.Int).Mul(absR, big.NewInt(2))
		if doubleR.Cmp(absD) >= 0 {
			if sign > 0 {
				q.Add(q, big.NewInt(1))
			} else {
				q.Sub(q, big.NewInt(1))
			}
		}
	default:
		return 0, fmt.Errorf("unsupported rounding mode: %s", mode)
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("amount overflow")
	}
	return q.Int64(), nil
}

func roundRatToInt64Positive(v *big.Rat, mode RoundingMode) (int64, error) {
	if v.Sign() < 0 {
		return 0, ErrCurrencyNegative
	}
	num := new(big.Int).Set(v.Num())
	den := new(big.Int).Set(v.Denom())
	q := new(big.Int)
	r := new(big.Int)
	q.QuoRem(num, den, r)
	switch mode {
	case RoundingHalfUp:
		doubleR := new(big.Int).Mul(r, big.NewInt(2))
		if doubleR.Cmp(den) >= 0 {
			q.Add(q, big.NewInt(1))
		}
	case RoundingCeil:
		if r.Sign() > 0 {
			q.Add(q, big.NewInt(1))
		}
	case RoundingFloor, RoundingTruncate, "":
		// 正数下 floor/truncate 等价。
	default:
		return 0, fmt.Errorf("unsupported rounding mode: %s", mode)
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("amount overflow")
	}
	return q.Int64(), nil
}
