package featureplugin

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	mw "github.com/csw/console/services/admin-api/internal/transport/http/middleware"
)

// RegisterRoutes 把功能插件管理路由注册到已挂载于 /api/admin 的父路由。
// 分类字典 / 插件主数据 / 参数模板同属系统管理员的一套职责，共用 feature_plugin.read / .write；
// 与游戏侧渠道实例插件配置用的 plugin.read / plugin.write 分开授权。
func RegisterRoutes(r chi.Router, h *Handler, issuer adminapp.TokenIssuer, env common.Environment, logger *slog.Logger, ready bool, auditMW mw.AuditWriter) {
	r.Group(func(gr chi.Router) {
		gr.Use(mw.Authn(issuer, env))
		gr.Use(mw.RequireBackend(ready))
		gr.Use(mw.Audit(logger, env, auditMW))

		gr.With(mw.RequirePerm("feature_plugin.read")).Get("/feature-plugin-categories", h.ListCategories)
		gr.With(mw.RequirePerm("feature_plugin.write")).Post("/feature-plugin-categories", h.CreateCategory)
		gr.With(mw.RequirePerm("feature_plugin.write")).Patch("/feature-plugin-categories/{id}", h.UpdateCategory)
		gr.With(mw.RequirePerm("feature_plugin.write")).Delete("/feature-plugin-categories/{id}", h.DeleteCategory)

		gr.With(mw.RequirePerm("feature_plugin.read")).Get("/feature-plugins", h.ListPlugins)
		gr.With(mw.RequirePerm("feature_plugin.write")).Post("/feature-plugins", h.CreatePlugin)
		gr.With(mw.RequirePerm("feature_plugin.read")).Get("/feature-plugins/{pluginId}", h.GetPlugin)
		gr.With(mw.RequirePerm("feature_plugin.write")).Patch("/feature-plugins/{pluginId}", h.UpdatePlugin)
		gr.With(mw.RequirePerm("feature_plugin.write")).Delete("/feature-plugins/{pluginId}", h.DeletePlugin)

		gr.With(mw.RequirePerm("feature_plugin.read")).Get("/feature-plugins/{pluginId}/templates", h.ListTemplates)
		gr.With(mw.RequirePerm("feature_plugin.write")).Post("/feature-plugins/{pluginId}/templates", h.CreateTemplate)
		gr.With(mw.RequirePerm("feature_plugin.read")).Get("/feature-plugin-templates/{templateId}", h.GetTemplate)
		gr.With(mw.RequirePerm("feature_plugin.write")).Patch("/feature-plugin-templates/{templateId}", h.UpdateTemplate)
	})
}
