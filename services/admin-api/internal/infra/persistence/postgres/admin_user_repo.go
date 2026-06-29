package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/csw/console/services/admin-api/internal/domain/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// AdminUserRepo platform.admin_users 仓储（SQL 不带 schema 前缀，靠 search_path）。
type AdminUserRepo struct{ db DBTX }

func (r *AdminUserRepo) Create(ctx context.Context, u *admin.AdminUser) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO admin_users (user_name, display_name, email, status)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, created_at, updated_at`,
		u.UserName, u.DisplayName, u.Email, string(u.Status),
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	return mapErr(err)
}

func (r *AdminUserRepo) Update(ctx context.Context, u *admin.AdminUser) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE admin_users
		 SET display_name=$2, email=$3, status=$4, updated_at=NOW()
		 WHERE id=$1`,
		u.ID, u.DisplayName, u.Email, string(u.Status),
	)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(errNoRows())
	}
	return nil
}

func (r *AdminUserRepo) FindByID(ctx context.Context, id int64) (*admin.AdminUser, error) {
	return r.scanOne(ctx,
		`SELECT id, user_name, display_name, email, status, created_at, updated_at
		 FROM admin_users WHERE id=$1`, id)
}

func (r *AdminUserRepo) FindByUserName(ctx context.Context, userName string) (*admin.AdminUser, error) {
	return r.scanOne(ctx,
		`SELECT id, user_name, display_name, email, status, created_at, updated_at
		 FROM admin_users WHERE user_name=$1`, userName)
}

func (r *AdminUserRepo) scanOne(ctx context.Context, query string, arg any) (*admin.AdminUser, error) {
	var u admin.AdminUser
	var status string
	err := r.db.QueryRow(ctx, query, arg).Scan(
		&u.ID, &u.UserName, &u.DisplayName, &u.Email, &status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	u.Status = common.AdminUserStatus(status)
	return &u, nil
}

func (r *AdminUserRepo) List(ctx context.Context, f admin.AdminUserFilter) ([]admin.AdminUser, int, error) {
	page, pageSize := admin.NormalizePage(f.Page, f.PageSize)

	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		where = append(where, fmt.Sprintf("(user_name ILIKE $%d OR display_name ILIKE $%d OR email ILIKE $%d)", idx, idx, idx))
		args = append(args, "%"+kw+"%")
		idx++
	}
	if f.Status != "" {
		where = append(where, fmt.Sprintf("status=$%d", idx))
		args = append(args, string(f.Status))
		idx++
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM admin_users WHERE "+cond, args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}

	order := orderBy(f.Sort, map[string]string{"updatedAt": "updated_at", "createdAt": "created_at", "userName": "user_name"}, "updated_at DESC")
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT id, user_name, display_name, email, status, created_at, updated_at
		 FROM admin_users WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`, cond, order, idx, idx+1),
		args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()

	var users []admin.AdminUser
	for rows.Next() {
		var u admin.AdminUser
		var status string
		if err := rows.Scan(&u.ID, &u.UserName, &u.DisplayName, &u.Email, &status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, mapErr(err)
		}
		u.Status = common.AdminUserStatus(status)
		users = append(users, u)
	}
	return users, total, mapErr(rows.Err())
}

func (r *AdminUserRepo) ReplaceRoles(ctx context.Context, userID int64, roleIDs []int64) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM admin_user_roles WHERE user_id_ref=$1`, userID); err != nil {
		return mapErr(err)
	}
	for _, rid := range roleIDs {
		if _, err := r.db.Exec(ctx,
			`INSERT INTO admin_user_roles (user_id_ref, role_id_ref) VALUES ($1,$2)
			 ON CONFLICT (user_id_ref, role_id_ref) DO NOTHING`, userID, rid); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func (r *AdminUserRepo) RolesByUser(ctx context.Context, userID int64) ([]admin.Role, error) {
	rows, err := r.db.Query(ctx,
		`SELECT ro.id, ro.role_code, ro.role_name, ro.created_at, ro.updated_at
		 FROM admin_user_roles ur JOIN admin_roles ro ON ro.id = ur.role_id_ref
		 WHERE ur.user_id_ref=$1 ORDER BY ro.role_code`, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var roles []admin.Role
	for rows.Next() {
		var ro admin.Role
		if err := rows.Scan(&ro.ID, &ro.RoleCode, &ro.RoleName, &ro.CreatedAt, &ro.UpdatedAt); err != nil {
			return nil, mapErr(err)
		}
		roles = append(roles, ro)
	}
	return roles, mapErr(rows.Err())
}
