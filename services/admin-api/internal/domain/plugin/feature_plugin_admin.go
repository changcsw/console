package plugin

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// 本文件是「系统管理员维护插件主数据」的纯领域规则（分类字典 / 插件主数据 / 参数模板四件套），
// 与同包内 compatibility.go、plugin_config.go 的游戏侧运行时规则职责分离：
// 后者判定某渠道实例上一份插件配置是否可用，前者只管平台级基础数据自身的合法性。
// 模板四件套的字段定义与渠道模版同构但独立成型（不 import domain/channel，保持领域边界干净）。

// ValidationIssue 领域校验明细（field 用 camelCase，与 API 契约字段同名，便于前端定位）。
type ValidationIssue struct {
	Field   string
	Rule    string
	Message string
}

// FeaturePluginCategory 插件分类字典实体（platform.feature_plugin_categories）。
// CategoryCode 是业务键，创建后不可改。
type FeaturePluginCategory struct {
	ID           int64
	CategoryCode string
	CategoryName string
	Enabled      bool
	Sort         int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// FeaturePlugin 功能插件主数据实体（platform.feature_plugins）。
// CategoryIDRef 可空：既有数据允许未归类，分类被删前也要求先转移插件。
type FeaturePlugin struct {
	ID            int64
	PluginID      string
	PluginName    string
	CategoryIDRef *int64
	Region        string
	Enabled       bool
	Sort          int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// FeaturePluginTemplate 插件参数模板实体（platform.feature_plugin_templates）。
// PluginID 为装配出的插件业务键，仅用于响应展示。
type FeaturePluginTemplate struct {
	ID              int64
	PluginIDRef     int64
	PluginID        string
	TemplateVersion string
	FormSchema      []PluginFormField
	SecretFields    []string
	FileFields      []PluginFileField
	ValidationRules map[string]PluginValidationRule
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PluginFormField 模板 form_schema_json 字段定义（含 scope：server 字段不下发客户端）。
type PluginFormField struct {
	Key         string              `json:"key"`
	Label       string              `json:"label"`
	Component   string              `json:"component"`
	Required    bool                `json:"required"`
	Order       int                 `json:"order"`
	Group       string              `json:"group"`
	Scope       string              `json:"scope"`
	Placeholder string              `json:"placeholder,omitempty"`
	Options     []PluginFieldOption `json:"options,omitempty"`
}

// PluginFieldOption 下拉字段候选项（component=select 时使用）。
type PluginFieldOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// PluginFileField 模板 file_fields_json 字段定义。
type PluginFileField struct {
	Key       string   `json:"key"`
	Accept    []string `json:"accept"`
	MaxSizeKB *int     `json:"maxSizeKB"`
}

// PluginValidationRule 模板 validation_rules_json 字段定义。
type PluginValidationRule struct {
	Required bool     `json:"required"`
	MinLen   *int     `json:"minLen"`
	MaxLen   *int     `json:"maxLen"`
	Min      *float64 `json:"min"`
	Max      *float64 `json:"max"`
	Pattern  string   `json:"pattern"`
	Format   string   `json:"format"`
	Enum     []string `json:"enum"`
}

var (
	businessKeyPattern           = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	pluginTemplateVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	pluginFieldKeyPattern        = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

// 模板字段组件取值集合（与前端表单渲染器一致，与渠道模版同集）。
var pluginTemplateComponents = []string{"input", "password", "textarea", "number", "select", "switch", "file", "json"}

// 模板字段作用域取值集合。"" 表示不区分（语义等价于双端）；"both" 是历史遗留取值，语义与 ""
// 完全相同，仅为兼容旧数据保留在允许集合里，新数据统一走 ""（前端编辑器已不再提供 "both" 选项）。
var pluginTemplateScopes = []string{"", "client", "server", "both"}

// IsValidPluginRegion 校验插件适用区域（与 compatibility.go 的 region 同集）。
func IsValidPluginRegion(region string) bool {
	return region == RegionDomestic || region == RegionOverseas
}

// ValidateCategoryCode 校验分类业务键格式（小写字母/数字/下划线，字母开头，≤64）。
func ValidateCategoryCode(categoryCode string) []ValidationIssue {
	if !businessKeyPattern.MatchString(categoryCode) || len(categoryCode) > 64 {
		return []ValidationIssue{{
			Field:   "categoryCode",
			Rule:    "pattern",
			Message: "分类编码只能用小写字母/数字/下划线，且以字母开头，长度不超过 64",
		}}
	}
	return nil
}

// ValidateFeaturePluginCategory 校验分类字典可变字段（创建/编辑共用，无 IO）。
// categoryCode 是业务键，仅创建时校验格式（见 ValidateCategoryCode），编辑时不可改。
func ValidateFeaturePluginCategory(c FeaturePluginCategory) []ValidationIssue {
	issues := []ValidationIssue{}
	if strings.TrimSpace(c.CategoryName) == "" || len(c.CategoryName) > 64 {
		issues = append(issues, ValidationIssue{Field: "categoryName", Rule: "length", Message: "分类名称必填且不超过 64 字符"})
	}
	if c.Sort < 0 || c.Sort > 9999 {
		issues = append(issues, ValidationIssue{Field: "sort", Rule: "range", Message: "排序值需在 0-9999 之间"})
	}
	return issues
}

// ValidatePluginID 校验插件业务键格式（小写字母/数字/下划线，字母开头，≤64）。
func ValidatePluginID(pluginID string) []ValidationIssue {
	if !businessKeyPattern.MatchString(pluginID) || len(pluginID) > 64 {
		return []ValidationIssue{{
			Field:   "pluginId",
			Rule:    "pattern",
			Message: "插件 ID 只能用小写字母/数字/下划线，且以字母开头，长度不超过 64",
		}}
	}
	return nil
}

// ValidateFeaturePluginMaster 校验插件主数据可变字段（创建/编辑共用，无 IO）。
// pluginId 与 region 属身份与兼容性口径，创建后不可改，故此处只校验 region 取值合法性。
func ValidateFeaturePluginMaster(p FeaturePlugin) []ValidationIssue {
	issues := []ValidationIssue{}
	if strings.TrimSpace(p.PluginName) == "" || len(p.PluginName) > 64 {
		issues = append(issues, ValidationIssue{Field: "pluginName", Rule: "length", Message: "插件名称必填且不超过 64 字符"})
	}
	if !IsValidPluginRegion(p.Region) {
		issues = append(issues, ValidationIssue{Field: "region", Rule: "enum", Message: "region 只能为 domestic/overseas"})
	}
	if p.CategoryIDRef != nil && *p.CategoryIDRef <= 0 {
		issues = append(issues, ValidationIssue{Field: "categoryId", Rule: "range", Message: "categoryId 必须为正整数"})
	}
	if p.Sort < 0 || p.Sort > 9999 {
		issues = append(issues, ValidationIssue{Field: "sort", Rule: "range", Message: "排序值需在 0-9999 之间"})
	}
	return issues
}

// ValidateFeaturePluginTemplate 校验插件参数模板四件套自洽性（无 IO，与渠道模版同口径）：
//   - template_version 格式；form_schema 非空、key 唯一合法、component/scope 枚举
//   - secret_fields / file_fields 的 key 必须在 form_schema 中声明
//   - component=file ⇔ 出现在 file_fields；component=password ⇒ 必须声明为 secret_fields
//     （避免管理员建出「口令字段明文入库」的模板）
//   - validation_rules 的 key 不要求在 form_schema 中声明（字段命名不确定），但 pattern 必须可编译
func ValidateFeaturePluginTemplate(tpl FeaturePluginTemplate) []ValidationIssue {
	issues := []ValidationIssue{}
	if !pluginTemplateVersionPattern.MatchString(tpl.TemplateVersion) || len(tpl.TemplateVersion) > 32 {
		issues = append(issues, ValidationIssue{Field: "templateVersion", Rule: "pattern", Message: "版本号只能用字母/数字/点/横线/下划线，长度不超过 32"})
	}
	if len(tpl.FormSchema) == 0 {
		issues = append(issues, ValidationIssue{Field: "formSchemaJson", Rule: "required", Message: "至少需要一个表单字段"})
	}

	keys := map[string]PluginFormField{}
	for i, f := range tpl.FormSchema {
		field := "formSchemaJson[" + strconv.Itoa(i) + "]"
		if !pluginFieldKeyPattern.MatchString(f.Key) || len(f.Key) > 64 {
			issues = append(issues, ValidationIssue{Field: field + ".key", Rule: "pattern", Message: "字段 key 需以字母开头，仅含字母/数字/下划线"})
			continue
		}
		if _, dup := keys[f.Key]; dup {
			issues = append(issues, ValidationIssue{Field: field + ".key", Rule: "duplicate", Message: "字段 key 重复：" + f.Key})
			continue
		}
		keys[f.Key] = f
		if strings.TrimSpace(f.Label) == "" || len(f.Label) > 64 {
			issues = append(issues, ValidationIssue{Field: field + ".label", Rule: "length", Message: "字段标签必填且不超过 64 字符"})
		}
		if !slices.Contains(pluginTemplateComponents, f.Component) {
			issues = append(issues, ValidationIssue{Field: field + ".component", Rule: "enum", Message: "组件类型非法：" + f.Component})
		}
		if !slices.Contains(pluginTemplateScopes, f.Scope) {
			issues = append(issues, ValidationIssue{Field: field + ".scope", Rule: "enum", Message: "scope 只能为 空串(不区分)/client/server"})
		}
		if f.Component == "select" && len(f.Options) == 0 {
			issues = append(issues, ValidationIssue{Field: field + ".options", Rule: "required", Message: "下拉字段必须配置选项"})
		}
	}

	secretSet := map[string]struct{}{}
	for _, key := range tpl.SecretFields {
		if _, ok := keys[key]; !ok {
			issues = append(issues, ValidationIssue{Field: "secretFieldsJson", Rule: "unknown", Message: "敏感字段未在表单中声明：" + key})
			continue
		}
		if _, dup := secretSet[key]; dup {
			issues = append(issues, ValidationIssue{Field: "secretFieldsJson", Rule: "duplicate", Message: "敏感字段重复：" + key})
			continue
		}
		secretSet[key] = struct{}{}
	}

	fileSet := map[string]struct{}{}
	for _, f := range tpl.FileFields {
		if _, ok := keys[f.Key]; !ok {
			issues = append(issues, ValidationIssue{Field: "fileFieldsJson", Rule: "unknown", Message: "文件字段未在表单中声明：" + f.Key})
			continue
		}
		if _, dup := fileSet[f.Key]; dup {
			issues = append(issues, ValidationIssue{Field: "fileFieldsJson", Rule: "duplicate", Message: "文件字段重复：" + f.Key})
			continue
		}
		fileSet[f.Key] = struct{}{}
		if f.MaxSizeKB != nil && *f.MaxSizeKB <= 0 {
			issues = append(issues, ValidationIssue{Field: "fileFieldsJson", Rule: "range", Message: "maxSizeKB 必须为正整数：" + f.Key})
		}
	}

	for key, field := range keys {
		if field.Component == "file" {
			if _, ok := fileSet[key]; !ok {
				issues = append(issues, ValidationIssue{Field: "fileFieldsJson", Rule: "required", Message: "文件组件字段必须登记到文件字段列表：" + key})
			}
		}
		if field.Component == "password" {
			if _, ok := secretSet[key]; !ok {
				issues = append(issues, ValidationIssue{Field: "secretFieldsJson", Rule: "required", Message: "口令组件字段必须登记为敏感字段：" + key})
			}
		}
	}

	// validation_rules 的 key 不要求必须在 form_schema 中声明：真实模版字段命名多变，管理员按需为任意
	// 字段登记规则即可；运行时校验已将 validation_rules 的 key 一并纳入 allowed 集合。
	for key, rule := range tpl.ValidationRules {
		if rule.Pattern == "" {
			continue
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			issues = append(issues, ValidationIssue{Field: "validationRulesJson." + key + ".pattern", Rule: "pattern", Message: "正则表达式非法"})
		}
	}
	return issues
}
