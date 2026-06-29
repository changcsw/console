package accountauth

import (
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func intPtr(v int) *int { return &v }

// 说明：ValidateConfigAgainstTemplate 仅返回 invalid / valid 两态；
// empty 态由 app 层（未启用且无配置）决定，不属本纯函数职责。
// 本测试覆盖：valid、各 invalid 分支（隐式必填 secret/file、显式必填、各 validationRules）、边界。

func TestValidate_EmptyTemplate_EnabledIsValid(t *testing.T) {
	// phone/email 等无第三方密钥类型：空四件套，配置为空即 valid（compact 关键假设）。
	status, msg := ValidateConfigAgainstTemplate(map[string]any{}, Template{TemplateVersion: "v1"})
	if status != common.ConfigStatusValid || msg != "" {
		t.Fatalf("empty template should be valid, got status=%s msg=%q", status, msg)
	}
}

func TestValidate_MissingSecret_IsInvalidWithSecretMessage(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		SecretFields:    []string{"clientSecret"},
	}
	status, msg := ValidateConfigAgainstTemplate(map[string]any{}, tpl)
	if status != common.ConfigStatusInvalid {
		t.Fatalf("missing secret should be invalid, got %s", status)
	}
	if !strings.HasPrefix(msg, MissingSecretOrFileMessage) || !strings.Contains(msg, "clientSecret") {
		t.Fatalf("message must flag missing secret field, got %q", msg)
	}
}

func TestValidate_EmptyStringSecret_TreatedAsMissing(t *testing.T) {
	tpl := Template{TemplateVersion: "v1", SecretFields: []string{"clientSecret"}}
	status, msg := ValidateConfigAgainstTemplate(map[string]any{"clientSecret": "   "}, tpl)
	if status != common.ConfigStatusInvalid || !strings.HasPrefix(msg, MissingSecretOrFileMessage) {
		t.Fatalf("whitespace secret should count as missing, got status=%s msg=%q", status, msg)
	}
}

func TestValidate_MissingFile_IsInvalidWithSecretOrFileMessage(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		FileFields:      []FileField{{Key: "keystore"}},
	}
	status, msg := ValidateConfigAgainstTemplate(map[string]any{}, tpl)
	if status != common.ConfigStatusInvalid {
		t.Fatalf("missing file should be invalid, got %s", status)
	}
	if !strings.HasPrefix(msg, MissingSecretOrFileMessage) || !strings.Contains(msg, "keystore") {
		t.Fatalf("message must flag missing file field, got %q", msg)
	}
}

func TestValidate_MissingRequiredNonSecret_IsInvalid(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		FormSchema:      []FormField{{Key: "clientId", Required: true}},
	}
	status, msg := ValidateConfigAgainstTemplate(map[string]any{}, tpl)
	if status != common.ConfigStatusInvalid {
		t.Fatalf("missing required field should be invalid, got %s", status)
	}
	if !strings.Contains(msg, "缺少必填字段") || !strings.Contains(msg, "clientId") {
		t.Fatalf("message must flag missing normal required field, got %q", msg)
	}
}

func TestValidate_RequiredViaValidationRules(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		ValidationRules: map[string]ValidationRule{"clientId": {Required: true}},
	}
	status, _ := ValidateConfigAgainstTemplate(map[string]any{}, tpl)
	if status != common.ConfigStatusInvalid {
		t.Fatalf("required via validationRules should be invalid when missing, got %s", status)
	}
}

func TestValidate_MinLenAndMaxLen(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		ValidationRules: map[string]ValidationRule{"clientId": {MinLen: intPtr(3), MaxLen: intPtr(5)}},
	}
	// 太短。
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"clientId": "ab"}, tpl); status != common.ConfigStatusInvalid {
		t.Fatalf("below minLen should be invalid")
	}
	// 太长。
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"clientId": "abcdef"}, tpl); status != common.ConfigStatusInvalid {
		t.Fatalf("above maxLen should be invalid")
	}
	// 边界：等于 minLen。
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"clientId": "abc"}, tpl); status != common.ConfigStatusValid {
		t.Fatalf("exactly minLen should be valid")
	}
	// 边界：等于 maxLen。
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"clientId": "abcde"}, tpl); status != common.ConfigStatusValid {
		t.Fatalf("exactly maxLen should be valid")
	}
}

func TestValidate_FormatURL(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		ValidationRules: map[string]ValidationRule{"redirectUri": {Format: "url"}},
	}
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"redirectUri": "not-a-url"}, tpl); status != common.ConfigStatusInvalid {
		t.Fatalf("invalid url should be invalid")
	}
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"redirectUri": "https://example.com/cb"}, tpl); status != common.ConfigStatusValid {
		t.Fatalf("valid url should be valid")
	}
}

func TestValidate_FormatEmail(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		ValidationRules: map[string]ValidationRule{"contact": {Format: "email"}},
	}
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"contact": "nope"}, tpl); status != common.ConfigStatusInvalid {
		t.Fatalf("invalid email should be invalid")
	}
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"contact": "a@b.com"}, tpl); status != common.ConfigStatusValid {
		t.Fatalf("valid email should be valid")
	}
}

func TestValidate_Pattern(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		ValidationRules: map[string]ValidationRule{"code": {Pattern: "^[a-z]+$"}},
	}
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"code": "ABC"}, tpl); status != common.ConfigStatusInvalid {
		t.Fatalf("pattern mismatch should be invalid")
	}
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"code": "abc"}, tpl); status != common.ConfigStatusValid {
		t.Fatalf("pattern match should be valid")
	}
}

func TestValidate_Enum(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		ValidationRules: map[string]ValidationRule{"region": {Enum: []string{"JP", "KR"}}},
	}
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"region": "US"}, tpl); status != common.ConfigStatusInvalid {
		t.Fatalf("value out of enum should be invalid")
	}
	if status, _ := ValidateConfigAgainstTemplate(map[string]any{"region": "JP"}, tpl); status != common.ConfigStatusValid {
		t.Fatalf("value in enum should be valid")
	}
}

func TestValidate_GoogleLikeTemplate_FullValid(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		FormSchema: []FormField{
			{Key: "clientId", Required: true, Component: "input"},
			{Key: "clientSecret", Required: true, Component: "password", Scope: "server"},
			{Key: "redirectUri", Required: true, Component: "input"},
		},
		SecretFields:    []string{"clientSecret"},
		ValidationRules: map[string]ValidationRule{"clientId": {MinLen: intPtr(1)}, "redirectUri": {Format: "url"}},
	}
	cfg := map[string]any{
		"clientId":     "abc",
		"clientSecret": "enc:xxxx",
		"redirectUri":  "https://example.com/oauth/cb",
	}
	if status, msg := ValidateConfigAgainstTemplate(cfg, tpl); status != common.ConfigStatusValid || msg != "" {
		t.Fatalf("complete google config should be valid, got status=%s msg=%q", status, msg)
	}
}

func TestValidate_GoogleLikeTemplate_MissingSecretIsInvalid(t *testing.T) {
	tpl := Template{
		TemplateVersion: "v1",
		FormSchema: []FormField{
			{Key: "clientId", Required: true},
			{Key: "redirectUri", Required: true},
		},
		SecretFields:    []string{"clientSecret"},
		ValidationRules: map[string]ValidationRule{"clientId": {MinLen: intPtr(1)}, "redirectUri": {Format: "url"}},
	}
	cfg := map[string]any{"clientId": "abc", "redirectUri": "https://example.com/cb"}
	status, msg := ValidateConfigAgainstTemplate(cfg, tpl)
	if status != common.ConfigStatusInvalid || !strings.Contains(msg, "clientSecret") {
		t.Fatalf("missing clientSecret must be invalid flagging clientSecret, got status=%s msg=%q", status, msg)
	}
}
