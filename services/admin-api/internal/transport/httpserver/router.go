package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/command"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	appquery "github.com/csw/console/services/admin-api/internal/app/query"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/domain/game"
	"github.com/csw/console/services/admin-api/internal/domain/sync"
	domaincashier "github.com/csw/console/services/admin-api/internal/domain/cashier"
	"github.com/csw/console/services/admin-api/internal/infra/config"
	cashierhttp "github.com/csw/console/services/admin-api/internal/transport/http/cashier"
	channelshttp "github.com/csw/console/services/admin-api/internal/transport/http/channels"
	syncapi "github.com/csw/console/services/admin-api/internal/transport/http/sync"
)

type Server struct {
	mux *http.ServeMux
}

type marketChannelScaffoldService struct{}
type sectionSyncScaffoldService struct{}
type templateVersionScaffoldService struct{}

func New(cfg config.Config) *http.Server {
	server := &Server{mux: http.NewServeMux()}
	server.registerRoutes(cfg)
	return &http.Server{
		Addr:    cfg.HTTPAddress,
		Handler: server.mux,
	}
}

func (s *Server) registerRoutes(cfg config.Config) {
	marketChannelHandler := channelshttp.NewHandler(
		marketChannelScaffoldService{},
		marketChannelScaffoldService{},
		marketChannelScaffoldService{},
	)
	sectionSyncHandler := syncapi.NewSectionSyncHandler(sectionSyncScaffoldService{})
	templateVersionHandler := cashierhttp.NewTemplateVersionHandler(templateVersionScaffoldService{})

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
		case strings.HasSuffix(path, "/market-channels"):
			marketChannelHandler.ListMarketChannels(w, r)
		case strings.Contains(path, "/markets/") && strings.HasSuffix(path, "/channels"):
			marketChannelHandler.CreateMarketChannel(w, r)
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

	s.mux.HandleFunc("/api/admin/game-market-channels/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/admin/game-market-channels/")
		switch {
		case strings.HasSuffix(path, "/hide"):
			marketChannelHandler.HideMarketChannel(w, r)
		case strings.HasSuffix(path, "/unhide"):
			marketChannelHandler.UnhideMarketChannel(w, r)
		default:
			writeJSON(w, http.StatusNotImplemented, map[string]string{
				"message": "route scaffolded but not implemented",
				"path":    r.URL.Path,
			})
		}
	})

	s.mux.HandleFunc("/api/admin/cashier/templates/", func(w http.ResponseWriter, r *http.Request) {
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

func (templateVersionScaffoldService) CopyToDraft(_ context.Context, cmd command.CopyTemplateVersionCommand) (domaincashier.TemplateVersion, error) {
	source := domaincashier.TemplateVersion{
		TemplateID: cmd.TemplateID,
		Version:    cmd.SourceVersion,
		Status:     domaincashier.StatusPublished,
	}

	return command.BuildDraftFromTemplateVersion(source, cmd.SourceVersion+1), nil
}

func (marketChannelScaffoldService) Create(_ context.Context, cmd command.CreateMarketChannelCommand) (domainchannel.GameMarketChannel, error) {
	source := domainchannel.GameMarketChannel{}
	if strings.TrimSpace(cmd.CopyFromMarket) != "" {
		source = domainchannel.GameMarketChannel{
			GameID:       cmd.GameID,
			Market:       cmd.CopyFromMarket,
			ChannelID:    cmd.ChannelID,
			NormalConfig: map[string]any{"copiedFromMarket": cmd.CopyFromMarket},
		}
	}

	return command.BuildCreatedMarketChannel(command.CreateMarketChannelCommand{
		GameID:         cmd.GameID,
		Market:         cmd.Market,
		ChannelID:      cmd.ChannelID,
		Region:         cmd.Region,
		CopyFromMarket: cmd.CopyFromMarket,
		Source:         source,
	}), nil
}

func (marketChannelScaffoldService) List(_ context.Context, q appquery.ListMarketChannelsQuery) ([]dto.GameMarketChannelListItem, error) {
	items := []dto.GameMarketChannelListItem{
		{
			ID:                      domainchannel.BuildGameMarketChannelID(q.GameID, string(common.MarketGlobal), "google"),
			GameID:                  q.GameID,
			Market:                  string(common.MarketGlobal),
			ChannelID:               "google",
			ConfigStatus:            common.ConfigStatusValid,
			IncludedInSnapshot:      true,
			IncludedInSync:          true,
			IncludedInRuntimeConfig: true,
		},
		{
			ID:                      domainchannel.BuildGameMarketChannelID(q.GameID, string(common.MarketJP), "google"),
			GameID:                  q.GameID,
			Market:                  string(common.MarketJP),
			ChannelID:               "google",
			ConfigStatus:            common.ConfigStatusInvalid,
			IncludedInSnapshot:      true,
			IncludedInSync:          true,
			IncludedInRuntimeConfig: false,
		},
	}

	return appquery.FilterMarketChannels(q, items), nil
}

func (marketChannelScaffoldService) Hide(_ context.Context, cmd command.HideMarketChannelCommand) (domainchannel.GameMarketChannel, error) {
	item := scaffoldMarketChannelFromID(cmd.ID)
	command.ApplyHideMarketChannel(cmd, &item)
	return item, nil
}

func (marketChannelScaffoldService) Unhide(_ context.Context, cmd command.UnhideMarketChannelCommand) (domainchannel.GameMarketChannel, error) {
	item := scaffoldMarketChannelFromID(cmd.ID)
	item.Hidden = true
	item.HiddenBy = "admin"
	command.ApplyUnhideMarketChannel(&item)
	return item, nil
}

func scaffoldMarketChannelFromID(id string) domainchannel.GameMarketChannel {
	parts := strings.SplitN(id, ":", 3)
	item := domainchannel.GameMarketChannel{
		ID:           id,
		ConfigStatus: common.ConfigStatusValid,
	}

	if len(parts) != 3 {
		return item
	}

	item.GameID = parts[0]
	item.Market = parts[1]
	item.ChannelID = parts[2]
	return item
}
