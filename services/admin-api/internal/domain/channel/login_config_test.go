package channel

import (
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// L1 单元（就近，无 IO）：覆盖 channel-login 领域纯函数——
//   - ValidateLoginConfigAgainstTemplate：未知字段、必填（含 secret/file 标记）、
//     validation_rules（minLen/maxLen/min/max/pattern/format/enum）、config_status 推导边界。
//   - NewCopiedLoginConfig：复制创建强约束（普通字段复制、secret/file 清空、强制 invalid 绝不 empty）。
// 维度对照见 docs/architecture/v2/03-testing.md（L1）与 14-channel-login/spec.compact.md（§业务规则2/3/4）。

func lcIntPtr(v int) *int           { return &v }
func lcFloatPtr(v float64) *float64 { return &v }

// huaweiTemplate 模拟 compact 示例：appId 普通必填 + appSecret 密文字段。
func huaweiTemplate() ChannelLoginTemplate {
	return ChannelLoginTemplate{
		ID:              1,
		ChannelIDRef:    100,
		TemplateVersion: "v1",
		FormSchema: []ChannelLoginFormField{
			{Key: "appId", Label: "App ID", Component: "input", Required: true, Order: 10, Group: "basic"},
			{Key: "appSecret", Label: "App Secret", Component: "password", Required: true, Order: 20, Group: "secret"},
		},
		SecretFields: []string{"appSecret"},
		ValidationRules: map[string]ChannelLoginValidationRule{
			"appId":     {MinLen: lcIntPtr(1), MaxLen: lcIntPtr(64), Pattern: "^[0-9A-Za-z_-]+$"},
			"appSecret": {MinLen: lcIntPtr(8), MaxLen: lcIntPtr(256)},
		},
		Enabled: true,
	}
}

// ───────────────────────── config_status 推导边界 ─────────────────────────

// 空 config（无任何字段）⇒ empty，无 message/issues。
func TestValidateEmptyConfigYieldsEmpty(t *testing.T) {
	for _, cfg := range []map[string]any{nil, {}} {
		status, msg, issues := ValidateLoginConfigAgainstTemplate(cfg, huaweiTemplate())
		if status != common.ConfigStatusEmpty {
			t.Fatalf("empty config must be empty, got %s", status)
		}
		if msg != "" || len(issues) != 0 {
			t.Fatalf("empty config must carry no message/issues, msg=%q issues=%v", msg, issues)
		}
	}
}

// 全字段补齐且校验通过 ⇒ valid，无 message。
func TestValidateAllFieldsValid(t *testing.T) {
	cfg := map[string]any{"appId": "app-123_AZ", "appSecret": "supersecret"}
	status, msg, issues := ValidateLoginConfigAgainstTemplate(cfg, huaweiTemplate())
	if status != common.ConfigStatusValid {
		t.Fatalf("expected valid, got %s (issues=%v)", status, issues)
	}
	if msg != "" || len(issues) != 0 {
		t.Fatalf("valid config must carry no message/issues, msg=%q issues=%v", msg, issues)
	}
}

// 有字段但缺必填密钥 ⇒ invalid（绝不 empty），message 含缺失敏感/文件字段前缀。
func TestValidateMissingSecretYieldsInvalid(t *testing.T) {
	cfg := map[string]any{"appId": "app-123"}
	status, msg, issues := ValidateLoginConfigAgainstTemplate(cfg, huaweiTemplate())
	if status != common.ConfigStatusInvalid {
		t.Fatalf("missing secret must be invalid, got %s", status)
	}
	if !strings.HasPrefix(msg, CopiedMissingFieldsMessage) || !strings.Contains(msg, "appSecret") {
		t.Fatalf("message must flag missing secret field, got %q", msg)
	}
	if !hasIssue(issues, "appSecret", "required") {
		t.Fatalf("expected required issue on appSecret, got %v", issues)
	}
}

// 必填文件字段缺失 ⇒ invalid，且归入「缺少必填敏感字段或文件字段」。
func TestValidateMissingFileFieldYieldsInvalid(t *testing.T) {
	tpl := huaweiTemplate()
	tpl.FormSchema = append(tpl.FormSchema, ChannelLoginFormField{Key: "keystore", Component: "file", Required: true})
	tpl.FileFields = []ChannelLoginFileField{{Key: "keystore"}}
	cfg := map[string]any{"appId": "app-1", "appSecret": "supersecret"}
	status, msg, issues := ValidateLoginConfigAgainstTemplate(cfg, tpl)
	if status != common.ConfigStatusInvalid {
		t.Fatalf("missing file field must be invalid, got %s", status)
	}
	if !strings.Contains(msg, "keystore") {
		t.Fatalf("message must flag missing file field keystore, got %q", msg)
	}
	if !hasIssue(issues, "keystore", "required") {
		t.Fatalf("expected required issue on keystore, got %v", issues)
	}
}

// ───────────────────────── 未知字段拒绝 ─────────────────────────

func TestValidateUnknownFieldRejected(t *testing.T) {
	cfg := map[string]any{"appId": "app-1", "appSecret": "supersecret", "bogus": "x"}
	status, _, issues := ValidateLoginConfigAgainstTemplate(cfg, huaweiTemplate())
	if status != common.ConfigStatusInvalid {
		t.Fatalf("unknown field must be invalid, got %s", status)
	}
	if !hasIssue(issues, "bogus", "unknown") {
		t.Fatalf("expected unknown issue on bogus field, got %v", issues)
	}
}

// ───────────────────────── validation_rules 逐项 ─────────────────────────

func TestValidateRuleMinLen(t *testing.T) {
	tpl := singleRuleTemplate("code", ChannelLoginValidationRule{MinLen: lcIntPtr(4)})
	status, _, issues := ValidateLoginConfigAgainstTemplate(map[string]any{"code": "ab"}, tpl)
	if status != common.ConfigStatusInvalid || !hasIssue(issues, "code", "minLen") {
		t.Fatalf("expected minLen issue, got status=%s issues=%v", status, issues)
	}
}

func TestValidateRuleMaxLen(t *testing.T) {
	tpl := singleRuleTemplate("code", ChannelLoginValidationRule{MaxLen: lcIntPtr(3)})
	status, _, issues := ValidateLoginConfigAgainstTemplate(map[string]any{"code": "abcd"}, tpl)
	if status != common.ConfigStatusInvalid || !hasIssue(issues, "code", "maxLen") {
		t.Fatalf("expected maxLen issue, got status=%s issues=%v", status, issues)
	}
}

func TestValidateRuleMinMaxNumeric(t *testing.T) {
	tpl := singleRuleTemplate("port", ChannelLoginValidationRule{Min: lcFloatPtr(10), Max: lcFloatPtr(20)})
	if _, _, issues := ValidateLoginConfigAgainstTemplate(map[string]any{"port": float64(5)}, tpl); !hasIssue(issues, "port", "min") {
		t.Fatalf("expected min issue, got %v", issues)
	}
	if _, _, issues := ValidateLoginConfigAgainstTemplate(map[string]any{"port": float64(25)}, tpl); !hasIssue(issues, "port", "max") {
		t.Fatalf("expected max issue, got %v", issues)
	}
	if status, _, _ := ValidateLoginConfigAgainstTemplate(map[string]any{"port": float64(15)}, tpl); status != common.ConfigStatusValid {
		t.Fatalf("in-range numeric must be valid, got %s", status)
	}
}

// 数值规则遇非数值 ⇒ type 校验失败。
func TestValidateRuleNumericTypeMismatch(t *testing.T) {
	tpl := singleRuleTemplate("port", ChannelLoginValidationRule{Min: lcFloatPtr(1)})
	_, _, issues := ValidateLoginConfigAgainstTemplate(map[string]any{"port": "not-a-number"}, tpl)
	if !hasIssue(issues, "port", "type") {
		t.Fatalf("expected type issue for numeric rule on string, got %v", issues)
	}
}

func TestValidateRulePattern(t *testing.T) {
	tpl := singleRuleTemplate("appId", ChannelLoginValidationRule{Pattern: "^[0-9A-Za-z_-]+$"})
	if _, _, issues := ValidateLoginConfigAgainstTemplate(map[string]any{"appId": "bad id!"}, tpl); !hasIssue(issues, "appId", "pattern") {
		t.Fatalf("expected pattern issue, got %v", issues)
	}
	if status, _, _ := ValidateLoginConfigAgainstTemplate(map[string]any{"appId": "ok_ID-1"}, tpl); status != common.ConfigStatusValid {
		t.Fatalf("matching pattern must be valid, got %s", status)
	}
}

func TestValidateRuleFormat(t *testing.T) {
	cases := []struct {
		format string
		bad    string
		good   string
	}{
		{"url", "not a url", "https://example.com/cb"},
		{"email", "nope", "a@b.com"},
		{"host", "https://x/y", "cdn.example.com"},
	}
	for _, c := range cases {
		tpl := singleRuleTemplate("f", ChannelLoginValidationRule{Format: c.format})
		if _, _, issues := ValidateLoginConfigAgainstTemplate(map[string]any{"f": c.bad}, tpl); !hasIssue(issues, "f", "format") {
			t.Fatalf("format %s: expected format issue for %q, got %v", c.format, c.bad, issues)
		}
		if status, _, _ := ValidateLoginConfigAgainstTemplate(map[string]any{"f": c.good}, tpl); status != common.ConfigStatusValid {
			t.Fatalf("format %s: %q must be valid", c.format, c.good)
		}
	}
}

func TestValidateRuleEnum(t *testing.T) {
	tpl := singleRuleTemplate("region", ChannelLoginValidationRule{Enum: []string{"cn", "sg"}})
	if _, _, issues := ValidateLoginConfigAgainstTemplate(map[string]any{"region": "us"}, tpl); !hasIssue(issues, "region", "enum") {
		t.Fatalf("expected enum issue, got %v", issues)
	}
	if status, _, _ := ValidateLoginConfigAgainstTemplate(map[string]any{"region": "cn"}, tpl); status != common.ConfigStatusValid {
		t.Fatalf("enum member must be valid, got %s", status)
	}
}

// validation_rules.required 单独成必填来源（即便 form_schema 未标 required）。
func TestValidateRuleRequiredFromRules(t *testing.T) {
	tpl := ChannelLoginTemplate{
		FormSchema:      []ChannelLoginFormField{{Key: "a"}, {Key: "b"}},
		ValidationRules: map[string]ChannelLoginValidationRule{"b": {Required: true}},
	}
	status, _, issues := ValidateLoginConfigAgainstTemplate(map[string]any{"a": "x"}, tpl)
	if status != common.ConfigStatusInvalid || !hasIssue(issues, "b", "required") {
		t.Fatalf("rule-required field b missing must be invalid+required, got status=%s issues=%v", status, issues)
	}
}

// ───────────────────────── 复制创建强约束（红线） ─────────────────────────

// 复制：普通字段复制；secret/file 字段一律清空；强制 invalid 且非 empty；固定提示。
func TestNewCopiedLoginConfigClearsSecretAndFile(t *testing.T) {
	tpl := huaweiTemplate()
	tpl.FormSchema = append(tpl.FormSchema, ChannelLoginFormField{Key: "keystore", Component: "file", Required: true})
	tpl.FileFields = []ChannelLoginFileField{{Key: "keystore"}}
	source := &ChannelLoginConfig{
		GameChannelIDRef: 1,
		ConfigJSON: map[string]any{
			"appId":     "app-from-source",
			"appSecret": "enc:source-secret",
			"keystore":  "storage://keystore-ref",
		},
	}

	copied := NewCopiedLoginConfig(2, &tpl, source)

	if copied.ConfigStatus != common.ConfigStatusInvalid {
		t.Fatalf("copy must force invalid (never empty), got %s", copied.ConfigStatus)
	}
	if copied.Enabled {
		t.Fatal("copied config must be disabled")
	}
	if copied.LastCheckMessage != CopiedMissingFieldsMessage {
		t.Fatalf("copied message must be fixed hint, got %q", copied.LastCheckMessage)
	}
	if copied.GameChannelIDRef != 2 {
		t.Fatalf("copied must bind new game channel id, got %d", copied.GameChannelIDRef)
	}
	if copied.ConfigJSON["appId"] != "app-from-source" {
		t.Fatalf("normal field appId must be copied, got %v", copied.ConfigJSON["appId"])
	}
	if _, ok := copied.ConfigJSON["appSecret"]; ok {
		t.Fatalf("secret field must be cleared on copy, got %v", copied.ConfigJSON["appSecret"])
	}
	if _, ok := copied.ConfigJSON["keystore"]; ok {
		t.Fatalf("file field must be cleared on copy, got %v", copied.ConfigJSON["keystore"])
	}
}

// 模板缺失（nil）：仍返回 invalid 占位（绝不 empty），不带任何字段。
func TestNewCopiedLoginConfigNilTemplateStillInvalid(t *testing.T) {
	copied := NewCopiedLoginConfig(3, nil, &ChannelLoginConfig{ConfigJSON: map[string]any{"appId": "x"}})
	if copied.ConfigStatus != common.ConfigStatusInvalid {
		t.Fatalf("nil-template copy must still be invalid, got %s", copied.ConfigStatus)
	}
	if len(copied.ConfigJSON) != 0 {
		t.Fatalf("nil-template copy must not carry fields, got %v", copied.ConfigJSON)
	}
	if copied.LastCheckMessage != CopiedMissingFieldsMessage {
		t.Fatalf("nil-template copy must carry fixed hint, got %q", copied.LastCheckMessage)
	}
}

// 复制结果立即过模板校验仍为 invalid（红线：清空 secret/file 后绝不会被判 valid/empty）。
func TestCopiedConfigRevalidatesInvalid(t *testing.T) {
	tpl := huaweiTemplate()
	source := &ChannelLoginConfig{ConfigJSON: map[string]any{"appId": "app-1", "appSecret": "enc:s"}}
	copied := NewCopiedLoginConfig(2, &tpl, source)
	status, _, _ := ValidateLoginConfigAgainstTemplate(copied.ConfigJSON, tpl)
	if status != common.ConfigStatusInvalid {
		t.Fatalf("revalidated copied config must remain invalid, got %s", status)
	}
}

// ───────────────────────── helpers ─────────────────────────

func singleRuleTemplate(field string, rule ChannelLoginValidationRule) ChannelLoginTemplate {
	return ChannelLoginTemplate{
		FormSchema:      []ChannelLoginFormField{{Key: field, Required: rule.Required}},
		ValidationRules: map[string]ChannelLoginValidationRule{field: rule},
	}
}

func hasIssue(issues []ValidationIssue, field, rule string) bool {
	for _, it := range issues {
		if it.Field == field && it.Rule == rule {
			return true
		}
	}
	return false
}
