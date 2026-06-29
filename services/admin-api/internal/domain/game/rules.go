package game

import (
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"strconv"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// gameIDSeqStart 每环境 schema 内 game_id 从此值起自增（compact §5.1）。
const gameIDSeqStart int64 = 100000

// secretEntropyBytes game_secret 随机熵字节数（hex 编码后 64 字符，≤128，compact §5.2）。
const secretEntropyBytes = 32

// 纯规则错误（app 层据此包装为统一错误码/消息）。
var (
	ErrInvalidMarket           = errors.New("invalid market code")
	ErrDuplicateMarket         = errors.New("duplicate market code")
	ErrNotExactlyOneDefault    = errors.New("exactly one default market is required")
	ErrDefaultMismatch         = errors.New("default market mismatch")
	ErrDefaultMarketNotEnabled = errors.New("default market must be enabled")
	ErrEmptyMarkets            = errors.New("markets must not be empty")
	ErrInvalidScopeType     = errors.New("invalid legal scope type")
	ErrInvalidScopeValue    = errors.New("invalid legal scope value")
)

var (
	aliasPattern  = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	localePattern = regexp.MustCompile(`^[a-zA-Z]{2}(-[a-zA-Z0-9]{2,8})?$`)
	urlPattern    = regexp.MustCompile(`^https?://`)
)

// IsValidAlias 校验 alias：1–64、仅字母数字下划线连字符（compact §5.4）。
func IsValidAlias(alias string) bool {
	return len(alias) >= 1 && len(alias) <= 64 && aliasPattern.MatchString(alias)
}

// IsValidGameStatus 校验游戏状态枚举（00 §3.1）。
func IsValidGameStatus(s common.GameStatus) bool {
	return s == common.GameStatusDraft || s == common.GameStatusActive || s == common.GameStatusDisabled
}

// IsValidMarket 校验 market 枚举（00 §3.1）。
func IsValidMarket(code string) bool {
	return common.Market(code).IsKnown()
}

// IsValidLocale 轻量语言标签校验：xx 或 xx-XX（compact §5.5，仅格式不限白名单）。
func IsValidLocale(locale string) bool {
	return localePattern.MatchString(locale)
}

// IsValidOptionalURL URL 非空时要求 http(s):// 前缀（compact：format=url 非空时）。
func IsValidOptionalURL(u string) bool {
	if u == "" {
		return true
	}
	return urlPattern.MatchString(u)
}

// GenerateGameID 生成对外 game_id：从 100000 起自增数字串（compact §5.1）。
// env 入参保留以对齐 compact 签名（各环境 schema 独立计数，值不含 env 语义）。
func GenerateGameID(env common.Environment, lastSeq int64) string {
	_ = env
	next := lastSeq + 1
	if next < gameIDSeqStart {
		next = gameIDSeqStart
	}
	return strconv.FormatInt(next, 10)
}

// GenerateGameSecret 生成高熵随机密钥（hex，≥32 字节熵，≤128 字符）；随机源注入（compact §5.2）。
func GenerateGameSecret(r io.Reader) (string, error) {
	buf := make([]byte, secretEntropyBytes)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// ApplyDefaultMarket 缺省补默认市场（compact §5.3）：
//   - defaultMarketCode 缺省取 GLOBAL；
//   - markets 为空则取 [defaultMarketCode]；
//   - defaultMarketCode 不在 markets 中则补入；
//   - marketCode==defaultMarketCode 那条 isDefault=true 其余 false，enabled=true、defaultLocale=en-US。
//
// 返回归一化后的市场集合与解析出的 defaultMarketCode。入参合法性（枚举）由调用方先行校验。
func ApplyDefaultMarket(marketCodes []string, defaultMarketCode string) ([]GameMarket, string) {
	dmc := defaultMarketCode
	if dmc == "" {
		dmc = string(common.MarketGlobal)
	}

	codes := make([]string, 0, len(marketCodes)+1)
	seen := map[string]bool{}
	for _, c := range marketCodes {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		codes = append(codes, c)
	}
	if len(codes) == 0 {
		codes = append(codes, dmc)
		seen[dmc] = true
	}
	if !seen[dmc] {
		codes = append(codes, dmc)
	}

	markets := make([]GameMarket, 0, len(codes))
	for _, c := range codes {
		markets = append(markets, GameMarket{
			MarketCode:    c,
			IsDefault:     c == dmc,
			Enabled:       true,
			DefaultLocale: "en-US",
		})
	}
	return markets, dmc
}

// ValidateMarkets 校验市场集合自洽（compact §5.6 / 聚合一致性）：
// 非空 + 每项枚举合法 + marketCode 不重复 + 恰好一条 isDefault=true 且 ==defaultMarketCode
// + 被标默认的市场其 enabled 必须为 true（默认市场须 ∈ 已启用 markets，与 PATCH defaultMarketCode 语义一致）。
func ValidateMarkets(markets []GameMarket, defaultMarketCode string) error {
	if len(markets) == 0 {
		return ErrEmptyMarkets
	}
	seen := map[string]bool{}
	defaultCount := 0
	var defaultCode string
	var defaultEnabled bool
	for _, m := range markets {
		if !IsValidMarket(m.MarketCode) {
			return ErrInvalidMarket
		}
		if seen[m.MarketCode] {
			return ErrDuplicateMarket
		}
		seen[m.MarketCode] = true
		if m.IsDefault {
			defaultCount++
			defaultCode = m.MarketCode
			defaultEnabled = m.Enabled
		}
	}
	if defaultCount != 1 {
		return ErrNotExactlyOneDefault
	}
	if defaultMarketCode != "" && defaultCode != defaultMarketCode {
		return ErrDefaultMismatch
	}
	if !defaultEnabled {
		return ErrDefaultMarketNotEnabled
	}
	return nil
}

// ValidateLegalScope 校验并归一化法务作用域取值（compact §5.5）：
//   - default ⇒ scopeValue 必为 '*'（空视为 '*'）；
//   - market  ⇒ ∈ Market 枚举；
//   - locale  ⇒ 合法语言标签。
//
// 返回归一化后的 scopeValue。
func ValidateLegalScope(scopeType, scopeValue string) (string, error) {
	switch common.LegalScopeType(scopeType) {
	case common.LegalScopeDefault:
		if scopeValue == "" || scopeValue == "*" {
			return "*", nil
		}
		return "", ErrInvalidScopeValue
	case common.LegalScopeMarket:
		if !IsValidMarket(scopeValue) {
			return "", ErrInvalidScopeValue
		}
		return scopeValue, nil
	case common.LegalScopeLocale:
		if !IsValidLocale(scopeValue) {
			return "", ErrInvalidScopeValue
		}
		return scopeValue, nil
	default:
		return "", ErrInvalidScopeType
	}
}
