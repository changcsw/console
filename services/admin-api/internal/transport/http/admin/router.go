package admin

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	mw "github.com/csw/console/services/admin-api/internal/transport/http/middleware"
)

// NewRouter 构造 auth 子路由（挂载于 /api/admin）。
// 中间件链：EnvContext -> [Authn -> Audit] -> RequirePerm -> Handler（compact 鉴权中间件链）。
// 豁免鉴权：/auth/login、/auth/refresh、/auth/feishu/callback。
// ready=false 表示后端（DB）未就绪：路由仍挂载，受保护路由先过 Authn（未携带令牌仍 401），
// 通过认证后返回 503 INTERNAL；公开路由直接 503。这样 API 形状稳定、契约一致。
func NewRouter(h *Handler, issuer adminapp.TokenIssuer, env common.Environment, logger *slog.Logger, ready bool, auditMW mw.AuditWriter) chi.Router {
	r := chi.NewRouter()
	r.Use(mw.EnvContext(env))
	readyMW := mw.RequireBackend(ready)

	// 公开（豁免鉴权）
	r.With(readyMW).Post("/auth/login", h.Login)
	r.With(readyMW).Post("/auth/refresh", h.Refresh)
	r.With(readyMW).Post("/auth/feishu/callback", h.FeishuCallback)

	// 需登录
	r.Group(func(pr chi.Router) {
		pr.Use(mw.Authn(issuer, env))
		pr.Use(readyMW)
		pr.Use(mw.Audit(logger, env, auditMW))

		pr.Post("/auth/logout", h.Logout)
		pr.Get("/me", h.Me)

		pr.Route("/system", func(sr chi.Router) {
			sr.With(mw.RequirePerm("system.read")).Get("/admin-users", h.ListUsers)
			sr.With(mw.RequirePerm("admin_user.write")).Post("/admin-users", h.CreateUser)
			sr.With(mw.RequirePerm("system.read")).Get("/admin-users/{id}", h.GetUser)
			sr.With(mw.RequirePerm("admin_user.write")).Patch("/admin-users/{id}", h.UpdateUser)
			sr.With(mw.RequirePerm("admin_user.write")).Put("/admin-users/{id}/roles", h.AssignUserRoles)
			sr.With(mw.RequirePerm("admin_user.write")).Post("/admin-users/{id}/reset-password", h.ResetPassword)

			sr.With(mw.RequirePerm("system.read")).Get("/roles", h.ListRoles)
			sr.With(mw.RequirePerm("role.write")).Post("/roles", h.CreateRole)
			sr.With(mw.RequirePerm("role.write")).Patch("/roles/{id}", h.UpdateRole)
			sr.With(mw.RequirePerm("role.write")).Delete("/roles/{id}", h.DeleteRole)
			sr.With(mw.RequirePerm("role.write")).Put("/roles/{id}/permissions", h.AssignRolePermissions)

			sr.With(mw.RequirePerm("system.read")).Get("/permissions", h.ListPermissions)
			sr.With(mw.RequirePerm("permission.write")).Post("/permissions", h.CreatePermission)
			sr.With(mw.RequirePerm("permission.write")).Delete("/permissions/{id}", h.DeletePermission)
		})
	})

	return r
}
