package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/csw/console/services/admin-api/internal/domain/admin"
)

// RoleRepo platform.admin_roles 仓储。
type RoleRepo struct{ db DBTX }

func (r *RoleRepo) Create(ctx context.Context, role *admin.Role) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO admin_roles (role_code, role_name) VALUES ($1,$2)
		 RETURNING id, created_at, updated_at`,
		role.RoleCode, role.RoleName).Scan(&role.ID, &role.CreatedAt, &role.UpdatedAt)
	return mapErr(err)
}

func (r *RoleRepo) Update(ctx context.Context, role *admin.Role) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE admin_roles SET role_name=$2, updated_at=NOW() WHERE id=$1`,
		role.ID, role.RoleName)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(errNoRows())
	}
	return nil
}

func (r *RoleRepo) Delete(ctx context.Context, id int64) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM admin_role_permissions WHERE role_id_ref=$1`, id); err != nil {
		return mapErr(err)
	}
	if _, err := r.db.Exec(ctx, `DELETE FROM admin_user_roles WHERE role_id_ref=$1`, id); err != nil {
		return mapErr(err)
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM admin_roles WHERE id=$1`, id)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(errNoRows())
	}
	return nil
}

func (r *RoleRepo) FindByID(ctx context.Context, id int64) (*admin.Role, error) {
	var role admin.Role
	err := r.db.QueryRow(ctx,
		`SELECT id, role_code, role_name, created_at, updated_at FROM admin_roles WHERE id=$1`, id).
		Scan(&role.ID, &role.RoleCode, &role.RoleName, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &role, nil
}

func (r *RoleRepo) List(ctx context.Context, f admin.RoleFilter) ([]admin.Role, int, error) {
	page, pageSize := admin.NormalizePage(f.Page, f.PageSize)
	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		where = append(where, fmt.Sprintf("(role_code ILIKE $%d OR role_name ILIKE $%d)", idx, idx))
		args = append(args, "%"+kw+"%")
		idx++
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM admin_roles WHERE "+cond, args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}

	order := orderBy(f.Sort, map[string]string{"updatedAt": "updated_at", "createdAt": "created_at", "roleCode": "role_code"}, "updated_at DESC")
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT id, role_code, role_name, created_at, updated_at
		 FROM admin_roles WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`, cond, order, idx, idx+1), args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	var roles []admin.Role
	for rows.Next() {
		var role admin.Role
		if err := rows.Scan(&role.ID, &role.RoleCode, &role.RoleName, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, 0, mapErr(err)
		}
		roles = append(roles, role)
	}
	return roles, total, mapErr(rows.Err())
}

func (r *RoleRepo) ReplacePermissions(ctx context.Context, roleID int64, permIDs []int64) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM admin_role_permissions WHERE role_id_ref=$1`, roleID); err != nil {
		return mapErr(err)
	}
	for _, pid := range permIDs {
		if _, err := r.db.Exec(ctx,
			`INSERT INTO admin_role_permissions (role_id_ref, permission_id_ref) VALUES ($1,$2)
			 ON CONFLICT (role_id_ref, permission_id_ref) DO NOTHING`, roleID, pid); err != nil {
			return mapErr(err)
		}
	}
	return nil
}

func (r *RoleRepo) CountUsers(ctx context.Context, roleID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM admin_user_roles WHERE role_id_ref=$1`, roleID).Scan(&n)
	return n, mapErr(err)
}

func (r *RoleRepo) PermissionsByRole(ctx context.Context, roleID int64) ([]admin.Permission, error) {
	rows, err := r.db.Query(ctx,
		`SELECT p.id, p.permission_code, p.permission_name, p.created_at, p.updated_at
		 FROM admin_role_permissions rp JOIN admin_permissions p ON p.id = rp.permission_id_ref
		 WHERE rp.role_id_ref=$1 ORDER BY p.permission_code`, roleID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var perms []admin.Permission
	for rows.Next() {
		var p admin.Permission
		if err := rows.Scan(&p.ID, &p.PermissionCode, &p.PermissionName, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, mapErr(err)
		}
		perms = append(perms, p)
	}
	return perms, mapErr(rows.Err())
}
