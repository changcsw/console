package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/command"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	appquery "github.com/csw/console/services/admin-api/internal/app/query"
	domainchannel "github.com/csw/console/services/admin-api/internal/domain/channel"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

type CreateMarketChannelService interface {
	Create(context.Context, command.CreateMarketChannelCommand) (domainchannel.GameMarketChannel, error)
}

type ListMarketChannelsService interface {
	List(context.Context, appquery.ListMarketChannelsQuery) ([]dto.GameMarketChannelListItem, error)
}

type MarketChannelVisibilityService interface {
	Hide(context.Context, command.HideMarketChannelCommand) (domainchannel.GameMarketChannel, error)
	Unhide(context.Context, command.UnhideMarketChannelCommand) (domainchannel.GameMarketChannel, error)
}

type Handler struct {
	createService     CreateMarketChannelService
	listService       ListMarketChannelsService
	visibilityService MarketChannelVisibilityService
}

type createMarketChannelRequest struct {
	ChannelID      string                      `json:"channelId"`
	Region         domainchannel.ChannelRegion `json:"region"`
	CopyFromMarket string                      `json:"copyFromMarket,omitempty"`
}

func NewHandler(
	createService CreateMarketChannelService,
	listService ListMarketChannelsService,
	visibilityService MarketChannelVisibilityService,
) *Handler {
	return &Handler{
		createService:     createService,
		listService:       listService,
		visibilityService: visibilityService,
	}
}

func (h *Handler) CreateMarketChannel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	gameID, market, err := parseCreatePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req createMarketChannelRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := domainchannel.ValidateMarketChannelCompatibility(common.Market(market), req.Region); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.createService.Create(r.Context(), command.CreateMarketChannelCommand{
		GameID:         gameID,
		Market:         market,
		ChannelID:      req.ChannelID,
		Region:         req.Region,
		CopyFromMarket: req.CopyFromMarket,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, toListItem(result))
}

func (h *Handler) ListMarketChannels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	gameID, err := parseListPath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	items, err := h.listService.List(r.Context(), appquery.ListMarketChannelsQuery{
		GameID: gameID,
		Market: r.URL.Query().Get("market"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) HideMarketChannel(w http.ResponseWriter, r *http.Request) {
	h.updateVisibility(w, r, true)
}

func (h *Handler) UnhideMarketChannel(w http.ResponseWriter, r *http.Request) {
	h.updateVisibility(w, r, false)
}

func (h *Handler) updateVisibility(w http.ResponseWriter, r *http.Request, hidden bool) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	id, action, err := parseVisibilityPath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if hidden && action != "hide" {
		http.Error(w, "unexpected visibility action", http.StatusBadRequest)
		return
	}

	if !hidden && action != "unhide" {
		http.Error(w, "unexpected visibility action", http.StatusBadRequest)
		return
	}

	var result domainchannel.GameMarketChannel
	if hidden {
		result, err = h.visibilityService.Hide(r.Context(), command.HideMarketChannelCommand{
			ID:       id,
			Operator: operatorFromRequest(r),
		})
	} else {
		result, err = h.visibilityService.Unhide(r.Context(), command.UnhideMarketChannelCommand{ID: id})
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, toListItem(result))
}

func parseCreatePath(path string) (string, string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 7 {
		return "", "", fmt.Errorf("unexpected path: %s", path)
	}

	if parts[0] != "api" || parts[1] != "admin" || parts[2] != "games" || parts[4] != "markets" || parts[6] != "channels" {
		return "", "", fmt.Errorf("unexpected path: %s", path)
	}

	return parts[3], parts[5], nil
}

func parseListPath(path string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 {
		return "", fmt.Errorf("unexpected path: %s", path)
	}

	if parts[0] != "api" || parts[1] != "admin" || parts[2] != "games" || parts[4] != "market-channels" {
		return "", fmt.Errorf("unexpected path: %s", path)
	}

	return parts[3], nil
}

func parseVisibilityPath(path string) (string, string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 {
		return "", "", fmt.Errorf("unexpected path: %s", path)
	}

	if parts[0] != "api" || parts[1] != "admin" || parts[2] != "game-market-channels" {
		return "", "", fmt.Errorf("unexpected path: %s", path)
	}

	return parts[3], parts[4], nil
}

func decodeJSONBody(r *http.Request, target any) error {
	if r.Body == nil {
		return nil
	}

	if err := json.NewDecoder(r.Body).Decode(target); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func operatorFromRequest(r *http.Request) string {
	operator := strings.TrimSpace(r.Header.Get("X-Operator"))
	if operator == "" {
		return "admin"
	}

	return operator
}

func toListItem(item domainchannel.GameMarketChannel) dto.GameMarketChannelListItem {
	return dto.GameMarketChannelListItem{
		ID:                      item.ID,
		GameID:                  item.GameID,
		Market:                  item.Market,
		ChannelID:               item.ChannelID,
		ConfigStatus:            item.ConfigStatus,
		Hidden:                  item.Hidden,
		IncludedInSnapshot:      !item.Hidden,
		IncludedInSync:          !item.Hidden,
		IncludedInRuntimeConfig: item.IncludedInRuntimeConfig(),
	}
}
