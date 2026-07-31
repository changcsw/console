package platformchannel

import (
	"context"
	"slices"
	"sort"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	platformchannelapp "github.com/csw/console/services/admin-api/internal/app/platformchannel"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// memState 是平台渠道 + 渠道模版的内存快照，仅用于进程内 httptest 全链路覆盖
// （transport -> app -> domain），不依赖真实 PG。InTx 通过克隆/回填实现真实回滚语义。
// 这些表位于共享 schema platform，与 env 无关，因此本层不建模 env 维度。
type memState struct {
	channels  map[string]*channelRow
	templates map[domainchannel.ChannelTemplateKind]map[int64]*domainchannel.ChannelTemplate
	chSeq     int64
	tplSeq    int64
}

type channelRow struct {
	channel   domainchannel.Channel
	policy    domainchannel.ChannelPolicy
	updatedAt time.Time
}

func newMemState() *memState {
	st := &memState{
		channels: map[string]*channelRow{},
		templates: map[domainchannel.ChannelTemplateKind]map[int64]*domainchannel.ChannelTemplate{
			domainchannel.ChannelTemplateKindLogin: {},
			domainchannel.ChannelTemplateKindIAP:   {},
		},
	}
	st.seedChannel("google", "Google Play", domainchannel.ChannelTypeStore, domainchannel.ChannelRegionGlobal, 1,
		common.LoginModeChannelOnly, common.PaymentModeChannelOnly)
	st.seedChannel("huawei_cn", "华为", domainchannel.ChannelTypeDomestic, domainchannel.ChannelRegionCN, 2,
		common.LoginModeChannelOnly, common.PaymentModeChannelOnly)

	st.seedTemplate("huawei_cn", domainchannel.ChannelTemplateKindLogin, "v1", true)
	st.seedTemplate("huawei_cn", domainchannel.ChannelTemplateKindLogin, "v2", false)
	return st
}

func (s *memState) seedChannel(
	channelID, name, channelType string, region domainchannel.ChannelRegion, sort int,
	loginMode common.LoginMode, paymentMode common.PaymentMode,
) {
	s.chSeq++
	s.channels[channelID] = &channelRow{
		channel: domainchannel.Channel{
			ID: s.chSeq, ChannelID: channelID, ChannelName: name,
			ChannelType: channelType, Region: region, Enabled: true, Sort: sort,
		},
		policy: domainchannel.ChannelPolicy{
			ChannelIDRef: s.chSeq, LoginMode: loginMode, PaymentMode: paymentMode,
		},
		updatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func (s *memState) seedTemplate(channelID string, kind domainchannel.ChannelTemplateKind, version string, enabled bool) {
	s.tplSeq++
	row := s.channels[channelID]
	s.templates[kind][s.tplSeq] = &domainchannel.ChannelTemplate{
		ID: s.tplSeq, Kind: kind, ChannelIDRef: row.channel.ID, ChannelID: channelID,
		TemplateVersion: version,
		FormSchema: []domainchannel.ChannelLoginFormField{
			{Key: "appId", Label: "App ID", Component: "input", Required: true, Order: 10},
		},
		SecretFields:    []string{},
		FileFields:      []domainchannel.ChannelLoginFileField{},
		ValidationRules: map[string]domainchannel.ChannelLoginValidationRule{},
		Enabled:         enabled,
	}
}

func (s *memState) clone() *memState {
	out := &memState{
		channels: map[string]*channelRow{},
		templates: map[domainchannel.ChannelTemplateKind]map[int64]*domainchannel.ChannelTemplate{
			domainchannel.ChannelTemplateKindLogin: {},
			domainchannel.ChannelTemplateKindIAP:   {},
		},
		chSeq:  s.chSeq,
		tplSeq: s.tplSeq,
	}
	for k, v := range s.channels {
		cp := *v
		out.channels[k] = &cp
	}
	for kind, byID := range s.templates {
		for id, tpl := range byID {
			cp := *tpl
			cp.FormSchema = slices.Clone(tpl.FormSchema)
			cp.SecretFields = slices.Clone(tpl.SecretFields)
			cp.FileFields = slices.Clone(tpl.FileFields)
			cp.ValidationRules = map[string]domainchannel.ChannelLoginValidationRule{}
			for rk, rv := range tpl.ValidationRules {
				cp.ValidationRules[rk] = rv
			}
			out.templates[kind][id] = &cp
		}
	}
	return out
}

func (s *memState) replaceWith(next *memState) {
	s.channels = next.channels
	s.templates = next.templates
	s.chSeq = next.chSeq
	s.tplSeq = next.tplSeq
}

// memStore 实现 platformchannelapp.TxManager。
type memStore struct{ state *memState }

func newMemStore() *memStore { return &memStore{state: newMemState()} }

func (m *memStore) Repositories() platformchannelapp.Repositories {
	return platformchannelapp.Repositories{
		Channels:  &memChannelRepo{state: m.state},
		Templates: &memTemplateRepo{state: m.state},
	}
}

func (m *memStore) InTx(ctx context.Context, fn func(platformchannelapp.Repositories) error) error {
	snapshot := m.state.clone()
	repos := platformchannelapp.Repositories{
		Channels:  &memChannelRepo{state: m.state},
		Templates: &memTemplateRepo{state: m.state},
	}
	if err := fn(repos); err != nil {
		m.state.replaceWith(snapshot) // 回滚
		return err
	}
	return nil
}

type memChannelRepo struct{ state *memState }

func (r *memChannelRepo) List(_ context.Context, q dto.ListPlatformChannelsQuery) ([]platformchannelapp.PlatformChannelRow, int, error) {
	rows := []platformchannelapp.PlatformChannelRow{}
	for _, row := range r.state.channels {
		if kw := strings.ToLower(strings.TrimSpace(q.Keyword)); kw != "" {
			if !strings.Contains(strings.ToLower(row.channel.ChannelID), kw) &&
				!strings.Contains(strings.ToLower(row.channel.ChannelName), kw) {
				continue
			}
		}
		if q.Region != "" && string(row.channel.Region) != q.Region {
			continue
		}
		if q.ChannelType != "" && row.channel.ChannelType != q.ChannelType {
			continue
		}
		if q.Enabled != nil && row.channel.Enabled != *q.Enabled {
			continue
		}
		rows = append(rows, r.toRow(row))
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Channel.Sort != rows[j].Channel.Sort {
			return rows[i].Channel.Sort < rows[j].Channel.Sort
		}
		return rows[i].Channel.ID < rows[j].Channel.ID
	})
	total := len(rows)
	start := (q.Page - 1) * q.PageSize
	if start > total {
		start = total
	}
	end := start + q.PageSize
	if end > total {
		end = total
	}
	return rows[start:end], total, nil
}

func (r *memChannelRepo) GetByChannelID(_ context.Context, channelID string) (platformchannelapp.PlatformChannelRow, error) {
	row, ok := r.state.channels[channelID]
	if !ok {
		return platformchannelapp.PlatformChannelRow{}, adminapp.ErrNotFound
	}
	return r.toRow(row), nil
}

func (r *memChannelRepo) Insert(_ context.Context, ch domainchannel.Channel, policy domainchannel.ChannelPolicy) error {
	if _, ok := r.state.channels[ch.ChannelID]; ok {
		return adminapp.ErrConflict
	}
	r.state.chSeq++
	ch.ID = r.state.chSeq
	policy.ChannelIDRef = ch.ID
	r.state.channels[ch.ChannelID] = &channelRow{channel: ch, policy: policy, updatedAt: time.Now().UTC()}
	return nil
}

func (r *memChannelRepo) UpdateMaster(_ context.Context, channelID string, patch platformchannelapp.ChannelMasterPatch) error {
	row, ok := r.state.channels[channelID]
	if !ok {
		return adminapp.ErrNotFound
	}
	if patch.ChannelName != nil {
		row.channel.ChannelName = *patch.ChannelName
	}
	if patch.Enabled != nil {
		row.channel.Enabled = *patch.Enabled
	}
	if patch.Sort != nil {
		row.channel.Sort = *patch.Sort
	}
	row.updatedAt = time.Now().UTC()
	return nil
}

func (r *memChannelRepo) UpsertPolicy(_ context.Context, channelID string, patch platformchannelapp.ChannelPolicyPatch) error {
	row, ok := r.state.channels[channelID]
	if !ok {
		return adminapp.ErrNotFound
	}
	if patch.LoginMode != nil {
		row.policy.LoginMode = common.LoginMode(*patch.LoginMode)
	}
	if patch.PaymentMode != nil {
		row.policy.PaymentMode = common.PaymentMode(*patch.PaymentMode)
	}
	if patch.LoginLocked != nil {
		row.policy.LoginLocked = *patch.LoginLocked
	}
	if patch.PaymentLocked != nil {
		row.policy.PaymentLocked = *patch.PaymentLocked
	}
	row.updatedAt = time.Now().UTC()
	return nil
}

func (r *memChannelRepo) toRow(row *channelRow) platformchannelapp.PlatformChannelRow {
	count := func(kind domainchannel.ChannelTemplateKind) int {
		n := 0
		for _, tpl := range r.state.templates[kind] {
			if tpl.ChannelIDRef == row.channel.ID {
				n++
			}
		}
		return n
	}
	return platformchannelapp.PlatformChannelRow{
		Channel:            row.channel,
		Policy:             row.policy,
		LoginTemplateCount: count(domainchannel.ChannelTemplateKindLogin),
		IAPTemplateCount:   count(domainchannel.ChannelTemplateKindIAP),
		UpdatedAt:          row.updatedAt,
	}
}

type memTemplateRepo struct{ state *memState }

func (r *memTemplateRepo) ListByChannel(
	_ context.Context, kind domainchannel.ChannelTemplateKind, channelIDRef int64,
) ([]domainchannel.ChannelTemplate, error) {
	out := []domainchannel.ChannelTemplate{}
	for _, tpl := range r.state.templates[kind] {
		if tpl.ChannelIDRef == channelIDRef {
			out = append(out, *tpl)
		}
	}
	// 与 SQL 一致：按 template_version 降序（运行时取 enabled 的最新版本）。
	sort.Slice(out, func(i, j int) bool {
		if out[i].TemplateVersion != out[j].TemplateVersion {
			return out[i].TemplateVersion > out[j].TemplateVersion
		}
		return out[i].ID > out[j].ID
	})
	return out, nil
}

func (r *memTemplateRepo) GetByID(
	_ context.Context, kind domainchannel.ChannelTemplateKind, id int64,
) (domainchannel.ChannelTemplate, error) {
	tpl, ok := r.state.templates[kind][id]
	if !ok {
		return domainchannel.ChannelTemplate{}, adminapp.ErrNotFound
	}
	return *tpl, nil
}

func (r *memTemplateRepo) Insert(_ context.Context, tpl domainchannel.ChannelTemplate) (domainchannel.ChannelTemplate, error) {
	for _, existing := range r.state.templates[tpl.Kind] {
		if existing.ChannelIDRef == tpl.ChannelIDRef && existing.TemplateVersion == tpl.TemplateVersion {
			return domainchannel.ChannelTemplate{}, adminapp.ErrConflict
		}
	}
	r.state.tplSeq++
	tpl.ID = r.state.tplSeq
	tpl.CreatedAt = time.Now().UTC()
	tpl.UpdatedAt = tpl.CreatedAt
	cp := tpl
	r.state.templates[tpl.Kind][tpl.ID] = &cp
	return tpl, nil
}

func (r *memTemplateRepo) Replace(_ context.Context, tpl domainchannel.ChannelTemplate) error {
	current, ok := r.state.templates[tpl.Kind][tpl.ID]
	if !ok {
		return adminapp.ErrNotFound
	}
	current.FormSchema = slices.Clone(tpl.FormSchema)
	current.SecretFields = slices.Clone(tpl.SecretFields)
	current.FileFields = slices.Clone(tpl.FileFields)
	current.ValidationRules = tpl.ValidationRules
	current.Enabled = tpl.Enabled
	current.UpdatedAt = time.Now().UTC()
	return nil
}

// fakeAudit 记录审计调用，供审计断言使用（与 channels httptest 同口径）。
type fakeAudit struct{ entries []platformchannelapp.AuditEntry }

func (a *fakeAudit) Write(_ context.Context, e platformchannelapp.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func (a *fakeAudit) byAction(action string) (platformchannelapp.AuditEntry, bool) {
	for _, e := range a.entries {
		if e.Action == action {
			return e, true
		}
	}
	return platformchannelapp.AuditEntry{}, false
}
