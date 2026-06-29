package channels

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	channelapp "github.com/csw/console/services/admin-api/internal/app/channel"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
)

// 进程内 L3 接口测试（httptest 全链路 transport->app->domain + 内存仓储 + 真实 JWT/路由/中间件）。
// 与 tests/backend/scenarios/channel.yaml 维度对齐，等价覆盖后端行为维度 S1/S3/S4/S5/S7/S9/S10，
// 并含 S2（无/坏令牌 401）。维度边界：
//   - S6（跨 env schema 隔离）：env 由连接 search_path 决定，本进程内层不建模 → N/A，由连库 harness 承担。
//   - S8（脱敏）：channel 自身 API 不返回任何密文/敏感载荷（DTO 不含 secret/file 配置）→ 本层 N/A，仅附带断言响应不泄漏内部配置载荷。

const testEnv = common.EnvDevelop

type harness struct {
	router http.Handler
	store  *memChannelStore
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
	store := newMemChannelStore()
	audit := &fakeAudit{}
	svc := channelapp.NewChannelService(store, func() time.Time { return time.Now() }, audit, testEnv)

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

func (h *harness) readToken(t *testing.T) string { return h.token(t, 10, []string{"channel.read"}) }
func (h *harness) writeToken(t *testing.T) string {
	return h.token(t, 11, []string{"channel.read", "channel.write"})
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

// createChannel 走真实 POST 创建渠道实例，返回创建响应。
func (h *harness) createChannel(t *testing.T, gameID, market string, body map[string]any) apiResp {
	t.Helper()
	return h.do(t, http.MethodPost, "/api/admin/games/"+gameID+"/markets/"+market+"/channels", h.writeToken(t), body)
}

// ───────────────────────── 创建渠道实例（S1/S4/S5/S7/S10） ─────────────────────────

func TestCreateMarketChannelSuccessBlank(t *testing.T) {
	h := newHarness(t)
	res := h.createChannel(t, "100001", "JP", map[string]any{"channelId": "google", "mode": "empty", "enabled": true, "remark": "seed"})
	assertStatus(t, res, http.StatusCreated)
	d := res.data()

	// S1：落库返回 gameChannelId + displayKey + 空白配置态。
	if d["gameChannelId"] == nil || d["gameChannelId"] == float64(0) {
		t.Fatalf("expected gameChannelId, got %v", d["gameChannelId"])
	}
	if d["displayKey"] != "100001:JP:google" {
		t.Fatalf("displayKey want 100001:JP:google got %v", d["displayKey"])
	}
	if d["configStatus"] != "empty" {
		t.Fatalf("blank create must be config_status=empty, got %v", d["configStatus"])
	}
	if d["copiedFromMarket"] != "" {
		t.Fatalf("blank create must not have copiedFromMarket, got %v", d["copiedFromMarket"])
	}

	// S7：写审计 channel.create，resourceId 与新实例 id 一致。
	e, ok := h.audit.byAction("channel.create")
	if !ok {
		t.Fatal("expected channel.create audit")
	}
	if e.ResourceType != "channel" {
		t.Fatalf("audit resourceType want channel, got %q", e.ResourceType)
	}
	if e.Detail["channelId"] != "google" || e.Detail["market"] != "JP" {
		t.Fatalf("audit detail mismatch: %v", e.Detail)
	}
}

func TestCreateMarketChannelCopyMarksInvalid(t *testing.T) {
	h := newHarness(t)
	// 先建 JP google（empty），再从 JP 复制到 KR。
	assertStatus(t, h.createChannel(t, "100001", "JP", map[string]any{"channelId": "google"}), http.StatusCreated)
	res := h.createChannel(t, "100001", "KR", map[string]any{"channelId": "google", "mode": "copy", "copyFromMarket": "JP"})
	assertStatus(t, res, http.StatusCreated)
	d := res.data()
	// S1：复制创建强制 config_status=invalid 且记 copiedFromMarket。
	if d["configStatus"] != "invalid" {
		t.Fatalf("copy create must be invalid, got %v", d["configStatus"])
	}
	if d["copiedFromMarket"] != "JP" {
		t.Fatalf("copiedFromMarket want JP, got %v", d["copiedFromMarket"])
	}
}

func TestCreateMarketChannelValidation(t *testing.T) {
	h := newHarness(t)
	tok := h.writeToken(t)
	cases := []struct {
		market string
		body   map[string]any
	}{
		{"JP", map[string]any{"channelId": ""}},                                 // 空 channelId
		{"JP", map[string]any{"channelId": "google", "mode": "copy"}},           // copy 缺 copyFromMarket
		{"JP", map[string]any{"channelId": "google", "mode": "bogus"}},          // 非法 mode
		{"JP", map[string]any{"channelId": "not-exist"}},                        // channelId 不存在
		{"JP", map[string]any{"channelId": "google", "remark": longString(256)}}, // remark 超长
	}
	for i, c := range cases {
		res := h.createChannel2(t, tok, "100001", c.market, c.body)
		assertStatus(t, res, http.StatusBadRequest)
		if res.errCode() != "VALIDATION_FAILED" {
			t.Fatalf("case %d want VALIDATION_FAILED got %q (%s)", i, res.errCode(), res.raw)
		}
	}

	// 非法 market（路径参数）→ 400 VALIDATION_FAILED。
	bad := h.createChannel2(t, tok, "100001", "US", map[string]any{"channelId": "google"})
	assertStatus(t, bad, http.StatusBadRequest)
	if bad.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("invalid market want VALIDATION_FAILED got %q", bad.errCode())
	}
}

// createChannel2 允许指定 token（用于校验/权限用例）。
func (h *harness) createChannel2(t *testing.T, token, gameID, market string, body map[string]any) apiResp {
	t.Helper()
	return h.do(t, http.MethodPost, "/api/admin/games/"+gameID+"/markets/"+market+"/channels", token, body)
}

// S4 + 模块私有错误码：market 与渠道 region 不兼容 → 400 MARKET_CHANNEL_INCOMPATIBLE。
func TestCreateMarketChannelIncompatible(t *testing.T) {
	h := newHarness(t)
	// google 为 overseas，CN 仅允许 domestic → 不兼容。
	res := h.createChannel(t, "100001", "CN", map[string]any{"channelId": "google"})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "MARKET_CHANNEL_INCOMPATIBLE" {
		t.Fatalf("want MARKET_CHANNEL_INCOMPATIBLE got %q (%s)", res.errCode(), res.raw)
	}
	// 反向：wechat 为 domestic，JP 仅允许 overseas → 不兼容。
	res2 := h.createChannel(t, "100001", "JP", map[string]any{"channelId": "wechat"})
	assertStatus(t, res2, http.StatusBadRequest)
	if res2.errCode() != "MARKET_CHANNEL_INCOMPATIBLE" {
		t.Fatalf("want MARKET_CHANNEL_INCOMPATIBLE got %q", res2.errCode())
	}
}

// S5：重复 (game, market, channel) 实例 → 409 CONFLICT。
func TestCreateMarketChannelConflict(t *testing.T) {
	h := newHarness(t)
	assertStatus(t, h.createChannel(t, "100001", "JP", map[string]any{"channelId": "google"}), http.StatusCreated)
	dup := h.createChannel(t, "100001", "JP", map[string]any{"channelId": "google"})
	assertStatus(t, dup, http.StatusConflict)
	if dup.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", dup.errCode())
	}
	// 冲突无副作用：列表仍只有一条。
	list := h.do(t, http.MethodGet, "/api/admin/games/100001/market-channels", h.readToken(t), nil)
	if total := list.data()["total"]; total != float64(1) {
		t.Fatalf("conflict must not create second instance, total=%v", total)
	}
}

// S10：复制来源缺失时在 InTx 内校验失败 → 整体回滚，无 id 占用（下一次成功创建仍取连续序号）。
func TestCreateMarketChannelTransactionRollback(t *testing.T) {
	h := newHarness(t)
	first := h.createChannel(t, "100001", "JP", map[string]any{"channelId": "google"})
	assertStatus(t, first, http.StatusCreated)
	firstID := first.data()["gameChannelId"].(float64)

	// 复制来源 NOT_EXISTS：InTx 内 FindInstance NotFound → VALIDATION_FAILED，回滚。
	rb := h.createChannel(t, "100001", "KR", map[string]any{"channelId": "google", "mode": "copy", "copyFromMarket": "NOT_EXISTS"})
	assertStatus(t, rb, http.StatusBadRequest)
	if rb.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q", rb.errCode())
	}

	// 回滚断言：列表仍只 1 条；且回滚未消耗序号（下一次创建 id == firstID+1）。
	list := h.do(t, http.MethodGet, "/api/admin/games/100001/market-channels", h.readToken(t), nil)
	if total := list.data()["total"]; total != float64(1) {
		t.Fatalf("rolled-back create must not persist, total=%v", total)
	}
	next := h.createChannel(t, "100001", "KR", map[string]any{"channelId": "apple"})
	assertStatus(t, next, http.StatusCreated)
	if got := next.data()["gameChannelId"].(float64); got != firstID+1 {
		t.Fatalf("rolled-back create must not consume an id; want %v got %v", firstID+1, got)
	}
}

func TestCreateMarketChannelGameNotFound(t *testing.T) {
	h := newHarness(t)
	res := h.createChannel(t, "999999", "JP", map[string]any{"channelId": "google"})
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", res.errCode())
	}
}

// ───────────────────────── 候选渠道 / 列表 / 详情（S1/S4/S9） ─────────────────────────

func TestListChannelOptionsSuccess(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/games/100001/channels", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	items, _ := res.data()["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("expected 3 channel options, got %v", items)
	}
	first := items[0].(map[string]any)
	if first["channelId"] != "google" {
		t.Fatalf("options must be sorted by sort asc, got %v", first["channelId"])
	}
}

func TestListChannelOptionsGameNotFound(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/games/999999/channels", h.readToken(t), nil)
	assertStatus(t, res, http.StatusNotFound)
}

func TestGetMarketChannelSuccessAndEnvEcho(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	res := h.do(t, http.MethodGet, "/api/admin/game-channels/"+itoa(id), h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	if d["market"] != "JP" || d["channelId"] != "google" {
		t.Fatalf("detail identity mismatch: %v", d)
	}
	// 运行态：valid + compatible + 未隐藏 → 全 true。
	if d["includedInRuntimeConfig"] != true || d["compatible"] != true {
		t.Fatalf("valid+compatible instance must be runtime-included: %v", d)
	}
	// env 回显（S6 的进程内可见部分；schema 隔离由连库 harness 覆盖）。
	if d["environment"] != string(testEnv) {
		t.Fatalf("environment want %s got %v", testEnv, d["environment"])
	}
	// S8（N/A 附带断言）：详情不泄漏任何内部配置载荷字段。
	if strings.Contains(res.raw, "secretConfig") || strings.Contains(res.raw, "normalConfig") || strings.Contains(res.raw, "fileConfig") {
		t.Fatalf("detail must not expose internal config payloads: %s", res.raw)
	}
}

func TestGetMarketChannelNotFoundAndBadID(t *testing.T) {
	h := newHarness(t)
	nf := h.do(t, http.MethodGet, "/api/admin/game-channels/999999", h.readToken(t), nil)
	assertStatus(t, nf, http.StatusNotFound)
	if nf.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", nf.errCode())
	}
	// S4：非法 int64 路径参数 → 400 VALIDATION_FAILED。
	bad := h.do(t, http.MethodGet, "/api/admin/game-channels/abc", h.readToken(t), nil)
	assertStatus(t, bad, http.StatusBadRequest)
	if bad.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q", bad.errCode())
	}
}

// S9：分页 pageSize 超上限钳制为 100。
func TestListMarketChannelsPaginationClamp(t *testing.T) {
	h := newHarness(t)
	h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	h.store.state.seedInstance("100001", "KR", "google", common.ConfigStatusEmpty)
	res := h.do(t, http.MethodGet, "/api/admin/games/100001/market-channels?page=1&pageSize=999", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	if res.data()["pageSize"] != float64(100) {
		t.Fatalf("pageSize must clamp to 100, got %v", res.data()["pageSize"])
	}
	if res.data()["total"] != float64(2) || res.data()["page"] != float64(1) {
		t.Fatalf("page/total wrong: %v", res.data())
	}
}

// S4：列表非法 query（market/configStatus/compatible/hidden）→ 400 VALIDATION_FAILED。
func TestListMarketChannelsBadQuery(t *testing.T) {
	h := newHarness(t)
	tok := h.readToken(t)
	cases := []string{
		"/api/admin/games/100001/market-channels?market=US",
		"/api/admin/games/100001/market-channels?configStatus=bad",
		"/api/admin/games/100001/market-channels?compatible=abc",
		"/api/admin/games/100001/market-channels?hidden=abc",
	}
	for _, p := range cases {
		res := h.do(t, http.MethodGet, p, tok, nil)
		assertStatus(t, res, http.StatusBadRequest)
		if res.errCode() != "VALIDATION_FAILED" {
			t.Fatalf("%s want VALIDATION_FAILED got %q", p, res.errCode())
		}
	}
}

func TestListMarketChannelsFilterAndHidden(t *testing.T) {
	h := newHarness(t)
	h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	krID := h.store.state.seedInstance("100001", "KR", "apple", common.ConfigStatusValid)
	// 隐藏 KR。
	tok := h.writeToken(t)
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/game-channels/"+itoa(krID)+"/hide", tok, map[string]any{"reason": "x"}), http.StatusOK)

	// 默认不含隐藏 → 仅 1 条。
	def := h.do(t, http.MethodGet, "/api/admin/games/100001/market-channels", h.readToken(t), nil)
	if def.data()["total"] != float64(1) {
		t.Fatalf("default must exclude hidden, total=%v", def.data()["total"])
	}
	// hidden=true → 2 条。
	all := h.do(t, http.MethodGet, "/api/admin/games/100001/market-channels?hidden=true", h.readToken(t), nil)
	if all.data()["total"] != float64(2) {
		t.Fatalf("hidden=true must include hidden, total=%v", all.data()["total"])
	}
	// market 过滤。
	jp := h.do(t, http.MethodGet, "/api/admin/games/100001/market-channels?market=JP", h.readToken(t), nil)
	if jp.data()["total"] != float64(1) {
		t.Fatalf("market filter want 1, got %v", jp.data()["total"])
	}
}

// ───────────────────────── 编辑 / 隐藏 / 恢复（S4/S5/S7） ─────────────────────────

func TestUpdateMarketChannelSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	res := h.do(t, http.MethodPatch, "/api/admin/game-channels/"+itoa(id), h.writeToken(t), map[string]any{"enabled": false, "remark": "off"})
	assertStatus(t, res, http.StatusOK)
	if res.data()["enabled"] != false || res.data()["remark"] != "off" {
		t.Fatalf("update not applied: %v", res.data())
	}
	if _, ok := h.audit.byAction("channel.update"); !ok {
		t.Fatal("expected channel.update audit")
	}
}

func TestUpdateMarketChannelValidationRemarkTooLong(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	res := h.do(t, http.MethodPatch, "/api/admin/game-channels/"+itoa(id), h.writeToken(t), map[string]any{"remark": longString(256)})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q", res.errCode())
	}
}

func TestHideMarketChannelSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	res := h.do(t, http.MethodPost, "/api/admin/game-channels/"+itoa(id)+"/hide", h.writeToken(t), map[string]any{"reason": "manual"})
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	if d["hidden"] != true {
		t.Fatalf("hide must set hidden=true, got %v", d)
	}
	// 隐藏后运行态全 false，原因 hidden。
	if d["includedInRuntimeConfig"] != false || d["runtimeReason"] != "hidden" {
		t.Fatalf("hidden instance must be runtime-excluded with reason hidden: %v", d)
	}
	if _, ok := h.audit.byAction("channel.hide"); !ok {
		t.Fatal("expected channel.hide audit")
	}
}

// S5：仅 valid 可隐藏，对 empty/invalid 隐藏 → 409 CONFLICT。
func TestHideMarketChannelInvalidStateConflict(t *testing.T) {
	h := newHarness(t)
	emptyID := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusEmpty)
	res := h.do(t, http.MethodPost, "/api/admin/game-channels/"+itoa(emptyID)+"/hide", h.writeToken(t), map[string]any{"reason": "x"})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}
}

func TestUnhideMarketChannelSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	tok := h.writeToken(t)
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/game-channels/"+itoa(id)+"/hide", tok, map[string]any{"reason": "x"}), http.StatusOK)
	res := h.do(t, http.MethodPost, "/api/admin/game-channels/"+itoa(id)+"/unhide", tok, nil)
	assertStatus(t, res, http.StatusOK)
	if res.data()["hidden"] != false {
		t.Fatalf("unhide must set hidden=false, got %v", res.data())
	}
	if _, ok := h.audit.byAction("channel.unhide"); !ok {
		t.Fatal("expected channel.unhide audit")
	}
}

// ───────────────────────── 渠道包（S1/S4/S5/S7） ─────────────────────────

func TestCreatePackageSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	res := h.do(t, http.MethodPost, "/api/admin/game-channels/"+itoa(id)+"/packages", h.writeToken(t), map[string]any{
		"packageCode": "pkg.b", "packageName": "Pkg B", "marketCode": "JP", "inheritChannelConfig": true, "enabled": true,
	})
	assertStatus(t, res, http.StatusCreated)
	if res.data()["packageCode"] != "pkg.b" {
		t.Fatalf("package not created: %v", res.data())
	}
	if _, ok := h.audit.byAction("package.create"); !ok {
		t.Fatal("expected package.create audit")
	}
}

// S4：渠道包 market 必须与所属实例一致，否则 400 VALIDATION_FAILED。
func TestCreatePackageMarketMismatch(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	res := h.do(t, http.MethodPost, "/api/admin/game-channels/"+itoa(id)+"/packages", h.writeToken(t), map[string]any{
		"packageCode": "pkg.a", "packageName": "Pkg A", "marketCode": "CN",
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q", res.errCode())
	}
}

// S5：同实例内 packageCode 重复 → 409 CONFLICT。
func TestCreatePackageConflict(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	h.store.state.seedPackage(id, "pkg.a", "Pkg A", "JP")
	res := h.do(t, http.MethodPost, "/api/admin/game-channels/"+itoa(id)+"/packages", h.writeToken(t), map[string]any{
		"packageCode": "pkg.a", "packageName": "Pkg A dup", "marketCode": "JP",
	})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}
}

func TestListPackagesSuccess(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	h.store.state.seedPackage(id, "pkg.a", "Pkg A", "JP")
	h.store.state.seedPackage(id, "pkg.b", "Pkg B", "JP")
	res := h.do(t, http.MethodGet, "/api/admin/game-channels/"+itoa(id)+"/packages", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	items, _ := res.data()["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 packages, got %v", items)
	}
}

func TestUpdatePackageSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	pkgID := h.store.state.seedPackage(id, "pkg.a", "Pkg A", "JP")
	res := h.do(t, http.MethodPatch, "/api/admin/channel-packages/"+itoa(pkgID), h.writeToken(t), map[string]any{
		"enabled": false, "overrideJson": map[string]any{"key": "value"},
	})
	assertStatus(t, res, http.StatusOK)
	if res.data()["enabled"] != false {
		t.Fatalf("update not applied: %v", res.data())
	}
	override, _ := res.data()["overrideJson"].(map[string]any)
	if override["key"] != "value" {
		t.Fatalf("overrideJson not applied: %v", res.data())
	}
	if _, ok := h.audit.byAction("package.update"); !ok {
		t.Fatal("expected package.update audit")
	}
}

func TestUpdatePackageNotFound(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPatch, "/api/admin/channel-packages/999999", h.writeToken(t), map[string]any{"enabled": false})
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", res.errCode())
	}
}

// ───────────────────────── 鉴权与权限（S2/S3） ─────────────────────────

func TestChannelsAuthnRequired(t *testing.T) {
	h := newHarness(t)
	// S2：无令牌 → 401 UNAUTHENTICATED（Authn 在 RequirePerm 之前）。
	endpoints := []struct {
		m, p string
		body any
	}{
		{http.MethodGet, "/api/admin/games/100001/channels", nil},
		{http.MethodGet, "/api/admin/games/100001/market-channels", nil},
		{http.MethodPost, "/api/admin/games/100001/markets/JP/channels", map[string]any{"channelId": "google"}},
		{http.MethodGet, "/api/admin/game-channels/1", nil},
		{http.MethodPatch, "/api/admin/game-channels/1", map[string]any{"enabled": false}},
		{http.MethodPost, "/api/admin/game-channels/1/hide", map[string]any{"reason": "x"}},
		{http.MethodPost, "/api/admin/game-channels/1/unhide", nil},
		{http.MethodGet, "/api/admin/game-channels/1/packages", nil},
		{http.MethodPost, "/api/admin/game-channels/1/packages", map[string]any{"packageCode": "p"}},
		{http.MethodPatch, "/api/admin/channel-packages/1", map[string]any{"enabled": false}},
	}
	for _, ep := range endpoints {
		res := h.do(t, ep.m, ep.p, "", ep.body)
		assertStatus(t, res, http.StatusUnauthorized)
		if res.errCode() != "UNAUTHENTICATED" {
			t.Fatalf("%s %s want UNAUTHENTICATED got %q", ep.m, ep.p, res.errCode())
		}
	}
	// S2：伪造 Bearer → 401。
	bad := h.do(t, http.MethodGet, "/api/admin/games/100001/market-channels", "not.a.valid.jwt", nil)
	assertStatus(t, bad, http.StatusUnauthorized)
}

// S3：登录但权限不足。
func TestChannelsRBACForbidden(t *testing.T) {
	h := newHarness(t)
	id := h.store.state.seedInstance("100001", "JP", "google", common.ConfigStatusValid)
	pkgID := h.store.state.seedPackage(id, "pkg.a", "Pkg A", "JP")

	// 读令牌（仅 channel.read）：可读，但所有写 → 403 FORBIDDEN。
	readOnly := h.readToken(t)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/games/100001/market-channels", readOnly, nil), http.StatusOK)
	writes := []struct {
		m, p string
		body any
	}{
		{http.MethodPost, "/api/admin/games/100001/markets/JP/channels", map[string]any{"channelId": "apple"}},
		{http.MethodPatch, "/api/admin/game-channels/" + itoa(id), map[string]any{"enabled": false}},
		{http.MethodPost, "/api/admin/game-channels/" + itoa(id) + "/hide", map[string]any{"reason": "x"}},
		{http.MethodPost, "/api/admin/game-channels/" + itoa(id) + "/unhide", nil},
		{http.MethodPost, "/api/admin/game-channels/" + itoa(id) + "/packages", map[string]any{"packageCode": "pkg.x", "packageName": "X", "marketCode": "JP"}},
		{http.MethodPatch, "/api/admin/channel-packages/" + itoa(pkgID), map[string]any{"enabled": false}},
	}
	for _, ep := range writes {
		res := h.do(t, ep.m, ep.p, readOnly, ep.body)
		assertStatus(t, res, http.StatusForbidden)
		if res.errCode() != "FORBIDDEN" {
			t.Fatalf("%s %s want FORBIDDEN got %q", ep.m, ep.p, res.errCode())
		}
	}

	// 无 channel.read（其它权限）→ 读列表 403。
	noPerm := h.token(t, 12, []string{"system.read"})
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/games/100001/market-channels", noPerm, nil), http.StatusForbidden)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/games/100001/channels", noPerm, nil), http.StatusForbidden)
}

// ===== helpers =====

func itoa(v int64) string {
	digits := []byte{}
	neg := v < 0
	if v == 0 {
		return "0"
	}
	if neg {
		v = -v
	}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func longString(n int) string {
	return strings.Repeat("x", n)
}
