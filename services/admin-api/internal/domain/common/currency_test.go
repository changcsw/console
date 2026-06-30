package common

import (
	"errors"
	"testing"
)

func specUSD(min int64, mode RoundingMode) CurrencySpec {
	return CurrencySpec{CurrencyCode: "USD", DecimalPlaces: 2, MinAmountMinor: min, RoundingMode: mode, Enabled: true}
}

// 精度 + 各舍入模式正向归一化（00 §5）。
func TestNormalizeAmountToMinorRounding(t *testing.T) {
	cases := []struct {
		name  string
		major string
		spec  CurrencySpec
		want  int64
	}{
		{"half_up_down", "10.004", specUSD(0, RoundingHalfUp), 1000},
		{"half_up_at_boundary", "10.005", specUSD(0, RoundingHalfUp), 1001},
		{"half_up_up", "10.006", specUSD(0, RoundingHalfUp), 1001},
		{"exact_no_round", "10.00", specUSD(0, RoundingHalfUp), 1000},
		{"floor_positive", "1.999", specUSD(0, RoundingFloor), 199},
		{"ceil_positive", "1.001", specUSD(0, RoundingCeil), 101},
		{"truncate_positive", "1.999", specUSD(0, RoundingTruncate), 199},
		{"floor_negative_away", "-1.001", specUSD(-100000, RoundingFloor), -101},
		{"ceil_negative_toward", "-1.999", specUSD(-100000, RoundingCeil), -199},
		{"truncate_negative_toward", "-1.999", specUSD(-100000, RoundingTruncate), -199},
		{"half_up_negative_away", "-10.005", specUSD(-100000, RoundingHalfUp), -1001},
		{"jpy_zero_decimals", "100", CurrencySpec{CurrencyCode: "JPY", DecimalPlaces: 0, MinAmountMinor: 1, RoundingMode: RoundingHalfUp, Enabled: true}, 100},
		{"jpy_round", "100.5", CurrencySpec{CurrencyCode: "JPY", DecimalPlaces: 0, MinAmountMinor: 1, RoundingMode: RoundingHalfUp, Enabled: true}, 101},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeAmountToMinor(c.major, c.spec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("NormalizeAmountToMinor(%q)=%d want %d", c.major, got, c.want)
			}
		})
	}
}

func TestNormalizeAmountToMinorBelowMinimum(t *testing.T) {
	if _, err := NormalizeAmountToMinor("0.10", specUSD(50, RoundingHalfUp)); err == nil {
		t.Fatal("expected below-minimum error")
	}
	if got, err := NormalizeAmountToMinor("0.50", specUSD(50, RoundingHalfUp)); err != nil || got != 50 {
		t.Fatalf("at-minimum should pass: got=%d err=%v", got, err)
	}
}

func TestNormalizeAmountToMinorInvalid(t *testing.T) {
	cases := []struct {
		name  string
		major string
		spec  CurrencySpec
	}{
		{"empty", "", specUSD(0, RoundingHalfUp)},
		{"blank", "   ", specUSD(0, RoundingHalfUp)},
		{"not_a_number", "abc", specUSD(0, RoundingHalfUp)},
		{"unsupported_rounding", "1.234", CurrencySpec{CurrencyCode: "USD", DecimalPlaces: 2, RoundingMode: RoundingMode("bogus"), Enabled: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NormalizeAmountToMinor(c.major, c.spec); err == nil {
				t.Fatalf("expected error for %q", c.major)
			}
		})
	}
}

var (
	majorSpecUSD = CurrencySpec{CurrencyCode: "USD", DecimalPlaces: 2, MinAmountMinor: 1, RoundingMode: RoundingHalfUp, Enabled: true}
	majorSpecJPY = CurrencySpec{CurrencyCode: "JPY", DecimalPlaces: 0, MinAmountMinor: 1, RoundingMode: RoundingHalfUp, Enabled: true}
)

func TestNormalizeMajorAmount_USD_HalfUp(t *testing.T) {
	got, err := NormalizeMajorAmount("4.999", majorSpecUSD)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 500 {
		t.Fatalf("USD 4.999 应 half_up 归一为 500 minor，got %d", got)
	}
}

func TestNormalizeMajorAmount_USD_BelowHalf_RoundsDown(t *testing.T) {
	got, err := NormalizeMajorAmount("0.001", majorSpecUSD)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 0 {
		t.Fatalf("USD 0.001 归一应为 0 minor，got %d", got)
	}
	if _, err := NormalizeMinorAmount(got, majorSpecUSD); err == nil {
		t.Fatalf("0 minor 低于 min=1 应被 NormalizeMinorAmount 拒绝")
	}
}

func TestNormalizeMajorAmount_JPY_NoDecimal_HalfUp(t *testing.T) {
	got, err := NormalizeMajorAmount("120.5", majorSpecJPY)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 121 {
		t.Fatalf("JPY 120.5 应 half_up 归一为 121，got %d", got)
	}
}

func TestNormalizeMajorAmount_Exact(t *testing.T) {
	got, err := NormalizeMajorAmount("1.00", majorSpecUSD)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != 100 {
		t.Fatalf("USD 1.00 应为 100 minor，got %d", got)
	}
}

func TestNormalizeMajorAmount_RoundingModes(t *testing.T) {
	floor := CurrencySpec{DecimalPlaces: 2, RoundingMode: RoundingFloor, Enabled: true}
	ceil := CurrencySpec{DecimalPlaces: 2, RoundingMode: RoundingCeil, Enabled: true}
	truncate := CurrencySpec{DecimalPlaces: 2, RoundingMode: RoundingTruncate, Enabled: true}
	cases := []struct {
		name string
		spec CurrencySpec
		want int64
	}{
		{"floor", floor, 100},
		{"ceil", ceil, 101},
		{"truncate", truncate, 100},
		{"half_up", CurrencySpec{DecimalPlaces: 2, RoundingMode: RoundingHalfUp, Enabled: true}, 101},
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
	if _, err := NormalizeMajorAmount("abc", majorSpecUSD); !errors.Is(err, ErrCurrencyFormat) {
		t.Fatalf("非法格式应返回 ErrCurrencyFormat, got %v", err)
	}
	if _, err := NormalizeMajorAmount("  ", majorSpecUSD); !errors.Is(err, ErrCurrencyFormat) {
		t.Fatalf("空串应返回 ErrCurrencyFormat, got %v", err)
	}
}

func TestNormalizeMajorAmount_Negative(t *testing.T) {
	if _, err := NormalizeMajorAmount("-1.00", majorSpecUSD); !errors.Is(err, ErrCurrencyNegative) {
		t.Fatalf("负金额应返回 ErrCurrencyNegative, got %v", err)
	}
}

func TestNormalizeMajorAmount_UnsupportedRounding(t *testing.T) {
	bad := CurrencySpec{DecimalPlaces: 2, RoundingMode: RoundingMode("bankers"), Enabled: true}
	if _, err := NormalizeMajorAmount("1.235", bad); err == nil {
		t.Fatalf("未知 rounding mode 应报错")
	}
}

func TestNormalizeMinorAmount_MinBoundary(t *testing.T) {
	if got, err := NormalizeMinorAmount(1, majorSpecUSD); err != nil || got != 1 {
		t.Fatalf("minor=min 应通过, got=%d err=%v", got, err)
	}
	if _, err := NormalizeMinorAmount(0, majorSpecUSD); err == nil {
		t.Fatalf("minor<min 应被拒绝")
	}
	if _, err := NormalizeMinorAmount(-5, majorSpecUSD); !errors.Is(err, ErrCurrencyNegative) {
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
