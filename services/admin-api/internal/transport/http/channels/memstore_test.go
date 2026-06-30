package channels

import (
	"context"
	"sort"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	channelapp "github.com/csw/console/services/admin-api/internal/app/channel"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// memChannelState 是 channel 模块的内存数据快照，仅用于进程内 httptest 全链路覆盖
// （transport -> app -> domain），不依赖真实 PG。InTx 通过克隆/回填实现真实回滚语义，
// 等价覆盖 S10（跨聚合写中途失败整体回滚、无 id 占用）。
// 平台级渠道主数据（channels/policies）为只读种子；env 由 schema 决定（本层不建模，S6 N/A）。
type memChannelState struct {
	games        map[string]int64 // 对外 game_id -> games.id（内部）
	channelByID  map[string]domainchannel.ChannelWithPolicy
	channels     []domainchannel.ChannelWithPolicy // 候选列表（按 sort 升序）
	gameChannels map[int64]*domainchannel.GameMarketChannel
	packages     map[int64]*domainchannel.ChannelPackage
	gcSeq        int64
	pkgSeq       int64
}

func newMemChannelState() *memChannelState {
	st := &memChannelState{
		games:        map[string]int64{},
		channelByID:  map[string]domainchannel.ChannelWithPolicy{},
		gameChannels: map[int64]*domainchannel.GameMarketChannel{},
		packages:     map[int64]*domainchannel.ChannelPackage{},
	}
	// 种子：两个游戏。
	st.games["100001"] = 1
	st.games["100002"] = 2
	// 种子：平台渠道主数据 + 策略（google/apple 海外；wechat 国内）。
	seed := []domainchannel.ChannelWithPolicy{
		{
			Channel: domainchannel.Channel{ID: 11, ChannelID: "google", ChannelName: "Google Play", ChannelType: domainchannel.ChannelTypeStore, Region: domainchannel.ChannelRegionOverseas, Enabled: true, Sort: 1},
			Policy:  domainchannel.ChannelPolicy{ChannelIDRef: 11, LoginMode: common.LoginModeChannelOnly, PaymentMode: common.PaymentModeChannelOnly},
		},
		{
			Channel: domainchannel.Channel{ID: 12, ChannelID: "apple", ChannelName: "App Store", ChannelType: domainchannel.ChannelTypeStore, Region: domainchannel.ChannelRegionOverseas, Enabled: true, Sort: 2},
			Policy:  domainchannel.ChannelPolicy{ChannelIDRef: 12, LoginMode: common.LoginModeAccountSystem, PaymentMode: common.PaymentModeHybrid},
		},
		{
			Channel: domainchannel.Channel{ID: 13, ChannelID: "wechat", ChannelName: "WeChat", ChannelType: domainchannel.ChannelTypeStore, Region: domainchannel.ChannelRegionDomestic, Enabled: true, Sort: 3},
			Policy:  domainchannel.ChannelPolicy{ChannelIDRef: 13, LoginMode: common.LoginModeChannelOnly, PaymentMode: common.PaymentModeChannelOnly, LoginLocked: true, PaymentLocked: true},
		},
	}
	for _, c := range seed {
		st.channels = append(st.channels, c)
		st.channelByID[c.Channel.ChannelID] = c
	}
	return st
}

func cloneGMC(g *domainchannel.GameMarketChannel) *domainchannel.GameMarketChannel {
	cp := *g
	if g.HiddenAt != nil {
		t := *g.HiddenAt
		cp.HiddenAt = &t
	}
	if g.LastCheckAt != nil {
		t := *g.LastCheckAt
		cp.LastCheckAt = &t
	}
	cp.NormalConfig = cloneAnyMap(g.NormalConfig)
	cp.SecretConfig = cloneStrMap(g.SecretConfig)
	cp.FileConfig = cloneStrMap(g.FileConfig)
	return &cp
}

func clonePkg(p *domainchannel.ChannelPackage) *domainchannel.ChannelPackage {
	cp := *p
	cp.OverrideJSON = cloneAnyMap(p.OverrideJSON)
	return &cp
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStrMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// clone 深拷贝可变部分（gameChannels/packages/games + 序号），渠道主数据为只读直接共享。
func (s *memChannelState) clone() *memChannelState {
	c := &memChannelState{
		games:        map[string]int64{},
		channelByID:  s.channelByID,
		channels:     s.channels,
		gameChannels: map[int64]*domainchannel.GameMarketChannel{},
		packages:     map[int64]*domainchannel.ChannelPackage{},
		gcSeq:        s.gcSeq,
		pkgSeq:       s.pkgSeq,
	}
	for k, v := range s.games {
		c.games[k] = v
	}
	for k, v := range s.gameChannels {
		c.gameChannels[k] = cloneGMC(v)
	}
	for k, v := range s.packages {
		c.packages[k] = clonePkg(v)
	}
	return c
}

// seedInstance 直接落一条渠道实例到当前状态（绕过 API），用于无法经 API 构造的前置态
// （如 config_status=valid 才能隐藏）。region 取自渠道主数据种子。
func (s *memChannelState) seedInstance(gameID, market, channelID string, status common.ConfigStatus) int64 {
	c := s.channelByID[channelID]
	s.gcSeq++
	now := time.Now()
	inst := &domainchannel.GameMarketChannel{
		ID:           s.gcSeq,
		GameIDRef:    s.games[gameID],
		GameID:       gameID,
		Market:       market,
		ChannelIDRef: c.Channel.ID,
		ChannelID:    channelID,
		Region:       c.Channel.Region,
		Enabled:      true,
		ConfigStatus: status,
		NormalConfig: map[string]any{},
		SecretConfig: map[string]string{},
		FileConfig:   map[string]string{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.gameChannels[inst.ID] = inst
	return inst.ID
}

// seedPackage 直接落一条渠道包，用于 package code 冲突/更新前置态。
func (s *memChannelState) seedPackage(gameChannelID int64, code, name, market string) int64 {
	s.pkgSeq++
	now := time.Now()
	pkg := &domainchannel.ChannelPackage{
		ID:                   s.pkgSeq,
		GameChannelIDRef:     gameChannelID,
		PackageCode:          code,
		PackageName:          name,
		MarketCode:           market,
		InheritChannelConfig: true,
		Enabled:              true,
		OverrideJSON:         map[string]any{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	s.packages[pkg.ID] = pkg
	return pkg.ID
}

// memChannelStore 实现 channelapp.TxManager。
type memChannelStore struct{ state *memChannelState }

func newMemChannelStore() *memChannelStore { return &memChannelStore{state: newMemChannelState()} }

func (s *memChannelStore) repos(st *memChannelState) channelapp.Repositories {
	return channelapp.Repositories{
		Channels:     &memChannelRepo{st},
		GameChannels: &memGameChannelRepo{st},
		Packages:     &memPackageRepo{st},
	}
}

func (s *memChannelStore) Repositories() channelapp.Repositories { return s.repos(s.state) }

func (s *memChannelStore) InTx(ctx context.Context, fn func(channelapp.Repositories) error) error {
	clone := s.state.clone()
	if err := fn(s.repos(clone)); err != nil {
		return err // 丢弃 clone = 回滚
	}
	s.state = clone
	return nil
}

// ===== 渠道主数据只读仓储 =====

type memChannelRepo struct{ st *memChannelState }

func (r *memChannelRepo) ListChannelsWithPolicy(_ context.Context) ([]domainchannel.ChannelWithPolicy, error) {
	out := append([]domainchannel.ChannelWithPolicy(nil), r.st.channels...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Channel.Sort < out[j].Channel.Sort })
	return out, nil
}

func (r *memChannelRepo) GetChannelByChannelID(_ context.Context, channelID string) (domainchannel.ChannelWithPolicy, error) {
	if c, ok := r.st.channelByID[channelID]; ok {
		return c, nil
	}
	return domainchannel.ChannelWithPolicy{}, adminapp.ErrNotFound
}

// ===== 渠道实例仓储 =====

type memGameChannelRepo struct{ st *memChannelState }

func (r *memGameChannelRepo) ResolveGameRowID(_ context.Context, gameID string) (int64, error) {
	if id, ok := r.st.games[gameID]; ok {
		return id, nil
	}
	return 0, adminapp.ErrNotFound
}

func (r *memGameChannelRepo) ExistsInstance(_ context.Context, gameIDRef int64, market, channelID string) (bool, error) {
	for _, g := range r.st.gameChannels {
		if g.GameIDRef == gameIDRef && g.Market == market && g.ChannelID == channelID {
			return true, nil
		}
	}
	return false, nil
}

func (r *memGameChannelRepo) FindInstance(_ context.Context, gameIDRef int64, market, channelID string) (domainchannel.GameMarketChannel, error) {
	for _, g := range r.st.gameChannels {
		if g.GameIDRef == gameIDRef && g.Market == market && g.ChannelID == channelID {
			return *cloneGMC(g), nil
		}
	}
	return domainchannel.GameMarketChannel{}, adminapp.ErrNotFound
}

func (r *memGameChannelRepo) Insert(_ context.Context, inst domainchannel.GameMarketChannel) (domainchannel.GameMarketChannel, error) {
	r.st.gcSeq++
	inst.ID = r.st.gcSeq
	now := time.Now()
	inst.CreatedAt, inst.UpdatedAt = now, now
	r.st.gameChannels[inst.ID] = cloneGMC(&inst)
	return inst, nil
}

func (r *memGameChannelRepo) GetByID(_ context.Context, id int64) (domainchannel.GameMarketChannel, error) {
	g, ok := r.st.gameChannels[id]
	if !ok {
		return domainchannel.GameMarketChannel{}, adminapp.ErrNotFound
	}
	out := *cloneGMC(g)
	// region 镜像真实仓储的 JOIN channels 实时取出。
	if c, ok := r.st.channelByID[out.ChannelID]; ok {
		out.Region = c.Channel.Region
	}
	return out, nil
}

func (r *memGameChannelRepo) List(_ context.Context, q dto.ListMarketChannelsQuery) ([]domainchannel.GameMarketChannel, int, error) {
	gameIDRef, ok := r.st.games[q.GameID]
	if !ok {
		return nil, 0, nil
	}
	matched := make([]*domainchannel.GameMarketChannel, 0, len(r.st.gameChannels))
	for _, g := range r.st.gameChannels {
		if g.GameIDRef != gameIDRef {
			continue
		}
		if q.Market != "" && !strings.EqualFold(q.Market, "ALL") && g.Market != q.Market {
			continue
		}
		if q.ChannelID != "" && g.ChannelID != q.ChannelID {
			continue
		}
		if q.ConfigStatus != "" && string(g.ConfigStatus) != q.ConfigStatus {
			continue
		}
		if !q.Hidden && g.Hidden {
			continue
		}
		if q.Compatible != nil {
			compat := common.Market(g.Market).IsCN() == (g.Region == domainchannel.ChannelRegionDomestic)
			if compat != *q.Compatible {
				continue
			}
		}
		matched = append(matched, g)
	}
	// 默认 -updatedAt，同时间退化为 id desc，保证分页稳定。
	sort.Slice(matched, func(i, j int) bool {
		if !matched[i].UpdatedAt.Equal(matched[j].UpdatedAt) {
			return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
		}
		return matched[i].ID > matched[j].ID
	})
	total := len(matched)

	page, pageSize := q.Page, q.PageSize
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
	out := make([]domainchannel.GameMarketChannel, 0, end-start)
	for _, g := range matched[start:end] {
		item := *cloneGMC(g)
		if c, ok := r.st.channelByID[item.ChannelID]; ok {
			item.Region = c.Channel.Region
		}
		out = append(out, item)
	}
	return out, total, nil
}

func (r *memGameChannelRepo) UpdateBasics(_ context.Context, id int64, patch channelapp.GameChannelPatch) error {
	g, ok := r.st.gameChannels[id]
	if !ok {
		return adminapp.ErrNotFound
	}
	if patch.Enabled != nil {
		g.Enabled = *patch.Enabled
	}
	if patch.Remark != nil {
		g.Remark = *patch.Remark
	}
	g.UpdatedAt = time.Now()
	return nil
}

func (r *memGameChannelRepo) Hide(_ context.Context, id int64, by string, at time.Time) error {
	g, ok := r.st.gameChannels[id]
	if !ok {
		return adminapp.ErrNotFound
	}
	g.Hide(by, at)
	g.UpdatedAt = time.Now()
	return nil
}

func (r *memGameChannelRepo) Unhide(_ context.Context, id int64) error {
	g, ok := r.st.gameChannels[id]
	if !ok {
		return adminapp.ErrNotFound
	}
	g.Unhide()
	g.UpdatedAt = time.Now()
	return nil
}

// ===== 渠道包仓储 =====

type memPackageRepo struct{ st *memChannelState }

func (r *memPackageRepo) ListByGameChannel(_ context.Context, gameChannelID int64) ([]domainchannel.ChannelPackage, error) {
	out := []domainchannel.ChannelPackage{}
	for _, p := range r.st.packages {
		if p.GameChannelIDRef == gameChannelID {
			out = append(out, *clonePkg(p))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *memPackageRepo) ExistsPackageCode(_ context.Context, gameChannelID int64, code string) (bool, error) {
	for _, p := range r.st.packages {
		if p.GameChannelIDRef == gameChannelID && p.PackageCode == code {
			return true, nil
		}
	}
	return false, nil
}

func (r *memPackageRepo) InsertPackage(_ context.Context, pkg domainchannel.ChannelPackage) (domainchannel.ChannelPackage, error) {
	r.st.pkgSeq++
	pkg.ID = r.st.pkgSeq
	now := time.Now()
	pkg.CreatedAt, pkg.UpdatedAt = now, now
	r.st.packages[pkg.ID] = clonePkg(&pkg)
	return pkg, nil
}

func (r *memPackageRepo) GetPackageByID(_ context.Context, id int64) (domainchannel.ChannelPackage, error) {
	p, ok := r.st.packages[id]
	if !ok {
		return domainchannel.ChannelPackage{}, adminapp.ErrNotFound
	}
	return *clonePkg(p), nil
}

func (r *memPackageRepo) UpdatePackage(_ context.Context, id int64, patch channelapp.PackagePatch) error {
	p, ok := r.st.packages[id]
	if !ok {
		return adminapp.ErrNotFound
	}
	if patch.PackageName != nil {
		p.PackageName = *patch.PackageName
	}
	if patch.BundleID != nil {
		p.BundleID = *patch.BundleID
	}
	if patch.InheritChannelConfig != nil {
		p.InheritChannelConfig = *patch.InheritChannelConfig
	}
	if patch.Enabled != nil {
		p.Enabled = *patch.Enabled
	}
	if patch.OverrideJSON != nil {
		p.OverrideJSON = cloneAnyMap(patch.OverrideJSON)
	}
	p.UpdatedAt = time.Now()
	return nil
}

// ===== fakeAudit =====

// fakeAudit 记录审计调用，供 S7 断言（与 game httptest 同口径：用 spy sink 断言 service 层审计写入）。
type fakeAudit struct{ entries []channelapp.AuditEntry }

func (a *fakeAudit) Write(_ context.Context, e channelapp.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func (a *fakeAudit) byAction(action string) (channelapp.AuditEntry, bool) {
	for _, e := range a.entries {
		if e.Action == action {
			return e, true
		}
	}
	return channelapp.AuditEntry{}, false
}
