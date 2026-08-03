package featureplugin

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

// Service 功能插件分类字典 / 插件主数据 / 参数模板的读写用例
// （系统管理员维护，权限码 feature_plugin.read / feature_plugin.write）。
type Service struct {
	tx    TxManager
	audit AuditSink
}

// NewService 构造服务。
func NewService(tx TxManager, audit AuditSink) *Service {
	return &Service{tx: tx, audit: audit}
}

// ===== 分类字典 =====

// ListCategories 分类字典全量列表（GET /feature-plugin-categories）。
// 字典体量小，不分页，按 sort 升序返回。
func (s *Service) ListCategories(ctx context.Context, enabled *bool) ([]dto.FeaturePluginCategoryView, error) {
	rows, err := s.tx.Repositories().Categories.List(ctx, enabled)
	if err != nil {
		return nil, err
	}
	items := make([]dto.FeaturePluginCategoryView, 0, len(rows))
	for i := range rows {
		items = append(items, toCategoryView(rows[i]))
	}
	return items, nil
}

// CreateCategory 新建分类（POST /feature-plugin-categories）。
func (s *Service) CreateCategory(ctx context.Context, cmd dto.CreateFeaturePluginCategoryCmd) (dto.FeaturePluginCategoryView, error) {
	zero := dto.FeaturePluginCategoryView{}
	code := strings.TrimSpace(cmd.CategoryCode)
	if issues := domainplugin.ValidateCategoryCode(code); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}
	cat := domainplugin.FeaturePluginCategory{
		CategoryCode: code,
		CategoryName: strings.TrimSpace(cmd.CategoryName),
		Enabled:      boolOr(cmd.Enabled, true),
		Sort:         intOr(cmd.Sort, 0),
	}
	if issues := domainplugin.ValidateFeaturePluginCategory(cat); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}

	var created domainplugin.FeaturePluginCategory
	err := s.tx.InTx(ctx, func(repos Repositories) error {
		if _, err := repos.Categories.GetByCode(ctx, code); err == nil {
			return conflictErr("分类编码已存在：" + code)
		} else if !errors.Is(err, adminapp.ErrNotFound) {
			return err
		}
		inserted, err := repos.Categories.Insert(ctx, cat)
		if err != nil {
			return err
		}
		created = inserted
		return nil
	})
	if err != nil {
		return zero, mapWriteErr(err, "分类编码已存在："+code)
	}
	if err := s.writeAudit(ctx, "feature_plugin_category.create", "feature_plugin_category", itoa(created.ID), map[string]any{
		"categoryId": created.ID, "categoryCode": created.CategoryCode, "categoryName": created.CategoryName,
	}); err != nil {
		return zero, err
	}
	return s.getCategory(ctx, created.ID)
}

// UpdateCategory 编辑分类（PATCH /feature-plugin-categories/{id}）。
// categoryCode 是业务键，创建后不可改（插件主数据与前端字典缓存都按它对齐）。
func (s *Service) UpdateCategory(ctx context.Context, cmd dto.UpdateFeaturePluginCategoryCmd) (dto.FeaturePluginCategoryView, error) {
	zero := dto.FeaturePluginCategoryView{}
	row, err := s.loadCategory(ctx, cmd.CategoryID)
	if err != nil {
		return zero, err
	}

	patch := CategoryPatch{Enabled: cmd.Enabled, Sort: cmd.Sort}
	if cmd.CategoryName != nil {
		name := strings.TrimSpace(*cmd.CategoryName)
		patch.CategoryName = &name
	}
	merged := row.Category
	if patch.CategoryName != nil {
		merged.CategoryName = *patch.CategoryName
	}
	if patch.Enabled != nil {
		merged.Enabled = *patch.Enabled
	}
	if patch.Sort != nil {
		merged.Sort = *patch.Sort
	}
	if issues := domainplugin.ValidateFeaturePluginCategory(merged); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}

	changed := changedCategoryFields(patch)
	if len(changed) == 0 {
		return toCategoryView(row), nil
	}
	if err := s.tx.Repositories().Categories.Update(ctx, cmd.CategoryID, patch); err != nil {
		return zero, mapWriteErr(err, "分类更新冲突")
	}
	if err := s.writeAudit(ctx, "feature_plugin_category.update", "feature_plugin_category", itoa(cmd.CategoryID), map[string]any{
		"categoryId": cmd.CategoryID, "categoryCode": row.Category.CategoryCode, "fields": changed,
	}); err != nil {
		return zero, err
	}
	return s.getCategory(ctx, cmd.CategoryID)
}

// DeleteCategory 删除分类（DELETE /feature-plugin-categories/{id}）。
// 仍有插件归属该分类时拒绝删除：插件的 category_id_ref 是可空外键，
// 静默置空会让既有插件悄悄变成「未归类」，故要求管理员先转移插件。
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	row, err := s.loadCategory(ctx, id)
	if err != nil {
		return err
	}
	err = s.tx.InTx(ctx, func(repos Repositories) error {
		count, err := repos.Plugins.CountByCategory(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return conflictErr("该分类下仍有插件，无法删除：请先将插件转移到其它分类")
		}
		return repos.Categories.Delete(ctx, id)
	})
	if err != nil {
		return mapWriteErr(err, "分类删除冲突")
	}
	return s.writeAudit(ctx, "feature_plugin_category.delete", "feature_plugin_category", itoa(id), map[string]any{
		"categoryId": id, "categoryCode": row.Category.CategoryCode,
	})
}

// ===== 插件主数据 =====

// ListPlugins 插件主数据分页列表（GET /feature-plugins）。
func (s *Service) ListPlugins(ctx context.Context, q dto.ListFeaturePluginsQuery) (dto.Page[dto.FeaturePluginView], error) {
	empty := dto.Page[dto.FeaturePluginView]{}
	if q.Region != "" && !domainplugin.IsValidPluginRegion(q.Region) {
		return empty, validationErr("region 非法", fieldDetail("region", "enum"))
	}
	if q.CategoryID < 0 {
		return empty, validationErr("categoryId 非法", fieldDetail("categoryId", "int64"))
	}
	page, pageSize := normalizePage(q.Page, q.PageSize)
	q.Page, q.PageSize = page, pageSize

	rows, total, err := s.tx.Repositories().Plugins.List(ctx, q)
	if err != nil {
		return empty, err
	}
	items := make([]dto.FeaturePluginView, 0, len(rows))
	for i := range rows {
		items = append(items, toPluginView(rows[i]))
	}
	return dto.Page[dto.FeaturePluginView]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// GetPlugin 插件主数据详情（GET /feature-plugins/{pluginId}）。
func (s *Service) GetPlugin(ctx context.Context, pluginID string) (dto.FeaturePluginView, error) {
	row, err := s.loadPlugin(ctx, strings.TrimSpace(pluginID))
	if err != nil {
		return dto.FeaturePluginView{}, err
	}
	return toPluginView(row), nil
}

// CreatePlugin 新建插件主数据（POST /feature-plugins）。
func (s *Service) CreatePlugin(ctx context.Context, cmd dto.CreateFeaturePluginCmd) (dto.FeaturePluginView, error) {
	zero := dto.FeaturePluginView{}
	pluginID := strings.TrimSpace(cmd.PluginID)
	if issues := domainplugin.ValidatePluginID(pluginID); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}
	p := domainplugin.FeaturePlugin{
		PluginID:      pluginID,
		PluginName:    strings.TrimSpace(cmd.PluginName),
		CategoryIDRef: normalizeCategoryID(cmd.CategoryID),
		Region:        strings.TrimSpace(cmd.Region),
		Enabled:       boolOr(cmd.Enabled, true),
		Sort:          intOr(cmd.Sort, 0),
	}
	if issues := domainplugin.ValidateFeaturePluginMaster(p); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}

	err := s.tx.InTx(ctx, func(repos Repositories) error {
		if err := ensureCategoryExists(ctx, repos, p.CategoryIDRef); err != nil {
			return err
		}
		if _, err := repos.Plugins.GetByPluginID(ctx, pluginID); err == nil {
			return conflictErr("插件 ID 已存在：" + pluginID)
		} else if !errors.Is(err, adminapp.ErrNotFound) {
			return err
		}
		return repos.Plugins.Insert(ctx, p)
	})
	if err != nil {
		return zero, mapWriteErr(err, "插件 ID 已存在："+pluginID)
	}
	if err := s.writeAudit(ctx, "feature_plugin.create", "feature_plugin", pluginID, map[string]any{
		"pluginId": pluginID, "pluginName": p.PluginName, "region": p.Region,
	}); err != nil {
		return zero, err
	}
	return s.GetPlugin(ctx, pluginID)
}

// UpdatePlugin 编辑插件主数据（PATCH /feature-plugins/{pluginId}）。
// pluginId / region 创建后不可改：region 决定 market 兼容性判定，
// 改动会让既有渠道实例上的插件配置集体失配（同渠道 region 的理由）。
func (s *Service) UpdatePlugin(ctx context.Context, cmd dto.UpdateFeaturePluginCmd) (dto.FeaturePluginView, error) {
	zero := dto.FeaturePluginView{}
	pluginID := strings.TrimSpace(cmd.PluginID)
	row, err := s.loadPlugin(ctx, pluginID)
	if err != nil {
		return zero, err
	}

	patch := FeaturePluginPatch{Enabled: cmd.Enabled, Sort: cmd.Sort, CategoryIDRef: cmd.CategoryID}
	if cmd.PluginName != nil {
		name := strings.TrimSpace(*cmd.PluginName)
		patch.PluginName = &name
	}
	merged := row.Plugin
	if patch.PluginName != nil {
		merged.PluginName = *patch.PluginName
	}
	if patch.CategoryIDRef.Present {
		merged.CategoryIDRef = patch.CategoryIDRef.Value
	}
	if patch.Enabled != nil {
		merged.Enabled = *patch.Enabled
	}
	if patch.Sort != nil {
		merged.Sort = *patch.Sort
	}
	if issues := domainplugin.ValidateFeaturePluginMaster(merged); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}

	changed := changedPluginFields(patch)
	if len(changed) == 0 {
		return toPluginView(row), nil
	}
	err = s.tx.InTx(ctx, func(repos Repositories) error {
		if patch.CategoryIDRef.Present {
			if err := ensureCategoryExists(ctx, repos, patch.CategoryIDRef.Value); err != nil {
				return err
			}
		}
		return repos.Plugins.Update(ctx, pluginID, patch)
	})
	if err != nil {
		return zero, mapWriteErr(err, "插件更新冲突")
	}
	if err := s.writeAudit(ctx, "feature_plugin.update", "feature_plugin", pluginID, map[string]any{
		"pluginId": pluginID, "fields": changed,
	}); err != nil {
		return zero, err
	}
	return s.GetPlugin(ctx, pluginID)
}

// DeletePlugin 删除插件主数据（DELETE /feature-plugins/{pluginId}）。
// 仍有模板版本、渠道绑定策略或游戏侧插件配置引用时拒绝删除（外键会挡住，这里给出可读原因）。
func (s *Service) DeletePlugin(ctx context.Context, pluginID string) error {
	pluginID = strings.TrimSpace(pluginID)
	row, err := s.loadPlugin(ctx, pluginID)
	if err != nil {
		return err
	}
	err = s.tx.InTx(ctx, func(repos Repositories) error {
		refs, err := repos.Plugins.CountReferences(ctx, row.Plugin.ID)
		if err != nil {
			return err
		}
		if refs.Total() > 0 {
			return conflictErr("该插件仍有关联数据（" + referenceSummary(refs) + "），请先删除关联数据")
		}
		return repos.Plugins.Delete(ctx, row.Plugin.ID)
	})
	if err != nil {
		return mapWriteErr(err, "插件删除冲突")
	}
	return s.writeAudit(ctx, "feature_plugin.delete", "feature_plugin", pluginID, map[string]any{
		"pluginId": pluginID, "pluginName": row.Plugin.PluginName,
	})
}

// ===== 参数模板 =====

// ListTemplates 列出某插件的模板版本（GET /feature-plugins/{pluginId}/templates）。
func (s *Service) ListTemplates(ctx context.Context, pluginID string) ([]dto.FeaturePluginTemplateVersionView, error) {
	row, err := s.loadPlugin(ctx, strings.TrimSpace(pluginID))
	if err != nil {
		return nil, err
	}
	items, err := s.tx.Repositories().Templates.ListByPlugin(ctx, row.Plugin.ID)
	if err != nil {
		return nil, err
	}
	// 仓储按 template_version 降序返回，运行时取「enabled 的最新版本」，故首个 enabled 即生效版本。
	out := make([]dto.FeaturePluginTemplateVersionView, 0, len(items))
	effectiveFound := false
	for i := range items {
		items[i].PluginID = row.Plugin.PluginID
		effective := false
		if !effectiveFound && items[i].Enabled {
			effective, effectiveFound = true, true
		}
		out = append(out, toTemplateView(items[i], effective))
	}
	return out, nil
}

// GetTemplate 取单个模板版本（GET /feature-plugin-templates/{id}）。
func (s *Service) GetTemplate(ctx context.Context, templateID int64) (dto.FeaturePluginTemplateVersionView, error) {
	tpl, err := s.tx.Repositories().Templates.GetByID(ctx, templateID)
	if err != nil {
		return dto.FeaturePluginTemplateVersionView{}, mapLoadErr(err, "插件模板不存在")
	}
	return s.withEffectiveFlag(ctx, tpl)
}

// CreateTemplate 新建模板版本（POST /feature-plugins/{pluginId}/templates）。
func (s *Service) CreateTemplate(ctx context.Context, cmd dto.CreateFeaturePluginTemplateCmd) (dto.FeaturePluginTemplateVersionView, error) {
	zero := dto.FeaturePluginTemplateVersionView{}
	row, err := s.loadPlugin(ctx, strings.TrimSpace(cmd.PluginID))
	if err != nil {
		return zero, err
	}
	tpl := domainplugin.FeaturePluginTemplate{
		PluginIDRef:     row.Plugin.ID,
		PluginID:        row.Plugin.PluginID,
		TemplateVersion: strings.TrimSpace(cmd.TemplateVersion),
		FormSchema:      toDomainFormSchema(cmd.FormSchema),
		SecretFields:    trimStrings(cmd.SecretFields),
		FileFields:      toDomainFileFields(cmd.FileFields),
		ValidationRules: toDomainRules(cmd.ValidationRules),
		Enabled:         boolOr(cmd.Enabled, true),
	}
	if issues := domainplugin.ValidateFeaturePluginTemplate(tpl); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}

	var created domainplugin.FeaturePluginTemplate
	err = s.tx.InTx(ctx, func(repos Repositories) error {
		inserted, err := repos.Templates.Insert(ctx, tpl)
		if err != nil {
			return err
		}
		created = inserted
		return nil
	})
	if err != nil {
		return zero, mapWriteErr(err, "该插件下模板版本 "+tpl.TemplateVersion+" 已存在")
	}
	created.PluginID = row.Plugin.PluginID
	if err := s.writeAudit(ctx, "feature_plugin_template.create", "feature_plugin_template", itoa(created.ID), map[string]any{
		"pluginId": row.Plugin.PluginID, "templateVersion": created.TemplateVersion,
	}); err != nil {
		return zero, err
	}
	return s.withEffectiveFlag(ctx, created)
}

// UpdateTemplate 编辑模板版本（PATCH /feature-plugin-templates/{id}）。
// 四件套按整体替换合并：入参为 nil 的部分保留原值。
func (s *Service) UpdateTemplate(ctx context.Context, cmd dto.UpdateFeaturePluginTemplateCmd) (dto.FeaturePluginTemplateVersionView, error) {
	zero := dto.FeaturePluginTemplateVersionView{}
	current, err := s.tx.Repositories().Templates.GetByID(ctx, cmd.TemplateID)
	if err != nil {
		return zero, mapLoadErr(err, "插件模板不存在")
	}

	merged := current
	changed := []string{}
	if cmd.FormSchema != nil {
		merged.FormSchema = toDomainFormSchema(cmd.FormSchema)
		changed = append(changed, "formSchemaJson")
	}
	if cmd.SecretFields != nil {
		merged.SecretFields = trimStrings(cmd.SecretFields)
		changed = append(changed, "secretFieldsJson")
	}
	if cmd.FileFields != nil {
		merged.FileFields = toDomainFileFields(cmd.FileFields)
		changed = append(changed, "fileFieldsJson")
	}
	if cmd.ValidationRules != nil {
		merged.ValidationRules = toDomainRules(cmd.ValidationRules)
		changed = append(changed, "validationRulesJson")
	}
	if cmd.Enabled != nil {
		merged.Enabled = *cmd.Enabled
		changed = append(changed, "enabled")
	}
	if len(changed) == 0 {
		return s.withEffectiveFlag(ctx, current)
	}
	if issues := domainplugin.ValidateFeaturePluginTemplate(merged); len(issues) > 0 {
		return zero, validationErr(issues[0].Message, issueDetails(issues)...)
	}

	if err := s.tx.Repositories().Templates.Replace(ctx, merged); err != nil {
		return zero, mapWriteErr(err, "插件模板更新冲突")
	}
	if err := s.writeAudit(ctx, "feature_plugin_template.update", "feature_plugin_template", itoa(merged.ID), map[string]any{
		"pluginId": merged.PluginID, "templateVersion": merged.TemplateVersion, "fields": changed,
	}); err != nil {
		return zero, err
	}
	return s.withEffectiveFlag(ctx, merged)
}

// ===== helpers =====

func (s *Service) loadCategory(ctx context.Context, id int64) (FeaturePluginCategoryRow, error) {
	if id <= 0 {
		return FeaturePluginCategoryRow{}, validationErr("categoryId 必填", fieldDetail("categoryId", "required"))
	}
	row, err := s.tx.Repositories().Categories.GetByID(ctx, id)
	if err != nil {
		return FeaturePluginCategoryRow{}, mapLoadErr(err, "插件分类不存在")
	}
	return row, nil
}

func (s *Service) getCategory(ctx context.Context, id int64) (dto.FeaturePluginCategoryView, error) {
	row, err := s.loadCategory(ctx, id)
	if err != nil {
		return dto.FeaturePluginCategoryView{}, err
	}
	return toCategoryView(row), nil
}

func (s *Service) loadPlugin(ctx context.Context, pluginID string) (FeaturePluginRow, error) {
	if pluginID == "" {
		return FeaturePluginRow{}, validationErr("pluginId 必填", fieldDetail("pluginId", "required"))
	}
	row, err := s.tx.Repositories().Plugins.GetByPluginID(ctx, pluginID)
	if err != nil {
		return FeaturePluginRow{}, mapLoadErr(err, "插件不存在")
	}
	return row, nil
}

// withEffectiveFlag 重新读取同插件的模板版本，判定 tpl 是否为当前生效版本后出视图。
// 仓储按 template_version 降序返回，运行时取「enabled 的最新版本」，故首个 enabled 即生效版本。
func (s *Service) withEffectiveFlag(ctx context.Context, tpl domainplugin.FeaturePluginTemplate) (dto.FeaturePluginTemplateVersionView, error) {
	items, err := s.tx.Repositories().Templates.ListByPlugin(ctx, tpl.PluginIDRef)
	if err != nil {
		return dto.FeaturePluginTemplateVersionView{}, err
	}
	effective := false
	for _, item := range items {
		if tpl.PluginID == "" {
			tpl.PluginID = item.PluginID
		}
		if item.Enabled {
			effective = item.ID == tpl.ID
			break
		}
	}
	return toTemplateView(tpl, effective), nil
}

func (s *Service) writeAudit(ctx context.Context, action, resourceType, resourceID string, detail map[string]any) error {
	if s.audit == nil {
		return nil
	}
	actor := int64(0)
	if ac, ok := adminapp.AuthContextFrom(ctx); ok {
		actor = ac.UserID
	}
	return s.audit.Write(ctx, AuditEntry{
		ActorID: actor, Action: action, ResourceType: resourceType, ResourceID: resourceID, Detail: detail,
	})
}

// ensureCategoryExists 校验插件归属的分类存在；categoryID 为 nil（未归类）时直接通过。
func ensureCategoryExists(ctx context.Context, repos Repositories, categoryID *int64) error {
	if categoryID == nil {
		return nil
	}
	if _, err := repos.Categories.GetByID(ctx, *categoryID); err != nil {
		if errors.Is(err, adminapp.ErrNotFound) {
			return validationErr("插件分类不存在："+itoa(*categoryID), fieldDetail("categoryId", "exists"))
		}
		return err
	}
	return nil
}

// normalizeCategoryID 把缺省与 0 视为「未归类」（与 dto.NullableInt64 的清空口径一致）；
// 负数保留原值交由领域校验拒绝，避免与 PATCH 路径出现「同一入参一边报错一边静默忽略」的分歧。
func normalizeCategoryID(in *int64) *int64 {
	if in == nil || *in == 0 {
		return nil
	}
	v := *in
	return &v
}

// referenceSummary 拼出人类可读的引用明细，帮助管理员定位要先清理什么。
func referenceSummary(refs FeaturePluginReferences) string {
	parts := []string{}
	if refs.Templates > 0 {
		parts = append(parts, "参数模板 "+strconv.Itoa(refs.Templates)+" 个")
	}
	if refs.ChannelBindings > 0 {
		parts = append(parts, "渠道绑定 "+strconv.Itoa(refs.ChannelBindings)+" 条")
	}
	if refs.GameConfigs > 0 {
		parts = append(parts, "渠道实例配置 "+strconv.Itoa(refs.GameConfigs)+" 条")
	}
	if refs.PackageOverride > 0 {
		parts = append(parts, "渠道包覆盖 "+strconv.Itoa(refs.PackageOverride)+" 条")
	}
	return strings.Join(parts, "、")
}

func toCategoryView(row FeaturePluginCategoryRow) dto.FeaturePluginCategoryView {
	return dto.FeaturePluginCategoryView{
		ID:           row.Category.ID,
		CategoryCode: row.Category.CategoryCode,
		CategoryName: row.Category.CategoryName,
		Enabled:      row.Category.Enabled,
		Sort:         row.Category.Sort,
		PluginCount:  row.PluginCount,
		CreatedAt:    row.Category.CreatedAt,
		UpdatedAt:    row.Category.UpdatedAt,
	}
}

func toPluginView(row FeaturePluginRow) dto.FeaturePluginView {
	var categoryID *int64
	if row.Plugin.CategoryIDRef != nil {
		v := *row.Plugin.CategoryIDRef
		categoryID = &v
	}
	return dto.FeaturePluginView{
		PluginID:      row.Plugin.PluginID,
		PluginName:    row.Plugin.PluginName,
		CategoryID:    categoryID,
		CategoryCode:  row.CategoryCode,
		CategoryName:  row.CategoryName,
		Region:        row.Plugin.Region,
		Enabled:       row.Plugin.Enabled,
		Sort:          row.Plugin.Sort,
		TemplateCount: row.TemplateCount,
		UpdatedAt:     row.Plugin.UpdatedAt,
	}
}

func toTemplateView(tpl domainplugin.FeaturePluginTemplate, effective bool) dto.FeaturePluginTemplateVersionView {
	form := make([]any, 0, len(tpl.FormSchema))
	for _, f := range tpl.FormSchema {
		entry := map[string]any{
			"key": f.Key, "label": f.Label, "component": f.Component,
			"required": f.Required, "order": f.Order, "group": f.Group, "scope": f.Scope,
		}
		if f.Placeholder != "" {
			entry["placeholder"] = f.Placeholder
		}
		if len(f.Options) > 0 {
			options := make([]any, 0, len(f.Options))
			for _, opt := range f.Options {
				options = append(options, map[string]any{"label": opt.Label, "value": opt.Value})
			}
			entry["options"] = options
		}
		form = append(form, entry)
	}

	files := make([]any, 0, len(tpl.FileFields))
	for _, f := range tpl.FileFields {
		entry := map[string]any{"key": f.Key}
		if len(f.Accept) > 0 {
			entry["accept"] = slices.Clone(f.Accept)
		}
		if f.MaxSizeKB != nil {
			entry["maxSizeKB"] = *f.MaxSizeKB
		}
		files = append(files, entry)
	}

	rules := map[string]any{}
	for key, rule := range tpl.ValidationRules {
		entry := map[string]any{}
		if rule.Required {
			entry["required"] = true
		}
		if rule.MinLen != nil {
			entry["minLen"] = *rule.MinLen
		}
		if rule.MaxLen != nil {
			entry["maxLen"] = *rule.MaxLen
		}
		if rule.Min != nil {
			entry["min"] = *rule.Min
		}
		if rule.Max != nil {
			entry["max"] = *rule.Max
		}
		if rule.Pattern != "" {
			entry["pattern"] = rule.Pattern
		}
		if rule.Format != "" {
			entry["format"] = rule.Format
		}
		if len(rule.Enum) > 0 {
			entry["enum"] = slices.Clone(rule.Enum)
		}
		rules[key] = entry
	}

	secrets := tpl.SecretFields
	if secrets == nil {
		secrets = []string{}
	}
	return dto.FeaturePluginTemplateVersionView{
		TemplateID:          tpl.ID,
		PluginID:            tpl.PluginID,
		TemplateVersion:     tpl.TemplateVersion,
		FormSchemaJSON:      form,
		SecretFieldsJSON:    slices.Clone(secrets),
		FileFieldsJSON:      files,
		ValidationRulesJSON: rules,
		Enabled:             tpl.Enabled,
		Effective:           effective,
		CreatedAt:           tpl.CreatedAt,
		UpdatedAt:           tpl.UpdatedAt,
	}
}

func toDomainFormSchema(in []dto.TemplateFieldInput) []domainplugin.PluginFormField {
	out := make([]domainplugin.PluginFormField, 0, len(in))
	for _, f := range in {
		field := domainplugin.PluginFormField{
			Key:         strings.TrimSpace(f.Key),
			Label:       strings.TrimSpace(f.Label),
			Component:   strings.TrimSpace(f.Component),
			Required:    f.Required,
			Order:       f.Order,
			Group:       strings.TrimSpace(f.Group),
			Scope:       strings.TrimSpace(f.Scope),
			Placeholder: f.Placeholder,
		}
		for _, opt := range f.Options {
			field.Options = append(field.Options, domainplugin.PluginFieldOption{Label: opt.Label, Value: opt.Value})
		}
		out = append(out, field)
	}
	return out
}

func toDomainFileFields(in []dto.TemplateFileFieldInput) []domainplugin.PluginFileField {
	out := make([]domainplugin.PluginFileField, 0, len(in))
	for _, f := range in {
		out = append(out, domainplugin.PluginFileField{
			Key:       strings.TrimSpace(f.Key),
			Accept:    trimStrings(f.Accept),
			MaxSizeKB: f.MaxSizeKB,
		})
	}
	return out
}

func toDomainRules(in map[string]dto.TemplateRuleInput) map[string]domainplugin.PluginValidationRule {
	out := map[string]domainplugin.PluginValidationRule{}
	for key, rule := range in {
		out[strings.TrimSpace(key)] = domainplugin.PluginValidationRule{
			Required: rule.Required,
			MinLen:   rule.MinLen,
			MaxLen:   rule.MaxLen,
			Min:      rule.Min,
			Max:      rule.Max,
			Pattern:  rule.Pattern,
			Format:   rule.Format,
			Enum:     trimStrings(rule.Enum),
		}
	}
	return out
}

func trimStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		out = append(out, strings.TrimSpace(item))
	}
	return out
}

func changedCategoryFields(patch CategoryPatch) []string {
	fields := []string{}
	if patch.CategoryName != nil {
		fields = append(fields, "categoryName")
	}
	if patch.Enabled != nil {
		fields = append(fields, "enabled")
	}
	if patch.Sort != nil {
		fields = append(fields, "sort")
	}
	return fields
}

func changedPluginFields(patch FeaturePluginPatch) []string {
	fields := []string{}
	if patch.PluginName != nil {
		fields = append(fields, "pluginName")
	}
	if patch.CategoryIDRef.Present {
		fields = append(fields, "categoryId")
	}
	if patch.Enabled != nil {
		fields = append(fields, "enabled")
	}
	if patch.Sort != nil {
		fields = append(fields, "sort")
	}
	return fields
}

func fieldDetail(field, reason string) any {
	return map[string]string{"field": field, "reason": reason}
}

func boolOr(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}

func intOr(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// normalizePage 归一化分页（00 §7.3：page>=1，pageSize 默认 20、最大 100）。
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func mapLoadErr(err error, notFoundMsg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr(notFoundMsg)
	}
	return err
}

func mapWriteErr(err error, conflictMsg string) error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, adminapp.ErrConflict) {
		return conflictErr(conflictMsg)
	}
	if errors.Is(err, adminapp.ErrNotFound) {
		return notFoundErr("目标记录不存在")
	}
	return err
}
