package syncapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/command"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"
)

type SectionSyncService interface {
	Preview(context.Context, command.PreviewSectionSyncCommand) (domainsync.Preview, error)
	Execute(context.Context, command.ExecuteSectionSyncCommand) error
}

type SectionSyncHandler struct {
	service SectionSyncService
}

func NewSectionSyncHandler(service SectionSyncService) *SectionSyncHandler {
	return &SectionSyncHandler{service: service}
}

func (h *SectionSyncHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	gameID, err := parseSectionSyncPath(r.URL.Path, "preview")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req dto.SyncPreviewRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd, err := command.NormalizePreviewSectionSync(command.PreviewSectionSyncCommand{
		GameID:           gameID,
		SelectedSections: req.SelectedSections,
		IncludeDeletes:   req.IncludeDeletes,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.service.Preview(r.Context(), cmd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *SectionSyncHandler) Execute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	gameID, err := parseSectionSyncPath(r.URL.Path, "execute")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req dto.SyncExecuteRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd, err := command.NormalizeExecuteSectionSync(command.ExecuteSectionSyncCommand{
		GameID:           gameID,
		SelectedSections: req.SelectedSections,
		IncludeDeletes:   req.IncludeDeletes,
		OperatorNote:     req.OperatorNote,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.Execute(r.Context(), cmd); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"gameId":            cmd.GameID,
		"selected_sections": cmd.SelectedSections,
		"status":            "accepted",
	})
}

func parseSectionSyncPath(path, action string) (string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 6 {
		return "", fmt.Errorf("unexpected path: %s", path)
	}

	if parts[0] != "api" || parts[1] != "admin" || parts[2] != "games" || parts[4] != "sync" || parts[5] != action {
		return "", fmt.Errorf("unexpected path: %s", path)
	}

	return parts[3], nil
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
