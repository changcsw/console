package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	gameapp "github.com/csw/console/services/admin-api/internal/app/game"
)

// GameStore 实现 gameapp.TxManager：池级仓储 + 事务编排（绑定主连接池，env 由 search_path 钉死）。
type GameStore struct {
	pool *pgxpool.Pool
}

// NewGameStore 绑定连接池。
func NewGameStore(pool *pgxpool.Pool) *GameStore { return &GameStore{pool: pool} }

func gameReposFrom(db DBTX) gameapp.Repositories {
	return gameapp.Repositories{Games: &GameRepo{db: db}}
}

// Repositories 返回绑定到连接池的仓储句柄（非事务，自动提交）。
func (s *GameStore) Repositories() gameapp.Repositories { return gameReposFrom(s.pool) }

// InTx 在单事务内执行 fn；fn 返回错误则回滚，否则提交。
func (s *GameStore) InTx(ctx context.Context, fn func(gameapp.Repositories) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(gameReposFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
