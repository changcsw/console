package common

import "testing"

func TestMarketHelpers(t *testing.T) {
	if !MarketCN.IsCN() {
		t.Fatal("CN should report IsCN")
	}

	if MarketGlobal.IsCN() {
		t.Fatal("GLOBAL should not report IsCN")
	}

	if !MarketJP.UsesGlobalFallback() {
		t.Fatal("JP should use GLOBAL fallback")
	}
}
