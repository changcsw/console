package audit

import (
	"net/http/httptest"
	"testing"

	auditapp "github.com/csw/console/services/admin-api/internal/app/audit"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// L1 单元（无 IO）：覆盖审计读侧 transport 纯逻辑——operator 系统占位、query 解析与校验。
// 规则来源 docs/architecture/v2/modules/22-audit/spec.compact.md §6.1/§7.2。

func TestToOperator_SystemPlaceholderForActorZero(t *testing.T) {
	item := auditapp.AuditLogItem{AuditLog: common.AuditLog{ActorID: 0}}
	got := toOperator(item).(map[string]any)
	if got["id"] != "0" || got["userName"] != "system" || got["displayName"] != "System" {
		t.Fatalf("actor=0 should yield system placeholder, got %v", got)
	}
}

func TestToOperator_NilWhenDeletedUser(t *testing.T) {
	item := auditapp.AuditLogItem{AuditLog: common.AuditLog{ActorID: 99}, Operator: nil}
	if got := toOperator(item); got != nil {
		t.Fatalf("deleted/unknown operator should be null, got %v", got)
	}
}

func TestToOperator_ExpandsJoinedUser(t *testing.T) {
	item := auditapp.AuditLogItem{
		AuditLog: common.AuditLog{ActorID: 12},
		Operator: &auditapp.Operator{ID: 12, UserName: "alice", DisplayName: "Alice Z"},
	}
	got := toOperator(item).(map[string]any)
	if got["id"] != "12" || got["userName"] != "alice" || got["displayName"] != "Alice Z" {
		t.Fatalf("operator expansion wrong: %v", got)
	}
}

func TestParseQuery_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/admin/audit-logs", nil)
	q, err := parseQuery(r)
	if err != nil {
		t.Fatalf("parseQuery: %v", err)
	}
	if q.Page != 1 || q.PageSize != 20 {
		t.Fatalf("defaults page=%d size=%d want 1/20", q.Page, q.PageSize)
	}
	if !q.SortDesc {
		t.Fatalf("default sort should be desc (-createdAt)")
	}
}

func TestParseQuery_SortAscending(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/admin/audit-logs?sort=createdAt", nil)
	q, _ := parseQuery(r)
	if q.SortDesc {
		t.Fatalf("sort=createdAt should be ascending")
	}
}

func TestParseQuery_UnknownSortFallsBackToDesc(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/admin/audit-logs?sort=bogus", nil)
	q, _ := parseQuery(r)
	if !q.SortDesc {
		t.Fatalf("unknown sort should fall back to desc")
	}
}

func TestParseQuery_FiltersParsed(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/admin/audit-logs?env=production&action=sync.execute&resourceType=sync_job&resourceId=g1&operator=12&keyword=foo", nil)
	q, err := parseQuery(r)
	if err != nil {
		t.Fatalf("parseQuery: %v", err)
	}
	if q.Env == nil || *q.Env != common.EnvProduction {
		t.Errorf("env not parsed: %v", q.Env)
	}
	if q.Action == nil || *q.Action != "sync.execute" {
		t.Errorf("action not parsed: %v", q.Action)
	}
	if q.Operator == nil || *q.Operator != 12 {
		t.Errorf("operator not parsed: %v", q.Operator)
	}
	if q.Keyword == nil || *q.Keyword != "foo" {
		t.Errorf("keyword not parsed: %v", q.Keyword)
	}
}

func TestParseQuery_InvalidOperator(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/admin/audit-logs?operator=abc", nil)
	if _, err := parseQuery(r); err == nil {
		t.Fatalf("expected error for non-integer operator")
	}
}

func TestParseQuery_NegativeOperatorRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/admin/audit-logs?operator=-3", nil)
	if _, err := parseQuery(r); err == nil {
		t.Fatalf("expected error for negative operator")
	}
}

func TestParseQuery_FromAfterToRejected(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/admin/audit-logs?from=2026-06-02T00:00:00Z&to=2026-06-01T00:00:00Z", nil)
	if _, err := parseQuery(r); err == nil {
		t.Fatalf("expected error for from>to")
	}
}

func TestParseQuery_BadTimeFormat(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/admin/audit-logs?from=not-a-time", nil)
	if _, err := parseQuery(r); err == nil {
		t.Fatalf("expected error for bad from time format")
	}
}

func TestParseQuery_PageSizeClampHandledByService(t *testing.T) {
	// parseQuery 透传原始 pageSize（>100），由 service.normalizePage 钳制。
	r := httptest.NewRequest("GET", "/api/admin/audit-logs?pageSize=500", nil)
	q, _ := parseQuery(r)
	if q.PageSize != 500 {
		t.Fatalf("parseQuery should pass through raw pageSize, got %d", q.PageSize)
	}
}
