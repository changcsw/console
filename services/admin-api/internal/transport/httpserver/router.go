package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/csw/console/services/admin-api/internal/app/command"
	domaincashier "github.com/csw/console/services/admin-api/internal/domain/cashier"
	"github.com/csw/console/services/admin-api/internal/infra/config"
	cashierhttp "github.com/csw/console/services/admin-api/internal/transport/http/cashier"
)

type templateVersionScaffoldService struct{}

// New 装配顶层 chi 路由：healthz + auth 模块真实路由（DB/JWT 就绪时）+ 其余未迁移的 scaffold 路由（回退）。
func New(cfg config.Config) *http.Server {
	logger := slog.Default()
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"app":         cfg.AppName,
			"environment": cfg.Environment,
			"status":      "ok",
		})
	})

	legacy := buildLegacyMux(cfg)

	adminRouter := buildAdminRouter(cfg, logger)
	// auth 模块未匹配的 /api/admin/* 回退到尚未迁移的 scaffold 路由（games/channels/cashier/sync）
	adminRouter.NotFound(legacy.ServeHTTP)
	r.Mount("/api/admin", adminRouter)
	// 其余（如 /healthz）回退
	r.NotFound(legacy.ServeHTTP)

	return &http.Server{
		Addr:    cfg.HTTPAddress,
		Handler: r,
	}
}

// buildLegacyMux 保留尚未迁移到真实实现的 scaffold 路由（games/channels/cashier/sync）。
// 注意：这些路由暂未接入鉴权中间件，待各自模块迁移；auth 模块 /me 已由真实实现接管。
func buildLegacyMux(cfg config.Config) *http.ServeMux {
	mux := http.NewServeMux()
	templateVersionHandler := cashierhttp.NewTemplateVersionHandler(templateVersionScaffoldService{})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"app":         cfg.AppName,
			"environment": cfg.Environment,
			"status":      "ok",
		})
	})

	// 注意：/api/admin/games（含 /markets|/legal-links）已由 game 模块、/channels|/market-channels 与
	// /game-channels/*、/channel-packages/*、/sync/* 已由真实路由接管（见 admin_wiring.go）；
	// 此处仅保留未迁移子路径 scaffold。
	mux.HandleFunc("/api/admin/games/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		default:
			writeJSON(w, http.StatusNotImplemented, map[string]string{
				"message": "route scaffolded but not implemented",
				"path":    r.URL.Path,
			})
		}
	})

	mux.HandleFunc("/api/admin/cashier/templates/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/admin/cashier/templates/")
		switch {
		case strings.HasSuffix(path, "/copy-to-draft"):
			templateVersionHandler.CopyToDraft(w, r)
		default:
			writeJSON(w, http.StatusNotImplemented, map[string]string{
				"message": "route scaffolded but not implemented",
				"path":    r.URL.Path,
			})
		}
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (templateVersionScaffoldService) CopyToDraft(_ context.Context, cmd command.CopyTemplateVersionCommand) (domaincashier.TemplateVersion, error) {
	source := domaincashier.TemplateVersion{
		TemplateID: cmd.TemplateID,
		Version:    cmd.SourceVersion,
		Status:     domaincashier.StatusPublished,
	}

	return command.BuildDraftFromTemplateVersion(source, cmd.SourceVersion+1), nil
}
