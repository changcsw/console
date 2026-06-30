package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"

	accountauthapp "github.com/csw/console/services/admin-api/internal/app/accountauth"
	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	auditapp "github.com/csw/console/services/admin-api/internal/app/audit"
	channelapp "github.com/csw/console/services/admin-api/internal/app/channel"
	gameapp "github.com/csw/console/services/admin-api/internal/app/game"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/infra/config"
	"github.com/csw/console/services/admin-api/internal/infra/crypto"
	"github.com/csw/console/services/admin-api/internal/infra/feishu"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
	"github.com/csw/console/services/admin-api/internal/infra/persistence/postgres"
	adminhttp "github.com/csw/console/services/admin-api/internal/transport/http/admin"
	audithttp "github.com/csw/console/services/admin-api/internal/transport/http/audit"
	channelshttp "github.com/csw/console/services/admin-api/internal/transport/http/channels"
	gameshttp "github.com/csw/console/services/admin-api/internal/transport/http/games"
)

// feishuAdapter 把 infra/feishu.Client 适配为 app 层 FeishuClient 端口。
type feishuAdapter struct{ c feishu.Client }

func (a feishuAdapter) ExchangeCode(ctx context.Context, code, redirectURI string) (adminapp.FeishuUser, error) {
	info, err := a.c.ExchangeCode(ctx, code, redirectURI)
	if err != nil {
		return adminapp.FeishuUser{}, err
	}
	return adminapp.FeishuUser{UnionID: info.UnionID, OpenID: info.OpenID, Name: info.Name, Email: info.Email}, nil
}

// buildAdminRouter 装配 auth 模块路由。auth 路由形状始终挂载：
//   - DB + JWT 密钥就绪 → 真实实现（ready=true）。
//   - 缺 DSN/密钥或建池失败 → 降级（ready=false）：受保护路由仍先过 Authn（无令牌 401），
//     通过认证后返回 503；公开路由 503。便于契约稳定与冒烟探活。
func buildAdminRouter(cfg config.Config, logger *slog.Logger) chi.Router {
	env := common.Environment(cfg.Environment)

	secret := cfg.JWTSecret
	if secret == "" {
		secret = ephemeralSecret() // 仅降级占位：无外部令牌可通过校验
	}
	// degraded 构造降级路由（auth + games 路由形状仍挂载，受保护路由过 Authn 后由 RequireBackend 返回 503）。
	degraded := func(iss adminapp.TokenIssuer) chi.Router {
		r := adminhttp.NewRouter(adminhttp.NewHandler(adminhttp.Deps{Env: env}), iss, env, logger, false, nil)
		gameshttp.RegisterRoutes(r, gameshttp.NewHandler(nil, env), iss, env, logger, false, nil)
		channelshttp.RegisterRoutes(r, channelshttp.NewHandler(nil, env), iss, env, logger, false, nil)
		audithttp.RegisterRoutes(r, audithttp.NewHandler(nil), iss, env, logger, false, nil)
		return r
	}

	issuer, err := infrajwt.NewIssuer(infrajwt.Config{
		Secret: secret, Issuer: cfg.JWTIssuer, AccessTTL: cfg.JWTAccessTTL, RefreshTTL: cfg.JWTRefreshTTL,
	})
	if err != nil {
		logger.Error("jwt issuer init failed; admin routes degraded", "err", err)
		return degraded(denyIssuer{})
	}

	if cfg.PostgresDSN == "" || cfg.JWTSecret == "" {
		logger.Warn("admin auth degraded: missing POSTGRES_DSN or ADMIN_JWT_SECRET")
		return degraded(issuer)
	}

	pool, err := postgres.NewPool(context.Background(), cfg.PostgresDSN, cfg.Environment)
	if err != nil {
		logger.Error("admin auth degraded: pgx pool init failed", "err", err)
		return degraded(issuer)
	}

	store := postgres.NewStore(pool)
	hasher := crypto.NewPasswordHasher(cfg.BcryptCost)
	cipher, _ := crypto.NewAESGCM(crypto.DecodeKey(cfg.AESKey))
	auditSvc := auditapp.NewService(postgres.NewAuditRepository(pool), env)
	auditSink := auditapp.NewSinkAdapter(auditSvc, logger)

	var fc adminapp.FeishuClient
	if cfg.FeishuMock {
		fc = feishuAdapter{c: feishu.NewMockClient()}
	} else {
		fc = feishuAdapter{c: feishu.NewHTTPClient(cfg.FeishuAppID, cfg.FeishuAppSecret, cfg.FeishuRedirectURI)}
	}

	var cipherPort adminapp.Cipher
	if cipher != nil {
		cipherPort = cipher
	}

	authSvc := adminapp.NewAdminAuthService(adminapp.AuthDeps{
		Tx: store, Hasher: hasher, Issuer: issuer, Feishu: fc, Cipher: cipherPort, Audit: auditSink, Env: env,
	})
	userSvc := adminapp.NewAdminUserService(store, hasher, auditSink)
	roleSvc := adminapp.NewRoleService(store, auditSink)
	permSvc := adminapp.NewPermissionService(store, auditSink)

	handler := adminhttp.NewHandler(adminhttp.Deps{
		Auth: authSvc, Users: userSvc, Roles: roleSvc, Perms: permSvc, Env: env,
	})

	r := adminhttp.NewRouter(handler, issuer, env, logger, true, auditSvc)

	// game/account-auth 模块：审计 sink 接入统一 AuditService。
	gameSvc := gameapp.NewGameService(postgres.NewGameStore(pool), rand.Reader, auditSink, env)
	accountAuthSvc := accountauthapp.NewService(postgres.NewAccountAuthStore(pool), cipher, auditSink, time.Now)
	gameshttp.RegisterRoutes(r, gameshttp.NewHandler(gameSvc, env, accountAuthSvc), issuer, env, logger, true, auditSvc)

	// channel 模块：审计 sink 接入统一 AuditService。
	channelSvc := channelapp.NewChannelService(postgres.NewChannelStore(pool), time.Now, auditSink, env)
	channelshttp.RegisterRoutes(r, channelshttp.NewHandler(channelSvc, env), issuer, env, logger, true, auditSvc)
	audithttp.RegisterRoutes(r, audithttp.NewHandler(auditSvc), issuer, env, logger, true, auditSvc)

	return r
}

// ephemeralSecret 降级占位密钥（随机；进程内有效，无外部令牌能通过校验）。
func ephemeralSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// denyIssuer 在 issuer 构造失败时兜底：所有 Parse 失败（401），不签发。
type denyIssuer struct{}

func (denyIssuer) IssuePair(domainauth.AuthContext) (domainauth.TokenPair, error) {
	return domainauth.TokenPair{}, infrajwt.ErrInvalidToken
}
func (denyIssuer) Parse(string, string) (domainauth.Claims, error) {
	return domainauth.Claims{}, infrajwt.ErrInvalidToken
}
func (denyIssuer) AccessExpiry() time.Time { return time.Time{} }
