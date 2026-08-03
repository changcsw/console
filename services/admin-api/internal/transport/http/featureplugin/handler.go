// Package featureplugin 是「功能插件管理」的 HTTP 传输层：
// handler + chi 路由注册 + 请求/响应 DTO（camelCase）+ 统一包络。
// 这些接口面向系统管理员维护插件分类字典 / 插件主数据 / 参数模板四件套，与游戏无关；
// 游戏侧在渠道实例上勾选并填参的接口在 http/channels（权限码 plugin.*）。
package featureplugin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	featurepluginapp "github.com/csw/console/services/admin-api/internal/app/featureplugin"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

// Handler 持有功能插件服务（后端未就绪时可为 nil，路由用 RequireBackend 拦截）。
type Handler struct {
	svc *featurepluginapp.Service
}

// NewHandler 构造 Handler。
func NewHandler(svc *featurepluginapp.Service) *Handler { return &Handler{svc: svc} }

// ===== 请求 DTO =====

type createCategoryRequest struct {
	CategoryCode string `json:"categoryCode"`
	CategoryName string `json:"categoryName"`
	Enabled      *bool  `json:"enabled"`
	Sort         *int   `json:"sort"`
}

type updateCategoryRequest struct {
	CategoryName *string `json:"categoryName"`
	Enabled      *bool   `json:"enabled"`
	Sort         *int    `json:"sort"`
}

type createPluginRequest struct {
	PluginID   string `json:"pluginId"`
	PluginName string `json:"pluginName"`
	CategoryID *int64 `json:"categoryId"`
	Region     string `json:"region"`
	Enabled    *bool  `json:"enabled"`
	Sort       *int   `json:"sort"`
}

// updatePluginRequest 的 categoryId 用 dto.NullableInt64：需要区分「未传」与「显式置空」。
type updatePluginRequest struct {
	PluginName *string           `json:"pluginName"`
	CategoryID dto.NullableInt64 `json:"categoryId"`
	Enabled    *bool             `json:"enabled"`
	Sort       *int              `json:"sort"`
}

type createTemplateRequest struct {
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

// ===== handlers：分类字典 =====

// ListCategories GET /feature-plugin-categories（feature_plugin.read）。
func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	enabled, err := parseOptionalBool(r.URL.Query().Get("enabled"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "enabled 非法",
			map[string]string{"field": "enabled", "reason": "bool"})
		return
	}
	items, err := h.svc.ListCategories(r.Context(), enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": items})
}

// CreateCategory POST /feature-plugin-categories（feature_plugin.write，审计 feature_plugin_category.create）。
func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req createCategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.CreateCategory(r.Context(), dto.CreateFeaturePluginCategoryCmd{
		CategoryCode: req.CategoryCode,
		CategoryName: req.CategoryName,
		Enabled:      req.Enabled,
		Sort:         req.Sort,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

// UpdateCategory PATCH /feature-plugin-categories/{id}（feature_plugin.write，审计 feature_plugin_category.update）。
func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var req updateCategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.UpdateCategory(r.Context(), dto.UpdateFeaturePluginCategoryCmd{
		CategoryID:   id,
		CategoryName: req.CategoryName,
		Enabled:      req.Enabled,
		Sort:         req.Sort,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// DeleteCategory DELETE /feature-plugin-categories/{id}（feature_plugin.write，审计 feature_plugin_category.delete）。
func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteCategory(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== handlers：插件主数据 =====

// ListPlugins GET /feature-plugins（feature_plugin.read）。
func (h *Handler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	enabled, err := parseOptionalBool(q.Get("enabled"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "enabled 非法",
			map[string]string{"field": "enabled", "reason": "bool"})
		return
	}
	categoryID, err := parseOptionalInt64(q.Get("categoryId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "categoryId 非法",
			map[string]string{"field": "categoryId", "reason": "int64"})
		return
	}
	result, err := h.svc.ListPlugins(r.Context(), dto.ListFeaturePluginsQuery{
		Keyword:    q.Get("keyword"),
		CategoryID: categoryID,
		Region:     q.Get("region"),
		Enabled:    enabled,
		Page:       atoiDefault(q.Get("page")),
		PageSize:   atoiDefault(q.Get("pageSize")),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// GetPlugin GET /feature-plugins/{pluginId}（feature_plugin.read）。
func (h *Handler) GetPlugin(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.GetPlugin(r.Context(), chi.URLParam(r, "pluginId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// CreatePlugin POST /feature-plugins（feature_plugin.write，审计 feature_plugin.create）。
func (h *Handler) CreatePlugin(w http.ResponseWriter, r *http.Request) {
	var req createPluginRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.CreatePlugin(r.Context(), dto.CreateFeaturePluginCmd{
		PluginID:   req.PluginID,
		PluginName: req.PluginName,
		CategoryID: req.CategoryID,
		Region:     req.Region,
		Enabled:    req.Enabled,
		Sort:       req.Sort,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

// UpdatePlugin PATCH /feature-plugins/{pluginId}（feature_plugin.write，审计 feature_plugin.update）。
func (h *Handler) UpdatePlugin(w http.ResponseWriter, r *http.Request) {
	var req updatePluginRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.UpdatePlugin(r.Context(), dto.UpdateFeaturePluginCmd{
		PluginID:   chi.URLParam(r, "pluginId"),
		PluginName: req.PluginName,
		CategoryID: req.CategoryID,
		Enabled:    req.Enabled,
		Sort:       req.Sort,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// DeletePlugin DELETE /feature-plugins/{pluginId}（feature_plugin.write，审计 feature_plugin.delete）。
func (h *Handler) DeletePlugin(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeletePlugin(r.Context(), chi.URLParam(r, "pluginId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ===== handlers：参数模板 =====

// ListTemplates GET /feature-plugins/{pluginId}/templates（feature_plugin.read）。
func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListTemplates(r.Context(), chi.URLParam(r, "pluginId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": items})
}

// CreateTemplate POST /feature-plugins/{pluginId}/templates（feature_plugin.write，审计 feature_plugin_template.create）。
func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req createTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.svc.CreateTemplate(r.Context(), dto.CreateFeaturePluginTemplateCmd{
		PluginID:        chi.URLParam(r, "pluginId"),
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

// GetTemplate GET /feature-plugin-templates/{templateId}（feature_plugin.read）。
func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "templateId")
	if !ok {
		return
	}
	result, err := h.svc.GetTemplate(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// UpdateTemplate PATCH /feature-plugin-templates/{templateId}（feature_plugin.write，审计 feature_plugin_template.update）。
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
	result, err := h.svc.UpdateTemplate(r.Context(), dto.UpdateFeaturePluginTemplateCmd{
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

// parseOptionalInt64 解析可选过滤参数；空串表示不过滤（返回 0）。
func parseOptionalInt64(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, errors.New("invalid int64")
	}
	return v, nil
}

// writeError 把 app 层 *featurepluginapp.Error 写为精确包络；其它回退 httpx.WriteAppError。
func writeError(w http.ResponseWriter, err error) {
	var appErr *featurepluginapp.Error
	if errors.As(err, &appErr) {
		httpx.WriteError(w, appErr.Status, appErr.Code, appErr.Message, appErr.Details...)
		return
	}
	httpx.WriteAppError(w, err)
}
