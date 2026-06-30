// Package channels 是渠道与渠道实例的 HTTP 传输层：handler + chi 路由注册 + 请求/响应 DTO（camelCase）+ 统一包络。
// 仅做 JSON↔DTO + 包络/错误码映射；编排与校验在 app 层（ChannelService）。
package channels

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	channelapp "github.com/csw/console/services/admin-api/internal/app/channel"
	channelloginapp "github.com/csw/console/services/admin-api/internal/app/channellogin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

// Handler 持有 ChannelService 与运行环境。
type Handler struct {
	svc      *channelapp.ChannelService
	loginSvc *channelloginapp.Service
	env      common.Environment
}

// NewHandler 构造 Handler（svc 在后端未就绪时可为 nil，路由用 RequireBackend 拦截）。
func NewHandler(svc *channelapp.ChannelService, env common.Environment, loginSvc ...*channelloginapp.Service) *Handler {
	var ls *channelloginapp.Service
	if len(loginSvc) > 0 {
		ls = loginSvc[0]
	}
	return &Handler{svc: svc, loginSvc: ls, env: env}
}

// ===== 请求 DTO =====

type createMarketChannelRequest struct {
	ChannelID      string `json:"channelId"`
	Mode           string `json:"mode"`
	CopyFromMarket string `json:"copyFromMarket"`
	Enabled        *bool  `json:"enabled"`
	Remark         string `json:"remark"`
}

type updateMarketChannelRequest struct {
	Enabled *bool   `json:"enabled"`
	Remark  *string `json:"remark"`
}

type hideRequest struct {
	Reason string `json:"reason"`
}

type createPackageRequest struct {
	PackageCode          string `json:"packageCode"`
	PackageName          string `json:"packageName"`
	MarketCode           string `json:"marketCode"`
	BundleID             string `json:"bundleId"`
	InheritChannelConfig *bool  `json:"inheritChannelConfig"`
	Enabled              *bool  `json:"enabled"`
}

type updatePackageRequest struct {
	PackageName          *string        `json:"packageName"`
	BundleID             *string        `json:"bundleId"`
	InheritChannelConfig *bool          `json:"inheritChannelConfig"`
	Enabled              *bool          `json:"enabled"`
	OverrideJSON         map[string]any `json:"overrideJson"`
}

type upsertLoginConfigRequest struct {
	Enabled         *bool          `json:"enabled"`
	ConfigJSON      map[string]any `json:"configJson"`
	TemplateVersion string         `json:"templateVersion"`
}

// ===== handlers =====

// ListChannelOptions GET /games/{gameId}/channels（channel.read）。
func (h *Handler) ListChannelOptions(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListChannelOptions(r.Context(), chi.URLParam(r, "gameId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": items})
}

// ListMarketChannels GET /games/{gameId}/market-channels（channel.read）。
func (h *Handler) ListMarketChannels(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := parsePage(q)
	compatible, err := parseOptionalBoolParam("compatible", q.Get("compatible"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, err.Error(),
			map[string]string{"field": "compatible", "reason": "bool"})
		return
	}
	hidden, err := parseBoolParamWithDefault("hidden", q.Get("hidden"), false)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, err.Error(),
			map[string]string{"field": "hidden", "reason": "bool"})
		return
	}
	result, err := h.svc.ListMarketChannels(r.Context(), dto.ListMarketChannelsQuery{
		GameID:       chi.URLParam(r, "gameId"),
		Market:       q.Get("market"),
		ChannelID:    q.Get("channelId"),
		Compatible:   compatible,
		Hidden:       hidden,
		ConfigStatus: q.Get("configStatus"),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// CreateMarketChannel POST /games/{gameId}/markets/{market}/channels（channel.write，审计 channel.create）。
func (h *Handler) CreateMarketChannel(w http.ResponseWriter, r *http.Request) {
	var req createMarketChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.CreateMarketChannel(r.Context(), dto.CreateMarketChannelCmd{
		GameID:         chi.URLParam(r, "gameId"),
		Market:         chi.URLParam(r, "market"),
		ChannelID:      req.ChannelID,
		Mode:           req.Mode,
		CopyFromMarket: req.CopyFromMarket,
		Enabled:        req.Enabled,
		Remark:         req.Remark,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

// GetMarketChannel GET /game-channels/{gameChannelId}（channel.read）。
func (h *Handler) GetMarketChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "gameChannelId")
	if !ok {
		return
	}
	result, err := h.svc.GetMarketChannel(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// UpdateMarketChannel PATCH /game-channels/{gameChannelId}（channel.write，审计 channel.update）。
func (h *Handler) UpdateMarketChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "gameChannelId")
	if !ok {
		return
	}
	var req updateMarketChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.UpdateMarketChannel(r.Context(), dto.UpdateMarketChannelCmd{
		GameChannelID: id, Enabled: req.Enabled, Remark: req.Remark,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// HideMarketChannel POST /game-channels/{gameChannelId}/hide（channel.write，审计 channel.hide）。
func (h *Handler) HideMarketChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "gameChannelId")
	if !ok {
		return
	}
	var req hideRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.HideMarketChannel(r.Context(), dto.HideMarketChannelCmd{GameChannelID: id, Reason: req.Reason})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// UnhideMarketChannel POST /game-channels/{gameChannelId}/unhide（channel.write，审计 channel.unhide）。
func (h *Handler) UnhideMarketChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "gameChannelId")
	if !ok {
		return
	}
	result, err := h.svc.UnhideMarketChannel(r.Context(), dto.UnhideMarketChannelCmd{GameChannelID: id})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// ListPackages GET /game-channels/{gameChannelId}/packages（channel.read）。
func (h *Handler) ListPackages(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "gameChannelId")
	if !ok {
		return
	}
	items, err := h.svc.ListPackages(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": items})
}

// CreatePackage POST /game-channels/{gameChannelId}/packages（channel.write，审计 package.create）。
func (h *Handler) CreatePackage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "gameChannelId")
	if !ok {
		return
	}
	var req createPackageRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.CreatePackage(r.Context(), dto.CreatePackageCmd{
		GameChannelID:        id,
		PackageCode:          req.PackageCode,
		PackageName:          req.PackageName,
		MarketCode:           req.MarketCode,
		BundleID:             req.BundleID,
		InheritChannelConfig: req.InheritChannelConfig,
		Enabled:              req.Enabled,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

// UpdatePackage PATCH /channel-packages/{packageId}（channel.write，审计 package.update）。
func (h *Handler) UpdatePackage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "packageId")
	if !ok {
		return
	}
	var req updatePackageRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.UpdatePackage(r.Context(), dto.UpdatePackageCmd{
		PackageID:            id,
		PackageName:          req.PackageName,
		BundleID:             req.BundleID,
		InheritChannelConfig: req.InheritChannelConfig,
		Enabled:              req.Enabled,
		OverrideJSON:         req.OverrideJSON,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// GetLoginConfig GET /game-channels/{gameChannelId}/login-config（channel.read）。
func (h *Handler) GetLoginConfig(w http.ResponseWriter, r *http.Request) {
	if h.loginSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "channel login backend unavailable")
		return
	}
	id, ok := parseID(w, r, "gameChannelId")
	if !ok {
		return
	}
	result, err := h.loginSvc.GetLoginConfig(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// PutLoginConfig PUT /game-channels/{gameChannelId}/login-config（channel.write）。
func (h *Handler) PutLoginConfig(w http.ResponseWriter, r *http.Request) {
	if h.loginSvc == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeInternal, "channel login backend unavailable")
		return
	}
	id, ok := parseID(w, r, "gameChannelId")
	if !ok {
		return
	}
	var req upsertLoginConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	if req.ConfigJSON == nil {
		req.ConfigJSON = map[string]any{}
	}
	result, err := h.loginSvc.UpsertLoginConfig(r.Context(), dto.UpsertChannelLoginConfigCmd{
		GameChannelID:   id,
		Enabled:         req.Enabled,
		ConfigJSON:      req.ConfigJSON,
		TemplateVersion: req.TemplateVersion,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
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

// parseID 解析 int64 路径参数（非法 → 400 VALIDATION_FAILED）。
func parseID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, name+" 非法",
			map[string]string{"field": name, "reason": "int64"})
		return 0, false
	}
	return id, true
}

func parsePage(q interface{ Get(string) string }) (int, int) {
	return atoiDefault(q.Get("page")), atoiDefault(q.Get("pageSize"))
}

func atoiDefault(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func parseOptionalBoolParam(name, s string) (*bool, error) {
	switch s {
	case "":
		return nil, nil
	case "true", "1":
		v := true
		return &v, nil
	case "false", "0":
		v := false
		return &v, nil
	default:
		return nil, errors.New(name + " 非法")
	}
}

func parseBoolParamWithDefault(name, s string, def bool) (bool, error) {
	if s == "" {
		return def, nil
	}
	v, err := parseOptionalBoolParam(name, s)
	if err != nil {
		return def, err
	}
	return *v, nil
}

// writeError 把 app 层 *channelapp.Error 写为精确包络；其它（仓储映射的哨兵）回退 httpx.WriteAppError。
func writeError(w http.ResponseWriter, err error) {
	var loginErr *channelloginapp.Error
	if errors.As(err, &loginErr) {
		httpx.WriteError(w, loginErr.Status, loginErr.Code, loginErr.Message, loginErr.Details...)
		return
	}
	var appErr *channelapp.Error
	if errors.As(err, &appErr) {
		httpx.WriteError(w, appErr.Status, appErr.Code, appErr.Message, appErr.Details...)
		return
	}
	httpx.WriteAppError(w, err)
}
