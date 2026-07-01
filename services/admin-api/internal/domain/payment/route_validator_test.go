package payment

import (
	"errors"
	"testing"
)

func TestValidateRouteSetDuplicatePriority(t *testing.T) {
	err := ValidateRouteSet([]Route{
		{PayWay: "card", Priority: 10, Enabled: true},
		{PayWay: "card", Priority: 10, Enabled: true},
	})
	if err == nil {
		t.Fatal("expected duplicate priority conflict")
	}
	var conflict *RouteConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want RouteConflictError got %T", err)
	}
	if conflict.Kind != ConflictDuplicatePriority {
		t.Fatalf("want duplicate_priority got %s", conflict.Kind)
	}
}

func TestValidateRouteSetDuplicateSelectorWithNullAndWildcardChannel(t *testing.T) {
	err := ValidateRouteSet([]Route{
		{PayWay: "card", Package: "pkg-a", Channel: "", Market: "JP", Country: "*", Currency: "*", Priority: 1, Enabled: true},
		{PayWay: "card", Package: "pkg-a", Channel: "*", Market: "JP", Country: "", Currency: "", Priority: 2, Enabled: true},
	})
	if err == nil {
		t.Fatal("expected duplicate selector conflict")
	}
	var conflict *RouteConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want RouteConflictError got %T", err)
	}
	if conflict.Kind != ConflictDuplicateSelector {
		t.Fatalf("want duplicate_selector got %s", conflict.Kind)
	}
}

func TestValidateRouteSetIgnoresDisabledAndOtherPayWay(t *testing.T) {
	err := ValidateRouteSet([]Route{
		{PayWay: "card", Priority: 10, Enabled: true},
		{PayWay: "card", Priority: 10, Enabled: false},
		{PayWay: "wallet", Priority: 10, Enabled: true},
	})
	if err != nil {
		t.Fatalf("expected no conflict got %v", err)
	}
}
