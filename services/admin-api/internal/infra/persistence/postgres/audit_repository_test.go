package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	auditapp "github.com/csw/console/services/admin-api/internal/app/audit"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// L1/仓储纯逻辑（无真实 IO，使用 fake DBTX）：覆盖 WHERE 组装、参数序、
// 排序（created_at DESC + id DESC）、分页 LIMIT/OFFSET 以及 Insert 的 detail 默认值。
// 真正连库的命中/事务断言归入 scenario(requiresDB) + 连库 harness。

var errCapture = errors.New("captured")

// fakeRow 实现 pgx.Row，count 查询恒返回给定 total。
type fakeRow struct{ total int64 }

func (r fakeRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if p, ok := dest[0].(*int64); ok {
			*p = r.total
		}
	}
	return nil
}

// fakeDBTX 捕获 Exec/Query 的 SQL 与参数，不连接真实数据库。
type fakeDBTX struct {
	execSQL  string
	execArgs []any

	querySQL  string
	queryArgs []any
	total     int64
}

func (f *fakeDBTX) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execSQL = sql
	f.execArgs = args
	return pgconn.CommandTag{}, nil
}

func (f *fakeDBTX) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.querySQL = sql
	f.queryArgs = args
	return nil, errCapture // 捕获后中断，避免实现 pgx.Rows
}

func (f *fakeDBTX) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return fakeRow{total: f.total}
}

// ───────────────────────── buildAuditWhere ─────────────────────────

func TestBuildAuditWhere_Empty(t *testing.T) {
	where, args := buildAuditWhere(auditapp.AuditQuery{})
	if where != "" {
		t.Fatalf("empty query should produce no WHERE, got %q", where)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got %v", args)
	}
}

func TestBuildAuditWhere_EqualityFiltersAndParamOrder(t *testing.T) {
	env := common.EnvProduction
	action := "sync.execute"
	rt := "sync_job"
	rid := "g_1001"
	op := int64(12)
	where, args := buildAuditWhere(auditapp.AuditQuery{
		Env: &env, Action: &action, ResourceType: &rt, ResourceID: &rid, Operator: &op,
	})
	for _, want := range []string{"al.env = $1", "al.action = $2", "al.resource_type = $3", "al.resource_id = $4", "al.actor_id = $5"} {
		if !strings.Contains(where, want) {
			t.Errorf("WHERE missing %q: %s", want, where)
		}
	}
	wantArgs := []any{common.EnvProduction, "sync.execute", "sync_job", "g_1001", int64(12)}
	if len(args) != len(wantArgs) {
		t.Fatalf("args len=%d want %d (%v)", len(args), len(wantArgs), args)
	}
	for i := range wantArgs {
		if args[i] != wantArgs[i] {
			t.Errorf("arg[%d]=%v want %v", i, args[i], wantArgs[i])
		}
	}
}

func TestBuildAuditWhere_OperatorTakesPrecedenceOverKeyword(t *testing.T) {
	op := int64(5)
	kw := "alice"
	where, args := buildAuditWhere(auditapp.AuditQuery{Operator: &op, OperatorKeyword: &kw})
	if !strings.Contains(where, "al.actor_id = $1") {
		t.Fatalf("expected actor_id equality, got %s", where)
	}
	if strings.Contains(where, "ILIKE") {
		t.Fatalf("operatorKeyword must be ignored when operator present: %s", where)
	}
	if len(args) != 1 || args[0] != int64(5) {
		t.Fatalf("args=%v want [5]", args)
	}
}

func TestBuildAuditWhere_OperatorKeywordDualParam(t *testing.T) {
	kw := "ali"
	where, args := buildAuditWhere(auditapp.AuditQuery{OperatorKeyword: &kw})
	if !strings.Contains(where, "au.user_name ILIKE $1") || !strings.Contains(where, "au.display_name ILIKE $2") {
		t.Fatalf("keyword join clause wrong: %s", where)
	}
	if len(args) != 2 || args[0] != "%ali%" || args[1] != "%ali%" {
		t.Fatalf("args=%v want two %%ali%%", args)
	}
}

func TestBuildAuditWhere_KeywordMatchesResourceAndSummary(t *testing.T) {
	kw := "g_1001"
	where, args := buildAuditWhere(auditapp.AuditQuery{Keyword: &kw})
	if !strings.Contains(where, "al.resource_id ILIKE $1") || !strings.Contains(where, "al.detail_json->>'summary'") {
		t.Fatalf("keyword clause wrong: %s", where)
	}
	if len(args) != 2 {
		t.Fatalf("keyword should bind 2 params, got %v", args)
	}
}

func TestBuildAuditWhere_TimeRangeUsesUTC(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	from := time.Date(2026, 6, 1, 8, 0, 0, 0, loc)
	to := time.Date(2026, 6, 2, 8, 0, 0, 0, loc)
	where, args := buildAuditWhere(auditapp.AuditQuery{From: &from, To: &to})
	if !strings.Contains(where, "al.created_at >= $1") || !strings.Contains(where, "al.created_at <= $2") {
		t.Fatalf("time range clause wrong: %s", where)
	}
	gotFrom := args[0].(time.Time)
	if gotFrom.Location() != time.UTC {
		t.Fatalf("from not normalized to UTC: %v", gotFrom.Location())
	}
}

// ───────────────────────── Query 排序与分页（fake DBTX）─────────────────────────

func TestQuery_DefaultSortDescStableByID(t *testing.T) {
	db := &fakeDBTX{total: 0}
	repo := NewAuditRepository(db)
	_, _, err := repo.Query(context.Background(), auditapp.AuditQuery{Page: 1, PageSize: 20, SortDesc: true})
	if !errors.Is(err, errCapture) {
		t.Fatalf("expected capture error, got %v", err)
	}
	if !strings.Contains(db.querySQL, "ORDER BY al.created_at DESC, al.id DESC") {
		t.Fatalf("default order must be created_at DESC, id DESC: %s", db.querySQL)
	}
}

func TestQuery_AscendingSort(t *testing.T) {
	db := &fakeDBTX{total: 0}
	repo := NewAuditRepository(db)
	_, _, _ = repo.Query(context.Background(), auditapp.AuditQuery{Page: 1, PageSize: 20, SortDesc: false})
	if !strings.Contains(db.querySQL, "ORDER BY al.created_at ASC, al.id ASC") {
		t.Fatalf("ascending order wrong: %s", db.querySQL)
	}
}

func TestQuery_PaginationLimitOffsetParams(t *testing.T) {
	db := &fakeDBTX{total: 0}
	repo := NewAuditRepository(db)
	env := common.EnvSandbox
	_, _, _ = repo.Query(context.Background(), auditapp.AuditQuery{Env: &env, Page: 3, PageSize: 25, SortDesc: true})
	// 1 filter arg + LIMIT + OFFSET
	if len(db.queryArgs) != 3 {
		t.Fatalf("want 3 query args (env, limit, offset), got %v", db.queryArgs)
	}
	if db.queryArgs[1] != 25 {
		t.Fatalf("limit arg=%v want 25", db.queryArgs[1])
	}
	if db.queryArgs[2] != (3-1)*25 {
		t.Fatalf("offset arg=%v want %d", db.queryArgs[2], (3-1)*25)
	}
	if !strings.Contains(db.querySQL, "LIMIT $2 OFFSET $3") {
		t.Fatalf("limit/offset placeholders wrong: %s", db.querySQL)
	}
}

// ───────────────────────── Insert: detail 默认 {} ─────────────────────────

func TestInsert_EmptyDetailMarshalsValidJSON(t *testing.T) {
	db := &fakeDBTX{}
	repo := NewAuditRepository(db)
	row := common.AuditLog{
		ActorID: 12, Action: "game.update", ResourceType: "game", ResourceID: "g1", Env: common.EnvSandbox,
		Detail: common.AuditDetail{Summary: "s"},
	}
	if err := repo.Insert(context.Background(), row); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if !strings.Contains(db.execSQL, "INSERT INTO audit_logs") {
		t.Fatalf("unexpected insert sql: %s", db.execSQL)
	}
	// args: actor_id, action, resource_type, resource_id, env, detail_json(string)
	if len(db.execArgs) != 6 {
		t.Fatalf("want 6 insert args, got %d: %v", len(db.execArgs), db.execArgs)
	}
	if db.execArgs[0] != int64(12) || db.execArgs[1] != "game.update" {
		t.Fatalf("insert scalar args wrong: %v", db.execArgs[:2])
	}
	detailJSON, ok := db.execArgs[5].(string)
	if !ok {
		t.Fatalf("detail arg not string: %T", db.execArgs[5])
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal([]byte(detailJSON), &decoded); err != nil {
		t.Fatalf("detail json invalid: %v", err)
	}
	if string(decoded["summary"]) != `"s"` {
		t.Fatalf("summary not marshaled: %s", detailJSON)
	}
	// Detail 非空时应按输入序列化（不依赖 DB default）。
	for _, key := range []string{"before", "after", "extra"} {
		if _, present := decoded[key]; present {
			t.Errorf("empty detail.%s is omitted by omitempty, but found in JSON: %s", key, detailJSON)
		}
	}
}

func TestInsert_ZeroDetailWritesJSONEmptyObject(t *testing.T) {
	db := &fakeDBTX{}
	repo := NewAuditRepository(db)
	row := common.AuditLog{
		ActorID: 1, Action: "channel.update", ResourceType: "channel", ResourceID: "gmc_1", Env: common.EnvSandbox,
	}
	if err := repo.Insert(context.Background(), row); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	detailJSON, ok := db.execArgs[5].(string)
	if !ok {
		t.Fatalf("detail arg not string: %T", db.execArgs[5])
	}
	if detailJSON != "{}" {
		t.Fatalf("zero detail should be {}, got %s", detailJSON)
	}
}

func TestInsert_PreservesProvidedDetail(t *testing.T) {
	db := &fakeDBTX{}
	repo := NewAuditRepository(db)
	row := common.AuditLog{
		ActorID: 1, Action: "game.update", ResourceType: "game", ResourceID: "g1", Env: common.EnvSandbox,
		Detail: common.AuditDetail{Before: map[string]any{"name": "a"}, After: map[string]any{"name": "b"}, Changed: []string{"name"}},
	}
	if err := repo.Insert(context.Background(), row); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	detailJSON := db.execArgs[5].(string)
	if !strings.Contains(detailJSON, `"before":{"name":"a"}`) || !strings.Contains(detailJSON, `"after":{"name":"b"}`) {
		t.Fatalf("provided detail not preserved: %s", detailJSON)
	}
}
