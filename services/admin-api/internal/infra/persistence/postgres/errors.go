package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
)

const pgUniqueViolation = "23505"

// mapErr 把 pgx 错误归一化为 app 层哨兵错误，供 handler 映射全局错误码。
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return adminapp.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return adminapp.ErrConflict
	}
	return err
}
