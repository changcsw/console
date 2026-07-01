package syncapi

import (
	"log/slog"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	mw "github.com/csw/console/services/admin-api/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, h *SectionSyncHandler, issuer adminapp.TokenIssuer, env common.Environment, logger *slog.Logger, ready bool, auditMW mw.AuditWriter) {
	r.Group(func(gr chi.Router) {
		gr.Use(mw.Authn(issuer, env))
		gr.Use(mw.RequireBackend(ready))
		gr.Use(mw.Audit(logger, env, auditMW))

		gr.With(mw.RequirePerm("sync.preview")).Post("/games/{gameId}/sync/preview", h.Preview)
		gr.With(mw.RequirePerm("sync.execute")).Post("/games/{gameId}/sync/execute", h.Execute)
		gr.With(mw.RequirePerm("sync.preview")).Get("/games/{gameId}/sync-jobs", h.ListJobs)
	})
}
