package admin

import (
	"reflect"
	"testing"
)

func TestIsValidPermissionCode(t *testing.T) {
	valid := []string{"game.read", "admin_user.write", "channel_login.read", "fx.approve", "a1.b2"}
	for _, c := range valid {
		if !IsValidPermissionCode(c) {
			t.Errorf("expected %q valid", c)
		}
	}
	invalid := []string{"", "game", "game.", ".read", "Game.read", "game.read.write", "game-read", "game read", "游戏.读"}
	for _, c := range invalid {
		if IsValidPermissionCode(c) {
			t.Errorf("expected %q invalid", c)
		}
	}
}

func TestNewPermissionCode(t *testing.T) {
	pc, err := NewPermissionCode("sync.execute")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pc.String() != "sync.execute" {
		t.Fatalf("got %q", pc.String())
	}
	if _, err := NewPermissionCode("BAD"); err == nil {
		t.Fatal("expected error for invalid code")
	}
}

func TestMergePermissionCodes(t *testing.T) {
	got := MergePermissionCodes(
		[]string{"game.read", "game.write", ""},
		[]string{"game.read", "sync.execute"},
		nil,
	)
	want := []string{"game.read", "game.write", "sync.execute"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestMergePermissionCodesInvariants(t *testing.T) {
	// 不变量4：并集去重 + 稳定（字典序）排序，跨多角色幂等。
	got := MergePermissionCodes(
		[]string{"sync.execute", "game.read"},
		[]string{"game.read", "game.write"},
		[]string{"audit.read"},
	)
	want := []string{"audit.read", "game.read", "game.write", "sync.execute"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want sorted dedup %v, got %v", want, got)
	}
	// 顺序无关：同一组角色不同排列结果一致
	reordered := MergePermissionCodes(
		[]string{"audit.read"},
		[]string{"game.write", "game.read"},
		[]string{"game.read", "sync.execute"},
	)
	if !reflect.DeepEqual(got, reordered) {
		t.Fatalf("merge must be order-independent: %v vs %v", got, reordered)
	}
}

func TestMergePermissionCodesEmpty(t *testing.T) {
	// 无角色 / 全空 → 空切片（非 nil 语义不强制，但长度为 0）
	if got := MergePermissionCodes(); len(got) != 0 {
		t.Fatalf("no groups want empty, got %v", got)
	}
	if got := MergePermissionCodes(nil, []string{}, []string{""}); len(got) != 0 {
		t.Fatalf("empty/blank-only want empty, got %v", got)
	}
}

func TestNewPermissionCodeBoundaries(t *testing.T) {
	// 合法：下划线多词 resource、数字
	for _, c := range []string{"admin_user.write", "fx.approve", "a1.b2", "channel_login.read"} {
		if _, err := NewPermissionCode(c); err != nil {
			t.Errorf("expected %q valid: %v", c, err)
		}
	}
	// 非法：大写、缺 action、多段、连字符、空格、中文
	for _, c := range []string{"", "game", "game.", ".read", "Game.read", "game.read.write", "game-read", "game read", "游戏.读"} {
		if _, err := NewPermissionCode(c); err == nil {
			t.Errorf("expected %q invalid", c)
		}
	}
}

func TestPermissionSet(t *testing.T) {
	set := PermissionSet([]string{"a.b", "c.d"})
	if _, ok := set["a.b"]; !ok {
		t.Fatal("missing a.b")
	}
	if _, ok := set["x.y"]; ok {
		t.Fatal("unexpected x.y")
	}
}
