// Package admin 是后台鉴权/RBAC 的 HTTP 传输层：handler + chi 路由 + 请求/响应 DTO（camelCase）+ 统一包络。
package admin

import (
	"encoding/json"
	"io"
	"net/http"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/transport/http/httpx"
)

// Handler 持有应用服务，处理 auth 与 system 端点。
type Handler struct {
	auth  *adminapp.AdminAuthService
	users *adminapp.AdminUserService
	roles *adminapp.RoleService
	perms *adminapp.PermissionService
	env   common.Environment
}

// Deps Handler 依赖。
type Deps struct {
	Auth  *adminapp.AdminAuthService
	Users *adminapp.AdminUserService
	Roles *adminapp.RoleService
	Perms *adminapp.PermissionService
	Env   common.Environment
}

// NewHandler 构造 Handler。
func NewHandler(d Deps) *Handler {
	return &Handler{auth: d.Auth, users: d.Users, roles: d.Roles, perms: d.Perms, env: d.Env}
}

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
