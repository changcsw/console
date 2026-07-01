package sync

import (
	"strings"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// 9 section 枚举 / ParseSections / UNKNOWN_SECTION
// ─────────────────────────────────────────────────────────────────────────────

func TestAllSectionsHasNineKnownEnums(t *testing.T) {
	all := AllSections()
	if len(all) != 9 {
		t.Fatalf("want 9 sections, got %d", len(all))
	}
	want := []Section{
		SectionGame, SectionMarkets, SectionLegal, SectionChannels, SectionPackages,
		SectionProducts, SectionCashier, SectionPayments, SectionConfig,
	}
	for i, s := range want {
		if all[i] != s {
			t.Errorf("section[%d]: want %s, got %s", i, s, all[i])
		}
		if !s.IsKnown() {
			t.Errorf("section %s should be known", s)
		}
	}
	// 返回副本，不可污染内部状态
	all[0] = "tampered"
	if AllSections()[0] != SectionGame {
		t.Fatalf("AllSections must return a defensive copy")
	}
}

func TestSectionIsKnownRejectsUnknown(t *testing.T) {
	if Section("login").IsKnown() {
		t.Fatalf("login is not a top-level sync section")
	}
	if Section("").IsKnown() {
		t.Fatalf("empty section must be unknown")
	}
}

func TestParseSections(t *testing.T) {
	t.Run("empty non-explicit defaults to all 9", func(t *testing.T) {
		got, err := ParseSections(nil, false)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(got) != 9 {
			t.Fatalf("want 9, got %d", len(got))
		}
	})
	t.Run("empty explicit is rejected", func(t *testing.T) {
		if _, err := ParseSections(nil, true); err == nil {
			t.Fatalf("expected error for empty explicit sections")
		}
	})
	t.Run("unknown section rejected", func(t *testing.T) {
		if _, err := ParseSections([]string{"channels", "iap"}, true); err == nil {
			t.Fatalf("expected UNKNOWN_SECTION style error")
		}
	})
	t.Run("dedup and trim", func(t *testing.T) {
		got, err := ParseSections([]string{" channels ", "channels", "products"}, true)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 deduped sections, got %d (%v)", len(got), got)
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// 依赖拓扑排序 + 依赖校验
// ─────────────────────────────────────────────────────────────────────────────

func TestSectionDependencies(t *testing.T) {
	cases := map[Section][]Section{
		SectionGame:     {},
		SectionMarkets:  {SectionGame},
		SectionChannels: {SectionGame, SectionMarkets},
		SectionPackages: {SectionChannels},
		SectionPayments: {SectionChannels, SectionPackages, SectionProducts, SectionCashier},
		SectionConfig:   {SectionChannels, SectionPackages, SectionProducts, SectionCashier, SectionPayments},
	}
	for sec, wantDeps := range cases {
		got := sec.Dependencies()
		if len(got) != len(wantDeps) {
			t.Errorf("%s deps: want %v, got %v", sec, wantDeps, got)
			continue
		}
		for i := range wantDeps {
			if got[i] != wantDeps[i] {
				t.Errorf("%s deps[%d]: want %s, got %s", sec, i, wantDeps[i], got[i])
			}
		}
	}
	// 返回副本
	d := SectionChannels.Dependencies()
	d[0] = "x"
	if SectionChannels.Dependencies()[0] != SectionGame {
		t.Fatalf("Dependencies must return a defensive copy")
	}
}

func TestSortSectionsByTopoRespectsWriteOrder(t *testing.T) {
	// 乱序输入应按 game→...→config 的固定写入次序回排
	in := []Section{SectionConfig, SectionPayments, SectionGame, SectionChannels}
	got := SortSectionsByTopo(in)
	want := []Section{SectionGame, SectionChannels, SectionPayments, SectionConfig}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("topo[%d]: want %s, got %s (%v)", i, want[i], got[i], got)
		}
	}
}

func TestValidateSectionDependencies(t *testing.T) {
	t.Run("channels alone missing game+markets in production", func(t *testing.T) {
		missing := ValidateSectionDependencies([]Section{SectionChannels}, map[Section][]EntityRecord{})
		gotDeps := map[Section]bool{}
		for _, m := range missing {
			if m.Section != SectionChannels {
				t.Errorf("unexpected owning section %s", m.Section)
			}
			gotDeps[m.MissingDependency] = true
		}
		if !gotDeps[SectionGame] || !gotDeps[SectionMarkets] {
			t.Fatalf("want missing game+markets, got %#v", missing)
		}
	})
	t.Run("deps satisfied by same batch selection", func(t *testing.T) {
		missing := ValidateSectionDependencies(
			[]Section{SectionGame, SectionMarkets, SectionChannels},
			map[Section][]EntityRecord{},
		)
		if len(missing) != 0 {
			t.Fatalf("same-batch deps should satisfy, got %#v", missing)
		}
	})
	t.Run("deps satisfied by existing production data", func(t *testing.T) {
		prod := map[Section][]EntityRecord{
			SectionGame:    {{EntityType: "game", EntityKey: "100001"}},
			SectionMarkets: {{EntityType: "game_market", EntityKey: "JP"}},
		}
		missing := ValidateSectionDependencies([]Section{SectionChannels}, prod)
		if len(missing) != 0 {
			t.Fatalf("existing prod deps should satisfy, got %#v", missing)
		}
	})
	t.Run("root game has no dependency", func(t *testing.T) {
		if missing := ValidateSectionDependencies([]Section{SectionGame}, map[Section][]EntityRecord{}); len(missing) != 0 {
			t.Fatalf("game is root, got %#v", missing)
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// diff：add / update / delete 字段级 + masked 判定
// ─────────────────────────────────────────────────────────────────────────────

func TestDiffEntitiesAdd(t *testing.T) {
	sandbox := []EntityRecord{{EntityType: "game_channel", EntityKey: "JP/google", Fields: map[string]any{"enabled": true}}}
	diff := DiffEntities(SectionChannels, sandbox, nil, nil)
	if diff.Summary.Add != 1 || diff.Summary.Update != 0 || diff.Summary.Delete != 0 {
		t.Fatalf("want add=1, got %+v", diff.Summary)
	}
	c := diff.Changes[0]
	if c.Op != OpAdd || c.FieldName != "*" || c.EntityKey != "JP/google" {
		t.Fatalf("unexpected add change: %+v", c)
	}
	if c.ProductionValue != nil {
		t.Fatalf("add.productionValue must be nil, got %v", c.ProductionValue)
	}
}

func TestDiffEntitiesDelete(t *testing.T) {
	production := []EntityRecord{{EntityType: "product", EntityKey: "gem_1", Fields: map[string]any{"enabled": true}}}
	diff := DiffEntities(SectionProducts, nil, production, nil)
	if diff.Summary.Delete != 1 {
		t.Fatalf("want delete=1, got %+v", diff.Summary)
	}
	c := diff.Changes[0]
	if c.Op != OpDelete || c.FieldName != "*" || c.SandboxValue != nil {
		t.Fatalf("unexpected delete change: %+v", c)
	}
}

func TestDiffEntitiesUpdateFieldLevel(t *testing.T) {
	sandbox := []EntityRecord{{EntityType: "product", EntityKey: "gem_1", Fields: map[string]any{
		"product_name": "Gems A", "base_amount_minor": int64(100), "enabled": true,
	}}}
	production := []EntityRecord{{EntityType: "product", EntityKey: "gem_1", Fields: map[string]any{
		"product_name": "Gems B", "base_amount_minor": int64(100), "enabled": true,
	}}}
	diff := DiffEntities(SectionProducts, sandbox, production, nil)
	if diff.Summary.Update != 1 {
		t.Fatalf("only product_name differs → want update=1, got %+v (%+v)", diff.Summary, diff.Changes)
	}
	c := diff.Changes[0]
	if c.Op != OpUpdate || c.FieldName != "product_name" {
		t.Fatalf("unexpected update change: %+v", c)
	}
	if c.SandboxValue != "Gems A" || c.ProductionValue != "Gems B" {
		t.Fatalf("want field values A→B, got %v→%v", c.SandboxValue, c.ProductionValue)
	}
}

func TestDiffEntitiesNoChange(t *testing.T) {
	rec := []EntityRecord{{EntityType: "game", EntityKey: "100001", Fields: map[string]any{"name": "G"}}}
	diff := DiffEntities(SectionGame, rec, rec, nil)
	if len(diff.Changes) != 0 {
		t.Fatalf("identical sets → no changes, got %+v", diff.Changes)
	}
}

func TestDiffEntitiesMaskedSecretFields(t *testing.T) {
	sandbox := []EntityRecord{{EntityType: "game_channel_login_config", EntityKey: "JP/google", Fields: map[string]any{
		"clientId":          "pub-client",
		"clientSecret":      "SANDBOX_PLAINTEXT",
		"api_key_ciphertext": "enc-sandbox",
	}}}
	production := []EntityRecord{{EntityType: "game_channel_login_config", EntityKey: "JP/google", Fields: map[string]any{
		"clientId":          "pub-client",
		"clientSecret":      "PROD_PLAINTEXT",
		"api_key_ciphertext": "enc-prod",
	}}}
	diff := DiffEntities(SectionChannels, sandbox, production, nil)
	for _, c := range diff.Changes {
		switch c.FieldName {
		case "clientSecret", "api_key_ciphertext":
			if !c.Masked {
				t.Errorf("field %s must be masked", c.FieldName)
			}
			if c.SandboxValue != "masked" || c.ProductionValue != "masked" {
				t.Errorf("masked field %s must never expose plaintext, got %v→%v", c.FieldName, c.SandboxValue, c.ProductionValue)
			}
		}
	}
	// 确认确实产生了两个 masked 更新（clientId 相同不产生 diff）
	maskedCount := 0
	for _, c := range diff.Changes {
		if c.Masked {
			maskedCount++
		}
	}
	if maskedCount != 2 {
		t.Fatalf("want 2 masked changes, got %d (%+v)", maskedCount, diff.Changes)
	}
}

func TestDiffEntitiesExplicitMaskedField(t *testing.T) {
	explicit := map[string]struct{}{"token": {}}
	sandbox := []EntityRecord{{EntityType: "x", EntityKey: "k", Fields: map[string]any{"token": "a"}}}
	production := []EntityRecord{{EntityType: "x", EntityKey: "k", Fields: map[string]any{"token": "b"}}}
	diff := DiffEntities(SectionConfig, sandbox, production, explicit)
	if len(diff.Changes) != 1 || !diff.Changes[0].Masked {
		t.Fatalf("explicit masked field 'token' must be masked, got %+v", diff.Changes)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 确定性 hash 规范化
// ─────────────────────────────────────────────────────────────────────────────

func TestHashEntitySetsDeterministic(t *testing.T) {
	build := func() map[Section][]EntityRecord {
		return map[Section][]EntityRecord{
			SectionGame: {{EntityType: "game", EntityKey: "100001", Fields: map[string]any{
				"name": "G", "alias": "g", "status": "online",
			}}},
			SectionProducts: {
				{EntityType: "product", EntityKey: "b", Fields: map[string]any{"product_name": "B"}},
				{EntityType: "product", EntityKey: "a", Fields: map[string]any{"product_name": "A"}},
			},
		}
	}
	h1, err := HashEntitySets(build())
	if err != nil {
		t.Fatalf("hash err: %v", err)
	}
	// 多次计算恒等（含 map 迭代顺序、切片内部排序）
	for i := 0; i < 5; i++ {
		h, err := HashEntitySets(build())
		if err != nil {
			t.Fatalf("hash err: %v", err)
		}
		if h != h1 {
			t.Fatalf("hash not deterministic: %s != %s", h, h1)
		}
	}
	if len(h1) != 64 {
		t.Fatalf("want 64-hex sha256, got len %d (%s)", len(h1), h1)
	}
}

func TestHashExcludesIDAndTimestamps(t *testing.T) {
	base := map[Section][]EntityRecord{
		SectionGame: {{EntityType: "game", EntityKey: "100001", Fields: map[string]any{
			"name": "G", "status": "online",
		}}},
	}
	withNoise := map[Section][]EntityRecord{
		SectionGame: {{EntityType: "game", EntityKey: "100001", Fields: map[string]any{
			"name": "G", "status": "online",
			"id": 42, "created_at": "2020-01-01", "updated_at": "2021-02-02", "updatedAt": "x",
		}}},
	}
	h1, _ := HashEntitySets(base)
	h2, _ := HashEntitySets(withNoise)
	if h1 != h2 {
		t.Fatalf("id/created_at/updated_at must be excluded from hash: %s != %s", h1, h2)
	}
}

func TestHashSensitiveToSemanticFieldAndCiphertext(t *testing.T) {
	base := map[Section][]EntityRecord{
		SectionGame: {{EntityType: "game", EntityKey: "100001", Fields: map[string]any{"name": "G"}}},
	}
	changed := map[Section][]EntityRecord{
		SectionGame: {{EntityType: "game", EntityKey: "100001", Fields: map[string]any{"name": "G2"}}},
	}
	h1, _ := HashEntitySets(base)
	h2, _ := HashEntitySets(changed)
	if h1 == h2 {
		t.Fatalf("semantic field change must alter hash")
	}
	// 密文以密文参与 hash（而非明文剔除）
	cipherA := map[Section][]EntityRecord{
		SectionChannels: {{EntityType: "c", EntityKey: "k", Fields: map[string]any{"api_ciphertext": "encA"}}},
	}
	cipherB := map[Section][]EntityRecord{
		SectionChannels: {{EntityType: "c", EntityKey: "k", Fields: map[string]any{"api_ciphertext": "encB"}}},
	}
	hc1, _ := HashEntitySets(cipherA)
	hc2, _ := HashEntitySets(cipherB)
	if hc1 == hc2 {
		t.Fatalf("ciphertext must participate in hash (differing ciphertext → differing hash)")
	}
}

func TestNormalizeForHashDropsNonSemanticColumns(t *testing.T) {
	in := map[string]any{
		"Name": "x", "id": 1, "ID": 2, "created_at": "t", "CreatedAt": "t2",
		"updated_at": "u", "UpdatedAt": "u2", "value": 3,
	}
	out := NormalizeForHash(in)
	for _, dropped := range []string{"id", "ID", "created_at", "CreatedAt", "updated_at", "UpdatedAt"} {
		if _, ok := out[dropped]; ok {
			t.Errorf("normalize must drop %q", dropped)
		}
	}
	if out["Name"] != "x" || out["value"] != 3 {
		t.Fatalf("semantic fields must survive: %#v", out)
	}
}

func TestCanonicalJSONStableKeyOrder(t *testing.T) {
	a, _ := CanonicalJSON(map[string]any{"b": 1, "a": 2, "c": map[string]any{"z": 1, "y": 2}})
	b, _ := CanonicalJSON(map[string]any{"c": map[string]any{"y": 2, "z": 1}, "a": 2, "b": 1})
	if string(a) != string(b) {
		t.Fatalf("canonical JSON must be key-order stable:\n%s\n%s", a, b)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// BaselineToken HMAC 签验（往返/过期/篡改/密钥不匹配/格式）
// ─────────────────────────────────────────────────────────────────────────────

func newTokenPayload(now time.Time) BaselineTokenPayload {
	return BaselineTokenPayload{
		GameID:           "100001",
		SyncJobID:        9012,
		SourceEnv:        DefaultSourceEnv(),
		TargetEnv:        DefaultTargetEnv(),
		SourceHash:       "src",
		TargetHashBefore: "tgt",
		PreviewedAt:      now,
		ExpiresAt:        now.Add(30 * time.Minute),
		Nonce:            "nonce-1",
	}
}

func TestBaselineTokenRoundTrip(t *testing.T) {
	secret := []byte("server-secret")
	now := time.Now().UTC()
	tok, err := BuildBaselineToken(newTokenPayload(now), secret)
	if err != nil {
		t.Fatalf("build err: %v", err)
	}
	got, err := ParseAndVerifyBaselineToken(tok, secret, now)
	if err != nil {
		t.Fatalf("verify err: %v", err)
	}
	if got.GameID != "100001" || got.SyncJobID != 9012 || got.Nonce != "nonce-1" {
		t.Fatalf("payload mismatch: %+v", got)
	}
	if got.SourceEnv != "sandbox" || got.TargetEnv != "production" {
		t.Fatalf("env fields mismatch: %+v", got)
	}
}

func TestBaselineTokenExpired(t *testing.T) {
	secret := []byte("server-secret")
	previewedAt := time.Now().UTC().Add(-2 * time.Hour)
	tok, _ := BuildBaselineToken(newTokenPayload(previewedAt), secret)
	// now 晚于 expiresAt(previewedAt+30min)
	if _, err := ParseAndVerifyBaselineToken(tok, secret, time.Now().UTC()); err == nil {
		t.Fatalf("expected expired token error")
	}
}

func TestBaselineTokenTampered(t *testing.T) {
	secret := []byte("server-secret")
	now := time.Now().UTC()
	tok, _ := BuildBaselineToken(newTokenPayload(now), secret)
	parts := strings.SplitN(tok, ".", 2)
	// 篡改 payload：替换其中一个字符
	tampered := flipFirstChar(parts[0]) + "." + parts[1]
	if _, err := ParseAndVerifyBaselineToken(tampered, secret, now); err == nil {
		t.Fatalf("expected signature failure on tampered payload")
	}
}

func TestBaselineTokenWrongSecret(t *testing.T) {
	now := time.Now().UTC()
	tok, _ := BuildBaselineToken(newTokenPayload(now), []byte("secret-A"))
	if _, err := ParseAndVerifyBaselineToken(tok, []byte("secret-B"), now); err == nil {
		t.Fatalf("expected signature failure with wrong secret")
	}
}

func TestBaselineTokenMalformedFormat(t *testing.T) {
	secret := []byte("s")
	now := time.Now().UTC()
	for _, bad := range []string{"", "nodot", "a.b.c", "!!!.###"} {
		if _, err := ParseAndVerifyBaselineToken(bad, secret, now); err == nil {
			t.Errorf("expected error for malformed token %q", bad)
		}
	}
}

func TestGenerateNonceUniqueAndLong(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 100; i++ {
		n, err := GenerateNonce()
		if err != nil {
			t.Fatalf("nonce err: %v", err)
		}
		if len(n) != 32 { // 16 bytes hex
			t.Fatalf("want 32-hex nonce, got %d (%s)", len(n), n)
		}
		if _, dup := seen[n]; dup {
			t.Fatalf("nonce collision: %s", n)
		}
		seen[n] = struct{}{}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 状态流转常量（previewed → succeeded/failed 终态）
// ─────────────────────────────────────────────────────────────────────────────

func TestJobStatusConstants(t *testing.T) {
	if JobStatusPreviewed != "previewed" || JobStatusSucceeded != "succeeded" || JobStatusFailed != "failed" {
		t.Fatalf("job status enum values drifted: %s/%s/%s", JobStatusPreviewed, JobStatusSucceeded, JobStatusFailed)
	}
	if DefaultSourceEnv() != "sandbox" || DefaultTargetEnv() != "production" {
		t.Fatalf("sync direction must be hardcoded sandbox→production, got %s→%s", DefaultSourceEnv(), DefaultTargetEnv())
	}
}

func flipFirstChar(s string) string {
	if s == "" {
		return "x"
	}
	b := []byte(s)
	if b[0] == 'A' {
		b[0] = 'B'
	} else {
		b[0] = 'A'
	}
	return string(b)
}
