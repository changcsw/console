package plugin

import "testing"

// ValidatePluginRegionCompatibility 纯规则（无 IO）：
// market=CN ⇒ region=domestic；market!=CN（且已知）⇒ region=overseas；未知 market / 未知 region ⇒ false。
// 与 12-channel §5.1 同源，服务端强制。
func TestValidatePluginRegionCompatibility(t *testing.T) {
	cases := []struct {
		name   string
		market string
		region string
		want   bool
	}{
		// CN 仅兼容 domestic
		{"cn_domestic_ok", "CN", RegionDomestic, true},
		{"cn_overseas_reject", "CN", RegionOverseas, false},
		// 非 CN（已知）仅兼容 overseas
		{"global_overseas_ok", "GLOBAL", RegionOverseas, true},
		{"jp_overseas_ok", "JP", RegionOverseas, true},
		{"kr_overseas_ok", "KR", RegionOverseas, true},
		{"sea_overseas_ok", "SEA", RegionOverseas, true},
		{"hmt_overseas_ok", "HMT", RegionOverseas, true},
		{"jp_domestic_reject", "JP", RegionDomestic, false},
		{"global_domestic_reject", "GLOBAL", RegionDomestic, false},
		// 未知 market ⇒ false（即使 region 合法）
		{"unknown_market_overseas", "US", RegionOverseas, false},
		{"unknown_market_domestic", "US", RegionDomestic, false},
		{"empty_market", "", RegionDomestic, false},
		// 未知 region ⇒ false（即使 market 合法）
		{"cn_unknown_region", "CN", "", false},
		{"cn_bogus_region", "CN", "regional", false},
		{"jp_empty_region", "JP", "", false},
		// 大小写敏感：region 必须精确匹配小写常量
		{"region_case_sensitive", "CN", "Domestic", false},
		// market 大小写敏感（common.Market 区分大小写）
		{"market_case_sensitive", "cn", RegionDomestic, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidatePluginRegionCompatibility(c.market, c.region); got != c.want {
				t.Fatalf("ValidatePluginRegionCompatibility(%q,%q)=%v want %v", c.market, c.region, got, c.want)
			}
		})
	}
}
