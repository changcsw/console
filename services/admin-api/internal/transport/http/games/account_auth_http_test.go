package games

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	accountauthapp "github.com/csw/console/services/admin-api/internal/app/accountauth"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
)

// 进程内 L3 接口测试（httptest 全链路 transport->app(accountauth)->domain + 内存仓储 + 真实 JWT
// + 真实"加密语义" spy cipher + spy AuditSink）。与 tests/backend/scenarios/account-auth.yaml 维度对齐，
// 等价覆盖 S1/S3/S4/S5/S7/S8/S10。S2 鉴权 401 由 scenario manifest 进程内执行；S6 schema 隔离由连库 harness 承担。

const gameID = "100001"

type aaHarness struct {
	router http.Handler
	store  *aaStore
	cipher *spyCipher
	audit  *fakeAudit
	issuer *infrajwt.Issuer
}

func newAAHarness(t *testing.T) *aaHarness {
	t.Helper()
	issuer, err := infrajwt.NewIssuer(infrajwt.Config{
		Secret: "test-secret-please-change", Issuer: "admin-api",
		AccessTTL: 30 * time.Minute, RefreshTTL: 336 * time.Hour,
	})
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	store := newAAStore()
	cipher := &spyCipher{}
	audit := &fakeAudit{}
	// now 固定，便于断言 lastCheckAt 仅在 valid 时被赋值。
	now := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	svc := accountauthapp.NewService(store, cipher, audit, now)

	root := chi.NewRouter()
	sub := chi.NewRouter()
	// game svc 传 nil：本测试只打 account-auth 路由，不触达 game 编排。
	RegisterRoutes(sub, NewHandler(nil, testEnv, svc), issuer, testEnv, slog.New(slog.NewTextHandler(io.Discard, nil)), true, nil)
	root.Mount("/api/admin", sub)

	return &aaHarness{router: root, store: store, cipher: cipher, audit: audit, issuer: issuer}
}

func (h *aaHarness) token(t *testing.T, userID int64, perms []string) string {
	t.Helper()
	pair, err := h.issuer.IssuePair(domainauth.NewAuthContext(userID, "tester", "Tester", []string{"editor"}, perms, testEnv))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return pair.AccessToken
}

func (h *aaHarness) readToken(t *testing.T) string { return h.token(t, 20, []string{"game.read"}) }
func (h *aaHarness) writeToken(t *testing.T) string {
	return h.token(t, 21, []string{"game.read", "game.write"})
}
func (h *aaHarness) noPermToken(t *testing.T) string { return h.token(t, 22, []string{"system.read"}) }

func (h *aaHarness) do(t *testing.T, method, path, token string, body any) apiResp {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = strings.NewReader(string(b))
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

// items 取响应体 data.items 数组。
func (r apiResp) items() []any {
	if d := r.data(); d != nil {
		if it, ok := d["items"].([]any); ok {
			return it
		}
	}
	return nil
}

// itemByAuthType 在 data.items 中按 authTypeId 找一项。
func (r apiResp) itemByAuthType(authTypeID string) map[string]any {
	for _, raw := range r.items() {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if m["authTypeId"] == authTypeID {
			return m
		}
	}
	return nil
}

const (
	typesPath        = "/api/admin/account-auth/types"
	channelTypesPath = "/api/admin/channels/ch_account/account-auth-types"
	configsPath      = "/api/admin/games/" + gameID + "/account-auth-configs"
)

// fullGoogleConfig 一份完整可通过 google 模板校验的配置。
func fullGoogleConfig(secret string) map[string]any {
	return map[string]any{
		"items": []map[string]any{
			{
				"authTypeId": "google",
				"enabled":    true,
				"configJson": map[string]any{
					"clientId":     "client-abc",
					"clientSecret": secret,
					"redirectUri":  "https://example.com/oauth/cb",
				},
			},
		},
	}
}

// seedGoogleValid 通过 PUT 建立一份 google valid 配置（含已加密密文），返回此时累计加密次数。
func (h *aaHarness) seedGoogleValid(t *testing.T, secret string) {
	t.Helper()
	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), fullGoogleConfig(secret))
	assertStatus(t, res, http.StatusOK)
	g := res.itemByAuthType("google")
	if g == nil || g["configStatus"] != "valid" {
		t.Fatalf("seed google must be valid, got %v", g)
	}
}

// ───────────────────────── GET /account-auth/types（S1，只读） ─────────────────────────

func TestListAccountAuthTypesSuccess(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodGet, typesPath, h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	if len(res.items()) == 0 {
		t.Fatalf("expected catalog items, got %s", res.raw)
	}
	g := res.itemByAuthType("google")
	if g == nil {
		t.Fatalf("expected google type in catalog, got %s", res.raw)
	}
	tpl, _ := g["template"].(map[string]any)
	if tpl == nil {
		t.Fatalf("google must carry template four-piece, got %v", g)
	}
	secrets, _ := tpl["secretFields"].([]any)
	if len(secrets) != 1 || secrets[0] != "clientSecret" {
		t.Fatalf("google template must declare secretFields=[clientSecret], got %v", tpl["secretFields"])
	}
	// 目录仅含字段定义（secretFields 名），绝不含任何密文值 → S8 在目录侧 N/A 但仍硬验无明文泄漏。
	if strings.Contains(res.raw, "topsecret") {
		t.Fatalf("catalog must not leak any secret value: %s", res.raw)
	}
}

// ───────────────────────── GET /channels/{id}/account-auth-types（S1/NOTFOUND） ─────────────────────────

func TestListChannelAccountAuthTypesSuccess(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodGet, channelTypesPath, h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	line := res.itemByAuthType("line")
	if line == nil || line["locked"] != true {
		t.Fatalf("channel policy must expose locked=true for line, got %v", line)
	}
	guest := res.itemByAuthType("guest")
	if guest == nil || guest["defaultEnabled"] != true {
		t.Fatalf("channel policy must expose defaultEnabled=true for guest, got %v", guest)
	}
}

func TestListChannelAccountAuthTypesNotFound(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/channels/ghost/account-auth-types", h.readToken(t), nil)
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", res.errCode())
	}
}

// ───────────────────────── GET /games/{id}/account-auth-configs（S1/NOTFOUND/S8） ─────────────────────────

func TestGetGameAccountAuthConfigsSuccessMergesAllowed(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodGet, configsPath, h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	// 合并 = 渠道允许集合并集（guest/phone/google/apple/line）。
	for _, want := range []string{"guest", "phone", "google", "apple", "line"} {
		if res.itemByAuthType(want) == nil {
			t.Fatalf("configs must include allowed type %q, got %s", want, res.raw)
		}
	}
	// 未配置时默认态：phone defaultEnabled=true → enabled=true；line locked → enabled=true。
	if res.itemByAuthType("phone")["enabled"] != true {
		t.Fatalf("phone defaultEnabled should surface enabled=true")
	}
	if res.itemByAuthType("line")["enabled"] != true {
		t.Fatalf("locked line should surface enabled=true")
	}
	// 默认态 configStatus=empty。
	if res.itemByAuthType("guest")["configStatus"] != "empty" {
		t.Fatalf("unconfigured guest should be empty, got %v", res.itemByAuthType("guest"))
	}
}

func TestGetGameAccountAuthConfigsNotFound(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/games/999999/account-auth-configs", h.readToken(t), nil)
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", res.errCode())
	}
}

// S8：已配置 google 的密文在读接口恒脱敏为 masked，绝不回明文。
func TestGetGameAccountAuthConfigsMasksSecret(t *testing.T) {
	h := newAAHarness(t)
	h.seedGoogleValid(t, "topsecret-plain")

	res := h.do(t, http.MethodGet, configsPath, h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	g := res.itemByAuthType("google")
	cfg, _ := g["configJson"].(map[string]any)
	if cfg["clientSecret"] != "masked" {
		t.Fatalf("clientSecret must be masked on read, got %v", cfg["clientSecret"])
	}
	if strings.Contains(res.raw, "topsecret-plain") || strings.Contains(res.raw, "enc:topsecret-plain") {
		t.Fatalf("read response must never leak plaintext/ciphertext secret: %s", res.raw)
	}
	// 非密文字段照常返回。
	if cfg["clientId"] != "client-abc" {
		t.Fatalf("non-secret field should be returned as-is, got %v", cfg["clientId"])
	}
}

// ───────────────────────── PUT /games/{id}/account-auth-configs（S1/S4/S5/S7/S8/S10） ─────────────────────────

// S1：启用空模板类型（guest）→ valid。
func TestReplaceConfigsEmptyTemplateValid(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{
		"items": []map[string]any{{"authTypeId": "guest", "enabled": true}},
	})
	assertStatus(t, res, http.StatusOK)
	g := res.itemByAuthType("guest")
	if g["enabled"] != true || g["configStatus"] != "valid" {
		t.Fatalf("enabled empty-template guest should be valid, got %v", g)
	}
}

// S4：缺 items 字段 → 400 VALIDATION_FAILED（handler 必填校验）。
func TestReplaceConfigsItemsRequired(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q (%s)", res.errCode(), res.raw)
	}
}

// S5：authTypeId 不在该游戏渠道允许集合 → 400 ACCOUNT_AUTH_TYPE_NOT_ALLOWED。
func TestReplaceConfigsTypeNotAllowed(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{
		"items": []map[string]any{{"authTypeId": "facebook", "enabled": true}},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "ACCOUNT_AUTH_TYPE_NOT_ALLOWED" {
		t.Fatalf("want ACCOUNT_AUTH_TYPE_NOT_ALLOWED got %q (%s)", res.errCode(), res.raw)
	}
	// 失败无副作用：未落任何配置。
	if _, ok := h.store.configFor(1, refGuest); ok {
		t.Fatalf("rejected request must not write any config")
	}
}

// S5：启用缺可用模板的类型（apple）→ 400 ACCOUNT_AUTH_TEMPLATE_NOT_FOUND。
func TestReplaceConfigsTemplateNotFound(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{
		"items": []map[string]any{{"authTypeId": "apple", "enabled": true}},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "ACCOUNT_AUTH_TEMPLATE_NOT_FOUND" {
		t.Fatalf("want ACCOUNT_AUTH_TEMPLATE_NOT_FOUND got %q (%s)", res.errCode(), res.raw)
	}
}

// 业务规则 3：仅启用未填密钥 → 落 invalid，并给出缺失项消息（不得静默 empty）。
func TestReplaceConfigsEnableWithoutSecretIsInvalid(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{
		"items": []map[string]any{{"authTypeId": "google", "enabled": true}},
	})
	assertStatus(t, res, http.StatusOK)
	g := res.itemByAuthType("google")
	if g["configStatus"] != "invalid" {
		t.Fatalf("enabled google without secret must be invalid, got %v", g)
	}
	if msg, _ := g["lastCheckMessage"].(string); !strings.Contains(msg, "clientSecret") {
		t.Fatalf("invalid message must flag clientSecret, got %q", msg)
	}
}

// S7：PUT 写一条 game.account_auth.update 审计；detail 记 before/after，且绝不含明文密钥。
func TestReplaceConfigsWritesAuditMaskedDetail(t *testing.T) {
	h := newAAHarness(t)
	h.seedGoogleValid(t, "topsecret-plain")

	e, ok := h.audit.byAction("game.account_auth.update")
	if !ok {
		t.Fatal("expected game.account_auth.update audit")
	}
	if e.ResourceType != "game" || e.ResourceID != gameID {
		t.Fatalf("audit resource mismatch: type=%q id=%q", e.ResourceType, e.ResourceID)
	}
	// detail.items 含 google 的 before/after 状态位。
	db, _ := json.Marshal(e.Detail)
	for _, frag := range []string{"google", "enabledAfter", "configStatusAfter", "configStatusBefore"} {
		if !strings.Contains(string(db), frag) {
			t.Fatalf("audit detail must record %q, got %s", frag, db)
		}
	}
	// 红线：审计 detail 绝不含明文/密文密钥值。
	if strings.Contains(string(db), "topsecret-plain") || strings.Contains(string(db), "enc:topsecret-plain") || strings.Contains(string(db), "clientSecret") {
		t.Fatalf("audit detail leaked secret material: %s", db)
	}
}

// S8：PUT 响应中 google 密文恒 masked，绝不回明文。
func TestReplaceConfigsResponseMasksSecret(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), fullGoogleConfig("topsecret-plain"))
	assertStatus(t, res, http.StatusOK)
	g := res.itemByAuthType("google")
	cfg, _ := g["configJson"].(map[string]any)
	if cfg["clientSecret"] != "masked" {
		t.Fatalf("response clientSecret must be masked, got %v", cfg["clientSecret"])
	}
	if strings.Contains(res.raw, "topsecret-plain") || strings.Contains(res.raw, "enc:topsecret-plain") {
		t.Fatalf("response must never leak plaintext/ciphertext: %s", res.raw)
	}
	// 仓储侧落的是密文（"enc:"+明文），绝非明文。
	it, ok := h.store.configFor(1, refGoogle)
	if !ok || it.ConfigJSON["clientSecret"] != "enc:topsecret-plain" {
		t.Fatalf("stored secret must be ciphertext, got %v", it.ConfigJSON["clientSecret"])
	}
}

// S10：ReplaceGameConfigs 中途失败 → 整体回滚，已有状态不变。
func TestReplaceConfigsTransactionRollback(t *testing.T) {
	h := newAAHarness(t)
	h.seedGoogleValid(t, "topsecret-plain")
	before, _ := h.store.configFor(1, refGoogle)

	// 注入失败，尝试改写（禁用 google）。
	h.store.state.failReplace = true
	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{
		"items": []map[string]any{{"authTypeId": "google", "enabled": false}},
	})
	if res.status != http.StatusInternalServerError {
		t.Fatalf("forced replace failure should map to 500, got %d (%s)", res.status, res.raw)
	}
	// 回滚断言：仓储仍为旧值（enabled + 同密文）。
	after, ok := h.store.configFor(1, refGoogle)
	if !ok || after.Enabled != true || after.ConfigJSON["clientSecret"] != before.ConfigJSON["clientSecret"] {
		t.Fatalf("state must roll back unchanged: before=%v after=%v", before, after)
	}
}

// ───────────────────────── 密文更新回归（重点） ─────────────────────────

// 只改 enabled（不带 configJson）→ 已存密文不变，且不重新加密。
func TestReplaceConfigsToggleEnabledKeepsCiphertext(t *testing.T) {
	h := newAAHarness(t)
	h.seedGoogleValid(t, "topsecret-plain")
	callsAfterSeed := h.cipher.encryptCalls

	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{
		"items": []map[string]any{{"authTypeId": "google", "enabled": false}},
	})
	assertStatus(t, res, http.StatusOK)

	it, _ := h.store.configFor(1, refGoogle)
	if it.Enabled != false {
		t.Fatalf("enabled should toggle to false")
	}
	if it.ConfigJSON["clientSecret"] != "enc:topsecret-plain" {
		t.Fatalf("ciphertext must remain unchanged, got %v", it.ConfigJSON["clientSecret"])
	}
	if h.cipher.encryptCalls != callsAfterSeed {
		t.Fatalf("toggling enabled must not re-encrypt: calls %d -> %d", callsAfterSeed, h.cipher.encryptCalls)
	}
}

// 只改非 secret 字段（configJson 不含 clientSecret）→ 密文保留不变，不重新加密。
func TestReplaceConfigsNonSecretFieldKeepsCiphertext(t *testing.T) {
	h := newAAHarness(t)
	h.seedGoogleValid(t, "topsecret-plain")
	callsAfterSeed := h.cipher.encryptCalls

	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{
		"items": []map[string]any{{
			"authTypeId": "google",
			"enabled":    true,
			"configJson": map[string]any{"clientId": "client-rotated", "redirectUri": "https://example.com/oauth/cb"},
		}},
	})
	assertStatus(t, res, http.StatusOK)

	it, _ := h.store.configFor(1, refGoogle)
	if it.ConfigJSON["clientSecret"] != "enc:topsecret-plain" {
		t.Fatalf("secret must be preserved when omitted, got %v", it.ConfigJSON["clientSecret"])
	}
	if it.ConfigJSON["clientId"] != "client-rotated" {
		t.Fatalf("non-secret field should update, got %v", it.ConfigJSON["clientId"])
	}
	if h.cipher.encryptCalls != callsAfterSeed {
		t.Fatalf("omitting secret must not re-encrypt: calls %d -> %d", callsAfterSeed, h.cipher.encryptCalls)
	}
}

// masked 提交 → 还原旧密文，不重新加密。
func TestReplaceConfigsMaskedSecretRestoresOldValue(t *testing.T) {
	h := newAAHarness(t)
	h.seedGoogleValid(t, "topsecret-plain")
	callsAfterSeed := h.cipher.encryptCalls

	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{
		"items": []map[string]any{{
			"authTypeId": "google",
			"enabled":    true,
			"configJson": map[string]any{"clientId": "client-abc", "clientSecret": "masked", "redirectUri": "https://example.com/oauth/cb"},
		}},
	})
	assertStatus(t, res, http.StatusOK)

	it, _ := h.store.configFor(1, refGoogle)
	if it.ConfigJSON["clientSecret"] != "enc:topsecret-plain" {
		t.Fatalf("masked submit must restore old ciphertext, got %v", it.ConfigJSON["clientSecret"])
	}
	if h.cipher.encryptCalls != callsAfterSeed {
		t.Fatalf("masked submit must not re-encrypt: calls %d -> %d", callsAfterSeed, h.cipher.encryptCalls)
	}
}

// 留空字符串提交 → 不修改旧密文，不重新加密。
func TestReplaceConfigsEmptySecretKeepsOldValue(t *testing.T) {
	h := newAAHarness(t)
	h.seedGoogleValid(t, "topsecret-plain")
	callsAfterSeed := h.cipher.encryptCalls

	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{
		"items": []map[string]any{{
			"authTypeId": "google",
			"enabled":    true,
			"configJson": map[string]any{"clientId": "client-abc", "clientSecret": "", "redirectUri": "https://example.com/oauth/cb"},
		}},
	})
	assertStatus(t, res, http.StatusOK)

	it, _ := h.store.configFor(1, refGoogle)
	if it.ConfigJSON["clientSecret"] != "enc:topsecret-plain" {
		t.Fatalf("empty submit must keep old ciphertext, got %v", it.ConfigJSON["clientSecret"])
	}
	if h.cipher.encryptCalls != callsAfterSeed {
		t.Fatalf("empty submit must not re-encrypt: calls %d -> %d", callsAfterSeed, h.cipher.encryptCalls)
	}
}

// 提交新密钥值 → 重新加密落新密文。
func TestReplaceConfigsNewSecretReEncrypts(t *testing.T) {
	h := newAAHarness(t)
	h.seedGoogleValid(t, "topsecret-plain")
	callsAfterSeed := h.cipher.encryptCalls

	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), fullGoogleConfig("rotated-secret"))
	assertStatus(t, res, http.StatusOK)

	it, _ := h.store.configFor(1, refGoogle)
	if it.ConfigJSON["clientSecret"] != "enc:rotated-secret" {
		t.Fatalf("new secret must be re-encrypted, got %v", it.ConfigJSON["clientSecret"])
	}
	if h.cipher.encryptCalls != callsAfterSeed+1 {
		t.Fatalf("new secret should encrypt exactly once more: calls %d -> %d", callsAfterSeed, h.cipher.encryptCalls)
	}
}

// 业务规则 4：locked 类型游戏侧不可关闭 → 提交 enabled=false 被忽略，仍启用。
func TestReplaceConfigsLockedTypeCannotBeDisabled(t *testing.T) {
	h := newAAHarness(t)
	res := h.do(t, http.MethodPut, configsPath, h.writeToken(t), map[string]any{
		"items": []map[string]any{{"authTypeId": "line", "enabled": false}},
	})
	assertStatus(t, res, http.StatusOK)
	line := res.itemByAuthType("line")
	if line["enabled"] != true {
		t.Fatalf("locked line must stay enabled despite enabled=false request, got %v", line)
	}
}

// ───────────────────────── RBAC（S2/S3）跨 4 接口 ─────────────────────────

func TestAccountAuthRBAC(t *testing.T) {
	h := newAAHarness(t)

	endpoints := []struct {
		m, p  string
		write bool
		body  any
	}{
		{http.MethodGet, typesPath, false, nil},
		{http.MethodGet, channelTypesPath, false, nil},
		{http.MethodGet, configsPath, false, nil},
		{http.MethodPut, configsPath, true, map[string]any{"items": []map[string]any{}}},
	}

	// S2：无令牌 → 401。
	for _, ep := range endpoints {
		res := h.do(t, ep.m, ep.p, "", ep.body)
		assertStatus(t, res, http.StatusUnauthorized)
		if res.errCode() != "UNAUTHENTICATED" {
			t.Fatalf("%s %s want UNAUTHENTICATED got %q", ep.m, ep.p, res.errCode())
		}
	}

	// S2：伪造令牌 → 401。
	bad := h.do(t, http.MethodGet, typesPath, "not.a.valid.jwt", nil)
	assertStatus(t, bad, http.StatusUnauthorized)

	// S3：缺 game.read 任何权限 → 403（读接口）。
	noPerm := h.noPermToken(t)
	for _, ep := range endpoints {
		if ep.write {
			continue
		}
		res := h.do(t, ep.m, ep.p, noPerm, ep.body)
		assertStatus(t, res, http.StatusForbidden)
		if res.errCode() != "FORBIDDEN" {
			t.Fatalf("%s %s want FORBIDDEN got %q", ep.m, ep.p, res.errCode())
		}
	}

	// S3：仅 game.read 读取 OK，但写 → 403。
	readOnly := h.readToken(t)
	assertStatus(t, h.do(t, http.MethodGet, typesPath, readOnly, nil), http.StatusOK)
	assertStatus(t, h.do(t, http.MethodGet, configsPath, readOnly, nil), http.StatusOK)

	putRes := h.do(t, http.MethodPut, configsPath, readOnly, map[string]any{"items": []map[string]any{}})
	assertStatus(t, putRes, http.StatusForbidden)
	if putRes.errCode() != "FORBIDDEN" {
		t.Fatalf("read-only PUT want FORBIDDEN got %q", putRes.errCode())
	}
}
