package admin

import (
	"context"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainadmin "github.com/csw/console/services/admin-api/internal/domain/admin"
)

// RoleService 角色的读/写用例。
type RoleService struct {
	tx    TxManager
	audit AuditSink
}

// NewRoleService 构造服务。
func NewRoleService(tx TxManager, audit AuditSink) *RoleService {
	return &RoleService{tx: tx, audit: audit}
}

// ===== query =====

// ListRoles 角色分页列表（含 permissionCount）。
func (s *RoleService) ListRoles(ctx context.Context, f domainadmin.RoleFilter) (dto.Page[dto.RoleListItem], error) {
	repos := s.tx.Repositories()
	roles, total, err := repos.Roles.List(ctx, f)
	if err != nil {
		return dto.Page[dto.RoleListItem]{}, err
	}
	page, pageSize := domainadmin.NormalizePage(f.Page, f.PageSize)
	items := make([]dto.RoleListItem, 0, len(roles))
	for i := range roles {
		perms, err := repos.Roles.PermissionsByRole(ctx, roles[i].ID)
		if err != nil {
			return dto.Page[dto.RoleListItem]{}, err
		}
		items = append(items, dto.RoleListItem{
			ID:              roles[i].ID,
			RoleCode:        roles[i].RoleCode,
			RoleName:        roles[i].RoleName,
			PermissionCount: len(perms),
			CreatedAt:       roles[i].CreatedAt,
			UpdatedAt:       roles[i].UpdatedAt,
		})
	}
	return dto.Page[dto.RoleListItem]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// GetRole 角色详情。
func (s *RoleService) GetRole(ctx context.Context, id int64) (dto.RoleDetail, error) {
	repos := s.tx.Repositories()
	role, err := repos.Roles.FindByID(ctx, id)
	if err != nil {
		return dto.RoleDetail{}, err
	}
	perms, err := repos.Roles.PermissionsByRole(ctx, id)
	if err != nil {
		return dto.RoleDetail{}, err
	}
	return dto.RoleDetail{
		ID:          role.ID,
		RoleCode:    role.RoleCode,
		RoleName:    role.RoleName,
		Permissions: permissionBriefs(perms),
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	}, nil
}

// ===== command =====

// CreateRole 新建角色（+ 可选权限）。
func (s *RoleService) CreateRole(ctx context.Context, cmd dto.CreateRoleCmd) (dto.RoleDetail, error) {
	code := strings.TrimSpace(cmd.RoleCode)
	name := strings.TrimSpace(cmd.RoleName)
	if code == "" || len(code) > 64 || name == "" || len(name) > 128 {
		return dto.RoleDetail{}, ErrValidation
	}

	var newID int64
	err := s.tx.InTx(ctx, func(repos Repositories) error {
		role := &domainadmin.Role{RoleCode: code, RoleName: name}
		if err := repos.Roles.Create(ctx, role); err != nil {
			return err
		}
		newID = role.ID
		if len(cmd.PermissionIDs) > 0 {
			if err := ensurePermsExist(ctx, repos, cmd.PermissionIDs); err != nil {
				return err
			}
			if err := repos.Roles.ReplacePermissions(ctx, role.ID, cmd.PermissionIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return dto.RoleDetail{}, err
	}
	s.writeAudit(ctx, "role.create", newID, map[string]any{"roleCode": code})
	return s.GetRole(ctx, newID)
}

// UpdateRole 更新角色名（role_code 不可改）。
func (s *RoleService) UpdateRole(ctx context.Context, cmd dto.UpdateRoleCmd) (dto.RoleDetail, error) {
	repos := s.tx.Repositories()
	role, err := repos.Roles.FindByID(ctx, cmd.ID)
	if err != nil {
		return dto.RoleDetail{}, err
	}
	if cmd.RoleName != nil {
		name := strings.TrimSpace(*cmd.RoleName)
		if name == "" || len(name) > 128 {
			return dto.RoleDetail{}, ErrValidation
		}
		role.RoleName = name
	}
	if err := repos.Roles.Update(ctx, role); err != nil {
		return dto.RoleDetail{}, err
	}
	s.writeAudit(ctx, "role.update", role.ID, map[string]any{"roleName": role.RoleName})
	return s.GetRole(ctx, role.ID)
}

// DeleteRole 删除角色；被用户引用时拒绝（CONFLICT），要求先解绑。
func (s *RoleService) DeleteRole(ctx context.Context, id int64) error {
	repos := s.tx.Repositories()
	if _, err := repos.Roles.FindByID(ctx, id); err != nil {
		return err
	}
	count, err := repos.Roles.CountUsers(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrConflict
	}
	if err := repos.Roles.Delete(ctx, id); err != nil {
		return err
	}
	s.writeAudit(ctx, "role.delete", id, nil)
	return nil
}

// AssignPermissions 全量覆盖角色权限（生效需令牌刷新）。
func (s *RoleService) AssignPermissions(ctx context.Context, cmd dto.AssignPermissionsCmd) (dto.RoleDetail, error) {
	err := s.tx.InTx(ctx, func(repos Repositories) error {
		if _, err := repos.Roles.FindByID(ctx, cmd.RoleID); err != nil {
			return err
		}
		if err := ensurePermsExist(ctx, repos, cmd.PermissionIDs); err != nil {
			return err
		}
		return repos.Roles.ReplacePermissions(ctx, cmd.RoleID, cmd.PermissionIDs)
	})
	if err != nil {
		return dto.RoleDetail{}, err
	}
	s.writeAudit(ctx, "role.assign_permissions", cmd.RoleID, map[string]any{"permissionIds": cmd.PermissionIDs})
	return s.GetRole(ctx, cmd.RoleID)
}

func (s *RoleService) writeAudit(ctx context.Context, action string, resourceID int64, detail map[string]any) {
	if s.audit == nil {
		return
	}
	s.audit.Write(ctx, AuditEntry{ActorID: actorFromCtx(ctx), Action: action, ResourceType: "role", ResourceID: int64ToStr(resourceID), Detail: detail})
}

func ensurePermsExist(ctx context.Context, repos Repositories, permIDs []int64) error {
	if len(permIDs) == 0 {
		return nil
	}
	found, err := repos.Permissions.FindByIDs(ctx, permIDs)
	if err != nil {
		return err
	}
	if len(found) != len(uniqueInt64(permIDs)) {
		return ErrValidation
	}
	return nil
}

func uniqueInt64(in []int64) []int64 {
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
