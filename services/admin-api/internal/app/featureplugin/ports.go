// Package featureplugin 是「功能插件管理」的应用层（system 侧基础数据维护，00 §4.4）：
// 插件分类字典 + 插件主数据 + 插件参数模板四件套，由系统管理员维护，与游戏无关。
// 游戏侧只在渠道实例/渠道包上引用插件模板填参（见 app/plugin），继续用 plugin.* 授权；
// 本层用独立权限码 feature_plugin.read / feature_plugin.write，避免把「改平台主数据」的
// 能力混进游戏运营用的 plugin.write。
package featureplugin

import (
	"context"
	"net/http"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

// Error 携带统一错误码/HTTP 状态/消息/明细的应用层错误（00 §7.4）。
type Error struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *Error) Error() string { return e.Message }

const (
	codeValidation = "VALIDATION_FAILED"
	codeConflict   = "CONFLICT"
	codeNotFound   = "NOT_FOUND"
)

func validationErr(msg string, details ...any) *Error {
	if details == nil {
		details = []any{}
	}
	return &Error{Status: http.StatusBadRequest, Code: codeValidation, Message: msg, Details: details}
}

func conflictErr(msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: codeConflict, Message: msg, Details: []any{}}
}

func notFoundErr(msg string) *Error {
	return &Error{Status: http.StatusNotFound, Code: codeNotFound, Message: msg, Details: []any{}}
}

// issueDetails 把领域校验明细转为统一 details（[{field, rule, message}]）。
func issueDetails(issues []domainplugin.ValidationIssue) []any {
	out := make([]any, 0, len(issues))
	for _, item := range issues {
		out = append(out, map[string]string{"field": item.Field, "rule": item.Rule, "message": item.Message})
	}
	return out
}

// FeaturePluginCategoryRow 分类字典行：主数据 + 该分类下插件数。
type FeaturePluginCategoryRow struct {
	Category    domainplugin.FeaturePluginCategory
	PluginCount int
}

// FeaturePluginRow 插件主数据行：主数据 + 分类冗余展示字段 + 模板版本数。
type FeaturePluginRow struct {
	Plugin        domainplugin.FeaturePlugin
	CategoryCode  string
	CategoryName  string
	TemplateCount int
}

// FeaturePluginReferences 插件被引用计数，用于删除前的 409 判定。
// 模板与渠道绑定在共享 schema platform；后两者在当前 env schema（游戏侧配置）。
type FeaturePluginReferences struct {
	Templates       int
	ChannelBindings int
	GameConfigs     int
	PackageOverride int
}

// Total 引用总数；>0 即不允许删除插件。
func (r FeaturePluginReferences) Total() int {
	return r.Templates + r.ChannelBindings + r.GameConfigs + r.PackageOverride
}

// CategoryPatch 分类字典列级补丁（nil 不改）。
// category_code 是业务键，创建后不可改，故不在补丁内。
type CategoryPatch struct {
	CategoryName *string
	Enabled      *bool
	Sort         *int
}

// FeaturePluginPatch 插件主数据列级补丁（nil 不改）。
// plugin_id / region 不在补丁内：前者是身份，后者决定 market 兼容性判定口径，
// 改动会让已存在的渠道实例插件配置集体失配，故创建后不可改。
// CategoryIDRef 用 NullableInt64 以支持「显式清空归属分类」。
type FeaturePluginPatch struct {
	PluginName    *string
	CategoryIDRef dto.NullableInt64
	Enabled       *bool
	Sort          *int
}

// FeaturePluginCategoryRepository 插件分类字典读写仓储（platform.feature_plugin_categories）。
type FeaturePluginCategoryRepository interface {
	// List 全量列出分类（按 sort、id 升序，含插件计数）；enabled 为 nil 表示不限。
	List(ctx context.Context, enabled *bool) ([]FeaturePluginCategoryRow, error)
	// GetByID 按主键取单行（不存在返回 adminapp.ErrNotFound）。
	GetByID(ctx context.Context, id int64) (FeaturePluginCategoryRow, error)
	// GetByCode 按业务键取单行（不存在返回 adminapp.ErrNotFound）。
	GetByCode(ctx context.Context, categoryCode string) (FeaturePluginCategoryRow, error)
	// Insert 落库新分类（category_code 冲突返回 adminapp.ErrConflict）。
	Insert(ctx context.Context, cat domainplugin.FeaturePluginCategory) (domainplugin.FeaturePluginCategory, error)
	// Update 更新可变列。
	Update(ctx context.Context, id int64, patch CategoryPatch) error
	// Delete 删除分类（调用方需先确认无插件引用）。
	Delete(ctx context.Context, id int64) error
}

// FeaturePluginAdminRepository 插件主数据读写仓储（platform.feature_plugins）。
type FeaturePluginAdminRepository interface {
	// List 分页列出插件（含分类与模板计数），按 sort、id 升序。
	List(ctx context.Context, q dto.ListFeaturePluginsQuery) ([]FeaturePluginRow, int, error)
	// GetByPluginID 按业务键取单行（不存在返回 adminapp.ErrNotFound）。
	GetByPluginID(ctx context.Context, pluginID string) (FeaturePluginRow, error)
	// Insert 落库插件主数据（plugin_id 冲突返回 adminapp.ErrConflict）。
	Insert(ctx context.Context, p domainplugin.FeaturePlugin) error
	// Update 更新主数据可变列。
	Update(ctx context.Context, pluginID string, patch FeaturePluginPatch) error
	// Delete 按主键删除插件（调用方需先确认无引用）。
	Delete(ctx context.Context, id int64) error
	// CountReferences 统计插件被模板/渠道绑定/游戏侧配置引用的条数。
	CountReferences(ctx context.Context, pluginIDRef int64) (FeaturePluginReferences, error)
	// CountByCategory 统计某分类下的插件数（分类删除前的 409 判定）。
	CountByCategory(ctx context.Context, categoryIDRef int64) (int, error)
}

// FeaturePluginTemplateAdminRepository 插件参数模板读写仓储（platform.feature_plugin_templates）。
type FeaturePluginTemplateAdminRepository interface {
	// ListByPlugin 列出该插件的全部模板版本（按 template_version 降序）。
	ListByPlugin(ctx context.Context, pluginIDRef int64) ([]domainplugin.FeaturePluginTemplate, error)
	// GetByID 取单个模板版本（不存在返回 adminapp.ErrNotFound）。
	GetByID(ctx context.Context, id int64) (domainplugin.FeaturePluginTemplate, error)
	// Insert 落库新版本（(plugin_id_ref, template_version) 冲突返回 adminapp.ErrConflict）。
	Insert(ctx context.Context, tpl domainplugin.FeaturePluginTemplate) (domainplugin.FeaturePluginTemplate, error)
	// Replace 整体覆盖四件套与 enabled（tpl 为服务层合并校验后的完整状态）。
	Replace(ctx context.Context, tpl domainplugin.FeaturePluginTemplate) error
}

// Repositories 一组仓储句柄（绑定到 pool 或某事务连接）。
type Repositories struct {
	Categories FeaturePluginCategoryRepository
	Plugins    FeaturePluginAdminRepository
	Templates  FeaturePluginTemplateAdminRepository
}

// TxManager 提供事务边界（唯一性预检 + 落库、引用预检 + 删除需同事务）。
type TxManager interface {
	Repositories() Repositories
	InTx(ctx context.Context, fn func(Repositories) error) error
}

// AuditSink / AuditEntry 复用 auth 应用层端口，保持审计写入一致（00 §8）。
type (
	AuditSink  = adminapp.AuditSink
	AuditEntry = adminapp.AuditEntry
)
