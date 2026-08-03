package featureplugin

import "testing"

// referenceSummary 只拼「阻断删除」的三项外部引用（渠道绑定/渠道实例配置/渠道包覆盖），
// 参数模板随插件级联删除、不属于需要管理员先清理的项，故 a1a6003 之后文案里绝不能再出现
// 「参数模板」字样（回退到修复前语义的防护性断言）。三项全为 0 时给兜底文案，避免拼出空串
// 或裸括号（该分支在当前调用点——BlockingTotal()>0 才调用——理论上走不到，但保留作为
// 防御性收尾，防止未来调用点变化后返回空串）。
func TestReferenceSummary(t *testing.T) {
	cases := []struct {
		name string
		refs FeaturePluginReferences
		want string
	}{
		{"all_zero_fallback", FeaturePluginReferences{}, "渠道侧仍有引用"},
		{"templates_only_still_fallback", FeaturePluginReferences{Templates: 3}, "渠道侧仍有引用"},
		{"channel_bindings_only", FeaturePluginReferences{ChannelBindings: 2}, "渠道绑定 2 条"},
		{"game_configs_only", FeaturePluginReferences{GameConfigs: 3}, "渠道实例配置 3 条"},
		{"package_override_only", FeaturePluginReferences{PackageOverride: 4}, "渠道包覆盖 4 条"},
		{
			"all_three_blocking_dims_joined_in_order",
			FeaturePluginReferences{ChannelBindings: 1, GameConfigs: 2, PackageOverride: 3},
			"渠道绑定 1 条、渠道实例配置 2 条、渠道包覆盖 3 条",
		},
		{
			"templates_present_but_excluded_from_text",
			FeaturePluginReferences{Templates: 99, ChannelBindings: 1},
			"渠道绑定 1 条",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := referenceSummary(c.refs)
			if got != c.want {
				t.Fatalf("referenceSummary(%+v) = %q, want %q", c.refs, got, c.want)
			}
			if containsSubstring(got, "参数模板") {
				t.Fatalf("referenceSummary must never mention 参数模板 (regression guard), got %q", got)
			}
		})
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
