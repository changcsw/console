package dashboard

import (
	"testing"
	"time"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	channeldomain "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestRangeToDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		in    string
		want  time.Duration
		valid bool
	}{
		{name: "24h", in: "24h", want: 24 * time.Hour, valid: true},
		{name: "7d", in: "7d", want: 7 * 24 * time.Hour, valid: true},
		{name: "30d", in: "30d", want: 30 * 24 * time.Hour, valid: true},
		{name: "90d", in: "90d", want: 90 * 24 * time.Hour, valid: true},
		{name: "trim and lower", in: "  7D  ", want: 7 * 24 * time.Hour, valid: true},
		{name: "invalid", in: "2d", want: 0, valid: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, ok := rangeToDuration(tc.in)
			if ok != tc.valid {
				t.Fatalf("valid want %v got %v", tc.valid, ok)
			}
			if got != tc.want {
				t.Fatalf("duration want %s got %s", tc.want, got)
			}
		})
	}
}

func TestNormalizeSummaryParams(t *testing.T) {
	t.Parallel()

	t.Run("normalizes defaults and accepts topN bounds", func(t *testing.T) {
		got, window, err := normalizeSummaryParams(dto.DashboardSummaryParams{Range: "", TopN: 0})
		if err != nil {
			t.Fatalf("normalize defaults: %v", err)
		}
		if got.Range != "7d" || got.TopN != 5 {
			t.Fatalf("want default range/topN = 7d/5, got %s/%d", got.Range, got.TopN)
		}
		if window != 7*24*time.Hour {
			t.Fatalf("want 7d window, got %s", window)
		}

		for _, topN := range []int{1, 20} {
			got, _, err := normalizeSummaryParams(dto.DashboardSummaryParams{Range: "24h", TopN: topN})
			if err != nil {
				t.Fatalf("normalize topN=%d: %v", topN, err)
			}
			if got.TopN != topN {
				t.Fatalf("topN want %d got %d", topN, got.TopN)
			}
		}
	})

	t.Run("rejects out-of-range topN", func(t *testing.T) {
		_, _, err := normalizeSummaryParams(dto.DashboardSummaryParams{Range: "7d", TopN: 21})
		if err == nil {
			t.Fatal("want topN error for >20")
		}
	})

	t.Run("rejects invalid range", func(t *testing.T) {
		_, _, err := normalizeSummaryParams(dto.DashboardSummaryParams{Range: "2w", TopN: 5})
		if err == nil {
			t.Fatal("want range validation error")
		}
	})
}

func TestChannelCompatibilityMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		market common.Market
		region channeldomain.ChannelRegion
		want   bool
	}{
		{name: "CN + domestic compatible", market: common.MarketCN, region: channeldomain.ChannelRegionDomestic, want: true},
		{name: "CN + overseas incompatible", market: common.MarketCN, region: channeldomain.ChannelRegionOverseas, want: false},
		{name: "JP + overseas compatible", market: common.MarketJP, region: channeldomain.ChannelRegionOverseas, want: true},
		{name: "JP + domestic incompatible", market: common.MarketJP, region: channeldomain.ChannelRegionDomestic, want: false},
		{name: "GLOBAL + overseas compatible", market: common.MarketGlobal, region: channeldomain.ChannelRegionOverseas, want: true},
		{name: "GLOBAL + domestic incompatible", market: common.MarketGlobal, region: channeldomain.ChannelRegionDomestic, want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := channeldomain.IsCompatible(tc.market, tc.region)
			if got != tc.want {
				t.Fatalf("compatibility want %v got %v", tc.want, got)
			}
		})
	}
}
