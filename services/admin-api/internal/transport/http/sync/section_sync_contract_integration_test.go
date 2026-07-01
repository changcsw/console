package syncapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/csw/console/services/admin-api/internal/app/command"
	domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"
	"github.com/go-chi/chi/v5"
)

// 🟪 集成测试专家：跨栈契约对账 e2e（进程内 httptest）。
// 目的：把后端 handler 实际写出的 JSON 包络/字段名/错误码，与前端 api/syncSections.ts
// 的 TS 契约逐项核对，锁定并行开发的契约漂移。连库维度（真实 upsert/审计）见 sync.yaml requiresDB。

type contractFakeService struct {
	preview domainsync.Preview
	execute domainsync.ExecuteResult
	jobs    domainsync.JobList
	err     error
}

func (f contractFakeService) Preview(context.Context, command.PreviewSectionSyncCommand) (domainsync.Preview, error) {
	return f.preview, f.err
}
func (f contractFakeService) Execute(context.Context, command.ExecuteSectionSyncCommand) (domainsync.ExecuteResult, error) {
	return f.execute, f.err
}
func (f contractFakeService) ListJobs(context.Context, command.ListSectionSyncJobsQuery) (domainsync.JobList, error) {
	return f.jobs, f.err
}

func routerWith(svc SectionSyncService) chi.Router {
	h := NewSectionSyncHandler(svc)
	r := chi.NewRouter()
	r.Post("/api/admin/games/{gameId}/sync/preview", h.Preview)
	r.Post("/api/admin/games/{gameId}/sync/execute", h.Execute)
	r.Get("/api/admin/games/{gameId}/sync-jobs", h.ListJobs)
	return r
}

func doJSON(t *testing.T, r chi.Router, method, path, body string) (int, map[string]any) {
	t.Helper()
	var rd *strings.Reader
	if body != "" {
		rd = strings.NewReader(body)
	} else {
		rd = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rd)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("响应非合法 JSON: %v; body=%s", err, rec.Body.String())
		}
	}
	return rec.Code, out
}

// preview 响应包络 + camelCase 字段 + 密文 masked（对账 SyncPreviewResponse / DiffChange）。
func TestContractPreviewEnvelopeAndMasking(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	svc := contractFakeService{preview: domainsync.Preview{
		GameID:           "100001",
		SourceEnv:        "sandbox",
		TargetEnv:        "production",
		SourceHash:       "sha256-src",
		TargetHashBefore: "sha256-dst",
		HasDiff:          true,
		BaselineToken:    "payload.sig",
		PreviewedAt:      now,
		ExpiresAt:        now.Add(30 * time.Minute),
		Sections: []domainsync.DiffSection{{
			Section:      domainsync.SectionChannels,
			Summary:      domainsync.DiffSummary{Add: 1, Update: 1},
			Dependencies: []domainsync.Section{domainsync.SectionGame, domainsync.SectionMarkets},
			Changes: []domainsync.DiffChange{
				{Op: domainsync.OpAdd, EntityType: "game_channel", EntityKey: "JP/google", FieldName: "*", SandboxValue: map[string]any{"enabled": true}},
				{Op: domainsync.OpUpdate, EntityType: "game_channel_login_config", EntityKey: "JP/google", FieldName: "clientSecret", SandboxValue: "masked", ProductionValue: "masked", Masked: true},
			},
		}},
	}}
	code, body := doJSON(t, routerWith(svc), http.MethodPost, "/api/admin/games/100001/sync/preview", `{"sections":["channels"]}`)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("缺少 data 包络: %v", body)
	}
	for _, k := range []string{"gameId", "sourceEnv", "targetEnv", "sourceHash", "targetHashBefore", "hasDiff", "baselineToken", "previewedAt", "expiresAt", "sections"} {
		if _, ok := data[k]; !ok {
			t.Fatalf("preview data 缺少前端契约字段 %q: %v", k, data)
		}
	}
	sections := data["sections"].([]any)
	change := sections[0].(map[string]any)["changes"].([]any)[1].(map[string]any)
	if change["masked"] != true || change["sandboxValue"] != "masked" || change["productionValue"] != "masked" {
		t.Fatalf("密文字段未按契约 masked: %v", change)
	}
	// 红线：明文串绝不出现在响应体
	raw := mustJSON(t, body)
	if strings.Contains(raw, "PLAINTEXT") {
		t.Fatalf("响应疑似泄漏明文: %s", raw)
	}
}

// execute 成功响应字段对账（SyncExecuteResponse）+ syncJobId 序列化类型漂移取证。
func TestContractExecuteResponseFields(t *testing.T) {
	now := time.Date(2026, 7, 1, 10, 5, 0, 0, time.UTC)
	svc := contractFakeService{execute: domainsync.ExecuteResult{
		SyncJobID:        9012,
		GameID:           "100001",
		SourceEnv:        "sandbox",
		TargetEnv:        "production",
		Status:           domainsync.JobStatusSucceeded,
		SelectedSections: []domainsync.Section{domainsync.SectionGame},
		IncludeDeletes:   false,
		SourceHash:       "sha256-src",
		TargetHashBefore: "sha256-dst",
		TargetHashAfter:  "sha256-after",
		AppliedSummary:   map[domainsync.Section]domainsync.DiffSummary{domainsync.SectionGame: {Update: 1}},
		Skipped:          domainsync.ExecuteSkipped{Deletes: []domainsync.ExecuteSkippedDelete{}, UnselectedSection: []domainsync.Section{domainsync.SectionConfig}},
		ExecutedAt:       now,
	}}
	code, body := doJSON(t, routerWith(svc), http.MethodPost, "/api/admin/games/100001/sync/execute", `{"selectedSections":["game"],"baselineToken":"payload.sig"}`)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	data := body["data"].(map[string]any)
	for _, k := range []string{"syncJobId", "gameId", "sourceEnv", "targetEnv", "status", "selectedSections", "includeDeletes", "sourceHash", "targetHashBefore", "targetHashAfter", "appliedSummary", "skipped", "executedAt"} {
		if _, ok := data[k]; !ok {
			t.Fatalf("execute data 缺少前端契约字段 %q: %v", k, data)
		}
	}
	// SYNC-INT-07 修复：后端 syncJobId 以 JSON string 序列化，与 compact 示例("9012")及前端 TS 声明一致。
	if s, isString := data["syncJobId"].(string); !isString || s == "" {
		t.Fatalf("syncJobId 必须为非空 JSON string（compact 契约）: %T=%v", data["syncJobId"], data["syncJobId"])
	}
}

// list 响应包络 + item 字段对账（SyncJobListItem vs compact 列表项）。
func TestContractListJobsItemFields(t *testing.T) {
	now := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	svc := contractFakeService{jobs: domainsync.JobList{
		Page:     1,
		PageSize: 20,
		Total:    1,
		Items: []domainsync.JobItem{{
			SyncJobID:        7001,
			GameID:           "100001",
			SourceEnv:        "sandbox",
			TargetEnv:        "production",
			Status:           "succeeded",
			IncludeDeletes:   false,
			OperatorID:       42,
			OperatorNote:     "release",
			SourceHash:       "sha256-src",
			TargetHashBefore: "sha256-dst",
			TargetHashAfter:  "sha256-after",
			ExecutedAt:       &now,
			CreatedAt:        now,
		}},
	}}
	code, body := doJSON(t, routerWith(svc), http.MethodGet, "/api/admin/games/100001/sync-jobs?page=1&pageSize=20", "")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	data := body["data"].(map[string]any)
	for _, k := range []string{"items", "page", "pageSize", "total"} {
		if _, ok := data[k]; !ok {
			t.Fatalf("list data 缺少分页包络字段 %q: %v", k, data)
		}
	}
	item := data["items"].([]any)[0].(map[string]any)
	// compact 列表项字段：后端必须提供
	for _, k := range []string{"syncJobId", "gameId", "sourceEnv", "targetEnv", "status", "includeDeletes", "operatorId", "operatorNote", "sourceHash", "targetHashBefore", "targetHashAfter", "executedAt", "createdAt"} {
		if _, ok := item[k]; !ok {
			t.Fatalf("list item 缺少 compact 字段 %q: %v", k, item)
		}
	}
	// 契约漂移取证：前端 SyncJobsTab 展示 selectedSections/appliedSummary，但 compact 列表项
	// 与后端 JobItem 均不返回（前端以可选链降级为 "-"/"{}")。若后端补齐则此处需同步放开。
	if _, present := item["selectedSections"]; present {
		t.Fatalf("后端 list item 已开始返回 selectedSections，请同步前端契约与本断言")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
