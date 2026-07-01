package payment

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

		gr.With(mw.RequirePerm("payment.read")).Get("/pay-ways", h.ListPayWays)
		gr.With(mw.RequirePerm("payment.read")).Get("/cashier/providers", h.ListProviders)
		gr.With(mw.RequirePerm("payment.read")).Get("/cashier/providers/{providerId}/template", h.GetProviderTemplate)
		gr.With(mw.RequirePerm("payment.read")).Get("/billing-subjects", h.ListBillingSubjects)
		gr.With(mw.RequirePerm("payment.write")).Post("/billing-subjects", h.CreateBillingSubject)
		gr.With(mw.RequirePerm("payment.read")).Get("/cashier/merchant-accounts", h.ListMerchantAccounts)
		gr.With(mw.RequirePerm("payment.write")).Post("/cashier/merchant-accounts", h.CreateMerchantAccount)
		gr.With(mw.RequirePerm("payment.read")).Get("/games/{gameId}/payment-routes", h.GetGameRoutes)
		gr.With(mw.RequirePerm("payment.write")).Put("/games/{gameId}/payment-routes", h.PutGameRoutes)
	})
}
