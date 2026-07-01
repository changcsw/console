package snapshot

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	snapshotapp "github.com/csw/console/services/admin-api/internal/app/snapshot"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc snapshotapp.Service
}

func NewHandler(svc snapshotapp.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Generate(r.Context(), chi.URLParam(r, "gameId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, out)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	filter := snapshotapp.ListFilter{
		Page:     parsePositiveInt(r.URL.Query().Get("page"), 1),
		PageSize: parsePositiveInt(r.URL.Query().Get("pageSize"), 20),
	}
	out, err := h.svc.List(r.Context(), chi.URLParam(r, "gameId"), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, out)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "snapshotId")), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, snapshotapp.CodeValidation, "snapshotId 非法")
		return
	}
	out, err := h.svc.Publish(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, out)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "snapshotId")), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, snapshotapp.CodeValidation, "snapshotId 非法")
		return
	}
	out, err := h.svc.Download(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+out.FileName+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out.Body)
}

func parsePositiveInt(raw string, def int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *snapshotapp.Error
	if errors.As(err, &appErr) {
		httpx.WriteError(w, appErr.Status, appErr.Code, appErr.Message, appErr.Details...)
		return
	}
	httpx.WriteAppError(w, err)
}
