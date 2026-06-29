package admin

import (
	"fmt"
	"regexp"
	"sort"
)

// permissionCodePattern 权限码格式 resource.action（compact「权限码命名与 seed 清单」）。
var permissionCodePattern = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)

// PermissionCode 不可变值对象，封装 resource.action 权限码。
type PermissionCode struct {
	value string
}

// NewPermissionCode 校验并构造 PermissionCode；格式非法返回错误。
func NewPermissionCode(code string) (PermissionCode, error) {
	if !IsValidPermissionCode(code) {
		return PermissionCode{}, fmt.Errorf("invalid permission code %q: must match %s", code, permissionCodePattern.String())
	}
	return PermissionCode{value: code}, nil
}

// String 返回原始权限码字符串。
func (p PermissionCode) String() string { return p.value }

// IsValidPermissionCode 纯校验：是否匹配 ^[a-z0-9_]+\.[a-z0-9_]+$。
func IsValidPermissionCode(code string) bool {
	return permissionCodePattern.MatchString(code)
}

// MergePermissionCodes 计算多组权限码的并集（去重 + 稳定排序）。
// 对应「有效权限 = 用户所有角色授予权限码的并集（去重）」（compact 不变量 4）。
func MergePermissionCodes(groups ...[]string) []string {
	set := make(map[string]struct{})
	for _, group := range groups {
		for _, code := range group {
			if code == "" {
				continue
			}
			set[code] = struct{}{}
		}
	}
	merged := make([]string, 0, len(set))
	for code := range set {
		merged = append(merged, code)
	}
	sort.Strings(merged)
	return merged
}

// PermissionSet 把权限码切片转为集合，便于 O(1) 判定。
func PermissionSet(codes []string) map[string]struct{} {
	set := make(map[string]struct{}, len(codes))
	for _, c := range codes {
		set[c] = struct{}{}
	}
	return set
}
