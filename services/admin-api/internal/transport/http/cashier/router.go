package cashier

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	mw "github.com/csw/console/services/admin-api/internal/transport/http/middleware"
)

func RegisterRoutes(r chi.Router, h *Handler, issuer adminapp.TokenIssuer, env common.Environment, logger *slog.Logger, ready bool, auditMW mw.AuditWriter) {
	r.Group(func(gr chi.Router) {
		gr.Use(mw.Authn(issuer, env))
		gr.Use(mw.RequireBackend(ready))
		gr.Use(mw.Audit(logger, env, auditMW))

		gr.With(mw.RequirePerm("cashier.read")).Get("/cashier/templates", h.ListTemplates)
		gr.With(mw.RequirePerm("cashier.write")).Post("/cashier/templates", h.CreateTemplate)
		gr.With(mw.RequirePerm("cashier.read")).Get("/cashier/templates/{templateId}", h.GetTemplate)

		gr.With(mw.RequirePerm("cashier.write")).Post("/cashier/templates/{templateId}/versions", h.CreateVersion)
		gr.With(mw.RequirePerm("cashier.write")).Post("/cashier/templates/{templateId}/versions/{version}/copy-to-draft", h.CopyToDraft)
		gr.With(mw.RequirePerm("cashier.read")).Get("/cashier/templates/{templateId}/versions/{version}/rows", h.ListRows)
		gr.With(mw.RequirePerm("cashier.write")).Put("/cashier/templates/{templateId}/versions/{version}/rows", h.UpsertRows)
		gr.With(mw.RequirePerm("cashier.publish")).Post("/cashier/templates/{templateId}/versions/{version}/publish", h.PublishVersion)

		gr.With(mw.RequirePerm("cashier.write")).Post("/cashier/templates/{templateId}/fx-sync/runs", h.CreateFXRun)
		gr.With(mw.RequirePerm("fx.approve")).Post("/cashier/fx-sync-runs/{runId}/approve", h.ApproveFXRun)

		gr.With(mw.RequirePerm("cashier.read")).Get("/games/{gameId}/cashier/profile", h.GetGameProfile)
		gr.With(mw.RequirePerm("cashier.write")).Put("/games/{gameId}/cashier/profile", h.BindGameProfile)
		gr.With(mw.RequirePerm("cashier.read")).Get("/games/{gameId}/cashier/price-overrides", h.ListGamePriceOverrides)
		gr.With(mw.RequirePerm("cashier.write")).Put("/games/{gameId}/cashier/price-overrides", h.SaveGamePriceOverrides)
	})
}
