package featureplugin

import "testing"

// FeaturePluginReferences.BlockingTotal / Total 是「阻断删除」与「展示/诊断」两个口径的纯函数，
// 覆盖 a1a6003 拆分后的关键性质：Templates 只计入 Total，绝不计入 BlockingTotal
// （否则插件一旦建过模板就永远删不掉，回归到修复前的 bug）。
func TestFeaturePluginReferences_BlockingTotal(t *testing.T) {
	cases := []struct {
		name string
		refs FeaturePluginReferences
		want int
	}{
		{"all_zero", FeaturePluginReferences{}, 0},
		{"templates_only_not_blocking", FeaturePluginReferences{Templates: 5}, 0},
		{"channel_bindings_only", FeaturePluginReferences{ChannelBindings: 2}, 2},
		{"game_configs_only", FeaturePluginReferences{GameConfigs: 3}, 3},
		{"package_override_only", FeaturePluginReferences{PackageOverride: 4}, 4},
		{"templates_plus_channel_bindings", FeaturePluginReferences{Templates: 9, ChannelBindings: 1}, 1},
		{
			"all_three_blocking_dims_plus_templates",
			FeaturePluginReferences{Templates: 9, ChannelBindings: 1, GameConfigs: 2, PackageOverride: 3},
			6,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.refs.BlockingTotal(); got != c.want {
				t.Fatalf("BlockingTotal() = %d, want %d (refs=%+v)", got, c.want, c.refs)
			}
		})
	}
}

func TestFeaturePluginReferences_Total(t *testing.T) {
	cases := []struct {
		name string
		refs FeaturePluginReferences
		want int
	}{
		{"all_zero", FeaturePluginReferences{}, 0},
		{"templates_only", FeaturePluginReferences{Templates: 5}, 5},
		{
			"all_four_dims",
			FeaturePluginReferences{Templates: 9, ChannelBindings: 1, GameConfigs: 2, PackageOverride: 3},
			15,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.refs.Total(); got != c.want {
				t.Fatalf("Total() = %d, want %d (refs=%+v)", got, c.want, c.refs)
			}
		})
	}
}
