// Package middleware 提供后台鉴权中间件链：EnvContext -> Authn -> Authz -> Audit（compact）。
package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

// EnvContext 注入运行环境到响应头 X-Environment（不逐请求 SET search_path，01 §4.4）。
func EnvContext(env common.Environment) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Environment", string(env))
			next.ServeHTTP(w, r)
		})
	}
}

// Authn 校验 Bearer access（签名/exp/typ=access），从 claims 还原 AuthContext（不回库）。
func Authn(issuer adminapp.TokenIssuer, env common.Environment) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "缺少访问令牌")
				return
			}
			claims, err := issuer.Parse(token, domainauth.TokenTypeAccess)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "令牌无效或已过期")
				return
			}
			userID, err := strconv.ParseInt(claims.Subject, 10, 64)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "令牌无效")
				return
			}
			ac := domainauth.NewAuthContext(userID, claims.UserName, claims.DisplayName, claims.Roles, claims.Perms, env)
			ctx := adminapp.WithAuthContext(r.Context(), ac)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePerm 路由级权限码校验：code ∈ ctx.perms（super_admin 短路放行）。
func RequirePerm(code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, ok := adminapp.AuthContextFrom(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "未认证")
				return
			}
			if !ac.HasPermission(code) {
				httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden, "无权限")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireBackend 后端就绪门：ready=false 时返回 503 INTERNAL（用于 DB 未配置/不可用时）。
func RequireBackend(ready bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !ready {
				httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "后端未就绪")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Audit 写操作访问日志（authoritative 审计在 service 层写 audit_logs；此处仅结构化日志，不阻断主流程）。
func Audit(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
			if logger != nil && r.Method != http.MethodGet {
				actor := int64(0)
				if ac, ok := adminapp.AuthContextFrom(r.Context()); ok {
					actor = ac.UserID
				}
				logger.Info("admin write request", "method", r.Method, "path", r.URL.Path, "actor", actor)
			}
		})
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, prefix))
}
