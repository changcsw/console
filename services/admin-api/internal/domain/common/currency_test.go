package common

import (
	"errors"
	"testing"
)

// 金额归一化纯逻辑（00 §5）单测：精度 / 最小值 / rounding。无 IO。
// 关键基准：USD(decimal=2,min=1,half_up) 4.999→500、0.001→拒绝(<min)；JPY(decimal=0) 120.5→121。

var (
	specUSD = CurrencySpec{CurrencyCode: "USD", DecimalPlaces: 2, MinAmountMinor: 1, RoundingMode: "half_up", Enabled: true}
	specJPY = CurrencySpec{CurrencyCode: "JPY", DecimalPlaces: 0, MinAmountMinor: 1, RoundingMode: "half_up", Enabled: true}
)

func TestNormalizeMajorAmount_USD_HalfUp(t *testing.T) {
	got, err := NormalizeMajorAmount("4.999", specUSD)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 500 {
		t.Fatalf("USD 4.999 应 half_up 归一为 500 minor，got %d", got)
	}
}

func TestNormalizeMajorAmount_USD_BelowHalf_RoundsDown(t *testing.T) {
	// 0.001 USD → 0.1 minor → half_up 落 0；下限拒绝由 NormalizeMinorAmount 承担。
	got, err := NormalizeMajorAmount("0.001", specUSD)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 0 {
		t.Fatalf("USD 0.001 归一应为 0 minor，got %d", got)
	}
	if _, err := NormalizeMinorAmount(got, specUSD); err == nil {
		t.Fatalf("0 minor 低于 min=1 应被 NormalizeMinorAmount 拒绝")
	}
}

func TestNormalizeMajorAmount_JPY_NoDecimal_HalfUp(t *testing.T) {
	got, err := NormalizeMajorAmount("120.5", specJPY)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 121 {
		t.Fatalf("JPY 120.5 应 half_up 归一为 121，got %d", got)
	}
}

func TestNormalizeMajorAmount_Exact(t *testing.T) {
	got, err := NormalizeMajorAmount("1.00", specUSD)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 100 {
		t.Fatalf("USD 1.00 应为 100 minor，got %d", got)
	}
}

func TestNormalizeMajorAmount_RoundingModes(t *testing.T) {
	floor := CurrencySpec{DecimalPlaces: 2, RoundingMode: "floor", Enabled: true}
	ceil := CurrencySpec{DecimalPlaces: 2, RoundingMode: "ceil", Enabled: true}
	truncate := CurrencySpec{DecimalPlaces: 2, RoundingMode: "truncate", Enabled: true}
	// 1.005 * 100 = 100.5 → floor=100, ceil=101, truncate=100, half_up=101
	cases := []struct {
		name string
		spec CurrencySpec
		want int64
	}{
		{"floor", floor, 100},
		{"ceil", ceil, 101},
		{"truncate", truncate, 100},
		{"half_up", CurrencySpec{DecimalPlaces: 2, RoundingMode: "half_up", Enabled: true}, 101},
	}
	for _, c := range cases {
		got, err := NormalizeMajorAmount("1.005", c.spec)
		if err != nil {
			t.Fatalf("%s: unexpected err: %v", c.name, err)
		}
		if got != c.want {
			t.Fatalf("%s: 1.005 want %d got %d", c.name, c.want, got)
		}
	}
}

func TestNormalizeMajorAmount_InvalidFormat(t *testing.T) {
	if _, err := NormalizeMajorAmount("abc", specUSD); !errors.Is(err, ErrCurrencyFormat) {
		t.Fatalf("非法格式应返回 ErrCurrencyFormat, got %v", err)
	}
	if _, err := NormalizeMajorAmount("  ", specUSD); !errors.Is(err, ErrCurrencyFormat) {
		t.Fatalf("空串应返回 ErrCurrencyFormat, got %v", err)
	}
}

func TestNormalizeMajorAmount_Negative(t *testing.T) {
	if _, err := NormalizeMajorAmount("-1.00", specUSD); !errors.Is(err, ErrCurrencyNegative) {
		t.Fatalf("负金额应返回 ErrCurrencyNegative, got %v", err)
	}
}

func TestNormalizeMajorAmount_UnsupportedRounding(t *testing.T) {
	bad := CurrencySpec{DecimalPlaces: 2, RoundingMode: "bankers", Enabled: true}
	if _, err := NormalizeMajorAmount("1.235", bad); err == nil {
		t.Fatalf("未知 rounding mode 应报错")
	}
}

func TestNormalizeMinorAmount_MinBoundary(t *testing.T) {
	// 等于 min 通过。
	if got, err := NormalizeMinorAmount(1, specUSD); err != nil || got != 1 {
		t.Fatalf("minor=min 应通过, got=%d err=%v", got, err)
	}
	// 低于 min 拒绝。
	if _, err := NormalizeMinorAmount(0, specUSD); err == nil {
		t.Fatalf("minor<min 应被拒绝")
	}
	// 负数拒绝。
	if _, err := NormalizeMinorAmount(-5, specUSD); !errors.Is(err, ErrCurrencyNegative) {
		t.Fatalf("负 minor 应返回 ErrCurrencyNegative, got %v", err)
	}
}

func TestFormatMinorAmount(t *testing.T) {
	cases := []struct {
		minor   int64
		decimal int
		want    string
	}{
		{500, 2, "5.00"},
		{499, 2, "4.99"},
		{1, 2, "0.01"},
		{121, 0, "121"},
		{0, 2, "0.00"},
		{-150, 2, "-1.50"},
	}
	for _, c := range cases {
		if got := FormatMinorAmount(c.minor, c.decimal); got != c.want {
			t.Fatalf("FormatMinorAmount(%d,%d) want %q got %q", c.minor, c.decimal, c.want, got)
		}
	}
}
