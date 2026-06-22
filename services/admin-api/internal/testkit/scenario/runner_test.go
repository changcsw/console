package scenario

import (
	"net/http"
	"testing"

	"github.com/csw/console/services/admin-api/internal/infra/config"
	"github.com/csw/console/services/admin-api/internal/transport/httpserver"
)

func testHandler() http.Handler {
	return httpserver.New(config.Config{AppName: "admin-api", Environment: "test", HTTPAddress: ":0"}).Handler
}

func TestRunCaseHealthzPasses(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name:    "healthz_ok",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 200, JSONContains: map[string]any{"status": "ok"}},
	})
	if !res.Passed {
		t.Fatalf("expected pass, got: %s", res.Message)
	}
}

func TestRunCaseDetectsStatusMismatch(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name:    "healthz_wrong",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 500},
	})
	if res.Passed {
		t.Fatal("expected failure on status mismatch")
	}
}

func TestRunCaseSyncPreviewRejectsUnknownSection(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name: "sync_preview_unknown_section",
		Request: Request{
			Method: "POST",
			Path:   "/api/admin/games/100001/sync/preview",
			Body:   map[string]any{"selected_sections": []any{"marketing"}},
		},
		Expect: Expect{Status: 400},
	})
	if !res.Passed {
		t.Fatalf("expected 400 for unknown section, got: %s", res.Message)
	}
}

func TestRunCaseJSONPathNotFound(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name:    "missing_path",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 200, JSONContains: map[string]any{"nope": "x"}},
	})
	if res.Passed {
		t.Fatal("expected failure when json path missing")
	}
}

func TestRunCaseJSONValueMismatch(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name:    "value_mismatch",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 200, JSONContains: map[string]any{"status": "nope"}},
	})
	if res.Passed {
		t.Fatal("expected failure on value mismatch")
	}
}

func TestRunCaseNonJSONBodyWithJSONContains(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("not json"))
	})
	res := RunCase(h, Case{
		Name:    "non_json",
		Request: Request{Method: "GET", Path: "/x"},
		Expect:  Expect{Status: 200, JSONContains: map[string]any{"a": "b"}},
	})
	if res.Passed {
		t.Fatal("expected failure when body is not JSON but JSONContains set")
	}
}
