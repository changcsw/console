package plugin

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

const MissingSecretOrFileMessage = "缺少必填敏感字段或文件字段"

type TemplateField struct {
	Key      string `json:"key"`
	Required bool   `json:"required"`
	Scope    string `json:"scope"`
}

type FileField struct {
	Key string `json:"key"`
}

type ValidationRule struct {
	Required bool     `json:"required"`
	MinLen   *int     `json:"minLen"`
	MaxLen   *int     `json:"maxLen"`
	Pattern  string   `json:"pattern"`
	Format   string   `json:"format"`
	Enum     []string `json:"enum"`
}

type ConfigTemplate struct {
	FormSchema      []TemplateField
	SecretFields    []string
	FileFields      []FileField
	ValidationRules map[string]ValidationRule
}

// ResolvePluginConfigStatus 纯规则：按模板校验并返回 config_status 与检查提示。
func ResolvePluginConfigStatus(enabled bool, template ConfigTemplate, config map[string]any) (common.ConfigStatus, string) {
	if !enabled {
		return common.ConfigStatusEmpty, ""
	}
	allowed := map[string]struct{}{}
	required := map[string]struct{}{}
	secret := map[string]struct{}{}
	file := map[string]struct{}{}
	for _, key := range template.SecretFields {
		secret[key] = struct{}{}
		required[key] = struct{}{}
		allowed[key] = struct{}{}
	}
	for _, f := range template.FileFields {
		file[f.Key] = struct{}{}
		required[f.Key] = struct{}{}
		allowed[f.Key] = struct{}{}
	}
	for _, f := range template.FormSchema {
		allowed[f.Key] = struct{}{}
		if f.Required {
			required[f.Key] = struct{}{}
		}
	}
	for key, r := range template.ValidationRules {
		allowed[key] = struct{}{}
		if r.Required {
			required[key] = struct{}{}
		}
	}

	missingSF := []string{}
	missingNormal := []string{}
	for key := range required {
		v, ok := config[key]
		if ok && !isBlank(v) {
			continue
		}
		if _, ok := secret[key]; ok {
			missingSF = append(missingSF, key)
			continue
		}
		if _, ok := file[key]; ok {
			missingSF = append(missingSF, key)
			continue
		}
		missingNormal = append(missingNormal, key)
	}
	if len(missingSF) > 0 {
		slices.Sort(missingSF)
		return common.ConfigStatusInvalid, fmt.Sprintf("%s: %s", MissingSecretOrFileMessage, strings.Join(missingSF, ","))
	}
	if len(missingNormal) > 0 {
		slices.Sort(missingNormal)
		return common.ConfigStatusInvalid, fmt.Sprintf("缺少必填字段: %s", strings.Join(missingNormal, ","))
	}

	for key := range config {
		if _, ok := allowed[key]; !ok {
			return common.ConfigStatusInvalid, fmt.Sprintf("字段未在模板中定义: %s", key)
		}
	}
	for key, rule := range template.ValidationRules {
		v, ok := config[key]
		if !ok || isBlank(v) {
			continue
		}
		if msg := validateRule(v, rule); msg != "" {
			return common.ConfigStatusInvalid, fmt.Sprintf("%s: %s", key, msg)
		}
	}
	return common.ConfigStatusValid, ""
}

type RuntimeFlags struct {
	IncludedInRuntimeConfig bool
	IncludedInSnapshot      bool
	IncludedInSync          bool
}

// ResolveRuntimeFlags 派生运行态可用标记。
func ResolveRuntimeFlags(channelHidden, compatible, enabled bool, status common.ConfigStatus) RuntimeFlags {
	ok := !channelHidden && compatible && enabled && status == common.ConfigStatusValid
	return RuntimeFlags{
		IncludedInRuntimeConfig: ok,
		IncludedInSnapshot:      ok,
		IncludedInSync:          ok,
	}
}

func validateRule(v any, rule ValidationRule) string {
	s, ok := v.(string)
	if !ok {
		return "字段类型必须为字符串"
	}
	if rule.MinLen != nil && len(s) < *rule.MinLen {
		return fmt.Sprintf("长度不能小于 %d", *rule.MinLen)
	}
	if rule.MaxLen != nil && len(s) > *rule.MaxLen {
		return fmt.Sprintf("长度不能大于 %d", *rule.MaxLen)
	}
	if rule.Pattern != "" {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return "模板 pattern 非法"
		}
		if !re.MatchString(s) {
			return "不匹配 pattern"
		}
	}
	if len(rule.Enum) > 0 && !slices.Contains(rule.Enum, s) {
		return "不在允许范围"
	}
	return ""
}

func isBlank(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) == ""
}
