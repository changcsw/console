package postgres

import (
	"context"

	"github.com/csw/console/services/admin-api/internal/domain/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// AdminIdentityRepo platform.admin_identities 仓储。
type AdminIdentityRepo struct{ db DBTX }

func (r *AdminIdentityRepo) FindByTypeKey(ctx context.Context, t string, key string) (*admin.AdminIdentity, error) {
	var id admin.AdminIdentity
	var typ string
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id_ref, identity_type, identity_key, credential_ciphertext, created_at, updated_at
		 FROM admin_identities WHERE identity_type=$1 AND identity_key=$2`, t, key).
		Scan(&id.ID, &id.UserIDRef, &typ, &id.IdentityKey, &id.CredentialCiphertext, &id.CreatedAt, &id.UpdatedAt)
	if err != nil {
		return nil, mapErr(err)
	}
	id.IdentityType = common.IdentityType(typ)
	return &id, nil
}

func (r *AdminIdentityRepo) ListByUser(ctx context.Context, userID int64) ([]admin.AdminIdentity, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id_ref, identity_type, identity_key, credential_ciphertext, created_at, updated_at
		 FROM admin_identities WHERE user_id_ref=$1 ORDER BY identity_type`, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var out []admin.AdminIdentity
	for rows.Next() {
		var id admin.AdminIdentity
		var typ string
		if err := rows.Scan(&id.ID, &id.UserIDRef, &typ, &id.IdentityKey, &id.CredentialCiphertext, &id.CreatedAt, &id.UpdatedAt); err != nil {
			return nil, mapErr(err)
		}
		id.IdentityType = common.IdentityType(typ)
		out = append(out, id)
	}
	return out, mapErr(rows.Err())
}

// Upsert 按 (identity_type, identity_key) 唯一键插入或更新 credential。
func (r *AdminIdentityRepo) Upsert(ctx context.Context, id *admin.AdminIdentity) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO admin_identities (user_id_ref, identity_type, identity_key, credential_ciphertext)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (identity_type, identity_key)
		 DO UPDATE SET credential_ciphertext=EXCLUDED.credential_ciphertext, updated_at=NOW()
		 RETURNING id, created_at, updated_at`,
		id.UserIDRef, string(id.IdentityType), id.IdentityKey, id.CredentialCiphertext).
		Scan(&id.ID, &id.CreatedAt, &id.UpdatedAt)
	return mapErr(err)
}
