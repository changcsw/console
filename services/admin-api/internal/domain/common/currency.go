package common

import (
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

type CurrencySpec struct {
	CurrencyCode   string
	DecimalPlaces  int
	MinAmountMinor int64
	RoundingMode   RoundingMode
	Enabled        bool
}

// NormalizeAmountToMinor 把 major 金额归一化为 minor（00 §5）。
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
