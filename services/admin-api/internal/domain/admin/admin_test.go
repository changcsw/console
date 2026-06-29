package admin

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func TestApplyStatus(t *testing.T) {
	u := AdminUser{Status: common.AdminUserStatusActive}
	if err := u.ApplyStatus(common.AdminUserStatusDisabled); err != nil {
		t.Fatalf("active->disabled should pass: %v", err)
	}
	if u.IsActive() {
		t.Fatal("should be disabled")
	}
	if err := u.ApplyStatus(common.AdminUserStatusActive); err != nil {
		t.Fatalf("disabled->active should pass: %v", err)
	}
	if err := u.ApplyStatus(common.AdminUserStatus("bogus")); err == nil {
		t.Fatal("invalid status must error")
	}
}

func TestCanTransitionStatus(t *testing.T) {
	a := common.AdminUserStatusActive
	d := common.AdminUserStatusDisabled
	bogus := common.AdminUserStatus("bogus")

	// active<->disabled 双向允许，同态幂等允许
	if !CanTransitionStatus(a, d) || !CanTransitionStatus(d, a) {
		t.Fatal("active<->disabled must be allowed")
	}
	if !CanTransitionStatus(a, a) || !CanTransitionStatus(d, d) {
		t.Fatal("idempotent same-status transition must be allowed")
	}
	// 任一端非法值都拒绝
	if CanTransitionStatus(bogus, a) || CanTransitionStatus(a, bogus) || CanTransitionStatus(bogus, bogus) {
		t.Fatal("invalid status must reject transition")
	}
}

func TestApplyStatusKeepsStatusOnError(t *testing.T) {
	u := AdminUser{Status: common.AdminUserStatusActive}
	if err := u.ApplyStatus(common.AdminUserStatus("nope")); err == nil {
		t.Fatal("invalid target must error")
	}
	if u.Status != common.AdminUserStatusActive {
		t.Fatalf("status must be unchanged on error, got %q", u.Status)
	}
}

func TestValidEnums(t *testing.T) {
	if !IsValidStatus(common.AdminUserStatusActive) || !IsValidStatus(common.AdminUserStatusDisabled) {
		t.Fatal("valid statuses rejected")
	}
	if IsValidStatus(common.AdminUserStatus("x")) {
		t.Fatal("invalid status accepted")
	}
	if !IsValidIdentityType(common.IdentityTypePassword) || !IsValidIdentityType(common.IdentityTypeFeishu) {
		t.Fatal("valid identity types rejected")
	}
	if IsValidIdentityType(common.IdentityType("sms")) {
		t.Fatal("invalid identity type accepted")
	}
}

func TestMaskIdentityKey(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"abcd":              "****",     // <=4 全遮
		"alice":             "****lice", // 无下划线且前缀会吃掉尾部，前缀清零
		"ab_cdef":           "ab_****cdef",
		"on_1234567890abcd": "on_****abcd",
	}
	for in, want := range cases {
		if got := MaskIdentityKey(in); got != want {
			t.Errorf("MaskIdentityKey(%q): want %q, got %q", in, want, got)
		}
	}
	// 脱敏结果绝不等于原文（>4 时）
	if MaskIdentityKey("on_1234567890abcd") == "on_1234567890abcd" {
		t.Fatal("mask must not equal original")
	}
}

func TestMaskIdentityKeyBoundaries(t *testing.T) {
	// 恰好 4 位 → 全遮
	if got := MaskIdentityKey("abcd"); got != "****" {
		t.Fatalf("len==4 want ****, got %q", got)
	}
	// 5 位（>4）：无下划线，前缀 2 会与尾 4 重叠 → 前缀清零
	if got := MaskIdentityKey("abcde"); got != "****bcde" {
		t.Fatalf("len==5 want ****bcde, got %q", got)
	}
	// 下划线在尾部窗口内 → 前缀被清零，保留尾 4
	if got := MaskIdentityKey("ab_xyz"); got != "****_xyz" {
		t.Fatalf("underscore in tail window: want ****_xyz, got %q", got)
	}
	// 永不回明文：长 union_id
	masked := MaskIdentityKey("ou_9f8e7d6c5b4a3210")
	if masked == "ou_9f8e7d6c5b4a3210" {
		t.Fatal("must not return plaintext identity key")
	}
	// 末 4 位必须保留
	if got := MaskIdentityKey("on_1234567890abcd"); got[len(got)-4:] != "abcd" {
		t.Fatalf("tail 4 must be preserved, got %q", got)
	}
}
