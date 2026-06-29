// Package admin 持有后台管理员 / 角色 / 权限的领域实体、值对象与纯规则（无 IO）。
package admin

import (
	"fmt"
	"time"

	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// AdminUser 管理员聚合根（platform.admin_users）。
type AdminUser struct {
	ID          int64
	UserName    string
	DisplayName string
	Email       string
	Status      common.AdminUserStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Identities []AdminIdentity
	Roles      []Role
}

// AdminIdentity 管理员身份（platform.admin_identities），一用户多身份。
type AdminIdentity struct {
	ID                   int64
	UserIDRef            int64
	IdentityType         common.IdentityType
	IdentityKey          string
	CredentialCiphertext string // password=bcrypt 哈希，feishu=空或 AES-GCM 加密令牌
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Role 角色聚合根（platform.admin_roles）。
type Role struct {
	ID          int64
	RoleCode    string
	RoleName    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Permissions []Permission
}

// Permission 权限码字典项（platform.admin_permissions）。
type Permission struct {
	ID             int64
	PermissionCode string
	PermissionName string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IsActive 是否处于可登录状态。
func (u AdminUser) IsActive() bool {
	return u.Status == common.AdminUserStatusActive
}

// CanTransitionStatus 校验状态机流转：active <-> disabled（compact「管理员状态机」）。
// 同态视为幂等允许（true）。非法值返回 false。
func CanTransitionStatus(from, to common.AdminUserStatus) bool {
	if !IsValidStatus(from) || !IsValidStatus(to) {
		return false
	}
	return true
}

// ApplyStatus 应用状态变更；非法目标状态返回错误。
func (u *AdminUser) ApplyStatus(to common.AdminUserStatus) error {
	if !CanTransitionStatus(u.Status, to) {
		return fmt.Errorf("invalid admin status transition %q -> %q", u.Status, to)
	}
	u.Status = to
	return nil
}

// IsValidStatus 校验状态枚举合法性。
func IsValidStatus(s common.AdminUserStatus) bool {
	return s == common.AdminUserStatusActive || s == common.AdminUserStatusDisabled
}

// IsValidIdentityType 校验身份类型枚举合法性。
func IsValidIdentityType(t common.IdentityType) bool {
	return t == common.IdentityTypePassword || t == common.IdentityTypeFeishu
}

// MaskIdentityKey 对身份标识脱敏（compact：identityKey 脱敏，如 on_****1a2b）。
// 规则：保留前缀（到首个下划线含之，或前 2 位）与末尾 4 位，中间以 **** 替换。
func MaskIdentityKey(key string) string {
	if key == "" {
		return ""
	}
	runes := []rune(key)
	if len(runes) <= 4 {
		return "****"
	}

	prefixEnd := 0
	for i, r := range runes {
		if r == '_' {
			prefixEnd = i + 1
			break
		}
	}
	if prefixEnd == 0 {
		prefixEnd = 2
	}
	if prefixEnd > len(runes)-4 {
		prefixEnd = 0
	}

	tail := string(runes[len(runes)-4:])
	return string(runes[:prefixEnd]) + "****" + tail
}
