package scenario

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/infra/config"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
	"github.com/csw/console/services/admin-api/internal/transport/httpserver"
)

// harnessConfigFromEnv 读取连库 scenario 运行所需的后端配置。
// POSTGRES_DSN 优先，其次 DATABASE_URL（与 scripts/regression/lib.sh 一致）。
func harnessConfigFromEnv() (config.Config, error) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return config.Config{}, fmt.Errorf("POSTGRES_DSN or DATABASE_URL required for SCENARIO_WITH_DB")
	}
	secret := os.Getenv("ADMIN_JWT_SECRET")
	if secret == "" {
		secret = "regression-scenario-jwt-secret"
	}
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "sandbox"
	}
	aesKey := os.Getenv("ADMIN_AES_KEY")
	if aesKey == "" {
		aesKey = "0123456789abcdef0123456789abcdef"
	}
	return config.Config{
		AppName:            "admin-api",
		Environment:        env,
		HTTPAddress:        ":0",
		PostgresDSN:        dsn,
		SandboxPostgresDSN: dsn,
		ProductionDSN:      dsn,
		JWTSecret:          secret,
		JWTIssuer:          "admin-api",
		JWTAccessTTL:       30 * time.Minute,
		JWTRefreshTTL:      336 * time.Hour,
		BcryptCost:         10,
		AESKey:             aesKey,
		FeishuMock:         true,
	}, nil
}

func newHarnessPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("scenario harness pg pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func buildDBHandler(t *testing.T) (http.Handler, *roleTokenIssuer) {
	t.Helper()
	cfg, err := harnessConfigFromEnv()
	if err != nil {
		t.Skip(err.Error())
	}
	srv := httpserver.New(cfg)
	pool := newHarnessPool(t, cfg.PostgresDSN)
	issuer, err := newRoleTokenIssuer(cfg, pool)
	if err != nil {
		t.Fatalf("scenario harness auth: %v", err)
	}
	return srv.Handler, issuer
}

// roleTokenIssuer 按 manifest auth.role 从 platform RBAC 查权限并签发 JWT。
type roleTokenIssuer struct {
	issuer *infrajwt.Issuer
	pool   *pgxpool.Pool
	env    common.Environment
}

func newRoleTokenIssuer(cfg config.Config, pool *pgxpool.Pool) (*roleTokenIssuer, error) {
	issuer, err := infrajwt.NewIssuer(infrajwt.Config{
		Secret:     cfg.JWTSecret,
		Issuer:     cfg.JWTIssuer,
		AccessTTL:  cfg.JWTAccessTTL,
		RefreshTTL: cfg.JWTRefreshTTL,
	})
	if err != nil {
		return nil, err
	}
	return &roleTokenIssuer{
		issuer: issuer,
		pool:   pool,
		env:    common.Environment(cfg.Environment),
	}, nil
}

func (r *roleTokenIssuer) accessToken(roleCode string) (string, error) {
	perms, err := r.permissionsForRole(roleCode)
	if err != nil {
		return "", err
	}
	roles := []string{roleCode}
	if roleCode == domainauth.SuperAdminRole {
		roles = []string{domainauth.SuperAdminRole}
	}
	ac := domainauth.NewAuthContext(1001, "scenario", "Scenario User", roles, perms, r.env)
	pair, err := r.issuer.IssuePair(ac)
	if err != nil {
		return "", err
	}
	return pair.AccessToken, nil
}

func (r *roleTokenIssuer) permissionsForRole(roleCode string) ([]string, error) {
	if roleCode == domainauth.SuperAdminRole {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := r.pool.Query(ctx, `
SELECT p.permission_code
FROM platform.admin_roles r
JOIN platform.admin_role_permissions rp ON rp.role_id_ref = r.id
JOIN platform.admin_permissions p ON p.id = rp.permission_id_ref
WHERE r.role_code = $1
ORDER BY p.permission_code`, roleCode)
	if err != nil {
		return nil, fmt.Errorf("query role %q permissions: %w", roleCode, err)
	}
	defer rows.Close()
	var perms []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		perms = append(perms, code)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return perms, nil
}
