package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/infra/config"
)

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

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"status":"draft"`) {
		t.Fatalf("expected draft response, got %s", rec.Body.String())
	}
}
