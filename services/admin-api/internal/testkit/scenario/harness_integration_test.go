package scenario

import (
	"context"
	"os"
	"testing"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	dashboardquery "github.com/csw/console/services/admin-api/internal/app/query/dashboard"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestHarnessDashboardSummarySmoke(t *testing.T) {
	if os.Getenv("SCENARIO_WITH_DB") != "1" {
		t.Skip("SCENARIO_WITH_DB=1 required")
	}
	cfg, err := harnessConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	pool := newHarnessPool(t, cfg.PostgresDSN)
	svc := dashboardquery.NewQueryService(pool)
	perms := []string{"dashboard.read", "cashier.read", "channel.read", "game.read", "sync.preview", "snapshot.read"}
	ac := domainauth.NewAuthContext(1001, "scenario", "Scenario", []string{"dashboard_reader"}, perms, common.Environment(cfg.Environment))
	ctx := adminapp.WithAuthContext(context.Background(), ac)
	_, err = svc.Summary(ctx, dto.DashboardSummaryParams{Range: "7d", WithTopItems: true, TopN: 5})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
}
