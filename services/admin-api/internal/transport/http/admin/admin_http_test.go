package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	domainadmin "github.com/csw/console/services/admin-api/internal/domain/admin"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/infra/crypto"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
)

const testEnv = common.EnvDevelop

type harness struct {
	router http.Handler
	store  *memStore
	issuer *infrajwt.Issuer
	hasher crypto.PasswordHasher
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
	hasher := crypto.NewPasswordHasher(4) // 低 cost 提速
	audit := &fakeAudit{}

	authSvc := adminapp.NewAdminAuthService(adminapp.AuthDeps{
		Tx: store, Hasher: hasher, Issuer: issuer, Feishu: fakeFeishu{}, Cipher: nil, Audit: audit, Env: testEnv,
	})
	userSvc := adminapp.NewAdminUserService(store, hasher, audit)
	roleSvc := adminapp.NewRoleService(store, audit)
	permSvc := adminapp.NewPermissionService(store, audit)

	handler := NewHandler(Deps{Auth: authSvc, Users: userSvc, Roles: roleSvc, Perms: permSvc, Env: testEnv})
	sub := NewRouter(handler, issuer, testEnv, slog.New(slog.NewTextHandler(io.Discard, nil)), true)

	root := chi.NewRouter()
	root.Mount("/api/admin", sub)

	return &harness{router: root, store: store, issuer: issuer, hasher: hasher, audit: audit}
}

// seedUser 直接在内存态种入一个用户（可选带 password 身份），返回其 ID。
func (h *harness) seedUser(t *testing.T, userName, password string, status common.AdminUserStatus) int64 {
	t.Helper()
	repos := h.store.Repositories()
	u := &domainadmin.AdminUser{UserName: userName, DisplayName: userName, Status: status}
	if err := repos.Users.Create(context.Background(), u); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if password != "" {
		hash, err := h.hasher.Hash(password)
		if err != nil {
			t.Fatalf("hash: %v", err)
		}
		if err := repos.Identities.Upsert(context.Background(), &domainadmin.AdminIdentity{
			UserIDRef: u.ID, IdentityType: common.IdentityTypePassword, IdentityKey: userName, CredentialCiphertext: hash,
		}); err != nil {
			t.Fatalf("seed identity: %v", err)
		}
	}
	return u.ID
}

// seedRole 种入一个角色（可附权限码）。
func (h *harness) seedRole(t *testing.T, roleCode string, permCodes ...string) int64 {
	t.Helper()
	repos := h.store.Repositories()
	role := &domainadmin.Role{RoleCode: roleCode, RoleName: roleCode}
	if err := repos.Roles.Create(context.Background(), role); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	permIDs := []int64{}
	for _, code := range permCodes {
		p := &domainadmin.Permission{PermissionCode: code, PermissionName: code}
		if err := repos.Permissions.Create(context.Background(), p); err != nil {
			// 已存在则复用
			list, _, _ := repos.Permissions.List(context.Background(), domainadmin.PermissionFilter{All: true})
			for i := range list {
				if list[i].PermissionCode == code {
					p.ID = list[i].ID
				}
			}
		}
		permIDs = append(permIDs, p.ID)
	}
	if len(permIDs) > 0 {
		_ = repos.Roles.ReplacePermissions(context.Background(), role.ID, permIDs)
	}
	return role.ID
}

func (h *harness) token(t *testing.T, userID int64, roles, perms []string) string {
	t.Helper()
	pair, err := h.issuer.IssuePair(domainauth.NewAuthContext(userID, "tester", "Tester", roles, perms, testEnv))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return pair.AccessToken
}

func (h *harness) superToken(t *testing.T) string {
	return h.token(t, 1, []string{domainauth.SuperAdminRole}, nil)
}

type apiResp struct {
	status int
	body   map[string]any
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
	out := apiResp{status: rec.Code}
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
		t.Fatalf("status: want %d got %d (body=%v)", want, got.status, got.body)
	}
}

// ───────────────────────── 鉴权 / 登录 ─────────────────────────

func TestLoginSuccess(t *testing.T) {
	h := newHarness(t)
	h.seedUser(t, "admin", "Admin@12345", common.AdminUserStatusActive)

	res := h.do(t, http.MethodPost, "/api/admin/auth/login", "", map[string]any{
		"userName": "admin", "password": "Admin@12345",
	})
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	if d["accessToken"] == "" || d["refreshToken"] == "" {
		t.Fatalf("expected tokens, got %v", d)
	}
	user, _ := d["user"].(map[string]any)
	if user["userName"] != "admin" {
		t.Fatalf("expected user.userName=admin, got %v", user)
	}
	// S8: 响应绝不回明文密码或哈希
	raw, _ := json.Marshal(res.body)
	if strings.Contains(string(raw), "Admin@12345") || strings.Contains(string(raw), "$2a$") {
		t.Fatal("login response leaked password or bcrypt hash")
	}
	// S7: 写审计 admin.login，且 detail 不含密码
	e, ok := h.audit.byAction("admin.login")
	if !ok {
		t.Fatal("expected admin.login audit entry")
	}
	db, _ := json.Marshal(e.Detail)
	if strings.Contains(string(db), "Admin@12345") {
		t.Fatal("audit detail leaked password")
	}
}

func TestLoginWrongPasswordEnumerationGuard(t *testing.T) {
	h := newHarness(t)
	h.seedUser(t, "admin", "Admin@12345", common.AdminUserStatusActive)

	wrong := h.do(t, http.MethodPost, "/api/admin/auth/login", "", map[string]any{"userName": "admin", "password": "nope"})
	assertStatus(t, wrong, http.StatusUnauthorized)
	if wrong.errCode() != "UNAUTHENTICATED" {
		t.Fatalf("want UNAUTHENTICATED, got %q", wrong.errCode())
	}
	ghost := h.do(t, http.MethodPost, "/api/admin/auth/login", "", map[string]any{"userName": "ghost", "password": "nope"})
	assertStatus(t, ghost, http.StatusUnauthorized)
	if ghost.errCode() != "UNAUTHENTICATED" {
		t.Fatalf("unknown user must also be UNAUTHENTICATED, got %q", ghost.errCode())
	}
}

func TestLoginDisabledRejected(t *testing.T) {
	h := newHarness(t)
	h.seedUser(t, "ghost", "Admin@12345", common.AdminUserStatusDisabled)
	res := h.do(t, http.MethodPost, "/api/admin/auth/login", "", map[string]any{"userName": "ghost", "password": "Admin@12345"})
	assertStatus(t, res, http.StatusUnauthorized)
}

func TestLoginValidation(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/admin/auth/login", "", map[string]any{"userName": ""})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED, got %q", res.errCode())
	}
}

func TestRefreshSuccessAndDisabledRejected(t *testing.T) {
	h := newHarness(t)
	uid := h.seedUser(t, "admin", "Admin@12345", common.AdminUserStatusActive)

	login := h.do(t, http.MethodPost, "/api/admin/auth/login", "", map[string]any{"userName": "admin", "password": "Admin@12345"})
	assertStatus(t, login, http.StatusOK)
	refreshTok, _ := login.data()["refreshToken"].(string)

	ok := h.do(t, http.MethodPost, "/api/admin/auth/refresh", "", map[string]any{"refreshToken": refreshTok})
	assertStatus(t, ok, http.StatusOK)
	if ok.data()["accessToken"] == "" {
		t.Fatal("refresh must return new access token")
	}

	// 禁用即拒绝：账号被禁后 refresh 必须回库重校验 status → 401
	u, _ := h.store.Repositories().Users.FindByID(context.Background(), uid)
	u.Status = common.AdminUserStatusDisabled
	_ = h.store.Repositories().Users.Update(context.Background(), u)

	denied := h.do(t, http.MethodPost, "/api/admin/auth/refresh", "", map[string]any{"refreshToken": refreshTok})
	assertStatus(t, denied, http.StatusUnauthorized)
}

func TestFeishuCallbackUnboundRejected(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPost, "/api/admin/auth/feishu/callback", "", map[string]any{"code": "mock:unbound"})
	assertStatus(t, res, http.StatusUnauthorized)
}

func TestFeishuCallbackBoundSuccess(t *testing.T) {
	h := newHarness(t)
	uid := h.seedUser(t, "alice", "", common.AdminUserStatusActive)
	_ = h.store.Repositories().Identities.Upsert(context.Background(), &domainadmin.AdminIdentity{
		UserIDRef: uid, IdentityType: common.IdentityTypeFeishu, IdentityKey: "uni-alice",
	})
	res := h.do(t, http.MethodPost, "/api/admin/auth/feishu/callback", "", map[string]any{"code": "mock:uni-alice"})
	assertStatus(t, res, http.StatusOK)
	if res.data()["accessToken"] == "" {
		t.Fatal("expected token for bound feishu identity")
	}
}

// ───────────────────────── /me ─────────────────────────

func TestMeRequiresAuth(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/me", "", nil)
	assertStatus(t, res, http.StatusUnauthorized)

	bad := h.do(t, http.MethodGet, "/api/admin/me", "not.a.jwt", nil)
	assertStatus(t, bad, http.StatusUnauthorized)
}

func TestMeMasksIdentityKeyAndEchoesEnv(t *testing.T) {
	h := newHarness(t)
	uid := h.seedUser(t, "alice", "", common.AdminUserStatusActive)
	_ = h.store.Repositories().Identities.Upsert(context.Background(), &domainadmin.AdminIdentity{
		UserIDRef: uid, IdentityType: common.IdentityTypeFeishu, IdentityKey: "on_1234567890abcd",
	})
	tok := h.token(t, uid, []string{"viewer"}, nil)
	res := h.do(t, http.MethodGet, "/api/admin/me", tok, nil)
	assertStatus(t, res, http.StatusOK)
	if res.data()["environment"] != string(testEnv) {
		t.Fatalf("environment mismatch: %v", res.data()["environment"])
	}
	ids, _ := res.data()["identities"].([]any)
	if len(ids) != 1 {
		t.Fatalf("expected 1 identity, got %v", ids)
	}
	first, _ := ids[0].(map[string]any)
	if key, _ := first["identityKey"].(string); key == "on_1234567890abcd" || !strings.Contains(key, "****") {
		t.Fatalf("identityKey must be masked, got %q", key)
	}
}

// ───────────────────────── RBAC（S2/S3） ─────────────────────────

func TestListUsersRBAC(t *testing.T) {
	h := newHarness(t)

	// S2 无令牌 → 401
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/system/admin-users", "", nil), http.StatusUnauthorized)

	// S3 登录但缺 system.read → 403
	noPerm := h.token(t, 2, []string{"viewer"}, []string{"game.read"})
	forbidden := h.do(t, http.MethodGet, "/api/admin/system/admin-users", noPerm, nil)
	assertStatus(t, forbidden, http.StatusForbidden)
	if forbidden.errCode() != "FORBIDDEN" {
		t.Fatalf("want FORBIDDEN, got %q", forbidden.errCode())
	}

	// S1 super_admin → 200
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/system/admin-users", h.superToken(t), nil), http.StatusOK)

	// system.read 精确权限 → 200
	reader := h.token(t, 3, []string{"reader"}, []string{"system.read"})
	assertStatus(t, h.do(t, http.MethodGet, "/api/admin/system/admin-users", reader, nil), http.StatusOK)
}

func TestCreateUserForbiddenWithoutWritePerm(t *testing.T) {
	h := newHarness(t)
	reader := h.token(t, 3, []string{"reader"}, []string{"system.read"})
	res := h.do(t, http.MethodPost, "/api/admin/system/admin-users", reader, map[string]any{"userName": "x", "displayName": "X"})
	assertStatus(t, res, http.StatusForbidden)
}

// ───────────────────────── 管理员 CRUD（S1/S4/S5/S7/S10） ─────────────────────────

func TestCreateUserSuccessAndConflictAndAudit(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)

	ok := h.do(t, http.MethodPost, "/api/admin/system/admin-users", tok, map[string]any{
		"userName": "alice", "displayName": "Alice", "password": "Alice@12345",
	})
	assertStatus(t, ok, http.StatusCreated)
	if ok.data()["userName"] != "alice" {
		t.Fatalf("expected alice, got %v", ok.data())
	}
	if _, found := h.audit.byAction("admin_user.create"); !found {
		t.Fatal("expected admin_user.create audit")
	}
	// S8: 创建响应不回明文密码/哈希
	raw, _ := json.Marshal(ok.body)
	if strings.Contains(string(raw), "Alice@12345") || strings.Contains(string(raw), "$2a$") {
		t.Fatal("create user response leaked password/hash")
	}

	// S5: 重名冲突
	dup := h.do(t, http.MethodPost, "/api/admin/system/admin-users", tok, map[string]any{"userName": "alice", "displayName": "Dup"})
	assertStatus(t, dup, http.StatusConflict)
	if dup.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT, got %q", dup.errCode())
	}
}

func TestCreateUserValidation(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)
	// 缺必填
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/system/admin-users", tok, map[string]any{"displayName": ""}), http.StatusBadRequest)
	// 弱口令 <8
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/system/admin-users", tok, map[string]any{"userName": "p", "displayName": "P", "password": "short"}), http.StatusBadRequest)
	// 邮箱格式非法
	assertStatus(t, h.do(t, http.MethodPost, "/api/admin/system/admin-users", tok, map[string]any{"userName": "e", "displayName": "E", "email": "not-an-email"}), http.StatusBadRequest)
}

func TestCreateUserTransactionRollback(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)
	res := h.do(t, http.MethodPost, "/api/admin/system/admin-users", tok, map[string]any{
		"userName": "dave", "displayName": "Dave", "roleIds": []int64{999999},
	})
	assertStatus(t, res, http.StatusBadRequest)
	// 回滚断言：用户未落库
	if _, err := h.store.Repositories().Users.FindByUserName(context.Background(), "dave"); err != adminapp.ErrNotFound {
		t.Fatalf("transaction must roll back; dave should not exist, err=%v", err)
	}
}

func TestUpdateUserDisableAndInvalidStatus(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)
	uid := h.seedUser(t, "u1", "", common.AdminUserStatusActive)

	ok := h.do(t, http.MethodPatch, "/api/admin/system/admin-users/"+itoa(uid), tok, map[string]any{"status": "disabled"})
	assertStatus(t, ok, http.StatusOK)
	if ok.data()["status"] != "disabled" {
		t.Fatalf("expected disabled, got %v", ok.data()["status"])
	}
	if e, found := h.audit.byAction("admin_user.update"); !found || e.Detail["statusBefore"] != "active" || e.Detail["statusAfter"] != "disabled" {
		t.Fatalf("audit before/after mismatch: %+v found=%v", e.Detail, found)
	}

	bad := h.do(t, http.MethodPatch, "/api/admin/system/admin-users/"+itoa(uid), tok, map[string]any{"status": "bogus"})
	assertStatus(t, bad, http.StatusBadRequest)
}

func TestListUsersInvalidStatusQuery(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, "/api/admin/system/admin-users?status=bogus", h.superToken(t), nil)
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED, got %q", res.errCode())
	}
}

func TestAssignRolesSuccessAndRollback(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)
	uid := h.seedUser(t, "u1", "", common.AdminUserStatusActive)
	rid := h.seedRole(t, "ops", "system.read")

	ok := h.do(t, http.MethodPut, "/api/admin/system/admin-users/"+itoa(uid)+"/roles", tok, map[string]any{"roleIds": []int64{rid}})
	assertStatus(t, ok, http.StatusOK)
	if _, found := h.audit.byAction("admin_user.assign_roles"); !found {
		t.Fatal("expected assign_roles audit")
	}

	// 必填校验
	assertStatus(t, h.do(t, http.MethodPut, "/api/admin/system/admin-users/"+itoa(uid)+"/roles", tok, map[string]any{}), http.StatusBadRequest)

	// 回滚：不存在的角色
	bad := h.do(t, http.MethodPut, "/api/admin/system/admin-users/"+itoa(uid)+"/roles", tok, map[string]any{"roleIds": []int64{999999}})
	assertStatus(t, bad, http.StatusBadRequest)
	roles, _ := h.store.Repositories().Users.RolesByUser(context.Background(), uid)
	if len(roles) != 1 || roles[0].ID != rid {
		t.Fatalf("roles must remain unchanged after rollback, got %v", roles)
	}
}

func TestResetPasswordNoPlaintextLeak(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)
	uid := h.seedUser(t, "u1", "Old@123456", common.AdminUserStatusActive)

	short := h.do(t, http.MethodPost, "/api/admin/system/admin-users/"+itoa(uid)+"/reset-password", tok, map[string]any{"newPassword": "short"})
	assertStatus(t, short, http.StatusBadRequest)

	ok := h.do(t, http.MethodPost, "/api/admin/system/admin-users/"+itoa(uid)+"/reset-password", tok, map[string]any{"newPassword": "Newpass@123"})
	assertStatus(t, ok, http.StatusOK)
	if ok.data()["reset"] != true {
		t.Fatalf("expected reset:true, got %v", ok.data())
	}
	e, found := h.audit.byAction("admin_user.reset_password")
	if !found {
		t.Fatal("expected reset_password audit")
	}
	db, _ := json.Marshal(e.Detail)
	if strings.Contains(string(db), "Newpass@123") {
		t.Fatal("audit detail leaked new password")
	}
	// 新密码可登录（明文落库为 bcrypt）
	login := h.do(t, http.MethodPost, "/api/admin/auth/login", "", map[string]any{"userName": "u1", "password": "Newpass@123"})
	assertStatus(t, login, http.StatusOK)
}

// ───────────────────────── 角色（S1/S5/S7） ─────────────────────────

func TestRoleCreateConflictAndDelete(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)

	ok := h.do(t, http.MethodPost, "/api/admin/system/roles", tok, map[string]any{"roleCode": "ops", "roleName": "Ops"})
	assertStatus(t, ok, http.StatusCreated)
	rid := int64(ok.data()["id"].(float64))

	dup := h.do(t, http.MethodPost, "/api/admin/system/roles", tok, map[string]any{"roleCode": "ops", "roleName": "Dup"})
	assertStatus(t, dup, http.StatusConflict)

	// 未被引用 → 可删
	del := h.do(t, http.MethodDelete, "/api/admin/system/roles/"+itoa(rid), tok, nil)
	assertStatus(t, del, http.StatusOK)
	if del.data()["deleted"] != true {
		t.Fatalf("expected deleted:true, got %v", del.data())
	}
}

func TestRoleDeleteConflictWhenReferenced(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)
	uid := h.seedUser(t, "u1", "", common.AdminUserStatusActive)
	rid := h.seedRole(t, "ops")
	_ = h.store.Repositories().Users.ReplaceRoles(context.Background(), uid, []int64{rid})

	del := h.do(t, http.MethodDelete, "/api/admin/system/roles/"+itoa(rid), tok, nil)
	assertStatus(t, del, http.StatusConflict)
	if del.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT, got %q", del.errCode())
	}
}

func TestAssignRolePermissionsSuccessAndRollback(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)
	rid := h.seedRole(t, "ops")
	repos := h.store.Repositories()
	p := &domainadmin.Permission{PermissionCode: "game.read", PermissionName: "g"}
	_ = repos.Permissions.Create(context.Background(), p)

	ok := h.do(t, http.MethodPut, "/api/admin/system/roles/"+itoa(rid)+"/permissions", tok, map[string]any{"permissionIds": []int64{p.ID}})
	assertStatus(t, ok, http.StatusOK)

	bad := h.do(t, http.MethodPut, "/api/admin/system/roles/"+itoa(rid)+"/permissions", tok, map[string]any{"permissionIds": []int64{999999}})
	assertStatus(t, bad, http.StatusBadRequest)
	perms, _ := h.store.Repositories().Roles.PermissionsByRole(context.Background(), rid)
	if len(perms) != 1 || perms[0].ID != p.ID {
		t.Fatalf("permissions must remain unchanged after rollback, got %v", perms)
	}
}

// ───────────────────────── 权限码（S1/S4/S5） ─────────────────────────

func TestPermissionCreateValidationConflictAndDelete(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)

	// 格式非法
	bad := h.do(t, http.MethodPost, "/api/admin/system/permissions", tok, map[string]any{"permissionCode": "Bad-Code", "permissionName": "Bad"})
	assertStatus(t, bad, http.StatusBadRequest)

	ok := h.do(t, http.MethodPost, "/api/admin/system/permissions", tok, map[string]any{"permissionCode": "demo.read", "permissionName": "Demo"})
	assertStatus(t, ok, http.StatusCreated)
	pid := int64(ok.data()["id"].(float64))

	dup := h.do(t, http.MethodPost, "/api/admin/system/permissions", tok, map[string]any{"permissionCode": "demo.read", "permissionName": "Dup"})
	assertStatus(t, dup, http.StatusConflict)

	del := h.do(t, http.MethodDelete, "/api/admin/system/permissions/"+itoa(pid), tok, nil)
	assertStatus(t, del, http.StatusOK)
}

func TestPermissionDeleteConflictWhenReferenced(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)
	rid := h.seedRole(t, "ops")
	repos := h.store.Repositories()
	p := &domainadmin.Permission{PermissionCode: "demo.read", PermissionName: "d"}
	_ = repos.Permissions.Create(context.Background(), p)
	_ = repos.Roles.ReplacePermissions(context.Background(), rid, []int64{p.ID})

	del := h.do(t, http.MethodDelete, "/api/admin/system/permissions/"+itoa(p.ID), tok, nil)
	assertStatus(t, del, http.StatusConflict)
}

// ───────────────────────── 分页（S9） ─────────────────────────

func TestListPageSizeClamped(t *testing.T) {
	h := newHarness(t)
	tok := h.superToken(t)
	for i := 0; i < 3; i++ {
		h.seedRole(t, "role"+itoa(int64(i)))
	}
	res := h.do(t, http.MethodGet, "/api/admin/system/roles?pageSize=99999", tok, nil)
	assertStatus(t, res, http.StatusOK)
	if got := res.data()["pageSize"]; got != float64(100) {
		t.Fatalf("pageSize must clamp to 100, got %v", got)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
