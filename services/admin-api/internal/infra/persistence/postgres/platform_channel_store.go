package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	platformchannelapp "github.com/csw/console/services/admin-api/internal/app/platformchannel"
)

// PlatformChannelStore 实现 platformchannelapp.TxManager：平台渠道与渠道模版的池级仓储 + 事务编排。
// 这里的表全部位于共享 schema platform，与当前 env 无关。
type PlatformChannelStore struct {
	pool *pgxpool.Pool
}

// NewPlatformChannelStore 绑定连接池。
func NewPlatformChannelStore(pool *pgxpool.Pool) *PlatformChannelStore {
	return &PlatformChannelStore{pool: pool}
}

func platformChannelReposFrom(db DBTX) platformchannelapp.Repositories {
	return platformchannelapp.Repositories{
		Channels:  &PlatformChannelRepo{db: db},
		Templates: &ChannelTemplateAdminRepo{db: db},
	}
}

// Repositories 返回绑定到连接池的仓储句柄（非事务，自动提交）。
func (s *PlatformChannelStore) Repositories() platformchannelapp.Repositories {
	return platformChannelReposFrom(s.pool)
}

// InTx 在单事务内执行 fn；fn 返回错误则回滚，否则提交。
func (s *PlatformChannelStore) InTx(ctx context.Context, fn func(platformchannelapp.Repositories) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(platformChannelReposFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
