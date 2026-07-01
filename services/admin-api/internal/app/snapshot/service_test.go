package snapshot

import (
	"context"
	"errors"
	"testing"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainsnapshot "github.com/csw/console/services/admin-api/internal/domain/snapshot"
)

// ─────────────────────────── 内存 fake（无真实 IO） ───────────────────────────

type fakeRepo struct {
	gameRowID int64
	view      domainsnapshot.ValidDataView
	payWays   []string
	rows      map[int64]domainsnapshot.ConfigSnapshot
	nextID    int64
	getErr    error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		gameRowID: 501,
		rows:      map[int64]domainsnapshot.ConfigSnapshot{},
		nextID:    11,
		view: domainsnapshot.ValidDataView{
			GameID:    "100001",
			GameIDRef: 501,
			Channels: []domainsnapshot.ChannelInput{
				{
					ChannelID:    "google",
					Region:       "overseas",
					Market:       common.MarketGlobal,
					Enabled:      true,
					ConfigStatus: common.ConfigStatusValid,
				},
			},
		},
	}
}

func (r *fakeRepo) ResolveGameRowID(_ context.Context, _ string) (int64, error) {
	return r.gameRowID, nil
}

func (r *fakeRepo) LoadValidData(_ context.Context, _ int64, _ string, generatedAt time.Time) (domainsnapshot.ValidDataView, []string, error) {
	v := r.view
	v.GeneratedAt = generatedAt
	return v, r.payWays, nil
}

func (r *fakeRepo) CreateSnapshot(_ context.Context, in CreateSnapshotInput) (domainsnapshot.ConfigSnapshot, error) {
	id := r.nextID
	r.nextID++
	row := domainsnapshot.ConfigSnapshot{
		ID:                  id,
		GameIDRef:           in.GameIDRef,
		ConfigSchemaVersion: in.ConfigSchemaVersion,
		ConfigVersion:       in.ConfigVersion,
		ConfigJSON:          in.ConfigJSON,
		FileName:            in.FileName,
		FileHash:            in.FileHash,
		StorageKey:          in.StorageKey,
		Status:              domainsnapshot.StatusDraft,
		GeneratedAt:         in.GeneratedAt,
	}
	r.rows[id] = row
	return row, nil
}

func (r *fakeRepo) ListSnapshots(_ context.Context, _ string, _ ListFilter) ([]domainsnapshot.ConfigSnapshot, int, error) {
	out := make([]domainsnapshot.ConfigSnapshot, 0, len(r.rows))
	for _, row := range r.rows {
		out = append(out, row)
	}
	return out, len(out), nil
}

func (r *fakeRepo) GetSnapshot(_ context.Context, id int64) (domainsnapshot.ConfigSnapshot, error) {
	if r.getErr != nil {
		return domainsnapshot.ConfigSnapshot{}, r.getErr
	}
	row, ok := r.rows[id]
	if !ok {
		return domainsnapshot.ConfigSnapshot{}, adminapp.ErrNotFound
	}
	return row, nil
}

func (r *fakeRepo) PublishSnapshot(_ context.Context, id int64, publishedAt time.Time) (domainsnapshot.ConfigSnapshot, error) {
	row := r.rows[id]
	row.Status = domainsnapshot.StatusPublished
	row.PublishedAt = &publishedAt
	r.rows[id] = row
	return row, nil
}

type fakeTx struct{ repo *fakeRepo }

func (t *fakeTx) Repository() Repository { return t.repo }
func (t *fakeTx) InTx(ctx context.Context, fn func(Repository) error) error {
	return fn(t.repo)
}

type fakeAudit struct{ entries []adminapp.AuditEntry }

func (a *fakeAudit) Write(_ context.Context, e adminapp.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func fixedNow() func() time.Time {
	return func() time.Time { return time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC) }
}

func newSvc(repo *fakeRepo, audit *fakeAudit) Service {
	return NewService(&fakeTx{repo: repo}, nil, audit, fixedNow())
}

func asAppErr(t *testing.T, err error) *Error {
	t.Helper()
	var appErr *Error
	if !errors.As(err, &appErr) {
		t.Fatalf("期望 *Error，got %T: %v", err, err)
	}
	return appErr
}

// ─────────────────────────── Generate ───────────────────────────

func TestService_Generate_SuccessDraftAndAudit(t *testing.T) {
	repo := newFakeRepo()
	audit := &fakeAudit{}
	res, err := newSvc(repo, audit).Generate(context.Background(), "100001")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if res.Status != string(domainsnapshot.StatusDraft) {
		t.Fatalf("生成结果应为 draft，got %s", res.Status)
	}
	if res.ConfigVersion == "" || res.FileHash == "" {
		t.Fatalf("应产出 configVersion/fileHash，got %+v", res)
	}
	// S7 审计：写 snapshot.generate。
	if len(audit.entries) != 1 || audit.entries[0].Action != "snapshot.generate" {
		t.Fatalf("应写一条 snapshot.generate 审计，got %+v", audit.entries)
	}
}

func TestService_Generate_ValidationEmptyGameID(t *testing.T) {
	_, err := newSvc(newFakeRepo(), &fakeAudit{}).Generate(context.Background(), "  ")
	if got := asAppErr(t, err).Code; got != CodeValidation {
		t.Fatalf("空 gameId 应 VALIDATION_FAILED，got %s", got)
	}
}

// I4：同源两次生成产出一致的 fileHash/configVersion（注入固定时钟）。
func TestService_Generate_Deterministic(t *testing.T) {
	r1, r2 := newFakeRepo(), newFakeRepo()
	a1, err := newSvc(r1, &fakeAudit{}).Generate(context.Background(), "100001")
	if err != nil {
		t.Fatalf("gen1: %v", err)
	}
	a2, err := newSvc(r2, &fakeAudit{}).Generate(context.Background(), "100001")
	if err != nil {
		t.Fatalf("gen2: %v", err)
	}
	if a1.FileHash != a2.FileHash || a1.ConfigVersion != a2.ConfigVersion {
		t.Fatalf("同源生成应确定性一致\n a1=%+v\n a2=%+v", a1, a2)
	}
}

// ─────────────────────────── Publish（I5 状态单调） ───────────────────────────

func TestService_Publish_DraftToPublished(t *testing.T) {
	repo := newFakeRepo()
	audit := &fakeAudit{}
	svc := newSvc(repo, audit)
	gen, err := svc.Generate(context.Background(), "100001")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	item, err := svc.Publish(context.Background(), gen.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if item.Status != string(domainsnapshot.StatusPublished) {
		t.Fatalf("发布后应为 published，got %s", item.Status)
	}
	if item.PublishedAt == nil {
		t.Fatalf("发布后 publishedAt 应置位")
	}
	// S7 审计：generate + publish 各一条。
	var publishAudited bool
	for _, e := range audit.entries {
		if e.Action == "snapshot.publish" {
			publishAudited = true
		}
	}
	if !publishAudited {
		t.Fatalf("应写一条 snapshot.publish 审计，got %+v", audit.entries)
	}
}

// I5：已发布快照不可再次发布（不可回退/重复发布）。
func TestService_Publish_AlreadyPublished_VersionStateInvalid(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo, &fakeAudit{})
	gen, err := svc.Generate(context.Background(), "100001")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := svc.Publish(context.Background(), gen.ID); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	_, err = svc.Publish(context.Background(), gen.ID)
	if got := asAppErr(t, err).Code; got != CodeVersionStateInvalid {
		t.Fatalf("重复发布应 VERSION_STATE_INVALID，got %s", got)
	}
}

func TestService_Publish_NotFound(t *testing.T) {
	_, err := newSvc(newFakeRepo(), &fakeAudit{}).Publish(context.Background(), 9999)
	if got := asAppErr(t, err).Code; got != CodeNotFound {
		t.Fatalf("不存在快照发布应 NOT_FOUND，got %s", got)
	}
}

func TestService_Publish_InvalidID(t *testing.T) {
	_, err := newSvc(newFakeRepo(), &fakeAudit{}).Publish(context.Background(), 0)
	if got := asAppErr(t, err).Code; got != CodeValidation {
		t.Fatalf("非法 snapshotId 应 VALIDATION_FAILED，got %s", got)
	}
}

// ─────────────────────────── Download ───────────────────────────

func TestService_Download_ReturnsCanonicalBody(t *testing.T) {
	repo := newFakeRepo()
	svc := newSvc(repo, &fakeAudit{})
	gen, err := svc.Generate(context.Background(), "100001")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	dl, err := svc.Download(context.Background(), gen.ID)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if dl.FileName == "" || len(dl.Body) == 0 {
		t.Fatalf("下载应返回文件名与内容，got %+v", dl)
	}
	// body 应为 canonical JSON（键有序）。
	want, err := domainsnapshot.CanonicalJSON(repo.rows[gen.ID].ConfigJSON)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	if string(dl.Body) != string(want) {
		t.Fatalf("下载 body 应为 canonical JSON")
	}
}

func TestService_Download_NotFound(t *testing.T) {
	_, err := newSvc(newFakeRepo(), &fakeAudit{}).Download(context.Background(), 123)
	if got := asAppErr(t, err).Code; got != CodeNotFound {
		t.Fatalf("下载不存在快照应 NOT_FOUND，got %s", got)
	}
}
