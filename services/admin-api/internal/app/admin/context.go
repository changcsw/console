package admin

import (
	"context"

	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
)

type ctxKey int

const (
	ctxKeyAuth ctxKey = iota
)

// WithAuthContext 把鉴权上下文注入 context（Authn 中间件调用）。
func WithAuthContext(ctx context.Context, ac domainauth.AuthContext) context.Context {
	return context.WithValue(ctx, ctxKeyAuth, ac)
}

// AuthContextFrom 从 context 取鉴权上下文。
func AuthContextFrom(ctx context.Context) (domainauth.AuthContext, bool) {
	ac, ok := ctx.Value(ctxKeyAuth).(domainauth.AuthContext)
	return ac, ok
}

func actorFromCtx(ctx context.Context) int64 {
	if ac, ok := AuthContextFrom(ctx); ok {
		return ac.UserID
	}
	return 0
}
