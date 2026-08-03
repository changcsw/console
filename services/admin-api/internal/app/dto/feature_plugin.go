package dto

import (
	"encoding/json"
	"time"
)

// ===== Commands（功能插件分类字典 / 插件主数据 / 参数模板；系统管理员维护，与游戏无关）=====

// NullableInt64 表达 PATCH 语义下的可空外键：
// Present=false 表示请求体未带该键（不改）；Present=true 且 Value=nil 表示显式清空归属。
// 直接用 *int64 无法区分「缺省」与「显式 null」，故单独建型并实现 json.Unmarshaler。
type NullableInt64 struct {
	Present bool
	Value   *int64
}

// UnmarshalJSON 记录该键出现过；null 与 0 都视为清空（0 不是合法自增主键）。
func (n *NullableInt64) UnmarshalJSON(b []byte) error {
	n.Present = true
	n.Value = nil
	if string(b) == "null" {
		return nil
	}
	var v int64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if v != 0 {
		n.Value = &v
	}
	return nil
}

// CreateFeaturePluginCategoryCmd 新建插件分类（POST /feature-plugin-categories）。
type CreateFeaturePluginCategoryCmd struct {
	CategoryCode string
	CategoryName string
	Enabled      *bool // 缺省 true
	Sort         *int  // 缺省 0
}

// UpdateFeaturePluginCategoryCmd 编辑插件分类（PATCH /feature-plugin-categories/{id}）。
// nil 表示不改；categoryCode 是业务键，创建后不可改。
type UpdateFeaturePluginCategoryCmd struct {
	CategoryID   int64
	CategoryName *string
	Enabled      *bool
	Sort         *int
}

// ListFeaturePluginsQuery 插件主数据列表查询（GET /feature-plugins）。
// Keyword 模糊匹配 plugin_id / plugin_name；CategoryID / Enabled 为零值/nil 表示不限。
type ListFeaturePluginsQuery struct {
	Keyword    string
	CategoryID int64
	Region     string
	Enabled    *bool
	Page       int
	PageSize   int
}

// CreateFeaturePluginCmd 新建插件主数据（POST /feature-plugins）。
type CreateFeaturePluginCmd struct {
	PluginID   string
	PluginName string
	CategoryID *int64 // nil 表示暂不归类
	Region     string
	Enabled    *bool // 缺省 true
	Sort       *int  // 缺省 0
}

// UpdateFeaturePluginCmd 编辑插件主数据（PATCH /feature-plugins/{pluginId}）。
// nil 表示不改。pluginId / region 属身份与兼容性判定口径，创建后不可改。
type UpdateFeaturePluginCmd struct {
	PluginID   string
	PluginName *string
	CategoryID NullableInt64
	Enabled    *bool
	Sort       *int
}

// CreateFeaturePluginTemplateCmd 新建插件参数模板版本（POST /feature-plugins/{pluginId}/templates）。
type CreateFeaturePluginTemplateCmd struct {
	PluginID        string
	TemplateVersion string
	FormSchema      []TemplateFieldInput
	SecretFields    []string
	FileFields      []TemplateFileFieldInput
	ValidationRules map[string]TemplateRuleInput
	Enabled         *bool // 缺省 true
}

// UpdateFeaturePluginTemplateCmd 编辑插件参数模板版本（PATCH /feature-plugin-templates/{id}）。
// 四件套为整体替换语义：为 nil 表示该件不改。templateVersion 与所属插件不可改。
type UpdateFeaturePluginTemplateCmd struct {
	TemplateID      int64
	FormSchema      []TemplateFieldInput
	SecretFields    []string
	FileFields      []TemplateFileFieldInput
	ValidationRules map[string]TemplateRuleInput
	Enabled         *bool
}

// ===== Views =====

// FeaturePluginCategoryView 插件分类字典视图。
// PluginCount 为该分类下插件数，供前端提示「删除前先转移插件」。
type FeaturePluginCategoryView struct {
	ID           int64     `json:"id"`
	CategoryCode string    `json:"categoryCode"`
	CategoryName string    `json:"categoryName"`
	Enabled      bool      `json:"enabled"`
	Sort         int       `json:"sort"`
	PluginCount  int       `json:"pluginCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// FeaturePluginView 插件主数据视图（含分类冗余展示字段与模板版本数）。
// CategoryID 为 null 表示未归类，此时 categoryCode / categoryName 为空串。
type FeaturePluginView struct {
	PluginID      string    `json:"pluginId"`
	PluginName    string    `json:"pluginName"`
	CategoryID    *int64    `json:"categoryId"`
	CategoryCode  string    `json:"categoryCode"`
	CategoryName  string    `json:"categoryName"`
	Region        string    `json:"region"`
	Enabled       bool      `json:"enabled"`
	Sort          int       `json:"sort"`
	TemplateCount int       `json:"templateCount"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// FeaturePluginTemplateVersionView 插件参数模板版本视图（四件套原样回传，供前端模板编辑器渲染）。
// Effective 标记该版本是否为当前生效版本（同插件下 enabled 的最新 template_version）。
// 与 plugin.go 里的 FeaturePluginTemplateView 区分：后者是游戏侧配置页内嵌的「当前生效模板」精简片段。
type FeaturePluginTemplateVersionView struct {
	TemplateID          int64          `json:"templateId"`
	PluginID            string         `json:"pluginId"`
	TemplateVersion     string         `json:"templateVersion"`
	FormSchemaJSON      []any          `json:"formSchemaJson"`
	SecretFieldsJSON    []string       `json:"secretFieldsJson"`
	FileFieldsJSON      []any          `json:"fileFieldsJson"`
	ValidationRulesJSON map[string]any `json:"validationRulesJson"`
	Enabled             bool           `json:"enabled"`
	Effective           bool           `json:"effective"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
}
