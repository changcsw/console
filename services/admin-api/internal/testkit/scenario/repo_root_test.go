package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoRootWalksUp(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "services", "admin-api", "internal", "testkit", "scenario")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tests", "backend", "scenarios"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok := findRepoRoot(deep)
	if !ok || got != root {
		t.Fatalf("want %s, got %s ok=%v", root, got, ok)
	}
}
