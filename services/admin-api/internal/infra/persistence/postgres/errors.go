package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
)

const (
	pgUniqueViolation = "23505"
	pgUndefinedTable  = "42P01"
)

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

// isUndefinedTableErr 判定错误是否为 Postgres「relation does not exist」（42P01）。
// 用于识别「当前 env schema 缺业务表结构」（如未跑 000012/000017）这类可读错误，
// 与真正的编程期 SQL 错误区分开，调用方应给出明确提示而非裸 500。
func isUndefinedTableErr(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUndefinedTable
}
