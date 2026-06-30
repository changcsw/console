package channels

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	channelloginapp "github.com/csw/console/services/admin-api/internal/app/channellogin"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
)

// 进程内 L3 接口测试（httptest 全链路 transport->app(channellogin)->domain + 内存仓储 + 真实 JWT/路由/中间件
// + spy cipher（"enc:"+明文 语义）+ spy AuditSink）。与 tests/backend/scenarios/channel-login.yaml 维度对齐，
// 等价覆盖 S1/S2/S3/S4/S5/S7/S8/S10；S6（schema 隔离）由连库 harness 承担（env 由 search_path 决定）；
// S9（分页）本模块两接口均单实例读/写 → N/A。

const (
	loginCfgPath1 = "/api/admin/game-channels/1/login-config" // huawei_cn channel_only（有模板）
	loginCfgPath2 = "/api/admin/game-channels/2/login-config" // google account_system（非 channel_only）
	loginCfgPath3 = "/api/admin/game-channels/3/login-config" // xiaomi_cn channel_only（无模板）
	loginCfgPath4 = "/api/admin/game-channels/4/login-config" // huawei_cn 复制实例（copiedFromMarket=GLOBAL）
)

type spyCipher struct{ encryptCalls int }

func (c *spyCipher) Encrypt(plain string) (string, error) {
	c.encryptCalls++
	return "enc:" + plain, nil
}

type loginHarness struct {
	router http.Handler
	store  *memLoginStore
	cipher *spyCipher
	audit  *fakeAudit
	issuer *infrajwt.Issuer
}

func newLoginHarness(t *testing.T) *loginHarness {
	t.Helper()
	issuer, err := infrajwt.NewIssuer(infrajwt.Config{
		Secret: "test-secret-please-change", Issuer: "admin-api",
		AccessTTL: 30 * time.Minute, RefreshTTL: 336 * time.Hour,
	})
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}

	store := newMemLoginStore()
	seedLoginFixtures(store.state)
	cipher := &spyCipher{}
	audit := &fakeAudit{}
	now := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	loginSvc := channelloginapp.NewService(store, cipher, nil, audit, now, testEnv)

	root := chi.NewRouter()
	sub := chi.NewRouter()
	// channel svc 传 nil：本测试只打 login-config 路由，不触达 channel 编排。
	RegisterRoutes(sub, NewHandler(nil, testEnv, loginSvc), issuer, testEnv, slog.New(slog.NewTextHandler(io.Discard, nil)), true)
	root.Mount("/api/admin", sub)

	return &loginHarness{router: root, store: store, cipher: cipher, audit: audit, issuer: issuer}
}

func seedLoginFixtures(st *memLoginState) {
	// 策略：huawei_cn/xiaomi_cn 为 channel_only；google 为 account_system。
	st.policies["huawei_cn"] = channel.ChannelPolicy{ChannelIDRef: 100, LoginMode: common.LoginModeChannelOnly, LoginLocked: true}
	st.policies["xiaomi_cn"] = channel.ChannelPolicy{ChannelIDRef: 300, LoginMode: common.LoginModeChannelOnly}
	st.policies["google"] = channel.ChannelPolicy{ChannelIDRef: 200, LoginMode: common.LoginModeAccountSystem}

	// 模板：仅 huawei_cn（channelIDRef=100）有 enabled v1 模板；xiaomi_cn 无模板。
	mn := func(v int) *int { return &v }
	st.templates[100] = channel.ChannelLoginTemplate{
		ID: 1, ChannelIDRef: 100, TemplateVersion: "v1", Enabled: true,
		FormSchema: []channel.ChannelLoginFormField{
			{Key: "appId", Label: "App ID", Component: "input", Required: true, Order: 10, Group: "basic"},
			{Key: "appSecret", Label: "App Secret", Component: "password", Required: true, Order: 20, Group: "secret"},
		},
		SecretFields: []string{"appSecret"},
		ValidationRules: map[string]channel.ChannelLoginValidationRule{
			"appId":     {MinLen: mn(1), MaxLen: mn(64), Pattern: "^[0-9A-Za-z_-]+$"},
			"appSecret": {MinLen: mn(8), MaxLen: mn(256)},
		},
	}

	st.seedInstance(1, 100, "huawei_cn", "CN", "")
	st.seedInstance(2, 200, "google", "JP", "")
	st.seedInstance(3, 300, "xiaomi_cn", "CN", "")
	st.seedInstance(4, 100, "huawei_cn", "KR", "GLOBAL")
}

func (h *loginHarness) token(t *testing.T, userID int64, perms []string) string {
	t.Helper()
	pair, err := h.issuer.IssuePair(domainauth.NewAuthContext(userID, "tester", "Tester", []string{"editor"}, perms, testEnv))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return pair.AccessToken
}

func (h *loginHarness) readToken(t *testing.T) string {
	return h.token(t, 30, []string{"channel.read"})
}
func (h *loginHarness) writeToken(t *testing.T) string {
	return h.token(t, 31, []string{"channel.read", "channel.write"})
}

func (h *loginHarness) do(t *testing.T, method, path, token string, body any) apiResp {
	t.Helper()
	hh := &harness{router: h.router}
	return hh.do(t, method, path, token, body)
}

// loginCfg 取响应 data.config 子对象。
func (r apiResp) loginCfg() map[string]any {
	if d := r.data(); d != nil {
		if c, ok := d["config"].(map[string]any); ok {
			return c
		}
	}
	return nil
}

// loginConfigJSON 取响应 data.config.configJson。
func (r apiResp) loginConfigJSON() map[string]any {
	if c := r.loginCfg(); c != nil {
		if j, ok := c["configJson"].(map[string]any); ok {
			return j
		}
	}
	return nil
}

func fullHuaweiConfig(secret string) map[string]any {
	return map[string]any{"enabled": true, "configJson": map[string]any{"appId": "app-123_AZ", "appSecret": secret}}
}

func (h *loginHarness) seedValid(t *testing.T, secret string) {
	t.Helper()
	res := h.do(t, http.MethodPut, loginCfgPath1, h.writeToken(t), fullHuaweiConfig(secret))
	assertStatus(t, res, http.StatusOK)
	if h.store.state.configs[1] == nil || h.store.state.configs[1].ConfigStatus != common.ConfigStatusValid {
		t.Fatalf("seed must persist valid config, got %v", h.store.state.configs[1])
	}
}

// ───────────────────────── GET（S1/S8/NOTFOUND/非 channel_only） ─────────────────────────

// S1：实例存在但未配置 → 返回空配置占位（enabled=false/configJson={}/configStatus=empty）+ 模板四件套。
func TestGetLoginConfigEmptyPlaceholder(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodGet, loginCfgPath1, h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	if d["loginMode"] != "channel_only" || d["marketCode"] != "CN" || d["channelId"] != "huawei_cn" {
		t.Fatalf("context echo mismatch: %v", d)
	}
	if d["loginLocked"] != true {
		t.Fatalf("loginLocked must surface from policy, got %v", d["loginLocked"])
	}
	cfg := res.loginCfg()
	if cfg["enabled"] != false || cfg["configStatus"] != "empty" {
		t.Fatalf("unconfigured instance must be empty placeholder, got %v", cfg)
	}
	tpl, _ := d["template"].(map[string]any)
	if tpl == nil || tpl["templateVersion"] != "v1" {
		t.Fatalf("GET must carry driving template, got %v", d["template"])
	}
	secrets, _ := tpl["secretFieldsJson"].([]any)
	if len(secrets) != 1 || secrets[0] != "appSecret" {
		t.Fatalf("template must declare secretFields=[appSecret], got %v", tpl["secretFieldsJson"])
	}
}

// S8：已配置的密文在读接口恒脱敏为 ******，绝不回明文/密文。
func TestGetLoginConfigMasksSecret(t *testing.T) {
	h := newLoginHarness(t)
	h.seedValid(t, "supersecret-plain")
	res := h.do(t, http.MethodGet, loginCfgPath1, h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	cfg := res.loginConfigJSON()
	if cfg["appSecret"] != "******" {
		t.Fatalf("appSecret must be masked on read, got %v", cfg["appSecret"])
	}
	if cfg["appId"] != "app-123_AZ" {
		t.Fatalf("non-secret field must be returned as-is, got %v", cfg["appId"])
	}
	if strings.Contains(res.raw, "supersecret-plain") || strings.Contains(res.raw, "enc:supersecret-plain") {
		t.Fatalf("read response must never leak plaintext/ciphertext: %s", res.raw)
	}
}

// 非 channel_only 实例（account_system）→ GET 400 VALIDATION_FAILED。
func TestGetLoginConfigRejectsNonChannelOnly(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodGet, loginCfgPath2, h.readToken(t), nil)
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q (%s)", res.errCode(), res.raw)
	}
}

func TestGetLoginConfigNotFound(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/game-channels/999/login-config", h.readToken(t), nil)
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", res.errCode())
	}
}

// 非法 int64 路径参数 → 400 VALIDATION_FAILED。
func TestGetLoginConfigBadID(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/game-channels/abc/login-config", h.readToken(t), nil)
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q", res.errCode())
	}
}

// ───────────────────────── PUT 成功 / 状态机 / 审计 / 脱敏（S1/S7/S8） ─────────────────────────

// S1：合法 PUT → 200，configStatus=valid，落库密文，回显脱敏；写审计。
func TestPutLoginConfigSuccessEncryptsAndAudits(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodPut, loginCfgPath1, h.writeToken(t), fullHuaweiConfig("supersecret-plain"))
	assertStatus(t, res, http.StatusOK)
	if res.loginCfg()["configStatus"] != "valid" {
		t.Fatalf("complete config must be valid, got %v", res.loginCfg())
	}
	// S8：响应脱敏，且不泄漏明文/密文。
	if res.loginConfigJSON()["appSecret"] != "******" {
		t.Fatalf("response secret must be masked, got %v", res.loginConfigJSON()["appSecret"])
	}
	if strings.Contains(res.raw, "supersecret-plain") {
		t.Fatalf("response must not leak plaintext: %s", res.raw)
	}
	// 落库为密文（"enc:"+明文），绝非明文。
	stored := h.store.state.configs[1]
	if stored.ConfigJSON["appSecret"] != "enc:supersecret-plain" {
		t.Fatalf("stored secret must be ciphertext, got %v", stored.ConfigJSON["appSecret"])
	}
	// S7：写一条 channel.login_config.update 审计；detail 不含明文/密文。
	e, ok := h.audit.byAction("channel.login_config.update")
	if !ok {
		t.Fatal("expected channel.login_config.update audit")
	}
	if e.ResourceType != "game_channel_login_config" || e.ResourceID != "1" {
		t.Fatalf("audit resource mismatch: type=%q id=%q", e.ResourceType, e.ResourceID)
	}
	db := mustJSON(t, e.Detail)
	if strings.Contains(db, "supersecret-plain") || strings.Contains(db, "enc:supersecret-plain") {
		t.Fatalf("audit detail leaked secret material: %s", db)
	}
}

// 复制实例（copiedFromMarket 非空）PUT 空配置 → 强制 invalid（绝不 empty）。
func TestPutLoginConfigCopiedEmptyForcesInvalid(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodPut, loginCfgPath4, h.writeToken(t), map[string]any{"enabled": false, "configJson": map[string]any{}})
	assertStatus(t, res, http.StatusOK)
	if res.loginCfg()["configStatus"] != "invalid" {
		t.Fatalf("copied instance empty PUT must be invalid, got %v", res.loginCfg())
	}
}

// ───────────────────────── PUT 校验失败（S4） + 失败不写审计 ─────────────────────────

// S4：缺必填密钥 → 400 VALIDATION_FAILED；落库 invalid 行内态；不写审计。
func TestPutLoginConfigMissingSecretInvalidNoAudit(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodPut, loginCfgPath1, h.writeToken(t), map[string]any{
		"enabled": true, "configJson": map[string]any{"appId": "app-1"},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q (%s)", res.errCode(), res.raw)
	}
	// 推荐落库 invalid（前端二次 GET 可见行内态）。
	if h.store.state.configs[1] == nil || h.store.state.configs[1].ConfigStatus != common.ConfigStatusInvalid {
		t.Fatalf("failed PUT should persist invalid row, got %v", h.store.state.configs[1])
	}
	// 红线：校验失败不写审计。
	if _, ok := h.audit.byAction("channel.login_config.update"); ok {
		t.Fatal("validation failure must NOT write audit")
	}
}

// S4：未知字段（不在 form_schema）→ 400 VALIDATION_FAILED。
func TestPutLoginConfigUnknownFieldRejected(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodPut, loginCfgPath1, h.writeToken(t), map[string]any{
		"enabled": true, "configJson": map[string]any{"appId": "app-1", "appSecret": "supersecret", "bogus": "x"},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q (%s)", res.errCode(), res.raw)
	}
}

// S4：validation_rules 未过（appId pattern）→ 400 VALIDATION_FAILED。
func TestPutLoginConfigRuleViolationRejected(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodPut, loginCfgPath1, h.writeToken(t), map[string]any{
		"enabled": true, "configJson": map[string]any{"appId": "bad id!", "appSecret": "supersecret"},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q (%s)", res.errCode(), res.raw)
	}
}

// S4：channel_only 但无模板 → 400 VALIDATION_FAILED（无模板拒绝）。
func TestPutLoginConfigTemplateMissingRejected(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodPut, loginCfgPath3, h.writeToken(t), map[string]any{
		"enabled": true, "configJson": map[string]any{"appId": "x"},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q (%s)", res.errCode(), res.raw)
	}
}

// 非 channel_only 实例 → PUT 400 VALIDATION_FAILED。
func TestPutLoginConfigRejectsNonChannelOnly(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodPut, loginCfgPath2, h.writeToken(t), fullHuaweiConfig("supersecret"))
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q", res.errCode())
	}
}

// ───────────────────────── 哨兵 ****** 逐字段处理（密文回归） ─────────────────────────

// 哨兵 ****** 有存量 → 保留原密文，不重新加密，状态保持 valid。
//
// 【疑似实现缺陷·当前 FAIL】channellogin/service.go 哨兵保留分支把字面 "******"
// 留在 logicalConfig 内参与模板校验；当该 secret 字段带 minLen/pattern 规则（如 seed
// huawei appSecret minLen:8）时，"******"(len=6) 触发 minLen 校验失败 → 误判 invalid/400，
// 导致带规则的密文配置无法在"未修改密钥"情况下二次保存。account-auth 的等价路径在校验前
// 用存量值替换哨兵从而规避（service.go encryptSecrets 先于 ValidateConfigAgainstTemplate）。
// 期望：保留存量密文且 config_status 维持 valid（不重新加密）。修复后本用例应转绿。
func TestPutLoginConfigSentinelKeepsCiphertext(t *testing.T) {
	h := newLoginHarness(t)
	h.seedValid(t, "supersecret-plain")
	callsAfterSeed := h.cipher.encryptCalls

	res := h.do(t, http.MethodPut, loginCfgPath1, h.writeToken(t), map[string]any{
		"enabled": true, "configJson": map[string]any{"appId": "app-123_AZ", "appSecret": "******"},
	})
	assertStatus(t, res, http.StatusOK)
	if res.loginCfg()["configStatus"] != "valid" {
		t.Fatalf("sentinel keep must stay valid, got %v", res.loginCfg())
	}
	if h.store.state.configs[1].ConfigJSON["appSecret"] != "enc:supersecret-plain" {
		t.Fatalf("sentinel must keep old ciphertext, got %v", h.store.state.configs[1].ConfigJSON["appSecret"])
	}
	if h.cipher.encryptCalls != callsAfterSeed {
		t.Fatalf("sentinel must not re-encrypt: %d -> %d", callsAfterSeed, h.cipher.encryptCalls)
	}
}

// 哨兵 ****** 无存量 → 视为未填 → invalid（避免 ****** 明文落库）。
func TestPutLoginConfigSentinelWithoutExistingIsInvalid(t *testing.T) {
	h := newLoginHarness(t)
	res := h.do(t, http.MethodPut, loginCfgPath1, h.writeToken(t), map[string]any{
		"enabled": true, "configJson": map[string]any{"appId": "app-123_AZ", "appSecret": "******"},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("sentinel without existing must be VALIDATION_FAILED, got %q", res.errCode())
	}
	// 红线：****** 绝不作为明文落库。
	if c := h.store.state.configs[1]; c != nil {
		if v, ok := c.ConfigJSON["appSecret"]; ok && v == "******" {
			t.Fatalf("sentinel must never persist as plaintext")
		}
	}
}

// 提交新密钥 → 重新加密落新密文。
func TestPutLoginConfigNewSecretReEncrypts(t *testing.T) {
	h := newLoginHarness(t)
	h.seedValid(t, "supersecret-plain")
	callsAfterSeed := h.cipher.encryptCalls

	res := h.do(t, http.MethodPut, loginCfgPath1, h.writeToken(t), fullHuaweiConfig("rotated-secret-x"))
	assertStatus(t, res, http.StatusOK)
	if h.store.state.configs[1].ConfigJSON["appSecret"] != "enc:rotated-secret-x" {
		t.Fatalf("new secret must be re-encrypted, got %v", h.store.state.configs[1].ConfigJSON["appSecret"])
	}
	if h.cipher.encryptCalls != callsAfterSeed+1 {
		t.Fatalf("new secret should encrypt exactly once more: %d -> %d", callsAfterSeed, h.cipher.encryptCalls)
	}
}

// ───────────────────────── S5 冲突 / S10 事务回滚 ─────────────────────────

// S5：唯一键 game_channel_id_ref 冲突 → 409 CONFLICT。
func TestPutLoginConfigConflict(t *testing.T) {
	h := newLoginHarness(t)
	h.store.state.failConflict = true
	res := h.do(t, http.MethodPut, loginCfgPath1, h.writeToken(t), fullHuaweiConfig("supersecret-plain"))
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q (%s)", res.errCode(), res.raw)
	}
}

// S10：Upsert 中途失败 → 整体回滚，无部分写入；不写审计。
func TestPutLoginConfigTransactionRollback(t *testing.T) {
	h := newLoginHarness(t)
	h.store.state.failUpsert = true
	res := h.do(t, http.MethodPut, loginCfgPath1, h.writeToken(t), fullHuaweiConfig("supersecret-plain"))
	if res.status != http.StatusInternalServerError {
		t.Fatalf("forced upsert failure should map to 500, got %d (%s)", res.status, res.raw)
	}
	if _, ok := h.store.state.configs[1]; ok {
		t.Fatalf("rolled-back write must not persist any row")
	}
	if _, ok := h.audit.byAction("channel.login_config.update"); ok {
		t.Fatal("rolled-back write must not write audit")
	}
}

// ───────────────────────── 鉴权与权限（S2/S3） ─────────────────────────

func TestLoginConfigAuthnRequired(t *testing.T) {
	h := newLoginHarness(t)
	endpoints := []struct {
		m, p string
		body any
	}{
		{http.MethodGet, loginCfgPath1, nil},
		{http.MethodPut, loginCfgPath1, fullHuaweiConfig("supersecret")},
	}
	for _, ep := range endpoints {
		// S2：无令牌 → 401。
		res := h.do(t, ep.m, ep.p, "", ep.body)
		assertStatus(t, res, http.StatusUnauthorized)
		if res.errCode() != "UNAUTHENTICATED" {
			t.Fatalf("%s %s want UNAUTHENTICATED got %q", ep.m, ep.p, res.errCode())
		}
	}
	// S2：伪造令牌 → 401。
	bad := h.do(t, http.MethodGet, loginCfgPath1, "not.a.valid.jwt", nil)
	assertStatus(t, bad, http.StatusUnauthorized)
}

func TestLoginConfigRBACForbidden(t *testing.T) {
	h := newLoginHarness(t)
	// 仅 channel.read：可读，写 → 403。
	readOnly := h.readToken(t)
	assertStatus(t, h.do(t, http.MethodGet, loginCfgPath1, readOnly, nil), http.StatusOK)
	put := h.do(t, http.MethodPut, loginCfgPath1, readOnly, fullHuaweiConfig("supersecret"))
	assertStatus(t, put, http.StatusForbidden)
	if put.errCode() != "FORBIDDEN" {
		t.Fatalf("read-only PUT want FORBIDDEN got %q", put.errCode())
	}
	// 无 channel.read（其它权限）→ 读 403。
	noPerm := h.token(t, 32, []string{"system.read"})
	assertStatus(t, h.do(t, http.MethodGet, loginCfgPath1, noPerm, nil), http.StatusForbidden)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
