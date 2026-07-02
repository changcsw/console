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
	cashierapp "github.com/csw/console/services/admin-api/internal/app/cashier"
	channelapp "github.com/csw/console/services/admin-api/internal/app/channel"
	channelloginapp "github.com/csw/console/services/admin-api/internal/app/channellogin"
	"github.com/csw/console/services/admin-api/internal/app/command"
	gameapp "github.com/csw/console/services/admin-api/internal/app/game"
	paymentapp "github.com/csw/console/services/admin-api/internal/app/payment"
	pluginapp "github.com/csw/console/services/admin-api/internal/app/plugin"
	productapp "github.com/csw/console/services/admin-api/internal/app/product"
	dashboardquery "github.com/csw/console/services/admin-api/internal/app/query/dashboard"
	snapshotapp "github.com/csw/console/services/admin-api/internal/app/snapshot"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/infra/config"
	"github.com/csw/console/services/admin-api/internal/infra/crypto"
	"github.com/csw/console/services/admin-api/internal/infra/feishu"
	fileinfra "github.com/csw/console/services/admin-api/internal/infra/file"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
	"github.com/csw/console/services/admin-api/internal/infra/persistence/postgres"
	adminhttp "github.com/csw/console/services/admin-api/internal/transport/http/admin"
	audithttp "github.com/csw/console/services/admin-api/internal/transport/http/audit"
	cashierhttp "github.com/csw/console/services/admin-api/internal/transport/http/cashier"
	channelshttp "github.com/csw/console/services/admin-api/internal/transport/http/channels"
	dashboardhttp "github.com/csw/console/services/admin-api/internal/transport/http/dashboard"
	gameshttp "github.com/csw/console/services/admin-api/internal/transport/http/games"
	paymenthttp "github.com/csw/console/services/admin-api/internal/transport/http/payment"
	snapshothttp "github.com/csw/console/services/admin-api/internal/transport/http/snapshot"
	syncapi "github.com/csw/console/services/admin-api/internal/transport/http/sync"
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
		secret = ephemeralSecret() // 仅降级占位：无外部令牌能通过校验
	}
	// degraded 构造降级路由（auth + games 路由形状仍挂载，受保护路由过 Authn 后由 RequireBackend 返回 503）。
	degraded := func(iss adminapp.TokenIssuer) chi.Router {
		r := adminhttp.NewRouter(adminhttp.NewHandler(adminhttp.Deps{Env: env}), iss, env, logger, false, nil)
		gameshttp.RegisterRoutes(r, gameshttp.NewHandler(nil, env), iss, env, logger, false, nil)
		channelshttp.RegisterRoutes(r, channelshttp.NewHandler(nil, env), iss, env, logger, false, nil)
		cashierhttp.RegisterRoutes(r, cashierhttp.NewHandler(nil), iss, env, logger, false, nil)
		paymenthttp.RegisterRoutes(r, paymenthttp.NewHandler(nil), iss, env, logger, false, nil)
		snapshothttp.RegisterRoutes(r, snapshothttp.NewHandler(nil), iss, env, logger, false, nil)
		syncapi.RegisterRoutes(r, syncapi.NewSectionSyncHandler(nil), iss, env, logger, false, nil)
		audithttp.RegisterRoutes(r, audithttp.NewHandler(nil), iss, env, logger, false, nil)
		dashboardhttp.RegisterRoutes(r, dashboardhttp.NewHandler(nil), iss, env, logger, false, nil)
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
	currencySvc := adminapp.NewCurrencySpecService(postgres.NewCurrencySpecRepo(pool))

	handler := adminhttp.NewHandler(adminhttp.Deps{
		Auth: authSvc, Users: userSvc, Roles: roleSvc, Perms: permSvc, Currency: currencySvc, Env: env,
	})

	r := adminhttp.NewRouter(handler, issuer, env, logger, true, auditSvc)

	gameSvc := gameapp.NewGameService(postgres.NewGameStore(pool), rand.Reader, auditSink, env)
	accountAuthSvc := accountauthapp.NewService(postgres.NewAccountAuthStore(pool), cipher, auditSink, time.Now)
	productStore := postgres.NewProductStore(pool)
	productSvc := productapp.NewProductService(productStore, auditSink, env, time.Now)
	iapSvc := productapp.NewIAPConfigService(productStore, cipher, fileinfra.NewLocalRefService(), auditSink, time.Now)
	gamesHandler := gameshttp.NewHandler(gameSvc, env, accountAuthSvc).WithProductServices(productSvc, iapSvc)
	gameshttp.RegisterRoutes(r, gamesHandler, issuer, env, logger, true, auditSvc)

	channelSvc := channelapp.NewChannelService(postgres.NewChannelStore(pool), time.Now, auditSink, env)
	channelLoginSvc := channelloginapp.NewService(postgres.NewChannelLoginStore(pool), cipher, nil, auditSink, time.Now, env)
	channelPluginSvc := pluginapp.NewService(postgres.NewPluginStore(pool), cipher, auditSink, env)
	channelshttp.RegisterRoutes(r, channelshttp.NewHandler(channelSvc, env, channelLoginSvc).WithPluginService(channelPluginSvc), issuer, env, logger, true, auditSvc)

	cashierSvc := cashierapp.NewService(postgres.NewCashierStore(pool), auditSink, time.Now)
	cashierhttp.RegisterRoutes(r, cashierhttp.NewHandler(cashierSvc), issuer, env, logger, true, auditSvc)

	paymentSvc := paymentapp.NewService(postgres.NewPaymentStore(pool), cipher, auditSink, time.Now)
	paymenthttp.RegisterRoutes(r, paymenthttp.NewHandler(paymentSvc), issuer, env, logger, true, auditSvc)
	snapshotSvc := snapshotapp.NewService(postgres.NewSnapshotStore(pool), paymentSvc, auditSink, time.Now)
	snapshothttp.RegisterRoutes(r, snapshothttp.NewHandler(snapshotSvc), issuer, env, logger, true, auditSvc)
	syncSvc := command.NewSectionSyncService(postgres.NewSyncStore(pool), auditSink, time.Now, cfg.JWTSecret)
	syncapi.RegisterRoutes(r, syncapi.NewSectionSyncHandler(syncSvc), issuer, env, logger, true, auditSvc)

	audithttp.RegisterRoutes(r, audithttp.NewHandler(auditSvc), issuer, env, logger, true, auditSvc)
	dashboardSvc := dashboardquery.NewQueryService(pool)
	dashboardhttp.RegisterRoutes(r, dashboardhttp.NewHandler(dashboardSvc), issuer, env, logger, true, auditSvc)

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
