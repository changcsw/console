package snapshot

import (
	"testing"
	"time"
)

func TestCanonicalJSONDeterministic(t *testing.T) {
	inputA := map[string]any{
		"b": 1,
		"a": map[string]any{"z": 2, "c": 3},
	}
	inputB := map[string]any{
		"a": map[string]any{"c": 3, "z": 2},
		"b": 1,
	}

	a, err := CanonicalJSON(inputA)
	if err != nil {
		t.Fatalf("canonical a: %v", err)
	}
	b, err := CanonicalJSON(inputB)
	if err != nil {
		t.Fatalf("canonical b: %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf("canonical not deterministic: %s != %s", string(a), string(b))
	}
}

func TestBuildConfigVersion(t *testing.T) {
	ts := time.Date(2026, 7, 1, 14, 35, 0, 0, time.UTC)
	got := BuildConfigVersion(ts, "abcdef123456")
	if got != "20260701143500-abcdef12" {
		t.Fatalf("unexpected version: %s", got)
	}
}
