package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	cashierapp "github.com/csw/console/services/admin-api/internal/app/cashier"
)

type CashierStore struct {
	pool *pgxpool.Pool
}

func NewCashierStore(pool *pgxpool.Pool) *CashierStore { return &CashierStore{pool: pool} }

func cashierRepoFrom(db DBTX) cashierapp.CashierTemplateRepository {
	return &CashierRepo{db: db}
}

func (s *CashierStore) Repository() cashierapp.CashierTemplateRepository {
	return cashierRepoFrom(s.pool)
}

func (s *CashierStore) InTx(ctx context.Context, fn func(cashierapp.CashierTemplateRepository) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(cashierRepoFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
