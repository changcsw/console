package admin

import (
	"context"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainadmin "github.com/csw/console/services/admin-api/internal/domain/admin"
)

// PermissionService 权限码目录的读/写用例。
type PermissionService struct {
	tx    TxManager
	audit AuditSink
}

// NewPermissionService 构造服务。
func NewPermissionService(tx TxManager, audit AuditSink) *PermissionService {
	return &PermissionService{tx: tx, audit: audit}
}

// ListPermissions 权限码目录分页（all=true 全量）。
func (s *PermissionService) ListPermissions(ctx context.Context, f domainadmin.PermissionFilter) (dto.Page[dto.PermissionView], error) {
	repos := s.tx.Repositories()
	perms, total, err := repos.Permissions.List(ctx, f)
	if err != nil {
		return dto.Page[dto.PermissionView]{}, err
	}
	page, pageSize := domainadmin.NormalizePage(f.Page, f.PageSize)
	items := make([]dto.PermissionView, 0, len(perms))
	for i := range perms {
		items = append(items, dto.PermissionView{
			ID:             perms[i].ID,
			PermissionCode: perms[i].PermissionCode,
			PermissionName: perms[i].PermissionName,
			CreatedAt:      perms[i].CreatedAt,
			UpdatedAt:      perms[i].UpdatedAt,
		})
	}
	if f.All {
		page, pageSize = 1, total
	}
	return dto.Page[dto.PermissionView]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// CreatePermission 新建权限码（格式 ^[a-z0-9_]+\.[a-z0-9_]+$）。
func (s *PermissionService) CreatePermission(ctx context.Context, cmd dto.CreatePermissionCmd) (dto.PermissionView, error) {
	code := strings.TrimSpace(cmd.PermissionCode)
	name := strings.TrimSpace(cmd.PermissionName)
	if name == "" || len(name) > 128 || len(code) > 128 {
		return dto.PermissionView{}, ErrValidation
	}
	if _, err := domainadmin.NewPermissionCode(code); err != nil {
		return dto.PermissionView{}, ErrValidation
	}

	repos := s.tx.Repositories()
	p := &domainadmin.Permission{PermissionCode: code, PermissionName: name}
	if err := repos.Permissions.Create(ctx, p); err != nil {
		return dto.PermissionView{}, err
	}
	s.writeAudit(ctx, "permission.create", p.ID, map[string]any{"permissionCode": code})
	return dto.PermissionView{
		ID:             p.ID,
		PermissionCode: p.PermissionCode,
		PermissionName: p.PermissionName,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}, nil
}

// DeletePermission 删除权限码；被角色引用时拒绝（CONFLICT），要求先解绑。
func (s *PermissionService) DeletePermission(ctx context.Context, id int64) error {
	repos := s.tx.Repositories()
	if _, err := repos.Permissions.FindByID(ctx, id); err != nil {
		return err
	}
	count, err := repos.Permissions.CountRoles(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrConflict
	}
	if err := repos.Permissions.Delete(ctx, id); err != nil {
		return err
	}
	s.writeAudit(ctx, "permission.delete", id, nil)
	return nil
}

func (s *PermissionService) writeAudit(ctx context.Context, action string, resourceID int64, detail map[string]any) {
	if s.audit == nil {
		return
	}
	s.audit.Write(ctx, AuditEntry{ActorID: actorFromCtx(ctx), Action: action, ResourceType: "permission", ResourceID: int64ToStr(resourceID), Detail: detail})
}
