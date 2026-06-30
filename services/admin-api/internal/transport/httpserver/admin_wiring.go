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
	channelapp "github.com/csw/console/services/admin-api/internal/app/channel"
	gameapp "github.com/csw/console/services/admin-api/internal/app/game"
	productapp "github.com/csw/console/services/admin-api/internal/app/product"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
	"github.com/csw/console/services/admin-api/internal/infra/config"
	"github.com/csw/console/services/admin-api/internal/infra/crypto"
	"github.com/csw/console/services/admin-api/internal/infra/feishu"
	fileinfra "github.com/csw/console/services/admin-api/internal/infra/file"
	infrajwt "github.com/csw/console/services/admin-api/internal/infra/jwt"
	"github.com/csw/console/services/admin-api/internal/infra/persistence/postgres"
	adminhttp "github.com/csw/console/services/admin-api/internal/transport/http/admin"
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
		r := adminhttp.NewRouter(adminhttp.NewHandler(adminhttp.Deps{Env: env}), iss, env, logger, false)
		gameshttp.RegisterRoutes(r, gameshttp.NewHandler(nil, env), iss, env, logger, false)
		channelshttp.RegisterRoutes(r, channelshttp.NewHandler(nil, env), iss, env, logger, false)
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
		Tx: store, Hasher: hasher, Issuer: issuer, Feishu: fc, Cipher: cipherPort, Audit: nil, Env: env,
	})
	userSvc := adminapp.NewAdminUserService(store, hasher, nil)
	roleSvc := adminapp.NewRoleService(store, nil)
	permSvc := adminapp.NewPermissionService(store, nil)
	// 平台级币种字典只读服务：读 platform.currency_specs（全 env 共享），供 /system/currency-specs。
	currencySvc := adminapp.NewCurrencySpecService(postgres.NewCurrencySpecRepo(pool))

	handler := adminhttp.NewHandler(adminhttp.Deps{
		Auth: authSvc, Users: userSvc, Roles: roleSvc, Perms: permSvc, Currency: currencySvc, Env: env,
	})

	r := adminhttp.NewRouter(handler, issuer, env, logger, true)

	// game 模块：真实 GameService（绑定主连接池，env 由 search_path 钉死）。审计 sink 待 audit 模块落地后注入。
	gameSvc := gameapp.NewGameService(postgres.NewGameStore(pool), rand.Reader, nil, env)
	// account-auth 模块：service 层审计调用已接好（写 game.account_auth.update）；
	// audit sink 与 game/channel 一致暂注入 nil（audit 模块 22 落地后统一接通，非本模块新增遗留）。
	accountAuthSvc := accountauthapp.NewService(postgres.NewAccountAuthStore(pool), cipher, nil, time.Now)
	productStore := postgres.NewProductStore(pool)
	productSvc := productapp.NewProductService(productStore, nil, env, time.Now)
	iapSvc := productapp.NewIAPConfigService(productStore, cipher, fileinfra.NewLocalRefService(), nil, time.Now)
	gamesHandler := gameshttp.NewHandler(gameSvc, env, accountAuthSvc).WithProductServices(productSvc, iapSvc)
	gameshttp.RegisterRoutes(r, gamesHandler, issuer, env, logger, true)

	// channel 模块：真实 ChannelService（绑定主连接池，env 由 search_path 钉死）。审计 sink 待 audit 模块落地后注入。
	channelSvc := channelapp.NewChannelService(postgres.NewChannelStore(pool), time.Now, nil, env)
	channelshttp.RegisterRoutes(r, channelshttp.NewHandler(channelSvc, env), issuer, env, logger, true)

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
