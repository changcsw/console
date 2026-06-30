// Package middleware 提供后台鉴权中间件链：EnvContext -> Authn -> Authz -> Audit（compact）。
package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	auditapp "github.com/csw/console/services/admin-api/internal/app/audit"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
	chimw "github.com/go-chi/chi/v5/middleware"
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

// AuditWriter 定义审计写入端口（用于中间件兜底）。
type AuditWriter interface {
	Write(ctx context.Context, in auditapp.SecretAwareAuditInput) error
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Audit 负责审计上下文注入 + 写操作兜底 + 去重（失败不阻断主流程）。
func Audit(logger *slog.Logger, env common.Environment, writer AuditWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actorID := int64(0)
			if ac, ok := adminapp.AuthContextFrom(r.Context()); ok {
				actorID = ac.UserID
			}
			reqMeta := common.AuditRequestMeta{
				IP:        clientIP(r),
				UserAgent: r.UserAgent(),
				RequestID: chimw.GetReqID(r.Context()),
				Method:    r.Method,
				Path:      r.URL.Path,
			}
			ctx := auditapp.InjectRequestContext(r.Context(), actorID, env, reqMeta)
			r = r.WithContext(ctx)

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			if !isWriteMethod(r.Method) || rec.status < 200 || rec.status >= 300 {
				return
			}
			if auditapp.IsWritten(r.Context()) || writer == nil {
				return
			}

			action, resourceType, resourceID := inferFallbackAudit(r)
			err := writer.Write(r.Context(), auditapp.SecretAwareAuditInput{
				AuditWriteInput: auditapp.AuditWriteInput{
					ActorID:      actorID,
					Action:       action,
					ResourceType: resourceType,
					ResourceID:   resourceID,
					Detail: common.AuditDetail{
						Summary: "middleware fallback",
						Request: &reqMeta,
					},
				},
			})
			if err != nil && logger != nil {
				logger.Error("audit middleware fallback failed",
					"err", err, "method", r.Method, "path", r.URL.Path, "status", rec.status,
					"action", action, "resourceType", resourceType, "resourceID", resourceID, "actorID", actorID)
			}
		})
	}
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

var digitRegex = regexp.MustCompile(`^\d+$`)

func inferFallbackAudit(r *http.Request) (action, resourceType, resourceID string) {
	pattern := r.URL.Path
	segments := strings.Split(strings.Trim(strings.TrimPrefix(pattern, "/api/admin"), "/"), "/")
	clean := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "" || strings.HasPrefix(seg, "{") || digitRegex.MatchString(seg) {
			continue
		}
		clean = append(clean, normalizeSegment(seg))
	}
	if len(clean) == 0 {
		resourceType = "admin"
	} else {
		resourceType = singular(clean[0])
	}
	last := ""
	if len(clean) > 0 {
		last = clean[len(clean)-1]
	}
	switch last {
	case "create", "update", "delete", "enable", "disable", "hide", "unhide", "publish", "archive", "copy", "approve", "reject", "ignore", "apply", "execute", "logout", "login":
		action = resourceType + "." + last
	default:
		action = resourceType + "." + methodToAction(r.Method)
	}
	resourceID = strings.TrimSpace(r.URL.Query().Get("id"))
	if resourceID == "" {
		resourceID = strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin"), "/")
		if resourceID == "" {
			resourceID = "unknown"
		}
	}
	const maxAuditResourceIDLen = 128
	if len(resourceID) > maxAuditResourceIDLen {
		resourceID = resourceID[:maxAuditResourceIDLen]
	}
	return action, resourceType, resourceID
}

func methodToAction(method string) string {
	switch method {
	case http.MethodPost:
		return "create"
	case http.MethodDelete:
		return "delete"
	default:
		return "update"
	}
}

func normalizeSegment(in string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(in)), "-", "_")
}

func singular(in string) string {
	if strings.HasSuffix(in, "ies") {
		return strings.TrimSuffix(in, "ies") + "y"
	}
	if strings.HasSuffix(in, "s") {
		return strings.TrimSuffix(in, "s")
	}
	return in
}

func clientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
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
