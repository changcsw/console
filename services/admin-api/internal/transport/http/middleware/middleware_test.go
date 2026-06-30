package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// L1 单元（无 IO）：覆盖审计中间件的兜底纯逻辑——写方法识别、action/resource 推断、
// resourceID 截断 128、resource_type 单数化、clientIP 解析。
// 规则来源 docs/architecture/v2/modules/22-audit/spec.compact.md §2.4 与 CR 小修。

func TestIsWriteMethod(t *testing.T) {
	write := []string{"POST", "PUT", "PATCH", "DELETE"}
	for _, m := range write {
		if !isWriteMethod(m) {
			t.Errorf("%s should be a write method", m)
		}
	}
	read := []string{"GET", "HEAD", "OPTIONS"}
	for _, m := range read {
		if isWriteMethod(m) {
			t.Errorf("%s should NOT be a write method", m)
		}
	}
}

func TestMethodToAction(t *testing.T) {
	cases := map[string]string{"POST": "create", "DELETE": "delete", "PUT": "update", "PATCH": "update"}
	for m, want := range cases {
		if got := methodToAction(m); got != want {
			t.Errorf("methodToAction(%s)=%s want %s", m, got, want)
		}
	}
}

func TestSingular(t *testing.T) {
	cases := map[string]string{
		"games":    "game",
		"roles":    "role",
		"policies": "policy",
		"game":     "game",
		"status":   "statu", // 朴素去 s（已知近似，记录于用例）
	}
	for in, want := range cases {
		if got := singular(in); got != want {
			t.Errorf("singular(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNormalizeSegment(t *testing.T) {
	if got := normalizeSegment(" Game-Channels "); got != "game_channels" {
		t.Fatalf("normalizeSegment=%q", got)
	}
}

func TestInferFallbackAudit_VerbSuffix(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/admin/games/123/publish", nil)
	action, resourceType, _ := inferFallbackAudit(r)
	if resourceType != "game" {
		t.Errorf("resourceType=%q want game", resourceType)
	}
	if action != "game.publish" {
		t.Errorf("action=%q want game.publish", action)
	}
}

func TestInferFallbackAudit_MethodFallbackAction(t *testing.T) {
	r := httptest.NewRequest("DELETE", "/api/admin/roles/5", nil)
	action, resourceType, _ := inferFallbackAudit(r)
	if resourceType != "role" {
		t.Errorf("resourceType=%q want role", resourceType)
	}
	if action != "role.delete" {
		t.Errorf("action=%q want role.delete", action)
	}
}

func TestInferFallbackAudit_ResourceIDFromQuery(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/admin/games?id=g_777", nil)
	_, _, resourceID := inferFallbackAudit(r)
	if resourceID != "g_777" {
		t.Fatalf("resourceID=%q want g_777", resourceID)
	}
}

func TestInferFallbackAudit_ResourceIDTruncatedTo128(t *testing.T) {
	longID := strings.Repeat("a", 200)
	r := httptest.NewRequest("POST", "/api/admin/games?id="+longID, nil)
	_, _, resourceID := inferFallbackAudit(r)
	if len(resourceID) != 128 {
		t.Fatalf("resourceID len=%d want 128 (防 INSERT 失败)", len(resourceID))
	}
}

func TestInferFallbackAudit_EmptyPathFallback(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/admin", nil)
	action, resourceType, resourceID := inferFallbackAudit(r)
	if resourceType != "admin" {
		t.Errorf("resourceType=%q want admin", resourceType)
	}
	if action != "admin.create" {
		t.Errorf("action=%q want admin.create", action)
	}
	if resourceID != "unknown" {
		t.Errorf("resourceID=%q want unknown", resourceID)
	}
}

func TestClientIP_XForwardedForFirst(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Fatalf("clientIP=%q want 203.0.113.7", got)
	}
}

func TestClientIP_RemoteAddrFallback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.5:54321"
	if got := clientIP(r); got != "192.0.2.5" {
		t.Fatalf("clientIP=%q want 192.0.2.5", got)
	}
}

func TestBearerToken(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer abc.def.ghi")
	if got := bearerToken(r); got != "abc.def.ghi" {
		t.Fatalf("bearerToken=%q", got)
	}
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Authorization", "Basic xxx")
	if got := bearerToken(r2); got != "" {
		t.Fatalf("non-bearer should yield empty, got %q", got)
	}
}
