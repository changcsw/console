package payment

import (
	"testing"
)

func TestMarketMatchesRules(t *testing.T) {
	cases := []struct {
		name        string
		routeMarket string
		inputMarket string
		want        bool
	}{
		{name: "CN only matches CN", routeMarket: "CN", inputMarket: "CN", want: true},
		{name: "CN does not use GLOBAL fallback", routeMarket: "GLOBAL", inputMarket: "CN", want: false},
		{name: "JP matches GLOBAL fallback", routeMarket: "GLOBAL", inputMarket: "JP", want: true},
		{name: "JP matches itself", routeMarket: "JP", inputMarket: "JP", want: true},
		{name: "GLOBAL input only matches GLOBAL", routeMarket: "JP", inputMarket: "GLOBAL", want: false},
		{name: "GLOBAL input matches wildcard", routeMarket: "*", inputMarket: "GLOBAL", want: true},
		{name: "wildcard route matches any", routeMarket: "*", inputMarket: "KR", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := marketMatches(tc.routeMarket, tc.inputMarket)
			if got != tc.want {
				t.Fatalf("marketMatches(%q,%q) want %v got %v", tc.routeMarket, tc.inputMarket, tc.want, got)
			}
		})
	}
}

func TestPickBestRouteDecisionOrder(t *testing.T) {
	t.Run("package exact beats wildcard", func(t *testing.T) {
		routes := []Route{
			{ID: 1, PayWay: "card", Package: "", Market: "JP", Priority: 1, Enabled: true},
			{ID: 2, PayWay: "card", Package: "pkg.a", Market: "JP", Priority: 99, Enabled: true},
		}
		best, err := PickBestRoute(routes, MatchInput{PayWay: "card", Package: "pkg.a", Market: "JP"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if best.ID != 2 {
			t.Fatalf("want ID 2 got %d", best.ID)
		}
	})

	t.Run("specific market beats global", func(t *testing.T) {
		routes := []Route{
			{ID: 1, PayWay: "card", Market: "GLOBAL", Priority: 1, Enabled: true},
			{ID: 2, PayWay: "card", Market: "JP", Priority: 100, Enabled: true},
		}
		best, err := PickBestRoute(routes, MatchInput{PayWay: "card", Market: "JP"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if best.ID != 2 {
			t.Fatalf("want ID 2 got %d", best.ID)
		}
	})

	t.Run("GLOBAL beats wildcard market", func(t *testing.T) {
		routes := []Route{
			{ID: 1, PayWay: "card", Market: "*", Priority: 1, Enabled: true},
			{ID: 2, PayWay: "card", Market: "GLOBAL", Priority: 99, Enabled: true},
		}
		best, err := PickBestRoute(routes, MatchInput{PayWay: "card", Market: "SEA"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if best.ID != 2 {
			t.Fatalf("want ID 2 got %d", best.ID)
		}
	})

	t.Run("explicit condition count beats priority", func(t *testing.T) {
		routes := []Route{
			{ID: 1, PayWay: "card", Market: "GLOBAL", Country: "*", Currency: "*", Priority: 1, Enabled: true},
			{ID: 2, PayWay: "card", Market: "GLOBAL", Country: "JP", Currency: "JPY", Priority: 99, Enabled: true},
		}
		best, err := PickBestRoute(routes, MatchInput{
			PayWay: "card", Market: "JP", Country: "JP", Currency: "JPY",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if best.ID != 2 {
			t.Fatalf("want ID 2 got %d", best.ID)
		}
	})

	t.Run("priority resolves final tie", func(t *testing.T) {
		routes := []Route{
			{ID: 1, PayWay: "card", Market: "GLOBAL", Country: "JP", Priority: 20, Enabled: true},
			{ID: 2, PayWay: "card", Market: "GLOBAL", Country: "JP", Priority: 10, Enabled: true},
		}
		best, err := PickBestRoute(routes, MatchInput{PayWay: "card", Market: "KR", Country: "JP"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if best.ID != 2 {
			t.Fatalf("want ID 2 got %d", best.ID)
		}
	})
}

func TestMatchedRoutesNormalizationAndFilters(t *testing.T) {
	routes := []Route{
		{ID: 1, PayWay: "card", Package: "", Channel: "", Market: "", Country: "", Currency: "", Enabled: true},
		{ID: 2, PayWay: "card", Package: "*", Channel: "*", Market: "*", Country: "*", Currency: "*", Enabled: true},
		{ID: 3, PayWay: "*", Package: "*", Channel: "*", Market: "*", Country: "*", Currency: "*", Enabled: true},
		{ID: 4, PayWay: "card", Package: "*", Channel: "*", Market: "*", Country: "*", Currency: "*", Enabled: false},
	}

	got := MatchedRoutes(routes, MatchInput{PayWay: "card", Market: "JP"})
	if len(got) != 2 {
		t.Fatalf("want 2 matched routes got %d", len(got))
	}
	if got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("unexpected matched IDs: %+v", []int64{got[0].ID, got[1].ID})
	}
}

func TestPickBestRouteNotFound(t *testing.T) {
	_, err := PickBestRoute([]Route{
		{PayWay: "wallet", Enabled: true},
	}, MatchInput{PayWay: "card"})
	if err != ErrRouteNotFound {
		t.Fatalf("want ErrRouteNotFound got %v", err)
	}
}
