package admin

import "github.com/csw/console/services/admin-api/internal/domain/common"

// AdminUserFilter 管理员列表过滤/分页参数（compact GET /system/admin-users）。
type AdminUserFilter struct {
	Keyword  string                 // 匹配 user_name/display_name/email
	Status   common.AdminUserStatus // 空表示不过滤
	Page     int
	PageSize int
	Sort     string // 形如 -updatedAt
}

// RoleFilter 角色列表过滤/分页参数。
type RoleFilter struct {
	Keyword  string // 匹配 role_code/role_name
	Page     int
	PageSize int
	Sort     string
}

// PermissionFilter 权限码目录过滤/分页参数。
type PermissionFilter struct {
	Keyword  string
	All      bool // all=true 返回全量
	Page     int
	PageSize int
	Sort     string
}

// NormalizePage 归一化分页（00 §7.3：page>=1，pageSize 默认 20、最大 100）。
func NormalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
