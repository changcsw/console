package jwt

import (
	"testing"
	"time"

	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

func newTestIssuer(t *testing.T) *Issuer {
	t.Helper()
	iss, err := NewIssuer(Config{Secret: "test-secret", Issuer: "admin-api", AccessTTL: 30 * time.Minute, RefreshTTL: 336 * time.Hour})
	if err != nil {
		t.Fatalf("new issuer: %v", err)
	}
	return iss
}

func TestNewIssuerRejectsEmptySecret(t *testing.T) {
	if _, err := NewIssuer(Config{Secret: ""}); err == nil {
		t.Fatal("empty secret must error")
	}
}

func TestIssueAndParse(t *testing.T) {
	iss := newTestIssuer(t)
	ac := domainauth.NewAuthContext(42, "alice", "Alice",
		[]string{"super_admin"}, []string{"game.read", "sync.execute"}, common.EnvDevelop)

	pair, err := iss.IssuePair(ac)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("tokens must be non-empty")
	}

	claims, err := iss.Parse(pair.AccessToken, domainauth.TokenTypeAccess)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if claims.Subject != "42" || claims.UserName != "alice" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	id, err := SubjectID(claims)
	if err != nil || id != 42 {
		t.Fatalf("subject id: %d %v", id, err)
	}

	// refresh token 必须 typ=refresh，按 access 解析应失败
	if _, err := iss.Parse(pair.RefreshToken, domainauth.TokenTypeAccess); err == nil {
		t.Fatal("refresh parsed as access must fail")
	}
	if _, err := iss.Parse(pair.RefreshToken, domainauth.TokenTypeRefresh); err != nil {
		t.Fatalf("refresh parse: %v", err)
	}
}

func TestParseRejectsExpired(t *testing.T) {
	iss := newTestIssuer(t)
	// 把签发时钟拨回到 2 小时前，使 30m 的 access 已过期。
	iss.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	ac := domainauth.NewAuthContext(1, "alice", "Alice", nil, []string{"game.read"}, common.EnvDevelop)
	pair, err := iss.IssuePair(ac)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	// 用真实时钟校验：access/refresh(14d) 中 access 已过期。
	iss.now = time.Now
	if _, err := iss.Parse(pair.AccessToken, domainauth.TokenTypeAccess); err == nil {
		t.Fatal("expired access token must be rejected")
	}
	// refresh TTL=14d 仍有效（仅验证过期判定不误伤 refresh）。
	if _, err := iss.Parse(pair.RefreshToken, domainauth.TokenTypeRefresh); err != nil {
		t.Fatalf("refresh within ttl must parse: %v", err)
	}
}

func TestParseRejectsExpiredRefresh(t *testing.T) {
	iss, err := NewIssuer(Config{Secret: "s", Issuer: "admin-api", AccessTTL: time.Minute, RefreshTTL: time.Minute})
	if err != nil {
		t.Fatalf("new issuer: %v", err)
	}
	iss.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	pair, _ := iss.IssuePair(domainauth.NewAuthContext(1, "a", "A", nil, nil, common.EnvDevelop))
	iss.now = time.Now
	if _, err := iss.Parse(pair.RefreshToken, domainauth.TokenTypeRefresh); err == nil {
		t.Fatal("expired refresh token must be rejected")
	}
}

func TestParseTypMismatchBothDirections(t *testing.T) {
	iss := newTestIssuer(t)
	pair, _ := iss.IssuePair(domainauth.NewAuthContext(1, "a", "A", nil, nil, common.EnvDevelop))
	// access 当 refresh 用 → typ 不符
	if _, err := iss.Parse(pair.AccessToken, domainauth.TokenTypeRefresh); err == nil {
		t.Fatal("access parsed as refresh must fail")
	}
	// refresh 当 access 用 → typ 不符（已在 TestIssueAndParse 验证，这里再固化方向）
	if _, err := iss.Parse(pair.RefreshToken, domainauth.TokenTypeAccess); err == nil {
		t.Fatal("refresh parsed as access must fail")
	}
	// expectedType="" 跳过 typ 校验（中间件之外的宽松解析）
	if _, err := iss.Parse(pair.AccessToken, ""); err != nil {
		t.Fatalf("empty expectedType must skip typ check: %v", err)
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	iss := newTestIssuer(t)
	for _, tok := range []string{"", "not-a-jwt", "a.b.c", "Bearer xxx"} {
		if _, err := iss.Parse(tok, domainauth.TokenTypeAccess); err == nil {
			t.Errorf("malformed token %q must be rejected", tok)
		}
	}
}

func TestSubjectIDInvalid(t *testing.T) {
	if _, err := SubjectID(domainauth.Claims{Subject: "not-a-number"}); err == nil {
		t.Fatal("non-numeric subject must error")
	}
	id, err := SubjectID(domainauth.Claims{Subject: "42"})
	if err != nil || id != 42 {
		t.Fatalf("subject id parse: %d %v", id, err)
	}
}

func TestIssuePairUniqueJTI(t *testing.T) {
	iss := newTestIssuer(t)
	pair, _ := iss.IssuePair(domainauth.NewAuthContext(1, "a", "A", nil, nil, common.EnvDevelop))
	ac1, _ := iss.Parse(pair.AccessToken, domainauth.TokenTypeAccess)
	rc1, _ := iss.Parse(pair.RefreshToken, domainauth.TokenTypeRefresh)
	if ac1.JTI == "" || rc1.JTI == "" {
		t.Fatal("jti must be set on both tokens")
	}
	if ac1.JTI == rc1.JTI {
		t.Fatal("access and refresh must have distinct jti")
	}
}

func TestParseRejectsTampered(t *testing.T) {
	iss := newTestIssuer(t)
	other, _ := NewIssuer(Config{Secret: "different", Issuer: "admin-api", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	ac := domainauth.NewAuthContext(1, "a", "A", nil, nil, common.EnvDevelop)
	pair, _ := other.IssuePair(ac)
	if _, err := iss.Parse(pair.AccessToken, domainauth.TokenTypeAccess); err == nil {
		t.Fatal("token signed by other secret must be rejected")
	}
}
