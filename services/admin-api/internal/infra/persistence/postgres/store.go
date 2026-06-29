package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
)

// DBTX 抽象 pgxpool.Pool 与 pgx.Tx 的公共查询接口，使仓储可同时用于池与事务。
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store 实现 adminapp.TxManager：池级仓储 + 事务编排。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 绑定连接池。
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func reposFrom(db DBTX) adminapp.Repositories {
	return adminapp.Repositories{
		Users:       &AdminUserRepo{db: db},
		Identities:  &AdminIdentityRepo{db: db},
		Roles:       &RoleRepo{db: db},
		Permissions: &PermissionRepo{db: db},
	}
}

// Repositories 返回绑定到连接池的仓储句柄（非事务，自动提交）。
func (s *Store) Repositories() adminapp.Repositories { return reposFrom(s.pool) }

// InTx 在单事务内执行 fn；fn 返回错误则回滚，否则提交。
func (s *Store) InTx(ctx context.Context, fn func(adminapp.Repositories) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(reposFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
