package game

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// L1 单元（无 IO）：覆盖 game 领域纯函数的边界 / 冲突 / 归一化 / 状态流转。
// 维度对照见 docs/architecture/v2/03-testing.md；规则来源 modules/11-game/spec.compact.md §5。

// ───────────────────────── IsValidAlias ─────────────────────────

func TestIsValidAlias(t *testing.T) {
	cases := []struct {
		name  string
		alias string
		want  bool
	}{
		{"simple", "demo", true},
		{"with_underscore_dash_digit", "demo_game-01", true},
		{"single_char", "a", true},
		{"max_64", strings.Repeat("a", 64), true},
		{"empty_rejected", "", false},
		{"over_64_rejected", strings.Repeat("a", 65), false},
		{"space_rejected", "demo game", false},
		{"dot_rejected", "demo.game", false},
		{"unicode_rejected", "游戏", false},
		{"slash_rejected", "demo/game", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsValidAlias(c.alias); got != c.want {
				t.Fatalf("IsValidAlias(%q)=%v want %v", c.alias, got, c.want)
			}
		})
	}
}

// ───────────────────────── IsValidGameStatus ─────────────────────────

func TestIsValidGameStatus(t *testing.T) {
	valid := []common.GameStatus{common.GameStatusDraft, common.GameStatusActive, common.GameStatusDisabled}
	for _, s := range valid {
		if !IsValidGameStatus(s) {
			t.Fatalf("expected %q valid", s)
		}
	}
	invalid := []common.GameStatus{"", "DRAFT", "enabled", "archived", "Active"}
	for _, s := range invalid {
		if IsValidGameStatus(s) {
			t.Fatalf("expected %q invalid", s)
		}
	}
}

// ───────────────────────── IsValidMarket ─────────────────────────

func TestIsValidMarket(t *testing.T) {
	for _, m := range []string{"GLOBAL", "JP", "KR", "SEA", "HMT", "CN"} {
		if !IsValidMarket(m) {
			t.Fatalf("expected %q valid market", m)
		}
	}
	for _, m := range []string{"", "global", "US", "EU", "jp", "CN "} {
		if IsValidMarket(m) {
			t.Fatalf("expected %q invalid market", m)
		}
	}
}

// ───────────────────────── IsValidLocale（归一化格式校验）─────────────────────────

func TestIsValidLocale(t *testing.T) {
	cases := []struct {
		locale string
		want   bool
	}{
		{"en", true},
		{"en-US", true},
		{"zh-Hant", true}, // xx-(2..8 字母数字)
		{"ja-JP", true},
		{"", false},
		{"english", false}, // 主标签必须 2 字母
		{"e", false},
		{"en_US", false}, // 下划线非法
		{"en-", false},
		{"123", false},
	}
	for _, c := range cases {
		if got := IsValidLocale(c.locale); got != c.want {
			t.Fatalf("IsValidLocale(%q)=%v want %v", c.locale, got, c.want)
		}
	}
}

// ───────────────────────── IsValidOptionalURL ─────────────────────────

func TestIsValidOptionalURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"", true}, // 空 → 合法（可选）
		{"http://a.com", true},
		{"https://a.com/x?y=1", true},
		{"ftp://a.com", false},
		{"a.com", false},
		{"javascript:alert(1)", false},
	}
	for _, c := range cases {
		if got := IsValidOptionalURL(c.url); got != c.want {
			t.Fatalf("IsValidOptionalURL(%q)=%v want %v", c.url, got, c.want)
		}
	}
}

// ───────────────────────── GenerateGameID（100000 起自增）─────────────────────────

func TestGenerateGameID(t *testing.T) {
	cases := []struct {
		name    string
		lastSeq int64
		want    string
	}{
		{"empty_table_starts_at_100000", 0, "100000"},
		{"negative_seq_clamped_to_start", -5, "100000"},
		{"below_start_clamped", 50, "100000"},
		{"increments_from_current_max", 100000, "100001"},
		{"increments_large", 123456, "123457"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := GenerateGameID(common.EnvSandbox, c.lastSeq); got != c.want {
				t.Fatalf("GenerateGameID(_, %d)=%q want %q", c.lastSeq, got, c.want)
			}
		})
	}
}

func TestGenerateGameIDEnvIndependent(t *testing.T) {
	// env 入参仅对齐签名，不参与计数语义：同 lastSeq 各 env 结果一致。
	for _, env := range []common.Environment{common.EnvDevelop, common.EnvSandbox, common.EnvProduction} {
		if got := GenerateGameID(env, 100005); got != "100006" {
			t.Fatalf("env %s: got %q want 100006", env, got)
		}
	}
}

// ───────────────────────── GenerateGameSecret（≥32 字节熵 hex）─────────────────────────

func TestGenerateGameSecret(t *testing.T) {
	// 固定随机源 → 确定输出，便于断言 hex 编码与长度。
	src := bytes.Repeat([]byte{0xAB}, 64)
	secret, err := GenerateGameSecret(bytes.NewReader(src))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(secret) != 64 { // 32 字节熵 → 64 hex 字符
		t.Fatalf("secret len=%d want 64", len(secret))
	}
	if len(secret) > 128 {
		t.Fatalf("secret exceeds VARCHAR(128): %d", len(secret))
	}
	if _, err := hex.DecodeString(secret); err != nil {
		t.Fatalf("secret must be hex: %v", err)
	}
	if secret != strings.Repeat("ab", 32) {
		t.Fatalf("unexpected hex encoding: %q", secret)
	}
}

func TestGenerateGameSecretUnique(t *testing.T) {
	// 不同随机源 → 不同密钥（高熵）。
	a, err := GenerateGameSecret(bytes.NewReader(bytes.Repeat([]byte{0x01}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateGameSecret(bytes.NewReader(bytes.Repeat([]byte{0x02}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("distinct entropy must yield distinct secrets")
	}
}

func TestGenerateGameSecretShortReaderErrors(t *testing.T) {
	// 熵不足（reader 提前 EOF）→ 报错，不返回弱密钥。
	_, err := GenerateGameSecret(bytes.NewReader([]byte{0x01, 0x02}))
	if err == nil {
		t.Fatal("expected error on insufficient entropy")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF-class error, got %v", err)
	}
}

// ───────────────────────── ApplyDefaultMarket（缺省补默认市场 / 去重 / 归一化）─────────────────────────

func TestApplyDefaultMarketEmptyDefaultsToGlobal(t *testing.T) {
	// 既未传 markets 也未传 defaultMarketCode → GLOBAL 单默认市场。
	markets, dmc := ApplyDefaultMarket(nil, "")
	if dmc != "GLOBAL" {
		t.Fatalf("dmc=%q want GLOBAL", dmc)
	}
	if len(markets) != 1 || markets[0].MarketCode != "GLOBAL" {
		t.Fatalf("expected [GLOBAL], got %+v", markets)
	}
	m := markets[0]
	if !m.IsDefault || !m.Enabled || m.DefaultLocale != "en-US" {
		t.Fatalf("default market must be isDefault/enabled/en-US, got %+v", m)
	}
}

func TestApplyDefaultMarketInjectsMissingDefault(t *testing.T) {
	// 传了 markets 但 defaultMarketCode 不在其中 → 补入并标默认。
	markets, dmc := ApplyDefaultMarket([]string{"JP", "KR"}, "GLOBAL")
	if dmc != "GLOBAL" {
		t.Fatalf("dmc=%q want GLOBAL", dmc)
	}
	if len(markets) != 3 {
		t.Fatalf("expected 3 markets (JP,KR,+GLOBAL), got %+v", markets)
	}
	defaults := 0
	var globalSeen bool
	for _, m := range markets {
		if m.MarketCode == "GLOBAL" {
			globalSeen = true
		}
		if m.IsDefault {
			defaults++
			if m.MarketCode != "GLOBAL" {
				t.Fatalf("default must be GLOBAL, got %q", m.MarketCode)
			}
		}
	}
	if !globalSeen {
		t.Fatal("GLOBAL must be injected")
	}
	if defaults != 1 {
		t.Fatalf("exactly one default expected, got %d", defaults)
	}
}

func TestApplyDefaultMarketDedupesAndKeepsOrder(t *testing.T) {
	// 去重（含空串过滤），defaultMarketCode 已在列表中则不重复补入。
	markets, dmc := ApplyDefaultMarket([]string{"JP", "JP", "", "GLOBAL", "KR"}, "JP")
	if dmc != "JP" {
		t.Fatalf("dmc=%q want JP", dmc)
	}
	codes := make([]string, 0, len(markets))
	seen := map[string]bool{}
	for _, m := range markets {
		if seen[m.MarketCode] {
			t.Fatalf("duplicate market in output: %q (%+v)", m.MarketCode, markets)
		}
		seen[m.MarketCode] = true
		codes = append(codes, m.MarketCode)
	}
	want := []string{"JP", "GLOBAL", "KR"}
	if strings.Join(codes, ",") != strings.Join(want, ",") {
		t.Fatalf("order/dedupe mismatch: got %v want %v", codes, want)
	}
	for _, m := range markets {
		if m.IsDefault != (m.MarketCode == "JP") {
			t.Fatalf("only JP should be default, got %+v", m)
		}
	}
}

func TestApplyDefaultMarketSingleNonGlobalDefault(t *testing.T) {
	// markets 仅含指定默认市场（不含 GLOBAL）→ 不强制补 GLOBAL。
	markets, dmc := ApplyDefaultMarket([]string{"CN"}, "CN")
	if dmc != "CN" || len(markets) != 1 || markets[0].MarketCode != "CN" || !markets[0].IsDefault {
		t.Fatalf("expected single CN default, got %+v dmc=%q", markets, dmc)
	}
}

// ───────────────────────── ValidateMarkets（恰好一默认 + 枚举 + 不重复）─────────────────────────

func TestValidateMarketsHappyPath(t *testing.T) {
	markets := []GameMarket{
		{MarketCode: "GLOBAL", IsDefault: true, Enabled: true, DefaultLocale: "en-US"},
		{MarketCode: "JP", IsDefault: false, Enabled: true, DefaultLocale: "ja-JP"},
	}
	if err := ValidateMarkets(markets, "GLOBAL"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// defaultMarketCode 留空时不强制比对，只校验自洽。
	if err := ValidateMarkets(markets, ""); err != nil {
		t.Fatalf("empty dmc should still validate self-consistency: %v", err)
	}
}

func TestValidateMarketsErrors(t *testing.T) {
	cases := []struct {
		name    string
		markets []GameMarket
		dmc     string
		wantErr error
	}{
		{
			name:    "empty",
			markets: nil,
			dmc:     "GLOBAL",
			wantErr: ErrEmptyMarkets,
		},
		{
			name: "invalid_market",
			markets: []GameMarket{
				{MarketCode: "US", IsDefault: true},
			},
			dmc:     "US",
			wantErr: ErrInvalidMarket,
		},
		{
			name: "duplicate_market",
			markets: []GameMarket{
				{MarketCode: "GLOBAL", IsDefault: true},
				{MarketCode: "GLOBAL", IsDefault: false},
			},
			dmc:     "GLOBAL",
			wantErr: ErrDuplicateMarket,
		},
		{
			name: "no_default",
			markets: []GameMarket{
				{MarketCode: "GLOBAL", IsDefault: false},
				{MarketCode: "JP", IsDefault: false},
			},
			dmc:     "GLOBAL",
			wantErr: ErrNotExactlyOneDefault,
		},
		{
			name: "multiple_defaults",
			markets: []GameMarket{
				{MarketCode: "GLOBAL", IsDefault: true},
				{MarketCode: "JP", IsDefault: true},
			},
			dmc:     "GLOBAL",
			wantErr: ErrNotExactlyOneDefault,
		},
		{
			name: "default_mismatch",
			markets: []GameMarket{
				{MarketCode: "GLOBAL", IsDefault: true},
				{MarketCode: "JP", IsDefault: false},
			},
			dmc:     "JP",
			wantErr: ErrDefaultMismatch,
		},
		{
			// 被标默认的市场 enabled=false → 拒绝（默认市场须 ∈ 已启用 markets）。
			name: "default_not_enabled",
			markets: []GameMarket{
				{MarketCode: "GLOBAL", IsDefault: true, Enabled: false},
				{MarketCode: "JP", IsDefault: false, Enabled: true},
			},
			dmc:     "GLOBAL",
			wantErr: ErrDefaultMarketNotEnabled,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateMarkets(c.markets, c.dmc)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("ValidateMarkets err=%v want %v", err, c.wantErr)
			}
		})
	}
}

// ───────────────────────── ValidateLegalScope（取值约束 + 归一化）─────────────────────────

func TestValidateLegalScopeDefault(t *testing.T) {
	// default ⇒ scopeValue 归一化为 '*'（空或 '*' 均合法）。
	for _, in := range []string{"", "*"} {
		got, err := ValidateLegalScope("default", in)
		if err != nil {
			t.Fatalf("default scope %q unexpected err: %v", in, err)
		}
		if got != "*" {
			t.Fatalf("default scopeValue must normalize to '*', got %q", got)
		}
	}
	// default 不接受具体值。
	if _, err := ValidateLegalScope("default", "GLOBAL"); !errors.Is(err, ErrInvalidScopeValue) {
		t.Fatalf("default with concrete value should err ErrInvalidScopeValue, got %v", err)
	}
}

func TestValidateLegalScopeMarket(t *testing.T) {
	got, err := ValidateLegalScope("market", "JP")
	if err != nil || got != "JP" {
		t.Fatalf("market JP => (%q,%v) want (JP,nil)", got, err)
	}
	if _, err := ValidateLegalScope("market", "US"); !errors.Is(err, ErrInvalidScopeValue) {
		t.Fatalf("market US should err, got %v", err)
	}
	if _, err := ValidateLegalScope("market", ""); !errors.Is(err, ErrInvalidScopeValue) {
		t.Fatalf("market empty should err, got %v", err)
	}
}

func TestValidateLegalScopeLocale(t *testing.T) {
	got, err := ValidateLegalScope("locale", "en-US")
	if err != nil || got != "en-US" {
		t.Fatalf("locale en-US => (%q,%v) want (en-US,nil)", got, err)
	}
	if _, err := ValidateLegalScope("locale", "english"); !errors.Is(err, ErrInvalidScopeValue) {
		t.Fatalf("invalid locale should err, got %v", err)
	}
}

func TestValidateLegalScopeUnknownType(t *testing.T) {
	if _, err := ValidateLegalScope("region", "x"); !errors.Is(err, ErrInvalidScopeType) {
		t.Fatalf("unknown scope type should err ErrInvalidScopeType, got %v", err)
	}
	if _, err := ValidateLegalScope("", "x"); !errors.Is(err, ErrInvalidScopeType) {
		t.Fatalf("empty scope type should err ErrInvalidScopeType, got %v", err)
	}
}
