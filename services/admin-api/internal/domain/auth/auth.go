// Package auth 持有令牌 / 身份相关的领域值对象与纯规则（无 IO）。
package auth

import (
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// Token 类型常量（JWT claim `typ`）。
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// SuperAdminRole 约定的全权角色 role_code（compact 关键假设）。
const SuperAdminRole = "super_admin"

// TokenPair 一次签发的访问/刷新令牌对。
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time // access 过期时间（ISO 透出）
}

// Claims JWT 载荷（access / refresh 共用，typ 区分）。
type Claims struct {
	Subject     string // sub = userID
	Type        string // access | refresh
	UserName    string
	DisplayName string
	Roles       []string
	Perms       []string
	Issuer      string
	IssuedAt    int64
	ExpiresAt   int64
	JTI         string
}

// AuthContext 请求期鉴权上下文（compact「权限解析到鉴权上下文」）。
type AuthContext struct {
	UserID      int64
	UserName    string
	DisplayName string
	Roles       []string
	perms       map[string]struct{}
	Environment common.Environment
}

// NewAuthContext 构造鉴权上下文，权限码转为集合。
func NewAuthContext(userID int64, userName, displayName string, roles, perms []string, env common.Environment) AuthContext {
	return AuthContext{
		UserID:      userID,
		UserName:    userName,
		DisplayName: displayName,
		Roles:       roles,
		perms:       admin.PermissionSet(perms),
		Environment: env,
	}
}

// IsSuperAdmin 是否具备约定全权角色（super_admin 中间件短路放行）。
func (c AuthContext) IsSuperAdmin() bool {
	for _, r := range c.Roles {
		if r == SuperAdminRole {
			return true
		}
	}
	return false
}

// HasPermission 判定是否拥有指定权限码；super_admin 直接放行。
func (c AuthContext) HasPermission(code string) bool {
	if code == "" {
		return true
	}
	if c.IsSuperAdmin() {
		return true
	}
	_, ok := c.perms[code]
	return ok
}

// HasAnyPermission 判定是否拥有任意一个给定权限码。
func (c AuthContext) HasAnyPermission(codes ...string) bool {
	for _, code := range codes {
		if c.HasPermission(code) {
			return true
		}
	}
	return false
}

// Permissions 返回权限码集合的切片副本（脱离内部 map）。
func (c AuthContext) Permissions() []string {
	out := make([]string, 0, len(c.perms))
	for code := range c.perms {
		out = append(out, code)
	}
	return out
}
