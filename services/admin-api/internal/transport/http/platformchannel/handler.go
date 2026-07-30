// Package platformchannel 是「平台渠道主数据 + 渠道模版」的 HTTP 传输层：
// handler + chi 路由注册 + 请求/响应 DTO（camelCase）+ 统一包络。
// 这些接口面向系统管理员，与游戏无关；游戏侧引用模版填参的接口在 http/channels 与 http/games。
package platformchannel

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	platformchannelapp "github.com/csw/console/services/admin-api/internal/app/platformchannel"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

// Handler 持有平台渠道服务（后端未就绪时可为 nil，路由用 RequireBackend 拦截）。
type Handler struct {
	svc *platformchannelapp.Service
}

// NewHandler 构造 Handler。
func NewHandler(svc *platformchannelapp.Service) *Handler { return &Handler{svc: svc} }

// ===== 请求 DTO =====

type createChannelRequest struct {
	ChannelID     string `json:"channelId"`
	ChannelName   string `json:"channelName"`
	ChannelType   string `json:"channelType"`
	Region        string `json:"region"`
	Enabled       *bool  `json:"enabled"`
	Sort          *int   `json:"sort"`
	LoginMode     string `json:"loginMode"`
	PaymentMode   string `json:"paymentMode"`
	LoginLocked   *bool  `json:"loginLocked"`
	PaymentLocked *bool  `json:"paymentLocked"`
}

type updateChannelRequest struct {
	ChannelName   *string `json:"channelName"`
	Enabled       *bool   `json:"enabled"`
	Sort          *int    `json:"sort"`
	LoginMode     *string `json:"loginMode"`
	PaymentMode   *string `json:"paymentMode"`
	LoginLocked   *bool   `json:"loginLocked"`
	PaymentLocked *bool   `json:"paymentLocked"`
}

type createTemplateRequest struct {
	Kind             string                           `json:"kind"`
	TemplateVersion  string                           `json:"templateVersion"`
	FormSchemaJSON   []dto.TemplateFieldInput         `json:"formSchemaJson"`
	SecretFieldsJSON []string                         `json:"secretFieldsJson"`
	FileFieldsJSON   []dto.TemplateFileFieldInput     `json:"fileFieldsJson"`
	ValidationRules  map[string]dto.TemplateRuleInput `json:"validationRulesJson"`
	Enabled          *bool                            `json:"enabled"`
}

type updateTemplateRequest struct {
	FormSchemaJSON   []dto.TemplateFieldInput         `json:"formSchemaJson"`
	SecretFieldsJSON []string                         `json:"secretFieldsJson"`
	FileFieldsJSON   []dto.TemplateFileFieldInput     `json:"fileFieldsJson"`
	ValidationRules  map[string]dto.TemplateRuleInput `json:"validationRulesJson"`
	Enabled          *bool                            `json:"enabled"`
}

// ===== handlers =====

// ListChannels GET /platform/channels（platform_channel.read）。
func (h *Handler) ListChannels(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	enabled, err := parseOptionalBool(q.Get("enabled"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "enabled 非法",
			map[string]string{"field": "enabled", "reason": "bool"})
		return
	}
	result, err := h.svc.ListChannels(r.Context(), dto.ListPlatformChannelsQuery{
		Keyword:     q.Get("keyword"),
		Region:      q.Get("region"),
		ChannelType: q.Get("channelType"),
		Enabled:     enabled,
		Page:        atoiDefault(q.Get("page")),
		PageSize:    atoiDefault(q.Get("pageSize")),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// GetChannel GET /platform/channels/{channelId}（platform_channel.read）。
func (h *Handler) GetChannel(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.GetChannel(r.Context(), chi.URLParam(r, "channelId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// CreateChannel POST /platform/channels（platform_channel.write，审计 platform_channel.create）。
func (h *Handler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var req createChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.CreateChannel(r.Context(), dto.CreatePlatformChannelCmd{
		ChannelID:     req.ChannelID,
		ChannelName:   req.ChannelName,
		ChannelType:   req.ChannelType,
		Region:        req.Region,
		Enabled:       req.Enabled,
		Sort:          req.Sort,
		LoginMode:     req.LoginMode,
		PaymentMode:   req.PaymentMode,
		LoginLocked:   req.LoginLocked,
		PaymentLocked: req.PaymentLocked,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

// UpdateChannel PATCH /platform/channels/{channelId}（platform_channel.write，审计 platform_channel.update）。
func (h *Handler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	var req updateChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.UpdateChannel(r.Context(), dto.UpdatePlatformChannelCmd{
		ChannelID:     chi.URLParam(r, "channelId"),
		ChannelName:   req.ChannelName,
		Enabled:       req.Enabled,
		Sort:          req.Sort,
		LoginMode:     req.LoginMode,
		PaymentMode:   req.PaymentMode,
		LoginLocked:   req.LoginLocked,
		PaymentLocked: req.PaymentLocked,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// ListTemplates GET /platform/channels/{channelId}/templates（channel_template.read）。
func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListTemplates(r.Context(), chi.URLParam(r, "channelId"), r.URL.Query().Get("kind"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": items})
}

// CreateTemplate POST /platform/channels/{channelId}/templates（channel_template.write，审计 channel_template.create）。
func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req createTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.CreateTemplate(r.Context(), dto.CreateChannelTemplateCmd{
		ChannelID:       chi.URLParam(r, "channelId"),
		Kind:            req.Kind,
		TemplateVersion: req.TemplateVersion,
		FormSchema:      req.FormSchemaJSON,
		SecretFields:    req.SecretFieldsJSON,
		FileFields:      req.FileFieldsJSON,
		ValidationRules: req.ValidationRules,
		Enabled:         req.Enabled,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

// GetTemplate GET /platform/channel-templates/{kind}/{templateId}（channel_template.read）。
func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "templateId")
	if !ok {
		return
	}
	result, err := h.svc.GetTemplate(r.Context(), chi.URLParam(r, "kind"), id)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// UpdateTemplate PATCH /platform/channel-templates/{kind}/{templateId}（channel_template.write，审计 channel_template.update）。
func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "templateId")
	if !ok {
		return
	}
	var req updateTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.UpdateTemplate(r.Context(), dto.UpdateChannelTemplateCmd{
		Kind:            chi.URLParam(r, "kind"),
		TemplateID:      id,
		FormSchema:      req.FormSchemaJSON,
		SecretFields:    req.SecretFieldsJSON,
		FileFields:      req.FileFieldsJSON,
		ValidationRules: req.ValidationRules,
		Enabled:         req.Enabled,
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

func parseID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, name+" 非法",
			map[string]string{"field": name, "reason": "int64"})
		return 0, false
	}
	return id, true
}

func atoiDefault(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func parseOptionalBool(s string) (*bool, error) {
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
		return nil, errors.New("invalid bool")
	}
}

// writeError 把 app 层 *platformchannelapp.Error 写为精确包络；其它回退 httpx.WriteAppError。
func writeError(w http.ResponseWriter, err error) {
	var appErr *platformchannelapp.Error
	if errors.As(err, &appErr) {
		httpx.WriteError(w, appErr.Status, appErr.Code, appErr.Message, appErr.Details...)
		return
	}
	httpx.WriteAppError(w, err)
}
