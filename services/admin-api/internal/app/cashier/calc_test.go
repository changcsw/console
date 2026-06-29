package cashier

import "testing"

// calcTaxAmount：按 preTaxMinor * taxRate 计算税额（minor），半进位舍入；纯函数无 IO。
func TestCalcTaxAmount(t *testing.T) {
	cases := []struct {
		name    string
		preTax  int64
		rate    string
		want    int64
		wantErr bool
	}{
		{"ten_percent_exact", 1000, "0.1", 100, false},
		{"zero_rate", 1000, "0", 0, false},
		{"round_down", 1234, "0.1", 123, false},    // 123.4 → 123
		{"round_up_half", 1235, "0.1", 124, false}, // 123.5 → 124
		{"fractional_rate", 1000, "0.085", 85, false},
		{"negative_rate_away_from_zero", 1235, "-0.1", -124, false}, // -123.5 → -124
		{"large_value", 100000000, "0.2", 20000000, false},
		{"invalid_rate", 1000, "abc", 0, true},
		{"empty_rate", 1000, "", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := calcTaxAmount(c.preTax, c.rate)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for rate %q", c.rate)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("calcTaxAmount(%d,%q)=%d want %d", c.preTax, c.rate, got, c.want)
			}
		})
	}
}
