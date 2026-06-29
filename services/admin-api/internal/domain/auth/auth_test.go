package auth

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestAuthContextHasPermission(t *testing.T) {
	ac := NewAuthContext(1, "alice", "Alice",
		[]string{"editor"},
		[]string{"game.read", "game.write"},
		common.EnvDevelop)

	if !ac.HasPermission("game.read") {
		t.Fatal("should have game.read")
	}
	if ac.HasPermission("sync.execute") {
		t.Fatal("should not have sync.execute")
	}
	if !ac.HasPermission("") {
		t.Fatal("empty code means no requirement -> allow")
	}
	if !ac.HasAnyPermission("x.y", "game.write") {
		t.Fatal("should match game.write")
	}
	if ac.HasAnyPermission("x.y", "z.w") {
		t.Fatal("should not match")
	}
}

func TestAuthContextSuperAdmin(t *testing.T) {
	ac := NewAuthContext(1, "root", "Root",
		[]string{SuperAdminRole},
		nil,
		common.EnvProduction)

	if !ac.IsSuperAdmin() {
		t.Fatal("should be super admin")
	}
	if !ac.HasPermission("anything.dangerous") {
		t.Fatal("super admin short-circuits all permissions")
	}
}

func TestAuthContextPermissionsCopy(t *testing.T) {
	ac := NewAuthContext(1, "a", "A", nil, []string{"a.b"}, common.EnvSandbox)
	perms := ac.Permissions()
	if len(perms) != 1 || perms[0] != "a.b" {
		t.Fatalf("unexpected perms: %v", perms)
	}
}

func TestAuthContextNoRolesNoPerms(t *testing.T) {
	// 无角色 → 空权限：除空码外一律拒绝（不变量4 反向）。
	ac := NewAuthContext(7, "noperm", "NoPerm", nil, nil, common.EnvProduction)
	if ac.IsSuperAdmin() {
		t.Fatal("no roles must not be super admin")
	}
	if ac.HasPermission("system.read") {
		t.Fatal("empty perms must deny concrete code")
	}
	if ac.HasAnyPermission("a.b", "c.d") {
		t.Fatal("empty perms must deny any concrete code")
	}
	if len(ac.Permissions()) != 0 {
		t.Fatalf("expected zero perms, got %v", ac.Permissions())
	}
}

func TestAuthContextSuperAdminWithoutExplicitPerms(t *testing.T) {
	// super_admin 即使 perms 为空也短路放行任意权限码（中间件短路语义）。
	ac := NewAuthContext(1, "root", "Root", []string{"viewer", SuperAdminRole}, nil, common.EnvDevelop)
	if !ac.HasPermission("permission.write") {
		t.Fatal("super admin must short-circuit any permission")
	}
	if !ac.HasAnyPermission("nope.nope") {
		t.Fatal("super admin must short-circuit HasAnyPermission")
	}
}

func TestAuthContextHasAnyPermissionEmptyArgs(t *testing.T) {
	ac := NewAuthContext(1, "a", "A", nil, []string{"x.y"}, common.EnvDevelop)
	if ac.HasAnyPermission() {
		t.Fatal("no codes provided must be false for non-super-admin")
	}
}

func TestAuthContextDuplicatePermsDeduped(t *testing.T) {
	ac := NewAuthContext(1, "a", "A", nil, []string{"x.y", "x.y", "z.w"}, common.EnvDevelop)
	if len(ac.Permissions()) != 2 {
		t.Fatalf("perms set must dedupe, got %v", ac.Permissions())
	}
}
