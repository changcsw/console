package snapshot

import (
	"log/slog"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	mw "github.com/csw/console/services/admin-api/internal/transport/http/middleware"
	"github.com/go-chi/chi/v5"
)

func RegisterRoutes(r chi.Router, h *Handler, issuer adminapp.TokenIssuer, env common.Environment, logger *slog.Logger, ready bool, auditMW mw.AuditWriter) {
	r.Group(func(gr chi.Router) {
		gr.Use(mw.Authn(issuer, env))
		gr.Use(mw.RequireBackend(ready))
		gr.Use(mw.Audit(logger, env, auditMW))

		gr.With(mw.RequirePerm("snapshot.generate")).Post("/games/{gameId}/config-snapshots/generate", h.Generate)
		gr.With(mw.RequirePerm("game.read")).Get("/games/{gameId}/config-snapshots", h.List)
		gr.With(mw.RequirePerm("snapshot.publish")).Post("/game-config-snapshots/{snapshotId}/publish", h.Publish)
		gr.With(mw.RequirePerm("game.read")).Get("/game-config-snapshots/{snapshotId}/download", h.Download)
	})
}
