package channel

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// ChannelTemplateKind 渠道模版种类。两类模版表（platform.channel_login_templates /
// platform.channel_iap_templates）结构同构（四件套 + template_version + enabled，00 §4.4.1），
// 平台侧用同一套实体与规则维护，仅落库表不同。
type ChannelTemplateKind string

// 渠道模版种类取值。
const (
	ChannelTemplateKindLogin ChannelTemplateKind = "login"
	ChannelTemplateKindIAP   ChannelTemplateKind = "iap"
)

// IsValidChannelTemplateKind 校验模版种类。
func IsValidChannelTemplateKind(k ChannelTemplateKind) bool {
	return k == ChannelTemplateKindLogin || k == ChannelTemplateKindIAP
}

// ChannelTemplate 平台渠道模版实体（登录/IAP 共用；Kind 决定落库表）。
// ChannelID 为装配出的渠道业务键，仅用于响应展示。
type ChannelTemplate struct {
	ID              int64
	Kind            ChannelTemplateKind
	ChannelIDRef    int64
	ChannelID       string
	TemplateVersion string
	FormSchema      []ChannelLoginFormField
	SecretFields    []string
	FileFields      []ChannelLoginFileField
	ValidationRules map[string]ChannelLoginValidationRule
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

var (
	channelIDPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)
	templateVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	templateFieldKeyPatt   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

// 模版字段组件取值集合（与前端表单渲染器一致）。
var templateComponents = []string{"input", "password", "textarea", "number", "select", "switch", "file", "json"}

// 模版字段作用域取值集合（空串表示不区分）。
var templateScopes = []string{"", "client", "server", "both"}

// ValidateChannelMaster 校验平台渠道主数据（创建/编辑共用，无 IO）。
// channelID 为业务键，仅创建时校验格式，编辑时不可改。
func ValidateChannelMaster(c Channel) []ValidationIssue {
	issues := []ValidationIssue{}
	if strings.TrimSpace(c.ChannelName) == "" || len(c.ChannelName) > 64 {
		issues = append(issues, ValidationIssue{Field: "channelName", Rule: "length", Message: "渠道名必填且不超过 64 字符"})
	}
	if !IsValidChannelType(c.ChannelType) {
		issues = append(issues, ValidationIssue{Field: "channelType", Rule: "enum", Message: "渠道类型非法"})
	}
	if c.Region != ChannelRegionDomestic && c.Region != ChannelRegionOverseas {
		issues = append(issues, ValidationIssue{Field: "region", Rule: "enum", Message: "region 只能为 domestic/overseas"})
	}
	if c.Sort < 0 || c.Sort > 9999 {
		issues = append(issues, ValidationIssue{Field: "sort", Rule: "range", Message: "排序值需在 0-9999 之间"})
	}
	return issues
}

// ValidateChannelID 校验渠道业务键格式（小写字母/数字/下划线，字母数字开头，≤64）。
func ValidateChannelID(channelID string) []ValidationIssue {
	if !channelIDPattern.MatchString(channelID) || len(channelID) > 64 {
		return []ValidationIssue{{
			Field:   "channelId",
			Rule:    "pattern",
			Message: "渠道 ID 只能用小写字母/数字/下划线，且以字母或数字开头，长度不超过 64",
		}}
	}
	return nil
}

// ValidateChannelPolicy 校验渠道策略枚举（无 IO）。
func ValidateChannelPolicy(p ChannelPolicy) []ValidationIssue {
	issues := []ValidationIssue{}
	switch p.LoginMode {
	case common.LoginModeChannelOnly, common.LoginModeAccountSystem:
	default:
		issues = append(issues, ValidationIssue{Field: "loginMode", Rule: "enum", Message: "loginMode 只能为 channel_only/account_system"})
	}
	switch p.PaymentMode {
	case common.PaymentModeChannelOnly, common.PaymentModeHybrid, common.PaymentModeCashierOnly:
	default:
		issues = append(issues, ValidationIssue{Field: "paymentMode", Rule: "enum", Message: "paymentMode 只能为 channel_only/hybrid/cashier_only"})
	}
	return issues
}

// ValidateChannelTemplate 校验渠道模版四件套自洽性（无 IO）：
//   - template_version 格式；form_schema 非空、key 唯一合法、component/scope 枚举
//   - secret_fields / file_fields / validation_rules 的 key 必须在 form_schema 中声明
//   - component=file ⇔ 出现在 file_fields；component=password ⇒ 必须声明为 secret_fields
//     （避免管理员建出「口令字段明文入库」的模版）
//   - validation_rules 的 pattern 必须可编译
func ValidateChannelTemplate(tpl ChannelTemplate) []ValidationIssue {
	issues := []ValidationIssue{}
	if !templateVersionPattern.MatchString(tpl.TemplateVersion) || len(tpl.TemplateVersion) > 32 {
		issues = append(issues, ValidationIssue{Field: "templateVersion", Rule: "pattern", Message: "版本号只能用字母/数字/点/横线/下划线，长度不超过 32"})
	}
	if len(tpl.FormSchema) == 0 {
		issues = append(issues, ValidationIssue{Field: "formSchemaJson", Rule: "required", Message: "至少需要一个表单字段"})
	}

	keys := map[string]ChannelLoginFormField{}
	for i, f := range tpl.FormSchema {
		field := "formSchemaJson[" + strconv.Itoa(i) + "]"
		if !templateFieldKeyPatt.MatchString(f.Key) || len(f.Key) > 64 {
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
		if !slices.Contains(templateComponents, f.Component) {
			issues = append(issues, ValidationIssue{Field: field + ".component", Rule: "enum", Message: "组件类型非法：" + f.Component})
		}
		if !slices.Contains(templateScopes, f.Scope) {
			issues = append(issues, ValidationIssue{Field: field + ".scope", Rule: "enum", Message: "scope 只能为 client/server/both"})
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

	for key, rule := range tpl.ValidationRules {
		if _, ok := keys[key]; !ok {
			issues = append(issues, ValidationIssue{Field: "validationRulesJson", Rule: "unknown", Message: "校验规则字段未在表单中声明：" + key})
			continue
		}
		if rule.Pattern == "" {
			continue
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			issues = append(issues, ValidationIssue{Field: "validationRulesJson." + key + ".pattern", Rule: "pattern", Message: "正则表达式非法"})
		}
	}
	return issues
}
