package cashier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/app/command"
	domaincashier "github.com/csw/console/services/admin-api/internal/domain/cashier"
)

type fakeCopyTemplateVersionService struct {
	result domaincashier.TemplateVersion
}

func (f fakeCopyTemplateVersionService) CopyToDraft(_ context.Context, _ command.CopyTemplateVersionCommand) (domaincashier.TemplateVersion, error) {
	return f.result, nil
}

func TestCopyTemplateVersionEndpointReturnsDraft(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/cashier/templates/template-1/versions/7/copy-to-draft", nil)
	rec := httptest.NewRecorder()

	handler := NewTemplateVersionHandler(fakeCopyTemplateVersionService{
		result: domaincashier.TemplateVersion{Version: 8, Status: domaincashier.StatusDraft},
	})
	handler.CopyToDraft(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"status":"draft"`) {
		t.Fatalf("expected draft response, got %s", rec.Body.String())
	}
}
