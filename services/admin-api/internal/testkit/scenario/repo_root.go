package scenario

import (
	"os"
	"path/filepath"
)

// findRepoRoot 从给定目录向上查找含 tests/backend/scenarios 的目录。
func findRepoRoot(start string) (string, bool) {
	dir := start
	for {
		if fi, err := os.Stat(filepath.Join(dir, "tests", "backend", "scenarios")); err == nil && fi.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// ScenariosDir 返回 tests/backend/scenarios 绝对路径。
func ScenariosDir() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	root, ok := findRepoRoot(wd)
	if !ok {
		return "", false
	}
	return filepath.Join(root, "tests", "backend", "scenarios"), true
}
