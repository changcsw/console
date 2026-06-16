package syncapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/app/command"
	domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"
)

type fakeSectionSyncService struct{}

func (fakeSectionSyncService) Preview(_ context.Context, _ command.PreviewSectionSyncCommand) (domainsync.Preview, error) {
	return domainsync.Preview{}, nil
}

func (fakeSectionSyncService) Execute(_ context.Context, _ command.ExecuteSectionSyncCommand) error {
	return nil
}

func TestExecuteSyncRejectsUnknownSection(t *testing.T) {
	body := strings.NewReader(`{"selected_sections":["channels","unknown"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/games/game-1/sync/execute", body)
	rec := httptest.NewRecorder()

	handler := NewSectionSyncHandler(fakeSectionSyncService{})
	handler.Execute(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "unknown section") {
		t.Fatalf("expected unknown section error, got %s", rec.Body.String())
	}
}
