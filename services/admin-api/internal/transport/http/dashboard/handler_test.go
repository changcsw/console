package dashboard

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
	"github.com/go-chi/chi/v5"
)

type stubSummaryService struct {
	out dto.DashboardSummary
	err error
}

func (s stubSummaryService) Summary(_ context.Context, _ dto.DashboardSummaryParams) (dto.DashboardSummary, error) {
	return s.out, s.err
}

func TestParseSummaryParamsValidation(t *testing.T) {
	t.Parallel()

	t.Run("rejects invalid withTopItems", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/summary?withTopItems=oops", nil)
		_, err := parseSummaryParams(req)
		if err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("rejects invalid topN", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/summary?topN=0", nil)
		_, err := parseSummaryParams(req)
		if err == nil {
			t.Fatal("want error")
		}
	})
}

func TestSummaryRouteAuthAndSuccess(t *testing.T) {
	t.Parallel()

	issuer, err := infrajwt.NewIssuer(infrajwt.Config{
		Secret:     "dashboard-test-secret",
		Issuer:     "admin-api",
		AccessTTL:  30 * time.Minute,
		RefreshTTL: 336 * time.Hour,
	})
	if err != nil {
		t.Fatalf("new issuer: %v", err)
	}

	svc := stubSummaryService{
		out: dto.DashboardSummary{
			Environment: "sandbox",
			GeneratedAt: time.Unix(1700000000, 0).UTC(),
			TimeRange: dto.DashboardTimeRange{
				Range: "7d",
				Since: time.Unix(1700000000, 0).UTC().Add(-7 * 24 * time.Hour),
				Until: time.Unix(1700000000, 0).UTC(),
			},
			FXReview: dto.DashboardFXReviewMetric{TopItems: []dto.DashboardFXReviewItem{}},
			ConfigIssues: dto.DashboardConfigIssuesMetric{
				BySource: []dto.DashboardConfigIssueCount{},
				TopItems: []dto.DashboardConfigIssueItem{},
			},
			RecentSyncJobs: dto.DashboardRecentSyncMetric{TopItems: []dto.DashboardSyncJobItem{}},
			PendingSnapshots: dto.DashboardPendingSnapMetric{
				TopItems: []dto.DashboardSnapshotTopItem{},
			},
			ChannelInstanceIssues: dto.DashboardChannelIssueMetric{TopItems: []dto.DashboardChannelIssueTopItem{}},
		},
	}

	root := chi.NewRouter()
	sub := chi.NewRouter()
	RegisterRoutes(sub, NewHandler(svc), issuer, common.EnvSandbox, slog.New(slog.NewTextHandler(io.Discard, nil)), true, nil)
	root.Mount("/api/admin", sub)

	t.Run("S2 unauthenticated", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/summary", nil)
		root.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("want 401 got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("S3 forbidden missing dashboard.read", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/summary", nil)
		req.Header.Set("Authorization", "Bearer "+issueToken(t, issuer, []string{"game.read"}))
		root.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("want 403 got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("S1 success envelope", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/dashboard/summary?range=7d&topN=5", nil)
		req.Header.Set("Authorization", "Bearer "+issueToken(t, issuer, []string{"dashboard.read"}))
		root.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200 got %d body=%s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal body: %v", err)
		}
		if _, ok := body["data"].(map[string]any); !ok {
			t.Fatalf("response should contain data envelope: %v", body)
		}
	})
}

func issueToken(t *testing.T, issuer *infrajwt.Issuer, perms []string) string {
	t.Helper()
	pair, err := issuer.IssuePair(domainauth.NewAuthContext(1001, "tester", "Tester", []string{"tester"}, perms, common.EnvSandbox))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return pair.AccessToken
}
