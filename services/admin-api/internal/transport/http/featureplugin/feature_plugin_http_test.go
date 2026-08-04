package featureplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	featurepluginapp "github.com/csw/console/services/admin-api/internal/app/featureplugin"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
)

// 进程内 L3 接口测试（httptest 全链路 transport->app->domain + 内存仓储 + 真实 JWT/路由/中间件）。
// 覆盖维度：无/坏令牌 401、缺权限 403（含与游戏侧 plugin.* 的隔离）、分类字典增删改与被引用拒删、
// 插件主数据分页过滤/创建冲突/不可改字段/引用检查、模板四件套整体替换与 effective 标记、审计写入。
// 维度边界：platform.* 表是跨 env 的共享主数据，本层不建模 env 隔离；模板四件套里的敏感字段
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
	svc := featurepluginapp.NewService(store, audit)

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

// readToken 只有读权限；writeToken 读写齐全。
func (h *harness) readToken(t *testing.T) string {
	return h.token(t, 10, []string{"feature_plugin.read"})
}

func (h *harness) writeToken(t *testing.T) string {
	return h.token(t, 11, []string{"feature_plugin.read", "feature_plugin.write"})
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

// doRaw 发送原始 JSON 请求体，用于表达「显式 null」等 map 编码无法稳定表达的语义。
func (h *harness) doRaw(t *testing.T, method, path, token, body string) apiResp {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
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

func (r apiResp) errMessage() string {
	if e, ok := r.body["error"].(map[string]any); ok {
		if m, ok := e["message"].(string); ok {
			return m
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

// templateBody 造一份合法的插件参数模板四件套请求体。
func templateBody(version string) map[string]any {
	return map[string]any{
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

func TestFeaturePluginEndpointsRequireToken(t *testing.T) {
	h := newHarness(t)
	endpoints := []struct {
		m, p string
		body any
	}{
		{http.MethodGet, "/api/admin/feature-plugin-categories", nil},
		{http.MethodPost, "/api/admin/feature-plugin-categories", map[string]any{"categoryCode": "x"}},
		{http.MethodPatch, "/api/admin/feature-plugin-categories/1", map[string]any{"categoryName": "X"}},
		{http.MethodDelete, "/api/admin/feature-plugin-categories/1", nil},
		{http.MethodGet, "/api/admin/feature-plugins", nil},
		{http.MethodPost, "/api/admin/feature-plugins", map[string]any{"pluginId": "x"}},
		{http.MethodGet, "/api/admin/feature-plugins/realname", nil},
		{http.MethodPatch, "/api/admin/feature-plugins/realname", map[string]any{"pluginName": "X"}},
		{http.MethodDelete, "/api/admin/feature-plugins/realname", nil},
		{http.MethodGet, "/api/admin/feature-plugins/realname/templates", nil},
		{http.MethodPost, "/api/admin/feature-plugins/realname/templates", templateBody("v9")},
		{http.MethodGet, "/api/admin/feature-plugin-templates/1", nil},
		{http.MethodPatch, "/api/admin/feature-plugin-templates/1", map[string]any{"enabled": false}},
	}
	for _, ep := range endpoints {
		res := h.do(t, ep.m, ep.p, "", ep.body)
		assertStatus(t, res, http.StatusUnauthorized)
		if res.errCode() != "UNAUTHENTICATED" {
			t.Fatalf("%s %s want UNAUTHENTICATED got %q", ep.m, ep.p, res.errCode())
		}
	}
	// 伪造 Bearer → 401。
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins", "not.a.valid.jwt", nil), http.StatusUnauthorized)
}

func TestFeaturePluginRBACForbidden(t *testing.T) {
	h := newHarness(t)

	// 读令牌可读，但所有写 → 403。
	readOnly := h.readToken(t)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugin-categories", readOnly, nil), http.StatusOK)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins", readOnly, nil), http.StatusOK)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins/realname/templates", readOnly, nil), http.StatusOK)

	writes := []struct {
		m, p string
		body any
	}{
		{http.MethodPost, "/api/admin/feature-plugin-categories", map[string]any{"categoryCode": "push", "categoryName": "推送类"}},
		{http.MethodPatch, "/api/admin/feature-plugin-categories/1", map[string]any{"categoryName": "登录"}},
		{http.MethodDelete, "/api/admin/feature-plugin-categories/3", nil},
		{http.MethodPost, "/api/admin/feature-plugins", map[string]any{
			"pluginId": "push_sdk", "pluginName": "推送", "region": "domestic",
		}},
		{http.MethodPatch, "/api/admin/feature-plugins/realname", map[string]any{"pluginName": "实名"}},
		{http.MethodDelete, "/api/admin/feature-plugins/customer_service", nil},
		{http.MethodPost, "/api/admin/feature-plugins/realname/templates", templateBody("v9")},
		{http.MethodPatch, "/api/admin/feature-plugin-templates/1", map[string]any{"enabled": false}},
	}
	for _, ep := range writes {
		res := h.do(t, ep.m, ep.p, readOnly, ep.body)
		assertStatus(t, res, http.StatusForbidden)
		if res.errCode() != "FORBIDDEN" {
			t.Fatalf("%s %s want FORBIDDEN got %q", ep.m, ep.p, res.errCode())
		}
	}

	// 游戏侧的 plugin.*（渠道实例插件配置）不能碰系统级插件主数据：这是两套独立权限码。
	gameSide := h.token(t, 12, []string{"plugin.read", "plugin.write"})
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins", gameSide, nil), http.StatusForbidden)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugin-categories", gameSide, nil), http.StatusForbidden)
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/feature-plugins", gameSide, map[string]any{
		"pluginId": "push_sdk", "pluginName": "推送", "region": "domestic",
	}), http.StatusForbidden)
	// 删除同样必须用 feature_plugin.write，游戏侧 plugin.write 不能越权删平台主数据。
	assertStatus(t, h.do(t, http.MethodDelete, "/api/admin/feature-plugins/customer_service", gameSide, nil), http.StatusForbidden)
}

// ───────────────────────── 分类字典 ─────────────────────────

func TestListFeaturePluginCategories(t *testing.T) {
	h := newHarness(t)
	token := h.readToken(t)

	res := h.do(t, http.MethodGet, "/api/admin/feature-plugin-categories", token, nil)
	assertStatus(t, res, http.StatusOK)
	items := res.items(t)
	if len(items) != 3 {
		t.Fatalf("want 3 categories, got %d (%s)", len(items), res.raw)
	}
	// 按 sort 升序：login(10) → payment(20) → ad(40)。
	assertField(t, items[0], "categoryCode", "login")
	assertField(t, items[1], "categoryCode", "payment")
	assertField(t, items[2], "categoryCode", "ad")

	// 视图字段齐全，pluginCount 为该分类下插件数（login 下有 realname）。
	assertField(t, items[0], "id", float64(1))
	assertField(t, items[0], "categoryName", "登录类")
	assertField(t, items[0], "enabled", true)
	assertField(t, items[0], "sort", float64(10))
	assertField(t, items[0], "pluginCount", float64(1))
	assertField(t, items[2], "enabled", false)
	assertField(t, items[2], "pluginCount", float64(0))
	if items[0]["createdAt"] == nil || items[0]["updatedAt"] == nil {
		t.Fatalf("createdAt/updatedAt should be present, got %v", items[0])
	}

	// enabled 过滤。
	enabled := h.do(t, http.MethodGet, "/api/admin/feature-plugin-categories?enabled=true", token, nil)
	if len(enabled.items(t)) != 2 {
		t.Fatalf("want 2 enabled categories, got %s", enabled.raw)
	}
	disabled := h.do(t, http.MethodGet, "/api/admin/feature-plugin-categories?enabled=false", token, nil)
	disabledItems := disabled.items(t)
	if len(disabledItems) != 1 || disabledItems[0]["categoryCode"] != "ad" {
		t.Fatalf("want only ad category, got %s", disabled.raw)
	}

	// enabled 非法 → 400。
	bad := h.do(t, http.MethodGet, "/api/admin/feature-plugin-categories?enabled=maybe", token, nil)
	assertStatus(t, bad, http.StatusBadRequest)
	if bad.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q", bad.errCode())
	}
}

func TestCreateFeaturePluginCategorySuccess(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/admin/feature-plugin-categories", h.writeToken(t), map[string]any{
		"categoryCode": "push", "categoryName": "推送类", "sort": 30,
	})
	assertStatus(t, res, http.StatusCreated)
	d := res.data()
	assertField(t, d, "categoryCode", "push")
	assertField(t, d, "categoryName", "推送类")
	assertField(t, d, "sort", float64(30))
	assertField(t, d, "enabled", true) // 缺省启用
	assertField(t, d, "pluginCount", float64(0))

	// 落库可回读（新分类排在 payment 之后、ad 之前）。
	list := h.do(t, http.MethodGet, "/api/admin/feature-plugin-categories", h.readToken(t), nil)
	items := list.items(t)
	if len(items) != 4 || items[2]["categoryCode"] != "push" {
		t.Fatalf("want push at index 2 of 4, got %s", list.raw)
	}

	// 审计：feature_plugin_category.create 写入，带 actor 与关键字段。
	entry, ok := h.audit.byAction("feature_plugin_category.create")
	if !ok {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ActorID != 11 {
		t.Fatalf("audit actor: want 11 got %d", entry.ActorID)
	}
	if entry.ResourceType != "feature_plugin_category" {
		t.Fatalf("audit resource type: got %s", entry.ResourceType)
	}
	if entry.Detail["categoryCode"] != "push" {
		t.Fatalf("audit detail: got %+v", entry.Detail)
	}
}

func TestCreateFeaturePluginCategoryDuplicate(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/admin/feature-plugin-categories", h.writeToken(t), map[string]any{
		"categoryCode": "login", "categoryName": "登录类 2",
	})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}
	// 冲突不得改动既有行。
	list := h.do(t, http.MethodGet, "/api/admin/feature-plugin-categories", h.readToken(t), nil)
	assertField(t, list.items(t)[0], "categoryName", "登录类")
}

func TestCreateFeaturePluginCategoryValidation(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)
	cases := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"categoryCode 含大写", map[string]any{"categoryCode": "Login2", "categoryName": "登录"}, "categoryCode"},
		{"categoryCode 含横线", map[string]any{"categoryCode": "log-in", "categoryName": "登录"}, "categoryCode"},
		{"categoryCode 数字开头", map[string]any{"categoryCode": "1login", "categoryName": "登录"}, "categoryCode"},
		{"categoryCode 为空", map[string]any{"categoryCode": "", "categoryName": "登录"}, "categoryCode"},
		{"categoryName 为空", map[string]any{"categoryCode": "push2", "categoryName": "  "}, "categoryName"},
		{"sort 越界", map[string]any{"categoryCode": "push2", "categoryName": "推送", "sort": 10000}, "sort"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := h.do(t, http.MethodPost, "/api/admin/feature-plugin-categories", token, tc.body)
			assertStatus(t, res, http.StatusBadRequest)
			if res.errCode() != "VALIDATION_FAILED" {
				t.Fatalf("want VALIDATION_FAILED got %q", res.errCode())
			}
			assertDetailField(t, res, tc.field)
		})
	}
	// 校验失败不落库。
	list := h.do(t, http.MethodGet, "/api/admin/feature-plugin-categories", h.readToken(t), nil)
	if len(list.items(t)) != 3 {
		t.Fatalf("categories should stay at 3, got %s", list.raw)
	}
}

func TestUpdateFeaturePluginCategory(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	res := h.do(t, http.MethodPatch, "/api/admin/feature-plugin-categories/1", token, map[string]any{
		"categoryName": "登录与账号", "sort": 5, "enabled": false,
	})
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	assertField(t, d, "categoryName", "登录与账号")
	assertField(t, d, "sort", float64(5))
	assertField(t, d, "enabled", false)
	// 业务键不受编辑影响（categoryCode 是未知字段，被忽略而非生效）。
	assertField(t, d, "categoryCode", "login")

	ignored := h.do(t, http.MethodPatch, "/api/admin/feature-plugin-categories/1", token, map[string]any{
		"categoryCode": "login_v2",
	})
	assertStatus(t, ignored, http.StatusOK)
	assertField(t, ignored.data(), "categoryCode", "login")

	// 审计记录改动字段集合。
	entry, ok := h.audit.byAction("feature_plugin_category.update")
	if !ok {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ResourceID != "1" {
		t.Fatalf("audit resource id: got %s", entry.ResourceID)
	}
	fields, _ := entry.Detail["fields"].([]string)
	for _, want := range []string{"categoryName", "enabled", "sort"} {
		if !containsString(fields, want) {
			t.Fatalf("audit fields %v missing %s", fields, want)
		}
	}
}

func TestUpdateFeaturePluginCategoryValidationAndNotFound(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	bad := h.do(t, http.MethodPatch, "/api/admin/feature-plugin-categories/1", token, map[string]any{"categoryName": " "})
	assertStatus(t, bad, http.StatusBadRequest)
	assertDetailField(t, bad, "categoryName")

	assertStatus(t, h.do(t, http.MethodPatch, "/api/admin/feature-plugin-categories/999", token,
		map[string]any{"categoryName": "X"}), http.StatusNotFound)
	assertStatus(t, h.do(t, http.MethodPatch, "/api/admin/feature-plugin-categories/abc", token,
		map[string]any{"categoryName": "X"}), http.StatusBadRequest)

	// 空 patch 为幂等无操作：不报错、不写审计。
	noop := h.do(t, http.MethodPatch, "/api/admin/feature-plugin-categories/1", token, map[string]any{})
	assertStatus(t, noop, http.StatusOK)
	assertField(t, noop.data(), "categoryName", "登录类")
	if _, ok := h.audit.byAction("feature_plugin_category.update"); ok {
		t.Fatalf("no-op patch should not write audit, got %+v", h.audit.entries)
	}
}

func TestDeleteFeaturePluginCategory(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	// login 分类下仍有 realname 插件 → 409，且分类不被删除。
	blocked := h.do(t, http.MethodDelete, "/api/admin/feature-plugin-categories/1", token, nil)
	assertStatus(t, blocked, http.StatusConflict)
	if blocked.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", blocked.errCode())
	}
	if !containsSubstring(blocked.errMessage(), "该分类下仍有插件，无法删除") {
		t.Fatalf("message should explain the conflict, got %q", blocked.errMessage())
	}
	if _, ok := h.audit.byAction("feature_plugin_category.delete"); ok {
		t.Fatalf("blocked delete should not write audit, got %+v", h.audit.entries)
	}

	// ad 分类无插件 → 204。
	ok := h.do(t, http.MethodDelete, "/api/admin/feature-plugin-categories/3", token, nil)
	assertStatus(t, ok, http.StatusNoContent)
	if ok.raw != "" {
		t.Fatalf("204 should have empty body, got %q", ok.raw)
	}
	list := h.do(t, http.MethodGet, "/api/admin/feature-plugin-categories", h.readToken(t), nil)
	if len(list.items(t)) != 2 {
		t.Fatalf("want 2 categories after delete, got %s", list.raw)
	}

	entry, found := h.audit.byAction("feature_plugin_category.delete")
	if !found {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ResourceID != "3" || entry.Detail["categoryCode"] != "ad" {
		t.Fatalf("audit entry: got %s / %+v", entry.ResourceID, entry.Detail)
	}

	// 已删除 / 不存在 → 404；非法 id → 400。
	assertStatus(t, h.do(t, http.MethodDelete, "/api/admin/feature-plugin-categories/3", token, nil), http.StatusNotFound)
	assertStatus(t, h.do(t, http.MethodDelete, "/api/admin/feature-plugin-categories/0", token, nil), http.StatusBadRequest)

	// 转移插件后原本被占用的分类可删除。
	moved := h.do(t, http.MethodPatch, "/api/admin/feature-plugins/realname", token, map[string]any{"categoryId": 2})
	assertStatus(t, moved, http.StatusOK)
	assertStatus(t, h.do(t, http.MethodDelete, "/api/admin/feature-plugin-categories/1", token, nil), http.StatusNoContent)
}

// ───────────────────────── 插件主数据：列表与详情 ─────────────────────────

func TestListFeaturePlugins(t *testing.T) {
	h := newHarness(t)
	token := h.readToken(t)

	res := h.do(t, http.MethodGet, "/api/admin/feature-plugins", token, nil)
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	assertField(t, d, "page", float64(1))
	assertField(t, d, "pageSize", float64(20))
	assertField(t, d, "total", float64(3))

	items := res.items(t)
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d (%s)", len(items), res.raw)
	}
	// 按 sort 升序。
	assertField(t, items[0], "pluginId", "realname")
	assertField(t, items[1], "pluginId", "apple_pay")
	assertField(t, items[2], "pluginId", "customer_service")

	// 视图字段齐全，含分类冗余字段与模板计数。
	rn := items[0]
	assertField(t, rn, "pluginName", "实名认证")
	assertField(t, rn, "categoryId", float64(1))
	assertField(t, rn, "categoryCode", "login")
	assertField(t, rn, "categoryName", "登录类")
	assertField(t, rn, "region", "domestic")
	assertField(t, rn, "enabled", true)
	assertField(t, rn, "sort", float64(1))
	assertField(t, rn, "templateCount", float64(2))
	if rn["updatedAt"] == nil {
		t.Fatalf("updatedAt should be present, got %v", rn)
	}

	// 未归类插件的 categoryId 为 null，分类展示字段为空串。
	cs := items[2]
	if cs["categoryId"] != nil {
		t.Fatalf("categoryId should be null for uncategorized plugin, got %v", cs["categoryId"])
	}
	assertField(t, cs, "categoryCode", "")
	assertField(t, cs, "categoryName", "")
	assertField(t, cs, "templateCount", float64(0))

	// 分页：pageSize=1 时 total 仍为全量。
	paged := h.do(t, http.MethodGet, "/api/admin/feature-plugins?page=2&pageSize=1", token, nil)
	assertStatus(t, paged, http.StatusOK)
	assertField(t, paged.data(), "total", float64(3))
	pagedItems := paged.items(t)
	if len(pagedItems) != 1 || pagedItems[0]["pluginId"] != "apple_pay" {
		t.Fatalf("page 2 want apple_pay only, got %s", paged.raw)
	}
}

func TestListFeaturePluginsFilters(t *testing.T) {
	h := newHarness(t)
	token := h.readToken(t)

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"region 过滤 domestic", "?region=domestic", []string{"realname", "customer_service"}},
		{"region 过滤 overseas", "?region=overseas", []string{"apple_pay"}},
		{"categoryId 过滤", "?categoryId=2", []string{"apple_pay"}},
		{"keyword 命中 pluginId", "?keyword=apple", []string{"apple_pay"}},
		{"keyword 命中插件名", "?keyword=实名", []string{"realname"}},
		{"enabled=true", "?enabled=true", []string{"realname", "apple_pay"}},
		{"enabled=false", "?enabled=false", []string{"customer_service"}},
		{"组合过滤", "?region=domestic&enabled=true", []string{"realname"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := h.do(t, http.MethodGet, "/api/admin/feature-plugins"+tc.query, token, nil)
			assertStatus(t, res, http.StatusOK)
			items := res.items(t)
			if len(items) != len(tc.want) {
				t.Fatalf("want %d items, got %d (%s)", len(tc.want), len(items), res.raw)
			}
			for i, id := range tc.want {
				assertField(t, items[i], "pluginId", id)
			}
		})
	}

	// 非法过滤参数 → 400 VALIDATION_FAILED。
	for _, q := range []string{"?region=mars", "?enabled=maybe", "?categoryId=abc", "?categoryId=-1"} {
		res := h.do(t, http.MethodGet, "/api/admin/feature-plugins"+q, token, nil)
		assertStatus(t, res, http.StatusBadRequest)
		if res.errCode() != "VALIDATION_FAILED" {
			t.Fatalf("%s want VALIDATION_FAILED got %q", q, res.errCode())
		}
	}
}

func TestGetFeaturePlugin(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/feature-plugins/realname", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	assertField(t, res.data(), "pluginId", "realname")
	assertField(t, res.data(), "categoryCode", "login")
	assertField(t, res.data(), "templateCount", float64(2))

	missing := h.do(t, http.MethodGet, "/api/admin/feature-plugins/nope", h.readToken(t), nil)
	assertStatus(t, missing, http.StatusNotFound)
	if missing.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", missing.errCode())
	}
}

// ───────────────────────── 插件主数据：创建 ─────────────────────────

func TestCreateFeaturePluginSuccess(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/admin/feature-plugins", h.writeToken(t), map[string]any{
		"pluginId": "push_sdk", "pluginName": "推送 SDK", "categoryId": 2, "region": "overseas", "sort": 5,
	})
	assertStatus(t, res, http.StatusCreated)
	d := res.data()
	assertField(t, d, "pluginId", "push_sdk")
	assertField(t, d, "pluginName", "推送 SDK")
	assertField(t, d, "categoryId", float64(2))
	assertField(t, d, "categoryCode", "payment")
	assertField(t, d, "region", "overseas")
	assertField(t, d, "sort", float64(5))
	assertField(t, d, "enabled", true) // 缺省启用
	assertField(t, d, "templateCount", float64(0))

	// 落库可回读。
	got := h.do(t, http.MethodGet, "/api/admin/feature-plugins/push_sdk", h.readToken(t), nil)
	assertStatus(t, got, http.StatusOK)
	assertField(t, got.data(), "pluginName", "推送 SDK")

	// categoryId 省略 → 未归类。
	uncategorized := h.do(t, http.MethodPost, "/api/admin/feature-plugins", h.writeToken(t), map[string]any{
		"pluginId": "share_sdk", "pluginName": "分享", "region": "domestic", "enabled": false,
	})
	assertStatus(t, uncategorized, http.StatusCreated)
	if uncategorized.data()["categoryId"] != nil {
		t.Fatalf("categoryId should be null, got %v", uncategorized.data()["categoryId"])
	}
	assertField(t, uncategorized.data(), "enabled", false)

	// 审计 feature_plugin.create。
	entry, ok := h.audit.byAction("feature_plugin.create")
	if !ok {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ActorID != 11 || entry.ResourceType != "feature_plugin" || entry.ResourceID != "push_sdk" {
		t.Fatalf("audit entry: got %d / %s / %s", entry.ActorID, entry.ResourceType, entry.ResourceID)
	}
	if entry.Detail["region"] != "overseas" {
		t.Fatalf("audit detail: got %+v", entry.Detail)
	}
}

func TestCreateFeaturePluginDuplicate(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/admin/feature-plugins", h.writeToken(t), map[string]any{
		"pluginId": "realname", "pluginName": "实名认证 2", "region": "overseas",
	})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}
	// 冲突不得改动既有行。
	got := h.do(t, http.MethodGet, "/api/admin/feature-plugins/realname", h.readToken(t), nil)
	assertField(t, got.data(), "pluginName", "实名认证")
	assertField(t, got.data(), "region", "domestic")
}

func TestCreateFeaturePluginValidation(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)
	base := func(over map[string]any) map[string]any {
		body := map[string]any{"pluginId": "ok_id", "pluginName": "名字", "region": "domestic"}
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
		{"pluginId 含大写", base(map[string]any{"pluginId": "Realname"}), "pluginId"},
		{"pluginId 含横线", base(map[string]any{"pluginId": "real-name"}), "pluginId"},
		{"pluginId 数字开头", base(map[string]any{"pluginId": "1realname"}), "pluginId"},
		{"pluginId 为空", base(map[string]any{"pluginId": ""}), "pluginId"},
		{"pluginName 为空", base(map[string]any{"pluginName": "  "}), "pluginName"},
		{"region 非枚举", base(map[string]any{"region": "CN"}), "region"},
		{"region 缺失", base(map[string]any{"region": ""}), "region"},
		{"sort 越界", base(map[string]any{"sort": 10000}), "sort"},
		{"categoryId 不存在", base(map[string]any{"categoryId": 999}), "categoryId"},
		{"categoryId 为负数", base(map[string]any{"categoryId": -1}), "categoryId"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := h.do(t, http.MethodPost, "/api/admin/feature-plugins", token, tc.body)
			assertStatus(t, res, http.StatusBadRequest)
			if res.errCode() != "VALIDATION_FAILED" {
				t.Fatalf("want VALIDATION_FAILED got %q", res.errCode())
			}
			assertDetailField(t, res, tc.field)
		})
	}
	// 校验失败不落库。
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins/ok_id", h.readToken(t), nil), http.StatusNotFound)
}

// ───────────────────────── 插件主数据：编辑与删除 ─────────────────────────

func TestUpdateFeaturePlugin(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	res := h.do(t, http.MethodPatch, "/api/admin/feature-plugins/realname", token, map[string]any{
		"pluginName": "实名认证（新）", "categoryId": 2, "sort": 9, "enabled": false,
	})
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	assertField(t, d, "pluginName", "实名认证（新）")
	assertField(t, d, "categoryId", float64(2))
	assertField(t, d, "categoryCode", "payment")
	assertField(t, d, "sort", float64(9))
	assertField(t, d, "enabled", false)
	// 身份与兼容性判定口径不受编辑影响（pluginId / region 是未知字段，被忽略而非生效）。
	assertField(t, d, "pluginId", "realname")
	assertField(t, d, "region", "domestic")

	ignored := h.do(t, http.MethodPatch, "/api/admin/feature-plugins/realname", token, map[string]any{
		"pluginId": "realname_v2", "region": "overseas",
	})
	assertStatus(t, ignored, http.StatusOK)
	assertField(t, ignored.data(), "pluginId", "realname")
	assertField(t, ignored.data(), "region", "domestic")
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins/realname_v2", token, nil), http.StatusNotFound)

	// 审计记录改动字段集合。
	entry, ok := h.audit.byAction("feature_plugin.update")
	if !ok {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ResourceID != "realname" {
		t.Fatalf("audit resource id: got %s", entry.ResourceID)
	}
	fields, _ := entry.Detail["fields"].([]string)
	for _, want := range []string{"pluginName", "categoryId", "enabled", "sort"} {
		if !containsString(fields, want) {
			t.Fatalf("audit fields %v missing %s", fields, want)
		}
	}
}

// categoryId 显式传 null（或 0）表示取消归属分类；省略该键则保持原样。
func TestUpdateFeaturePluginClearsCategory(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	cleared := h.doRaw(t, http.MethodPatch, "/api/admin/feature-plugins/realname", token, `{"categoryId":null}`)
	assertStatus(t, cleared, http.StatusOK)
	if cleared.data()["categoryId"] != nil {
		t.Fatalf("categoryId should be cleared, got %v", cleared.data()["categoryId"])
	}
	assertField(t, cleared.data(), "categoryCode", "")

	// 省略 categoryId 时不改动归属。
	restored := h.do(t, http.MethodPatch, "/api/admin/feature-plugins/apple_pay", token, map[string]any{"sort": 7})
	assertStatus(t, restored, http.StatusOK)
	assertField(t, restored.data(), "categoryId", float64(2))
}

func TestUpdateFeaturePluginValidationAndNotFound(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	bad := h.do(t, http.MethodPatch, "/api/admin/feature-plugins/realname", token, map[string]any{"pluginName": " "})
	assertStatus(t, bad, http.StatusBadRequest)
	assertDetailField(t, bad, "pluginName")

	missingCategory := h.do(t, http.MethodPatch, "/api/admin/feature-plugins/realname", token, map[string]any{"categoryId": 999})
	assertStatus(t, missingCategory, http.StatusBadRequest)
	assertDetailField(t, missingCategory, "categoryId")

	assertStatus(t, h.do(t, http.MethodPatch, "/api/admin/feature-plugins/nope", token,
		map[string]any{"pluginName": "X"}), http.StatusNotFound)

	// 校验失败与 404 都不改动既有行、不写审计。
	got := h.do(t, http.MethodGet, "/api/admin/feature-plugins/realname", token, nil)
	assertField(t, got.data(), "pluginName", "实名认证")
	assertField(t, got.data(), "categoryId", float64(1))

	// 空 patch 为幂等无操作。
	noop := h.do(t, http.MethodPatch, "/api/admin/feature-plugins/realname", token, map[string]any{})
	assertStatus(t, noop, http.StatusOK)
	if _, ok := h.audit.byAction("feature_plugin.update"); ok {
		t.Fatalf("no-op patch should not write audit, got %+v", h.audit.entries)
	}
}

func TestDeleteFeaturePlugin(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	// apple_pay 无模板但被渠道绑定策略引用 → 409（渠道侧引用仍阻断删除）。
	h.store.state.channelRefs[2] = 2
	channelBlocked := h.do(t, http.MethodDelete, "/api/admin/feature-plugins/apple_pay", token, nil)
	assertStatus(t, channelBlocked, http.StatusConflict)
	if channelBlocked.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", channelBlocked.errCode())
	}
	if !containsSubstring(channelBlocked.errMessage(), "渠道绑定 2 条") {
		t.Fatalf("message should list referencing data, got %q", channelBlocked.errMessage())
	}
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins/apple_pay", token, nil), http.StatusOK)
	delete(h.store.state.channelRefs, 2)

	// apple_pay 被游戏侧渠道实例配置引用 → 409。
	h.store.state.gameRefs[2] = 3
	gameBlocked := h.do(t, http.MethodDelete, "/api/admin/feature-plugins/apple_pay", token, nil)
	assertStatus(t, gameBlocked, http.StatusConflict)
	if !containsSubstring(gameBlocked.errMessage(), "渠道实例配置 3 条") {
		t.Fatalf("message should mention game-side refs, got %q", gameBlocked.errMessage())
	}
	if _, ok := h.audit.byAction("feature_plugin.delete"); ok {
		t.Fatalf("blocked delete should not write audit, got %+v", h.audit.entries)
	}
	delete(h.store.state.gameRefs, 2)

	// apple_pay 被渠道包覆盖引用 → 409（三项外部引用里此前唯一没有 HTTP 层覆盖的一项）。
	h.store.state.packageOverrideRefs[2] = 5
	packageBlocked := h.do(t, http.MethodDelete, "/api/admin/feature-plugins/apple_pay", token, nil)
	assertStatus(t, packageBlocked, http.StatusConflict)
	if packageBlocked.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", packageBlocked.errCode())
	}
	if !containsSubstring(packageBlocked.errMessage(), "渠道包覆盖 5 条") {
		t.Fatalf("message should mention package override refs, got %q", packageBlocked.errMessage())
	}
	if containsSubstring(packageBlocked.errMessage(), "参数模板") {
		t.Fatalf("message must never mention 参数模板 (regression guard), got %q", packageBlocked.errMessage())
	}
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins/apple_pay", token, nil), http.StatusOK)
	if _, ok := h.audit.byAction("feature_plugin.delete"); ok {
		t.Fatalf("blocked delete should not write audit, got %+v", h.audit.entries)
	}
	delete(h.store.state.packageOverrideRefs, 2)

	// realname 有 2 个模板版本但无渠道侧引用 → 204，模板随插件级联删除，
	// 且 409 文案不再把「参数模板」列为需要先清理的项（否则建过模板的插件永远删不掉）。
	cascade := h.do(t, http.MethodDelete, "/api/admin/feature-plugins/realname", token, nil)
	assertStatus(t, cascade, http.StatusNoContent)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins/realname", token, nil), http.StatusNotFound)
	for id, tpl := range h.store.state.templates {
		if tpl.PluginID == "realname" {
			t.Fatalf("template %d of realname should be cascade-deleted, got %+v", id, tpl)
		}
	}
	cascadeAudit, found := h.audit.byAction("feature_plugin.delete")
	if !found {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if cascadeAudit.ResourceID != "realname" {
		t.Fatalf("audit resource id: got %s", cascadeAudit.ResourceID)
	}
	if n, _ := cascadeAudit.Detail["deletedTemplates"].(int); n != 2 {
		t.Fatalf("audit should record cascade-deleted template count 2, got %+v", cascadeAudit.Detail)
	}

	// customer_service 无模板、无任何引用 → 204，审计 deletedTemplates=0。
	ok := h.do(t, http.MethodDelete, "/api/admin/feature-plugins/customer_service", token, nil)
	assertStatus(t, ok, http.StatusNoContent)
	if ok.raw != "" {
		t.Fatalf("204 should have empty body, got %q", ok.raw)
	}
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins/customer_service", token, nil), http.StatusNotFound)

	plainAudit := h.audit.entries[len(h.audit.entries)-1]
	if plainAudit.Action != "feature_plugin.delete" || plainAudit.ResourceID != "customer_service" {
		t.Fatalf("last audit entry should be the customer_service delete, got %+v", plainAudit)
	}
	if n, _ := plainAudit.Detail["deletedTemplates"].(int); n != 0 {
		t.Fatalf("plugin without templates should record deletedTemplates=0, got %+v", plainAudit.Detail)
	}

	// 重复删除 → 404。
	assertStatus(t, h.do(t, http.MethodDelete, "/api/admin/feature-plugins/customer_service", token, nil), http.StatusNotFound)
}

// TestDeleteFeaturePluginRollsBackCascade 校验「模板级联删除 + 插件删除」的事务边界：
// 插件删除失败时，已删除的模板必须随事务一起回滚，不留下「模板被清空但插件还在」的半删状态。
func TestDeleteFeaturePluginRollsBackCascade(t *testing.T) {
	h := newHarness(t)

	// 让插件删除在模板已删之后失败：直接把 realname 的主数据行从内存态摘掉，
	// Plugins.Delete 会返回 ErrNotFound（等价于并发删除/外键挡回），触发 InTx 回滚。
	svc := featurepluginapp.NewService(&deleteFailingStore{memStore: h.store}, h.audit)
	if err := svc.DeletePlugin(context.Background(), "realname"); err == nil {
		t.Fatal("want error when plugin delete fails")
	}
	count := 0
	for _, tpl := range h.store.state.templates {
		if tpl.PluginID == "realname" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("templates must be restored by rollback, want 2 got %d", count)
	}
	if _, ok := h.audit.byAction("feature_plugin.delete"); ok {
		t.Fatalf("failed delete should not write audit, got %+v", h.audit.entries)
	}
}

// ───────────────────────── 参数模板：列表与 effective 标记 ─────────────────────────

func TestListFeaturePluginTemplatesEffective(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/feature-plugins/realname/templates", h.readToken(t), nil)
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

	// 视图带插件业务键与四件套。
	assertField(t, items[1], "pluginId", "realname")
	assertField(t, items[1], "templateId", float64(1))
	if _, ok := items[1]["formSchemaJson"].([]any); !ok {
		t.Fatalf("formSchemaJson should be an array, got %v", items[1]["formSchemaJson"])
	}
	if _, ok := items[1]["validationRulesJson"].(map[string]any); !ok {
		t.Fatalf("validationRulesJson should be an object, got %v", items[1]["validationRulesJson"])
	}

	// 无模板的插件 → 空数组而非 null；插件不存在 → 404。
	empty := h.do(t, http.MethodGet, "/api/admin/feature-plugins/apple_pay/templates", h.readToken(t), nil)
	assertStatus(t, empty, http.StatusOK)
	if len(empty.items(t)) != 0 {
		t.Fatalf("want empty items, got %s", empty.raw)
	}
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugins/nope/templates", h.readToken(t), nil), http.StatusNotFound)
}

// 禁用生效版本后，effective 顺延到下一个 enabled 的最新版本。
func TestFeaturePluginTemplateEffectiveFollowsEnabledFlag(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	// 启用 v2 → v2 成为生效版本（版本号更大）。
	res := h.do(t, http.MethodPatch, "/api/admin/feature-plugin-templates/2", token, map[string]any{"enabled": true})
	assertStatus(t, res, http.StatusOK)
	assertField(t, res.data(), "templateVersion", "v2")
	assertField(t, res.data(), "effective", true)

	list := h.do(t, http.MethodGet, "/api/admin/feature-plugins/realname/templates", token, nil)
	items := list.items(t)
	assertField(t, items[0], "templateVersion", "v2")
	assertField(t, items[0], "effective", true)
	assertField(t, items[1], "templateVersion", "v1")
	assertField(t, items[1], "effective", false)

	// 审计写入 feature_plugin_template.update，记录被改的件。
	entry, ok := h.audit.byAction("feature_plugin_template.update")
	if !ok {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ResourceType != "feature_plugin_template" || entry.ResourceID != "2" {
		t.Fatalf("audit resource: got %s/%s", entry.ResourceType, entry.ResourceID)
	}
	if entry.Detail["pluginId"] != "realname" || entry.Detail["templateVersion"] != "v2" {
		t.Fatalf("audit detail: got %+v", entry.Detail)
	}
	fields, _ := entry.Detail["fields"].([]string)
	if !containsString(fields, "enabled") {
		t.Fatalf("audit fields %v missing enabled", fields)
	}
}

func TestGetFeaturePluginTemplate(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/feature-plugin-templates/1", h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	assertField(t, res.data(), "templateId", float64(1))
	assertField(t, res.data(), "pluginId", "realname")
	assertField(t, res.data(), "templateVersion", "v1")
	assertField(t, res.data(), "effective", true)

	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugin-templates/999", h.readToken(t), nil), http.StatusNotFound)
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/feature-plugin-templates/abc", h.readToken(t), nil), http.StatusBadRequest)
}

// ───────────────────────── 参数模板：创建 ─────────────────────────

func TestCreateFeaturePluginTemplateSuccess(t *testing.T) {
	h := newHarness(t)
	body := templateBody("v3")
	body["formSchemaJson"] = []map[string]any{
		{"key": "appId", "label": "App ID", "component": "input", "required": true, "order": 10, "scope": "both", "placeholder": "请输入 App ID"},
		{"key": "appSecret", "label": "App Secret", "component": "password", "required": true, "order": 20, "scope": "server"},
		{"key": "cert", "label": "证书文件", "component": "file", "required": false, "order": 30, "scope": "client"},
		{"key": "mode", "label": "模式", "component": "select", "required": true, "order": 40, "group": "高级",
			"options": []map[string]any{{"label": "静默", "value": "silent"}, {"label": "弹窗", "value": "dialog"}}},
	}
	body["fileFieldsJson"] = []map[string]any{{"key": "cert", "accept": []string{".p12"}, "maxSizeKB": 2048}}

	res := h.do(t, http.MethodPost, "/api/admin/feature-plugins/realname/templates", h.writeToken(t), body)
	assertStatus(t, res, http.StatusCreated)
	d := res.data()
	assertField(t, d, "pluginId", "realname")
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
	assertField(t, fileEntry, "key", "cert")
	assertField(t, fileEntry, "maxSizeKB", float64(2048))
	secrets, _ := d["secretFieldsJson"].([]any)
	if len(secrets) != 1 || secrets[0] != "appSecret" {
		t.Fatalf("want secretFieldsJson [appSecret], got %v", d["secretFieldsJson"])
	}

	// 插件视图上的模板计数随之 +1（原 2 个）。
	p := h.do(t, http.MethodGet, "/api/admin/feature-plugins/realname", h.readToken(t), nil)
	assertField(t, p.data(), "templateCount", float64(3))

	// 审计 feature_plugin_template.create。
	entry, ok := h.audit.byAction("feature_plugin_template.create")
	if !ok {
		t.Fatalf("missing audit entry, got %+v", h.audit.entries)
	}
	if entry.ResourceType != "feature_plugin_template" {
		t.Fatalf("audit resource type: got %s", entry.ResourceType)
	}
	if entry.Detail["pluginId"] != "realname" || entry.Detail["templateVersion"] != "v3" {
		t.Fatalf("audit detail: got %+v", entry.Detail)
	}
}

func TestCreateFeaturePluginTemplateDuplicateVersion(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/admin/feature-plugins/realname/templates", h.writeToken(t), templateBody("v1"))
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q", res.errCode())
	}
	// 冲突回滚：不应多出一个版本。
	p := h.do(t, http.MethodGet, "/api/admin/feature-plugins/realname", h.readToken(t), nil)
	assertField(t, p.data(), "templateCount", float64(2))

	// 不同插件下同版本号互不冲突。
	other := h.do(t, http.MethodPost, "/api/admin/feature-plugins/apple_pay/templates", h.writeToken(t), templateBody("v1"))
	assertStatus(t, other, http.StatusCreated)
	assertField(t, other.data(), "pluginId", "apple_pay")
	assertField(t, other.data(), "templateVersion", "v1")
}

func TestCreateFeaturePluginTemplateValidation(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	withForm := func(fields []map[string]any, over map[string]any) map[string]any {
		body := templateBody("v9")
		body["formSchemaJson"] = fields
		body["secretFieldsJson"] = []string{}
		body["validationRulesJson"] = map[string]any{}
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
			body:  withForm([]map[string]any{{"key": "cert", "label": "证书", "component": "file", "order": 10}}, nil),
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
			res := h.do(t, http.MethodPost, "/api/admin/feature-plugins/apple_pay/templates", token, tc.body)
			assertStatus(t, res, http.StatusBadRequest)
			if res.errCode() != "VALIDATION_FAILED" {
				t.Fatalf("want VALIDATION_FAILED got %q", res.errCode())
			}
			assertDetailField(t, res, tc.field)
		})
	}

	// 插件不存在 → 404。
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/feature-plugins/nope/templates", token, templateBody("v9")), http.StatusNotFound)

	// 全部校验失败后 apple_pay 仍无模板。
	list := h.do(t, http.MethodGet, "/api/admin/feature-plugins/apple_pay/templates", token, nil)
	if len(list.items(t)) != 0 {
		t.Fatalf("apple_pay should have no templates, got %s", list.raw)
	}
}

// ───────────────────────── 参数模板：编辑（整体替换语义） ─────────────────────────

func TestUpdateFeaturePluginTemplateReplacesQuartet(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	// 先给 v1 补一份带文件字段的四件套，便于验证「省略件保留原值」。
	seeded := h.do(t, http.MethodPatch, "/api/admin/feature-plugin-templates/1", token, map[string]any{
		"formSchemaJson": []map[string]any{
			{"key": "appId", "label": "App ID", "component": "input", "required": true, "order": 10},
			{"key": "cert", "label": "证书", "component": "file", "required": false, "order": 20},
		},
		"fileFieldsJson": []map[string]any{{"key": "cert", "accept": []string{".p12"}}},
	})
	assertStatus(t, seeded, http.StatusOK)

	res := h.do(t, http.MethodPatch, "/api/admin/feature-plugin-templates/1", token, map[string]any{
		"formSchemaJson": []map[string]any{
			{"key": "clientId", "label": "Client ID", "component": "input", "required": true, "order": 10},
			{"key": "clientSecret", "label": "Client Secret", "component": "password", "required": true, "order": 20},
			{"key": "cert", "label": "证书", "component": "file", "required": false, "order": 30},
		},
		"secretFieldsJson":    []string{"clientSecret"},
		"validationRulesJson": map[string]any{"clientId": map[string]any{"required": true, "maxLen": 32}},
	})
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	// 版本与所属插件不可改。
	assertField(t, d, "templateVersion", "v1")
	assertField(t, d, "pluginId", "realname")

	// 传入的三件被整体替换。
	form, _ := d["formSchemaJson"].([]any)
	if len(form) != 3 {
		t.Fatalf("want 3 fields after replace, got %v", d["formSchemaJson"])
	}
	first, _ := form[0].(map[string]any)
	assertField(t, first, "key", "clientId")
	secrets, _ := d["secretFieldsJson"].([]any)
	if len(secrets) != 1 || secrets[0] != "clientSecret" {
		t.Fatalf("secretFieldsJson should be replaced wholesale, got %v", d["secretFieldsJson"])
	}
	rules, _ := d["validationRulesJson"].(map[string]any)
	if _, ok := rules["clientId"]; !ok {
		t.Fatalf("want clientId rule, got %v", d["validationRulesJson"])
	}
	if _, stale := rules["appId"]; stale {
		t.Fatalf("old appId rule should be gone, got %v", d["validationRulesJson"])
	}

	// 未传的 fileFieldsJson 保持原值（仍含 cert）。
	files, _ := d["fileFieldsJson"].([]any)
	if len(files) != 1 {
		t.Fatalf("fileFieldsJson should keep its previous value, got %v", d["fileFieldsJson"])
	}
	fileEntry, _ := files[0].(map[string]any)
	assertField(t, fileEntry, "key", "cert")

	// 显式传空数组则清空该件（与「省略=不改」区分）。
	cleared := h.do(t, http.MethodPatch, "/api/admin/feature-plugin-templates/1", token, map[string]any{
		"formSchemaJson": []map[string]any{
			{"key": "clientId", "label": "Client ID", "component": "input", "required": true, "order": 10},
		},
		"secretFieldsJson": []string{},
		"fileFieldsJson":   []map[string]any{},
	})
	assertStatus(t, cleared, http.StatusOK)
	if files, _ := cleared.data()["fileFieldsJson"].([]any); len(files) != 0 {
		t.Fatalf("fileFieldsJson should be cleared, got %v", cleared.data()["fileFieldsJson"])
	}
	if secrets, _ := cleared.data()["secretFieldsJson"].([]any); len(secrets) != 0 {
		t.Fatalf("secretFieldsJson should be cleared, got %v", cleared.data()["secretFieldsJson"])
	}
}

func TestUpdateFeaturePluginTemplateValidationKeepsOldState(t *testing.T) {
	h := newHarness(t)
	token := h.writeToken(t)

	// 换成 password 字段但不登记敏感字段 → 400，且原模板不受影响。
	bad := h.do(t, http.MethodPatch, "/api/admin/feature-plugin-templates/1", token, map[string]any{
		"formSchemaJson": []map[string]any{
			{"key": "secretKey", "label": "密钥", "component": "password", "required": true, "order": 10},
		},
	})
	assertStatus(t, bad, http.StatusBadRequest)
	assertDetailField(t, bad, "secretFieldsJson")

	got := h.do(t, http.MethodGet, "/api/admin/feature-plugin-templates/1", token, nil)
	assertStatus(t, got, http.StatusOK)
	form, _ := got.data()["formSchemaJson"].([]any)
	if len(form) != 1 {
		t.Fatalf("want original single field, got %v", got.data()["formSchemaJson"])
	}
	if first, _ := form[0].(map[string]any); first["key"] != "appId" {
		t.Fatalf("original field should stay appId, got %v", form[0])
	}
	if _, ok := h.audit.byAction("feature_plugin_template.update"); ok {
		t.Fatalf("failed update should not write audit, got %+v", h.audit.entries)
	}

	// 空 patch 幂等；不存在的模板 → 404。
	noop := h.do(t, http.MethodPatch, "/api/admin/feature-plugin-templates/1", token, map[string]any{})
	assertStatus(t, noop, http.StatusOK)
	assertField(t, noop.data(), "templateVersion", "v1")
	assertStatus(t, h.do(t, http.MethodPatch, "/api/admin/feature-plugin-templates/999", token,
		map[string]any{"enabled": false}), http.StatusNotFound)
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

func containsSubstring(s, sub string) bool {
	return strings.Contains(s, sub)
}
