package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	pluginapp "github.com/csw/console/services/admin-api/internal/app/plugin"
)

type PluginStore struct {
	pool *pgxpool.Pool
}

func NewPluginStore(pool *pgxpool.Pool) *PluginStore { return &PluginStore{pool: pool} }

func pluginReposFrom(db DBTX) pluginapp.Repositories {
	return pluginapp.Repositories{
		Features: &FeaturePluginRepo{db: db},
		Game:     &GameChannelPluginRepo{db: db},
		Packages: &ChannelPackagePluginRepo{db: db},
	}
}

func (s *PluginStore) Repositories() pluginapp.Repositories { return pluginReposFrom(s.pool) }

func (s *PluginStore) InTx(ctx context.Context, fn func(pluginapp.Repositories) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(pluginReposFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
