package games

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	gameapp "github.com/csw/console/services/admin-api/internal/app/game"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
)

// 进程内 L3 接口测试（httptest 全链路 transport->app->domain + 内存仓储 + 真实 JWT）。
// 与 tests/backend/scenarios/game.yaml 维度对齐，等价覆盖 S1/S2/S3/S4/S5/S7/S8/S9/S10。
// 真实 PG 专属断言（schema 隔离 S6、DB 唯一约束）由连库 harness（SCENARIO_WITH_DB=1）承担。

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
	svc := gameapp.NewGameService(store, rand.Reader, audit, testEnv)

	root := chi.NewRouter()
	sub := chi.NewRouter()
	RegisterRoutes(sub, NewHandler(svc, testEnv), issuer, testEnv, slog.New(slog.NewTextHandler(io.Discard, nil)), true)
	root.Mount("/api/admin", sub)

	return &harness{router: root, store: store, issuer: issuer, audit: audit}
}

func (h *harness) token(t *testing.T, userID int64, perms []string) string {
	t.Helper()
	pair, err := h.issuer.IssuePair(domainauth.NewAuthContext(userID, "tester", "Tester", []string{"editor"}, perms, testEnv))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return pair.AccessToken
}

func (h *harness) readToken(t *testing.T) string { return h.token(t, 10, []string{"game.read"}) }
func (h *harness) writeToken(t *testing.T) string {
	return h.token(t, 11, []string{"game.read", "game.write"})
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

// createGame 走真实 POST /games，返回创建响应（含一次性明文 gameSecret）。
func (h *harness) createGame(t *testing.T, body map[string]any) apiResp {
	t.Helper()
	res := h.do(t, http.MethodPost, "/api/admin/games", h.writeToken(t), body)
	assertStatus(t, res, http.StatusCreated)
	return res
}

// ───────────────────────── 创建（S1/S4/S5/S7/S8/S10） ─────────────────────────

func TestCreateGameSuccessReturnsPlaintextSecretOnce(t *testing.T) {
	h := newHarness(t)
	res := h.createGame(t, map[string]any{"name": "Demo", "alias": "demo-game"})
	d := res.data()

	// S1：落库 + 默认 GLOBAL 市场。
	if d["gameId"] != "100000" {
		t.Fatalf("gameId want 100000 got %v", d["gameId"])
	}
	if d["status"] != "draft" || d["defaultMarketCode"] != "GLOBAL" {
		t.Fatalf("defaults wrong: %v", d)
	}
	markets, _ := d["markets"].([]any)
	if len(markets) != 1 {
		t.Fatalf("expected default GLOBAL market, got %v", markets)
	}

	// S8：创建一次性返回明文 secret（secretMasked=false 且非 "masked"）。
	if d["secretMasked"] != false {
		t.Fatalf("create must return secretMasked=false, got %v", d["secretMasked"])
	}
	secret, _ := d["gameSecret"].(string)
	if secret == "" || secret == "masked" || len(secret) < 32 {
		t.Fatalf("create must return high-entropy plaintext secret, got %q", secret)
	}

	// S7：写审计 game.create。
	e, ok := h.audit.byAction("game.create")
	if !ok {
		t.Fatal("expected game.create audit")
	}
	// S8：审计 detail 不泄漏明文 secret。
	db, _ := json.Marshal(e.Detail)
	if strings.Contains(string(db), secret) {
		t.Fatal("audit detail leaked plaintext gameSecret")
	}
	if e.ResourceID != "100000" {
		t.Fatalf("audit resourceId want 100000, got %q", e.ResourceID)
	}
}

func TestCreateGameSecretMaskedOnDetailAndList(t *testing.T) {
	h := newHarness(t)
	created := h.createGame(t, map[string]any{"name": "Demo", "alias": "demo-game"})
	plaintext, _ := created.data()["gameSecret"].(string)

	// S8：详情恒脱敏。
	detail := h.do(t, http.MethodGet, "/api/admin/games/100000", h.readToken(t), nil)
	assertStatus(t, detail, http.StatusOK)
	if detail.data()["secretMasked"] != true || detail.data()["gameSecret"] != "masked" {
		t.Fatalf("detail must mask secret, got %v", detail.data())
	}
	if strings.Contains(detail.raw, plaintext) {
		t.Fatal("detail response leaked plaintext gameSecret")
	}

	// S8：列表绝不返回 gameSecret 字段（轻量摘要）。
	list := h.do(t, http.MethodGet, "/api/admin/games", h.readToken(t), nil)
	assertStatus(t, list, http.StatusOK)
	if strings.Contains(list.raw, "gameSecret") || strings.Contains(list.raw, plaintext) {
		t.Fatalf("list must not expose gameSecret: %s", list.raw)
	}
}

func TestCreateGameGameIDAutoIncrements(t *testing.T) {
	h := newHarness(t)
	a := h.createGame(t, map[string]any{"name": "A", "alias": "a1"})
	b := h.createGame(t, map[string]any{"name": "B", "alias": "b1"})
	if a.data()["gameId"] != "100000" || b.data()["gameId"] != "100001" {
		t.Fatalf("gameId must auto-increment from 100000: %v %v", a.data()["gameId"], b.data()["gameId"])
	}
}

func TestCreateGameValidation(t *testing.T) {
	h := newHarness(t)
	tok := h.writeToken(t)
	cases := []map[string]any{
		{"name": "", "alias": "ok"},                             // 空 name
		{"name": "X", "alias": "bad alias"},                     // alias 非法字符
		{"name": "X", "alias": ""},                              // 空 alias
		{"name": "X", "alias": "ok", "iconUrl": "not-a-url"},    // iconUrl 非 url
		{"name": "X", "alias": "ok", "defaultMarketCode": "US"}, // 非法 market
		{"name": "X", "alias": "ok", "status": "archived"},      // 非法 status
		{"name": "X", "alias": "ok", "markets": []string{"US"}}, // markets 含非法
	}
	for i, body := range cases {
		res := h.do(t, http.MethodPost, "/api/admin/games", tok, body)
		assertStatus(t, res, http.StatusBadRequest)
		if res.errCode() != "VALIDATION_FAILED" {
			t.Fatalf("case %d want VALIDATION_FAILED got %q (%s)", i, res.errCode(), res.raw)
		}
	}
}

func TestCreateGameAliasConflict(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "First", "alias": "dup"})
	dup := h.do(t, http.MethodPost, "/api/admin/games", h.writeToken(t), map[string]any{"name": "Second", "alias": "dup"})
	assertStatus(t, dup, http.StatusConflict)
	if dup.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", dup.errCode())
	}
	// 冲突无副作用：仍只有一个游戏。
	list := h.do(t, http.MethodGet, "/api/admin/games", h.readToken(t), nil)
	if total := list.data()["total"]; total != float64(1) {
		t.Fatalf("conflict must not create second game, total=%v", total)
	}
}

func TestCreateGameWithExplicitMarketsInjectsDefault(t *testing.T) {
	h := newHarness(t)
	res := h.createGame(t, map[string]any{
		"name": "M", "alias": "m1", "defaultMarketCode": "JP", "markets": []string{"JP", "KR"},
	})
	d := res.data()
	if d["defaultMarketCode"] != "JP" {
		t.Fatalf("defaultMarketCode want JP got %v", d["defaultMarketCode"])
	}
	markets, _ := d["markets"].([]any)
	defaults := 0
	for _, mi := range markets {
		m := mi.(map[string]any)
		if m["isDefault"] == true {
			defaults++
			if m["marketCode"] != "JP" {
				t.Fatalf("default must be JP, got %v", m["marketCode"])
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("exactly one default market, got %d", defaults)
	}
}

// S10：CreateGame 跨表写（games + 默认 market）；alias 冲突在事务内中止 → 整体回滚，无 game_id 占用。
func TestCreateGameTransactionRollbackOnConflict(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "First", "alias": "dup"}) // 100000
	dup := h.do(t, http.MethodPost, "/api/admin/games", h.writeToken(t), map[string]any{"name": "Second", "alias": "dup"})
	assertStatus(t, dup, http.StatusConflict)

	// 回滚断言：第二次创建未写任何行（下一个成功创建仍取 100001，非 100002）。
	ok := h.createGame(t, map[string]any{"name": "Third", "alias": "third"})
	if ok.data()["gameId"] != "100001" {
		t.Fatalf("rolled-back create must not consume a game_id; got %v", ok.data()["gameId"])
	}
}

// ───────────────────────── 详情（S1/S4-NOTFOUND/S8） ─────────────────────────

func TestGetGameNotFound(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/games/999999", h.readToken(t), nil)
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", res.errCode())
	}
}

func TestGetGameEchoesEnvironment(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "Demo", "alias": "demo"})
	res := h.do(t, http.MethodGet, "/api/admin/games/100000", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	if res.data()["environment"] != string(testEnv) {
		t.Fatalf("environment want %s got %v", testEnv, res.data()["environment"])
	}
}

// ───────────────────────── RBAC（S2/S3） ─────────────────────────

func TestGamesRBAC(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "Demo", "alias": "demo"})

	// S2：无令牌 → 401（Authn 在 RequireBackend 之前）。
	for _, ep := range []struct{ m, p string }{
		{http.MethodGet, "/api/admin/games"},
		{http.MethodPost, "/api/admin/games"},
		{http.MethodGet, "/api/admin/games/100000"},
		{http.MethodPatch, "/api/admin/games/100000"},
		{http.MethodPut, "/api/admin/games/100000/markets"},
		{http.MethodPut, "/api/admin/games/100000/legal-links"},
	} {
		res := h.do(t, ep.m, ep.p, "", nil)
		assertStatus(t, res, http.StatusUnauthorized)
		if res.errCode() != "UNAUTHENTICATED" {
			t.Fatalf("%s %s want UNAUTHENTICATED got %q", ep.m, ep.p, res.errCode())
		}
	}

	// S2：伪造 / 无效 Bearer → 401。
	bad := h.do(t, http.MethodGet, "/api/admin/games", "not.a.valid.jwt", nil)
	assertStatus(t, bad, http.StatusUnauthorized)

	// S3：登录但权限不足。
	readOnly := h.readToken(t)
	// 读权限可读列表/详情。
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/games", readOnly, nil), http.StatusOK)
	// 读权限不能写 → 403。
	for _, ep := range []struct {
		m, p string
		body any
	}{
		{http.MethodPost, "/api/admin/games", map[string]any{"name": "x", "alias": "x"}},
		{http.MethodPatch, "/api/admin/games/100000", map[string]any{"name": "y"}},
		{http.MethodPut, "/api/admin/games/100000/markets", map[string]any{"markets": []any{}}},
		{http.MethodPut, "/api/admin/games/100000/legal-links", map[string]any{"legalLinks": []any{}}},
	} {
		res := h.do(t, ep.m, ep.p, readOnly, ep.body)
		assertStatus(t, res, http.StatusForbidden)
		if res.errCode() != "FORBIDDEN" {
			t.Fatalf("%s %s want FORBIDDEN got %q", ep.m, ep.p, res.errCode())
		}
	}

	// 无 game.read → 列表 403。
	noPerm := h.token(t, 12, []string{"system.read"})
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/games", noPerm, nil), http.StatusForbidden)
}

// ───────────────────────── 编辑基础信息（S1/S4/S5/S7） ─────────────────────────

func TestUpdateGameSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "Old", "alias": "old"})
	newName := "New Name"
	res := h.do(t, http.MethodPatch, "/api/admin/games/100000", h.writeToken(t), map[string]any{"name": newName, "status": "active"})
	assertStatus(t, res, http.StatusOK)
	if res.data()["name"] != newName || res.data()["status"] != "active" {
		t.Fatalf("update not applied: %v", res.data())
	}
	if _, ok := h.audit.byAction("game.update"); !ok {
		t.Fatal("expected game.update audit")
	}
}

func TestUpdateGameValidation(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "Old", "alias": "old"})
	tok := h.writeToken(t)
	cases := []map[string]any{
		{"name": ""},
		{"alias": "bad alias"},
		{"status": "archived"},
		{"iconUrl": "not-a-url"},
		{"defaultMarketCode": "US"},
	}
	for i, body := range cases {
		res := h.do(t, http.MethodPatch, "/api/admin/games/100000", tok, body)
		assertStatus(t, res, http.StatusBadRequest)
		if res.errCode() != "VALIDATION_FAILED" {
			t.Fatalf("case %d want VALIDATION_FAILED got %q", i, res.errCode())
		}
	}
}

func TestUpdateGameDefaultMarketMustBeEnabled(t *testing.T) {
	h := newHarness(t)
	// 仅 GLOBAL 启用；切默认到未启用 market → VALIDATION_FAILED。
	h.createGame(t, map[string]any{"name": "G", "alias": "g"})
	res := h.do(t, http.MethodPatch, "/api/admin/games/100000", h.writeToken(t), map[string]any{"defaultMarketCode": "JP"})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" || !strings.Contains(res.raw, "enabled market") {
		t.Fatalf("want enabled-market validation, got %s", res.raw)
	}
}

func TestUpdateGameAliasConflict(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "A", "alias": "a1"}) // 100000
	h.createGame(t, map[string]any{"name": "B", "alias": "b1"}) // 100001
	// 100001 改 alias 为 a1 → 冲突。
	res := h.do(t, http.MethodPatch, "/api/admin/games/100001", h.writeToken(t), map[string]any{"alias": "a1"})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}
}

// ───────────────────────── 市场覆盖（S1/S4/S5/S7/S10） ─────────────────────────

func TestReplaceMarketsSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "G", "alias": "g"})
	res := h.do(t, http.MethodPut, "/api/admin/games/100000/markets", h.writeToken(t), map[string]any{
		"markets": []map[string]any{
			{"marketCode": "GLOBAL", "isDefault": false, "enabled": true},
			{"marketCode": "JP", "isDefault": true, "enabled": true, "defaultLocale": "ja-JP"},
		},
	})
	assertStatus(t, res, http.StatusOK)
	if res.data()["defaultMarketCode"] != "JP" {
		t.Fatalf("default must be rewritten to JP, got %v", res.data()["defaultMarketCode"])
	}
	if _, ok := h.audit.byAction("game.markets.update"); !ok {
		t.Fatal("expected game.markets.update audit")
	}
}

func TestReplaceMarketsValidation(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "G", "alias": "g"})
	tok := h.writeToken(t)

	// 多默认 → VALIDATION_FAILED（exactly one default）。
	multi := h.do(t, http.MethodPut, "/api/admin/games/100000/markets", tok, map[string]any{
		"markets": []map[string]any{
			{"marketCode": "GLOBAL", "isDefault": true},
			{"marketCode": "JP", "isDefault": true},
		},
	})
	assertStatus(t, multi, http.StatusBadRequest)
	if !strings.Contains(multi.raw, "exactly one default") {
		t.Fatalf("want exactly-one-default msg, got %s", multi.raw)
	}

	// 重复 market → VALIDATION_FAILED。
	dup := h.do(t, http.MethodPut, "/api/admin/games/100000/markets", tok, map[string]any{
		"markets": []map[string]any{
			{"marketCode": "GLOBAL", "isDefault": true},
			{"marketCode": "GLOBAL", "isDefault": false},
		},
	})
	assertStatus(t, dup, http.StatusBadRequest)

	// 缺 markets 字段 → 400（必填）。
	assertStatus(t, h.do(t, http.MethodPut, "/api/admin/games/100000/markets", tok, map[string]any{}), http.StatusBadRequest)

	// 默认市场 enabled=false → VALIDATION_FAILED（默认市场须 ∈ 已启用 markets，与 PATCH defaultMarketCode 语义一致）。
	disabledDefault := h.do(t, http.MethodPut, "/api/admin/games/100000/markets", tok, map[string]any{
		"markets": []map[string]any{
			{"marketCode": "GLOBAL", "isDefault": true, "enabled": false},
			{"marketCode": "JP", "isDefault": false, "enabled": true},
		},
	})
	assertStatus(t, disabledDefault, http.StatusBadRequest)
	if disabledDefault.errCode() != "VALIDATION_FAILED" || !strings.Contains(disabledDefault.raw, "default market must be enabled") {
		t.Fatalf("want default-must-be-enabled validation, got %s", disabledDefault.raw)
	}
}

// S5 + S10：移除当前默认市场 → CONFLICT，且事务回滚（原市场集合不变）。
func TestReplaceMarketsRemoveDefaultConflictRollback(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "G", "alias": "g"}) // 默认 GLOBAL
	res := h.do(t, http.MethodPut, "/api/admin/games/100000/markets", h.writeToken(t), map[string]any{
		"markets": []map[string]any{
			{"marketCode": "JP", "isDefault": true, "enabled": true}, // 移除了旧默认 GLOBAL
		},
	})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}

	// 回滚断言：详情仍为原 GLOBAL 单市场，未部分写入 JP。
	detail := h.do(t, http.MethodGet, "/api/admin/games/100000", h.readToken(t), nil)
	if detail.data()["defaultMarketCode"] != "GLOBAL" {
		t.Fatalf("state must roll back to GLOBAL default, got %v", detail.data()["defaultMarketCode"])
	}
	markets, _ := detail.data()["markets"].([]any)
	if len(markets) != 1 || markets[0].(map[string]any)["marketCode"] != "GLOBAL" {
		t.Fatalf("markets must remain [GLOBAL] after rollback, got %v", markets)
	}
}

// S5 + S10：删除被渠道实例引用的 market → CONFLICT 且回滚（模拟 CountChannelsByMarket>0）。
func TestReplaceMarketsRemoveReferencedMarketConflict(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "G", "alias": "g", "defaultMarketCode": "GLOBAL", "markets": []string{"GLOBAL", "JP"}})
	// 模拟 JP 下已有渠道实例。
	h.store.state.channelCounts["100000"] = map[string]int{"JP": 2}

	res := h.do(t, http.MethodPut, "/api/admin/games/100000/markets", h.writeToken(t), map[string]any{
		"markets": []map[string]any{
			{"marketCode": "GLOBAL", "isDefault": true, "enabled": true}, // 试图移除被引用的 JP
		},
	})
	assertStatus(t, res, http.StatusConflict)
	if !strings.Contains(res.raw, "existing channels") {
		t.Fatalf("want existing-channels conflict msg, got %s", res.raw)
	}
	// 回滚：JP 仍在。
	detail := h.do(t, http.MethodGet, "/api/admin/games/100000", h.readToken(t), nil)
	markets, _ := detail.data()["markets"].([]any)
	if len(markets) != 2 {
		t.Fatalf("markets must remain 2 after rollback, got %v", markets)
	}
}

// ───────────────────────── 法务链接覆盖（S1/S4/S5/S7） ─────────────────────────

func TestReplaceLegalLinksSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "G", "alias": "g"})
	res := h.do(t, http.MethodPut, "/api/admin/games/100000/legal-links", h.writeToken(t), map[string]any{
		"legalLinks": []map[string]any{
			{"scopeType": "default", "scopeValue": "*", "termsUrl": "https://a.com/t"},
			{"scopeType": "market", "scopeValue": "JP", "privacyUrl": "https://a.com/p"},
			{"scopeType": "locale", "scopeValue": "en-US"},
		},
	})
	assertStatus(t, res, http.StatusOK)
	links, _ := res.data()["legalLinks"].([]any)
	if len(links) != 3 {
		t.Fatalf("expected 3 legal links, got %v", links)
	}
	if _, ok := h.audit.byAction("game.legal.update"); !ok {
		t.Fatal("expected game.legal.update audit")
	}
}

func TestReplaceLegalLinksScopeNormalization(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "G", "alias": "g"})
	// default 传空 scopeValue → 归一化为 '*'。
	res := h.do(t, http.MethodPut, "/api/admin/games/100000/legal-links", h.writeToken(t), map[string]any{
		"legalLinks": []map[string]any{
			{"scopeType": "default", "scopeValue": ""},
		},
	})
	assertStatus(t, res, http.StatusOK)
	links, _ := res.data()["legalLinks"].([]any)
	if links[0].(map[string]any)["scopeValue"] != "*" {
		t.Fatalf("default scopeValue must normalize to '*', got %v", links[0])
	}
}

func TestReplaceLegalLinksValidation(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "G", "alias": "g"})
	tok := h.writeToken(t)

	// 非法 scopeType。
	assertStatus(t, h.do(t, http.MethodPut, "/api/admin/games/100000/legal-links", tok, map[string]any{
		"legalLinks": []map[string]any{{"scopeType": "region", "scopeValue": "x"}},
	}), http.StatusBadRequest)

	// market scopeValue 非法枚举。
	assertStatus(t, h.do(t, http.MethodPut, "/api/admin/games/100000/legal-links", tok, map[string]any{
		"legalLinks": []map[string]any{{"scopeType": "market", "scopeValue": "US"}},
	}), http.StatusBadRequest)

	// 非法 URL。
	assertStatus(t, h.do(t, http.MethodPut, "/api/admin/games/100000/legal-links", tok, map[string]any{
		"legalLinks": []map[string]any{{"scopeType": "default", "termsUrl": "not-a-url"}},
	}), http.StatusBadRequest)

	// 缺 legalLinks 字段 → 400。
	assertStatus(t, h.do(t, http.MethodPut, "/api/admin/games/100000/legal-links", tok, map[string]any{}), http.StatusBadRequest)
}

// S5：(scopeType, scopeValue) 重复 → CONFLICT。
func TestReplaceLegalLinksDuplicateScopeConflict(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "G", "alias": "g"})
	res := h.do(t, http.MethodPut, "/api/admin/games/100000/legal-links", h.writeToken(t), map[string]any{
		"legalLinks": []map[string]any{
			{"scopeType": "market", "scopeValue": "JP"},
			{"scopeType": "market", "scopeValue": "JP"},
		},
	})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}
}

// ───────────────────────── 分页（S9） ─────────────────────────

func TestListGamesPaginationAndClamp(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 3; i++ {
		h.createGame(t, map[string]any{"name": "G", "alias": "g" + itoa(i)})
	}
	// pageSize 超上限 → 钳制为 100。
	res := h.do(t, http.MethodGet, "/api/admin/games?page=1&pageSize=99999&sort=-updatedAt", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	if res.data()["pageSize"] != float64(100) {
		t.Fatalf("pageSize must clamp to 100, got %v", res.data()["pageSize"])
	}
	if res.data()["total"] != float64(3) || res.data()["page"] != float64(1) {
		t.Fatalf("page/total wrong: %v", res.data())
	}
}

func TestListGamesFilterAndInvalidQuery(t *testing.T) {
	h := newHarness(t)
	h.createGame(t, map[string]any{"name": "Alpha", "alias": "alpha", "status": "active"})
	h.createGame(t, map[string]any{"name": "Beta", "alias": "beta", "status": "draft"})

	// keyword 过滤。
	kw := h.do(t, http.MethodGet, "/api/admin/games?keyword=alpha", h.readToken(t), nil)
	assertStatus(t, kw, http.StatusOK)
	if kw.data()["total"] != float64(1) {
		t.Fatalf("keyword filter want 1, got %v", kw.data()["total"])
	}

	// status 过滤。
	st := h.do(t, http.MethodGet, "/api/admin/games?status=active", h.readToken(t), nil)
	if st.data()["total"] != float64(1) {
		t.Fatalf("status filter want 1, got %v", st.data()["total"])
	}

	// S4：非法 status query → 400。
	bad := h.do(t, http.MethodGet, "/api/admin/games?status=bogus", h.readToken(t), nil)
	assertStatus(t, bad, http.StatusBadRequest)
	if bad.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q", bad.errCode())
	}

	// S4：非法 marketCode query → 400。
	badM := h.do(t, http.MethodGet, "/api/admin/games?marketCode=US", h.readToken(t), nil)
	assertStatus(t, badM, http.StatusBadRequest)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
