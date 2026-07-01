package postgres

import (
	"context"

	paymentapp "github.com/csw/console/services/admin-api/internal/app/payment"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentStore struct {
	pool *pgxpool.Pool
}

func NewPaymentStore(pool *pgxpool.Pool) *PaymentStore { return &PaymentStore{pool: pool} }

func paymentRepoFrom(db DBTX) paymentapp.Repository {
	return &PaymentRepo{db: db}
}

func (s *PaymentStore) Repository() paymentapp.Repository {
	return paymentRepoFrom(s.pool)
}

func (s *PaymentStore) InTx(ctx context.Context, fn func(paymentapp.Repository) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(paymentRepoFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
