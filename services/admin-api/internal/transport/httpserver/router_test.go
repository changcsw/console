package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/infra/config"
)

// cashier 路由（含 copy-to-draft）现由真实 cashier 模块路由接管，并在降级模式下
// 仍挂载路由形状：受保护接口先过 Authn，无令牌即 401（不再走旧 scaffold 的免鉴权 201）。
func TestTemplateVersionCopyRouteIsRegistered(t *testing.T) {
	server := New(config.Config{
		AppName:     "admin-api",
		Environment: "test",
		HTTPAddress: ":0",
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/admin/cashier/templates/template-1/versions/7/copy-to-draft",
		nil,
	)
	rec := httptest.NewRecorder()

	server.Handler.ServeHTTP(rec, req)

	// 路由已注册且受鉴权保护：无令牌 → 401 UNAUTHENTICATED。
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d with body %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "UNAUTHENTICATED") {
		t.Fatalf("expected UNAUTHENTICATED, got %s", rec.Body.String())
	}
}
