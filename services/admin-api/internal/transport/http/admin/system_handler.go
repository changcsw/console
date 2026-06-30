package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainadmin "github.com/csw/console/services/admin-api/internal/domain/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

// ===== 请求 DTO =====

type createUserRequest struct {
	UserName    string  `json:"userName"`
	DisplayName string  `json:"displayName"`
	Email       string  `json:"email"`
	Status      string  `json:"status"`
	Password    string  `json:"password"`
	RoleIDs     []int64 `json:"roleIds"`
	FeishuKey   string  `json:"feishuKey"`
}

type updateUserRequest struct {
	DisplayName *string `json:"displayName"`
	Email       *string `json:"email"`
	Status      *string `json:"status"`
}

type assignRolesRequest struct {
	RoleIDs []int64 `json:"roleIds"`
}

type resetPasswordRequest struct {
	NewPassword string `json:"newPassword"`
}

type createRoleRequest struct {
	RoleCode      string  `json:"roleCode"`
	RoleName      string  `json:"roleName"`
	PermissionIDs []int64 `json:"permissionIds"`
}

type updateRoleRequest struct {
	RoleName *string `json:"roleName"`
}

type assignPermissionsRequest struct {
	PermissionIDs []int64 `json:"permissionIds"`
}

type createPermissionRequest struct {
	PermissionCode string `json:"permissionCode"`
	PermissionName string `json:"permissionName"`
}

// ===== admin-users =====

// ListUsers GET /system/admin-users（system.read）。
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := parsePage(q)
	status := common.AdminUserStatus(q.Get("status"))
	if status != "" && !domainadmin.IsValidStatus(status) {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "status 必须为 active 或 disabled")
		return
	}
	filter := domainadmin.AdminUserFilter{
		Keyword:  q.Get("keyword"),
		Status:   status,
		Page:     page,
		PageSize: pageSize,
		Sort:     q.Get("sort"),
	}
	result, err := h.users.ListUsers(r.Context(), filter)
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// CreateUser POST /system/admin-users（admin_user.write）。
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.users.CreateUser(r.Context(), dto.CreateUserCmd{
		UserName: req.UserName, DisplayName: req.DisplayName, Email: req.Email,
		Status: req.Status, Password: req.Password, RoleIDs: req.RoleIDs, FeishuKey: req.FeishuKey,
	})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

// GetUser GET /system/admin-users/{id}（system.read）。
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	result, err := h.users.GetUser(r.Context(), id)
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// UpdateUser PATCH /system/admin-users/{id}（admin_user.write）。
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req updateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.users.UpdateUser(r.Context(), dto.UpdateUserCmd{
		ID: id, DisplayName: req.DisplayName, Email: req.Email, Status: req.Status,
	})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// AssignUserRoles PUT /system/admin-users/{id}/roles（admin_user.write）。
func (h *Handler) AssignUserRoles(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req assignRolesRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	if req.RoleIDs == nil { // compact：roleIds 必填（缺字段→校验失败；空数组=全量解绑，合法）
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "roleIds 必填")
		return
	}
	result, err := h.users.AssignRoles(r.Context(), dto.AssignRolesCmd{UserID: id, RoleIDs: req.RoleIDs})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"id": result.ID, "roles": result.Roles})
}

// ResetPassword POST /system/admin-users/{id}/reset-password（admin_user.write）。
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req resetPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	if err := h.users.ResetPassword(r.Context(), dto.ResetPasswordCmd{UserID: id, NewPassword: req.NewPassword}); err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"id": id, "reset": true})
}

// ===== roles =====

// ListRoles GET /system/roles（system.read）。
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := parsePage(q)
	result, err := h.roles.ListRoles(r.Context(), domainadmin.RoleFilter{
		Keyword: q.Get("keyword"), Page: page, PageSize: pageSize, Sort: q.Get("sort"),
	})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// CreateRole POST /system/roles（role.write）。
func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req createRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.roles.CreateRole(r.Context(), dto.CreateRoleCmd{
		RoleCode: req.RoleCode, RoleName: req.RoleName, PermissionIDs: req.PermissionIDs,
	})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

// UpdateRole PATCH /system/roles/{id}（role.write）。
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req updateRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.roles.UpdateRole(r.Context(), dto.UpdateRoleCmd{ID: id, RoleName: req.RoleName})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// DeleteRole DELETE /system/roles/{id}（role.write）。
func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.roles.DeleteRole(r.Context(), id); err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

// AssignRolePermissions PUT /system/roles/{id}/permissions（role.write）。
func (h *Handler) AssignRolePermissions(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req assignPermissionsRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	if req.PermissionIDs == nil { // compact：permissionIds 必填（缺字段→校验失败；空数组=全量清空，合法）
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "permissionIds 必填")
		return
	}
	result, err := h.roles.AssignPermissions(r.Context(), dto.AssignPermissionsCmd{RoleID: id, PermissionIDs: req.PermissionIDs})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"id": result.ID, "permissions": result.Permissions})
}

// ===== permissions =====

// ListPermissions GET /system/permissions（system.read）。
func (h *Handler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := parsePage(q)
	all, _ := strconv.ParseBool(q.Get("all"))
	result, err := h.perms.ListPermissions(r.Context(), domainadmin.PermissionFilter{
		Keyword: q.Get("keyword"), All: all, Page: page, PageSize: pageSize, Sort: q.Get("sort"),
	})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, result)
}

// CreatePermission POST /system/permissions（permission.write）。
func (h *Handler) CreatePermission(w http.ResponseWriter, r *http.Request) {
	var req createPermissionRequest
	if err := decodeJSON(r, &req); err != nil {
		badRequest(w)
		return
	}
	result, err := h.perms.CreatePermission(r.Context(), dto.CreatePermissionCmd{
		PermissionCode: req.PermissionCode, PermissionName: req.PermissionName,
	})
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusCreated, result)
}

// DeletePermission DELETE /system/permissions/{id}（permission.write）。
func (h *Handler) DeletePermission(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.perms.DeletePermission(r.Context(), id); err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

// ===== currency-specs（平台级只读字典） =====

// ListCurrencySpecs GET /system/currency-specs（登录态即可读公共字典）。
// 返回统一信封 {data:{items:[CurrencySpecView]}}，供前端 dictionary store 解包。
func (h *Handler) ListCurrencySpecs(w http.ResponseWriter, r *http.Request) {
	items, err := h.currency.ListCurrencySpecs(r.Context())
	if err != nil {
		httpx.WriteAppError(w, err)
		return
	}
	httpx.WriteData(w, http.StatusOK, map[string]any{"items": items})
}

// ===== helpers =====

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "非法 id")
		return 0, false
	}
	return id, true
}

func parsePage(q interface{ Get(string) string }) (int, int) {
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	return page, pageSize
}
