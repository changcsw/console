# Integration Checklist (Backend) - module 23 dashboard

## Module APIs

- `GET /api/admin/dashboard/summary`
  - permission: `dashboard.read` (entry)
  - metric-level permission clipping:
    - `fxReview`: `cashier.read`
    - `configIssues`: `channel.read` + `game.read`
    - `recentSyncJobs`: `sync.preview`
    - `pendingSnapshots`: `snapshot.read`
    - `channelInstanceIssues`: `channel.read`
  - query: `range` (`24h|7d|30d|90d`, default `7d`), `withTopItems` (default `false`), `topN` (`1..20`, default `5`)

## Backend Test Assets

- Unit: `services/admin-api/internal/app/query/dashboard/service_test.go`
- Handler/route: `services/admin-api/internal/transport/http/dashboard/handler_test.go`
- Scenario manifest: `tests/backend/scenarios/dashboard.yaml` (S1-S10 complete, N/A reasons included)
- Fixtures entry:
  - `tests/fixtures/common/dashboard.sql`
  - `tests/fixtures/sandbox/dashboard.sql`
  - `tests/fixtures/production/dashboard.sql`

## Referenced Modules / Source Tables

- `17-cashier-template`: `platform.cashier_fx_sync_runs`, `platform.cashier_price_templates`
- `20-snapshot`: `game_config_snapshots`, `games`
- `21-sync`: `platform.sync_jobs`
- channel/account-auth/channellogin/plugin/product sources:
  - `game_channels`, `platform.channels`
  - `game_account_auth_configs`, `platform.account_auth_types`
  - `game_channel_login_configs`, `game_channel_iap_configs`
  - `channel_package_iap_overrides`, `game_channel_plugin_configs`, `channel_package_plugin_overrides`, `channel_packages`

## Shared Surfaces Touched

- `services/admin-api/internal/transport/httpserver/admin_wiring.go` (added dashboard route wiring)
- New source-module repository read methods added: **none** (dashboard query uses dedicated read-only SQL service)

## Known Issues / Deviations

- Optional drill-down APIs are not implemented in this phase:
  - `/dashboard/pending-fx-runs`
  - `/dashboard/config-issues`
  - `/dashboard/recent-sync-jobs`
  - `/dashboard/pending-snapshots`
  - `/dashboard/channel-instance-issues`

## Integration Steps

1. Ensure auth seed contains `dashboard.read` (already present per module constraint).
2. Deploy backend with new dashboard transport/query packages.
3. Verify summary API under different permission combinations.

## Verification Commands

- `cd /Users/csw/gitproject/console-dashboard/services/admin-api && go test ./internal/app/query/dashboard/... ./internal/transport/http/dashboard/... ./...`

## Risks

- Dashboard query references multiple upstream tables; if any upstream schema drifts, summary queries may fail at runtime.
- Config issue topItems rely on `last_check_at` ordering and existing target join semantics from source modules.
- DB-dependent scenario dimensions (`requiresDB=true`) still require PG CI execution for data/assertion verification.
