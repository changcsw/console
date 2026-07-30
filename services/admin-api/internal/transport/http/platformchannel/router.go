package platformchannel

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	mw "github.com/csw/console/services/admin-api/internal/transport/http/middleware"
)

// RegisterRoutes 把平台渠道与渠道模版路由注册到已挂载于 /api/admin 的父路由。
// 渠道主数据用 platform_channel.*，模版四件套用 channel_template.*：两者都是系统管理员职责，
// 与游戏侧的 channel.*（渠道实例读写）分开授权。
func RegisterRoutes(r chi.Router, h *Handler, issuer adminapp.TokenIssuer, env common.Environment, logger *slog.Logger, ready bool, auditMW mw.AuditWriter) {
	r.Group(func(gr chi.Router) {
		gr.Use(mw.Authn(issuer, env))
		gr.Use(mw.RequireBackend(ready))
		gr.Use(mw.Audit(logger, env, auditMW))

		gr.With(mw.RequirePerm("platform_channel.read")).Get("/platform/channels", h.ListChannels)
		gr.With(mw.RequirePerm("platform_channel.write")).Post("/platform/channels", h.CreateChannel)
		gr.With(mw.RequirePerm("platform_channel.read")).Get("/platform/channels/{channelId}", h.GetChannel)
		gr.With(mw.RequirePerm("platform_channel.write")).Patch("/platform/channels/{channelId}", h.UpdateChannel)

		gr.With(mw.RequirePerm("channel_template.read")).Get("/platform/channels/{channelId}/templates", h.ListTemplates)
		gr.With(mw.RequirePerm("channel_template.write")).Post("/platform/channels/{channelId}/templates", h.CreateTemplate)
		gr.With(mw.RequirePerm("channel_template.read")).Get("/platform/channel-templates/{kind}/{templateId}", h.GetTemplate)
		gr.With(mw.RequirePerm("channel_template.write")).Patch("/platform/channel-templates/{kind}/{templateId}", h.UpdateTemplate)
	})
}
