package payment

import "testing"

func int64Ptr(v int64) *int64 {
	return &v
}

func TestSpecificMarketBeatsGlobalFallback(t *testing.T) {
	routes := []Route{
		{PayWayIDRef: 1, MarketCode: "GLOBAL", Priority: 20, Enabled: true},
		{PayWayIDRef: 1, MarketCode: "JP", Priority: 30, Enabled: true},
	}

	got, err := PickBestRoute(routes, RouteMatchInput{PayWayIDRef: 1, MarketCode: "JP"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.MarketCode != "JP" {
		t.Fatalf("expected JP route, got %s", got.MarketCode)
	}
}

func TestDuplicatePriorityWithinSamePayWayFails(t *testing.T) {
	routes := []Route{
		{PayWayIDRef: 1, Priority: 10, Enabled: true},
		{PayWayIDRef: 1, Priority: 10, Enabled: true},
	}

	if err := ValidateRouteSet(routes); err == nil {
		t.Fatal("expected duplicate priority failure")
	}
}

func TestExactPackageBeatsWildcardBeforePriority(t *testing.T) {
	packageID := int64(88)
	routes := []Route{
		{PayWayIDRef: 1, PackageIDRef: nil, MarketCode: "JP", Priority: 5, Enabled: true},
		{PayWayIDRef: 1, PackageIDRef: int64Ptr(packageID), MarketCode: "JP", Priority: 99, Enabled: true},
	}

	got, err := PickBestRoute(routes, RouteMatchInput{
		PayWayIDRef:  1,
		PackageIDRef: int64Ptr(packageID),
		MarketCode:   "JP",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.PackageIDRef == nil || *got.PackageIDRef != packageID {
		t.Fatalf("expected exact package route, got %#v", got.PackageIDRef)
	}
}

func TestDuplicateSelectorTreatsEmptyAsWildcard(t *testing.T) {
	routes := []Route{
		{PayWayIDRef: 1, MarketCode: "", ChannelIDRef: nil, PackageIDRef: int64Ptr(10), Enabled: true},
		{PayWayIDRef: 1, MarketCode: "*", ChannelIDRef: nil, PackageIDRef: int64Ptr(10), Enabled: true},
	}

	if err := ValidateRouteSet(routes); err == nil {
		t.Fatal("expected duplicate selector failure")
	}
}
