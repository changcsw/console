package audit

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	auditapp "github.com/csw/console/services/admin-api/internal/app/audit"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

type Handler struct {
	svc auditapp.Service
}

func NewHandler(svc auditapp.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q, err := parseQuery(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, err.Error())
		return
	}
	page, err := h.svc.Query(r.Context(), q)
	if err != nil {
		writeError(w, err)
		return
	}

	items := make([]map[string]any, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, map[string]any{
			"id":           strconv.FormatInt(item.ID, 10),
			"actorId":      strconv.FormatInt(item.ActorID, 10),
			"operator":     toOperator(item),
			"action":       item.Action,
			"resourceType": item.ResourceType,
			"resourceId":   item.ResourceID,
			"env":          item.Env,
			"detail":       item.Detail,
			"createdAt":    item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{
		"items":    items,
		"page":     page.Page,
		"pageSize": page.PageSize,
		"total":    page.Total,
	})
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "id 非法")
		return
	}
	q := auditapp.AuditQuery{ID: &id, Page: 1, PageSize: 1, SortDesc: true}
	page, err := h.svc.Query(r.Context(), q)
	if err != nil {
		writeError(w, err)
		return
	}
	item := page.Items[0]
	httpx.WriteData(w, http.StatusOK, map[string]any{
		"id":           strconv.FormatInt(item.ID, 10),
		"actorId":      strconv.FormatInt(item.ActorID, 10),
		"operator":     toOperator(item),
		"action":       item.Action,
		"resourceType": item.ResourceType,
		"resourceId":   item.ResourceID,
		"env":          item.Env,
		"detail":       item.Detail,
		"createdAt":    item.CreatedAt.UTC().Format(time.RFC3339),
	})
}

// Facets 为可选过滤器候选接口（compact §6.3）。本期不提供动态候选，
// 前端使用静态字典并在此接口失败时回退；显式返回 NOT_FOUND(404) 表达「可选、未实现」，
// 优于落到 /{id} 路由把非数字段当 id 校验返回的 400。
func (h *Handler) Facets(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "facets 接口未实现")
}

func parseQuery(r *http.Request) (auditapp.AuditQuery, error) {
	query := r.URL.Query()
	var out auditapp.AuditQuery

	if v := strings.TrimSpace(query.Get("env")); v != "" {
		env := common.Environment(v)
		out.Env = &env
	}
	if v := strings.TrimSpace(query.Get("action")); v != "" {
		out.Action = &v
	}
	if v := strings.TrimSpace(query.Get("resourceType")); v != "" {
		out.ResourceType = &v
	}
	if v := strings.TrimSpace(query.Get("resourceId")); v != "" {
		out.ResourceID = &v
	}
	if v := strings.TrimSpace(query.Get("operatorKeyword")); v != "" {
		out.OperatorKeyword = &v
	}
	if v := strings.TrimSpace(query.Get("keyword")); v != "" {
		out.Keyword = &v
	}
	if v := strings.TrimSpace(query.Get("operator")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id < 0 {
			return auditapp.AuditQuery{}, errors.New("operator 非法")
		}
		out.Operator = &id
	}
	if v := strings.TrimSpace(query.Get("from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return auditapp.AuditQuery{}, errors.New("from 时间格式错误")
		}
		out.From = &t
	}
	if v := strings.TrimSpace(query.Get("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return auditapp.AuditQuery{}, errors.New("to 时间格式错误")
		}
		out.To = &t
	}
	if out.From != nil && out.To != nil && out.From.After(*out.To) {
		return auditapp.AuditQuery{}, errors.New("from 不能晚于 to")
	}

	out.Page = parsePositiveInt(query.Get("page"), 1)
	out.PageSize = parsePositiveInt(query.Get("pageSize"), 20)
	sort := strings.TrimSpace(query.Get("sort"))
	out.SortDesc = sort != "createdAt"
	return out, nil
}

func parsePositiveInt(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func toOperator(item auditapp.AuditLogItem) any {
	if item.ActorID == 0 {
		return map[string]any{"id": "0", "userName": "system", "displayName": "System"}
	}
	if item.Operator == nil {
		return nil
	}
	return map[string]any{
		"id":          strconv.FormatInt(item.Operator.ID, 10),
		"userName":    item.Operator.UserName,
		"displayName": item.Operator.DisplayName,
	}
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auditapp.ErrValidation):
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "入参校验失败")
	case errors.Is(err, auditapp.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeNotFound, "资源不存在")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "服务端内部错误")
	}
}
