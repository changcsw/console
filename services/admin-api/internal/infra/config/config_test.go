package config

import "testing"

func TestMustLoadUsesLocalDefaultHTTPAddress(t *testing.T) {
	t.Setenv("HTTP_ADDRESS", "")

	cfg := MustLoad()

	if cfg.HTTPAddress != ":18080" {
		t.Fatalf("expected default http address :18080, got %s", cfg.HTTPAddress)
	}
}
