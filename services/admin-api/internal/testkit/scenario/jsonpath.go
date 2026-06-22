package scenario

import (
	"fmt"
	"strconv"
	"strings"
)

// lookup 按点路径在已解码 JSON（map/slice）中取值；支持数字下标，如 items.0.id。
func lookup(root any, path string) (any, bool) {
	cur := root
	for _, seg := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// equalScalar 以字符串化方式比较标量；复合类型（map/slice）一律视为不相等，
// 避免 fmt 字符串化（受 map key 顺序影响）造成的脆弱/误判。
func equalScalar(got, want any) bool {
	if !isScalar(got) || !isScalar(want) {
		return false
	}
	return fmt.Sprintf("%v", got) == fmt.Sprintf("%v", want)
}

func isScalar(v any) bool {
	switch v.(type) {
	case nil, bool, string, float64, int, int64:
		return true
	default:
		return false
	}
}
