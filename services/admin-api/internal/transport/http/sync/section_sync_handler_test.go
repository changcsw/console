package syncapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/app/command"
	domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"
	"github.com/go-chi/chi/v5"
)

type fakeSectionSyncService struct{}

func (fakeSectionSyncService) Preview(_ context.Context, _ command.PreviewSectionSyncCommand) (domainsync.Preview, error) {
	return domainsync.Preview{}, nil
}

func (fakeSectionSyncService) Execute(_ context.Context, _ command.ExecuteSectionSyncCommand) (domainsync.ExecuteResult, error) {
	return domainsync.ExecuteResult{}, nil
}

func (fakeSectionSyncService) ListJobs(_ context.Context, _ command.ListSectionSyncJobsQuery) (domainsync.JobList, error) {
	return domainsync.JobList{}, nil
}

func TestExecuteSyncRejectsUnknownSection(t *testing.T) {
	body := strings.NewReader(`{"selectedSections":["channels","unknown"],"baselineToken":"t"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/games/game-1/sync/execute", body)
	rec := httptest.NewRecorder()

	handler := NewSectionSyncHandler(fakeSectionSyncService{})
	r := chi.NewRouter()
	r.Post("/api/admin/games/{gameId}/sync/execute", handler.Execute)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), command.CodeUnknownSection) {
		t.Fatalf("expected unknown section error, got %s", rec.Body.String())
	}
}

func TestPreviewSyncRejectsUnknownSection(t *testing.T) {
	body := strings.NewReader(`{"sections":["channels","marketing"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/games/game-1/sync/preview", body)
	rec := httptest.NewRecorder()

	handler := NewSectionSyncHandler(fakeSectionSyncService{})
	r := chi.NewRouter()
	r.Post("/api/admin/games/{gameId}/sync/preview", handler.Preview)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), command.CodeUnknownSection) {
		t.Fatalf("expected unknown section error, got %s", rec.Body.String())
	}
}
