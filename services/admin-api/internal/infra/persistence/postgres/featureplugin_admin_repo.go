package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	featurepluginapp "github.com/csw/console/services/admin-api/internal/app/featureplugin"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

// FeaturePluginCategoryRepo 插件分类字典读写仓储（platform.feature_plugin_categories）。
// 平台表显式写 platform 前缀：这些表全环境共享，不随 search_path 的 env schema 变化。
type FeaturePluginCategoryRepo struct{ db DBTX }

const featurePluginCategorySelect = `
SELECT c.id, c.category_code, c.category_name, c.enabled, c.sort, c.created_at, c.updated_at,
       (SELECT COUNT(*) FROM platform.feature_plugins p WHERE p.category_id_ref = c.id)
FROM platform.feature_plugin_categories c`

// List 全量列出分类（字典体量小，不分页），按 sort、id 升序。
func (r *FeaturePluginCategoryRepo) List(ctx context.Context, enabled *bool) ([]featurepluginapp.FeaturePluginCategoryRow, error) {
	args := []any{}
	clause := ""
	if enabled != nil {
		args = append(args, *enabled)
		clause = " WHERE c.enabled = $1"
	}
	rows, err := r.db.Query(ctx, featurePluginCategorySelect+clause+` ORDER BY c.sort ASC, c.id ASC`, args...)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []featurepluginapp.FeaturePluginCategoryRow{}
	for rows.Next() {
		row, err := scanFeaturePluginCategoryRow(rows)
		if err != nil {
			return nil, mapErr(err)
		}
		out = append(out, row)
	}
	return out, mapErr(rows.Err())
}

// GetByID 按主键取单行。
func (r *FeaturePluginCategoryRepo) GetByID(ctx context.Context, id int64) (featurepluginapp.FeaturePluginCategoryRow, error) {
	row := r.db.QueryRow(ctx, featurePluginCategorySelect+` WHERE c.id = $1`, id)
	out, err := scanFeaturePluginCategoryRow(row)
	if err != nil {
		return featurepluginapp.FeaturePluginCategoryRow{}, mapErr(err)
	}
	return out, nil
}

// GetByCode 按业务键取单行。
func (r *FeaturePluginCategoryRepo) GetByCode(ctx context.Context, categoryCode string) (featurepluginapp.FeaturePluginCategoryRow, error) {
	row := r.db.QueryRow(ctx, featurePluginCategorySelect+` WHERE c.category_code = $1`, categoryCode)
	out, err := scanFeaturePluginCategoryRow(row)
	if err != nil {
		return featurepluginapp.FeaturePluginCategoryRow{}, mapErr(err)
	}
	return out, nil
}

// Insert 落库新分类。
func (r *FeaturePluginCategoryRepo) Insert(
	ctx context.Context, cat domainplugin.FeaturePluginCategory,
) (domainplugin.FeaturePluginCategory, error) {
	out := cat
	err := r.db.QueryRow(ctx, `
INSERT INTO platform.feature_plugin_categories (category_code, category_name, enabled, sort)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at, updated_at`,
		cat.CategoryCode, cat.CategoryName, cat.Enabled, cat.Sort,
	).Scan(&out.ID, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return domainplugin.FeaturePluginCategory{}, mapErr(err)
	}
	return out, nil
}

// Update 更新可变列（category_code 不可改）。
func (r *FeaturePluginCategoryRepo) Update(ctx context.Context, id int64, patch featurepluginapp.CategoryPatch) error {
	sets := []string{}
	args := []any{}
	if patch.CategoryName != nil {
		args = append(args, *patch.CategoryName)
		sets = append(sets, fmt.Sprintf("category_name = $%d", len(args)))
	}
	if patch.Enabled != nil {
		args = append(args, *patch.Enabled)
		sets = append(sets, fmt.Sprintf("enabled = $%d", len(args)))
	}
	if patch.Sort != nil {
		args = append(args, *patch.Sort)
		sets = append(sets, fmt.Sprintf("sort = $%d", len(args)))
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, id)
	sql := `UPDATE platform.feature_plugin_categories SET ` + strings.Join(sets, ", ") +
		fmt.Sprintf(" WHERE id = $%d", len(args))
	tag, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(pgx.ErrNoRows)
	}
	return nil
}

// Delete 删除分类（服务层已在同事务内确认无插件引用）。
func (r *FeaturePluginCategoryRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM platform.feature_plugin_categories WHERE id = $1`, id)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(pgx.ErrNoRows)
	}
	return nil
}

func scanFeaturePluginCategoryRow(row interface{ Scan(...any) error }) (featurepluginapp.FeaturePluginCategoryRow, error) {
	var out featurepluginapp.FeaturePluginCategoryRow
	if err := row.Scan(
		&out.Category.ID, &out.Category.CategoryCode, &out.Category.CategoryName,
		&out.Category.Enabled, &out.Category.Sort, &out.Category.CreatedAt, &out.Category.UpdatedAt,
		&out.PluginCount,
	); err != nil {
		return featurepluginapp.FeaturePluginCategoryRow{}, err
	}
	return out, nil
}

// FeaturePluginAdminRepo 插件主数据读写仓储（platform.feature_plugins）。
type FeaturePluginAdminRepo struct{ db DBTX }

const featurePluginSelect = `
SELECT p.id, p.plugin_id, p.plugin_name, p.category_id_ref, p.region, p.enabled, p.sort,
       p.created_at, p.updated_at,
       COALESCE(c.category_code, ''), COALESCE(c.category_name, ''),
       (SELECT COUNT(*) FROM platform.feature_plugin_templates t WHERE t.plugin_id_ref = p.id)
FROM platform.feature_plugins p
LEFT JOIN platform.feature_plugin_categories c ON c.id = p.category_id_ref`

// List 分页列出插件（含分类与模板计数），按 sort、id 升序。
func (r *FeaturePluginAdminRepo) List(
	ctx context.Context, q dto.ListFeaturePluginsQuery,
) ([]featurepluginapp.FeaturePluginRow, int, error) {
	where := []string{}
	args := []any{}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		where = append(where, fmt.Sprintf("(p.plugin_id ILIKE $%d OR p.plugin_name ILIKE $%d)", len(args), len(args)))
	}
	if q.CategoryID > 0 {
		args = append(args, q.CategoryID)
		where = append(where, fmt.Sprintf("p.category_id_ref = $%d", len(args)))
	}
	if q.Region != "" {
		args = append(args, q.Region)
		where = append(where, fmt.Sprintf("p.region = $%d", len(args)))
	}
	if q.Enabled != nil {
		args = append(args, *q.Enabled)
		where = append(where, fmt.Sprintf("p.enabled = $%d", len(args)))
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM platform.feature_plugins p`+clause, args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}

	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)
	sql := featurePluginSelect + clause +
		fmt.Sprintf(" ORDER BY p.sort ASC, p.id ASC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	out := []featurepluginapp.FeaturePluginRow{}
	for rows.Next() {
		row, err := scanFeaturePluginRow(rows)
		if err != nil {
			return nil, 0, mapErr(err)
		}
		out = append(out, row)
	}
	return out, total, mapErr(rows.Err())
}

// GetByPluginID 按业务键取单行。
func (r *FeaturePluginAdminRepo) GetByPluginID(ctx context.Context, pluginID string) (featurepluginapp.FeaturePluginRow, error) {
	row := r.db.QueryRow(ctx, featurePluginSelect+` WHERE p.plugin_id = $1`, pluginID)
	out, err := scanFeaturePluginRow(row)
	if err != nil {
		return featurepluginapp.FeaturePluginRow{}, mapErr(err)
	}
	return out, nil
}

// Insert 落库插件主数据。
func (r *FeaturePluginAdminRepo) Insert(ctx context.Context, p domainplugin.FeaturePlugin) error {
	_, err := r.db.Exec(ctx, `
INSERT INTO platform.feature_plugins (plugin_id, plugin_name, category_id_ref, region, enabled, sort)
VALUES ($1, $2, $3, $4, $5, $6)`,
		p.PluginID, p.PluginName, p.CategoryIDRef, p.Region, p.Enabled, p.Sort,
	)
	return mapErr(err)
}

// Update 更新主数据可变列（plugin_id / region 不可改）。
func (r *FeaturePluginAdminRepo) Update(ctx context.Context, pluginID string, patch featurepluginapp.FeaturePluginPatch) error {
	sets := []string{}
	args := []any{}
	if patch.PluginName != nil {
		args = append(args, *patch.PluginName)
		sets = append(sets, fmt.Sprintf("plugin_name = $%d", len(args)))
	}
	if patch.CategoryIDRef.Present {
		// Value 为 nil 时写入 NULL：显式取消归属分类。
		args = append(args, patch.CategoryIDRef.Value)
		sets = append(sets, fmt.Sprintf("category_id_ref = $%d", len(args)))
	}
	if patch.Enabled != nil {
		args = append(args, *patch.Enabled)
		sets = append(sets, fmt.Sprintf("enabled = $%d", len(args)))
	}
	if patch.Sort != nil {
		args = append(args, *patch.Sort)
		sets = append(sets, fmt.Sprintf("sort = $%d", len(args)))
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, pluginID)
	sql := `UPDATE platform.feature_plugins SET ` + strings.Join(sets, ", ") +
		fmt.Sprintf(" WHERE plugin_id = $%d", len(args))
	tag, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(pgx.ErrNoRows)
	}
	return nil
}

// Delete 按主键删除插件（服务层已在同事务内确认无引用）。
func (r *FeaturePluginAdminRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM platform.feature_plugins WHERE id = $1`, id)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(pgx.ErrNoRows)
	}
	return nil
}

// CountReferences 统计插件被引用条数。
// 前两项在共享 schema platform；后两项是游戏侧业务表，不写 schema 前缀（由 search_path 决定 env），
// 这里只读不写，符合「业务表不允许跨 schema 写」的约束。
//
// 边界：若当前 env schema 尚未执行 000012/000017（缺 game_channel_plugin_configs /
// channel_package_plugin_overrides 表），底层会报 42P01（relation does not exist）。
// mapErr 不认识这个 code，会原样上抛，最终在 handler 兜底成裸 500——这里显式识别并
// 转成可读的 409 ENV_SCHEMA_NOT_READY，提示运维先补齐该环境的迁移，而不是让管理员
// 看到一个无信息量的「服务端内部错误」。
func (r *FeaturePluginAdminRepo) CountReferences(
	ctx context.Context, pluginIDRef int64,
) (featurepluginapp.FeaturePluginReferences, error) {
	var out featurepluginapp.FeaturePluginReferences
	err := r.db.QueryRow(ctx, `
SELECT (SELECT COUNT(*) FROM platform.feature_plugin_templates  WHERE plugin_id_ref = $1),
       (SELECT COUNT(*) FROM platform.channel_feature_plugins   WHERE plugin_id_ref = $1),
       (SELECT COUNT(*) FROM game_channel_plugin_configs        WHERE plugin_id_ref = $1),
       (SELECT COUNT(*) FROM channel_package_plugin_overrides   WHERE plugin_id_ref = $1)`,
		pluginIDRef,
	).Scan(&out.Templates, &out.ChannelBindings, &out.GameConfigs, &out.PackageOverride)
	if err != nil {
		if isUndefinedTableErr(err) {
			return featurepluginapp.FeaturePluginReferences{}, &featurepluginapp.Error{
				Status: http.StatusConflict,
				Code:   "ENV_SCHEMA_NOT_READY",
				Message: "当前环境尚未初始化游戏侧插件配置表结构（缺少 game_channel_plugin_configs / " +
					"channel_package_plugin_overrides，需先在该 env schema 执行 000012/000017 迁移），" +
					"暂无法判断插件是否仍被引用，请联系运维补齐迁移后重试",
				Details: []any{},
			}
		}
		return featurepluginapp.FeaturePluginReferences{}, mapErr(err)
	}
	return out, nil
}

// CountByCategory 统计某分类下的插件数。
func (r *FeaturePluginAdminRepo) CountByCategory(ctx context.Context, categoryIDRef int64) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM platform.feature_plugins WHERE category_id_ref = $1`, categoryIDRef,
	).Scan(&count)
	if err != nil {
		return 0, mapErr(err)
	}
	return count, nil
}

func scanFeaturePluginRow(row interface{ Scan(...any) error }) (featurepluginapp.FeaturePluginRow, error) {
	var out featurepluginapp.FeaturePluginRow
	if err := row.Scan(
		&out.Plugin.ID, &out.Plugin.PluginID, &out.Plugin.PluginName, &out.Plugin.CategoryIDRef,
		&out.Plugin.Region, &out.Plugin.Enabled, &out.Plugin.Sort,
		&out.Plugin.CreatedAt, &out.Plugin.UpdatedAt,
		&out.CategoryCode, &out.CategoryName, &out.TemplateCount,
	); err != nil {
		return featurepluginapp.FeaturePluginRow{}, err
	}
	return out, nil
}

// FeaturePluginTemplateAdminRepo 插件参数模板读写仓储（platform.feature_plugin_templates）。
type FeaturePluginTemplateAdminRepo struct{ db DBTX }

const featurePluginTemplateColumns = `t.id, t.plugin_id_ref, p.plugin_id, t.template_version,
       t.form_schema_json, t.secret_fields_json, t.file_fields_json, t.validation_rules_json,
       t.enabled, t.created_at, t.updated_at`

// ListByPlugin 列出该插件的全部模板版本（按 template_version 降序，与运行时取版本口径一致）。
func (r *FeaturePluginTemplateAdminRepo) ListByPlugin(
	ctx context.Context, pluginIDRef int64,
) ([]domainplugin.FeaturePluginTemplate, error) {
	rows, err := r.db.Query(ctx, `SELECT `+featurePluginTemplateColumns+`
FROM platform.feature_plugin_templates t
JOIN platform.feature_plugins p ON p.id = t.plugin_id_ref
WHERE t.plugin_id_ref = $1
ORDER BY t.template_version DESC, t.id DESC`, pluginIDRef)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []domainplugin.FeaturePluginTemplate{}
	for rows.Next() {
		tpl, err := scanFeaturePluginTemplate(rows)
		if err != nil {
			return nil, mapErr(err)
		}
		out = append(out, tpl)
	}
	return out, mapErr(rows.Err())
}

// GetByID 取单个模板版本。
func (r *FeaturePluginTemplateAdminRepo) GetByID(ctx context.Context, id int64) (domainplugin.FeaturePluginTemplate, error) {
	row := r.db.QueryRow(ctx, `SELECT `+featurePluginTemplateColumns+`
FROM platform.feature_plugin_templates t
JOIN platform.feature_plugins p ON p.id = t.plugin_id_ref
WHERE t.id = $1`, id)
	tpl, err := scanFeaturePluginTemplate(row)
	if err != nil {
		return domainplugin.FeaturePluginTemplate{}, mapErr(err)
	}
	return tpl, nil
}

// Insert 落库新版本。
func (r *FeaturePluginTemplateAdminRepo) Insert(
	ctx context.Context, tpl domainplugin.FeaturePluginTemplate,
) (domainplugin.FeaturePluginTemplate, error) {
	form, secret, file, rules, err := marshalPluginTemplateJSON(tpl)
	if err != nil {
		return domainplugin.FeaturePluginTemplate{}, err
	}
	out := tpl
	err = r.db.QueryRow(ctx, `
INSERT INTO platform.feature_plugin_templates (
  plugin_id_ref, template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json, enabled
) VALUES ($1, $2, $3::jsonb, $4::jsonb, $5::jsonb, $6::jsonb, $7)
RETURNING id, created_at, updated_at`,
		tpl.PluginIDRef, tpl.TemplateVersion, form, secret, file, rules, tpl.Enabled,
	).Scan(&out.ID, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return domainplugin.FeaturePluginTemplate{}, mapErr(err)
	}
	return out, nil
}

// Replace 整体覆盖四件套与 enabled（template_version 与所属插件不可改）。
func (r *FeaturePluginTemplateAdminRepo) Replace(ctx context.Context, tpl domainplugin.FeaturePluginTemplate) error {
	form, secret, file, rules, err := marshalPluginTemplateJSON(tpl)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, `UPDATE platform.feature_plugin_templates SET
  form_schema_json      = $2::jsonb,
  secret_fields_json    = $3::jsonb,
  file_fields_json      = $4::jsonb,
  validation_rules_json = $5::jsonb,
  enabled               = $6,
  updated_at            = NOW()
WHERE id = $1`, tpl.ID, form, secret, file, rules, tpl.Enabled)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(pgx.ErrNoRows)
	}
	return nil
}

func marshalPluginTemplateJSON(tpl domainplugin.FeaturePluginTemplate) (form, secret, file, rules string, err error) {
	formSchema := tpl.FormSchema
	if formSchema == nil {
		formSchema = []domainplugin.PluginFormField{}
	}
	secretFields := tpl.SecretFields
	if secretFields == nil {
		secretFields = []string{}
	}
	fileFields := tpl.FileFields
	if fileFields == nil {
		fileFields = []domainplugin.PluginFileField{}
	}
	validationRules := tpl.ValidationRules
	if validationRules == nil {
		validationRules = map[string]domainplugin.PluginValidationRule{}
	}
	formRaw, err := json.Marshal(formSchema)
	if err != nil {
		return "", "", "", "", fmt.Errorf("marshal form_schema_json: %w", err)
	}
	secretRaw, err := json.Marshal(secretFields)
	if err != nil {
		return "", "", "", "", fmt.Errorf("marshal secret_fields_json: %w", err)
	}
	fileRaw, err := json.Marshal(fileFields)
	if err != nil {
		return "", "", "", "", fmt.Errorf("marshal file_fields_json: %w", err)
	}
	rulesRaw, err := json.Marshal(validationRules)
	if err != nil {
		return "", "", "", "", fmt.Errorf("marshal validation_rules_json: %w", err)
	}
	return string(formRaw), string(secretRaw), string(fileRaw), string(rulesRaw), nil
}

func scanFeaturePluginTemplate(row interface{ Scan(...any) error }) (domainplugin.FeaturePluginTemplate, error) {
	var (
		tpl                                  domainplugin.FeaturePluginTemplate
		formRaw, secretRaw, fileRaw, ruleRaw []byte
	)
	if err := row.Scan(
		&tpl.ID, &tpl.PluginIDRef, &tpl.PluginID, &tpl.TemplateVersion,
		&formRaw, &secretRaw, &fileRaw, &ruleRaw,
		&tpl.Enabled, &tpl.CreatedAt, &tpl.UpdatedAt,
	); err != nil {
		return domainplugin.FeaturePluginTemplate{}, err
	}
	tpl.FormSchema = []domainplugin.PluginFormField{}
	tpl.SecretFields = []string{}
	tpl.FileFields = []domainplugin.PluginFileField{}
	tpl.ValidationRules = map[string]domainplugin.PluginValidationRule{}
	if len(formRaw) > 0 {
		if err := json.Unmarshal(formRaw, &tpl.FormSchema); err != nil {
			return domainplugin.FeaturePluginTemplate{}, fmt.Errorf("decode form_schema_json: %w", err)
		}
	}
	if len(secretRaw) > 0 {
		if err := json.Unmarshal(secretRaw, &tpl.SecretFields); err != nil {
			return domainplugin.FeaturePluginTemplate{}, fmt.Errorf("decode secret_fields_json: %w", err)
		}
	}
	if len(fileRaw) > 0 {
		if err := json.Unmarshal(fileRaw, &tpl.FileFields); err != nil {
			return domainplugin.FeaturePluginTemplate{}, fmt.Errorf("decode file_fields_json: %w", err)
		}
	}
	if len(ruleRaw) > 0 {
		if err := json.Unmarshal(ruleRaw, &tpl.ValidationRules); err != nil {
			return domainplugin.FeaturePluginTemplate{}, fmt.Errorf("decode validation_rules_json: %w", err)
		}
	}
	return tpl, nil
}

// 接口符合性编译期断言。
var (
	_ featurepluginapp.FeaturePluginCategoryRepository      = (*FeaturePluginCategoryRepo)(nil)
	_ featurepluginapp.FeaturePluginAdminRepository         = (*FeaturePluginAdminRepo)(nil)
	_ featurepluginapp.FeaturePluginTemplateAdminRepository = (*FeaturePluginTemplateAdminRepo)(nil)
	_ featurepluginapp.TxManager                            = (*FeaturePluginAdminStore)(nil)
)
