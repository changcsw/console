// Package postgres 提供 pgx 连接池与 admin_* 仓储实现。
// 业务/平台表 SQL 不写 schema 前缀、不带 env 谓词，靠连接固定的 search_path 路由（01 §4.2/§4.4）。
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool 构造主连接池，并在 AfterConnect 钩子里对每条物理连接钉死
// search_path = <env>, platform（不含 public），整个连接生命周期不再切换（01 §4.4.1）。
func NewPool(ctx context.Context, dsn, env string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	searchPath := fmt.Sprintf("%s, platform", quoteIdent(env))
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path = "+searchPath)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func quoteIdent(s string) string {
	return `"` + s + `"`
}
