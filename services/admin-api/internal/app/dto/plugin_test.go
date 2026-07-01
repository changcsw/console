package dto

import (
	"encoding/json"
	"testing"
)

// I-1 回归：渠道包覆盖项配置体的 JSON key 必须为 camelCase 的 configJson，
// 与实例项 ChannelPluginItemView 及前端 ChannelPackagePluginItem 一致，
// 避免自定义覆盖配置 round-trip 读取/回填失效。
func TestPackagePluginItemView_ConfigJSONFieldName(t *testing.T) {
	view := PackagePluginItemView{
		ConfigJSON: map[string]any{"endpoint": "https://example.com/aa"},
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := decoded["configJson"]; !ok {
		t.Fatalf("响应必须含 configJson 字段, got keys=%v", keysOf(decoded))
	}
	if _, ok := decoded["config"]; ok {
		t.Fatalf("响应不得再使用旧 config 字段（契约漂移）, got keys=%v", keysOf(decoded))
	}
}

// 实例项与渠道包项的配置体 key 须一致，确保前端两侧按同一字段读取。
func TestChannelAndPackageViews_ConfigKeyConsistent(t *testing.T) {
	instanceKey := configJSONKeyOf(t, ChannelPluginItemView{ConfigJSON: map[string]any{"a": 1}})
	packageKey := configJSONKeyOf(t, PackagePluginItemView{ConfigJSON: map[string]any{"a": 1}})
	if instanceKey != "configJson" || packageKey != "configJson" {
		t.Fatalf("实例/渠道包配置 key 须均为 configJson, got instance=%q package=%q", instanceKey, packageKey)
	}
}

func configJSONKeyOf(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	for k := range decoded {
		if k == "configJson" {
			return k
		}
	}
	return ""
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
