package games

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	mw "github.com/csw/console/services/admin-api/internal/transport/http/middleware"
)

// RegisterRoutes 把 game 路由注册到已挂载于 /api/admin 的父路由（auth 子路由）。
// 中间件链：父级 EnvContext → Authn(Bearer access) → RequireBackend → Audit → RequirePerm(game.read/game.write)。
// ready=false（后端未就绪）：受保护路由先过 Authn（无令牌仍 401），通过后 RequireBackend 返回 503，契约形状稳定。
func RegisterRoutes(r chi.Router, h *Handler, issuer adminapp.TokenIssuer, env common.Environment, logger *slog.Logger, ready bool, auditMW mw.AuditWriter) {
	r.Group(func(gr chi.Router) {
		gr.Use(mw.Authn(issuer, env))
		gr.Use(mw.RequireBackend(ready))
		gr.Use(mw.Audit(logger, env, auditMW))

		gr.With(mw.RequirePerm("game.read")).Get("/games", h.ListGames)
		gr.With(mw.RequirePerm("game.write")).Post("/games", h.CreateGame)
		gr.With(mw.RequirePerm("game.read")).Get("/games/{gameId}", h.GetGame)
		gr.With(mw.RequirePerm("game.write")).Patch("/games/{gameId}", h.UpdateGame)
		gr.With(mw.RequirePerm("game.write")).Put("/games/{gameId}/markets", h.ReplaceMarkets)
		gr.With(mw.RequirePerm("game.write")).Put("/games/{gameId}/legal-links", h.ReplaceLegalLinks)

		// account-auth（模块 13）后端接口挂在 games transport 目录。
		gr.With(mw.RequirePerm("game.read")).Get("/account-auth/types", h.ListAccountAuthTypes)
		gr.With(mw.RequirePerm("game.read")).Get("/channels/{channelId}/account-auth-types", h.ListChannelAccountAuthTypes)
		gr.With(mw.RequirePerm("game.read")).Get("/games/{gameId}/account-auth-configs", h.GetGameAccountAuthConfigs)
		gr.With(mw.RequirePerm("game.write")).Put("/games/{gameId}/account-auth-configs", h.ReplaceGameAccountAuthConfigs)

		// product（模块 16）路由组，挂载在 games 共享 surface。
		gr.With(mw.RequirePerm("product.read")).Get("/games/{gameId}/products", h.ListProducts)
		gr.With(mw.RequirePerm("product.write")).Post("/games/{gameId}/products", h.CreateProduct)
		gr.With(mw.RequirePerm("product.write")).Patch("/products/{productId}", h.UpdateProduct)
		gr.With(mw.RequirePerm("product.read")).Get("/channel-packages/{packageId}/products", h.GetPackageProducts)
		gr.With(mw.RequirePerm("product.write")).Put("/channel-packages/{packageId}/products", h.PutPackageProducts)
		gr.With(mw.RequirePerm("product.read")).Get("/game-channels/{gameChannelId}/iap-config", h.GetGameChannelIAPConfig)
		gr.With(mw.RequirePerm("product.write")).Put("/game-channels/{gameChannelId}/iap-config", h.PutGameChannelIAPConfig)
		gr.With(mw.RequirePerm("product.read")).Get("/channel-packages/{packageId}/iap-override", h.GetPackageIAPOverride)
		gr.With(mw.RequirePerm("product.write")).Put("/channel-packages/{packageId}/iap-override", h.PutPackageIAPOverride)
	})
}
