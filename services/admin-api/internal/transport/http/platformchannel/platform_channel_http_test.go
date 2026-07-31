package platformchannel

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	platformchannelapp "github.com/csw/console/services/admin-api/internal/app/platformchannel"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
)

// 进程内 L3 接口测试（httptest 全链路 transport->app->domain + 内存仓储 + 真实 JWT/路由/中间件）。
// 覆盖维度：无/坏令牌 401、缺权限 403、列表分页与过滤、创建与冲突、业务键与模版四件套校验、
// 编辑不触碰 channelType/region、模版 effective 标记、审计写入。
// 维度边界：platform.* 表是跨 env 的共享主数据，本层不建模 env 隔离；模版四件套里的敏感字段
// 只登记 key、不存密文，故无脱敏维度。
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
	svc := platformchannelapp.NewService(store, audit)

	root := chi.NewRouter()
	sub := chi.NewRouter()
	RegisterRoutes(sub, NewHandler(svc), issuer, testEnv, slog.New(slog.NewTextHandler(io.Discard, nil)), true, nil)
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

// readToken 只有两个读权限；writeToken 读写齐全（渠道 + 模版）。
func (h *harness) readToken(t *testing.T) string {
	return h.token(t, 10, []string{"platform_channel.read", "channel_template.read"})
}

func (h *harness) writeToken(t *testing.T) string {
	return h.token(t, 11, []string{
		"platform_channel.read", "platform_channel.write",
		"channel_template.read", "channel_template.write",
	})
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

// items 取列表包络里的 items 数组。
func (r apiResp) items(t *testing.T) []map[string]any {
	t.Helper()
	raw, ok := r.data()["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got body=%s", r.raw)
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected object item, got %v", item)
		}
		out = append(out, m)
	}
	return out
}

func assertStatus(t *testing.T, got apiResp, want int) {
	t.Helper()
	if got.status != want {
		t.Fatalf("status: want %d got %d (body=%s)", want, got.status, got.raw)
	}
}

func assertField(t *testing.T, m map[string]any, key string, want any) {
	t.Helper()
	if m[key] != want {
		t.Fatalf("%s: want %v got %v", key, want, m[key])
	}
}

// loginTemplateBody 造一份合法的登录模版四件套请求体。
func loginTemplateBody(version string) map[string]any {
	return map[string]any{
		"kind":            "login",
		"templateVersion": version,
		"formSchemaJson": []map[string]any{
			{"key": "appId", "label": "App ID", "component": "input", "required": true, "order": 10, "scope": "both"},
			{"key": "appSecret", "label": "App Secret", "component": "password", "required": true, "order": 20, "scope": "server"},
		},
		"secretFieldsJson":    []string{"appSecret"},
		"fileFieldsJson":      []map[string]any{},
		"validationRulesJson": map[string]any{"appId": map[string]any{"required": true, "minLen": 2}},
	}
}

// ───────────────────────── 认证与授权 ─────────────────────────

func TestPlatformChannelEndpointsRequireToken(t *testing.T) {
	h := newHarness(t)
	endpoints := []struct {
		m, p string
		body any
	}{
		{http.MethodGet, "/api/admin/platform/channels", nil},
		{http.MethodPost, "/api/admin/platform/channels", map[string]any{"channelId": "x"}},
		{http.MethodGet, "/api/admin/platform/channels/google", nil},
		{http.MethodPatch, "/api/admin/platform/channels/google", map[string]any{"channelName": "X"}},
		{http.MethodGet, "/api/admin/platform/channels/google/templates", nil},
		{http.MethodPost, "/api/admin/platform/channels/google/templates", loginTemplateBody("v1")},
		{http.MethodGet, "/api/admin/platform/channel-templates/login/1", nil},
		{http.MethodPatch, "/api/admin/platform/channel-templates/login/1", map[string]any{"enabled": false}},
	}
	for _, ep := range endpoints {
		res := h.do(t, ep.m, ep.p, "", ep.body)
		assertStatus(t, res, http.StatusUnauthorized)
		if res.errCode() != "UNAUTHENTICATED" {
			t.Fatalf("%s %s want UNAUTHENTICATED got %q", ep.m, ep.p, res.errCode())
		}
	}
	// 伪造 Bearer → 401。
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channels", "not.a.valid.jwt", nil), http.StatusUnauthorized)
}

func TestPlatformChannelRBACForbidden(t *testing.T) {
	h := newHarness(t)

	// 读令牌可读，但所有写 → 403。
	readOnly := h.readToken(t)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channels", readOnly, nil), http.StatusOK)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channels/google/templates", readOnly, nil), http.StatusOK)

	writes := []struct {
		m, p string
		body any
	}{
		{http.MethodPost, "/api/admin/platform/channels", map[string]any{
			"channelId": "vivo_cn", "channelName": "vivo", "channelType": "domestic", "region": "CN",
			"loginMode": "channel_only", "paymentMode": "channel_only",
		}},
		{http.MethodPatch, "/api/admin/platform/channels/google", map[string]any{"channelName": "Google Play 商店"}},
		{http.MethodPost, "/api/admin/platform/channels/google/templates", loginTemplateBody("v1")},
		{http.MethodPatch, "/api/admin/platform/channel-templates/login/1", map[string]any{"enabled": false}},
	}
	for _, ep := range writes {
		res := h.do(t, ep.m, ep.p, readOnly, ep.body)
		assertStatus(t, res, http.StatusForbidden)
		if res.errCode() != "FORBIDDEN" {
			t.Fatalf("%s %s want FORBIDDEN got %q", ep.m, ep.p, res.errCode())
		}
	}

	// 渠道读写权限不能越界到模版读写：channel_template.* 是独立权限码。
	channelOnly := h.token(t, 12, []string{"platform_channel.read", "platform_channel.write"})
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channels/google/templates", channelOnly, nil), http.StatusForbidden)
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/platform/channels/google/templates", channelOnly, loginTemplateBody("v1")), http.StatusForbidden)

	// 游戏侧的 channel.* 也不能读平台渠道主数据。
	gameSide := h.token(t, 13, []string{"channel.read", "channel.write"})
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channels", gameSide, nil), http.StatusForbidden)
}

// ───────────────────────── 渠道列表：分页与过滤 ─────────────────────────

func TestListPlatformChannels(t *testing.T) {
	h := newHarness(t)
	token := h.readToken(t)

	res := h.do(t, http.MethodGet, "/api/admin/platform/channels", token, nil)
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	assertField(t, d, "page", float64(1))
	assertField(t, d, "pageSize", float64(20))
	assertField(t, d, "total", float64(2))

	items := res.items(t)
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d (%s)", len(items), res.raw)
	}
	// 按 sort 升序：google(1) 在 huawei_cn(2) 之前。
	assertField(t, items[0], "channelId", "google")
	assertField(t, items[1], "channelId", "huawei_cn")

	// 视图字段齐全，含策略与模版计数（huawei_cn 有 2 个登录模版、0 个 IAP 模版）。
	hw := items[1]
	assertField(t, hw, "channelName", "华为")
	assertField(t, hw, "channelType", "domestic")
	assertField(t, hw, "region", "CN")
	assertField(t, hw, "enabled", true)
	assertField(t, hw, "loginMode", "channel_only")
	assertField(t, hw, "paymentMode", "channel_only")
	assertField(t, hw, "loginLocked", false)
	assertField(t, hw, "paymentLocked", false)
	assertField(t, hw, "loginTemplateCount", float64(2))
	assertField(t, hw, "iapTemplateCount", float64(0))

	// 分页：pageSize=1 时 total 仍为全量。
	paged := h.do(t, http.MethodGet, "/api/admin/platform/channels?page=2&pageSize=1", token, nil)
	assertStatus(t, paged, http.StatusOK)
	assertField(t, paged.data(), "total", float64(2))
	pagedItems := paged.items(t)
	if len(pagedItems) != 1 || pagedItems[0]["channelId"] != "huawei_cn" {
		t.Fatalf("page 2 want huawei_cn only, got %s", paged.raw)
	}
}

func TestListPlatformChannelsFilters(t *testing.T) {
	h := newHarness(t)
	token := h.readToken(t)

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"region 过滤", "?region=GLOBAL", []string{"google"}},
		{"channelType 过滤", "?channelType=domestic", []string{"huawei_cn"}},
		{"keyword 命中 channelId", "?keyword=huawei", []string{"huawei_cn"}},
		{"keyword 命中渠道名", "?keyword=Google", []string{"google"}},
		{"enabled=true 全命中", "?enabled=true", []string{"google", "huawei_cn"}},
		{"enabled=false 无命中", "?enabled=false", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := h.do(t, http.MethodGet, "/api/admin/platform/channels"+tc.query, token, nil)
			assertStatus(t, res, http.StatusOK)
			items := res.items(t)
			if len(items) != len(tc.want) {
				t.Fatalf("want %d items, got %d (%s)", len(tc.want), len(items), res.raw)
			}
			for i, id := range tc.want {
				assertField(t, items[i], "channelId", id)
			}
		})
	}

	// 非法枚举 → 400 VALIDATION_FAILED。
	for _, q := range []string{"?region=mars", "?channelType=unknown", "?enabled=maybe"} {
		res := h.do(t, http.MethodGet, "/api/admin/platform/channels"+q, token, nil)
		assertStatus(t, res, http.StatusBadRequest)
		if res.errCode() != "VALIDATION_FAILED" {
			t.Fatalf("%s want VALIDATION_FAILED got %q", q, res.errCode())
		}
	}
}

func TestGetPlatformChannelNotFound(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/platform/channels/nope", h.readToken(t), nil)
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", res.errCode())
	}
}

// ───────────────────────── 创建渠道 ─────────────────────────

func TestCreatePlatformChannelSuccess(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/admin/platform/channels", h.writeToken(t), map[string]any{
		"channelId": "vivo_cn", "channelName": "vivo", "channelType": "domestic", "region": "CN",
		"sort": 5, "loginMode": "account_system", "paymentMode": "hybrid", "paymentLocked": true,
	})
	assertStatus(t, res, http.StatusCreated)
	d := res.data()
	assertField(t, d, "channelId", "vivo_cn")
	assertField(t, d, "channelName", "vivo")
	assertField(t, d, "channelType", "domestic")
	assertField(t, d, "region", "CN")
	assertField(t, d, "sort", float64(5))
	assertField(t, d, "enabled", true) // 缺省启用
	assertField(t, d, "loginMode", "account_system")
	assertField(t, d, "paymentMode", "hybrid")
	assertField(t, d, "loginLocked", false)
	assertField(t, d, "paymentLocked", true)
	assertField(t, d, "loginTemplateCount", float64(0))
	assertField(t, d, "iapTemplateCount", float64(0))

	// 落库可回读。
	got := h.do(t, http.MethodGet, "/api/admin/platform/channels/vivo_cn", h.readToken(t), nil)
	assertStatus(t, got, http.StatusOK)
	assertField(t, got.data(), "paymentMode", "hybrid")

	// 审计：platform_channel.create 写入，带 actor 与关键字段。
	entry, ok := h.audit.byAction("platform_channel.create")
	if !ok {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ActorID != 11 {
		t.Fatalf("audit actor: want 11 got %d", entry.ActorID)
	}
	if entry.ResourceType != "platform_channel" || entry.ResourceID != "vivo_cn" {
		t.Fatalf("audit resource: got %s/%s", entry.ResourceType, entry.ResourceID)
	}
	if entry.Detail["channelType"] != "domestic" || entry.Detail["region"] != "CN" {
		t.Fatalf("audit detail: got %+v", entry.Detail)
	}
}

func TestCreatePlatformChannelDuplicate(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/admin/platform/channels", h.writeToken(t), map[string]any{
		"channelId": "google", "channelName": "Google Play 2", "channelType": "store", "region": "GLOBAL",
		"loginMode": "channel_only", "paymentMode": "channel_only",
	})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}
	// 冲突不得改动既有行。
	got := h.do(t, http.MethodGet, "/api/admin/platform/channels/google", h.readToken(t), nil)
	assertField(t, got.data(), "channelName", "Google Play")
}

func TestCreatePlatformChannelValidation(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)
	base := func(over map[string]any) map[string]any {
		body := map[string]any{
			"channelId": "ok_id", "channelName": "名字", "channelType": "store", "region": "GLOBAL",
			"loginMode": "channel_only", "paymentMode": "channel_only",
		}
		for k, v := range over {
			body[k] = v
		}
		return body
	}
	cases := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"channelId 含大写", base(map[string]any{"channelId": "Google"}), "channelId"},
		{"channelId 含横线", base(map[string]any{"channelId": "huawei-cn"}), "channelId"},
		{"channelId 数字下划线开头以外的符号", base(map[string]any{"channelId": "_x"}), "channelId"},
		{"channelId 为空", base(map[string]any{"channelId": ""}), "channelId"},
		{"channelName 为空", base(map[string]any{"channelName": "  "}), "channelName"},
		{"channelType 非枚举", base(map[string]any{"channelType": "console"}), "channelType"},
		{"region 非枚举", base(map[string]any{"region": "global"}), "region"},
		{"loginMode 非枚举", base(map[string]any{"loginMode": "guest"}), "loginMode"},
		{"paymentMode 非枚举", base(map[string]any{"paymentMode": "free"}), "paymentMode"},
		{"sort 越界", base(map[string]any{"sort": 10000}), "sort"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := h.do(t, http.MethodPost, "/api/admin/platform/channels", token, tc.body)
			assertStatus(t, res, http.StatusBadRequest)
			if res.errCode() != "VALIDATION_FAILED" {
				t.Fatalf("want VALIDATION_FAILED got %q", res.errCode())
			}
			assertDetailField(t, res, tc.field)
		})
	}
	// 校验失败不落库。
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channels/ok_id", h.readToken(t), nil), http.StatusNotFound)
}

// ───────────────────────── 编辑渠道 ─────────────────────────

func TestUpdatePlatformChannel(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPatch, "/api/admin/platform/channels/google", h.writeToken(t), map[string]any{
		"channelName": "Google Play 商店", "sort": 9, "enabled": false,
		"loginMode": "account_system", "paymentMode": "cashier_only", "loginLocked": true,
	})
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	assertField(t, d, "channelName", "Google Play 商店")
	assertField(t, d, "sort", float64(9))
	assertField(t, d, "enabled", false)
	assertField(t, d, "loginMode", "account_system")
	assertField(t, d, "paymentMode", "cashier_only")
	assertField(t, d, "loginLocked", true)
	assertField(t, d, "paymentLocked", false)
	// 身份与 market 兼容性口径不受编辑影响。
	assertField(t, d, "channelId", "google")
	assertField(t, d, "channelType", "store")
	assertField(t, d, "region", "GLOBAL")

	// 审计记录改动字段集合。
	entry, ok := h.audit.byAction("platform_channel.update")
	if !ok {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ResourceID != "google" {
		t.Fatalf("audit resource id: got %s", entry.ResourceID)
	}
	fields, _ := entry.Detail["fields"].([]string)
	for _, want := range []string{"channelName", "enabled", "sort", "loginMode", "paymentMode", "loginLocked"} {
		if !containsString(fields, want) {
			t.Fatalf("audit fields %v missing %s", fields, want)
		}
	}
	if containsString(fields, "paymentLocked") {
		t.Fatalf("audit fields %v should not include untouched paymentLocked", fields)
	}
}

// PATCH 里 channelType / region / channelId 是未知字段，被忽略而不是生效。
func TestUpdatePlatformChannelIgnoresImmutableFields(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPatch, "/api/admin/platform/channels/huawei_cn", h.writeToken(t), map[string]any{
		"channelName": "华为应用市场",
		"channelId":   "huawei_global",
		"channelType": "store",
		"region":      "GLOBAL",
	})
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	assertField(t, d, "channelName", "华为应用市场")
	assertField(t, d, "channelId", "huawei_cn")
	assertField(t, d, "channelType", "domestic")
	assertField(t, d, "region", "CN")

	// 原业务键仍可读，未产生新键。
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channels/huawei_cn", h.readToken(t), nil), http.StatusOK)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channels/huawei_global", h.readToken(t), nil), http.StatusNotFound)
}

func TestUpdatePlatformChannelValidationAndNotFound(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	bad := h.do(t, http.MethodPatch, "/api/admin/platform/channels/google", token, map[string]any{"paymentMode": "free"})
	assertStatus(t, bad, http.StatusBadRequest)
	assertDetailField(t, bad, "paymentMode")

	missing := h.do(t, http.MethodPatch, "/api/admin/platform/channels/nope", token, map[string]any{"channelName": "X"})
	assertStatus(t, missing, http.StatusNotFound)

	// 空 patch 为幂等无操作：不报错、不写审计。
	noop := h.do(t, http.MethodPatch, "/api/admin/platform/channels/google", token, map[string]any{})
	assertStatus(t, noop, http.StatusOK)
	assertField(t, noop.data(), "channelName", "Google Play")
	if _, ok := h.audit.byAction("platform_channel.update"); ok {
		t.Fatalf("no-op patch should not write audit, got %+v", h.audit.entries)
	}
}

// ───────────────────────── 模版列表与 effective 标记 ─────────────────────────

func TestListChannelTemplatesEffective(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/platform/channels/huawei_cn/templates?kind=login", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	items := res.items(t)
	if len(items) != 2 {
		t.Fatalf("want 2 templates, got %d (%s)", len(items), res.raw)
	}

	// 按 template_version 降序：v2（disabled）在前，但生效版本是 enabled 的最新版本 v1。
	assertField(t, items[0], "templateVersion", "v2")
	assertField(t, items[0], "enabled", false)
	assertField(t, items[0], "effective", false)
	assertField(t, items[1], "templateVersion", "v1")
	assertField(t, items[1], "enabled", true)
	assertField(t, items[1], "effective", true)

	// 视图带渠道业务键与四件套。
	assertField(t, items[1], "channelId", "huawei_cn")
	assertField(t, items[1], "kind", "login")
	if _, ok := items[1]["formSchemaJson"].([]any); !ok {
		t.Fatalf("formSchemaJson should be an array, got %v", items[1]["formSchemaJson"])
	}

	// kind 省略 → 登录 + IAP 都返（IAP 无版本，故仍是 2 条）。
	all := h.do(t, http.MethodGet, "/api/admin/platform/channels/huawei_cn/templates", h.readToken(t), nil)
	assertStatus(t, all, http.StatusOK)
	if len(all.items(t)) != 2 {
		t.Fatalf("want 2 templates for both kinds, got %s", all.raw)
	}

	// 无模版的渠道 → 空数组而非 null。
	empty := h.do(t, http.MethodGet, "/api/admin/platform/channels/google/templates", h.readToken(t), nil)
	assertStatus(t, empty, http.StatusOK)
	if len(empty.items(t)) != 0 {
		t.Fatalf("want empty items, got %s", empty.raw)
	}

	// kind 非法 → 400；渠道不存在 → 404。
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channels/google/templates?kind=feature", h.readToken(t), nil), http.StatusBadRequest)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channels/nope/templates", h.readToken(t), nil), http.StatusNotFound)
}

// 禁用生效版本后，effective 顺延到下一个 enabled 的最新版本。
func TestTemplateEffectiveFollowsEnabledFlag(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	// 启用 v2 → v2 成为生效版本（版本号更大）。
	res := h.do(t, http.MethodPatch, "/api/admin/platform/channel-templates/login/2", token, map[string]any{"enabled": true})
	assertStatus(t, res, http.StatusOK)
	assertField(t, res.data(), "templateVersion", "v2")
	assertField(t, res.data(), "effective", true)

	list := h.do(t, http.MethodGet, "/api/admin/platform/channels/huawei_cn/templates?kind=login", token, nil)
	items := list.items(t)
	assertField(t, items[0], "templateVersion", "v2")
	assertField(t, items[0], "effective", true)
	assertField(t, items[1], "templateVersion", "v1")
	assertField(t, items[1], "effective", false)

	// 审计写入 channel_template.update，记录被改的件。
	entry, ok := h.audit.byAction("channel_template.update")
	if !ok {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ResourceType != "channel_template" || entry.ResourceID != "2" {
		t.Fatalf("audit resource: got %s/%s", entry.ResourceType, entry.ResourceID)
	}
	if entry.Detail["kind"] != "login" || entry.Detail["templateVersion"] != "v2" {
		t.Fatalf("audit detail: got %+v", entry.Detail)
	}
	fields, _ := entry.Detail["fields"].([]string)
	if !containsString(fields, "enabled") {
		t.Fatalf("audit fields %v missing enabled", fields)
	}
}

func TestGetChannelTemplate(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/platform/channel-templates/login/1", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	assertField(t, res.data(), "templateId", float64(1))
	assertField(t, res.data(), "templateVersion", "v1")
	assertField(t, res.data(), "effective", true)

	// kind 与 id 的组合必须匹配：登录模版 1 不在 IAP 表里。
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channel-templates/iap/1", h.readToken(t), nil), http.StatusNotFound)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channel-templates/login/999", h.readToken(t), nil), http.StatusNotFound)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channel-templates/feature/1", h.readToken(t), nil), http.StatusBadRequest)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/platform/channel-templates/login/abc", h.readToken(t), nil), http.StatusBadRequest)
}

// ───────────────────────── 创建模版 ─────────────────────────

func TestCreateChannelTemplateSuccess(t *testing.T) {
	h := newHarness(t)
	body := loginTemplateBody("v3")
	body["formSchemaJson"] = []map[string]any{
		{"key": "appId", "label": "App ID", "component": "input", "required": true, "order": 10, "scope": "both", "placeholder": "请输入 App ID"},
		{"key": "appSecret", "label": "App Secret", "component": "password", "required": true, "order": 20, "scope": "server"},
		{"key": "keystore", "label": "签名文件", "component": "file", "required": false, "order": 30, "scope": "client"},
		{"key": "loginType", "label": "登录方式", "component": "select", "required": true, "order": 40, "group": "高级",
			"options": []map[string]any{{"label": "静默", "value": "silent"}, {"label": "弹窗", "value": "dialog"}}},
	}
	body["fileFieldsJson"] = []map[string]any{{"key": "keystore", "accept": []string{".jks"}, "maxSizeKB": 2048}}

	res := h.do(t, http.MethodPost, "/api/admin/platform/channels/huawei_cn/templates", h.writeToken(t), body)
	assertStatus(t, res, http.StatusCreated)
	d := res.data()
	assertField(t, d, "kind", "login")
	assertField(t, d, "channelId", "huawei_cn")
	assertField(t, d, "templateVersion", "v3")
	assertField(t, d, "enabled", true)
	// v3 是 enabled 的最新版本 → 生效。
	assertField(t, d, "effective", true)

	// 四件套原样回传，含 placeholder / options / 文件约束。
	form, _ := d["formSchemaJson"].([]any)
	if len(form) != 4 {
		t.Fatalf("want 4 form fields, got %v", d["formSchemaJson"])
	}
	first, _ := form[0].(map[string]any)
	assertField(t, first, "placeholder", "请输入 App ID")
	assertField(t, first, "scope", "both")
	last, _ := form[3].(map[string]any)
	opts, _ := last["options"].([]any)
	if len(opts) != 2 {
		t.Fatalf("want 2 options, got %v", last["options"])
	}
	files, _ := d["fileFieldsJson"].([]any)
	if len(files) != 1 {
		t.Fatalf("want 1 file field, got %v", d["fileFieldsJson"])
	}
	fileEntry, _ := files[0].(map[string]any)
	assertField(t, fileEntry, "key", "keystore")
	assertField(t, fileEntry, "maxSizeKB", float64(2048))
	secrets, _ := d["secretFieldsJson"].([]any)
	if len(secrets) != 1 || secrets[0] != "appSecret" {
		t.Fatalf("want secretFieldsJson [appSecret], got %v", d["secretFieldsJson"])
	}

	// 渠道视图上的登录模版计数随之 +1（原 2 个）。
	ch := h.do(t, http.MethodGet, "/api/admin/platform/channels/huawei_cn", h.readToken(t), nil)
	assertField(t, ch.data(), "loginTemplateCount", float64(3))

	// 审计 channel_template.create。
	entry, ok := h.audit.byAction("channel_template.create")
	if !ok {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ResourceType != "channel_template" {
		t.Fatalf("audit resource type: got %s", entry.ResourceType)
	}
	if entry.Detail["kind"] != "login" || entry.Detail["channelId"] != "huawei_cn" || entry.Detail["templateVersion"] != "v3" {
		t.Fatalf("audit detail: got %+v", entry.Detail)
	}
}

// IAP 模版与登录模版分表：同渠道同版本号互不冲突。
func TestCreateIAPTemplateIsolatedFromLogin(t *testing.T) {
	h := newHarness(t)
	body := loginTemplateBody("v1")
	body["kind"] = "iap"

	res := h.do(t, http.MethodPost, "/api/admin/platform/channels/huawei_cn/templates", h.writeToken(t), body)
	assertStatus(t, res, http.StatusCreated)
	assertField(t, res.data(), "kind", "iap")
	assertField(t, res.data(), "templateVersion", "v1")

	ch := h.do(t, http.MethodGet, "/api/admin/platform/channels/huawei_cn", h.readToken(t), nil)
	assertField(t, ch.data(), "loginTemplateCount", float64(2))
	assertField(t, ch.data(), "iapTemplateCount", float64(1))
}

func TestCreateChannelTemplateDuplicateVersion(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/admin/platform/channels/huawei_cn/templates", h.writeToken(t), loginTemplateBody("v1"))
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}
	// 冲突回滚：不应多出一个版本。
	ch := h.do(t, http.MethodGet, "/api/admin/platform/channels/huawei_cn", h.readToken(t), nil)
	assertField(t, ch.data(), "loginTemplateCount", float64(2))
}

func TestCreateChannelTemplateValidation(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	withForm := func(fields []map[string]any, over map[string]any) map[string]any {
		body := loginTemplateBody("v9")
		body["formSchemaJson"] = fields
		body["secretFieldsJson"] = []string{}
		for k, v := range over {
			body[k] = v
		}
		return body
	}
	input := func(key string) map[string]any {
		return map[string]any{"key": key, "label": key, "component": "input", "order": 10}
	}

	cases := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{
			name:  "password 字段未登记敏感字段",
			body:  withForm([]map[string]any{{"key": "appSecret", "label": "密钥", "component": "password", "order": 10}}, nil),
			field: "secretFieldsJson",
		},
		{
			name:  "file 字段未登记文件字段",
			body:  withForm([]map[string]any{{"key": "keystore", "label": "签名", "component": "file", "order": 10}}, nil),
			field: "fileFieldsJson",
		},
		{
			name:  "select 字段无候选项",
			body:  withForm([]map[string]any{{"key": "mode", "label": "模式", "component": "select", "order": 10}}, nil),
			field: "formSchemaJson[0].options",
		},
		{
			name:  "formSchema 为空",
			body:  withForm([]map[string]any{}, nil),
			field: "formSchemaJson",
		},
		{
			name:  "字段 key 重复",
			body:  withForm([]map[string]any{input("appId"), input("appId")}, nil),
			field: "formSchemaJson[1].key",
		},
		{
			name:  "字段 key 非法",
			body:  withForm([]map[string]any{input("1appId")}, nil),
			field: "formSchemaJson[0].key",
		},
		{
			name:  "字段 label 为空",
			body:  withForm([]map[string]any{{"key": "appId", "label": " ", "component": "input", "order": 10}}, nil),
			field: "formSchemaJson[0].label",
		},
		{
			name:  "component 非枚举",
			body:  withForm([]map[string]any{{"key": "appId", "label": "App", "component": "richtext", "order": 10}}, nil),
			field: "formSchemaJson[0].component",
		},
		{
			name:  "scope 非枚举",
			body:  withForm([]map[string]any{{"key": "appId", "label": "App", "component": "input", "scope": "cluster", "order": 10}}, nil),
			field: "formSchemaJson[0].scope",
		},
		{
			name:  "敏感字段未在表单声明",
			body:  withForm([]map[string]any{input("appId")}, map[string]any{"secretFieldsJson": []string{"ghost"}}),
			field: "secretFieldsJson",
		},
		{
			name: "文件字段未在表单声明",
			body: withForm([]map[string]any{input("appId")},
				map[string]any{"fileFieldsJson": []map[string]any{{"key": "ghost"}}}),
			field: "fileFieldsJson",
		},
		{
			name: "校验规则字段未在表单声明",
			body: withForm([]map[string]any{input("appId")},
				map[string]any{"validationRulesJson": map[string]any{"ghost": map[string]any{"required": true}}}),
			field: "validationRulesJson",
		},
		{
			name: "pattern 无法编译",
			body: withForm([]map[string]any{input("appId")},
				map[string]any{"validationRulesJson": map[string]any{"appId": map[string]any{"pattern": "([a-z"}}}),
			field: "validationRulesJson.appId.pattern",
		},
		{
			name:  "templateVersion 非法",
			body:  withForm([]map[string]any{input("appId")}, map[string]any{"templateVersion": "v 1"}),
			field: "templateVersion",
		},
		{
			name:  "templateVersion 为空",
			body:  withForm([]map[string]any{input("appId")}, map[string]any{"templateVersion": ""}),
			field: "templateVersion",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := h.do(t, http.MethodPost, "/api/admin/platform/channels/google/templates", token, tc.body)
			assertStatus(t, res, http.StatusBadRequest)
			if res.errCode() != "VALIDATION_FAILED" {
				t.Fatalf("want VALIDATION_FAILED got %q", res.errCode())
			}
			assertDetailField(t, res, tc.field)
		})
	}

	// kind 非法 → 400；渠道不存在 → 404。
	badKind := loginTemplateBody("v9")
	badKind["kind"] = "feature_plugin"
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/platform/channels/google/templates", token, badKind), http.StatusBadRequest)
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/platform/channels/nope/templates", token, loginTemplateBody("v9")), http.StatusNotFound)

	// 全部校验失败后 google 仍无模版。
	list := h.do(t, http.MethodGet, "/api/admin/platform/channels/google/templates", token, nil)
	if len(list.items(t)) != 0 {
		t.Fatalf("google should have no templates, got %s", list.raw)
	}
}

// ───────────────────────── 编辑模版 ─────────────────────────

func TestUpdateChannelTemplateReplacesQuartet(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	res := h.do(t, http.MethodPatch, "/api/admin/platform/channel-templates/login/1", token, map[string]any{
		"formSchemaJson": []map[string]any{
			{"key": "clientId", "label": "Client ID", "component": "input", "required": true, "order": 10},
			{"key": "clientSecret", "label": "Client Secret", "component": "password", "required": true, "order": 20},
		},
		"secretFieldsJson":    []string{"clientSecret"},
		"validationRulesJson": map[string]any{"clientId": map[string]any{"required": true, "maxLen": 32}},
	})
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	// 版本与所属渠道不可改。
	assertField(t, d, "templateVersion", "v1")
	assertField(t, d, "channelId", "huawei_cn")
	form, _ := d["formSchemaJson"].([]any)
	if len(form) != 2 {
		t.Fatalf("want 2 fields after replace, got %v", d["formSchemaJson"])
	}
	first, _ := form[0].(map[string]any)
	assertField(t, first, "key", "clientId")
	rules, _ := d["validationRulesJson"].(map[string]any)
	if _, ok := rules["clientId"]; !ok {
		t.Fatalf("want clientId rule, got %v", d["validationRulesJson"])
	}
	// 未传的 fileFieldsJson 保持原值（空）。
	if files, _ := d["fileFieldsJson"].([]any); len(files) != 0 {
		t.Fatalf("fileFieldsJson should stay empty, got %v", d["fileFieldsJson"])
	}
}

func TestUpdateChannelTemplateValidationKeepsOldState(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	// 换成 password 字段但不登记敏感字段 → 400，且原模版不受影响。
	bad := h.do(t, http.MethodPatch, "/api/admin/platform/channel-templates/login/1", token, map[string]any{
		"formSchemaJson": []map[string]any{
			{"key": "secretKey", "label": "密钥", "component": "password", "required": true, "order": 10},
		},
	})
	assertStatus(t, bad, http.StatusBadRequest)
	assertDetailField(t, bad, "secretFieldsJson")

	got := h.do(t, http.MethodGet, "/api/admin/platform/channel-templates/login/1", token, nil)
	assertStatus(t, got, http.StatusOK)
	form, _ := got.data()["formSchemaJson"].([]any)
	if len(form) != 1 {
		t.Fatalf("want original single field, got %v", got.data()["formSchemaJson"])
	}
	if first, _ := form[0].(map[string]any); first["key"] != "appId" {
		t.Fatalf("original field should stay appId, got %v", form[0])
	}
	if _, ok := h.audit.byAction("channel_template.update"); ok {
		t.Fatalf("failed update should not write audit, got %+v", h.audit.entries)
	}

	// 不存在的模版 → 404。
	assertStatus(t, h.do(t, http.MethodPatch, "/api/admin/platform/channel-templates/login/999", token, map[string]any{"enabled": false}), http.StatusNotFound)
}

// ───────────────────────── helpers ─────────────────────────

// assertDetailField 断言 error.details 里出现了指定 field。
func assertDetailField(t *testing.T, res apiResp, field string) {
	t.Helper()
	e, ok := res.body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %s", res.raw)
	}
	details, ok := e["details"].([]any)
	if !ok {
		t.Fatalf("expected error.details array, got %s", res.raw)
	}
	for _, item := range details {
		if m, ok := item.(map[string]any); ok && m["field"] == field {
			return
		}
	}
	t.Fatalf("details missing field %q: %s", field, res.raw)
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
