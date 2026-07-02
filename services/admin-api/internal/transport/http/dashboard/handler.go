package dashboard

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	dashboardquery "github.com/csw/console/services/admin-api/internal/app/query/dashboard"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

type SummaryService interface {
	Summary(ctx context.Context, params dto.DashboardSummaryParams) (dto.DashboardSummary, error)
}

type Handler struct {
	service dashboardquery.SummaryService
}

func NewHandler(service dashboardquery.SummaryService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	params, err := parseSummaryParams(r)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.service.Summary(r.Context(), params)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, out)
}

func parseSummaryParams(r *http.Request) (dto.DashboardSummaryParams, error) {
	q := r.URL.Query()
	out := dto.DashboardSummaryParams{
		Range: strings.TrimSpace(q.Get("range")),
		TopN:  5,
	}
	withTopRaw := strings.TrimSpace(q.Get("withTopItems"))
	if withTopRaw != "" {
		value, err := strconv.ParseBool(withTopRaw)
		if err != nil {
			return dto.DashboardSummaryParams{}, &dashboardquery.Error{
				Status:  http.StatusBadRequest,
				Code:    dashboardquery.CodeValidation,
				Message: "withTopItems 必须为布尔值",
				Details: []any{},
			}
		}
		out.WithTopItems = value
	}
	topNRaw := strings.TrimSpace(q.Get("topN"))
	if topNRaw != "" {
		topN, err := strconv.Atoi(topNRaw)
		if err != nil {
			return dto.DashboardSummaryParams{}, &dashboardquery.Error{
				Status:  http.StatusBadRequest,
				Code:    dashboardquery.CodeValidation,
				Message: "topN 必须为整数",
				Details: []any{},
			}
		}
		if topN < 1 || topN > 20 {
			return dto.DashboardSummaryParams{}, &dashboardquery.Error{
				Status:  http.StatusBadRequest,
				Code:    dashboardquery.CodeValidation,
				Message: "topN 超出范围，允许 1..20",
				Details: []any{},
			}
		}
		out.TopN = topN
	}
	return out, nil
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *dashboardquery.Error
	if errors.As(err, &appErr) {
		httpx.WriteError(w, appErr.Status, appErr.Code, appErr.Message, appErr.Details...)
		return
	}
	httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "服务端内部错误")
}
