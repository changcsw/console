package channels

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	mw "github.com/csw/console/services/admin-api/internal/transport/http/middleware"
)

// RegisterRoutes 把 channel 路由注册到已挂载于 /api/admin 的父路由。
// 中间件链：父级 EnvContext → Authn(Bearer access) → RequireBackend → Audit → RequirePerm(channel.read/channel.write)。
// ready=false（后端未就绪）：受保护路由先过 Authn（无令牌仍 401），通过后 RequireBackend 返回 503，契约形状稳定。
func RegisterRoutes(r chi.Router, h *Handler, issuer adminapp.TokenIssuer, env common.Environment, logger *slog.Logger, ready bool, auditMW mw.AuditWriter) {
	r.Group(func(gr chi.Router) {
		gr.Use(mw.Authn(issuer, env))
		gr.Use(mw.RequireBackend(ready))
		gr.Use(mw.Audit(logger, env, auditMW))

		// 游戏维度：候选渠道 + 渠道实例列表/创建。
		gr.With(mw.RequirePerm("channel.read")).Get("/games/{gameId}/channels", h.ListChannelOptions)
		gr.With(mw.RequirePerm("channel.read")).Get("/games/{gameId}/market-channels", h.ListMarketChannels)
		gr.With(mw.RequirePerm("channel.write")).Post("/games/{gameId}/markets/{market}/channels", h.CreateMarketChannel)

		// 渠道实例（按 int64 gameChannelId）。
		gr.With(mw.RequirePerm("channel.read")).Get("/game-channels/{gameChannelId}", h.GetMarketChannel)
		gr.With(mw.RequirePerm("channel.write")).Patch("/game-channels/{gameChannelId}", h.UpdateMarketChannel)
		gr.With(mw.RequirePerm("channel.write")).Post("/game-channels/{gameChannelId}/hide", h.HideMarketChannel)
		gr.With(mw.RequirePerm("channel.write")).Post("/game-channels/{gameChannelId}/unhide", h.UnhideMarketChannel)
		gr.With(mw.RequirePerm("channel.read")).Get("/game-channels/{gameChannelId}/login-config", h.GetLoginConfig)
		gr.With(mw.RequirePerm("channel.write")).Put("/game-channels/{gameChannelId}/login-config", h.PutLoginConfig)
		gr.With(mw.RequirePerm("plugin.read")).Get("/game-channels/{gameChannelId}/plugins", h.ListChannelPlugins)
		gr.With(mw.RequirePerm("plugin.write")).Post("/game-channels/{gameChannelId}/plugins", h.ConfigureChannelPlugin)
		gr.With(mw.RequirePerm("plugin.write")).Patch("/game-channel-plugins/{id}", h.PatchChannelPlugin)

		// 渠道包。
		gr.With(mw.RequirePerm("channel.read")).Get("/game-channels/{gameChannelId}/packages", h.ListPackages)
		gr.With(mw.RequirePerm("channel.write")).Post("/game-channels/{gameChannelId}/packages", h.CreatePackage)
		gr.With(mw.RequirePerm("channel.write")).Patch("/channel-packages/{packageId}", h.UpdatePackage)
		gr.With(mw.RequirePerm("plugin.read")).Get("/channel-packages/{packageId}/plugins", h.ListPackagePlugins)
		gr.With(mw.RequirePerm("plugin.write")).Post("/channel-packages/{packageId}/plugins", h.OverridePackagePlugin)
	})
}
