package audit

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	mw "github.com/csw/console/services/admin-api/internal/transport/http/middleware"
)

// RegisterRoutes 注册审计查询接口（只读，权限 audit.read）。
func RegisterRoutes(r chi.Router, h *Handler, issuer adminapp.TokenIssuer, env common.Environment, logger *slog.Logger, ready bool, auditMW mw.AuditWriter) {
	r.Group(func(ar chi.Router) {
		ar.Use(mw.Authn(issuer, env))
		ar.Use(mw.RequireBackend(ready))
		ar.Use(mw.Audit(logger, env, auditMW))
		ar.With(mw.RequirePerm("audit.read")).Get("/audit-logs", h.List)
		// facets 为 compact §可选接口，本期未实现（前端 facets 失败回退静态字典）。
		// 显式注册静态段，使其返回 NOT_FOUND(404) 而非落到 /{id} 把 "facets" 当 id 解析报 400。
		ar.With(mw.RequirePerm("audit.read")).Get("/audit-logs/facets", h.Facets)
		ar.With(mw.RequirePerm("audit.read")).Get("/audit-logs/{id}", h.GetByID)
	})
}
