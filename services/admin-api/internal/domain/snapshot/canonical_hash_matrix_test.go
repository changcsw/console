package snapshot

import (
	"regexp"
	"testing"
	"time"
)

// canonical 键有序 + 数组保序 + 标量格式稳定。
func TestCanonicalJSON_KeyOrderAndArrayStability(t *testing.T) {
	in := map[string]any{
		"z":     1,
		"a":     []any{3, 1, 2}, // 数组保持插入序，不排序
		"m":     map[string]any{"y": true, "x": nil},
		"num":   1.5,
		"empty": map[string]any{},
	}
	got, err := CanonicalJSON(in)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	want := `{"a":[3,1,2],"empty":{},"m":{"x":null,"y":true},"num":1.5,"z":1}`
	if string(got) != want {
		t.Fatalf("canonical 输出不符\n want=%s\n got =%s", want, string(got))
	}
}

// 数值：整数不带小数点、无科学计数。
func TestCanonicalJSON_NumberFormat(t *testing.T) {
	got, err := CanonicalJSON(map[string]any{"n": 1000000, "f": 0.25})
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	if string(got) != `{"f":0.25,"n":1000000}` {
		t.Fatalf("数值格式不稳定，got %s", string(got))
	}
}

func TestHashCanonicalJSON_HexLengthAndStable(t *testing.T) {
	data := []byte(`{"a":1}`)
	h1 := HashCanonicalJSON(data)
	h2 := HashCanonicalJSON(data)
	if h1 != h2 {
		t.Fatalf("同输入 hash 应一致")
	}
	if len(h1) != 64 {
		t.Fatalf("sha256 十六进制应为 64 字符，got %d", len(h1))
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(h1) {
		t.Fatalf("hash 应为小写十六进制，got %s", h1)
	}
	// 内容变化 → hash 变化。
	if HashCanonicalJSON([]byte(`{"a":2}`)) == h1 {
		t.Fatalf("不同内容应产出不同 hash")
	}
}

// config_version 格式：<yyyymmddHHMMSS>-<file_hash 前 8 位>。
func TestBuildConfigVersion_FormatAndBoundaries(t *testing.T) {
	ts := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	got := BuildConfigVersion(ts, "a1b2c3d4e5f6")
	if got != "20260615100000-a1b2c3d4" {
		t.Fatalf("config_version 格式错误，got %s", got)
	}
	if !regexp.MustCompile(`^\d{14}-[0-9a-fA-F]{1,8}$`).MatchString(got) {
		t.Fatalf("config_version 未匹配格式规范，got %s", got)
	}
}

// 边界：hash 短于 8 位时不截断（前缀取整个 hash）。
func TestBuildConfigVersion_ShortHashNotTruncated(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got := BuildConfigVersion(ts, "abc")
	if got != "20260102030405-abc" {
		t.Fatalf("短 hash 不应截断，got %s", got)
	}
}

// 边界：generatedAt 非 UTC 时应归一到 UTC 再格式化（可复现）。
func TestBuildConfigVersion_NormalizesToUTC(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	ts := time.Date(2026, 6, 15, 18, 0, 0, 0, loc) // = 2026-06-15T10:00:00Z
	got := BuildConfigVersion(ts, "deadbeefcafe")
	if got != "20260615100000-deadbeef" {
		t.Fatalf("config_version 应基于 UTC，got %s", got)
	}
}
