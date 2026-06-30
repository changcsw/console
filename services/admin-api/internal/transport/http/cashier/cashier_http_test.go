package cashier

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	cashierapp "github.com/csw/console/services/admin-api/internal/app/cashier"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
)

// 进程内 L3 接口测试（httptest 全链路 transport->app->domain + 内存仓储 + 真实 JWT）。
// 与 tests/backend/scenarios/cashier-template.yaml 维度对齐，等价覆盖 S1/S2/S3/S4/S5/S7/S9/S10
// 以及模块私有维度（状态机流转、copy-to-draft、金额归一化、FX 人工/自动确认）。
// 本模块为平台级（schema=platform，业务行无 env 列）→ S6 由 schema 隔离 + search_path 说明承担，
// 真实 PG 专属断言由连库 harness（SCENARIO_WITH_DB=1）承担。

const testEnv = common.EnvDevelop

type harness struct {
	router http.Handler
	store  *memStore
	issuer *infrajwt.Issuer
	audit  *fakeAudit
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	issuer, err := infrajwt.NewIssuer(infrajwt.Config{
		Secret: "test-secret-please-change", Issuer: "admin-api",
		AccessTTL: 30 * time.Minute, RefreshTTL: 336 * time.Hour,
	})
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	store := newMemStore()
	audit := &fakeAudit{}
	svc := cashierapp.NewService(store, audit, func() time.Time { return time.Unix(1700000000, 0).UTC() })

	root := chi.NewRouter()
	sub := chi.NewRouter()
	RegisterRoutes(sub, NewHandler(svc), issuer, testEnv, slog.New(slog.NewTextHandler(io.Discard, nil)), true, nil)
	root.Mount("/api/admin", sub)

	return &harness{router: root, store: store, issuer: issuer, audit: audit}
}

var allPerms = []string{"cashier.read", "cashier.write", "cashier.publish", "fx.approve"}

func (h *harness) token(t *testing.T, userID int64, perms []string) string {
	t.Helper()
	pair, err := h.issuer.IssuePair(domainauth.NewAuthContext(userID, "tester", "Tester", []string{"editor"}, perms, testEnv))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return pair.AccessToken
}

func (h *harness) writeToken(t *testing.T) string { return h.token(t, 11, allPerms) }
func (h *harness) readToken(t *testing.T) string  { return h.token(t, 10, []string{"cashier.read"}) }

type apiResp struct {
	status int
	body   map[string]any
	raw    string
}

func (h *harness) do(t *testing.T, method, path, token string, body any) apiResp {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	out := apiResp{status: rec.Code, raw: rec.Body.String()}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
	}
	return out
}

func (r apiResp) errCode() string {
	if e, ok := r.body["error"].(map[string]any); ok {
		if c, ok := e["code"].(string); ok {
			return c
		}
	}
	return ""
}

func (r apiResp) data() map[string]any {
	if d, ok := r.body["data"].(map[string]any); ok {
		return d
	}
	return nil
}

func assertStatus(t *testing.T, got apiResp, want int) {
	t.Helper()
	if got.status != want {
		t.Fatalf("status: want %d got %d (body=%s)", want, got.status, got.raw)
	}
}

// ───────────────────────── 测试夹具构建（走真实 API） ─────────────────────────

func (h *harness) createTemplate(t *testing.T, id, name, mode string) apiResp {
	t.Helper()
	res := h.do(t, http.MethodPost, "/api/admin/cashier/templates", h.writeToken(t), map[string]any{
		"templateId": id, "templateName": name, "fxSyncEnabled": true, "fxSyncMode": mode, "fxSyncSchedule": "monthly",
	})
	assertStatus(t, res, http.StatusCreated)
	return res
}

// createVersion 创建一个空白 draft 版本，返回其 version。
func (h *harness) createVersion(t *testing.T, templateID string) int {
	t.Helper()
	res := h.do(t, http.MethodPost, "/api/admin/cashier/templates/"+templateID+"/versions", h.writeToken(t), map[string]any{})
	assertStatus(t, res, http.StatusCreated)
	v, _ := res.data()["version"].(float64)
	return int(v)
}

func (h *harness) upsertUSDRow(t *testing.T, templateID string, version int, pre, rate string) apiResp {
	t.Helper()
	return h.do(t, http.MethodPut, versionPath(templateID, version, "rows"), h.writeToken(t), map[string]any{
		"rows": []map[string]any{{
			"countryCode": "US", "regionCode": "*", "currency": "USD", "priceId": "p_basic",
			"preTaxAmount": pre, "taxRate": rate, "effectiveAt": "2026-01-01T00:00:00Z",
		}},
	})
}

func (h *harness) publish(t *testing.T, templateID string, version int) apiResp {
	t.Helper()
	return h.do(t, http.MethodPost, versionPath(templateID, version, "publish"), h.writeToken(t), nil)
}

func versionPath(templateID string, version int, suffix string) string {
	return "/api/admin/cashier/templates/" + templateID + "/versions/" + itoa(version) + "/" + suffix
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// ───────────────────────── S2/S3 鉴权与权限 ─────────────────────────

func TestCashierRBAC(t *testing.T) {
	h := newHarness(t)

	// S2：无令牌 → 401（Authn 在 RequireBackend 之前）。
	for _, ep := range []struct{ m, p string }{
		{http.MethodGet, "/api/admin/cashier/templates"},
		{http.MethodPost, "/api/admin/cashier/templates"},
		{http.MethodGet, "/api/admin/cashier/templates/global_default"},
		{http.MethodPost, "/api/admin/cashier/templates/global_default/versions"},
		{http.MethodPost, "/api/admin/cashier/fx-sync-runs/1/approve"},
	} {
		res := h.do(t, ep.m, ep.p, "", nil)
		assertStatus(t, res, http.StatusUnauthorized)
		if res.errCode() != "UNAUTHENTICATED" {
			t.Fatalf("%s %s want UNAUTHENTICATED got %q", ep.m, ep.p, res.errCode())
		}
	}

	// S2：伪造 Bearer → 401。
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/cashier/templates", "not.a.jwt", nil), http.StatusUnauthorized)

	// S3：读令牌可读列表，但不能写/发布/审核 → 403 FORBIDDEN。
	readOnly := h.readToken(t)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/cashier/templates", readOnly, nil), http.StatusOK)
	for _, ep := range []struct {
		m, p string
		body any
	}{
		{http.MethodPost, "/api/admin/cashier/templates", map[string]any{"templateId": "x", "templateName": "X"}},
		{http.MethodPost, "/api/admin/cashier/templates/x/versions", map[string]any{}},
		{http.MethodPost, "/api/admin/cashier/templates/x/versions/1/publish", nil},
		{http.MethodPost, "/api/admin/cashier/fx-sync-runs/1/approve", map[string]any{}},
	} {
		res := h.do(t, ep.m, ep.p, readOnly, ep.body)
		assertStatus(t, res, http.StatusForbidden)
		if res.errCode() != "FORBIDDEN" {
			t.Fatalf("%s %s want FORBIDDEN got %q", ep.m, ep.p, res.errCode())
		}
	}

	// 缺 cashier.read → 列表 403。
	noPerm := h.token(t, 12, []string{"system.read"})
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/cashier/templates", noPerm, nil), http.StatusForbidden)

	// S3：publish 需 cashier.publish；仅 cashier.write 不足。
	writerNoPublish := h.token(t, 13, []string{"cashier.read", "cashier.write"})
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/cashier/templates/x/versions/1/publish", writerNoPublish, nil), http.StatusForbidden)
}

// ───────────────────────── S1/S4/S5/S7 模板创建 ─────────────────────────

func TestCreateTemplateSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	res := h.createTemplate(t, "global_default", "Global Default", "manual_confirm")
	d := res.data()
	if d["templateId"] != "global_default" || d["status"] != "draft" {
		t.Fatalf("create result wrong: %v", d)
	}
	// S7：写审计 cashier.template.create。
	if _, ok := h.audit.byAction("cashier.template.create"); !ok {
		t.Fatal("expected cashier.template.create audit")
	}
}

func TestCreateTemplateValidation(t *testing.T) {
	h := newHarness(t)
	tok := h.writeToken(t)
	cases := []map[string]any{
		{"templateName": "X"}, // 缺 templateId
		{"templateId": "x"},   // 缺 templateName
		{"templateId": "x", "templateName": "X", "fxSyncMode": "bogus"},      // 非法 mode
		{"templateId": "x", "templateName": "X", "fxSyncSchedule": "weekly"}, // 非法 schedule
	}
	for i, body := range cases {
		res := h.do(t, http.MethodPost, "/api/admin/cashier/templates", tok, body)
		assertStatus(t, res, http.StatusBadRequest)
		if res.errCode() != "VALIDATION_FAILED" {
			t.Fatalf("case %d want VALIDATION_FAILED got %q (%s)", i, res.errCode(), res.raw)
		}
	}
}

// S5：templateId 唯一 → CONFLICT。
func TestCreateTemplateConflict(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "dup", "First", "manual_confirm")
	res := h.do(t, http.MethodPost, "/api/admin/cashier/templates", h.writeToken(t), map[string]any{
		"templateId": "dup", "templateName": "Second", "fxSyncMode": "manual_confirm", "fxSyncSchedule": "monthly",
	})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}
}

func TestGetTemplateNotFound(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/cashier/templates/ghost", h.readToken(t), nil)
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", res.errCode())
	}
}

// ───────────────────────── S9 分页 ─────────────────────────

func TestListTemplatesPaginationAndClamp(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.createTemplate(t, "tpl_"+itoa(i), "T"+itoa(i), "manual_confirm")
	}
	res := h.do(t, http.MethodGet, "/api/admin/cashier/templates?page=1&pageSize=99999", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	if res.data()["pageSize"] != float64(100) {
		t.Fatalf("pageSize must clamp to 100, got %v", res.data()["pageSize"])
	}
	if res.data()["total"] != float64(3) || res.data()["page"] != float64(1) {
		t.Fatalf("page/total wrong: %v", res.data())
	}
}

// ───────────────────────── 版本创建 / copy-to-draft（私有维度 + S4/S5） ─────────────────────────

func TestCreateVersionDefaultsToDraft(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	res := h.do(t, http.MethodPost, "/api/admin/cashier/templates/t1/versions", h.writeToken(t), map[string]any{})
	assertStatus(t, res, http.StatusCreated)
	if res.data()["status"] != "draft" || res.data()["version"] != float64(1) {
		t.Fatalf("first version must be draft v1, got %v", res.data())
	}
}

// S4：copy 来源但缺 sourceVersion → VALIDATION_FAILED。
func TestCreateVersionCopyMissingSourceVersion(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	res := h.do(t, http.MethodPost, "/api/admin/cashier/templates/t1/versions", h.writeToken(t), map[string]any{
		"sourceType": "copy_published",
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q", res.errCode())
	}
}

// S5：从 draft 复制（来源非 published/archived）→ VERSION_STATE_INVALID。
func TestCreateVersionCopyFromDraftRejected(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1") // draft
	res := h.do(t, http.MethodPost, "/api/admin/cashier/templates/t1/versions", h.writeToken(t), map[string]any{
		"sourceType": "copy_published", "sourceVersion": itoa(v1), // compact：sourceVersion 为字符串
	})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "VERSION_STATE_INVALID" {
		t.Fatalf("want VERSION_STATE_INVALID got %q", res.errCode())
	}
}

// copy-to-draft：产物恒 draft，source_type 标记来源（published → copy_published）。
func TestCopyToDraftFromPublished(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	assertStatus(t, h.upsertUSDRow(t, "t1", v1, "10.00", "0.1"), http.StatusOK)
	assertStatus(t, h.publish(t, "t1", v1), http.StatusOK)

	res := h.do(t, http.MethodPost, versionPath("t1", v1, "copy-to-draft"), h.writeToken(t), nil)
	assertStatus(t, res, http.StatusCreated)
	if res.data()["status"] != "draft" {
		t.Fatalf("copy-to-draft must produce draft, got %v", res.data())
	}
	if res.data()["sourceType"] != "copy_published" {
		t.Fatalf("sourceType must be copy_published, got %v", res.data()["sourceType"])
	}
	// 复制了价格行：新 draft 行与来源等量。
	newV := int(res.data()["version"].(float64))
	rows := h.do(t, http.MethodGet, versionPath("t1", newV, "rows"), h.readToken(t), nil)
	items, _ := rows.data()["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("copy-to-draft must copy rows, got %v", items)
	}
}

func TestCopyToDraftFromArchived(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	assertStatus(t, h.publish(t, "t1", v1), http.StatusOK)
	v2 := h.createVersion(t, "t1")
	assertStatus(t, h.publish(t, "t1", v2), http.StatusOK) // v1 自动归档

	res := h.do(t, http.MethodPost, versionPath("t1", v1, "copy-to-draft"), h.writeToken(t), nil)
	assertStatus(t, res, http.StatusCreated)
	if res.data()["sourceType"] != "copy_archived" {
		t.Fatalf("sourceType must be copy_archived, got %v", res.data()["sourceType"])
	}
}

// ───────────────────────── 金额归一化（S1/S4/CURRENCY_NOT_SUPPORTED） ─────────────────────────

func TestUpsertRowsNormalizesAmount(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	res := h.upsertUSDRow(t, "t1", v1, "10.00", "0.1")
	assertStatus(t, res, http.StatusOK)
	items, _ := res.data()["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 row, got %v", items)
	}
	row := items[0].(map[string]any)
	if row["preTaxAmountMinor"] != float64(1000) || row["taxAmountMinor"] != float64(100) || row["afterTaxAmountMinor"] != float64(1100) {
		t.Fatalf("normalization wrong: %v", row)
	}
}

func TestUpsertRowsCurrencyNotSupported(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	res := h.do(t, http.MethodPut, versionPath("t1", v1, "rows"), h.writeToken(t), map[string]any{
		"rows": []map[string]any{{
			"countryCode": "DE", "currency": "EUR", "priceId": "p", "preTaxAmount": "10.00", "taxRate": "0", "effectiveAt": "2026-01-01T00:00:00Z",
		}},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "CURRENCY_NOT_SUPPORTED" {
		t.Fatalf("want CURRENCY_NOT_SUPPORTED got %q (%s)", res.errCode(), res.raw)
	}
}

func TestUpsertRowsBelowMinimum(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	res := h.upsertUSDRow(t, "t1", v1, "0.10", "0") // 10 minor < min 50
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q (%s)", res.errCode(), res.raw)
	}
}

// S5：published 版本只读 → VERSION_STATE_INVALID。
func TestUpsertRowsOnPublishedRejected(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	assertStatus(t, h.upsertUSDRow(t, "t1", v1, "10.00", "0.1"), http.StatusOK)
	assertStatus(t, h.publish(t, "t1", v1), http.StatusOK)

	res := h.upsertUSDRow(t, "t1", v1, "12.00", "0.1")
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "VERSION_STATE_INVALID" {
		t.Fatalf("want VERSION_STATE_INVALID got %q", res.errCode())
	}
}

// ───────────────────────── 发布状态机（S1/S5/S7） ─────────────────────────

func TestPublishArchivesOldPublishedSameTx(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	assertStatus(t, h.publish(t, "t1", v1), http.StatusOK)
	v2 := h.createVersion(t, "t1")
	assertStatus(t, h.publish(t, "t1", v2), http.StatusOK)

	// S7：审计 cashier.version.publish。
	if _, ok := h.audit.byAction("cashier.version.publish"); !ok {
		t.Fatal("expected cashier.version.publish audit")
	}

	got := h.do(t, http.MethodGet, "/api/admin/cashier/templates/t1", h.readToken(t), nil)
	versions, _ := got.data()["versions"].([]any)
	statuses := map[int]string{}
	for _, vi := range versions {
		v := vi.(map[string]any)
		statuses[int(v["version"].(float64))] = v["status"].(string)
	}
	if statuses[v1] != "archived" {
		t.Fatalf("old published v%d must be archived, got %s", v1, statuses[v1])
	}
	if statuses[v2] != "published" {
		t.Fatalf("new v%d must be published, got %s", v2, statuses[v2])
	}
}

// S5：发布非 draft（已 published）→ VERSION_STATE_INVALID。
func TestPublishNonDraftRejected(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	assertStatus(t, h.publish(t, "t1", v1), http.StatusOK)
	res := h.publish(t, "t1", v1) // 再次发布 published 版本
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "VERSION_STATE_INVALID" {
		t.Fatalf("want VERSION_STATE_INVALID got %q", res.errCode())
	}
}

// S10：发布事务中第二步（PublishVersion）失败 → 整体回滚（旧 published 不被归档）。
func TestPublishTransactionRollback(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	assertStatus(t, h.publish(t, "t1", v1), http.StatusOK)
	v2 := h.createVersion(t, "t1")

	h.store.state.forcePublishErr = errors.New("boom")
	res := h.publish(t, "t1", v2)
	if res.status < 400 {
		t.Fatalf("expected failure, got %d (%s)", res.status, res.raw)
	}

	// 回滚断言：v1 仍 published，v2 仍 draft（archive 未生效）。
	h.store.state.forcePublishErr = nil
	got := h.do(t, http.MethodGet, "/api/admin/cashier/templates/t1", h.readToken(t), nil)
	versions, _ := got.data()["versions"].([]any)
	for _, vi := range versions {
		v := vi.(map[string]any)
		ver := int(v["version"].(float64))
		if ver == v1 && v["status"] != "published" {
			t.Fatalf("rollback failed: v%d must stay published, got %s", v1, v["status"])
		}
		if ver == v2 && v["status"] != "draft" {
			t.Fatalf("rollback failed: v%d must stay draft, got %s", v2, v["status"])
		}
	}
}

// ───────────────────────── FX 汇率同步（私有维度 + S7/S10） ─────────────────────────

// manual_confirm：trigger 仅生成候选 draft + run(pending_review)，不自动应用。
func TestFXManualConfirmTriggerThenApprove(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	assertStatus(t, h.upsertUSDRow(t, "t1", v1, "10.00", "0.1"), http.StatusOK)
	assertStatus(t, h.publish(t, "t1", v1), http.StatusOK)

	// trigger。
	run := h.do(t, http.MethodPost, "/api/admin/cashier/templates/t1/fx-sync/runs", h.writeToken(t), nil)
	assertStatus(t, run, http.StatusCreated)
	if run.data()["status"] != "pending_review" {
		t.Fatalf("manual_confirm trigger must yield pending_review, got %v", run.data()["status"])
	}
	runID := int64(run.data()["runId"].(float64))

	// approve → applied（同事务发布候选 + 归档旧 published）。
	appr := h.do(t, http.MethodPost, "/api/admin/cashier/fx-sync-runs/"+itoa(int(runID))+"/approve", h.writeToken(t), map[string]any{"reviewNote": "ok"})
	assertStatus(t, appr, http.StatusOK)
	if appr.data()["status"] != "applied" {
		t.Fatalf("approve must apply, got %v", appr.data())
	}
	// S7：审计 fx.approve。
	if _, ok := h.audit.byAction("fx.approve"); !ok {
		t.Fatal("expected fx.approve audit")
	}
	// 旧 published v1 归档。
	got := h.do(t, http.MethodGet, "/api/admin/cashier/templates/t1", h.readToken(t), nil)
	versions, _ := got.data()["versions"].([]any)
	for _, vi := range versions {
		v := vi.(map[string]any)
		if int(v["version"].(float64)) == v1 && v["status"] != "archived" {
			t.Fatalf("approve must archive old published v%d, got %s", v1, v["status"])
		}
	}
}

// auto_apply：trigger 后自动 approve → run applied + 候选 published。
func TestFXAutoApplyTriggerPublishes(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "auto_apply")
	v1 := h.createVersion(t, "t1")
	assertStatus(t, h.upsertUSDRow(t, "t1", v1, "10.00", "0.1"), http.StatusOK)
	assertStatus(t, h.publish(t, "t1", v1), http.StatusOK)

	run := h.do(t, http.MethodPost, "/api/admin/cashier/templates/t1/fx-sync/runs", h.writeToken(t), nil)
	assertStatus(t, run, http.StatusCreated)
	// P4 修复后：auto_apply 触发响应在事务内 reload run，status 与落库一致（applied），
	// 且 DTO 返回候选版本号（candidateVersion 字符串，非内部 id）。
	if run.data()["status"] != "applied" {
		t.Fatalf("auto_apply trigger must reflect applied status, got %v", run.data()["status"])
	}
	if cv, _ := run.data()["candidateVersion"].(string); cv == "" {
		t.Fatalf("auto_apply trigger must return candidateVersion, got %v", run.data()["candidateVersion"])
	}

	// 断言真实落库效果：auto_apply 下候选版本被发布、旧 published v1 被归档。
	got := h.do(t, http.MethodGet, "/api/admin/cashier/templates/t1", h.readToken(t), nil)
	versions, _ := got.data()["versions"].([]any)
	var published, archivedV1 bool
	for _, vi := range versions {
		v := vi.(map[string]any)
		ver := int(v["version"].(float64))
		if ver == v1 && v["status"] == "archived" {
			archivedV1 = true
		}
		if ver != v1 && v["status"] == "published" {
			published = true
		}
	}
	if !published {
		t.Fatalf("auto_apply must publish candidate version, versions=%v", versions)
	}
	if !archivedV1 {
		t.Fatalf("auto_apply must archive old published v%d, versions=%v", v1, versions)
	}
}

// FX ignore：approve 端点 action=ignore → run ignored，不发布候选、不归档旧 published。
func TestFXIgnore(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	assertStatus(t, h.publish(t, "t1", v1), http.StatusOK)

	run := h.do(t, http.MethodPost, "/api/admin/cashier/templates/t1/fx-sync/runs", h.writeToken(t), nil)
	assertStatus(t, run, http.StatusCreated)
	runID := int(run.data()["runId"].(float64))

	res := h.do(t, http.MethodPost, "/api/admin/cashier/fx-sync-runs/"+itoa(runID)+"/approve", h.writeToken(t), map[string]any{"action": "ignore"})
	assertStatus(t, res, http.StatusOK)
	if res.data()["status"] != "ignored" {
		t.Fatalf("ignore must yield ignored, got %v", res.data())
	}
	// v1 仍 published（未被归档）。
	got := h.do(t, http.MethodGet, "/api/admin/cashier/templates/t1", h.readToken(t), nil)
	versions, _ := got.data()["versions"].([]any)
	for _, vi := range versions {
		v := vi.(map[string]any)
		if int(v["version"].(float64)) == v1 && v["status"] != "published" {
			t.Fatalf("ignore must not archive v%d, got %s", v1, v["status"])
		}
	}
}

// S10：FX approve 同事务在发布候选失败 → 整体回滚（run 仍 pending_review，旧 published 不归档）。
func TestFXApproveTransactionRollback(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "t1", "T1", "manual_confirm")
	v1 := h.createVersion(t, "t1")
	assertStatus(t, h.publish(t, "t1", v1), http.StatusOK)

	run := h.do(t, http.MethodPost, "/api/admin/cashier/templates/t1/fx-sync/runs", h.writeToken(t), nil)
	assertStatus(t, run, http.StatusCreated)
	runID := int(run.data()["runId"].(float64))

	h.store.state.forcePublishErr = errors.New("boom")
	res := h.do(t, http.MethodPost, "/api/admin/cashier/fx-sync-runs/"+itoa(runID)+"/approve", h.writeToken(t), map[string]any{})
	if res.status < 400 {
		t.Fatalf("expected approve failure, got %d (%s)", res.status, res.raw)
	}

	// 回滚断言：旧 published v1 仍 published。
	h.store.state.forcePublishErr = nil
	got := h.do(t, http.MethodGet, "/api/admin/cashier/templates/t1", h.readToken(t), nil)
	versions, _ := got.data()["versions"].([]any)
	for _, vi := range versions {
		v := vi.(map[string]any)
		if int(v["version"].(float64)) == v1 && v["status"] != "published" {
			t.Fatalf("rollback failed: v%d must stay published, got %s", v1, v["status"])
		}
	}
}
