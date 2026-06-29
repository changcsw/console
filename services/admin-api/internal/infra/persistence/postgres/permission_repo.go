package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/csw/console/services/admin-api/internal/domain/admin"
)

// PermissionRepo platform.admin_permissions 仓储。
type PermissionRepo struct{ db DBTX }

func (r *PermissionRepo) Create(ctx context.Context, p *admin.Permission) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO admin_permissions (permission_code, permission_name) VALUES ($1,$2)
		 RETURNING id, created_at, updated_at`,
		p.PermissionCode, p.PermissionName).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	return mapErr(err)
}

func (r *PermissionRepo) Delete(ctx context.Context, id int64) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM admin_role_permissions WHERE permission_id_ref=$1`, id); err != nil {
		return mapErr(err)
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM admin_permissions WHERE id=$1`, id)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return mapErr(errNoRows())
	}
	return nil
}

func (r *PermissionRepo) FindByID(ctx context.Context, id int64) (*admin.Permission, error) {
	var p admin.Permission
	err := r.db.QueryRow(ctx,
		`SELECT id, permission_code, permission_name, created_at, updated_at FROM admin_permissions WHERE id=$1`, id).
		Scan(&p.ID, &p.PermissionCode, &p.PermissionName, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	return &p, nil
}

func (r *PermissionRepo) List(ctx context.Context, f admin.PermissionFilter) ([]admin.Permission, int, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		where = append(where, fmt.Sprintf("(permission_code ILIKE $%d OR permission_name ILIKE $%d)", idx, idx))
		args = append(args, "%"+kw+"%")
		idx++
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM admin_permissions WHERE "+cond, args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err)
	}

	order := orderBy(f.Sort, map[string]string{"updatedAt": "updated_at", "createdAt": "created_at", "permissionCode": "permission_code"}, "permission_code ASC")
	query := fmt.Sprintf(`SELECT id, permission_code, permission_name, created_at, updated_at
		 FROM admin_permissions WHERE %s ORDER BY %s`, cond, order)
	if !f.All {
		page, pageSize := admin.NormalizePage(f.Page, f.PageSize)
		args = append(args, pageSize, (page-1)*pageSize)
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	defer rows.Close()
	var perms []admin.Permission
	for rows.Next() {
		var p admin.Permission
		if err := rows.Scan(&p.ID, &p.PermissionCode, &p.PermissionName, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, mapErr(err)
		}
		perms = append(perms, p)
	}
	return perms, total, mapErr(rows.Err())
}

func (r *PermissionRepo) FindByIDs(ctx context.Context, ids []int64) ([]admin.Permission, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx,
		`SELECT id, permission_code, permission_name, created_at, updated_at
		 FROM admin_permissions WHERE id = ANY($1) ORDER BY permission_code`, ids)
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

// ListCodesByUser 权限解析：用户所有角色授予权限码的并集（DISTINCT）。
func (r *PermissionRepo) ListCodesByUser(ctx context.Context, userID int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT p.permission_code
		 FROM admin_user_roles ur
		 JOIN admin_role_permissions rp ON rp.role_id_ref = ur.role_id_ref
		 JOIN admin_permissions p ON p.id = rp.permission_id_ref
		 WHERE ur.user_id_ref=$1
		 ORDER BY p.permission_code`, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var codes []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, mapErr(err)
		}
		codes = append(codes, c)
	}
	return codes, mapErr(rows.Err())
}

func (r *PermissionRepo) CountRoles(ctx context.Context, permID int64) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM admin_role_permissions WHERE permission_id_ref=$1`, permID).Scan(&n)
	return n, mapErr(err)
}
