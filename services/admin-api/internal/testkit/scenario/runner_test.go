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
	}, nil)
	if !res.Passed {
		t.Fatalf("expected pass, got: %s", res.Message)
	}
}

func TestRunCaseDetectsStatusMismatch(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name:    "healthz_wrong",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 500},
	}, nil)
	if res.Passed {
		t.Fatal("expected failure on status mismatch")
	}
}

// sync/preview 是受保护的真实路由（sync.preview 权限）；进程内无令牌时应被 Authn 拦为 401。
// 未知 section 的 400 断言需到达 handler（连库 harness / handler 单测覆盖，见
// transport/http/sync 的 TestPreviewSyncRejectsUnknownSection 与 tests/backend/scenarios/sync.yaml）。
func TestRunCaseSyncPreviewRequiresAuth(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name: "sync_preview_requires_auth",
		Request: Request{
			Method: "POST",
			Path:   "/api/admin/games/100001/sync/preview",
			Body:   map[string]any{"sections": []any{"channels"}},
		},
		Expect: Expect{Status: 401, JSONContains: map[string]any{"error.code": "UNAUTHENTICATED"}},
	}, nil)
	if !res.Passed {
		t.Fatalf("expected 401 for unauthenticated sync preview, got: %s", res.Message)
	}
}

func TestRunCaseJSONPathNotFound(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name:    "missing_path",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 200, JSONContains: map[string]any{"nope": "x"}},
	}, nil)
	if res.Passed {
		t.Fatal("expected failure when json path missing")
	}
}

func TestRunCaseJSONValueMismatch(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name:    "value_mismatch",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 200, JSONContains: map[string]any{"status": "nope"}},
	}, nil)
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
	}, nil)
	if res.Passed {
		t.Fatal("expected failure when body is not JSON but JSONContains set")
	}
}
