package scenario

import (
	"path/filepath"
	"testing"

	"github.com/csw/console/services/admin-api/internal/infra/config"
	"github.com/csw/console/services/admin-api/internal/transport/httpserver"
)

func TestScenarioManifests(t *testing.T) {
	dir, ok := ScenariosDir()
	if !ok {
		t.Skip("tests/backend/scenarios not found from cwd")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no scenario manifests")
	}

	handler := httpserver.New(config.Config{AppName: "admin-api", Environment: "test", HTTPAddress: ":0"}).Handler

	for _, f := range files {
		f := f
		m, err := LoadManifest(f)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		t.Run(m.Module, func(t *testing.T) {
			for _, c := range m.Cases {
				c := c
				t.Run(c.Name, func(t *testing.T) {
					res := RunCase(handler, c)
					if !res.Passed {
						t.Errorf("[%s] %s", res.Dimension, res.Message)
					}
				})
			}
		})
	}
}
