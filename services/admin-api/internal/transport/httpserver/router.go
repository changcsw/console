package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/command"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/game"
	"github.com/csw/console/services/admin-api/internal/domain/sync"
	"github.com/csw/console/services/admin-api/internal/infra/config"
	syncapi "github.com/csw/console/services/admin-api/internal/transport/http/sync"
)

type Server struct {
	mux *http.ServeMux
}

type sectionSyncScaffoldService struct{}

func New(cfg config.Config) *http.Server {
	server := &Server{mux: http.NewServeMux()}
	server.registerRoutes(cfg)
	return &http.Server{
		Addr:    cfg.HTTPAddress,
		Handler: server.mux,
	}
}

func (s *Server) registerRoutes(cfg config.Config) {
	sectionSyncHandler := syncapi.NewSectionSyncHandler(sectionSyncScaffoldService{})

	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"app":         cfg.AppName,
			"environment": cfg.Environment,
			"status":      "ok",
		})
	})

	s.mux.HandleFunc("/api/admin/me", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"userId":      1,
			"displayName": "Admin",
			"roles":       []string{"admin"},
			"permissions": []string{"game.read", "game.write", "sync.execute"},
		})
	})

	s.mux.HandleFunc("/api/admin/games", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []game.Game{},
			})
		case http.MethodPost:
			var req dto.CreateGameRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{
				"message": "scaffold route only",
				"request": req,
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	s.mux.HandleFunc("/api/admin/games/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/admin/games/")
		switch {
		case strings.HasSuffix(path, "/sync/preview"):
			sectionSyncHandler.Preview(w, r)
		case strings.HasSuffix(path, "/sync/execute"):
			sectionSyncHandler.Execute(w, r)
		default:
			writeJSON(w, http.StatusNotImplemented, map[string]string{
				"message": "route scaffolded but not implemented",
				"path":    r.URL.Path,
			})
		}
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (sectionSyncScaffoldService) Preview(_ context.Context, cmd command.PreviewSectionSyncCommand) (sync.Preview, error) {
	sections, err := sync.ParseSections(cmd.SelectedSections, false)
	if err != nil {
		return sync.Preview{}, err
	}

	resultSections := make([]sync.DiffSection, 0, len(sections))
	for _, section := range sections {
		resultSections = append(resultSections, sync.DiffSection{
			Section: string(section),
			Changes: []sync.DiffChange{},
		})
	}

	return sync.Preview{
		GameID:           cmd.GameID,
		SourceEnv:        "sandbox",
		TargetEnv:        "production",
		SourceHash:       "pending",
		TargetHashBefore: "pending",
		HasDiff:          false,
		Sections:         resultSections,
	}, nil
}

func (sectionSyncScaffoldService) Execute(_ context.Context, cmd command.ExecuteSectionSyncCommand) error {
	_, err := sync.ParseSections(cmd.SelectedSections, true)
	return err
}
