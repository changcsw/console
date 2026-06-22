package scenario

import "testing"

func TestLookupNestedAndIndex(t *testing.T) {
	root := map[string]any{
		"items": []any{
			map[string]any{"id": "x"},
		},
		"data": map[string]any{"status": "ok"},
	}
	if v, ok := lookup(root, "data.status"); !ok || v != "ok" {
		t.Fatalf("data.status: got %v ok=%v", v, ok)
	}
	if v, ok := lookup(root, "items.0.id"); !ok || v != "x" {
		t.Fatalf("items.0.id: got %v ok=%v", v, ok)
	}
	if _, ok := lookup(root, "items.5.id"); ok {
		t.Fatal("expected out-of-range index to miss")
	}
	if _, ok := lookup(root, "missing.key"); ok {
		t.Fatal("expected missing key to miss")
	}
}

func TestEqualScalar(t *testing.T) {
	if !equalScalar(float64(200), 200) {
		t.Fatal("float64(200) should equal int 200")
	}
	if !equalScalar("ok", "ok") {
		t.Fatal("string equality")
	}
	if equalScalar(map[string]any{"a": 1}, map[string]any{"a": 1}) {
		t.Fatal("composite values must not compare equal via equalScalar")
	}
	if equalScalar([]any{1, 2}, []any{1, 2}) {
		t.Fatal("slices must not compare equal")
	}
}
