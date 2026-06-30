package plugin

import (
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func intp(v int) *int { return &v }

// 典型模板：appId 普通必填（含 pattern/长度规则）+ appSecret 密文必填，
// secret 字段同时出现在 form_schema 与 secret_fields（与 channel-login 模板一致）。
func sampleTemplate() ConfigTemplate {
	return ConfigTemplate{
		FormSchema: []TemplateField{
			{Key: "appId", Required: true, Scope: "both"},
			{Key: "appSecret", Required: true, Scope: "server"},
		},
		SecretFields: []string{"appSecret"},
		ValidationRules: map[string]ValidationRule{
			"appId": {MinLen: intp(2), MaxLen: intp(32), Pattern: "^[0-9A-Za-z_-]+$"},
		},
	}
}

// ResolvePluginConfigStatus —— enabled=false：恒 empty，不做必填校验（00 §3.4）。
func TestResolvePluginConfigStatus_DisabledIsEmpty(t *testing.T) {
	cases := []struct {
		name   string
		config map[string]any
	}{
		{"nil_config", nil},
		{"empty_config", map[string]any{}},
		{"config_present_still_empty", map[string]any{"appId": "abc", "appSecret": "x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, msg := ResolvePluginConfigStatus(false, sampleTemplate(), c.config)
			if status != common.ConfigStatusEmpty || msg != "" {
				t.Fatalf("enabled=false 应为 empty/空消息, got status=%s msg=%q", status, msg)
			}
		})
	}
}

// enabled=true 且必填齐全且规则通过 ⇒ valid。
func TestResolvePluginConfigStatus_ValidWhenComplete(t *testing.T) {
	status, msg := ResolvePluginConfigStatus(true, sampleTemplate(), map[string]any{
		"appId":     "app_123",
		"appSecret": "supersecret",
	})
	if status != common.ConfigStatusValid {
		t.Fatalf("齐全配置应 valid, got %s msg=%q", status, msg)
	}
	if msg != "" {
		t.Fatalf("valid 应无提示, got %q", msg)
	}
}

// 缺密文必填字段 ⇒ invalid，且提示走「敏感字段或文件字段」缺失语。
func TestResolvePluginConfigStatus_MissingSecretInvalid(t *testing.T) {
	status, msg := ResolvePluginConfigStatus(true, sampleTemplate(), map[string]any{
		"appId": "app_123",
	})
	if status != common.ConfigStatusInvalid {
		t.Fatalf("缺密文必填应 invalid, got %s", status)
	}
	if !strings.Contains(msg, MissingSecretOrFileMessage) || !strings.Contains(msg, "appSecret") {
		t.Fatalf("提示应含敏感字段缺失语与字段名, got %q", msg)
	}
}

// 缺文件必填字段 ⇒ invalid（file 字段缺失同样归入敏感/文件缺失语）。
func TestResolvePluginConfigStatus_MissingFileInvalid(t *testing.T) {
	tpl := ConfigTemplate{
		FileFields: []FileField{{Key: "license"}},
	}
	status, msg := ResolvePluginConfigStatus(true, tpl, map[string]any{})
	if status != common.ConfigStatusInvalid {
		t.Fatalf("缺文件必填应 invalid, got %s", status)
	}
	if !strings.Contains(msg, MissingSecretOrFileMessage) || !strings.Contains(msg, "license") {
		t.Fatalf("提示应含文件字段缺失, got %q", msg)
	}
}

// 缺普通必填字段（form_schema Required）⇒ invalid，走「缺少必填字段」语。
func TestResolvePluginConfigStatus_MissingNormalRequired(t *testing.T) {
	tpl := ConfigTemplate{
		FormSchema: []TemplateField{{Key: "displayName", Required: true}},
	}
	status, msg := ResolvePluginConfigStatus(true, tpl, map[string]any{})
	if status != common.ConfigStatusInvalid {
		t.Fatalf("缺普通必填应 invalid, got %s", status)
	}
	if !strings.Contains(msg, "缺少必填字段") || !strings.Contains(msg, "displayName") {
		t.Fatalf("提示应为缺少必填字段, got %q", msg)
	}
}

// validation_rules 的 Required 也参与必填判定。
func TestResolvePluginConfigStatus_RequiredViaValidationRule(t *testing.T) {
	tpl := ConfigTemplate{
		ValidationRules: map[string]ValidationRule{
			"token": {Required: true},
		},
	}
	status, _ := ResolvePluginConfigStatus(true, tpl, map[string]any{})
	if status != common.ConfigStatusInvalid {
		t.Fatalf("validation_rules.Required 缺失应 invalid, got %s", status)
	}
}

// 空白字符串（仅空格）视同缺失。
func TestResolvePluginConfigStatus_BlankCountsAsMissing(t *testing.T) {
	status, msg := ResolvePluginConfigStatus(true, sampleTemplate(), map[string]any{
		"appId":     "app_1",
		"appSecret": "   ",
	})
	if status != common.ConfigStatusInvalid {
		t.Fatalf("空白密文应判缺失 invalid, got %s msg=%q", status, msg)
	}
	if !strings.Contains(msg, "appSecret") {
		t.Fatalf("提示应含 appSecret, got %q", msg)
	}
}

// 敏感/文件缺失优先于普通字段缺失（同时缺多类时 SF 语优先）。
func TestResolvePluginConfigStatus_SecretMissingTakesPrecedence(t *testing.T) {
	tpl := ConfigTemplate{
		FormSchema:   []TemplateField{{Key: "displayName", Required: true}},
		SecretFields: []string{"appSecret"},
	}
	status, msg := ResolvePluginConfigStatus(true, tpl, map[string]any{})
	if status != common.ConfigStatusInvalid {
		t.Fatalf("应 invalid, got %s", status)
	}
	if !strings.Contains(msg, MissingSecretOrFileMessage) {
		t.Fatalf("同时缺普通+敏感时，敏感缺失语应优先, got %q", msg)
	}
}

// 多个敏感/文件缺失：消息内字段名按字典序稳定排序。
func TestResolvePluginConfigStatus_MissingSortedDeterministic(t *testing.T) {
	tpl := ConfigTemplate{
		SecretFields: []string{"zeta", "alpha"},
	}
	_, msg := ResolvePluginConfigStatus(true, tpl, map[string]any{})
	if !strings.Contains(msg, "alpha,zeta") {
		t.Fatalf("缺失字段应字典序稳定输出 alpha,zeta, got %q", msg)
	}
}

// 未在模板中定义的多余字段 ⇒ invalid。
func TestResolvePluginConfigStatus_UnknownFieldInvalid(t *testing.T) {
	status, msg := ResolvePluginConfigStatus(true, sampleTemplate(), map[string]any{
		"appId":     "app_1",
		"appSecret": "supersecret",
		"bogus":     "x",
	})
	if status != common.ConfigStatusInvalid {
		t.Fatalf("未知字段应 invalid, got %s", status)
	}
	if !strings.Contains(msg, "字段未在模板中定义") || !strings.Contains(msg, "bogus") {
		t.Fatalf("提示应指出未知字段, got %q", msg)
	}
}

// P3：secret/file 字段仅声明于 secret_fields_json/file_fields_json（未同列 form_schema_json）时，
// 其提交值不得被误判为「未在模板中定义」。allowed 集合须并入 secret/file 键。
func TestResolvePluginConfigStatus_SecretFileOnlyFieldsAllowed(t *testing.T) {
	tpl := ConfigTemplate{
		FormSchema:   []TemplateField{{Key: "appId", Required: true}},
		SecretFields: []string{"appSecret"},
		FileFields:   []FileField{{Key: "license"}},
	}
	status, msg := ResolvePluginConfigStatus(true, tpl, map[string]any{
		"appId":     "app_1",
		"appSecret": "supersecret",
		"license":   "license-blob",
	})
	if status != common.ConfigStatusValid {
		t.Fatalf("仅声明于 secret/file_fields 的字段提交应 valid, got %s msg=%q", status, msg)
	}
	if msg != "" {
		t.Fatalf("valid 应无提示, got %q", msg)
	}
}

// validation_rules：长度/正则/枚举/类型校验逐条命中 ⇒ invalid 且消息带字段前缀。
func TestResolvePluginConfigStatus_RuleViolations(t *testing.T) {
	cases := []struct {
		name      string
		tpl       ConfigTemplate
		config    map[string]any
		msgSubstr string
	}{
		{
			name: "min_len",
			tpl: ConfigTemplate{
				FormSchema:      []TemplateField{{Key: "appId", Required: true}},
				ValidationRules: map[string]ValidationRule{"appId": {MinLen: intp(5)}},
			},
			config:    map[string]any{"appId": "ab"},
			msgSubstr: "长度不能小于",
		},
		{
			name: "max_len",
			tpl: ConfigTemplate{
				FormSchema:      []TemplateField{{Key: "appId", Required: true}},
				ValidationRules: map[string]ValidationRule{"appId": {MaxLen: intp(3)}},
			},
			config:    map[string]any{"appId": "abcdef"},
			msgSubstr: "长度不能大于",
		},
		{
			name: "pattern_mismatch",
			tpl: ConfigTemplate{
				FormSchema:      []TemplateField{{Key: "appId", Required: true}},
				ValidationRules: map[string]ValidationRule{"appId": {Pattern: "^[0-9]+$"}},
			},
			config:    map[string]any{"appId": "abc"},
			msgSubstr: "不匹配 pattern",
		},
		{
			name: "pattern_invalid_template",
			tpl: ConfigTemplate{
				FormSchema:      []TemplateField{{Key: "appId", Required: true}},
				ValidationRules: map[string]ValidationRule{"appId": {Pattern: "([0-9]+"}},
			},
			config:    map[string]any{"appId": "123"},
			msgSubstr: "模板 pattern 非法",
		},
		{
			name: "enum_not_allowed",
			tpl: ConfigTemplate{
				FormSchema:      []TemplateField{{Key: "mode", Required: true}},
				ValidationRules: map[string]ValidationRule{"mode": {Enum: []string{"a", "b"}}},
			},
			config:    map[string]any{"mode": "c"},
			msgSubstr: "不在允许范围",
		},
		{
			name: "non_string_type",
			tpl: ConfigTemplate{
				FormSchema:      []TemplateField{{Key: "appId", Required: true}},
				ValidationRules: map[string]ValidationRule{"appId": {MinLen: intp(1)}},
			},
			config:    map[string]any{"appId": 123},
			msgSubstr: "字段类型必须为字符串",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, msg := ResolvePluginConfigStatus(true, c.tpl, c.config)
			if status != common.ConfigStatusInvalid {
				t.Fatalf("%s 应 invalid, got %s", c.name, status)
			}
			if !strings.Contains(msg, c.msgSubstr) {
				t.Fatalf("%s 消息应含 %q, got %q", c.name, c.msgSubstr, msg)
			}
		})
	}
}

// 空模板 + enabled=true + 空配置：无必填、无多余字段 ⇒ valid。
func TestResolvePluginConfigStatus_EmptyTemplateEnabledValid(t *testing.T) {
	status, msg := ResolvePluginConfigStatus(true, ConfigTemplate{}, map[string]any{})
	if status != common.ConfigStatusValid || msg != "" {
		t.Fatalf("空模板启用空配置应 valid, got %s msg=%q", status, msg)
	}
}

// 非必填字段未填，但提供合法可选字段 ⇒ valid（可选字段允许缺省）。
func TestResolvePluginConfigStatus_OptionalFieldOmittedValid(t *testing.T) {
	tpl := ConfigTemplate{
		FormSchema: []TemplateField{
			{Key: "appId", Required: true},
			{Key: "note", Required: false},
		},
	}
	status, _ := ResolvePluginConfigStatus(true, tpl, map[string]any{"appId": "a"})
	if status != common.ConfigStatusValid {
		t.Fatalf("可选字段缺省应 valid, got %s", status)
	}
}

// ResolveRuntimeFlags —— 运行态派生：仅全部条件满足才进入运行配置/快照/同步，且三标志同口径。
func TestResolveRuntimeFlags(t *testing.T) {
	cases := []struct {
		name     string
		hidden   bool
		compat   bool
		enabled  bool
		status   common.ConfigStatus
		wantIncl bool
	}{
		{"all_satisfied", false, true, true, common.ConfigStatusValid, true},
		{"channel_hidden", true, true, true, common.ConfigStatusValid, false},
		{"incompatible", false, false, true, common.ConfigStatusValid, false},
		{"disabled", false, true, false, common.ConfigStatusValid, false},
		{"status_empty", false, true, true, common.ConfigStatusEmpty, false},
		{"status_invalid", false, true, true, common.ConfigStatusInvalid, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := ResolveRuntimeFlags(c.hidden, c.compat, c.enabled, c.status)
			if f.IncludedInRuntimeConfig != c.wantIncl {
				t.Fatalf("IncludedInRuntimeConfig=%v want %v", f.IncludedInRuntimeConfig, c.wantIncl)
			}
			// 三标志必须同口径（快照/同步与运行配置一致，落实 00 §9）。
			if f.IncludedInSnapshot != f.IncludedInRuntimeConfig || f.IncludedInSync != f.IncludedInRuntimeConfig {
				t.Fatalf("快照/同步标志须与运行配置同口径: %+v", f)
			}
		})
	}
}
