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
func RegisterRoutes(r chi.Router, h *Handler, issuer adminapp.TokenIssuer, env common.Environment, logger *slog.Logger, ready bool) {
	r.Group(func(gr chi.Router) {
		gr.Use(mw.Authn(issuer, env))
		gr.Use(mw.RequireBackend(ready))
		gr.Use(mw.Audit(logger))

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
	})
}
