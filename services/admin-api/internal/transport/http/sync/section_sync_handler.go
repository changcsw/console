package syncapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/command"
	domainsync "github.com/csw/console/services/admin-api/internal/domain/sync"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
	"github.com/go-chi/chi/v5"
)

type SectionSyncService interface {
	Preview(ctx context.Context, cmd command.PreviewSectionSyncCommand) (domainsync.Preview, error)
	Execute(ctx context.Context, cmd command.ExecuteSectionSyncCommand) (domainsync.ExecuteResult, error)
	ListJobs(ctx context.Context, query command.ListSectionSyncJobsQuery) (domainsync.JobList, error)
}

type SectionSyncHandler struct {
	service SectionSyncService
}

func NewSectionSyncHandler(service SectionSyncService) *SectionSyncHandler {
	return &SectionSyncHandler{service: service}
}

func (h *SectionSyncHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sections       []string `json:"sections"`
		IncludeDeletes bool     `json:"includeDeletes"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, command.CodeValidation, "请求体格式错误")
		return
	}

	cmd, err := command.NormalizePreviewSectionSync(command.PreviewSectionSyncCommand{
		GameID:           strings.TrimSpace(chi.URLParam(r, "gameId")),
		SelectedSections: req.Sections,
		IncludeDeletes:   req.IncludeDeletes,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	result, err := h.service.Preview(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}

	httpx.WriteData(w, http.StatusOK, result)
}

func (h *SectionSyncHandler) Execute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SelectedSections []string `json:"selectedSections"`
		BaselineToken    string   `json:"baselineToken"`
		IncludeDeletes   bool     `json:"includeDeletes"`
		OperatorNote     string   `json:"operatorNote"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, command.CodeValidation, "请求体格式错误")
		return
	}

	cmd, err := command.NormalizeExecuteSectionSync(command.ExecuteSectionSyncCommand{
		GameID:           strings.TrimSpace(chi.URLParam(r, "gameId")),
		SelectedSections: req.SelectedSections,
		BaselineToken:    req.BaselineToken,
		IncludeDeletes:   req.IncludeDeletes,
		OperatorNote:     req.OperatorNote,
	})
	if err != nil {
		writeError(w, err)
		return
	}

	result, err := h.service.Execute(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

func (h *SectionSyncHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	page := parsePositiveInt(r.URL.Query().Get("page"), 1)
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 20)
	out, err := h.service.ListJobs(r.Context(), command.ListSectionSyncJobsQuery{
		GameID:   strings.TrimSpace(chi.URLParam(r, "gameId")),
		Page:     page,
		PageSize: pageSize,
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, out)
}

func parsePositiveInt(raw string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return def
	}
	if v > 100 {
		return 100
	}
	return v
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *command.SectionSyncError
	if errors.As(err, &appErr) {
		httpx.WriteError(w, appErr.Status, appErr.Code, appErr.Message, appErr.Details...)
		return
	}
	httpx.WriteAppError(w, err)
}

func decodeJSONBody(r *http.Request, target any) error {
	if r.Body == nil {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(target); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
