package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	featurepluginapp "github.com/csw/console/services/admin-api/internal/app/featureplugin"
)

// FeaturePluginAdminStore 实现 featurepluginapp.TxManager：
// 插件分类字典 / 插件主数据 / 参数模板的池级仓储 + 事务编排。
// 这三张表都位于共享 schema platform，与当前 env 无关（删除前的引用检查会只读当前 env 的游戏侧配置表）。
type FeaturePluginAdminStore struct {
	pool *pgxpool.Pool
}

// NewFeaturePluginAdminStore 绑定连接池。
func NewFeaturePluginAdminStore(pool *pgxpool.Pool) *FeaturePluginAdminStore {
	return &FeaturePluginAdminStore{pool: pool}
}

func featurePluginReposFrom(db DBTX) featurepluginapp.Repositories {
	return featurepluginapp.Repositories{
		Categories: &FeaturePluginCategoryRepo{db: db},
		Plugins:    &FeaturePluginAdminRepo{db: db},
		Templates:  &FeaturePluginTemplateAdminRepo{db: db},
	}
}

// Repositories 返回绑定到连接池的仓储句柄（非事务，自动提交）。
func (s *FeaturePluginAdminStore) Repositories() featurepluginapp.Repositories {
	return featurePluginReposFrom(s.pool)
}

// InTx 在单事务内执行 fn；fn 返回错误则回滚，否则提交。
func (s *FeaturePluginAdminStore) InTx(ctx context.Context, fn func(featurepluginapp.Repositories) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(featurePluginReposFrom(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
