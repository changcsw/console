package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	channelapp "github.com/csw/console/services/admin-api/internal/app/channel"
)

// ChannelStore 实现 channelapp.TxManager：池级仓储 + 事务编排（绑定主连接池，env 由 search_path 钉死）。
type ChannelStore struct {
	pool *pgxpool.Pool
}

// NewChannelStore 绑定连接池。
func NewChannelStore(pool *pgxpool.Pool) *ChannelStore { return &ChannelStore{pool: pool} }

func channelReposFrom(db DBTX) channelapp.Repositories {
	return channelapp.Repositories{
		Channels:       &ChannelRepo{db: db},
		GameChannels:   &GameChannelRepo{db: db},
		Packages:       &ChannelPackageRepo{db: db},
		LoginTemplates: &ChannelLoginTemplateRepo{db: db},
		LoginConfigs:   &ChannelLoginConfigRepo{db: db},
	}
}

// Repositories 返回绑定到连接池的仓储句柄（非事务，自动提交）。
func (s *ChannelStore) Repositories() channelapp.Repositories { return channelReposFrom(s.pool) }

// InTx 在单事务内执行 fn；fn 返回错误则回滚，否则提交。
func (s *ChannelStore) InTx(ctx context.Context, fn func(channelapp.Repositories) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(channelReposFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
