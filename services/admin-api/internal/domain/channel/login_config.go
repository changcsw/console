package channel

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

const (
	// SecretMaskedValue 密文字段统一脱敏/保留哨兵（compact 约定）。
	SecretMaskedValue = "******"
	// SecretMaskedAlias 兼容账号认证模块历史哨兵值。
	SecretMaskedAlias = "masked"
)

// ValidationIssue 模板校验失败明细（用于 details[]）。
type ValidationIssue struct {
	Field   string
	Rule    string
	Message string
}

// ChannelLoginFormField 模板 form_schema 字段定义。
type ChannelLoginFormField struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Component string `json:"component"`
	Required  bool   `json:"required"`
	Order     int    `json:"order"`
	Group     string `json:"group"`
	Scope     string `json:"scope"`
}

// ChannelLoginFileField 模板 file_fields 字段定义。
type ChannelLoginFileField struct {
	Key       string   `json:"key"`
	Accept    []string `json:"accept"`
	MaxSizeKB *int     `json:"maxSizeKB"`
}

// ChannelLoginValidationRule 模板 validation_rules 字段定义。
type ChannelLoginValidationRule struct {
	Required bool     `json:"required"`
	MinLen   *int     `json:"minLen"`
	MaxLen   *int     `json:"maxLen"`
	Min      *float64 `json:"min"`
	Max      *float64 `json:"max"`
	Pattern  string   `json:"pattern"`
	Format   string   `json:"format"`
	Enum     []string `json:"enum"`
}

// ChannelLoginTemplate 平台模板实体（platform.channel_login_templates）。
type ChannelLoginTemplate struct {
	ID              int64
	ChannelIDRef    int64
	TemplateVersion string
	FormSchema      []ChannelLoginFormField
	SecretFields    []string
	FileFields      []ChannelLoginFileField
	ValidationRules map[string]ChannelLoginValidationRule
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ChannelLoginConfig 渠道实例登录配置实体（game_channel_login_configs）。
type ChannelLoginConfig struct {
	ID               int64
	GameChannelIDRef int64
	Enabled          bool
	ConfigJSON       map[string]any
	ConfigStatus     common.ConfigStatus
	LastCheckAt      *time.Time
	LastCheckMessage string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NewCopiedLoginConfig 复制创建渠道登录配置（纯领域，无 IO）：
// 仅复制 form_schema 中的普通字段（非 secret/非 file），secret/file 字段一律清空；
// 新实例 enabled=false、config_status=invalid、last_check_message 固定为缺字段文案
// （00 §3.4 复制创建强约束：secret/file 清空必 invalid，绝不 empty）。
// tpl 为 nil（模板缺失）时不带入任何字段，但仍返回 invalid 占位（绝不 empty）。
func NewCopiedLoginConfig(gameChannelID int64, tpl *ChannelLoginTemplate, source *ChannelLoginConfig) ChannelLoginConfig {
	normal := map[string]any{}
	if tpl != nil && source != nil && source.ConfigJSON != nil {
		secretSet := map[string]struct{}{}
		fileSet := map[string]struct{}{}
		for _, key := range tpl.SecretFields {
			secretSet[key] = struct{}{}
		}
		for _, f := range tpl.FileFields {
			fileSet[f.Key] = struct{}{}
		}
		for _, f := range tpl.FormSchema {
			if _, ok := secretSet[f.Key]; ok {
				continue
			}
			if _, ok := fileSet[f.Key]; ok {
				continue
			}
			if v, ok := source.ConfigJSON[f.Key]; ok {
				normal[f.Key] = v
			}
		}
	}
	return ChannelLoginConfig{
		GameChannelIDRef: gameChannelID,
		Enabled:          false,
		ConfigJSON:       normal,
		ConfigStatus:     common.ConfigStatusInvalid,
		LastCheckMessage: CopiedMissingFieldsMessage,
	}
}

// ValidateLoginConfigAgainstTemplate 纯领域校验：
// - 拒绝未知字段
// - required（含 secret/file）校验
// - validation_rules 校验（minLen/maxLen/min/max/pattern/format/enum）
// 返回 status/message/issues（无 IO）。
func ValidateLoginConfigAgainstTemplate(config map[string]any, tpl ChannelLoginTemplate) (common.ConfigStatus, string, []ValidationIssue) {
	cfg := config
	if cfg == nil {
		cfg = map[string]any{}
	}
	if len(cfg) == 0 {
		return common.ConfigStatusEmpty, "", nil
	}

	issues := []ValidationIssue{}
	allowed := map[string]struct{}{}
	required := map[string]struct{}{}
	secretSet := map[string]struct{}{}
	fileSet := map[string]struct{}{}

	for _, key := range tpl.SecretFields {
		secretSet[key] = struct{}{}
		required[key] = struct{}{}
	}
	for _, f := range tpl.FileFields {
		fileSet[f.Key] = struct{}{}
		required[f.Key] = struct{}{}
	}
	for _, f := range tpl.FormSchema {
		allowed[f.Key] = struct{}{}
		if f.Required {
			required[f.Key] = struct{}{}
		}
	}
	for key, rule := range tpl.ValidationRules {
		allowed[key] = struct{}{}
		if rule.Required {
			required[key] = struct{}{}
		}
	}

	for key := range cfg {
		if _, ok := allowed[key]; ok {
			continue
		}
		issues = append(issues, ValidationIssue{
			Field:   key,
			Rule:    "unknown",
			Message: "字段未在模板中定义",
		})
	}

	missingSecretOrFile := []string{}
	for key := range required {
		v, ok := cfg[key]
		if ok && !isBlankValue(v) {
			continue
		}
		if _, ok := secretSet[key]; ok {
			missingSecretOrFile = append(missingSecretOrFile, key)
			issues = append(issues, ValidationIssue{
				Field:   key,
				Rule:    "required",
				Message: "缺少必填敏感字段",
			})
			continue
		}
		if _, ok := fileSet[key]; ok {
			missingSecretOrFile = append(missingSecretOrFile, key)
			issues = append(issues, ValidationIssue{
				Field:   key,
				Rule:    "required",
				Message: "缺少必填文件字段",
			})
			continue
		}
		issues = append(issues, ValidationIssue{
			Field:   key,
			Rule:    "required",
			Message: "缺少必填字段",
		})
	}

	for key, rule := range tpl.ValidationRules {
		value, ok := cfg[key]
		if !ok || isBlankValue(value) {
			continue
		}
		issues = append(issues, validateByRule(key, value, rule)...)
	}

	if len(issues) == 0 {
		return common.ConfigStatusValid, "", nil
	}
	message := issues[0].Message
	if len(missingSecretOrFile) > 0 {
		slices.Sort(missingSecretOrFile)
		message = CopiedMissingFieldsMessage + ": " + strings.Join(missingSecretOrFile, ",")
	}
	return common.ConfigStatusInvalid, message, issues
}

func validateByRule(field string, value any, rule ChannelLoginValidationRule) []ValidationIssue {
	issues := []ValidationIssue{}
	if rule.MinLen != nil || rule.MaxLen != nil || rule.Pattern != "" || rule.Format != "" || len(rule.Enum) > 0 {
		str, ok := value.(string)
		if !ok {
			issues = append(issues, ValidationIssue{Field: field, Rule: "type", Message: "字段类型必须为字符串"})
			return issues
		}
		if rule.MinLen != nil && len(str) < *rule.MinLen {
			issues = append(issues, ValidationIssue{Field: field, Rule: "minLen", Message: fmt.Sprintf("长度不能小于 %d", *rule.MinLen)})
		}
		if rule.MaxLen != nil && len(str) > *rule.MaxLen {
			issues = append(issues, ValidationIssue{Field: field, Rule: "maxLen", Message: fmt.Sprintf("长度不能大于 %d", *rule.MaxLen)})
		}
		if rule.Pattern != "" {
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				issues = append(issues, ValidationIssue{Field: field, Rule: "pattern", Message: "模板 pattern 非法"})
			} else if !re.MatchString(str) {
				issues = append(issues, ValidationIssue{Field: field, Rule: "pattern", Message: "不匹配 pattern"})
			}
		}
		if rule.Format != "" && !matchFormat(str, rule.Format) {
			issues = append(issues, ValidationIssue{Field: field, Rule: "format", Message: fmt.Sprintf("不符合格式 %s", rule.Format)})
		}
		if len(rule.Enum) > 0 && !slices.Contains(rule.Enum, str) {
			issues = append(issues, ValidationIssue{Field: field, Rule: "enum", Message: "不在允许范围"})
		}
	}

	if rule.Min != nil || rule.Max != nil {
		num, ok := toFloat64(value)
		if !ok {
			issues = append(issues, ValidationIssue{Field: field, Rule: "type", Message: "字段类型必须为数值"})
			return issues
		}
		if rule.Min != nil && num < *rule.Min {
			issues = append(issues, ValidationIssue{Field: field, Rule: "min", Message: fmt.Sprintf("不能小于 %v", *rule.Min)})
		}
		if rule.Max != nil && num > *rule.Max {
			issues = append(issues, ValidationIssue{Field: field, Rule: "max", Message: fmt.Sprintf("不能大于 %v", *rule.Max)})
		}
	}
	return issues
}

func matchFormat(v, format string) bool {
	switch format {
	case "url":
		_, err := url.ParseRequestURI(v)
		return err == nil
	case "email":
		_, err := mail.ParseAddress(v)
		return err == nil
	case "host":
		return strings.TrimSpace(v) != "" &&
			!strings.Contains(v, "://") &&
			!strings.ContainsAny(v, "/?#")
	default:
		return false
	}
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}

func isBlankValue(v any) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(val) == ""
	default:
		return false
	}
}
