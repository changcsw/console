package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	productapp "github.com/csw/console/services/admin-api/internal/app/product"
)

type ProductStore struct {
	pool *pgxpool.Pool
}

func NewProductStore(pool *pgxpool.Pool) *ProductStore { return &ProductStore{pool: pool} }

func productReposFrom(db DBTX) productapp.Repositories {
	return productapp.Repositories{
		Products:            &ProductRepo{db: db},
		ChannelProducts:     &ChannelProductRepo{db: db},
		Packages:            &ProductChannelPackageRepo{db: db},
		CurrencySpecs:       &CurrencySpecRepo{db: db},
		GameChannelIAP:      &GameChannelIAPConfigRepo{db: db},
		PackageIAPOverrides: &ChannelPackageIAPOverrideRepo{db: db},
		ChannelIAPTemplates: &ChannelIAPTemplateRepo{db: db},
	}
}

func (s *ProductStore) Repositories() productapp.Repositories { return productReposFrom(s.pool) }

func (s *ProductStore) InTx(ctx context.Context, fn func(productapp.Repositories) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(productReposFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
