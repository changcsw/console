package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	accountauthapp "github.com/csw/console/services/admin-api/internal/app/accountauth"
)

type AccountAuthStore struct {
	pool *pgxpool.Pool
}

func NewAccountAuthStore(pool *pgxpool.Pool) *AccountAuthStore { return &AccountAuthStore{pool: pool} }

func accountAuthRepoFrom(db DBTX) accountauthapp.Repository {
	return &AccountAuthRepo{db: db}
}

func (s *AccountAuthStore) Repository() accountauthapp.Repository { return accountAuthRepoFrom(s.pool) }

func (s *AccountAuthStore) InTx(ctx context.Context, fn func(accountauthapp.Repository) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(accountAuthRepoFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
