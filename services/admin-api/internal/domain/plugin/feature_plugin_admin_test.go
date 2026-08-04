package plugin

import "testing"

// 纯规则单测：只覆盖系统管理员维护插件主数据时的自洽性判定，不涉及 IO。
// 游戏侧运行时规则（ValidatePluginRegionCompatibility / ResolvePluginConfigStatus）见同包其它测试。

func hasIssueField(issues []ValidationIssue, field string) bool {
	for _, item := range issues {
		if item.Field == field {
			return true
		}
	}
	return false
}

func TestValidateCategoryCode(t *testing.T) {
	valid := []string{"login", "payment", "push_v2", "ad2"}
	for _, code := range valid {
		if issues := ValidateCategoryCode(code); len(issues) != 0 {
			t.Fatalf("%q should be valid, got %+v", code, issues)
		}
	}
	invalid := []string{"", "Login", "log-in", "1login", "_login", "login "}
	for _, code := range invalid {
		if issues := ValidateCategoryCode(code); !hasIssueField(issues, "categoryCode") {
			t.Fatalf("%q should be rejected, got %+v", code, issues)
		}
	}
}

func TestValidatePluginID(t *testing.T) {
	if issues := ValidatePluginID("realname"); len(issues) != 0 {
		t.Fatalf("realname should be valid, got %+v", issues)
	}
	// 与渠道 ID 的差别：插件 ID 必须以字母开头（数字开头无业务含义）。
	for _, id := range []string{"", "Realname", "real-name", "1realname"} {
		if issues := ValidatePluginID(id); !hasIssueField(issues, "pluginId") {
			t.Fatalf("%q should be rejected, got %+v", id, issues)
		}
	}
}

func TestValidateFeaturePluginCategory(t *testing.T) {
	if issues := ValidateFeaturePluginCategory(FeaturePluginCategory{CategoryName: "登录类", Sort: 10}); len(issues) != 0 {
		t.Fatalf("should be valid, got %+v", issues)
	}
	if issues := ValidateFeaturePluginCategory(FeaturePluginCategory{CategoryName: "  ", Sort: 10}); !hasIssueField(issues, "categoryName") {
		t.Fatalf("blank name should be rejected, got %+v", issues)
	}
	for _, sortOrder := range []int{-1, 10000} {
		issues := ValidateFeaturePluginCategory(FeaturePluginCategory{CategoryName: "登录类", Sort: sortOrder})
		if !hasIssueField(issues, "sort") {
			t.Fatalf("sort %d should be rejected, got %+v", sortOrder, issues)
		}
	}
}

func TestValidateFeaturePluginMaster(t *testing.T) {
	base := FeaturePlugin{PluginName: "实名认证", Region: RegionDomestic, Sort: 1}
	if issues := ValidateFeaturePluginMaster(base); len(issues) != 0 {
		t.Fatalf("should be valid, got %+v", issues)
	}

	blank := base
	blank.PluginName = " "
	if issues := ValidateFeaturePluginMaster(blank); !hasIssueField(issues, "pluginName") {
		t.Fatalf("blank name should be rejected, got %+v", issues)
	}

	// region 只认 domestic/overseas：渠道那套发行市场枚举（CN/GLOBAL...）在插件上非法。
	for _, region := range []string{"", "CN", "GLOBAL", "Domestic"} {
		bad := base
		bad.Region = region
		if issues := ValidateFeaturePluginMaster(bad); !hasIssueField(issues, "region") {
			t.Fatalf("region %q should be rejected, got %+v", region, issues)
		}
	}

	// 未归类（nil）合法；显式传非正数的分类主键非法。
	unassigned := base
	unassigned.CategoryIDRef = nil
	if issues := ValidateFeaturePluginMaster(unassigned); len(issues) != 0 {
		t.Fatalf("nil category should be valid, got %+v", issues)
	}
	zero := int64(0)
	bad := base
	bad.CategoryIDRef = &zero
	if issues := ValidateFeaturePluginMaster(bad); !hasIssueField(issues, "categoryId") {
		t.Fatalf("zero category id should be rejected, got %+v", issues)
	}
}

func TestValidateFeaturePluginTemplate(t *testing.T) {
	valid := FeaturePluginTemplate{
		TemplateVersion: "v1",
		FormSchema: []PluginFormField{
			{Key: "appId", Label: "App ID", Component: "input", Required: true, Order: 10, Scope: "both"},
			{Key: "appSecret", Label: "App Secret", Component: "password", Required: true, Order: 20, Scope: "server"},
			{Key: "cert", Label: "证书", Component: "file", Order: 30},
		},
		SecretFields:    []string{"appSecret"},
		FileFields:      []PluginFileField{{Key: "cert"}},
		ValidationRules: map[string]PluginValidationRule{"appId": {Required: true}},
	}
	if issues := ValidateFeaturePluginTemplate(valid); len(issues) != 0 {
		t.Fatalf("template should be valid, got %+v", issues)
	}

	cases := []struct {
		name   string
		mutate func(*FeaturePluginTemplate)
		field  string
	}{
		{"版本号非法", func(tpl *FeaturePluginTemplate) { tpl.TemplateVersion = "v 1" }, "templateVersion"},
		{"表单为空", func(tpl *FeaturePluginTemplate) { tpl.FormSchema = nil }, "formSchemaJson"},
		{"口令字段未登记敏感字段", func(tpl *FeaturePluginTemplate) { tpl.SecretFields = []string{} }, "secretFieldsJson"},
		{"文件字段未登记", func(tpl *FeaturePluginTemplate) { tpl.FileFields = nil }, "fileFieldsJson"},
		{"敏感字段未在表单声明", func(tpl *FeaturePluginTemplate) {
			tpl.SecretFields = append(tpl.SecretFields, "ghost")
		}, "secretFieldsJson"},
		{"正则无法编译", func(tpl *FeaturePluginTemplate) {
			tpl.ValidationRules = map[string]PluginValidationRule{"appId": {Pattern: "([a-z"}}
		}, "validationRulesJson.appId.pattern"},
		{"下拉无候选项", func(tpl *FeaturePluginTemplate) {
			tpl.FormSchema = append(tpl.FormSchema, PluginFormField{Key: "mode", Label: "模式", Component: "select", Order: 40})
		}, "formSchemaJson[3].options"},
		{"字段 key 重复", func(tpl *FeaturePluginTemplate) {
			tpl.FormSchema = append(tpl.FormSchema, PluginFormField{Key: "appId", Label: "App ID", Component: "input", Order: 40})
		}, "formSchemaJson[3].key"},
		{"scope 非枚举", func(tpl *FeaturePluginTemplate) { tpl.FormSchema[0].Scope = "cluster" }, "formSchemaJson[0].scope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tpl := cloneTemplateForTest(valid)
			tc.mutate(&tpl)
			issues := ValidateFeaturePluginTemplate(tpl)
			if !hasIssueField(issues, tc.field) {
				t.Fatalf("want issue on %q, got %+v", tc.field, issues)
			}
		})
	}
}

// validation_rules 的 key 允许不在 form_schema 中声明：真实模板字段命名不确定（00 §4.4.1 变更）。
func TestValidateFeaturePluginTemplateAllowsUndeclaredRuleKey(t *testing.T) {
	valid := FeaturePluginTemplate{
		TemplateVersion: "v1",
		FormSchema:      []PluginFormField{{Key: "appId", Label: "App ID", Component: "input", Required: true, Order: 10}},
		ValidationRules: map[string]PluginValidationRule{"ghost": {Required: true}},
	}
	if issues := ValidateFeaturePluginTemplate(valid); len(issues) != 0 {
		t.Fatalf("expected valid, got %+v", issues)
	}
}

func cloneTemplateForTest(tpl FeaturePluginTemplate) FeaturePluginTemplate {
	out := tpl
	out.FormSchema = append([]PluginFormField(nil), tpl.FormSchema...)
	out.SecretFields = append([]string(nil), tpl.SecretFields...)
	out.FileFields = append([]PluginFileField(nil), tpl.FileFields...)
	out.ValidationRules = map[string]PluginValidationRule{}
	for k, v := range tpl.ValidationRules {
		out.ValidationRules[k] = v
	}
	return out
}
