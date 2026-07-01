package sync

import (
	"strings"
	"testing"
)

// 🟪 测试专家(第2轮)：守护 SYNC-INT-03 修复——整行 add/delete diff 的字段级脱敏。
// 补齐既有仅覆盖 update 分支的 masked 断言：add(仅源)/delete(仅目标) 整行也必须对密文字段脱敏，
// 明文/密文绝不出现在 DiffChange 中（S8 红线）。hash 仍用原值（另由 HashEntitySets 测试守护）。

func TestDiffEntitiesAddRowMasksSecretFields(t *testing.T) {
	sandbox := []EntityRecord{{
		EntityType: "game_channel_login_config",
		EntityKey:  "JP/google",
		Fields: map[string]any{
			"client_id":          "pub-client",
			"clientSecret":       "SANDBOX_PLAINTEXT_SECRET",
			"api_key_ciphertext": "enc-sandbox",
			"config_json":        map[string]any{"nested": "SANDBOX_PLAINTEXT_SECRET"},
		},
	}}
	// production 无此 entityKey → add（整行）
	diff := DiffEntities(SectionChannels, sandbox, nil, map[string]struct{}{"config_json": {}})
	if len(diff.Changes) != 1 || diff.Changes[0].Op != OpAdd {
		t.Fatalf("expected single add change, got %+v", diff.Changes)
	}
	c := diff.Changes[0]
	if !c.Masked {
		t.Fatalf("add row with secret fields must be flagged masked=true")
	}
	row, ok := c.SandboxValue.(map[string]any)
	if !ok {
		t.Fatalf("add SandboxValue must be a field map, got %T", c.SandboxValue)
	}
	for _, secretField := range []string{"clientSecret", "api_key_ciphertext", "config_json"} {
		if row[secretField] != "masked" {
			t.Errorf("secret field %q must be masked in add row, got %v", secretField, row[secretField])
		}
	}
	if row["client_id"] != "pub-client" {
		t.Errorf("non-secret field client_id must be preserved, got %v", row["client_id"])
	}
	assertNoPlaintext(t, c)
}

func TestDiffEntitiesDeleteRowMasksSecretFields(t *testing.T) {
	production := []EntityRecord{{
		EntityType: "game_channel_login_config",
		EntityKey:  "JP/google",
		Fields: map[string]any{
			"client_id":    "pub-client",
			"clientSecret": "PROD_PLAINTEXT_SECRET",
		},
	}}
	// sandbox 无此 entityKey → delete（整行）
	diff := DiffEntities(SectionChannels, nil, production, nil)
	if len(diff.Changes) != 1 || diff.Changes[0].Op != OpDelete {
		t.Fatalf("expected single delete change, got %+v", diff.Changes)
	}
	c := diff.Changes[0]
	if !c.Masked {
		t.Fatalf("delete row with secret fields must be flagged masked=true")
	}
	row, ok := c.ProductionValue.(map[string]any)
	if !ok {
		t.Fatalf("delete ProductionValue must be a field map, got %T", c.ProductionValue)
	}
	if row["clientSecret"] != "masked" {
		t.Errorf("secret field clientSecret must be masked in delete row, got %v", row["clientSecret"])
	}
	assertNoPlaintext(t, c)
}

// game_secret 整行 add/delete 亦脱敏（SYNC-INT-06 关联：game_secret 参与 hash 但不泄漏）。
func TestDiffEntitiesGameRowMasksGameSecret(t *testing.T) {
	sandbox := []EntityRecord{{
		EntityType: "game",
		EntityKey:  "100001",
		Fields: map[string]any{
			"game_id":     "100001",
			"game_secret": "TOPSECRET_PLAINTEXT",
			"name":        "Demo",
		},
	}}
	diff := DiffEntities(SectionGame, sandbox, nil, map[string]struct{}{"game_secret": {}})
	c := diff.Changes[0]
	row := c.SandboxValue.(map[string]any)
	if row["game_secret"] != "masked" {
		t.Fatalf("game_secret must be masked in add row, got %v", row["game_secret"])
	}
	if row["game_id"] != "100001" || row["name"] != "Demo" {
		t.Fatalf("non-secret fields must be preserved: %+v", row)
	}
	assertNoPlaintext(t, c)
}

func assertNoPlaintext(t *testing.T, c DiffChange) {
	t.Helper()
	blob := canonicalString(t, c.SandboxValue) + "|" + canonicalString(t, c.ProductionValue)
	if strings.Contains(blob, "PLAINTEXT") || strings.Contains(blob, "enc-") {
		t.Fatalf("diff change leaked secret material: %s", blob)
	}
}

func canonicalString(t *testing.T, v any) string {
	t.Helper()
	if v == nil {
		return ""
	}
	b, err := CanonicalJSON(v)
	if err != nil {
		t.Fatalf("canonical json: %v", err)
	}
	return string(b)
}
