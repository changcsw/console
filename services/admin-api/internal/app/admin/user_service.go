package admin

import (
	"context"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainadmin "github.com/csw/console/services/admin-api/internal/domain/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// AdminUserService 管理员的读/写用例。
type AdminUserService struct {
	tx     TxManager
	hasher PasswordHasher
	audit  AuditSink
}

// NewAdminUserService 构造服务。
func NewAdminUserService(tx TxManager, hasher PasswordHasher, audit AuditSink) *AdminUserService {
	return &AdminUserService{tx: tx, hasher: hasher, audit: audit}
}

// ===== query =====

// ListUsers 管理员分页列表。
func (s *AdminUserService) ListUsers(ctx context.Context, f domainadmin.AdminUserFilter) (dto.Page[dto.AdminUserListItem], error) {
	repos := s.tx.Repositories()
	users, total, err := repos.Users.List(ctx, f)
	if err != nil {
		return dto.Page[dto.AdminUserListItem]{}, err
	}
	page, pageSize := domainadmin.NormalizePage(f.Page, f.PageSize)
	items := make([]dto.AdminUserListItem, 0, len(users))
	for i := range users {
		roles, err := repos.Users.RolesByUser(ctx, users[i].ID)
		if err != nil {
			return dto.Page[dto.AdminUserListItem]{}, err
		}
		items = append(items, dto.AdminUserListItem{
			ID:          users[i].ID,
			UserName:    users[i].UserName,
			DisplayName: users[i].DisplayName,
			Email:       users[i].Email,
			Status:      string(users[i].Status),
			Roles:       roleBriefs(roles),
			CreatedAt:   users[i].CreatedAt,
			UpdatedAt:   users[i].UpdatedAt,
		})
	}
	return dto.Page[dto.AdminUserListItem]{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// GetUser 管理员详情。
func (s *AdminUserService) GetUser(ctx context.Context, id int64) (dto.AdminUserDetail, error) {
	repos := s.tx.Repositories()
	user, err := repos.Users.FindByID(ctx, id)
	if err != nil {
		return dto.AdminUserDetail{}, err
	}
	roles, err := repos.Users.RolesByUser(ctx, id)
	if err != nil {
		return dto.AdminUserDetail{}, err
	}
	identities, err := repos.Identities.ListByUser(ctx, id)
	if err != nil {
		return dto.AdminUserDetail{}, err
	}
	perms, err := repos.Permissions.ListCodesByUser(ctx, id)
	if err != nil {
		return dto.AdminUserDetail{}, err
	}
	return dto.AdminUserDetail{
		ID:          user.ID,
		UserName:    user.UserName,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Status:      string(user.Status),
		Roles:       roleBriefs(roles),
		Identities:  maskIdentities(identities),
		Permissions: emptyIfNil(perms),
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

// ===== command =====

// CreateUser 新建管理员（user + 可选 password/feishu 身份 + 角色，单事务）。
func (s *AdminUserService) CreateUser(ctx context.Context, cmd dto.CreateUserCmd) (dto.AdminUserDetail, error) {
	userName := strings.TrimSpace(cmd.UserName)
	displayName := strings.TrimSpace(cmd.DisplayName)
	if userName == "" || len(userName) > 64 || displayName == "" || len(displayName) > 128 {
		return dto.AdminUserDetail{}, ErrValidation
	}
	if len(cmd.Email) > 128 || !isValidEmail(cmd.Email) {
		return dto.AdminUserDetail{}, ErrValidation
	}
	status := common.AdminUserStatus(cmd.Status)
	if cmd.Status == "" {
		status = common.AdminUserStatusActive
	}
	if !domainadmin.IsValidStatus(status) {
		return dto.AdminUserDetail{}, ErrValidation
	}
	if cmd.Password != "" && (len(cmd.Password) < 8 || len(cmd.Password) > 128) {
		return dto.AdminUserDetail{}, ErrValidation
	}

	var newID int64
	err := s.tx.InTx(ctx, func(repos Repositories) error {
		user := &domainadmin.AdminUser{
			UserName:    userName,
			DisplayName: displayName,
			Email:       cmd.Email,
			Status:      status,
		}
		if err := repos.Users.Create(ctx, user); err != nil {
			return err
		}
		newID = user.ID

		if cmd.Password != "" {
			hash, err := s.hasher.Hash(cmd.Password)
			if err != nil {
				return err
			}
			if err := repos.Identities.Upsert(ctx, &domainadmin.AdminIdentity{
				UserIDRef:            user.ID,
				IdentityType:         common.IdentityTypePassword,
				IdentityKey:          userName,
				CredentialCiphertext: hash,
			}); err != nil {
				return err
			}
		}
		if strings.TrimSpace(cmd.FeishuKey) != "" {
			if err := repos.Identities.Upsert(ctx, &domainadmin.AdminIdentity{
				UserIDRef:    user.ID,
				IdentityType: common.IdentityTypeFeishu,
				IdentityKey:  cmd.FeishuKey,
			}); err != nil {
				return err
			}
		}
		if len(cmd.RoleIDs) > 0 {
			if err := ensureRolesExist(ctx, repos, cmd.RoleIDs); err != nil {
				return err
			}
			if err := repos.Users.ReplaceRoles(ctx, user.ID, cmd.RoleIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return dto.AdminUserDetail{}, err
	}

	if err := s.writeAudit(ctx, actorFromCtx(ctx), "admin_user.create", "admin_user", newID, map[string]any{"userName": userName}); err != nil {
		return dto.AdminUserDetail{}, err
	}
	return s.GetUser(ctx, newID)
}

// UpdateUser 更新管理员（含启停状态机）。
func (s *AdminUserService) UpdateUser(ctx context.Context, cmd dto.UpdateUserCmd) (dto.AdminUserDetail, error) {
	repos := s.tx.Repositories()
	user, err := repos.Users.FindByID(ctx, cmd.ID)
	if err != nil {
		return dto.AdminUserDetail{}, err
	}
	statusBefore := user.Status

	if cmd.DisplayName != nil {
		dn := strings.TrimSpace(*cmd.DisplayName)
		if dn == "" || len(dn) > 128 {
			return dto.AdminUserDetail{}, ErrValidation
		}
		user.DisplayName = dn
	}
	if cmd.Email != nil {
		if len(*cmd.Email) > 128 || !isValidEmail(*cmd.Email) {
			return dto.AdminUserDetail{}, ErrValidation
		}
		user.Email = *cmd.Email
	}
	if cmd.Status != nil {
		to := common.AdminUserStatus(*cmd.Status)
		if err := user.ApplyStatus(to); err != nil {
			return dto.AdminUserDetail{}, ErrValidation
		}
	}
	if err := repos.Users.Update(ctx, user); err != nil {
		return dto.AdminUserDetail{}, err
	}

	if err := s.writeAudit(ctx, actorFromCtx(ctx), "admin_user.update", "admin_user", user.ID, map[string]any{
		"statusBefore": string(statusBefore), "statusAfter": string(user.Status),
	}); err != nil {
		return dto.AdminUserDetail{}, err
	}
	return s.GetUser(ctx, user.ID)
}

// AssignRoles 全量覆盖用户角色。
func (s *AdminUserService) AssignRoles(ctx context.Context, cmd dto.AssignRolesCmd) (dto.AdminUserDetail, error) {
	err := s.tx.InTx(ctx, func(repos Repositories) error {
		if _, err := repos.Users.FindByID(ctx, cmd.UserID); err != nil {
			return err
		}
		if err := ensureRolesExist(ctx, repos, cmd.RoleIDs); err != nil {
			return err
		}
		return repos.Users.ReplaceRoles(ctx, cmd.UserID, cmd.RoleIDs)
	})
	if err != nil {
		return dto.AdminUserDetail{}, err
	}
	if err := s.writeAudit(ctx, actorFromCtx(ctx), "admin_user.assign_roles", "admin_user", cmd.UserID, map[string]any{"roleIds": cmd.RoleIDs}); err != nil {
		return dto.AdminUserDetail{}, err
	}
	return s.GetUser(ctx, cmd.UserID)
}

// ResetPassword 重置密码（明文不落库、不入审计）。
func (s *AdminUserService) ResetPassword(ctx context.Context, cmd dto.ResetPasswordCmd) error {
	if len(cmd.NewPassword) < 8 || len(cmd.NewPassword) > 128 {
		return ErrValidation
	}
	repos := s.tx.Repositories()
	user, err := repos.Users.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	hash, err := s.hasher.Hash(cmd.NewPassword)
	if err != nil {
		return err
	}
	if err := repos.Identities.Upsert(ctx, &domainadmin.AdminIdentity{
		UserIDRef:            user.ID,
		IdentityType:         common.IdentityTypePassword,
		IdentityKey:          user.UserName,
		CredentialCiphertext: hash,
	}); err != nil {
		return err
	}
	return s.writeAudit(ctx, actorFromCtx(ctx), "admin_user.reset_password", "admin_user", user.ID, nil)
}

func (s *AdminUserService) writeAudit(ctx context.Context, actorID int64, action, resourceType string, resourceID int64, detail map[string]any) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Write(ctx, AuditEntry{ActorID: actorID, Action: action, ResourceType: resourceType, ResourceID: int64ToStr(resourceID), Detail: detail})
}

func ensureRolesExist(ctx context.Context, repos Repositories, roleIDs []int64) error {
	for _, id := range roleIDs {
		if _, err := repos.Roles.FindByID(ctx, id); err != nil {
			if err == ErrNotFound {
				return ErrValidation
			}
			return err
		}
	}
	return nil
}
