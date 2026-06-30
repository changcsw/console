package audit

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// L1 单元（无 IO）：覆盖 AuditService 纯逻辑——递归脱敏、detail 结构规整、
// changed 校验、env/actor 取值、ctx 去重标志、Query 校验与分页钳制。
// 规则来源 docs/architecture/v2/modules/22-audit/spec.compact.md §5/§7。

// fakeRepo 是无 IO 的内存仓储替身，记录写入行并回放查询结果。
type fakeRepo struct {
	inserted  []common.AuditLog
	insertErr error

	queryItems []AuditLogItem
	queryTotal int64
	queryErr   error
	lastQuery  AuditQuery
}

func (f *fakeRepo) Insert(_ context.Context, row common.AuditLog) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, row)
	return nil
}

func (f *fakeRepo) Query(_ context.Context, q AuditQuery) ([]AuditLogItem, int64, error) {
	f.lastQuery = q
	if f.queryErr != nil {
		return nil, 0, f.queryErr
	}
	return f.queryItems, f.queryTotal, nil
}

func newService(repo Repository, env common.Environment) Service {
	return NewService(repo, env)
}

// ───────────────────────── Write: 入参校验 ─────────────────────────

func TestWrite_ValidationRejectsEmptyRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		in   AuditWriteInput
	}{
		{"empty_action", AuditWriteInput{Action: "", ResourceType: "game", ResourceID: "g1"}},
		{"empty_resource_type", AuditWriteInput{Action: "game.update", ResourceType: "", ResourceID: "g1"}},
		{"empty_resource_id", AuditWriteInput{Action: "game.update", ResourceType: "game", ResourceID: ""}},
		{"blank_action", AuditWriteInput{Action: "   ", ResourceType: "game", ResourceID: "g1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeRepo{}
			err := newService(repo, common.EnvSandbox).Write(context.Background(), SecretAwareAuditInput{AuditWriteInput: c.in})
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("want ErrValidation, got %v", err)
			}
			if len(repo.inserted) != 0 {
				t.Fatalf("expected no insert on validation failure, got %d", len(repo.inserted))
			}
		})
	}
}

// ───────────────────────── Write: 递归脱敏 ─────────────────────────

func TestWrite_SanitizeMasksSecretKeysRecursively(t *testing.T) {
	repo := &fakeRepo{}
	in := SecretAwareAuditInput{
		AuditWriteInput: AuditWriteInput{
			Action: "payment.update", ResourceType: "payment_route", ResourceID: "pr_1",
			Env: common.EnvProduction,
			Detail: common.AuditDetail{
				Before: map[string]any{
					"name":   "old",
					"secret": "plain-old",
				},
				After: map[string]any{
					"name":   "new",
					"Secret": "plain-new", // 大小写不敏感
					"nested": map[string]any{
						"apiKey": "should-mask",
						"keep":   "visible",
					},
					"list": []any{
						map[string]any{"token": "t-plain", "ok": "v"},
					},
				},
				Extra: map[string]any{
					"password": "pw",
					"meta":     map[string]any{"deep": map[string]any{"secret": "x"}},
				},
			},
		},
		SecretKeys: []string{"secret", "apiKey", "token", "password"},
	}
	if err := newService(repo, common.EnvSandbox).Write(context.Background(), in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := repo.inserted[0].Detail

	if got.Before["secret"] != "masked" {
		t.Errorf("before.secret not masked: %v", got.Before["secret"])
	}
	if got.Before["name"] != "old" {
		t.Errorf("before.name should be untouched: %v", got.Before["name"])
	}
	if got.After["Secret"] != "masked" {
		t.Errorf("after.Secret (case-insensitive) not masked: %v", got.After["Secret"])
	}
	nested := got.After["nested"].(map[string]any)
	if nested["apiKey"] != "masked" {
		t.Errorf("after.nested.apiKey not masked: %v", nested["apiKey"])
	}
	if nested["keep"] != "visible" {
		t.Errorf("after.nested.keep should be visible: %v", nested["keep"])
	}
	list := got.After["list"].([]any)
	elem := list[0].(map[string]any)
	if elem["token"] != "masked" {
		t.Errorf("after.list[0].token not masked: %v", elem["token"])
	}
	if elem["ok"] != "v" {
		t.Errorf("after.list[0].ok should be visible: %v", elem["ok"])
	}
	if got.Extra["password"] != "masked" {
		t.Errorf("extra.password not masked: %v", got.Extra["password"])
	}
	deep := got.Extra["meta"].(map[string]any)["deep"].(map[string]any)
	if deep["secret"] != "masked" {
		t.Errorf("extra.meta.deep.secret not masked: %v", deep["secret"])
	}
}

func TestWrite_NoSecretKeysLeavesPlaintext(t *testing.T) {
	repo := &fakeRepo{}
	in := SecretAwareAuditInput{
		AuditWriteInput: AuditWriteInput{
			Action: "game.update", ResourceType: "game", ResourceID: "g1",
			Env:    common.EnvSandbox,
			Detail: common.AuditDetail{After: map[string]any{"name": "v"}},
		},
	}
	if err := newService(repo, common.EnvSandbox).Write(context.Background(), in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if repo.inserted[0].Detail.After["name"] != "v" {
		t.Fatalf("expected plaintext when no secret keys")
	}
}

// ───────────────────────── Write: changed 规整 ─────────────────────────

func TestWrite_NormalizeChanged(t *testing.T) {
	repo := &fakeRepo{}
	in := SecretAwareAuditInput{
		AuditWriteInput: AuditWriteInput{
			Action: "game.update", ResourceType: "game", ResourceID: "g1", Env: common.EnvSandbox,
			Detail: common.AuditDetail{
				Summary: "x",
				Changed: []string{"status", "name", "status", "  ", "name", " env "},
			},
		},
	}
	if err := newService(repo, common.EnvSandbox).Write(context.Background(), in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := repo.inserted[0].Detail.Changed
	want := []string{"env", "name", "status"} // 去空白、去重、排序
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeChanged got %v want %v", got, want)
	}
}

func TestWrite_EmptyChangedBecomesNil(t *testing.T) {
	repo := &fakeRepo{}
	in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
		Action: "game.update", ResourceType: "game", ResourceID: "g1", Env: common.EnvSandbox,
		Detail: common.AuditDetail{Summary: "x", Changed: []string{"  ", ""}},
	}}
	if err := newService(repo, common.EnvSandbox).Write(context.Background(), in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if repo.inserted[0].Detail.Changed != nil {
		t.Fatalf("expected nil changed, got %v", repo.inserted[0].Detail.Changed)
	}
}

// ───────────────────────── Write: summary 缺省 ─────────────────────────

func TestWrite_DefaultSummary(t *testing.T) {
	t.Run("with_changed", func(t *testing.T) {
		repo := &fakeRepo{}
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			Action: "game.update", ResourceType: "game", ResourceID: "g1", Env: common.EnvSandbox,
			Detail: common.AuditDetail{Changed: []string{"name"}},
		}}
		_ = newService(repo, common.EnvSandbox).Write(context.Background(), in)
		if got := repo.inserted[0].Detail.Summary; got != "game.update g1 fields: name" {
			t.Fatalf("summary=%q", got)
		}
	})
	t.Run("without_changed", func(t *testing.T) {
		repo := &fakeRepo{}
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			Action: "game.delete", ResourceType: "game", ResourceID: "g1", Env: common.EnvSandbox,
		}}
		_ = newService(repo, common.EnvSandbox).Write(context.Background(), in)
		if got := repo.inserted[0].Detail.Summary; got != "game.delete g1" {
			t.Fatalf("summary=%q", got)
		}
	})
	t.Run("explicit_summary_kept", func(t *testing.T) {
		repo := &fakeRepo{}
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			Action: "game.update", ResourceType: "game", ResourceID: "g1", Env: common.EnvSandbox,
			Detail: common.AuditDetail{Summary: "custom"},
		}}
		_ = newService(repo, common.EnvSandbox).Write(context.Background(), in)
		if got := repo.inserted[0].Detail.Summary; got != "custom" {
			t.Fatalf("summary=%q", got)
		}
	})
}

// ───────────────────────── Write: env 取值 ─────────────────────────

func TestWrite_EnvResolution(t *testing.T) {
	t.Run("explicit_env_wins", func(t *testing.T) {
		repo := &fakeRepo{}
		ctx := InjectRequestContext(context.Background(), 1, common.EnvSandbox, common.AuditRequestMeta{})
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			Action: "sync.execute", ResourceType: "sync_job", ResourceID: "g1",
			Env: common.EnvProduction, Detail: common.AuditDetail{Summary: "x"},
		}}
		_ = newService(repo, common.EnvDevelop).Write(ctx, in)
		if repo.inserted[0].Env != common.EnvProduction {
			t.Fatalf("env=%q want production", repo.inserted[0].Env)
		}
	})
	t.Run("env_from_ctx_meta", func(t *testing.T) {
		repo := &fakeRepo{}
		ctx := InjectRequestContext(context.Background(), 1, common.EnvSandbox, common.AuditRequestMeta{})
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			Action: "game.update", ResourceType: "game", ResourceID: "g1", Detail: common.AuditDetail{Summary: "x"},
		}}
		_ = newService(repo, common.EnvDevelop).Write(ctx, in)
		if repo.inserted[0].Env != common.EnvSandbox {
			t.Fatalf("env=%q want sandbox (from ctx)", repo.inserted[0].Env)
		}
	})
	t.Run("env_fallback_runtime", func(t *testing.T) {
		repo := &fakeRepo{}
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			Action: "game.update", ResourceType: "game", ResourceID: "g1", Detail: common.AuditDetail{Summary: "x"},
		}}
		_ = newService(repo, common.EnvDevelop).Write(context.Background(), in)
		if repo.inserted[0].Env != common.EnvDevelop {
			t.Fatalf("env=%q want develop (runtime fallback)", repo.inserted[0].Env)
		}
	})
}

// ───────────────────────── Write: actor 取值 ─────────────────────────

func TestWrite_ActorResolution(t *testing.T) {
	t.Run("explicit_actor_wins", func(t *testing.T) {
		repo := &fakeRepo{}
		ctx := InjectRequestContext(context.Background(), 99, common.EnvSandbox, common.AuditRequestMeta{})
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			ActorID: 7, Action: "game.update", ResourceType: "game", ResourceID: "g1", Detail: common.AuditDetail{Summary: "x"},
		}}
		_ = newService(repo, common.EnvSandbox).Write(ctx, in)
		if repo.inserted[0].ActorID != 7 {
			t.Fatalf("actor=%d want 7", repo.inserted[0].ActorID)
		}
	})
	t.Run("actor_from_ctx_meta", func(t *testing.T) {
		repo := &fakeRepo{}
		ctx := InjectRequestContext(context.Background(), 42, common.EnvSandbox, common.AuditRequestMeta{})
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			Action: "game.update", ResourceType: "game", ResourceID: "g1", Detail: common.AuditDetail{Summary: "x"},
		}}
		_ = newService(repo, common.EnvSandbox).Write(ctx, in)
		if repo.inserted[0].ActorID != 42 {
			t.Fatalf("actor=%d want 42", repo.inserted[0].ActorID)
		}
	})
	t.Run("actor_from_auth_context", func(t *testing.T) {
		repo := &fakeRepo{}
		ac := domainauth.NewAuthContext(123, "alice", "Alice", nil, nil, common.EnvSandbox)
		ctx := adminapp.WithAuthContext(context.Background(), ac)
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			Action: "game.update", ResourceType: "game", ResourceID: "g1", Detail: common.AuditDetail{Summary: "x"},
		}}
		_ = newService(repo, common.EnvSandbox).Write(ctx, in)
		if repo.inserted[0].ActorID != 123 {
			t.Fatalf("actor=%d want 123 (from auth ctx)", repo.inserted[0].ActorID)
		}
	})
	t.Run("system_actor_zero", func(t *testing.T) {
		repo := &fakeRepo{}
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			Action: "fx.apply", ResourceType: "fx", ResourceID: "run1", Env: common.EnvProduction, Detail: common.AuditDetail{Summary: "x"},
		}}
		_ = newService(repo, common.EnvProduction).Write(context.Background(), in)
		if repo.inserted[0].ActorID != 0 {
			t.Fatalf("actor=%d want 0 (system placeholder)", repo.inserted[0].ActorID)
		}
	})
	t.Run("negative_actor_clamped_to_zero", func(t *testing.T) {
		repo := &fakeRepo{}
		in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
			ActorID: -5, Action: "fx.apply", ResourceType: "fx", ResourceID: "run1", Env: common.EnvProduction, Detail: common.AuditDetail{Summary: "x"},
		}}
		_ = newService(repo, common.EnvProduction).Write(context.Background(), in)
		if repo.inserted[0].ActorID != 0 {
			t.Fatalf("actor=%d want 0 (negative clamped)", repo.inserted[0].ActorID)
		}
	})
}

// ───────────────────────── Write: request 元补全 ─────────────────────────

func TestWrite_RequestMetaFilledFromContext(t *testing.T) {
	repo := &fakeRepo{}
	meta := common.AuditRequestMeta{IP: "10.0.0.1", Method: "POST", Path: "/api/admin/games"}
	ctx := InjectRequestContext(context.Background(), 1, common.EnvSandbox, meta)
	in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
		Action: "game.create", ResourceType: "game", ResourceID: "g1", Detail: common.AuditDetail{Summary: "x"},
	}}
	_ = newService(repo, common.EnvSandbox).Write(ctx, in)
	req := repo.inserted[0].Detail.Request
	if req == nil || req.IP != "10.0.0.1" || req.Method != "POST" {
		t.Fatalf("request meta not filled from ctx: %+v", req)
	}
}

// ───────────────────────── Write: ctx 去重标志 ─────────────────────────

func TestWrite_MarksWrittenForDedup(t *testing.T) {
	repo := &fakeRepo{}
	ctx := InjectRequestContext(context.Background(), 1, common.EnvSandbox, common.AuditRequestMeta{})
	if IsWritten(ctx) {
		t.Fatalf("ctx should start unmarked")
	}
	in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
		Action: "game.update", ResourceType: "game", ResourceID: "g1", Detail: common.AuditDetail{Summary: "x"},
	}}
	if err := newService(repo, common.EnvSandbox).Write(ctx, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !IsWritten(ctx) {
		t.Fatalf("ctx should be marked written after explicit Write (dedup)")
	}
}

func TestWrite_DoesNotMarkWhenInsertFails(t *testing.T) {
	repo := &fakeRepo{insertErr: errors.New("db down")}
	ctx := InjectRequestContext(context.Background(), 1, common.EnvSandbox, common.AuditRequestMeta{})
	in := SecretAwareAuditInput{AuditWriteInput: AuditWriteInput{
		Action: "game.update", ResourceType: "game", ResourceID: "g1", Detail: common.AuditDetail{Summary: "x"},
	}}
	if err := newService(repo, common.EnvSandbox).Write(ctx, in); err == nil {
		t.Fatalf("expected insert error propagated")
	}
	if IsWritten(ctx) {
		t.Fatalf("ctx must NOT be marked written when insert fails (so middleware can fallback)")
	}
}

// ───────────────────────── Query: 校验与分页 ─────────────────────────

func TestQuery_FromAfterToValidation(t *testing.T) {
	repo := &fakeRepo{}
	from := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	_, err := newService(repo, common.EnvSandbox).Query(context.Background(), AuditQuery{From: &from, To: &to})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for from>to, got %v", err)
	}
}

func TestQuery_FromEqualToAllowed(t *testing.T) {
	repo := &fakeRepo{queryItems: []AuditLogItem{}, queryTotal: 0}
	at := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	_, err := newService(repo, common.EnvSandbox).Query(context.Background(), AuditQuery{From: &at, To: &at})
	if err != nil {
		t.Fatalf("from==to should be allowed, got %v", err)
	}
}

func TestQuery_PageNormalization(t *testing.T) {
	cases := []struct {
		name                       string
		inPage, inSize             int
		wantPage, wantSize         int
	}{
		{"defaults", 0, 0, 1, 20},
		{"page_below_one", -3, 10, 1, 10},
		{"pagesize_clamped_to_100", 2, 500, 2, 100},
		{"pagesize_exactly_100", 1, 100, 1, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeRepo{queryItems: []AuditLogItem{}}
			page, err := newService(repo, common.EnvSandbox).Query(context.Background(), AuditQuery{Page: c.inPage, PageSize: c.inSize})
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			if page.Page != c.wantPage || page.PageSize != c.wantSize {
				t.Fatalf("page=%d size=%d want page=%d size=%d", page.Page, page.PageSize, c.wantPage, c.wantSize)
			}
			if repo.lastQuery.Page != c.wantPage || repo.lastQuery.PageSize != c.wantSize {
				t.Fatalf("repo received page=%d size=%d want %d/%d", repo.lastQuery.Page, repo.lastQuery.PageSize, c.wantPage, c.wantSize)
			}
		})
	}
}

func TestQuery_ByIDNotFound(t *testing.T) {
	repo := &fakeRepo{queryItems: []AuditLogItem{}}
	id := int64(123)
	_, err := newService(repo, common.EnvSandbox).Query(context.Background(), AuditQuery{ID: &id})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for missing id, got %v", err)
	}
}

func TestQuery_ReturnsPageAndPassesSort(t *testing.T) {
	repo := &fakeRepo{
		queryItems: []AuditLogItem{{AuditLog: common.AuditLog{ID: 1, Action: "game.update"}}},
		queryTotal: 7,
	}
	page, err := newService(repo, common.EnvSandbox).Query(context.Background(), AuditQuery{Page: 1, PageSize: 20, SortDesc: false})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if page.Total != 7 || len(page.Items) != 1 {
		t.Fatalf("unexpected page: total=%d items=%d", page.Total, len(page.Items))
	}
	if repo.lastQuery.SortDesc != false {
		t.Fatalf("SortDesc not passed through to repo")
	}
}
