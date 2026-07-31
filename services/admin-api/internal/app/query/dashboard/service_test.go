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
		{name: "CN + CN compatible", market: common.MarketCN, region: channeldomain.ChannelRegionCN, want: true},
		{name: "CN + GLOBAL incompatible", market: common.MarketCN, region: channeldomain.ChannelRegionGlobal, want: false},
		{name: "JP + GLOBAL compatible", market: common.MarketJP, region: channeldomain.ChannelRegionGlobal, want: true},
		{name: "JP + JP compatible", market: common.MarketJP, region: channeldomain.ChannelRegionJP, want: true},
		{name: "JP + CN incompatible", market: common.MarketJP, region: channeldomain.ChannelRegionCN, want: false},
		{name: "JP + KR incompatible", market: common.MarketJP, region: channeldomain.ChannelRegionKR, want: false},
		{name: "GLOBAL + GLOBAL compatible", market: common.MarketGlobal, region: channeldomain.ChannelRegionGlobal, want: true},
		{name: "GLOBAL + CN incompatible", market: common.MarketGlobal, region: channeldomain.ChannelRegionCN, want: false},
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
