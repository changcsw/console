package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	channellogin "github.com/csw/console/services/admin-api/internal/app/channellogin"
)

// ChannelLoginStore 实现 channellogin.TxManager：池级仓储 + 事务编排。
// 复用 GameChannelRepo / ChannelRepo 读 game_channels / channel_policies；
// 新增 ChannelLoginTemplateRepo / ChannelLoginConfigRepo 读写模板与配置。
type ChannelLoginStore struct {
	pool *pgxpool.Pool
}

// NewChannelLoginStore 绑定主连接池（env 由连接 search_path 钉死，01 §4.4）。
func NewChannelLoginStore(pool *pgxpool.Pool) *ChannelLoginStore {
	return &ChannelLoginStore{pool: pool}
}

func channelLoginReposFrom(db DBTX) channellogin.Repositories {
	return channellogin.Repositories{
		GameChannels: &GameChannelRepo{db: db},
		Policies:     channelLoginPolicyAdapter{repo: &ChannelRepo{db: db}},
		Templates:    &ChannelLoginTemplateRepo{db: db},
		Configs:      &ChannelLoginConfigRepo{db: db},
	}
}

// Repositories 返回绑定到连接池的仓储句柄（非事务，自动提交）。
func (s *ChannelLoginStore) Repositories() channellogin.Repositories {
	return channelLoginReposFrom(s.pool)
}

// InTx 在单事务内执行 fn；fn 返回错误则回滚，否则提交。
func (s *ChannelLoginStore) InTx(ctx context.Context, fn func(channellogin.Repositories) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(channelLoginReposFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
