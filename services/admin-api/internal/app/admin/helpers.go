package admin

import (
	"regexp"
	"strconv"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainadmin "github.com/csw/console/services/admin-api/internal/domain/admin"
)

// emailPattern 轻量邮箱格式校验（compact：email 非空时 format=email）。
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// isValidEmail 仅在非空时要求匹配邮箱格式；空字符串视为合法（email 可选，默认 ""）。
func isValidEmail(email string) bool {
	if email == "" {
		return true
	}
	return emailPattern.MatchString(email)
}

func parseInt64(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }

func int64ToStr(v int64) string { return strconv.FormatInt(v, 10) }

func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func roleCodes(roles []domainadmin.Role) []string {
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		out = append(out, r.RoleCode)
	}
	return out
}

func roleBriefs(roles []domainadmin.Role) []dto.RoleBrief {
	out := make([]dto.RoleBrief, 0, len(roles))
	for _, r := range roles {
		out = append(out, dto.RoleBrief{ID: r.ID, RoleCode: r.RoleCode, RoleName: r.RoleName})
	}
	return out
}

func permissionBriefs(perms []domainadmin.Permission) []dto.PermissionBrief {
	out := make([]dto.PermissionBrief, 0, len(perms))
	for _, p := range perms {
		out = append(out, dto.PermissionBrief{ID: p.ID, PermissionCode: p.PermissionCode, PermissionName: p.PermissionName})
	}
	return out
}

func maskIdentities(identities []domainadmin.AdminIdentity) []dto.IdentityView {
	out := make([]dto.IdentityView, 0, len(identities))
	for _, id := range identities {
		out = append(out, dto.IdentityView{
			IdentityType: string(id.IdentityType),
			IdentityKey:  domainadmin.MaskIdentityKey(id.IdentityKey),
		})
	}
	return out
}
