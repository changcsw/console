// Package games 是游戏主数据的 HTTP 传输层：handler + chi 路由注册 + 请求/响应 DTO（camelCase）+ 统一包络。
// 仅做 JSON↔DTO + 包络/错误码映射；编排与校验在 app 层（GameService）。
package games

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	accountauthapp "github.com/csw/console/services/admin-api/internal/app/accountauth"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	gameapp "github.com/csw/console/services/admin-api/internal/app/game"
	productapp "github.com/csw/console/services/admin-api/internal/app/product"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

// Handler 持有 GameService 与运行环境。
type Handler struct {
	svc            *gameapp.GameService
	accountAuthSvc *accountauthapp.Service
	productSvc     *productapp.ProductService
	iapSvc         *productapp.IAPConfigService
	env            common.Environment
}

// NewHandler 构造 Handler（svc 在后端未就绪时可为 nil，路由用 RequireBackend 拦截）。
func NewHandler(svc *gameapp.GameService, env common.Environment, accountAuthSvc ...*accountauthapp.Service) *Handler {
	var aas *accountauthapp.Service
	if len(accountAuthSvc) > 0 {
		aas = accountAuthSvc[0]
	}
	return &Handler{svc: svc, accountAuthSvc: aas, env: env}
}

// WithProductServices 注入 product 模块服务（模块 16）。
func (h *Handler) WithProductServices(productSvc *productapp.ProductService, iapSvc *productapp.IAPConfigService) *Handler {
	h.productSvc = productSvc
	h.iapSvc = iapSvc
	return h
}

// ===== 请求 DTO =====

type createGameRequest struct {
	Name              string   `json:"name"`
	Alias             string   `json:"alias"`
	IconURL           string   `json:"iconUrl"`
	DefaultMarketCode string   `json:"defaultMarketCode"`
	Status            string   `json:"status"`
	Markets           []string `json:"markets"`
}

type updateGameRequest struct {
	Name              *string `json:"name"`
	Alias             *string `json:"alias"`
	IconURL           *string `json:"iconUrl"`
	Status            *string `json:"status"`
	DefaultMarketCode *string `json:"defaultMarketCode"`
}

type marketItemRequest struct {
	MarketCode    string `json:"marketCode"`
	IsDefault     bool   `json:"isDefault"`
	Enabled       *bool  `json:"enabled"` // 缺省=true
	DefaultLocale string `json:"defaultLocale"`
}

type replaceMarketsRequest struct {
	Markets []marketItemRequest `json:"markets"`
}

type legalLinkItemRequest struct {
	ScopeType        string `json:"scopeType"`
	ScopeValue       string `json:"scopeValue"`
	TermsURL         string `json:"termsUrl"`
	PrivacyURL       string `json:"privacyUrl"`
	DeleteAccountURL string `json:"deleteAccountUrl"`
}

type replaceLegalLinksRequest struct {
	LegalLinks []legalLinkItemRequest `json:"legalLinks"`
}

type replaceAccountAuthConfigsRequest struct {
	Items []replaceAccountAuthConfigItem `json:"items"`
}

type replaceAccountAuthConfigItem struct {
	AuthTypeID string         `json:"authTypeId"`
	Enabled    *bool          `json:"enabled"`
	ConfigJSON map[string]any `json:"configJson"`
}

// ===== handlers =====

// ListGames GET /games（game.read）。
func (h *Handler) ListGames(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := parsePage(q)
	result, err := h.svc.ListGames(r.Context(), dto.ListGamesQuery{
		Keyword:    q.Get("keyword"),
		Status:     q.Get("status"),
		MarketCode: q.Get("marketCode"),
		Page:       page,
		PageSize:   pageSize,
		Sort:       q.Get("sort"),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// CreateGame POST /games（game.write，审计 game.create）。
func (h *Handler) CreateGame(w http.ResponseWriter, r *http.Request) {
	var req createGameRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.CreateGame(r.Context(), dto.CreateGameCmd{
		Name: req.Name, Alias: req.Alias, IconURL: req.IconURL,
		DefaultMarketCode: req.DefaultMarketCode, Status: req.Status, Markets: req.Markets,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

// GetGame GET /games/{gameId}（game.read）。
func (h *Handler) GetGame(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.GetGame(r.Context(), chi.URLParam(r, "gameId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// UpdateGame PATCH /games/{gameId}（game.write，审计 game.update）。
func (h *Handler) UpdateGame(w http.ResponseWriter, r *http.Request) {
	var req updateGameRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.UpdateGame(r.Context(), dto.UpdateGameCmd{
		GameID: chi.URLParam(r, "gameId"),
		Name:   req.Name, Alias: req.Alias, IconURL: req.IconURL,
		Status: req.Status, DefaultMarketCode: req.DefaultMarketCode,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// ReplaceMarkets PUT /games/{gameId}/markets（game.write，审计 game.markets.update）。
func (h *Handler) ReplaceMarkets(w http.ResponseWriter, r *http.Request) {
	var req replaceMarketsRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	if req.Markets == nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "markets 必填")
		return
	}
	markets := make([]dto.MarketInput, 0, len(req.Markets))
	for _, m := range req.Markets {
		enabled := true
		if m.Enabled != nil {
			enabled = *m.Enabled
		}
		markets = append(markets, dto.MarketInput{
			MarketCode: m.MarketCode, IsDefault: m.IsDefault, Enabled: enabled, DefaultLocale: m.DefaultLocale,
		})
	}
	result, err := h.svc.ReplaceMarkets(r.Context(), dto.ReplaceMarketsCmd{GameID: chi.URLParam(r, "gameId"), Markets: markets})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// ReplaceLegalLinks PUT /games/{gameId}/legal-links（game.write，审计 game.legal.update）。
func (h *Handler) ReplaceLegalLinks(w http.ResponseWriter, r *http.Request) {
	var req replaceLegalLinksRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	if req.LegalLinks == nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "legalLinks 必填")
		return
	}
	links := make([]dto.LegalLinkInput, 0, len(req.LegalLinks))
	for _, l := range req.LegalLinks {
		links = append(links, dto.LegalLinkInput{
			ScopeType: l.ScopeType, ScopeValue: l.ScopeValue,
			TermsURL: l.TermsURL, PrivacyURL: l.PrivacyURL, DeleteAccountURL: l.DeleteAccountURL,
		})
	}
	result, err := h.svc.ReplaceLegalLinks(r.Context(), dto.ReplaceLegalLinksCmd{GameID: chi.URLParam(r, "gameId"), LegalLinks: links})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// ListAccountAuthTypes GET /account-auth/types（game.read）。
func (h *Handler) ListAccountAuthTypes(w http.ResponseWriter, r *http.Request) {
	if h.accountAuthSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "account auth backend unavailable")
		return
	}
	result, err := h.accountAuthSvc.ListTypes(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": result})
}

// ListChannelAccountAuthTypes GET /channels/{channelId}/account-auth-types（game.read）。
func (h *Handler) ListChannelAccountAuthTypes(w http.ResponseWriter, r *http.Request) {
	if h.accountAuthSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "account auth backend unavailable")
		return
	}
	result, err := h.accountAuthSvc.ListChannelTypes(r.Context(), chi.URLParam(r, "channelId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": result})
}

// GetGameAccountAuthConfigs GET /games/{gameId}/account-auth-configs（game.read）。
func (h *Handler) GetGameAccountAuthConfigs(w http.ResponseWriter, r *http.Request) {
	if h.accountAuthSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "account auth backend unavailable")
		return
	}
	result, err := h.accountAuthSvc.GetGameConfigs(r.Context(), chi.URLParam(r, "gameId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": result})
}

// ReplaceGameAccountAuthConfigs PUT /games/{gameId}/account-auth-configs（game.write）。
func (h *Handler) ReplaceGameAccountAuthConfigs(w http.ResponseWriter, r *http.Request) {
	if h.accountAuthSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "account auth backend unavailable")
		return
	}
	var req replaceAccountAuthConfigsRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	if req.Items == nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "items 必填")
		return
	}
	items := make([]dto.ReplaceGameAccountAuthConfigItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, dto.ReplaceGameAccountAuthConfigItem{
			AuthTypeID: item.AuthTypeID,
			Enabled:    item.Enabled,
			ConfigJSON: item.ConfigJSON,
		})
	}
	result, err := h.accountAuthSvc.ReplaceGameConfigs(r.Context(), dto.ReplaceGameAccountAuthConfigsCmd{
		GameID: chi.URLParam(r, "gameId"),
		Items:  items,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": result})
}

// ===== helpers =====

func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(target); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func badRequest(w http.ResponseWriter) {
	httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "请求体格式错误")
}

func parsePage(q interface{ Get(string) string }) (int, int) {
	page := atoiDefault(q.Get("page"))
	pageSize := atoiDefault(q.Get("pageSize"))
	return page, pageSize
}

func atoiDefault(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// writeError 把 app 层 *gameapp.Error 写为精确包络；其它（仓储映射的哨兵）回退 httpx.WriteAppError。
func writeError(w http.ResponseWriter, err error) {
	var productErr *productapp.Error
	if errors.As(err, &productErr) {
		httpx.WriteError(w, productErr.Status, productErr.Code, productErr.Message, productErr.Details...)
		return
	}
	var aaErr *accountauthapp.Error
	if errors.As(err, &aaErr) {
		httpx.WriteError(w, aaErr.Status, aaErr.Code, aaErr.Message, aaErr.Details...)
		return
	}
	var appErr *gameapp.Error
	if errors.As(err, &appErr) {
		httpx.WriteError(w, appErr.Status, appErr.Code, appErr.Message, appErr.Details...)
		return
	}
	httpx.WriteAppError(w, err)
}
