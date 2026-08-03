package featureplugin

import (
	"context"
	"slices"
	"sort"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	featurepluginapp "github.com/csw/console/services/admin-api/internal/app/featureplugin"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

// memState 是插件分类字典 + 插件主数据 + 参数模板的内存快照，仅用于进程内 httptest 全链路覆盖
// （transport -> app -> domain），不依赖真实 PG。InTx 通过克隆/回填实现真实回滚语义。
// 这些表位于共享 schema platform，与 env 无关，因此本层不建模 env 维度。
// gameRefs / channelRefs / packageOverrideRefs 模拟外部引用（game_channel_plugin_configs、
// platform.channel_feature_plugins、channel_package_plugin_overrides）：只影响插件能否删除
// （模板不阻断，随插件级联删除）。
type memState struct {
	categories          map[int64]*domainplugin.FeaturePluginCategory
	plugins             map[string]*domainplugin.FeaturePlugin
	templates           map[int64]*domainplugin.FeaturePluginTemplate
	gameRefs            map[int64]int
	channelRefs         map[int64]int
	packageOverrideRefs map[int64]int
	catSeq              int64
	pluginSeq           int64
	tplSeq              int64
}

func newMemState() *memState {
	st := &memState{
		categories:          map[int64]*domainplugin.FeaturePluginCategory{},
		plugins:             map[string]*domainplugin.FeaturePlugin{},
		templates:           map[int64]*domainplugin.FeaturePluginTemplate{},
		gameRefs:            map[int64]int{},
		channelRefs:         map[int64]int{},
		packageOverrideRefs: map[int64]int{},
	}
	loginCat := st.seedCategory("login", "登录类", 10, true)
	payCat := st.seedCategory("payment", "支付类", 20, true)
	st.seedCategory("ad", "广告类", 40, false)

	st.seedPlugin("realname", "实名认证", &loginCat, domainplugin.RegionDomestic, 1, true)
	st.seedPlugin("apple_pay", "Apple Pay", &payCat, domainplugin.RegionOverseas, 2, true)
	st.seedPlugin("customer_service", "客服", nil, domainplugin.RegionDomestic, 3, false)

	st.seedTemplate("realname", "v1", true)
	st.seedTemplate("realname", "v2", false)
	return st
}

func (s *memState) seedCategory(code, name string, sortOrder int, enabled bool) int64 {
	s.catSeq++
	s.categories[s.catSeq] = &domainplugin.FeaturePluginCategory{
		ID: s.catSeq, CategoryCode: code, CategoryName: name, Enabled: enabled, Sort: sortOrder,
		CreatedAt: seedTime, UpdatedAt: seedTime,
	}
	return s.catSeq
}

func (s *memState) seedPlugin(pluginID, name string, categoryID *int64, region string, sortOrder int, enabled bool) {
	s.pluginSeq++
	s.plugins[pluginID] = &domainplugin.FeaturePlugin{
		ID: s.pluginSeq, PluginID: pluginID, PluginName: name, CategoryIDRef: categoryID,
		Region: region, Enabled: enabled, Sort: sortOrder, CreatedAt: seedTime, UpdatedAt: seedTime,
	}
}

func (s *memState) seedTemplate(pluginID, version string, enabled bool) {
	s.tplSeq++
	p := s.plugins[pluginID]
	s.templates[s.tplSeq] = &domainplugin.FeaturePluginTemplate{
		ID: s.tplSeq, PluginIDRef: p.ID, PluginID: pluginID, TemplateVersion: version,
		FormSchema: []domainplugin.PluginFormField{
			{Key: "appId", Label: "App ID", Component: "input", Required: true, Order: 10},
		},
		SecretFields:    []string{},
		FileFields:      []domainplugin.PluginFileField{},
		ValidationRules: map[string]domainplugin.PluginValidationRule{},
		Enabled:         enabled,
		CreatedAt:       seedTime,
		UpdatedAt:       seedTime,
	}
}

var seedTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func (s *memState) clone() *memState {
	out := &memState{
		categories:          map[int64]*domainplugin.FeaturePluginCategory{},
		plugins:             map[string]*domainplugin.FeaturePlugin{},
		templates:           map[int64]*domainplugin.FeaturePluginTemplate{},
		gameRefs:            map[int64]int{},
		channelRefs:         map[int64]int{},
		packageOverrideRefs: map[int64]int{},
		catSeq:              s.catSeq,
		pluginSeq:           s.pluginSeq,
		tplSeq:              s.tplSeq,
	}
	for k, v := range s.categories {
		cp := *v
		out.categories[k] = &cp
	}
	for k, v := range s.plugins {
		cp := *v
		if v.CategoryIDRef != nil {
			id := *v.CategoryIDRef
			cp.CategoryIDRef = &id
		}
		out.plugins[k] = &cp
	}
	for k, v := range s.templates {
		cp := *v
		cp.FormSchema = slices.Clone(v.FormSchema)
		cp.SecretFields = slices.Clone(v.SecretFields)
		cp.FileFields = slices.Clone(v.FileFields)
		cp.ValidationRules = map[string]domainplugin.PluginValidationRule{}
		for rk, rv := range v.ValidationRules {
			cp.ValidationRules[rk] = rv
		}
		out.templates[k] = &cp
	}
	for k, v := range s.gameRefs {
		out.gameRefs[k] = v
	}
	for k, v := range s.channelRefs {
		out.channelRefs[k] = v
	}
	for k, v := range s.packageOverrideRefs {
		out.packageOverrideRefs[k] = v
	}
	return out
}

func (s *memState) replaceWith(next *memState) {
	s.categories = next.categories
	s.plugins = next.plugins
	s.templates = next.templates
	s.gameRefs = next.gameRefs
	s.channelRefs = next.channelRefs
	s.packageOverrideRefs = next.packageOverrideRefs
	s.catSeq = next.catSeq
	s.pluginSeq = next.pluginSeq
	s.tplSeq = next.tplSeq
}

// memStore 实现 featurepluginapp.TxManager。
type memStore struct{ state *memState }

func newMemStore() *memStore { return &memStore{state: newMemState()} }

func (m *memStore) repos() featurepluginapp.Repositories {
	return featurepluginapp.Repositories{
		Categories: &memCategoryRepo{state: m.state},
		Plugins:    &memPluginRepo{state: m.state},
		Templates:  &memTemplateRepo{state: m.state},
	}
}

func (m *memStore) Repositories() featurepluginapp.Repositories { return m.repos() }

func (m *memStore) InTx(_ context.Context, fn func(featurepluginapp.Repositories) error) error {
	snapshot := m.state.clone()
	if err := fn(m.repos()); err != nil {
		m.state.replaceWith(snapshot) // 回滚
		return err
	}
	return nil
}

// deleteFailingStore 让插件删除恒定失败（模板级联删除已经发生之后），
// 用于验证 DeletePlugin 的事务边界：失败时模板必须随事务回滚而复原。
type deleteFailingStore struct{ *memStore }

func (m *deleteFailingStore) failingRepos() featurepluginapp.Repositories {
	repos := m.memStore.repos()
	repos.Plugins = &failingDeletePluginRepo{FeaturePluginAdminRepository: repos.Plugins}
	return repos
}

func (m *deleteFailingStore) Repositories() featurepluginapp.Repositories { return m.failingRepos() }

func (m *deleteFailingStore) InTx(_ context.Context, fn func(featurepluginapp.Repositories) error) error {
	snapshot := m.state.clone()
	if err := fn(m.failingRepos()); err != nil {
		m.state.replaceWith(snapshot) // 回滚
		return err
	}
	return nil
}

type failingDeletePluginRepo struct {
	featurepluginapp.FeaturePluginAdminRepository
}

func (r *failingDeletePluginRepo) Delete(context.Context, int64) error { return adminapp.ErrNotFound }

type memCategoryRepo struct{ state *memState }

func (r *memCategoryRepo) List(_ context.Context, enabled *bool) ([]featurepluginapp.FeaturePluginCategoryRow, error) {
	rows := []featurepluginapp.FeaturePluginCategoryRow{}
	for _, cat := range r.state.categories {
		if enabled != nil && cat.Enabled != *enabled {
			continue
		}
		rows = append(rows, r.toRow(cat))
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Category.Sort != rows[j].Category.Sort {
			return rows[i].Category.Sort < rows[j].Category.Sort
		}
		return rows[i].Category.ID < rows[j].Category.ID
	})
	return rows, nil
}

func (r *memCategoryRepo) GetByID(_ context.Context, id int64) (featurepluginapp.FeaturePluginCategoryRow, error) {
	cat, ok := r.state.categories[id]
	if !ok {
		return featurepluginapp.FeaturePluginCategoryRow{}, adminapp.ErrNotFound
	}
	return r.toRow(cat), nil
}

func (r *memCategoryRepo) GetByCode(_ context.Context, code string) (featurepluginapp.FeaturePluginCategoryRow, error) {
	for _, cat := range r.state.categories {
		if cat.CategoryCode == code {
			return r.toRow(cat), nil
		}
	}
	return featurepluginapp.FeaturePluginCategoryRow{}, adminapp.ErrNotFound
}

func (r *memCategoryRepo) Insert(
	_ context.Context, cat domainplugin.FeaturePluginCategory,
) (domainplugin.FeaturePluginCategory, error) {
	for _, existing := range r.state.categories {
		if existing.CategoryCode == cat.CategoryCode {
			return domainplugin.FeaturePluginCategory{}, adminapp.ErrConflict
		}
	}
	r.state.catSeq++
	cat.ID = r.state.catSeq
	cat.CreatedAt = time.Now().UTC()
	cat.UpdatedAt = cat.CreatedAt
	cp := cat
	r.state.categories[cat.ID] = &cp
	return cat, nil
}

func (r *memCategoryRepo) Update(_ context.Context, id int64, patch featurepluginapp.CategoryPatch) error {
	cat, ok := r.state.categories[id]
	if !ok {
		return adminapp.ErrNotFound
	}
	if patch.CategoryName != nil {
		cat.CategoryName = *patch.CategoryName
	}
	if patch.Enabled != nil {
		cat.Enabled = *patch.Enabled
	}
	if patch.Sort != nil {
		cat.Sort = *patch.Sort
	}
	cat.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *memCategoryRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.state.categories[id]; !ok {
		return adminapp.ErrNotFound
	}
	delete(r.state.categories, id)
	return nil
}

func (r *memCategoryRepo) toRow(cat *domainplugin.FeaturePluginCategory) featurepluginapp.FeaturePluginCategoryRow {
	count := 0
	for _, p := range r.state.plugins {
		if p.CategoryIDRef != nil && *p.CategoryIDRef == cat.ID {
			count++
		}
	}
	return featurepluginapp.FeaturePluginCategoryRow{Category: *cat, PluginCount: count}
}

type memPluginRepo struct{ state *memState }

func (r *memPluginRepo) List(
	_ context.Context, q dto.ListFeaturePluginsQuery,
) ([]featurepluginapp.FeaturePluginRow, int, error) {
	rows := []featurepluginapp.FeaturePluginRow{}
	for _, p := range r.state.plugins {
		if kw := strings.ToLower(strings.TrimSpace(q.Keyword)); kw != "" {
			if !strings.Contains(strings.ToLower(p.PluginID), kw) &&
				!strings.Contains(strings.ToLower(p.PluginName), kw) {
				continue
			}
		}
		if q.CategoryID > 0 && (p.CategoryIDRef == nil || *p.CategoryIDRef != q.CategoryID) {
			continue
		}
		if q.Region != "" && p.Region != q.Region {
			continue
		}
		if q.Enabled != nil && p.Enabled != *q.Enabled {
			continue
		}
		rows = append(rows, r.toRow(p))
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Plugin.Sort != rows[j].Plugin.Sort {
			return rows[i].Plugin.Sort < rows[j].Plugin.Sort
		}
		return rows[i].Plugin.ID < rows[j].Plugin.ID
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

func (r *memPluginRepo) GetByPluginID(_ context.Context, pluginID string) (featurepluginapp.FeaturePluginRow, error) {
	p, ok := r.state.plugins[pluginID]
	if !ok {
		return featurepluginapp.FeaturePluginRow{}, adminapp.ErrNotFound
	}
	return r.toRow(p), nil
}

func (r *memPluginRepo) Insert(_ context.Context, p domainplugin.FeaturePlugin) error {
	if _, ok := r.state.plugins[p.PluginID]; ok {
		return adminapp.ErrConflict
	}
	r.state.pluginSeq++
	p.ID = r.state.pluginSeq
	p.CreatedAt = time.Now().UTC()
	p.UpdatedAt = p.CreatedAt
	cp := p
	r.state.plugins[p.PluginID] = &cp
	return nil
}

func (r *memPluginRepo) Update(_ context.Context, pluginID string, patch featurepluginapp.FeaturePluginPatch) error {
	p, ok := r.state.plugins[pluginID]
	if !ok {
		return adminapp.ErrNotFound
	}
	if patch.PluginName != nil {
		p.PluginName = *patch.PluginName
	}
	if patch.CategoryIDRef.Present {
		p.CategoryIDRef = patch.CategoryIDRef.Value
	}
	if patch.Enabled != nil {
		p.Enabled = *patch.Enabled
	}
	if patch.Sort != nil {
		p.Sort = *patch.Sort
	}
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *memPluginRepo) Delete(_ context.Context, id int64) error {
	for key, p := range r.state.plugins {
		if p.ID == id {
			delete(r.state.plugins, key)
			return nil
		}
	}
	return adminapp.ErrNotFound
}

func (r *memPluginRepo) CountReferences(
	_ context.Context, pluginIDRef int64,
) (featurepluginapp.FeaturePluginReferences, error) {
	out := featurepluginapp.FeaturePluginReferences{
		ChannelBindings: r.state.channelRefs[pluginIDRef],
		GameConfigs:     r.state.gameRefs[pluginIDRef],
		PackageOverride: r.state.packageOverrideRefs[pluginIDRef],
	}
	for _, tpl := range r.state.templates {
		if tpl.PluginIDRef == pluginIDRef {
			out.Templates++
		}
	}
	return out, nil
}

func (r *memPluginRepo) CountByCategory(_ context.Context, categoryIDRef int64) (int, error) {
	count := 0
	for _, p := range r.state.plugins {
		if p.CategoryIDRef != nil && *p.CategoryIDRef == categoryIDRef {
			count++
		}
	}
	return count, nil
}

func (r *memPluginRepo) toRow(p *domainplugin.FeaturePlugin) featurepluginapp.FeaturePluginRow {
	row := featurepluginapp.FeaturePluginRow{Plugin: *p}
	if p.CategoryIDRef != nil {
		if cat, ok := r.state.categories[*p.CategoryIDRef]; ok {
			row.CategoryCode, row.CategoryName = cat.CategoryCode, cat.CategoryName
		}
	}
	for _, tpl := range r.state.templates {
		if tpl.PluginIDRef == p.ID {
			row.TemplateCount++
		}
	}
	return row
}

type memTemplateRepo struct{ state *memState }

func (r *memTemplateRepo) ListByPlugin(_ context.Context, pluginIDRef int64) ([]domainplugin.FeaturePluginTemplate, error) {
	out := []domainplugin.FeaturePluginTemplate{}
	for _, tpl := range r.state.templates {
		if tpl.PluginIDRef == pluginIDRef {
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

func (r *memTemplateRepo) GetByID(_ context.Context, id int64) (domainplugin.FeaturePluginTemplate, error) {
	tpl, ok := r.state.templates[id]
	if !ok {
		return domainplugin.FeaturePluginTemplate{}, adminapp.ErrNotFound
	}
	return *tpl, nil
}

func (r *memTemplateRepo) Insert(
	_ context.Context, tpl domainplugin.FeaturePluginTemplate,
) (domainplugin.FeaturePluginTemplate, error) {
	for _, existing := range r.state.templates {
		if existing.PluginIDRef == tpl.PluginIDRef && existing.TemplateVersion == tpl.TemplateVersion {
			return domainplugin.FeaturePluginTemplate{}, adminapp.ErrConflict
		}
	}
	r.state.tplSeq++
	tpl.ID = r.state.tplSeq
	tpl.CreatedAt = time.Now().UTC()
	tpl.UpdatedAt = tpl.CreatedAt
	cp := tpl
	r.state.templates[tpl.ID] = &cp
	return tpl, nil
}

func (r *memTemplateRepo) Replace(_ context.Context, tpl domainplugin.FeaturePluginTemplate) error {
	current, ok := r.state.templates[tpl.ID]
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

// DeleteByPlugin 随插件级联删除该插件的全部模板版本，返回删除条数。
func (r *memTemplateRepo) DeleteByPlugin(_ context.Context, pluginIDRef int64) (int, error) {
	deleted := 0
	for id, tpl := range r.state.templates {
		if tpl.PluginIDRef == pluginIDRef {
			delete(r.state.templates, id)
			deleted++
		}
	}
	return deleted, nil
}

// fakeAudit 记录审计调用，供审计断言使用（与 platformchannel httptest 同口径）。
type fakeAudit struct{ entries []featurepluginapp.AuditEntry }

func (a *fakeAudit) Write(_ context.Context, e featurepluginapp.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func (a *fakeAudit) byAction(action string) (featurepluginapp.AuditEntry, bool) {
	for _, e := range a.entries {
		if e.Action == action {
			return e, true
		}
	}
	return featurepluginapp.AuditEntry{}, false
}
