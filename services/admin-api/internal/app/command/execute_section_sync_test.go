package command

import (
	"context"
	"errors"
	"testing"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"
)

// ─────────────────────────────────────────────────────────────────────────────
// fake 仓储 / 事务管理器 / 审计（进程内覆盖 execute 双闸门、事务回滚、审计红线）
// ─────────────────────────────────────────────────────────────────────────────

type fakeSyncRepo struct {
	gameExists map[string]bool // schema+"/"+gameID → exists
	sandbox    map[domainsync.Section][]domainsync.EntityRecord
	prod       map[domainsync.Section][]domainsync.EntityRecord
	masked     map[domainsync.Section]map[string]struct{}

	consumed        map[string]int64 // nonce → jobID
	consumedAt      map[string]time.Time
	consumeErr      error // ConsumeNonce 返回的错误（模拟并发唯一冲突）
	applyErrSection domainsync.Section
	applyErr        error

	createdJob   CreateSyncJobInput
	createdJobID int64
	addedItems   []SyncJobItemInput
	statusLog    []domainsync.SyncJobStatus
	appliedMark  bool

	listJobs  []domainsync.JobItem
	listTotal int
	lastList  struct {
		gameID   string
		page     int
		pageSize int
		status   string
	}
}

func newFakeRepo() *fakeSyncRepo {
	return &fakeSyncRepo{
		gameExists: map[string]bool{},
		sandbox:    map[domainsync.Section][]domainsync.EntityRecord{},
		prod:       map[domainsync.Section][]domainsync.EntityRecord{},
		masked:     map[domainsync.Section]map[string]struct{}{},
		consumed:   map[string]int64{},
		consumedAt: map[string]time.Time{},
	}
}

func (f *fakeSyncRepo) ResolveGameExists(_ context.Context, schema, gameID string) (bool, error) {
	return f.gameExists[schema+"/"+gameID], nil
}

func (f *fakeSyncRepo) LoadSectionEntities(_ context.Context, _, _, _ string, section domainsync.Section) ([]domainsync.EntityRecord, []domainsync.EntityRecord, map[string]struct{}, error) {
	m := f.masked[section]
	if m == nil {
		m = map[string]struct{}{}
	}
	return f.sandbox[section], f.prod[section], m, nil
}

func (f *fakeSyncRepo) CreateJob(_ context.Context, in CreateSyncJobInput) (int64, error) {
	f.createdJob = in
	if f.createdJobID == 0 {
		f.createdJobID = 4242
	}
	f.statusLog = append(f.statusLog, in.Status)
	return f.createdJobID, nil
}

func (f *fakeSyncRepo) AddItems(_ context.Context, _ int64, items []SyncJobItemInput) error {
	f.addedItems = append(f.addedItems, items...)
	return nil
}

func (f *fakeSyncRepo) ListJobsByGame(_ context.Context, gameID string, page, pageSize int, status string) ([]domainsync.JobItem, int, error) {
	f.lastList.gameID = gameID
	f.lastList.page = page
	f.lastList.pageSize = pageSize
	f.lastList.status = status
	return f.listJobs, f.listTotal, nil
}

func (f *fakeSyncRepo) UpdateJobResult(_ context.Context, _ int64, status domainsync.SyncJobStatus, _ string, _ string, _ *time.Time) error {
	f.statusLog = append(f.statusLog, status)
	return nil
}

func (f *fakeSyncRepo) MarkItemsApplied(_ context.Context, _ int64, selected []domainsync.Section, _ bool) (map[domainsync.Section]domainsync.DiffSummary, []domainsync.ExecuteSkippedDelete, error) {
	f.appliedMark = true
	out := map[domainsync.Section]domainsync.DiffSummary{}
	for _, s := range selected {
		out[s] = domainsync.DiffSummary{Add: 1}
	}
	return out, []domainsync.ExecuteSkippedDelete{}, nil
}

func (f *fakeSyncRepo) IsNonceConsumed(_ context.Context, nonce string) (bool, int64, *time.Time, error) {
	if id, ok := f.consumed[nonce]; ok {
		at := f.consumedAt[nonce]
		return true, id, &at, nil
	}
	return false, 0, nil, nil
}

func (f *fakeSyncRepo) ConsumeNonce(_ context.Context, nonce string, syncJobID int64) error {
	if f.consumeErr != nil {
		return f.consumeErr
	}
	f.consumed[nonce] = syncJobID
	f.consumedAt[nonce] = time.Now().UTC()
	return nil
}

func (f *fakeSyncRepo) ApplySection(_ context.Context, section domainsync.Section, _ string, _ bool) error {
	if f.applyErr != nil && section == f.applyErrSection {
		return f.applyErr
	}
	return nil
}

// fakeTx 以快照/还原模拟单事务的提交/回滚语义。
type fakeTx struct {
	repo *fakeSyncRepo
}

func (t *fakeTx) Repository() SectionSyncRepository { return t.repo }

func (t *fakeTx) InTx(ctx context.Context, fn func(repo SectionSyncRepository) error) error {
	// 快照可回滚状态
	snapConsumed := make(map[string]int64, len(t.repo.consumed))
	for k, v := range t.repo.consumed {
		snapConsumed[k] = v
	}
	snapApplied := t.repo.appliedMark
	if err := fn(t.repo); err != nil {
		// 回滚：还原 nonce 与 applied 标记
		t.repo.consumed = snapConsumed
		t.repo.appliedMark = snapApplied
		return err
	}
	return nil
}

type fakeAuditSink struct {
	entries []adminapp.AuditEntry
}

func (s *fakeAuditSink) Write(_ context.Context, e adminapp.AuditEntry) error {
	s.entries = append(s.entries, e)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// 辅助：构造服务 + 合法 token
// ─────────────────────────────────────────────────────────────────────────────

const testTokenKey = "unit-test-secret"

func newService(repo *fakeSyncRepo, sink *fakeAuditSink) SectionSyncService {
	now := func() time.Time { return time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC) }
	return NewSectionSyncService(&fakeTx{repo: repo}, sink, now, testTokenKey)
}

func prodHash(t *testing.T, repo *fakeSyncRepo) string {
	t.Helper()
	sets := map[domainsync.Section][]domainsync.EntityRecord{}
	for _, s := range domainsync.AllSections() {
		sets[s] = repo.prod[s]
	}
	h, err := domainsync.HashEntitySets(sets)
	if err != nil {
		t.Fatalf("prod hash: %v", err)
	}
	return h
}

func validToken(t *testing.T, gameID string, targetHashBefore string, expiresAt time.Time) string {
	t.Helper()
	tok, err := domainsync.BuildBaselineToken(domainsync.BaselineTokenPayload{
		GameID:           gameID,
		SyncJobID:        4242,
		SourceEnv:        domainsync.DefaultSourceEnv(),
		TargetEnv:        domainsync.DefaultTargetEnv(),
		SourceHash:       "src",
		TargetHashBefore: targetHashBefore,
		PreviewedAt:      time.Date(2026, 7, 1, 9, 45, 0, 0, time.UTC),
		ExpiresAt:        expiresAt,
		Nonce:            "nonce-exec",
	}, []byte(testTokenKey))
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	return tok
}

func authCtx(userID int64) context.Context {
	ac := domainauth.NewAuthContext(userID, "op", "Op", nil, nil, common.EnvSandbox)
	return adminapp.WithAuthContext(context.Background(), ac)
}

func asSyncErr(t *testing.T, err error) *SectionSyncError {
	t.Helper()
	var se *SectionSyncError
	if !errors.As(err, &se) {
		t.Fatalf("want *SectionSyncError, got %T: %v", err, err)
	}
	return se
}

// ─────────────────────────────────────────────────────────────────────────────
// Execute — 成功路径 + 审计红线（S1/S7/env=production）
// ─────────────────────────────────────────────────────────────────────────────

func TestExecuteSuccessWritesAuditWithProductionEnv(t *testing.T) {
	repo := newFakeRepo()
	repo.gameExists["production/100001"] = true
	repo.prod[domainsync.SectionGame] = []domainsync.EntityRecord{{EntityType: "game", EntityKey: "100001", Fields: map[string]any{"name": "G"}}}
	sink := &fakeAuditSink{}
	svc := newService(repo, sink)

	tok := validToken(t, "100001", prodHash(t, repo), time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC))
	res, err := svc.Execute(authCtx(7), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"game"}, BaselineToken: tok,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res.Status != domainsync.JobStatusSucceeded {
		t.Fatalf("want succeeded, got %s", res.Status)
	}
	if res.TargetEnv != "production" || res.SourceEnv != "sandbox" {
		t.Fatalf("direction must be sandbox→production, got %s→%s", res.SourceEnv, res.TargetEnv)
	}
	if len(sink.entries) != 1 {
		t.Fatalf("want exactly one audit entry, got %d", len(sink.entries))
	}
	e := sink.entries[0]
	if e.Action != "sync.execute" {
		t.Errorf("audit action want sync.execute, got %s", e.Action)
	}
	if e.Env != common.EnvProduction {
		t.Errorf("audit env must be production (红线), got %s", e.Env)
	}
	if e.ActorID != 7 {
		t.Errorf("audit actorID must come from auth ctx, got %d", e.ActorID)
	}
	if e.ResourceID != "100001" {
		t.Errorf("audit resourceID want gameId, got %s", e.ResourceID)
	}
	if _, ok := e.Detail["selectedSections"]; !ok {
		t.Errorf("audit detail must contain selectedSections")
	}
	// nonce 应已消费（成功提交）
	if _, ok := repo.consumed["nonce-exec"]; !ok {
		t.Errorf("nonce must be consumed after success")
	}
	// 状态流转最终为 succeeded
	if repo.statusLog[len(repo.statusLog)-1] != domainsync.JobStatusSucceeded {
		t.Errorf("final status must be succeeded, got %v", repo.statusLog)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Execute — 闸门与错误码
// ─────────────────────────────────────────────────────────────────────────────

func TestExecuteUnknownSection(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, &fakeAuditSink{})
	_, err := svc.Execute(context.Background(), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"channels", "bogus"}, BaselineToken: "x.y",
	})
	if se := asSyncErr(t, err); se.Code != CodeUnknownSection || se.Status != 400 {
		t.Fatalf("want UNKNOWN_SECTION/400, got %s/%d", se.Code, se.Status)
	}
}

func TestExecuteInvalidToken(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, &fakeAuditSink{})
	_, err := svc.Execute(context.Background(), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"game"}, BaselineToken: "garbage.token",
	})
	if se := asSyncErr(t, err); se.Code != CodeValidation || se.Status != 400 {
		t.Fatalf("want VALIDATION_FAILED/400, got %s/%d", se.Code, se.Status)
	}
}

func TestExecuteExpiredToken(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, &fakeAuditSink{})
	// expiresAt 早于 now(2026-07-01T10:00Z)
	tok := validToken(t, "100001", "any", time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC))
	_, err := svc.Execute(context.Background(), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"game"}, BaselineToken: tok,
	})
	if se := asSyncErr(t, err); se.Code != CodeValidation {
		t.Fatalf("expired token want VALIDATION_FAILED, got %s", se.Code)
	}
}

func TestExecuteTokenGameMismatch(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, &fakeAuditSink{})
	// token 属于 game 999，请求 game 100001 → 上下文不匹配
	tok := validToken(t, "999", "any", time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC))
	_, err := svc.Execute(context.Background(), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"game"}, BaselineToken: tok,
	})
	if se := asSyncErr(t, err); se.Code != CodeValidation {
		t.Fatalf("game mismatch want VALIDATION_FAILED, got %s", se.Code)
	}
}

func TestExecuteNonceAlreadyConsumed(t *testing.T) {
	repo := newFakeRepo()
	repo.consumed["nonce-exec"] = 111
	repo.consumedAt["nonce-exec"] = time.Now().UTC()
	svc := newService(repo, &fakeAuditSink{})
	tok := validToken(t, "100001", "any", time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC))
	_, err := svc.Execute(context.Background(), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"game"}, BaselineToken: tok,
	})
	se := asSyncErr(t, err)
	if se.Code != CodeTokenConsumed || se.Status != 409 {
		t.Fatalf("want SYNC_TOKEN_CONSUMED/409, got %s/%d", se.Code, se.Status)
	}
}

func TestExecuteBaselineMismatch(t *testing.T) {
	repo := newFakeRepo()
	repo.gameExists["production/100001"] = true
	repo.prod[domainsync.SectionGame] = []domainsync.EntityRecord{{EntityType: "game", EntityKey: "100001", Fields: map[string]any{"name": "G"}}}
	svc := newService(repo, &fakeAuditSink{})
	// token 记录的 targetHashBefore 与当前实时 prod hash 不一致
	tok := validToken(t, "100001", "STALE_HASH", time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC))
	_, err := svc.Execute(context.Background(), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"game"}, BaselineToken: tok,
	})
	se := asSyncErr(t, err)
	if se.Code != CodeBaselineMismatch || se.Status != 409 {
		t.Fatalf("want SYNC_BASELINE_MISMATCH/409, got %s/%d", se.Code, se.Status)
	}
}

func TestExecuteDependencyMissing(t *testing.T) {
	repo := newFakeRepo()
	repo.gameExists["production/100001"] = true
	// production 为空：选 channels 时缺 game+markets 前置
	svc := newService(repo, &fakeAuditSink{})
	tok := validToken(t, "100001", prodHash(t, repo), time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC))
	_, err := svc.Execute(context.Background(), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"channels"}, BaselineToken: tok,
	})
	se := asSyncErr(t, err)
	if se.Code != CodeValidation || se.Status != 400 {
		t.Fatalf("want VALIDATION_FAILED/400, got %s/%d", se.Code, se.Status)
	}
	if len(se.Details) == 0 {
		t.Fatalf("dependency failure must carry details")
	}
}

func TestExecuteGameNotFoundInProduction(t *testing.T) {
	repo := newFakeRepo() // gameExists 未置 true
	svc := newService(repo, &fakeAuditSink{})
	tok := validToken(t, "100001", prodHash(t, repo), time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC))
	_, err := svc.Execute(context.Background(), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"game"}, BaselineToken: tok,
	})
	if se := asSyncErr(t, err); se.Code != CodeNotFound || se.Status != 404 {
		t.Fatalf("want NOT_FOUND/404, got %s/%d", se.Code, se.Status)
	}
}

func TestExecuteOperatorNoteTooLong(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, &fakeAuditSink{})
	note := make([]byte, 256)
	for i := range note {
		note[i] = 'a'
	}
	_, err := svc.Execute(context.Background(), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"game"}, BaselineToken: "x.y", OperatorNote: string(note),
	})
	if se := asSyncErr(t, err); se.Code != CodeValidation {
		t.Fatalf("operatorNote>255 want VALIDATION_FAILED, got %s", se.Code)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Execute — 事务回滚（S10）：中途失败整体回滚、状态 failed、不部分写入
// ─────────────────────────────────────────────────────────────────────────────

func TestExecuteTransactionRollbackOnApplyFailure(t *testing.T) {
	repo := newFakeRepo()
	repo.gameExists["production/100001"] = true
	repo.prod[domainsync.SectionGame] = []domainsync.EntityRecord{{EntityType: "game", EntityKey: "100001", Fields: map[string]any{"name": "G"}}}
	repo.applyErrSection = domainsync.SectionGame
	repo.applyErr = errors.New("boom: write failed mid-transaction")
	sink := &fakeAuditSink{}
	svc := newService(repo, sink)

	tok := validToken(t, "100001", prodHash(t, repo), time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC))
	_, err := svc.Execute(authCtx(7), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"game"}, BaselineToken: tok,
	})
	if err == nil {
		t.Fatalf("expected error on apply failure")
	}
	// 不部分写入：nonce 未消费（随事务回滚）
	if _, ok := repo.consumed["nonce-exec"]; ok {
		t.Errorf("nonce must NOT be consumed after rollback")
	}
	// 未写审计（提交前失败）
	if len(sink.entries) != 0 {
		t.Errorf("no audit should be written on failed execute, got %d", len(sink.entries))
	}
	// 任务状态最终标记 failed
	if repo.statusLog[len(repo.statusLog)-1] != domainsync.JobStatusFailed {
		t.Errorf("failed execute must mark job failed, got %v", repo.statusLog)
	}
}

func TestExecuteNonceRaceInTxReturnsConsumed(t *testing.T) {
	repo := newFakeRepo()
	repo.gameExists["production/100001"] = true
	repo.prod[domainsync.SectionGame] = []domainsync.EntityRecord{{EntityType: "game", EntityKey: "100001", Fields: map[string]any{"name": "G"}}}
	repo.consumeErr = adminapp.ErrConflict // 事务内落 nonce 命中唯一冲突（并发重复）
	svc := newService(repo, &fakeAuditSink{})

	tok := validToken(t, "100001", prodHash(t, repo), time.Date(2026, 7, 1, 10, 30, 0, 0, time.UTC))
	_, err := svc.Execute(context.Background(), ExecuteSectionSyncCommand{
		GameID: "100001", SelectedSections: []string{"game"}, BaselineToken: tok,
	})
	if se := asSyncErr(t, err); se.Code != CodeTokenConsumed || se.Status != 409 {
		t.Fatalf("nonce race want SYNC_TOKEN_CONSUMED/409, got %s/%d", se.Code, se.Status)
	}
	if repo.statusLog[len(repo.statusLog)-1] != domainsync.JobStatusFailed {
		t.Errorf("nonce race must mark job failed, got %v", repo.statusLog)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Preview — 落库 previewed + items(applied=false)、脱敏透传、baselineToken 可验签
// ─────────────────────────────────────────────────────────────────────────────

func TestPreviewCreatesJobAndVerifiableToken(t *testing.T) {
	repo := newFakeRepo()
	repo.gameExists["sandbox/100001"] = true
	repo.sandbox[domainsync.SectionChannels] = []domainsync.EntityRecord{
		{EntityType: "game_channel", EntityKey: "JP/google", Fields: map[string]any{"enabled": true}},
	}
	// 含密文字段的 login 配置：preview 恒 masked
	repo.sandbox[domainsync.SectionChannels] = append(repo.sandbox[domainsync.SectionChannels],
		domainsync.EntityRecord{EntityType: "login", EntityKey: "JP/google", Fields: map[string]any{"clientSecret": "PLAIN"}})
	repo.prod[domainsync.SectionChannels] = []domainsync.EntityRecord{
		{EntityType: "login", EntityKey: "JP/google", Fields: map[string]any{"clientSecret": "OTHER"}},
	}
	sink := &fakeAuditSink{}
	svc := newService(repo, sink)

	prev, err := svc.Preview(authCtx(9), PreviewSectionSyncCommand{GameID: "100001", SelectedSections: []string{"channels"}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.createdJob.Status != domainsync.JobStatusPreviewed {
		t.Errorf("preview must create job in previewed status, got %s", repo.createdJob.Status)
	}
	if !prev.HasDiff {
		t.Errorf("expected hasDiff=true")
	}
	// token 可验签
	if _, err := domainsync.ParseAndVerifyBaselineToken(prev.BaselineToken, []byte(testTokenKey), time.Date(2026, 7, 1, 10, 1, 0, 0, time.UTC)); err != nil {
		t.Errorf("preview token must verify: %v", err)
	}
	// 明文绝不入库：所有 masked item 的值恒为 "masked"
	sawMasked := false
	for _, it := range repo.addedItems {
		if it.Applied {
			t.Errorf("preview items must be applied=false")
		}
		if it.Masked {
			sawMasked = true
			if sv, ok := it.SandboxValue.(map[string]any); ok {
				if sv["value"] == "PLAIN" {
					t.Errorf("masked item leaked plaintext: %v", sv)
				}
			}
		}
	}
	if !sawMasked {
		t.Errorf("expected at least one masked item (clientSecret)")
	}
	// preview 不写审计
	if len(sink.entries) != 0 {
		t.Errorf("preview must not write audit, got %d", len(sink.entries))
	}
}

func TestPreviewUnknownSection(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, &fakeAuditSink{})
	_, err := svc.Preview(context.Background(), PreviewSectionSyncCommand{GameID: "100001", SelectedSections: []string{"nope"}})
	if se := asSyncErr(t, err); se.Code != CodeUnknownSection {
		t.Fatalf("want UNKNOWN_SECTION, got %s", se.Code)
	}
}

func TestPreviewGameNotFound(t *testing.T) {
	repo := newFakeRepo() // sandbox game 不存在
	svc := newService(repo, &fakeAuditSink{})
	_, err := svc.Preview(context.Background(), PreviewSectionSyncCommand{GameID: "100001"})
	if se := asSyncErr(t, err); se.Code != CodeNotFound {
		t.Fatalf("want NOT_FOUND, got %s", se.Code)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ListJobs — 分页边界（S9）
// ─────────────────────────────────────────────────────────────────────────────

func TestListJobsPaginationClamp(t *testing.T) {
	repo := newFakeRepo()
	repo.listTotal = 3
	svc := newService(repo, &fakeAuditSink{})
	_, err := svc.ListJobs(context.Background(), ListSectionSyncJobsQuery{GameID: "100001", Page: 0, PageSize: 99999, Status: "succeeded"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if repo.lastList.page != 1 {
		t.Errorf("page<=0 must clamp to 1, got %d", repo.lastList.page)
	}
	if repo.lastList.pageSize != 100 {
		t.Errorf("pageSize must clamp to 100, got %d", repo.lastList.pageSize)
	}
	if repo.lastList.status != "succeeded" {
		t.Errorf("status filter passthrough, got %q", repo.lastList.status)
	}
}

func TestListJobsRequiresGameID(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, &fakeAuditSink{})
	_, err := svc.ListJobs(context.Background(), ListSectionSyncJobsQuery{GameID: "  "})
	if se := asSyncErr(t, err); se.Code != CodeValidation {
		t.Fatalf("empty gameID want VALIDATION_FAILED, got %s", se.Code)
	}
}
