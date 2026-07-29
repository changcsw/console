package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 连库回归（SCENARIO_WITH_DB=1 + POSTGRES_DSN）：认证类型可以没有可用模板
// （模板 enabled=FALSE 或整行缺失，见 tests/fixtures/common/account-auth.sql 的 apple 前置），
// 此时 LEFT JOIN LATERAL 的 template_version 为 NULL，读路径必须照常返回而非报错。
func TestAccountAuthRepoTypeCatalogTolerateMissingTemplate(t *testing.T) {
	if os.Getenv("SCENARIO_WITH_DB") != "1" {
		t.Skip("SCENARIO_WITH_DB=1 required")
	}
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("POSTGRES_DSN or DATABASE_URL required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pg pool: %v", err)
	}
	defer pool.Close()

	// 在事务内制造「缺模板」状态并回滚，避免污染其它用例的库状态。
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM platform.account_auth_templates`); err != nil {
		t.Fatalf("clear templates: %v", err)
	}

	repo := &AccountAuthRepo{db: tx}
	items, err := repo.ListTypeCatalog(ctx)
	if err != nil {
		t.Fatalf("ListTypeCatalog with missing templates: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected auth type catalog to be non-empty")
	}
	for _, item := range items {
		if item.Template.TemplateVersion != "" {
			t.Fatalf("auth type %s: expected empty template version, got %q", item.AuthTypeID, item.Template.TemplateVersion)
		}
	}
}
