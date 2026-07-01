package payment

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	paymentapp "github.com/csw/console/services/admin-api/internal/app/payment"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
	"github.com/go-chi/chi/v5"
)

const paymentTestEnv = common.EnvSandbox

type harness struct {
	router http.Handler
	store  *paymentMemStore
	issuer *infrajwt.Issuer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	issuer, err := infrajwt.NewIssuer(infrajwt.Config{
		Secret:     "test-secret-please-change",
		Issuer:     "admin-api",
		AccessTTL:  30 * time.Minute,
		RefreshTTL: 336 * time.Hour,
	})
	if err != nil {
		t.Fatalf("new issuer: %v", err)
	}
	store := newPaymentMemStore()
	service := paymentapp.NewService(store, fakeCrypto{}, &fakeAudit{}, func() time.Time { return time.Unix(1700000000, 0).UTC() })

	root := chi.NewRouter()
	sub := chi.NewRouter()
	RegisterRoutes(sub, NewHandler(service), issuer, paymentTestEnv, slog.New(slog.NewTextHandler(io.Discard, nil)), true, nil)
	root.Mount("/api/admin", sub)
	return &harness{router: root, store: store, issuer: issuer}
}

func (h *harness) token(t *testing.T, userID int64, perms []string) string {
	t.Helper()
	pair, err := h.issuer.IssuePair(domainauth.NewAuthContext(userID, "tester", "Tester", []string{"tester"}, perms, paymentTestEnv))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return pair.AccessToken
}

func (h *harness) readToken(t *testing.T) string {
	return h.token(t, 100, []string{"payment.read"})
}

func (h *harness) writeToken(t *testing.T) string {
	return h.token(t, 101, []string{"payment.read", "payment.write"})
}

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
		if code, ok := e["code"].(string); ok {
			return code
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
		t.Fatalf("status want %d got %d body=%s", want, got.status, got.raw)
	}
}

func TestPaymentRBAC(t *testing.T) {
	h := newHarness(t)
	endpoints := []struct {
		method string
		path   string
		body   any
	}{
		{method: http.MethodGet, path: "/api/admin/pay-ways"},
		{method: http.MethodGet, path: "/api/admin/cashier/merchant-accounts"},
		{
			method: http.MethodPut,
			path:   "/api/admin/games/100001/payment-routes",
			body: map[string]any{"items": []map[string]any{
				{"payWayId": "credit_card", "providerId": "airwallex", "merchantAccountId": "merchant_aw_main"},
			}},
		},
	}

	for _, ep := range endpoints {
		res := h.do(t, ep.method, ep.path, "", ep.body)
		assertStatus(t, res, http.StatusUnauthorized)
		if res.errCode() != "UNAUTHENTICATED" {
			t.Fatalf("%s %s want UNAUTHENTICATED got %q", ep.method, ep.path, res.errCode())
		}
	}

	noWrite := h.token(t, 102, []string{"payment.read"})
	res := h.do(t, http.MethodPut, "/api/admin/games/100001/payment-routes", noWrite, map[string]any{
		"items": []map[string]any{{"payWayId": "credit_card", "providerId": "airwallex", "merchantAccountId": "merchant_aw_main"}},
	})
	assertStatus(t, res, http.StatusForbidden)
	if res.errCode() != "FORBIDDEN" {
		t.Fatalf("want FORBIDDEN got %q", res.errCode())
	}
}

func TestListMerchantAccountsMasksSecret(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/cashier/merchant-accounts", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	items, _ := res.data()["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected at least one merchant account")
	}
	first := items[0].(map[string]any)
	if first["secret"] != "masked" {
		t.Fatalf("secret must be masked, got %v", first["secret"])
	}
}

func TestPutGameRoutesConflictKinds(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	t.Run("duplicate priority", func(t *testing.T) {
		res := h.do(t, http.MethodPut, "/api/admin/games/100001/payment-routes", token, map[string]any{
			"items": []map[string]any{
				{"payWayId": "credit_card", "providerId": "airwallex", "merchantAccountId": "merchant_aw_main", "priority": 10, "marketCode": "JP"},
				{"payWayId": "credit_card", "providerId": "payermax", "merchantAccountId": "merchant_pm_main", "priority": 10, "marketCode": "GLOBAL"},
			},
		})
		assertStatus(t, res, http.StatusConflict)
		if res.errCode() != "ROUTE_CONFLICT" {
			t.Fatalf("want ROUTE_CONFLICT got %q", res.errCode())
		}
	})

	t.Run("duplicate selector", func(t *testing.T) {
		res := h.do(t, http.MethodPut, "/api/admin/games/100001/payment-routes", token, map[string]any{
			"items": []map[string]any{
				{"payWayId": "credit_card", "providerId": "airwallex", "merchantAccountId": "merchant_aw_main", "priority": 10, "marketCode": "*", "channelId": nil},
				{"payWayId": "credit_card", "providerId": "payermax", "merchantAccountId": "merchant_pm_main", "priority": 11, "marketCode": "", "channelId": "*"},
			},
		})
		assertStatus(t, res, http.StatusConflict)
		if res.errCode() != "ROUTE_CONFLICT" {
			t.Fatalf("want ROUTE_CONFLICT got %q", res.errCode())
		}
	})
}

func TestGetProviderTemplate(t *testing.T) {
	h := newHarness(t)
	token := h.readToken(t)

	t.Run("provider 有模板返回四件套", func(t *testing.T) {
		res := h.do(t, http.MethodGet, "/api/admin/cashier/providers/airwallex/template", token, nil)
		assertStatus(t, res, http.StatusOK)
		data := res.data()
		if data["templateVersion"] != "3" {
			t.Fatalf("templateVersion mismatch: %v", data["templateVersion"])
		}
		form, _ := data["formSchema"].([]any)
		if len(form) != 2 {
			t.Fatalf("want 2 form fields got %d body=%s", len(form), res.raw)
		}
		secrets, _ := data["secretFields"].([]any)
		if len(secrets) != 1 || secrets[0] != "api_key" {
			t.Fatalf("unexpected secretFields: %v", data["secretFields"])
		}
		if _, ok := data["fileFields"]; !ok {
			t.Fatalf("fileFields missing: %s", res.raw)
		}
		if _, ok := data["validationRules"]; !ok {
			t.Fatalf("validationRules missing: %s", res.raw)
		}
	})

	t.Run("provider 无模板返回 404 供前端降级", func(t *testing.T) {
		res := h.do(t, http.MethodGet, "/api/admin/cashier/providers/payermax/template", token, nil)
		assertStatus(t, res, http.StatusNotFound)
		if res.errCode() != "NOT_FOUND" {
			t.Fatalf("want NOT_FOUND got %q", res.errCode())
		}
	})

	t.Run("provider 不存在返回 404", func(t *testing.T) {
		res := h.do(t, http.MethodGet, "/api/admin/cashier/providers/ghost/template", token, nil)
		assertStatus(t, res, http.StatusNotFound)
	})

	t.Run("无鉴权 401", func(t *testing.T) {
		res := h.do(t, http.MethodGet, "/api/admin/cashier/providers/airwallex/template", "", nil)
		assertStatus(t, res, http.StatusUnauthorized)
	})
}

func TestListPayWaysPaginationEnvelope(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/pay-ways", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	data := res.data()
	if _, ok := data["total"].(float64); !ok {
		t.Fatalf("total missing in list envelope: %s", res.raw)
	}
	if data["page"] != float64(1) || data["pageSize"] != float64(20) {
		t.Fatalf("page/pageSize mismatch: %v/%v", data["page"], data["pageSize"])
	}
	items, _ := data["items"].([]any)
	if data["total"] != float64(len(items)) {
		t.Fatalf("total should equal item count: total=%v items=%d", data["total"], len(items))
	}
}

func TestPutGameRoutesConflictDetailsCarryPositions(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPut, "/api/admin/games/100001/payment-routes", h.writeToken(t), map[string]any{
		"items": []map[string]any{
			{"payWayId": "credit_card", "providerId": "airwallex", "merchantAccountId": "merchant_aw_main", "priority": 10, "marketCode": "JP"},
			{"payWayId": "credit_card", "providerId": "payermax", "merchantAccountId": "merchant_pm_main", "priority": 10, "marketCode": "GLOBAL"},
		},
	})
	assertStatus(t, res, http.StatusConflict)
	errBody, _ := res.body["error"].(map[string]any)
	details, _ := errBody["details"].([]any)
	if len(details) == 0 {
		t.Fatalf("expected conflict details, body=%s", res.raw)
	}
	detail := details[0].(map[string]any)
	if detail["kind"] != "duplicate_priority" {
		t.Fatalf("want duplicate_priority got %v", detail["kind"])
	}
	if detail["leftIndex"] != float64(0) || detail["rightIndex"] != float64(1) {
		t.Fatalf("want leftIndex=0 rightIndex=1 got %v/%v", detail["leftIndex"], detail["rightIndex"])
	}
}

func TestPutGameRoutesTransactionRollback(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	before := h.do(t, http.MethodGet, "/api/admin/games/100001/payment-routes", token, nil)
	assertStatus(t, before, http.StatusOK)

	h.store.mu.Lock()
	h.store.state.failReplaceAfterDelete = true
	h.store.mu.Unlock()

	failRes := h.do(t, http.MethodPut, "/api/admin/games/100001/payment-routes", token, map[string]any{
		"items": []map[string]any{
			{"payWayId": "credit_card", "providerId": "payermax", "merchantAccountId": "merchant_pm_main", "priority": 10, "marketCode": "JP"},
		},
	})
	if failRes.status < http.StatusInternalServerError {
		t.Fatalf("want server failure got %d body=%s", failRes.status, failRes.raw)
	}

	h.store.mu.Lock()
	h.store.state.failReplaceAfterDelete = false
	h.store.mu.Unlock()

	after := h.do(t, http.MethodGet, "/api/admin/games/100001/payment-routes", token, nil)
	assertStatus(t, after, http.StatusOK)
	if before.raw != after.raw {
		t.Fatalf("routes should roll back to original before=%s after=%s", before.raw, after.raw)
	}
}

func TestGetAndPutGameRoutesSuccess(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	put := h.do(t, http.MethodPut, "/api/admin/games/100001/payment-routes", token, map[string]any{
		"items": []map[string]any{
			{
				"payWayId": "credit_card", "providerId": "payermax", "merchantAccountId": "merchant_pm_main",
				"priority": 10, "marketCode": "JP", "currency": "JPY", "channelId": "googleplay", "packageCode": "pkg.jp",
			},
			{
				"payWayId": "credit_card", "providerId": "airwallex", "merchantAccountId": "merchant_aw_main",
				"priority": 100, "marketCode": "GLOBAL",
			},
		},
	})
	assertStatus(t, put, http.StatusOK)

	get := h.do(t, http.MethodGet, "/api/admin/games/100001/payment-routes", token, nil)
	assertStatus(t, get, http.StatusOK)
	data := get.data()
	if data["gameId"] != "100001" {
		t.Fatalf("gameId mismatch: %v", data["gameId"])
	}
	groups, _ := data["groups"].([]any)
	if len(groups) == 0 {
		t.Fatal("expected at least one route group")
	}
	group := groups[0].(map[string]any)
	routes, _ := group["routes"].([]any)
	if len(routes) != 2 {
		t.Fatalf("want 2 routes got %d", len(routes))
	}
	first := routes[0].(map[string]any)
	selector := first["selector"].(map[string]any)
	if selector["marketCode"] != "JP" || selector["packageCode"] != "pkg.jp" {
		t.Fatalf("unexpected first selector: %v", selector)
	}
}
