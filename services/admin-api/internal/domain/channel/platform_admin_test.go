package channel

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestValidateChannelID(t *testing.T) {
	valid := []string{"google", "huawei_cn", "mini2"}
	for _, id := range valid {
		if issues := ValidateChannelID(id); len(issues) != 0 {
			t.Fatalf("channelId %q should be valid, got %v", id, issues)
		}
	}
	invalid := []string{"", "Google", "huawei-cn", "_cn", "渠道"}
	for _, id := range invalid {
		if issues := ValidateChannelID(id); len(issues) == 0 {
			t.Fatalf("channelId %q should be rejected", id)
		}
	}
}

func TestValidateChannelMaster(t *testing.T) {
	ok := Channel{ChannelID: "google", ChannelName: "Google Play", ChannelType: ChannelTypeStore, Region: ChannelRegionOverseas, Sort: 10}
	if issues := ValidateChannelMaster(ok); len(issues) != 0 {
		t.Fatalf("expected valid, got %v", issues)
	}

	cases := map[string]Channel{
		"空渠道名":      {ChannelName: "  ", ChannelType: ChannelTypeStore, Region: ChannelRegionOverseas},
		"非法类型":      {ChannelName: "X", ChannelType: "unknown", Region: ChannelRegionOverseas},
		"非法 region": {ChannelName: "X", ChannelType: ChannelTypeStore, Region: ChannelRegion("cn")},
		"排序越界":      {ChannelName: "X", ChannelType: ChannelTypeStore, Region: ChannelRegionOverseas, Sort: -1},
	}
	for name, ch := range cases {
		if issues := ValidateChannelMaster(ch); len(issues) == 0 {
			t.Fatalf("%s 应被拒绝", name)
		}
	}
}

func TestValidateChannelPolicy(t *testing.T) {
	ok := ChannelPolicy{LoginMode: common.LoginModeChannelOnly, PaymentMode: common.PaymentModeHybrid}
	if issues := ValidateChannelPolicy(ok); len(issues) != 0 {
		t.Fatalf("expected valid, got %v", issues)
	}
	bad := ChannelPolicy{LoginMode: common.LoginMode("sso"), PaymentMode: common.PaymentMode("free")}
	if issues := ValidateChannelPolicy(bad); len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %v", issues)
	}
}

func validTemplate() ChannelTemplate {
	return ChannelTemplate{
		Kind:            ChannelTemplateKindLogin,
		TemplateVersion: "v1",
		FormSchema: []ChannelLoginFormField{
			{Key: "appId", Label: "App ID", Component: "input", Required: true, Order: 10, Scope: "both"},
			{Key: "appSecret", Label: "App Secret", Component: "password", Required: true, Order: 20, Scope: "server"},
		},
		SecretFields:    []string{"appSecret"},
		FileFields:      []ChannelLoginFileField{},
		ValidationRules: map[string]ChannelLoginValidationRule{"appId": {Pattern: `^[0-9A-Za-z_-]+$`}},
		Enabled:         true,
	}
}

func TestValidateChannelTemplateAcceptsWellFormed(t *testing.T) {
	if issues := ValidateChannelTemplate(validTemplate()); len(issues) != 0 {
		t.Fatalf("expected valid, got %v", issues)
	}
}

func TestValidateChannelTemplateRejectsUndeclaredKeys(t *testing.T) {
	tpl := validTemplate()
	tpl.SecretFields = append(tpl.SecretFields, "ghost")
	tpl.ValidationRules["phantom"] = ChannelLoginValidationRule{Required: true}
	issues := ValidateChannelTemplate(tpl)
	if len(issues) != 2 {
		t.Fatalf("expected secret/rule 未声明各 1 条，got %v", issues)
	}
}

func TestValidateChannelTemplateRequiresSecretForPasswordComponent(t *testing.T) {
	tpl := validTemplate()
	tpl.SecretFields = []string{}
	issues := ValidateChannelTemplate(tpl)
	if len(issues) != 1 || issues[0].Field != "secretFieldsJson" {
		t.Fatalf("口令字段未登记为敏感字段应被拒绝，got %v", issues)
	}
}

func TestValidateChannelTemplateRequiresFileFieldRegistration(t *testing.T) {
	tpl := validTemplate()
	tpl.FormSchema = append(tpl.FormSchema, ChannelLoginFormField{Key: "keystore", Label: "Keystore", Component: "file"})
	issues := ValidateChannelTemplate(tpl)
	if len(issues) != 1 || issues[0].Field != "fileFieldsJson" {
		t.Fatalf("文件组件未登记到文件字段应被拒绝，got %v", issues)
	}
}

func TestValidateChannelTemplateRejectsDuplicateAndBadFields(t *testing.T) {
	tpl := validTemplate()
	tpl.TemplateVersion = "v 1"
	tpl.FormSchema = []ChannelLoginFormField{
		{Key: "appId", Label: "App ID", Component: "input"},
		{Key: "appId", Label: "重复", Component: "input"},
		{Key: "3bad", Label: "非法 key", Component: "input"},
		{Key: "mode", Label: "模式", Component: "select"},
	}
	tpl.SecretFields = []string{}
	tpl.ValidationRules = map[string]ChannelLoginValidationRule{"appId": {Pattern: "([a-z"}}
	issues := ValidateChannelTemplate(tpl)
	rules := map[string]int{}
	for _, item := range issues {
		rules[item.Rule]++
	}
	for _, want := range []string{"pattern", "duplicate", "required"} {
		if rules[want] == 0 {
			t.Fatalf("expected issue rule %q, got %v", want, issues)
		}
	}
}

func TestValidateChannelTemplateRejectsEmptyFormSchema(t *testing.T) {
	tpl := validTemplate()
	tpl.FormSchema = nil
	tpl.SecretFields = nil
	tpl.ValidationRules = nil
	issues := ValidateChannelTemplate(tpl)
	if len(issues) != 1 || issues[0].Field != "formSchemaJson" {
		t.Fatalf("空表单应被拒绝，got %v", issues)
	}
}

func TestIsValidChannelTemplateKind(t *testing.T) {
	if !IsValidChannelTemplateKind(ChannelTemplateKindLogin) || !IsValidChannelTemplateKind(ChannelTemplateKindIAP) {
		t.Fatal("login/iap 应为合法 kind")
	}
	if IsValidChannelTemplateKind(ChannelTemplateKind("plugin")) {
		t.Fatal("未知 kind 应被拒绝")
	}
}
