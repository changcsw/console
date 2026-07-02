package dashboard

import (
	"log/slog"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	mw "github.com/csw/console/services/admin-api/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, h *Handler, issuer adminapp.TokenIssuer, env common.Environment, _ *slog.Logger, ready bool, _ mw.AuditWriter) {
	r.Group(func(gr chi.Router) {
		gr.Use(mw.Authn(issuer, env))
		gr.Use(mw.RequireBackend(ready))
		gr.With(mw.RequirePerm("dashboard.read")).Get("/dashboard/summary", h.Summary)
	})
}
