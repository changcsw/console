package dto

import "time"

// ===== Commands（写用例输入）=====

// LoginCmd 密码登录命令。
type LoginCmd struct {
	UserName string
	Password string
}

// FeishuCallbackCmd 飞书回调登录命令。
type FeishuCallbackCmd struct {
	Code        string
	State       string
	RedirectURI string
}

// RefreshCmd 刷新令牌命令。
type RefreshCmd struct {
	RefreshToken string
}

// LogoutCmd 登出命令。
type LogoutCmd struct {
	AccessJTI    string
	RefreshToken string
}

// CreateUserCmd 新建管理员命令。
type CreateUserCmd struct {
	UserName    string
	DisplayName string
	Email       string
	Status      string
	Password    string
	RoleIDs     []int64
	FeishuKey   string
}

// UpdateUserCmd 更新管理员命令（指针表示可选，nil 不改）。
type UpdateUserCmd struct {
	ID          int64
	DisplayName *string
	Email       *string
	Status      *string
}

// AssignRolesCmd 全量覆盖用户角色。
type AssignRolesCmd struct {
	UserID  int64
	RoleIDs []int64
}

// ResetPasswordCmd 重置管理员密码。
type ResetPasswordCmd struct {
	UserID      int64
	NewPassword string
}

// CreateRoleCmd 新建角色。
type CreateRoleCmd struct {
	RoleCode      string
	RoleName      string
	PermissionIDs []int64
}

// UpdateRoleCmd 更新角色（role_code 不可改）。
type UpdateRoleCmd struct {
	ID       int64
	RoleName *string
}

// AssignPermissionsCmd 全量覆盖角色权限。
type AssignPermissionsCmd struct {
	RoleID        int64
	PermissionIDs []int64
}

// CreatePermissionCmd 新建权限码。
type CreatePermissionCmd struct {
	PermissionCode string
	PermissionName string
}

// ===== Views（响应输出，camelCase 由 transport JSON tag 决定）=====

// TokenPairView 登录/刷新返回的令牌信息。
type TokenPairView struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// LoginResult 登录/飞书回调返回（令牌 + 用户摘要）。
type LoginResult struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	User         UserView  `json:"user"`
}

// UserView 登录响应里的用户摘要。
type UserView struct {
	UserID      int64    `json:"userId"`
	UserName    string   `json:"userName"`
	DisplayName string   `json:"displayName"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// IdentityView 身份（identityKey 脱敏）。
type IdentityView struct {
	IdentityType string `json:"identityType"`
	IdentityKey  string `json:"identityKey"`
}

// MeView GET /me 响应。
type MeView struct {
	UserID      int64          `json:"userId"`
	UserName    string         `json:"userName"`
	DisplayName string         `json:"displayName"`
	Email       string         `json:"email"`
	Status      string         `json:"status"`
	Roles       []string       `json:"roles"`
	Permissions []string       `json:"permissions"`
	Identities  []IdentityView `json:"identities"`
	Environment string         `json:"environment"`
}

// RoleBrief 角色简要（用户详情内）。
type RoleBrief struct {
	ID       int64  `json:"id"`
	RoleCode string `json:"roleCode"`
	RoleName string `json:"roleName"`
}

// AdminUserListItem 管理员列表项。
type AdminUserListItem struct {
	ID          int64       `json:"id"`
	UserName    string      `json:"userName"`
	DisplayName string      `json:"displayName"`
	Email       string      `json:"email"`
	Status      string      `json:"status"`
	Roles       []RoleBrief `json:"roles"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

// AdminUserDetail 管理员详情。
type AdminUserDetail struct {
	ID          int64          `json:"id"`
	UserName    string         `json:"userName"`
	DisplayName string         `json:"displayName"`
	Email       string         `json:"email"`
	Status      string         `json:"status"`
	Roles       []RoleBrief    `json:"roles"`
	Identities  []IdentityView `json:"identities"`
	Permissions []string       `json:"permissions"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// PermissionBrief 权限简要。
type PermissionBrief struct {
	ID             int64  `json:"id"`
	PermissionCode string `json:"permissionCode"`
	PermissionName string `json:"permissionName"`
}

// RoleListItem 角色列表项。
type RoleListItem struct {
	ID              int64     `json:"id"`
	RoleCode        string    `json:"roleCode"`
	RoleName        string    `json:"roleName"`
	PermissionCount int       `json:"permissionCount"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// RoleDetail 角色详情（含 permissions）。
type RoleDetail struct {
	ID          int64             `json:"id"`
	RoleCode    string            `json:"roleCode"`
	RoleName    string            `json:"roleName"`
	Permissions []PermissionBrief `json:"permissions"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// PermissionView 权限码目录项。
type PermissionView struct {
	ID             int64     `json:"id"`
	PermissionCode string    `json:"permissionCode"`
	PermissionName string    `json:"permissionName"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Page 通用分页包装（items + 分页元信息，00 §7.2）。
type Page[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}
