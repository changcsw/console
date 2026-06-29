package accountauth

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

const (
	// MissingSecretOrFileMessage 缺少必填敏感/文件字段时的统一提示前缀（compact 固定文案）。
	MissingSecretOrFileMessage = "缺少必填敏感字段或文件字段"
)

// FormField 模板字段定义（来自 form_schema_json）。
type FormField struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Component string `json:"component"`
	Required  bool   `json:"required"`
	Order     int    `json:"order"`
	Scope     string `json:"scope"`
}

// FileField 文件字段定义（来自 file_fields_json）。
type FileField struct {
	Key string `json:"key"`
}

// ValidationRule 模板字段校验规则（来自 validation_rules_json）。
type ValidationRule struct {
	Required bool     `json:"required"`
	MinLen   *int     `json:"minLen"`
	MaxLen   *int     `json:"maxLen"`
	Pattern  string   `json:"pattern"`
	Format   string   `json:"format"`
	Enum     []string `json:"enum"`
}

// Template 账号认证模板四件套。
type Template struct {
	TemplateVersion string                    `json:"templateVersion"`
	FormSchema      []FormField               `json:"formSchema"`
	SecretFields    []string                  `json:"secretFields"`
	FileFields      []FileField               `json:"fileFields"`
	ValidationRules map[string]ValidationRule `json:"validationRules"`
}

// ValidateConfigAgainstTemplate 纯规则：按模板校验配置，返回 config_status 与校验消息。
func ValidateConfigAgainstTemplate(config map[string]any, tpl Template) (common.ConfigStatus, string) {
	requiredBySchema := map[string]bool{}
	secretSet := map[string]bool{}
	fileSet := map[string]bool{}
	for _, key := range tpl.SecretFields {
		secretSet[key] = true
	}
	for _, f := range tpl.FileFields {
		fileSet[f.Key] = true
	}
	for _, f := range tpl.FormSchema {
		if f.Required {
			requiredBySchema[f.Key] = true
		}
	}

	required := map[string]bool{}
	for key, yes := range requiredBySchema {
		if yes {
			required[key] = true
		}
	}
	for key, rule := range tpl.ValidationRules {
		if rule.Required {
			required[key] = true
		}
	}
	for key := range secretSet {
		required[key] = true
	}
	for key := range fileSet {
		required[key] = true
	}

	missingSecretOrFile := []string{}
	missingNormal := []string{}
	for key := range required {
		val, ok := config[key]
		if !ok || isEmptyValue(val) {
			if secretSet[key] || fileSet[key] {
				missingSecretOrFile = append(missingSecretOrFile, key)
			} else {
				missingNormal = append(missingNormal, key)
			}
		}
	}
	if len(missingSecretOrFile) > 0 {
		return common.ConfigStatusInvalid, MissingSecretOrFileMessage + ": " + strings.Join(missingSecretOrFile, ",")
	}
	if len(missingNormal) > 0 {
		return common.ConfigStatusInvalid, "缺少必填字段: " + strings.Join(missingNormal, ",")
	}

	for key, rule := range tpl.ValidationRules {
		val, ok := config[key]
		if !ok || isEmptyValue(val) {
			continue
		}
		if msg := validateValueByRule(key, val, rule); msg != "" {
			return common.ConfigStatusInvalid, msg
		}
	}
	return common.ConfigStatusValid, ""
}

func validateValueByRule(key string, value any, rule ValidationRule) string {
	switch v := value.(type) {
	case string:
		if rule.MinLen != nil && len(v) < *rule.MinLen {
			return fmt.Sprintf("%s 长度不能小于 %d", key, *rule.MinLen)
		}
		if rule.MaxLen != nil && len(v) > *rule.MaxLen {
			return fmt.Sprintf("%s 长度不能大于 %d", key, *rule.MaxLen)
		}
		if rule.Pattern != "" {
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				return fmt.Sprintf("%s pattern 非法", key)
			}
			if !re.MatchString(v) {
				return fmt.Sprintf("%s 不匹配 pattern", key)
			}
		}
		if rule.Format != "" {
			switch rule.Format {
			case "url":
				if _, err := url.ParseRequestURI(v); err != nil {
					return fmt.Sprintf("%s 不是合法 URL", key)
				}
			case "email":
				if _, err := mail.ParseAddress(v); err != nil {
					return fmt.Sprintf("%s 不是合法邮箱", key)
				}
			case "host":
				if strings.Contains(v, "://") || strings.ContainsAny(v, "/?#") || strings.TrimSpace(v) == "" {
					return fmt.Sprintf("%s 不是合法 host", key)
				}
			}
		}
		if len(rule.Enum) > 0 {
			hit := false
			for _, item := range rule.Enum {
				if v == item {
					hit = true
					break
				}
			}
			if !hit {
				return fmt.Sprintf("%s 不在允许范围内", key)
			}
		}
	case float64, int, int64:
		// 数值 min/max 可在后续模板真正使用数值字段时补强；当前模块模板以字符串字段为主。
	default:
		if len(rule.Enum) > 0 {
			return fmt.Sprintf("%s 类型不匹配 enum 规则", key)
		}
	}
	return ""
}

func isEmptyValue(v any) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(val) == ""
	default:
		return false
	}
}
