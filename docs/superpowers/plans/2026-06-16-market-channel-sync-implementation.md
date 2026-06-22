# Market Channel Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the approved market/channel instance model, section-scoped sync behavior, template version workflow, payment route constraints, and matching admin UI behavior across `services/admin-api` and `apps/admin-web`.

**Architecture:** Extend the Go backend around `GameMarket` and `GameMarketChannel`, enforce all market visibility and runtime rules server-side, then expose those rules through dedicated admin APIs. Build the Vue admin UI around an all-market channel-instance overview, market-aware creation flows, explicit hidden/incompatible/runtime statuses, and section-based sync actions that mirror backend semantics exactly.

**Tech Stack:** Go, PostgreSQL, Vue 3, Vite, TypeScript, Pinia, Vue Router, Element Plus

---

## Scope Note

This plan spans backend domain rules, HTTP APIs, and frontend admin UX. Keep it as one coordinated implementation because the data model, sync behavior, and UI states are tightly coupled; backend-only or frontend-only delivery would leave the feature unusable.

## File Structure Map

Assume the existing skeletons from `docs/architecture/zh-CN/README.md` are the roots to extend:

- `services/admin-api/internal/domain/common/market.go`
  - Canonical `market` enum and helper methods.
- `services/admin-api/internal/domain/channel/visibility.go`
  - Domestic/overseas channel-region compatibility rules.
- `services/admin-api/internal/domain/channel/game_market_channel.go`
  - `GameMarketChannel` aggregate, hide/unhide, copy-from-market creation rules.
- `services/admin-api/internal/domain/payment/route_matcher.go`
  - Payment route normalization, wildcard handling, match ordering.
- `services/admin-api/internal/domain/payment/route_validator.go`
  - Same-`pay_way` priority and selector uniqueness checks.
- `services/admin-api/internal/domain/cashier/template_version.go`
  - `draft / published / archived` lifecycle and copy-to-draft helpers.
- `services/admin-api/internal/app/dto/game_market_channel_dto.go`
  - Channel-instance list/create/update transport DTOs.
- `services/admin-api/internal/app/command/create_market_channel.go`
  - Create empty or copy-based market channel instances.
- `services/admin-api/internal/app/command/hide_market_channel.go`
  - Hide/unhide behavior and runtime invalidation.
- `services/admin-api/internal/app/command/copy_template_version.go`
  - Copy `published` or `archived` versions into a new `draft`.
- `services/admin-api/internal/app/command/preview_section_sync.go`
  - Section-scoped preview builder.
- `services/admin-api/internal/app/command/execute_section_sync.go`
  - Section-scoped sync executor with dependency checks.
- `services/admin-api/internal/app/query/list_market_channels.go`
  - All-market channel overview plus market/status filters.
- `services/admin-api/internal/app/query/build_runtime_config.go`
  - `GLOBAL` plus specific-market merge logic.
- `services/admin-api/internal/transport/http/channels/market_channel_handler.go`
  - `/games/{gameId}/markets/{market}/channels` and hide/unhide endpoints.
- `services/admin-api/internal/transport/http/payment/route_handler.go`
  - Route validation and normalized save behavior.
- `services/admin-api/internal/transport/http/sync/section_sync_handler.go`
  - `selected_sections` request handling.
- `services/admin-api/internal/transport/http/cashier/template_version_handler.go`
  - Copy-to-draft endpoint.
- `apps/admin-web/src/api/gameMarketChannels.ts`
  - Channel-instance API client and request types.
- `apps/admin-web/src/api/templateVersions.ts`
  - Template version copy-to-draft API client.
- `apps/admin-web/src/api/syncSections.ts`
  - Sync preview/execute API client with `selected_sections`.
- `apps/admin-web/src/views/games/detail/ChannelInstancesTab.vue`
  - All-market overview entry point.
- `apps/admin-web/src/views/games/detail/components/ChannelInstanceTable.vue`
  - Table, filters, runtime flags, hidden/incompatible states.
- `apps/admin-web/src/views/games/detail/components/CreateMarketChannelDrawer.vue`
  - Create empty or copy-from-market instance flow.
- `apps/admin-web/src/views/games/detail/components/ChannelInstanceStatusTag.vue`
  - `empty / invalid / valid` and compatibility tags.
- `apps/admin-web/src/views/games/detail/components/ChannelInstanceRuntimeFlags.vue`
  - Snapshot/sync/runtime inclusion badges.
- `apps/admin-web/src/views/games/detail/components/SyncSectionDrawer.vue`
  - `section`-scoped preview and execution UI.
- `apps/admin-web/src/views/cashier/templates/TemplateVersionsTab.vue`
  - Version table with copy-to-draft action.
- `apps/admin-web/src/views/cashier/templates/components/CopyPublishedToDraftDialog.vue`
  - Draft copy dialog.

### Task 1: Lock Shared Market and Channel Visibility Rules

**Files:**
- Create: `services/admin-api/internal/domain/common/market.go`
- Create: `services/admin-api/internal/domain/common/market_test.go`
- Create: `services/admin-api/internal/domain/channel/visibility.go`
- Create: `services/admin-api/internal/domain/channel/visibility_test.go`

- [ ] **Step 1: Write the failing market and visibility tests**

```go
package common

import "testing"

func TestMarketHelpers(t *testing.T) {
	if !MarketCN.IsCN() {
		t.Fatal("CN should report IsCN")
	}

	if MarketGlobal.IsCN() {
		t.Fatal("GLOBAL should not report IsCN")
	}

	if !MarketJP.UsesGlobalFallback() {
		t.Fatal("JP should use GLOBAL fallback")
	}
}
```

```go
package channel

import (
	"testing"

	"services/admin-api/internal/domain/common"
)

func TestValidateMarketChannelCompatibility(t *testing.T) {
	if err := ValidateMarketChannelCompatibility(common.MarketCN, ChannelRegionDomestic); err != nil {
		t.Fatalf("CN + domestic should pass: %v", err)
	}

	if err := ValidateMarketChannelCompatibility(common.MarketCN, ChannelRegionOverseas); err == nil {
		t.Fatal("CN + overseas should fail")
	}

	if err := ValidateMarketChannelCompatibility(common.MarketJP, ChannelRegionDomestic); err == nil {
		t.Fatal("JP + domestic should fail")
	}
}
```

- [ ] **Step 2: Run the backend domain tests and verify they fail**

Run: `cd services/admin-api && go test ./internal/domain/common ./internal/domain/channel -run 'TestMarketHelpers|TestValidateMarketChannelCompatibility' -v`

Expected: FAIL with missing `Market`, `IsCN`, or `ValidateMarketChannelCompatibility` symbols.

- [ ] **Step 3: Write the minimal market and visibility implementation**

```go
package common

type Market string

const (
	MarketGlobal Market = "GLOBAL"
	MarketJP     Market = "JP"
	MarketKR     Market = "KR"
	MarketSEA    Market = "SEA"
	MarketHMT    Market = "HMT"
	MarketCN     Market = "CN"
)

func (m Market) IsCN() bool {
	return m == MarketCN
}

func (m Market) UsesGlobalFallback() bool {
	return m == MarketJP || m == MarketKR || m == MarketSEA || m == MarketHMT
}
```

```go
package channel

import (
	"fmt"

	"services/admin-api/internal/domain/common"
)

type ChannelRegion string

const (
	ChannelRegionDomestic ChannelRegion = "domestic"
	ChannelRegionOverseas ChannelRegion = "overseas"
)

func ValidateMarketChannelCompatibility(market common.Market, region ChannelRegion) error {
	if market.IsCN() && region != ChannelRegionDomestic {
		return fmt.Errorf("market %s only accepts domestic channels", market)
	}

	if !market.IsCN() && region != ChannelRegionOverseas {
		return fmt.Errorf("market %s only accepts overseas channels", market)
	}

	return nil
}
```

- [ ] **Step 4: Re-run the backend domain tests and verify they pass**

Run: `cd services/admin-api && go test ./internal/domain/common ./internal/domain/channel -run 'TestMarketHelpers|TestValidateMarketChannelCompatibility' -v`

Expected: PASS for both tests.

- [ ] **Step 5: Commit the shared rule primitives**

```bash
git add services/admin-api/internal/domain/common/market.go services/admin-api/internal/domain/common/market_test.go services/admin-api/internal/domain/channel/visibility.go services/admin-api/internal/domain/channel/visibility_test.go
git commit -m "feat: add market and channel visibility rules"
```

### Task 2: Implement the GameMarketChannel Aggregate and Copy Workflow

**Files:**
- Create: `services/admin-api/internal/domain/channel/game_market_channel.go`
- Create: `services/admin-api/internal/domain/channel/game_market_channel_test.go`
- Create: `services/admin-api/internal/app/command/create_market_channel.go`
- Create: `services/admin-api/internal/app/command/hide_market_channel.go`
- Create: `services/admin-api/internal/app/query/list_market_channels.go`
- Create: `services/admin-api/internal/app/dto/game_market_channel_dto.go`

- [ ] **Step 1: Write the failing aggregate tests for copy and hide behavior**

```go
package channel

import "testing"

func TestNewCopiedMarketChannelClearsSensitiveFields(t *testing.T) {
	source := GameMarketChannel{
		NormalConfig: map[string]any{"client_id": "jp-client"},
		SecretConfig: map[string]string{"client_secret": "secret"},
		FileConfig:   map[string]string{"keystore": "file-id"},
	}

	copied := NewCopiedMarketChannel("game-1", "JP", "google", source)

	if copied.ConfigStatus != ConfigStatusInvalid {
		t.Fatalf("expected invalid status, got %s", copied.ConfigStatus)
	}

	if copied.SecretConfig["client_secret"] != "" {
		t.Fatal("secret should be cleared")
	}

	if copied.FileConfig["keystore"] != "" {
		t.Fatal("file should be cleared")
	}
}

func TestHideMarksChannelExcludedFromRuntime(t *testing.T) {
	item := GameMarketChannel{}
	item.Hide("ops@example.com")

	if !item.Hidden {
		t.Fatal("channel should be hidden")
	}

	if item.IncludedInRuntimeConfig() {
		t.Fatal("hidden channel should be excluded from runtime")
	}
}
```

- [ ] **Step 2: Run the aggregate tests and verify they fail**

Run: `cd services/admin-api && go test ./internal/domain/channel -run 'TestNewCopiedMarketChannelClearsSensitiveFields|TestHideMarksChannelExcludedFromRuntime' -v`

Expected: FAIL with missing `GameMarketChannel`, `NewCopiedMarketChannel`, or `Hide`.

- [ ] **Step 3: Write the aggregate, DTO, and app-service skeleton**

```go
package channel

type ConfigStatus string

const (
	ConfigStatusEmpty   ConfigStatus = "empty"
	ConfigStatusInvalid ConfigStatus = "invalid"
	ConfigStatusValid   ConfigStatus = "valid"
)

type GameMarketChannel struct {
	GameID       string
	Market       string
	ChannelID    string
	Hidden       bool
	NormalConfig map[string]any
	SecretConfig map[string]string
	FileConfig   map[string]string
	ConfigStatus ConfigStatus
}

func NewCopiedMarketChannel(gameID, market, channelID string, source GameMarketChannel) GameMarketChannel {
	return GameMarketChannel{
		GameID:       gameID,
		Market:       market,
		ChannelID:    channelID,
		NormalConfig: cloneAnyMap(source.NormalConfig),
		SecretConfig: map[string]string{},
		FileConfig:   map[string]string{},
		ConfigStatus: ConfigStatusInvalid,
	}
}

func (g *GameMarketChannel) Hide(_ string) {
	g.Hidden = true
}

func (g GameMarketChannel) IncludedInRuntimeConfig() bool {
	return !g.Hidden && g.ConfigStatus == ConfigStatusValid
}
```

```go
package dto

type GameMarketChannelListItem struct {
	GameID                   string `json:"gameId"`
	Market                   string `json:"market"`
	ChannelID                string `json:"channelId"`
	ConfigStatus             string `json:"configStatus"`
	Hidden                   bool   `json:"hidden"`
	IncludedInSnapshot       bool   `json:"includedInSnapshot"`
	IncludedInSync           bool   `json:"includedInSync"`
	IncludedInRuntimeConfig  bool   `json:"includedInRuntimeConfig"`
	IncompatibleWithMarket   bool   `json:"incompatibleWithMarket"`
}
```

- [ ] **Step 4: Add create/list/hide command-query tests and make them pass**

```go
func TestListMarketChannelsDefaultsToAllMarkets(t *testing.T) {
	query := ListMarketChannelsQuery{GameID: "game-1"}
	items := []dto.GameMarketChannelListItem{
		{Market: "GLOBAL", ChannelID: "google"},
		{Market: "JP", ChannelID: "google"},
	}

	got := FilterMarketChannels(query, items)
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
}
```

Run: `cd services/admin-api && go test ./internal/domain/channel ./internal/app/... -run 'Test(NewCopiedMarketChannelClearsSensitiveFields|HideMarksChannelExcludedFromRuntime|ListMarketChannelsDefaultsToAllMarkets)' -v`

Expected: PASS.

- [ ] **Step 5: Commit the market-channel aggregate layer**

```bash
git add services/admin-api/internal/domain/channel/game_market_channel.go services/admin-api/internal/domain/channel/game_market_channel_test.go services/admin-api/internal/app/command/create_market_channel.go services/admin-api/internal/app/command/hide_market_channel.go services/admin-api/internal/app/query/list_market_channels.go services/admin-api/internal/app/dto/game_market_channel_dto.go
git commit -m "feat: add market channel aggregate workflow"
```

### Task 3: Implement Payment Route Matching and Uniqueness Validation

**Files:**
- Create: `services/admin-api/internal/domain/payment/route_matcher.go`
- Create: `services/admin-api/internal/domain/payment/route_validator.go`
- Create: `services/admin-api/internal/domain/payment/route_matcher_test.go`

- [ ] **Step 1: Write the failing payment-route tests**

```go
package payment

import "testing"

func TestSpecificMarketBeatsGlobalFallback(t *testing.T) {
	routes := []Route{
		{PayWay: "card", Market: "GLOBAL", Priority: 20},
		{PayWay: "card", Market: "JP", Priority: 30},
	}

	got, err := PickBestRoute(routes, MatchInput{PayWay: "card", Market: "JP"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Market != "JP" {
		t.Fatalf("expected JP route, got %s", got.Market)
	}
}

func TestDuplicatePriorityWithinSamePayWayFails(t *testing.T) {
	routes := []Route{
		{PayWay: "card", Priority: 10},
		{PayWay: "card", Priority: 10},
	}

	if err := ValidateRouteSet(routes); err == nil {
		t.Fatal("expected duplicate priority failure")
	}
}
```

- [ ] **Step 2: Run the payment-route tests and verify they fail**

Run: `cd services/admin-api && go test ./internal/domain/payment -run 'TestSpecificMarketBeatsGlobalFallback|TestDuplicatePriorityWithinSamePayWayFails' -v`

Expected: FAIL with missing matcher or validator symbols.

- [ ] **Step 3: Write the matcher and validator**

```go
package payment

import "sort"

type Route struct {
	PayWay   string
	Package  string
	Channel  string
	Market   string
	Country  string
	Currency string
	Priority int
}

type MatchInput struct {
	PayWay   string
	Package  string
	Channel  string
	Market   string
	Country  string
	Currency string
}

func normalize(value string) string {
	if value == "" {
		return "*"
	}
	return value
}

func PickBestRoute(routes []Route, input MatchInput) (Route, error) {
	candidates := matchedRoutes(routes, input)
	sort.SliceStable(candidates, func(i, j int) bool {
		return compareRouteSpecificity(candidates[i], candidates[j]) < 0
	})
	return candidates[0], nil
}
```

```go
package payment

import "fmt"

func ValidateRouteSet(routes []Route) error {
	seenPriority := map[string]struct{}{}
	seenSelector := map[string]struct{}{}

	for _, route := range routes {
		priorityKey := fmt.Sprintf("%s:%d", route.PayWay, route.Priority)
		if _, ok := seenPriority[priorityKey]; ok {
			return fmt.Errorf("duplicate priority for pay_way %s", route.PayWay)
		}
		seenPriority[priorityKey] = struct{}{}

		selectorKey := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
			route.PayWay,
			normalize(route.Package),
			normalize(route.Channel),
			normalize(route.Market),
			normalize(route.Country),
			normalize(route.Currency),
		)
		if _, ok := seenSelector[selectorKey]; ok {
			return fmt.Errorf("duplicate selector for pay_way %s", route.PayWay)
		}
		seenSelector[selectorKey] = struct{}{}
	}

	return nil
}
```

- [ ] **Step 4: Re-run the payment-route tests and verify they pass**

Run: `cd services/admin-api && go test ./internal/domain/payment -run 'TestSpecificMarketBeatsGlobalFallback|TestDuplicatePriorityWithinSamePayWayFails' -v`

Expected: PASS.

- [ ] **Step 5: Commit the payment-route domain rules**

```bash
git add services/admin-api/internal/domain/payment/route_matcher.go services/admin-api/internal/domain/payment/route_validator.go services/admin-api/internal/domain/payment/route_matcher_test.go
git commit -m "feat: add payment route matcher rules"
```

### Task 4: Implement Template Version Copy-to-Draft Workflow

**Files:**
- Create: `services/admin-api/internal/domain/cashier/template_version.go`
- Create: `services/admin-api/internal/domain/cashier/template_version_test.go`
- Create: `services/admin-api/internal/app/command/copy_template_version.go`
- Create: `services/admin-api/internal/transport/http/cashier/template_version_handler.go`

- [ ] **Step 1: Write the failing template-version tests**

```go
package cashier

import "testing"

func TestCopyPublishedVersionCreatesDraft(t *testing.T) {
	source := TemplateVersion{Version: 7, Status: StatusPublished}
	copied := source.CopyToDraft(8)

	if copied.Status != StatusDraft {
		t.Fatalf("expected draft status, got %s", copied.Status)
	}

	if copied.Version != 8 {
		t.Fatalf("expected version 8, got %d", copied.Version)
	}
}
```

- [ ] **Step 2: Run the template-version tests and verify they fail**

Run: `cd services/admin-api && go test ./internal/domain/cashier -run TestCopyPublishedVersionCreatesDraft -v`

Expected: FAIL with missing `TemplateVersion` or `CopyToDraft`.

- [ ] **Step 3: Write the lifecycle model and command**

```go
package cashier

type VersionStatus string

const (
	StatusDraft     VersionStatus = "draft"
	StatusPublished VersionStatus = "published"
	StatusArchived  VersionStatus = "archived"
)

type TemplateVersion struct {
	Version int
	Status  VersionStatus
}

func (t TemplateVersion) CopyToDraft(nextVersion int) TemplateVersion {
	return TemplateVersion{
		Version: nextVersion,
		Status:  StatusDraft,
	}
}
```

```go
package command

type CopyTemplateVersionCommand struct {
	TemplateID     string
	SourceVersion  int
	SourceStatus   string
	RequestedBy    string
}
```

- [ ] **Step 4: Add the copy endpoint test and make it pass**

```go
func TestCopyTemplateVersionEndpointReturnsDraft(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/admin/cashier/templates/template-1/versions/7/copy-to-draft", nil)
	rec := httptest.NewRecorder()

	handler := NewTemplateVersionHandler(fakeCopyTemplateVersionService{
		Result: dto.TemplateVersionDTO{Version: 8, Status: "draft"},
	})
	handler.CopyToDraft(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"status":"draft"`) {
		t.Fatalf("expected draft response, got %s", rec.Body.String())
	}
}
```

Run: `cd services/admin-api && go test ./internal/domain/cashier ./internal/transport/http/cashier -run 'TestCopyPublishedVersionCreatesDraft|TestCopyTemplateVersionEndpointReturnsDraft' -v`

Expected: PASS.

- [ ] **Step 5: Commit the template version workflow**

```bash
git add services/admin-api/internal/domain/cashier/template_version.go services/admin-api/internal/domain/cashier/template_version_test.go services/admin-api/internal/app/command/copy_template_version.go services/admin-api/internal/transport/http/cashier/template_version_handler.go
git commit -m "feat: add template copy to draft workflow"
```

### Task 5: Implement Section-Scoped Sync and Runtime Config Merge

**Files:**
- Create: `services/admin-api/internal/app/command/preview_section_sync.go`
- Create: `services/admin-api/internal/app/command/execute_section_sync.go`
- Create: `services/admin-api/internal/app/query/build_runtime_config.go`
- Create: `services/admin-api/internal/app/query/build_runtime_config_test.go`
- Create: `services/admin-api/internal/transport/http/sync/section_sync_handler.go`

- [ ] **Step 1: Write the failing runtime-config and sync tests**

```go
package query

import "testing"

func TestSpecificMarketOverridesGlobalChannelInstance(t *testing.T) {
	cfg := BuildRuntimeConfig(RuntimeConfigInput{
		TargetMarket: "JP",
		GlobalChannels: []ChannelInstance{{ChannelID: "google", Value: "global"}},
		MarketChannels: []ChannelInstance{{ChannelID: "google", Value: "jp"}},
	})

	if cfg.Channels["google"].Value != "jp" {
		t.Fatalf("expected JP override, got %s", cfg.Channels["google"].Value)
	}
}

func TestHiddenChannelExcludedFromSyncPreview(t *testing.T) {
	diff := BuildSectionPreview(SectionPreviewInput{
		Sections: []string{"channels"},
		Channels: []ChannelInstance{{ChannelID: "google", Hidden: true}},
	})

	if len(diff["channels"]) != 0 {
		t.Fatal("hidden channel should not appear in preview")
	}
}
```

- [ ] **Step 2: Run the sync and runtime-config tests and verify they fail**

Run: `cd services/admin-api && go test ./internal/app/query ./internal/app/command -run 'TestSpecificMarketOverridesGlobalChannelInstance|TestHiddenChannelExcludedFromSyncPreview' -v`

Expected: FAIL with missing `BuildRuntimeConfig` or `BuildSectionPreview`.

- [ ] **Step 3: Write the merge and preview logic**

```go
package query

func BuildRuntimeConfig(input RuntimeConfigInput) RuntimeConfig {
	result := RuntimeConfig{Channels: map[string]ChannelInstance{}}

	for _, item := range input.GlobalChannels {
		if !item.Hidden && item.Valid {
			result.Channels[item.ChannelID] = item
		}
	}

	for _, item := range input.MarketChannels {
		if !item.Hidden && item.Valid {
			result.Channels[item.ChannelID] = item
		}
	}

	return result
}
```

```go
package command

func BuildSectionPreview(input SectionPreviewInput) map[string][]DiffItem {
	result := map[string][]DiffItem{}

	for _, section := range input.Sections {
		if section == "channels" {
			for _, item := range input.Channels {
				if item.Hidden {
					continue
				}
				result["channels"] = append(result["channels"], DiffItem{Key: item.ChannelID})
			}
		}
	}

	return result
}
```

- [ ] **Step 4: Add selected-sections HTTP tests and make them pass**

```go
func TestExecuteSyncRejectsUnknownSection(t *testing.T) {
	body := strings.NewReader(`{"selected_sections":["channels","unknown"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/games/game-1/sync/execute", body)
	rec := httptest.NewRecorder()

	handler := NewSectionSyncHandler(fakeSyncService{})
	handler.Execute(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "unknown section") {
		t.Fatalf("expected unknown section error, got %s", rec.Body.String())
	}
}
```

Run: `cd services/admin-api && go test ./internal/app/query ./internal/app/command ./internal/transport/http/sync -run 'Test(SpecificMarketOverridesGlobalChannelInstance|HiddenChannelExcludedFromSyncPreview|ExecuteSyncRejectsUnknownSection)' -v`

Expected: PASS.

- [ ] **Step 5: Commit the sync and runtime-config layer**

```bash
git add services/admin-api/internal/app/command/preview_section_sync.go services/admin-api/internal/app/command/execute_section_sync.go services/admin-api/internal/app/query/build_runtime_config.go services/admin-api/internal/app/query/build_runtime_config_test.go services/admin-api/internal/transport/http/sync/section_sync_handler.go
git commit -m "feat: add section scoped sync rules"
```

### Task 6: Expose Backend Market-Channel HTTP APIs

**Files:**
- Create: `services/admin-api/internal/transport/http/channels/market_channel_handler.go`
- Modify: `services/admin-api/internal/transport/http/channels/router.go`
- Modify: `services/admin-api/internal/transport/http/admin/router.go`

- [ ] **Step 1: Write the failing HTTP handler tests**

```go
func TestCreateMarketChannelRejectsDomesticChannelForJP(t *testing.T) {
	body := strings.NewReader(`{"channelId":"bilibili","region":"domestic"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/games/game-1/markets/JP/channels", body)
	rec := httptest.NewRecorder()

	handler := NewHandler(fakeCreateService{})
	handler.CreateMarketChannel(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "only accepts overseas channels") {
		t.Fatalf("expected compatibility error, got %s", rec.Body.String())
	}
}

func TestListMarketChannelsReturnsAllMarketsByDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/admin/games/game-1/market-channels", nil)
	rec := httptest.NewRecorder()

	handler := NewHandler(fakeListService{
		Items: []dto.GameMarketChannelListItem{
			{Market: "GLOBAL", ChannelID: "google"},
			{Market: "JP", ChannelID: "google"},
		},
	})
	handler.ListMarketChannels(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `"market":"GLOBAL"`) || !strings.Contains(body, `"market":"JP"`) {
		t.Fatalf("expected GLOBAL and JP rows, got %s", body)
	}
}
```

- [ ] **Step 2: Run the HTTP handler tests and verify they fail**

Run: `cd services/admin-api && go test ./internal/transport/http/channels -run 'Test(CreateMarketChannelRejectsDomesticChannelForJP|ListMarketChannelsReturnsAllMarketsByDefault)' -v`

Expected: FAIL with missing routes or handlers.

- [ ] **Step 3: Implement the handlers and route wiring**

```go
func (h *Handler) CreateMarketChannel(w http.ResponseWriter, r *http.Request) {
	market := chi.URLParam(r, "market")
	var req createMarketChannelRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	cmd := command.CreateMarketChannelCommand{
		GameID:    chi.URLParam(r, "gameId"),
		Market:    market,
		ChannelID: req.ChannelID,
		Region:    req.Region,
		CopyFrom:  req.CopyFrom,
	}

	result, err := h.createService.Execute(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) ListMarketChannels(w http.ResponseWriter, r *http.Request) {
	query := appquery.ListMarketChannelsQuery{
		GameID: chi.URLParam(r, "gameId"),
		Market: r.URL.Query().Get("market"),
	}

	items, err := h.listService.Execute(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_ = json.NewEncoder(w).Encode(items)
}
```

```go
r.Route("/api/admin/games/{gameId}", func(r chi.Router) {
	r.Get("/market-channels", marketChannelHandler.ListMarketChannels)
	r.Post("/markets/{market}/channels", marketChannelHandler.CreateMarketChannel)
})
r.Post("/api/admin/game-market-channels/{id}/hide", marketChannelHandler.HideMarketChannel)
r.Post("/api/admin/game-market-channels/{id}/unhide", marketChannelHandler.UnhideMarketChannel)
```

- [ ] **Step 4: Re-run the HTTP handler tests and verify they pass**

Run: `cd services/admin-api && go test ./internal/transport/http/channels -run 'Test(CreateMarketChannelRejectsDomesticChannelForJP|ListMarketChannelsReturnsAllMarketsByDefault)' -v`

Expected: PASS.

- [ ] **Step 5: Commit the backend HTTP surface**

```bash
git add services/admin-api/internal/transport/http/channels/market_channel_handler.go services/admin-api/internal/transport/http/channels/router.go services/admin-api/internal/transport/http/admin/router.go
git commit -m "feat: expose market channel admin endpoints"
```

### Task 7: Build the Frontend All-Market Channel Overview

**Files:**
- Create: `apps/admin-web/src/api/gameMarketChannels.ts`
- Create: `apps/admin-web/src/views/games/detail/ChannelInstancesTab.vue`
- Create: `apps/admin-web/src/views/games/detail/components/ChannelInstanceTable.vue`
- Create: `apps/admin-web/src/views/games/detail/components/ChannelInstanceStatusTag.vue`
- Create: `apps/admin-web/src/views/games/detail/components/ChannelInstanceRuntimeFlags.vue`
- Create: `apps/admin-web/src/views/games/detail/__tests__/channel-instance-table.spec.ts`

- [ ] **Step 1: Write the failing component test for the all-market default view**

```ts
import { render, screen } from "@testing-library/vue";
import ChannelInstanceTable from "../components/ChannelInstanceTable.vue";

test("shows all market rows by default", async () => {
  render(ChannelInstanceTable, {
    props: {
      items: [
        { market: "GLOBAL", channelId: "google", configStatus: "valid" },
        { market: "JP", channelId: "google", configStatus: "invalid" }
      ],
      marketFilter: "ALL"
    }
  });

  screen.getByText("GLOBAL");
  screen.getByText("JP");
});
```

- [ ] **Step 2: Run the frontend component test and verify it fails**

Run: `pnpm --dir apps/admin-web vitest run src/views/games/detail/__tests__/channel-instance-table.spec.ts`

Expected: FAIL with missing table component or props.

- [ ] **Step 3: Implement the API client, table, tags, and runtime flags**

```ts
export interface GameMarketChannelItem {
  gameId: string;
  market: "GLOBAL" | "JP" | "KR" | "SEA" | "HMT" | "CN";
  channelId: string;
  configStatus: "empty" | "invalid" | "valid";
  hidden: boolean;
  incompatibleWithMarket: boolean;
  includedInSnapshot: boolean;
  includedInSync: boolean;
  includedInRuntimeConfig: boolean;
}
```

```vue
<script setup lang="ts">
const props = defineProps<{
  items: GameMarketChannelItem[];
  marketFilter: string;
}>();

const filteredItems = computed(() =>
  props.marketFilter === "ALL"
    ? props.items
    : props.items.filter(item => item.market === props.marketFilter)
);
</script>
```

- [ ] **Step 4: Re-run the component test and verify it passes**

Run: `pnpm --dir apps/admin-web vitest run src/views/games/detail/__tests__/channel-instance-table.spec.ts`

Expected: PASS.

- [ ] **Step 5: Commit the all-market overview UI**

```bash
git add apps/admin-web/src/api/gameMarketChannels.ts apps/admin-web/src/views/games/detail/ChannelInstancesTab.vue apps/admin-web/src/views/games/detail/components/ChannelInstanceTable.vue apps/admin-web/src/views/games/detail/components/ChannelInstanceStatusTag.vue apps/admin-web/src/views/games/detail/components/ChannelInstanceRuntimeFlags.vue apps/admin-web/src/views/games/detail/__tests__/channel-instance-table.spec.ts
git commit -m "feat: add all market channel overview"
```

### Task 8: Build the Frontend Create/Copy/Hide Channel Flows

**Files:**
- Create: `apps/admin-web/src/views/games/detail/components/CreateMarketChannelDrawer.vue`
- Create: `apps/admin-web/src/views/games/detail/__tests__/create-market-channel-drawer.spec.ts`
- Modify: `apps/admin-web/src/views/games/detail/ChannelInstancesTab.vue`
- Modify: `apps/admin-web/src/views/games/detail/components/ChannelInstanceTable.vue`

- [ ] **Step 1: Write the failing create-drawer tests**

```ts
import { render, fireEvent, screen } from "@testing-library/vue";
import CreateMarketChannelDrawer from "../components/CreateMarketChannelDrawer.vue";

test("copy mode clears secret and file fields", async () => {
  render(CreateMarketChannelDrawer, {
    props: {
      selectedMarket: "JP",
      sourceInstance: {
        market: "GLOBAL",
        channelId: "google",
        normalConfig: { clientId: "global-id" }
      }
    }
  });

  await fireEvent.click(screen.getByLabelText("Copy from existing market"));
  expect((screen.getByLabelText("clientSecret") as HTMLInputElement).value).toBe("");
  expect((screen.getByLabelText("keystoreFile") as HTMLInputElement).value).toBe("");
});
```

- [ ] **Step 2: Run the drawer tests and verify they fail**

Run: `pnpm --dir apps/admin-web vitest run src/views/games/detail/__tests__/create-market-channel-drawer.spec.ts`

Expected: FAIL with missing drawer implementation or copy behavior.

- [ ] **Step 3: Implement the create drawer and hide flow**

```vue
<script setup lang="ts">
const model = reactive({
  market: "JP",
  mode: "empty" as "empty" | "copy",
  normalConfig: {} as Record<string, unknown>,
  secretConfig: {} as Record<string, string>,
  fileConfig: {} as Record<string, string>
});

function applyCopy(source: SourceInstance) {
  model.normalConfig = structuredClone(source.normalConfig);
  model.secretConfig = {};
  model.fileConfig = {};
}
</script>
```

```ts
async function hideChannel(id: string) {
  await post(`/api/admin/game-market-channels/${id}/hide`);
  await reload();
}
```

- [ ] **Step 4: Re-run the drawer tests and verify they pass**

Run: `pnpm --dir apps/admin-web vitest run src/views/games/detail/__tests__/create-market-channel-drawer.spec.ts`

Expected: PASS.

- [ ] **Step 5: Commit the create/copy/hide UI**

```bash
git add apps/admin-web/src/views/games/detail/components/CreateMarketChannelDrawer.vue apps/admin-web/src/views/games/detail/__tests__/create-market-channel-drawer.spec.ts apps/admin-web/src/views/games/detail/ChannelInstancesTab.vue apps/admin-web/src/views/games/detail/components/ChannelInstanceTable.vue
git commit -m "feat: add market channel copy and hide flows"
```

### Task 9: Build Template Version Copy UI and Section Sync UI

**Files:**
- Create: `apps/admin-web/src/api/templateVersions.ts`
- Create: `apps/admin-web/src/api/syncSections.ts`
- Create: `apps/admin-web/src/views/cashier/templates/TemplateVersionsTab.vue`
- Create: `apps/admin-web/src/views/cashier/templates/components/CopyPublishedToDraftDialog.vue`
- Create: `apps/admin-web/src/views/games/detail/components/SyncSectionDrawer.vue`
- Create: `apps/admin-web/src/views/games/detail/__tests__/sync-section-drawer.spec.ts`

- [ ] **Step 1: Write the failing sync-drawer and template-copy tests**

```ts
import { render, fireEvent, screen } from "@testing-library/vue";
import SyncSectionDrawer from "../components/SyncSectionDrawer.vue";

test("execute payload only includes selected sections", async () => {
  render(SyncSectionDrawer, {
    props: { preview: [{ section: "channels" }, { section: "payments" }] }
  });

  await fireEvent.click(screen.getByLabelText("channels"));
  await fireEvent.click(screen.getByText("Execute"));
  screen.getByText("selected_sections: channels");
});
```

```ts
test("published version can be copied into draft", async () => {
  render(CopyPublishedToDraftDialog, {
    props: { sourceVersion: { version: 7, status: "published" } }
  });

  screen.getByText("Create draft from published v7");
});
```

- [ ] **Step 2: Run the frontend tests and verify they fail**

Run: `pnpm --dir apps/admin-web vitest run src/views/games/detail/__tests__/sync-section-drawer.spec.ts`

Expected: FAIL with missing sync drawer or template copy dialog.

- [ ] **Step 3: Implement the sync drawer and template copy UI**

```ts
export interface SyncExecutePayload {
  selected_sections: Array<"game" | "markets" | "legal" | "channels" | "packages" | "products" | "cashier" | "payments" | "config">;
}
```

```vue
<script setup lang="ts">
const selectedSections = ref<string[]>([]);

const executePayload = computed(() => ({
  selected_sections: selectedSections.value
}));
</script>
```

```vue
<template>
  <el-button type="primary" @click="copyToDraft(sourceVersion)">
    Create draft from published v{{ sourceVersion.version }}
  </el-button>
</template>
```

- [ ] **Step 4: Re-run the frontend tests and verify they pass**

Run: `pnpm --dir apps/admin-web vitest run src/views/games/detail/__tests__/sync-section-drawer.spec.ts`

Expected: PASS.

- [ ] **Step 5: Commit the template and sync UI**

```bash
git add apps/admin-web/src/api/templateVersions.ts apps/admin-web/src/api/syncSections.ts apps/admin-web/src/views/cashier/templates/TemplateVersionsTab.vue apps/admin-web/src/views/cashier/templates/components/CopyPublishedToDraftDialog.vue apps/admin-web/src/views/games/detail/components/SyncSectionDrawer.vue apps/admin-web/src/views/games/detail/__tests__/sync-section-drawer.spec.ts
git commit -m "feat: add template copy and section sync ui"
```

### Task 10: Run Final Backend and Frontend Verification

**Files:**
- Modify: `docs/architecture/zh-CN/go_domain_api_draft.md`
- Modify: `docs/architecture/zh-CN/backend_agent_execution.md`
- Modify: `docs/architecture/zh-CN/frontend_agent_execution.md`

- [ ] **Step 1: Update the architecture docs to match the implemented behavior**

```md
- `GET /api/admin/games/{gameId}/market-channels`
- `POST /api/admin/games/{gameId}/markets/{market}/channels`
- `POST /api/admin/game-market-channels/{id}/hide`
- `POST /api/admin/game-market-channels/{id}/unhide`
```

- [ ] **Step 2: Run backend verification**

Run: `cd services/admin-api && go test ./...`

Expected: PASS for domain, app, and transport layers.

- [ ] **Step 3: Run frontend verification**

Run: `pnpm --dir apps/admin-web vitest run`

Expected: PASS for channel overview, create drawer, and sync drawer tests.

- [ ] **Step 4: Smoke-test the combined flow manually**

Run:

```bash
cd services/admin-api && go test ./internal/transport/http/... -v
pnpm --dir apps/admin-web vitest run src/views/games/detail/__tests__/channel-instance-table.spec.ts src/views/games/detail/__tests__/create-market-channel-drawer.spec.ts src/views/games/detail/__tests__/sync-section-drawer.spec.ts
```

Expected:

- `JP` market rejects domestic channels
- copied market channels start in `invalid`
- hidden channels disappear from the default list
- `selected_sections` only sends checked sections
- template copy action creates a new `draft`

- [ ] **Step 5: Commit the final verified slice**

```bash
git add docs/architecture/zh-CN/go_domain_api_draft.md docs/architecture/zh-CN/backend_agent_execution.md docs/architecture/zh-CN/frontend_agent_execution.md
git commit -m "docs: align architecture docs with market channel implementation"
```
