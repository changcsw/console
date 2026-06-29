// Package jwt 用 HS256 签发/校验后台访问与刷新令牌（compact「Token」）。
package jwt

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"

	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
)

var (
	// ErrInvalidToken 令牌签名/格式/过期校验失败。
	ErrInvalidToken = errors.New("invalid token")
	// ErrWrongTokenType typ 不符合预期（access vs refresh）。
	ErrWrongTokenType = errors.New("wrong token type")
)

// Issuer 负责签发与校验 JWT。
type Issuer struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

// Config Issuer 构造参数。
type Config struct {
	Secret     string
	Issuer     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// NewIssuer 构造 Issuer；secret 为空返回错误（禁硬编码/禁空密钥）。
func NewIssuer(cfg Config) (*Issuer, error) {
	if cfg.Secret == "" {
		return nil, errors.New("jwt secret must not be empty")
	}
	return &Issuer{
		secret:     []byte(cfg.Secret),
		issuer:     cfg.Issuer,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		now:        time.Now,
	}, nil
}

type claims struct {
	Type        string   `json:"typ"`
	UserName    string   `json:"userName,omitempty"`
	DisplayName string   `json:"displayName,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Perms       []string `json:"perms,omitempty"`
	jwtv5.RegisteredClaims
}

// AccessExpiry 返回 access 令牌的过期时间（用于响应透出 expiresAt）。
func (i *Issuer) AccessExpiry() time.Time { return i.now().Add(i.accessTTL) }

// IssuePair 同时签发 access(含 roles/perms) 与 refresh(仅 sub/jti/exp)，jti 各异。
func (i *Issuer) IssuePair(ac domainauth.AuthContext) (domainauth.TokenPair, error) {
	now := i.now()
	accessExp := now.Add(i.accessTTL)
	sub := strconv.FormatInt(ac.UserID, 10)

	access, err := i.sign(claims{
		Type:        domainauth.TokenTypeAccess,
		UserName:    ac.UserName,
		DisplayName: ac.DisplayName,
		Roles:       ac.Roles,
		Perms:       ac.Permissions(),
		RegisteredClaims: jwtv5.RegisteredClaims{
			Subject:   sub,
			Issuer:    i.issuer,
			IssuedAt:  jwtv5.NewNumericDate(now),
			ExpiresAt: jwtv5.NewNumericDate(accessExp),
			ID:        newJTI(),
		},
	})
	if err != nil {
		return domainauth.TokenPair{}, err
	}

	refresh, err := i.sign(claims{
		Type: domainauth.TokenTypeRefresh,
		RegisteredClaims: jwtv5.RegisteredClaims{
			Subject:   sub,
			Issuer:    i.issuer,
			IssuedAt:  jwtv5.NewNumericDate(now),
			ExpiresAt: jwtv5.NewNumericDate(now.Add(i.refreshTTL)),
			ID:        newJTI(),
		},
	})
	if err != nil {
		return domainauth.TokenPair{}, err
	}

	return domainauth.TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: accessExp}, nil
}

func (i *Issuer) sign(c claims) (string, error) {
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, c)
	return tok.SignedString(i.secret)
}

// Parse 校验签名/exp，并要求 typ==expectedType；返回领域 Claims。
func (i *Issuer) Parse(tokenStr, expectedType string) (domainauth.Claims, error) {
	var c claims
	parsed, err := jwtv5.ParseWithClaims(tokenStr, &c, func(t *jwtv5.Token) (any, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	}, jwtv5.WithValidMethods([]string{"HS256"}))
	if err != nil || !parsed.Valid {
		return domainauth.Claims{}, ErrInvalidToken
	}
	if expectedType != "" && c.Type != expectedType {
		return domainauth.Claims{}, ErrWrongTokenType
	}

	out := domainauth.Claims{
		Subject:     c.Subject,
		Type:        c.Type,
		UserName:    c.UserName,
		DisplayName: c.DisplayName,
		Roles:       c.Roles,
		Perms:       c.Perms,
		Issuer:      c.Issuer,
		JTI:         c.ID,
	}
	if c.IssuedAt != nil {
		out.IssuedAt = c.IssuedAt.Unix()
	}
	if c.ExpiresAt != nil {
		out.ExpiresAt = c.ExpiresAt.Unix()
	}
	return out, nil
}

// SubjectID 把 claims.sub 解析为 int64 用户 ID。
func SubjectID(c domainauth.Claims) (int64, error) {
	return strconv.ParseInt(c.Subject, 10, 64)
}

func newJTI() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
