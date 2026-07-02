package scenario

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/csw/console/services/admin-api/internal/infra/config"
	"github.com/csw/console/services/admin-api/internal/transport/httpserver"
)

// scenarioDBReady 报告进程内 harness 是否接入了真实 PG + 全装配。
// 当前进程内 httptest handler 不连库（降级 ready=false），故标记 requiresDB
// 的 case 默认跳过；待连库 harness 落地后置 SCENARIO_WITH_DB=1 由其执行。
func scenarioDBReady() bool {
	return os.Getenv("SCENARIO_WITH_DB") == "1"
}

func noteSuffix(note string) string {
	if note == "" {
		return ""
	}
	return ": " + note
}

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

	var handler http.Handler
	var auth *roleTokenIssuer
	if scenarioDBReady() {
		handler, auth = buildDBHandler(t)
	} else {
		handler = httpserver.New(config.Config{AppName: "admin-api", Environment: "test", HTTPAddress: ":0"}).Handler
	}

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
					if c.RequiresDB && !scenarioDBReady() {
						t.Skipf("[%s] requires PG + full wiring (set SCENARIO_WITH_DB=1); manifest parsed OK%s",
							c.Dimension, noteSuffix(c.Note))
						return
					}
					res := RunCase(handler, c, auth)
					if !res.Passed {
						t.Errorf("[%s] %s", res.Dimension, res.Message)
					}
				})
			}
		})
	}
}
