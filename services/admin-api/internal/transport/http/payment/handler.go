package payment

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	paymentapp "github.com/csw/console/services/admin-api/internal/app/payment"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc paymentapp.PaymentRouteService
}

func NewHandler(svc paymentapp.PaymentRouteService) *Handler {
	return &Handler{svc: svc}
}

type createBillingSubjectRequest struct {
	SubjectID       string `json:"subjectId"`
	SubjectName     string `json:"subjectName"`
	LegalEntityName string `json:"legalEntityName"`
	Enabled         *bool  `json:"enabled"`
}

type createMerchantAccountRequest struct {
	MerchantAccountID string            `json:"merchantAccountId"`
	ProviderID        string            `json:"providerId"`
	SubjectID         string            `json:"subjectId"`
	MerchantID        string            `json:"merchantId"`
	MerchantName      string            `json:"merchantName"`
	ConfigJSON        map[string]any    `json:"configJson"`
	Secrets           map[string]string `json:"secrets"`
	Enabled           *bool             `json:"enabled"`
}

type putGameRoutesRequest struct {
	Items []struct {
		Package           *string `json:"packageCode"`
		Channel           *string `json:"channelId"`
		Market            *string `json:"marketCode"`
		Country           *string `json:"countryCode"`
		Currency          *string `json:"currency"`
		PayWayID          string  `json:"payWayId"`
		ProviderID        string  `json:"providerId"`
		MerchantAccountID string  `json:"merchantAccountId"`
		Priority          *int    `json:"priority"`
		Enabled           *bool   `json:"enabled"`
	} `json:"items"`
}

func (h *Handler) ListPayWays(w http.ResponseWriter, r *http.Request) {
	filter := parseListFilter(r)
	filter.Type = strings.TrimSpace(r.URL.Query().Get("type"))
	items, total, err := h.svc.ListPayWays(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, filter, total)
}

func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
	filter := parseListFilter(r)
	filter.Kind = strings.TrimSpace(r.URL.Query().Get("kind"))
	items, total, err := h.svc.ListProviders(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, filter, total)
}

func (h *Handler) ListBillingSubjects(w http.ResponseWriter, r *http.Request) {
	filter := parseListFilter(r)
	items, total, err := h.svc.ListBillingSubjects(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, filter, total)
}

func (h *Handler) GetProviderTemplate(w http.ResponseWriter, r *http.Request) {
	tpl, err := h.svc.GetProviderTemplate(r.Context(), chi.URLParam(r, "providerId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, tpl)
}

func (h *Handler) CreateBillingSubject(w http.ResponseWriter, r *http.Request) {
	var req createBillingSubjectRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "请求体格式错误")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	out, err := h.svc.CreateBillingSubject(r.Context(), paymentapp.CreateBillingSubjectCommand{
		SubjectID:       req.SubjectID,
		SubjectName:     req.SubjectName,
		LegalEntityName: req.LegalEntityName,
		Enabled:         enabled,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, out)
}

func (h *Handler) ListMerchantAccounts(w http.ResponseWriter, r *http.Request) {
	filter := parseListFilter(r)
	filter.ProviderID = strings.TrimSpace(r.URL.Query().Get("providerId"))
	filter.SubjectID = strings.TrimSpace(r.URL.Query().Get("subjectId"))
	items, total, err := h.svc.ListMerchantAccounts(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeList(w, items, filter, total)
}

func (h *Handler) CreateMerchantAccount(w http.ResponseWriter, r *http.Request) {
	var req createMerchantAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "请求体格式错误")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	out, err := h.svc.CreateMerchantAccount(r.Context(), paymentapp.CreateMerchantAccountCommand{
		MerchantAccountID: req.MerchantAccountID,
		ProviderID:        req.ProviderID,
		SubjectID:         req.SubjectID,
		MerchantID:        req.MerchantID,
		MerchantName:      req.MerchantName,
		ConfigJSON:        req.ConfigJSON,
		Secrets:           req.Secrets,
		Enabled:           enabled,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, out)
}

func (h *Handler) GetGameRoutes(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetGameRoutes(r.Context(), chi.URLParam(r, "gameId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, out)
}

func (h *Handler) PutGameRoutes(w http.ResponseWriter, r *http.Request) {
	var req putGameRoutesRequest
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "请求体格式错误")
		return
	}
	items := make([]paymentapp.SaveRouteItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, paymentapp.SaveRouteItem{
			Package:           item.Package,
			Channel:           item.Channel,
			Market:            item.Market,
			Country:           item.Country,
			Currency:          item.Currency,
			PayWayID:          item.PayWayID,
			ProviderID:        item.ProviderID,
			MerchantAccountID: item.MerchantAccountID,
			Priority:          item.Priority,
			Enabled:           item.Enabled,
		})
	}
	out, err := h.svc.SaveGameRoutes(r.Context(), chi.URLParam(r, "gameId"), paymentapp.SaveGameRoutesCommand{Items: items})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, out)
}

func writeList(w http.ResponseWriter, items any, filter paymentapp.ListFilter, total int) {
	httpx.WriteData(w, http.StatusOK, map[string]any{
		"items":    items,
		"page":     filter.Page,
		"pageSize": filter.PageSize,
		"total":    total,
	})
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *paymentapp.Error
	if errors.As(err, &appErr) {
		httpx.WriteError(w, appErr.Status, appErr.Code, appErr.Message, appErr.Details...)
		return
	}
	httpx.WriteAppError(w, err)
}

func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	return dec.Decode(target)
}

func parseListFilter(r *http.Request) paymentapp.ListFilter {
	page := 1
	pageSize := 20
	if v, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page"))); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("pageSize"))); err == nil && v > 0 {
		pageSize = v
	}
	if pageSize > 100 {
		pageSize = 100
	}
	filter := paymentapp.ListFilter{Page: page, PageSize: pageSize}
	if raw := strings.TrimSpace(r.URL.Query().Get("enabled")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			filter.Enabled = &parsed
		}
	}
	return filter
}
