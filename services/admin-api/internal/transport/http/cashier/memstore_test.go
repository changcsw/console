package cashier

import (
	"context"
	"sort"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	cashierapp "github.com/csw/console/services/admin-api/internal/app/cashier"
	domaincashier "github.com/csw/console/services/admin-api/internal/domain/cashier"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// memState 是 cashier 模块的内存数据快照，仅用于进程内 httptest 全链路覆盖
// （transport -> app -> domain），不依赖真实 PG。InTx 通过克隆/回填实现真实回滚语义，
// 等价覆盖 S10（多表写中途失败整体回滚）。
type memState struct {
	templates map[string]*domaincashier.PriceTemplate // 按对外 templateId
	tplIDToID map[int64]string                        // 内部主键 -> templateId
	seqTpl    int64

	versions map[int64]*domaincashier.TemplateVersionRecord
	seqVer   int64

	rows   map[int64]*domaincashier.PriceRow
	seqRow int64

	runs   map[int64]*domaincashier.FXSyncRun
	seqRun int64

	specs map[string]common.CurrencySpec

	games          map[string]int64
	gameProfiles   map[int64]*domaincashier.GameCashierProfile
	gameOverrides  map[int64][]domaincashier.GameCashierPriceOverride
	seqGameProfile int64
	seqGameOv      int64

	// forcePublishErr 故障注入：非 nil 时 PublishVersion 返回该错误，
	// 用于验证发布/审核同事务在第二步失败时整体回滚（S10）。
	forcePublishErr error
}

func newMemState() *memState {
	return &memState{
		templates:     map[string]*domaincashier.PriceTemplate{},
		tplIDToID:     map[int64]string{},
		versions:      map[int64]*domaincashier.TemplateVersionRecord{},
		rows:          map[int64]*domaincashier.PriceRow{},
		runs:          map[int64]*domaincashier.FXSyncRun{},
		specs:         map[string]common.CurrencySpec{},
		games:         map[string]int64{"100001": 1},
		gameProfiles:  map[int64]*domaincashier.GameCashierProfile{},
		gameOverrides: map[int64][]domaincashier.GameCashierPriceOverride{},
	}
}

func (s *memState) clone() *memState {
	c := newMemState()
	for k, v := range s.templates {
		cp := *v
		c.templates[k] = &cp
	}
	for k, v := range s.tplIDToID {
		c.tplIDToID[k] = v
	}
	for k, v := range s.versions {
		cp := *v
		c.versions[k] = &cp
	}
	for k, v := range s.rows {
		cp := *v
		c.rows[k] = &cp
	}
	for k, v := range s.runs {
		cp := *v
		cp.DiffSummaryJSON = cloneMap(v.DiffSummaryJSON)
		c.runs[k] = &cp
	}
	for k, v := range s.specs {
		c.specs[k] = v
	}
	for k, v := range s.games {
		c.games[k] = v
	}
	for k, v := range s.gameProfiles {
		cp := *v
		c.gameProfiles[k] = &cp
	}
	for k, rows := range s.gameOverrides {
		cp := make([]domaincashier.GameCashierPriceOverride, len(rows))
		copy(cp, rows)
		c.gameOverrides[k] = cp
	}
	c.seqTpl, c.seqVer, c.seqRow, c.seqRun = s.seqTpl, s.seqVer, s.seqRow, s.seqRun
	c.seqGameProfile, c.seqGameOv = s.seqGameProfile, s.seqGameOv
	c.forcePublishErr = s.forcePublishErr
	return c
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// memStore 实现 cashierapp.TxManager。
type memStore struct{ state *memState }

func newMemStore() *memStore {
	st := newMemState()
	// 默认 currency_specs（platform schema 模拟）。
	st.specs["USD"] = common.CurrencySpec{CurrencyCode: "USD", DecimalPlaces: 2, MinAmountMinor: 50, RoundingMode: common.RoundingHalfUp, Enabled: true}
	st.specs["JPY"] = common.CurrencySpec{CurrencyCode: "JPY", DecimalPlaces: 0, MinAmountMinor: 1, RoundingMode: common.RoundingHalfUp, Enabled: true}
	return &memStore{state: st}
}

func (s *memStore) Repository() cashierapp.CashierTemplateRepository { return &memRepo{s.state} }

func (s *memStore) InTx(ctx context.Context, fn func(cashierapp.CashierTemplateRepository) error) error {
	clone := s.state.clone()
	if err := fn(&memRepo{clone}); err != nil {
		return err // 丢弃 clone = 回滚
	}
	s.state = clone
	return nil
}

// memRepo 实现 cashierapp.CashierTemplateRepository。
type memRepo struct{ st *memState }

func (r *memRepo) ListTemplates(_ context.Context, page, pageSize int) ([]domaincashier.PriceTemplate, int, error) {
	all := make([]domaincashier.PriceTemplate, 0, len(r.st.templates))
	for _, t := range r.st.templates {
		all = append(all, *t)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := len(all)
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (r *memRepo) CreateTemplate(_ context.Context, item domaincashier.PriceTemplate) (domaincashier.PriceTemplate, error) {
	if _, ok := r.st.templates[item.TemplateID]; ok {
		return domaincashier.PriceTemplate{}, adminapp.ErrConflict
	}
	r.st.seqTpl++
	item.ID = r.st.seqTpl
	cp := item
	r.st.templates[item.TemplateID] = &cp
	r.st.tplIDToID[item.ID] = item.TemplateID
	return cp, nil
}

func (r *memRepo) GetTemplateByTemplateID(_ context.Context, templateID string) (domaincashier.PriceTemplate, error) {
	t, ok := r.st.templates[templateID]
	if !ok {
		return domaincashier.PriceTemplate{}, adminapp.ErrNotFound
	}
	return *t, nil
}

func (r *memRepo) GetTemplateByID(_ context.Context, templateRowID int64) (domaincashier.PriceTemplate, error) {
	tid, ok := r.st.tplIDToID[templateRowID]
	if !ok {
		return domaincashier.PriceTemplate{}, adminapp.ErrNotFound
	}
	return *r.st.templates[tid], nil
}

func (r *memRepo) ListVersions(_ context.Context, templateIDRef int64) ([]domaincashier.TemplateVersionRecord, error) {
	out := make([]domaincashier.TemplateVersionRecord, 0)
	for _, v := range r.st.versions {
		if v.TemplateIDRef == templateIDRef {
			out = append(out, *v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func (r *memRepo) GetVersionByTemplateAndVersion(_ context.Context, templateIDRef int64, version int) (domaincashier.TemplateVersionRecord, error) {
	for _, v := range r.st.versions {
		if v.TemplateIDRef == templateIDRef && v.Version == version {
			return *v, nil
		}
	}
	return domaincashier.TemplateVersionRecord{}, adminapp.ErrNotFound
}

func (r *memRepo) GetVersionByID(_ context.Context, id int64) (domaincashier.TemplateVersionRecord, error) {
	v, ok := r.st.versions[id]
	if !ok {
		return domaincashier.TemplateVersionRecord{}, adminapp.ErrNotFound
	}
	return *v, nil
}

func (r *memRepo) GetPublishedVersion(_ context.Context, templateIDRef int64) (*domaincashier.TemplateVersionRecord, error) {
	for _, v := range r.st.versions {
		if v.TemplateIDRef == templateIDRef && v.Status == domaincashier.StatusPublished {
			cp := *v
			return &cp, nil
		}
	}
	return nil, nil
}

func (r *memRepo) NextVersion(_ context.Context, templateIDRef int64) (int, error) {
	max := 0
	for _, v := range r.st.versions {
		if v.TemplateIDRef == templateIDRef && v.Version > max {
			max = v.Version
		}
	}
	return max + 1, nil
}

func (r *memRepo) CreateVersion(_ context.Context, version domaincashier.TemplateVersionRecord) (domaincashier.TemplateVersionRecord, error) {
	r.st.seqVer++
	version.ID = r.st.seqVer
	now := time.Now()
	version.CreatedAt, version.UpdatedAt = now, now
	if version.Status == "" {
		version.Status = domaincashier.StatusDraft
	}
	cp := version
	r.st.versions[version.ID] = &cp
	return cp, nil
}

func (r *memRepo) ArchiveVersion(_ context.Context, versionID int64, at time.Time) error {
	v, ok := r.st.versions[versionID]
	if !ok {
		return adminapp.ErrNotFound
	}
	v.Status = domaincashier.StatusArchived
	v.UpdatedAt = at
	return nil
}

func (r *memRepo) PublishVersion(_ context.Context, versionID int64, at time.Time, checksum string) error {
	if r.st.forcePublishErr != nil {
		return r.st.forcePublishErr
	}
	v, ok := r.st.versions[versionID]
	if !ok {
		return adminapp.ErrNotFound
	}
	v.Status = domaincashier.StatusPublished
	v.PublishedAt = &at
	v.UpdatedAt = at
	v.Checksum = checksum
	return nil
}

func (r *memRepo) ListRows(_ context.Context, versionID int64) ([]domaincashier.PriceRow, error) {
	out := make([]domaincashier.PriceRow, 0)
	for _, row := range r.st.rows {
		if row.TemplateVersionIDRef == versionID {
			out = append(out, *row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *memRepo) ReplaceRows(_ context.Context, versionID int64, rows []domaincashier.PriceRow) error {
	for id, row := range r.st.rows {
		if row.TemplateVersionIDRef == versionID {
			delete(r.st.rows, id)
		}
	}
	for _, row := range rows {
		r.st.seqRow++
		row.ID = r.st.seqRow
		row.TemplateVersionIDRef = versionID
		cp := row
		r.st.rows[row.ID] = &cp
	}
	return nil
}

func (r *memRepo) CopyRows(_ context.Context, sourceVersionID, targetVersionID int64) (int, error) {
	src := make([]domaincashier.PriceRow, 0)
	for _, row := range r.st.rows {
		if row.TemplateVersionIDRef == sourceVersionID {
			src = append(src, *row)
		}
	}
	sort.Slice(src, func(i, j int) bool { return src[i].ID < src[j].ID })
	for _, row := range src {
		r.st.seqRow++
		row.ID = r.st.seqRow
		row.TemplateVersionIDRef = targetVersionID
		cp := row
		r.st.rows[row.ID] = &cp
	}
	return len(src), nil
}

func (r *memRepo) GetCurrencySpec(_ context.Context, currency string) (common.CurrencySpec, error) {
	spec, ok := r.st.specs[currency]
	if !ok || !spec.Enabled {
		return common.CurrencySpec{}, adminapp.ErrNotFound
	}
	return spec, nil
}

func (r *memRepo) CreateFXSyncRun(_ context.Context, run domaincashier.FXSyncRun) (domaincashier.FXSyncRun, error) {
	r.st.seqRun++
	run.ID = r.st.seqRun
	now := time.Now()
	if run.TriggeredAt.IsZero() {
		run.TriggeredAt = now
	}
	run.CreatedAt, run.UpdatedAt = now, now
	cp := run
	cp.DiffSummaryJSON = cloneMap(run.DiffSummaryJSON)
	r.st.runs[run.ID] = &cp
	return cp, nil
}

func (r *memRepo) GetFXSyncRun(_ context.Context, runID int64) (domaincashier.FXSyncRun, error) {
	run, ok := r.st.runs[runID]
	if !ok {
		return domaincashier.FXSyncRun{}, adminapp.ErrNotFound
	}
	return *run, nil
}

func (r *memRepo) ListFXSyncRuns(_ context.Context, templateIDRef int64) ([]domaincashier.FXSyncRun, error) {
	out := make([]domaincashier.FXSyncRun, 0)
	for _, run := range r.st.runs {
		if run.TemplateIDRef == templateIDRef {
			cp := *run
			cp.DiffSummaryJSON = cloneMap(run.DiffSummaryJSON)
			out = append(out, cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (r *memRepo) UpdateFXSyncRunReview(_ context.Context, runID int64, status domaincashier.FXRunStatus, reviewer int64, reviewedAt time.Time, note string) error {
	run, ok := r.st.runs[runID]
	if !ok {
		return adminapp.ErrNotFound
	}
	run.Status = status
	if reviewer != 0 {
		run.ReviewedBy = &reviewer
	}
	run.ReviewedAt = &reviewedAt
	run.ReviewNote = note
	run.UpdatedAt = reviewedAt
	return nil
}

func (r *memRepo) ResolveGameRowID(_ context.Context, gameID string) (int64, error) {
	id, ok := r.st.games[gameID]
	if !ok {
		return 0, adminapp.ErrNotFound
	}
	return id, nil
}

func (r *memRepo) GetGameCashierProfile(_ context.Context, gameIDRef int64) (domaincashier.GameCashierProfile, error) {
	p, ok := r.st.gameProfiles[gameIDRef]
	if !ok {
		return domaincashier.GameCashierProfile{}, adminapp.ErrNotFound
	}
	return *p, nil
}

func (r *memRepo) UpsertGameCashierProfile(_ context.Context, profile domaincashier.GameCashierProfile) (domaincashier.GameCashierProfile, error) {
	now := time.Now()
	if old, ok := r.st.gameProfiles[profile.GameIDRef]; ok {
		old.TemplateIDRef = profile.TemplateIDRef
		old.AppliedTemplateVersionID = profile.AppliedTemplateVersionID
		old.SnapshotChecksum = profile.SnapshotChecksum
		old.AppliedAt = profile.AppliedAt
		old.UpdatedAt = now
		return *old, nil
	}
	r.st.seqGameProfile++
	profile.ID = r.st.seqGameProfile
	profile.CreatedAt = now
	profile.UpdatedAt = now
	cp := profile
	r.st.gameProfiles[profile.GameIDRef] = &cp
	return cp, nil
}

func (r *memRepo) ListGameCashierPriceOverrides(_ context.Context, gameIDRef int64) ([]domaincashier.GameCashierPriceOverride, error) {
	rows := r.st.gameOverrides[gameIDRef]
	out := make([]domaincashier.GameCashierPriceOverride, len(rows))
	copy(out, rows)
	return out, nil
}

func (r *memRepo) ReplaceGameCashierPriceOverrides(_ context.Context, gameIDRef int64, rows []domaincashier.GameCashierPriceOverride) error {
	out := make([]domaincashier.GameCashierPriceOverride, 0, len(rows))
	now := time.Now()
	for _, row := range rows {
		r.st.seqGameOv++
		row.ID = r.st.seqGameOv
		row.GameIDRef = gameIDRef
		row.CreatedAt = now
		row.UpdatedAt = now
		out = append(out, row)
	}
	r.st.gameOverrides[gameIDRef] = out
	return nil
}

// fakeAudit 记录审计调用，供 S7/S8 断言。
type fakeAudit struct{ entries []cashierapp.AuditEntry }

func (a *fakeAudit) Write(_ context.Context, e cashierapp.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func (a *fakeAudit) byAction(action string) (cashierapp.AuditEntry, bool) {
	for _, e := range a.entries {
		if e.Action == action {
			return e, true
		}
	}
	return cashierapp.AuditEntry{}, false
}
