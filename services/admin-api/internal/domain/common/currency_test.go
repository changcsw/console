package common

import "testing"

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
		// 0 位小数币种（JPY 形态）。
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

// 下限校验：归一化结果低于 MinAmountMinor → 报错（上游映射 VALIDATION_FAILED）。
func TestNormalizeAmountToMinorBelowMinimum(t *testing.T) {
	if _, err := NormalizeAmountToMinor("0.10", specUSD(50, RoundingHalfUp)); err == nil {
		t.Fatal("expected below-minimum error")
	}
	// 恰好等于下限不报错。
	if got, err := NormalizeAmountToMinor("0.50", specUSD(50, RoundingHalfUp)); err != nil || got != 50 {
		t.Fatalf("at-minimum should pass: got=%d err=%v", got, err)
	}
}

// 输入/配置非法路径。
func TestNormalizeAmountToMinorInvalid(t *testing.T) {
	cases := []struct {
		name  string
		major string
		spec  CurrencySpec
	}{
		{"empty", "", specUSD(0, RoundingHalfUp)},
		{"blank", "   ", specUSD(0, RoundingHalfUp)},
		{"not_a_number", "abc", specUSD(0, RoundingHalfUp)},
		// 需带小数余项才会进入舍入分支，从而命中非法 mode。
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
