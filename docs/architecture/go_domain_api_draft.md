# Go Domain Model and API DTO Draft

This draft is designed for a Go backend with a clean separation between domain, application, transport, and persistence.

## Recommended Package Layout

```text
internal/
  app/
    command/
    query/
    dto/
  domain/
    admin/
    auth/
    game/
    channel/
    product/
    cashier/
    payment/
    sync/
    common/
  infra/
    persistence/postgres/
    crypto/
    file/
    fx/
  transport/
    http/
      admin/
      auth/
      games/
      channels/
      products/
      cashier/
      payment/
      sync/
```

## Shared Domain Types

```go
type Environment string

const (
    EnvDevelop    Environment = "develop"
    EnvSandbox    Environment = "sandbox"
    EnvProduction Environment = "production"
)

type LoginMode string

const (
    LoginModeChannelOnly   LoginMode = "channel_only"
    LoginModeAccountSystem LoginMode = "account_system"
)

type PaymentMode string

const (
    PaymentModeChannelOnly PaymentMode = "channel_only"
    PaymentModeHybrid      PaymentMode = "hybrid"
    PaymentModeCashierOnly PaymentMode = "cashier_only"
)

type ConfigStatus string

const (
    ConfigStatusEmpty   ConfigStatus = "empty"
    ConfigStatusInvalid ConfigStatus = "invalid"
    ConfigStatusValid   ConfigStatus = "valid"
)

type OverrideMode string

const (
    OverrideModeDefault  OverrideMode = "default"
    OverrideModeOverride OverrideMode = "override"
)

type FXSyncMode string

const (
    FXSyncModeManualConfirm FXSyncMode = "manual_confirm"
    FXSyncModeAutoApply     FXSyncMode = "auto_apply"
)
```

## Core Aggregates

### Game Aggregate

`game.Aggregate` should own:

- base game info
- markets
- legal links
- account auth configs
- game channels
- products
- cashier profile

Suggested shape:

```go
type Game struct {
    ID                int64
    GameID            string
    GameSecret        string
    Name              string
    Alias             string
    IconURL           string
    DefaultMarketCode string
    Status            string
    Markets           []GameMarket
    LegalLinks        []GameLegalLink
}
```

### Game Channel Aggregate

`channel.GameChannelAggregate` should own:

- selected channel
- channel policy snapshot
- channel login config
- channel IAP config
- packages

### Product Aggregate

`product.Aggregate` should own:

- logical product
- per-package product overrides
- per-package price id overrides

### Cashier Template Aggregate

`cashier.TemplateAggregate` should own:

- template
- versions
- rows
- FX sync runs

### Payment Routing Aggregate

`payment.RoutingAggregate` should own:

- pay ways
- providers
- merchant accounts
- route priority sets

### Sync Aggregate

`sync.Aggregate` should own:

- preview
- execute
- audit trail

## Domain Service Boundaries

Use focused services instead of a single large game service.

- `GameService`
- `GameChannelService`
- `AccountAuthService`
- `ChannelLoginService`
- `ProductService`
- `IAPConfigService`
- `CashierTemplateService`
- `GameCashierService`
- `PaymentRouteService`
- `ConfigSnapshotService`
- `SyncService`
- `AdminAuthService`

## HTTP API Draft

Prefix all APIs with `/api/admin`.

### Admin Auth

- `POST /api/admin/auth/login`
- `POST /api/admin/auth/feishu/callback`
- `GET /api/admin/me`

```json
POST /api/admin/auth/login
{
  "userName": "alice",
  "password": "secret"
}
```

```json
200 OK
{
  "accessToken": "jwt",
  "refreshToken": "jwt",
  "expiresAt": "2026-06-15T10:00:00Z",
  "user": {
    "userId": 1,
    "displayName": "Alice",
    "roles": ["admin"],
    "permissions": ["game.read", "game.write"]
  }
}
```

### Games

- `GET /api/admin/games`
- `POST /api/admin/games`
- `GET /api/admin/games/{gameId}`
- `PATCH /api/admin/games/{gameId}`
- `PUT /api/admin/games/{gameId}/markets`
- `PUT /api/admin/games/{gameId}/legal-links`

```json
POST /api/admin/games
{
  "name": "Project A",
  "alias": "pa",
  "defaultMarketCode": "GLOBAL",
  "iconUrl": "",
  "markets": ["GLOBAL", "JP"]
}
```

```json
GET /api/admin/games/{gameId}
{
  "gameId": "100001",
  "name": "Project A",
  "alias": "pa",
  "gameSecret": "masked",
  "defaultMarketCode": "GLOBAL",
  "status": "active",
  "markets": [
    { "marketCode": "GLOBAL", "isDefault": true, "enabled": true, "defaultLocale": "en-US" }
  ],
  "legalLinks": [
    {
      "scopeType": "default",
      "scopeValue": "*",
      "termsUrl": "https://...",
      "privacyUrl": "https://...",
      "deleteAccountUrl": "https://..."
    }
  ]
}
```

### Channels and Packages

- `GET /api/admin/games/{gameId}/channels`
- `POST /api/admin/games/{gameId}/channels`
- `GET /api/admin/game-channels/{gameChannelId}`
- `PATCH /api/admin/game-channels/{gameChannelId}`
- `POST /api/admin/game-channels/{gameChannelId}/packages`
- `GET /api/admin/game-channels/{gameChannelId}/packages`
- `PATCH /api/admin/channel-packages/{packageId}`

```json
POST /api/admin/games/{gameId}/channels
{
  "channelId": "google",
  "enabled": true,
  "remark": ""
}
```

```json
POST /api/admin/game-channels/{gameChannelId}/packages
{
  "packageCode": "google-jp",
  "packageName": "Google JP",
  "marketCode": "JP",
  "bundleId": "com.company.game.jp",
  "inheritChannelConfig": true,
  "enabled": true
}
```

### Account Auth

- `GET /api/admin/account-auth/types`
- `GET /api/admin/channels/{channelId}/account-auth-types`
- `PUT /api/admin/games/{gameId}/account-auth-configs`

```json
PUT /api/admin/games/{gameId}/account-auth-configs
{
  "items": [
    {
      "authTypeId": "guest",
      "enabled": true,
      "configJson": {},
      "configStatus": "valid"
    },
    {
      "authTypeId": "google",
      "enabled": true,
      "configJson": {
        "clientId": "xxx",
        "clientSecret": "xxx",
        "redirectUri": "https://..."
      },
      "configStatus": "valid"
    }
  ]
}
```

### Channel Login

- `GET /api/admin/game-channels/{gameChannelId}/login-config`
- `PUT /api/admin/game-channels/{gameChannelId}/login-config`

```json
PUT /api/admin/game-channels/{gameChannelId}/login-config
{
  "enabled": true,
  "configJson": {
    "appId": "xxx",
    "appSecret": "xxx"
  }
}
```

### Products and Package Overrides

- `GET /api/admin/games/{gameId}/products`
- `POST /api/admin/games/{gameId}/products`
- `PATCH /api/admin/products/{productId}`
- `GET /api/admin/channel-packages/{packageId}/products`
- `PUT /api/admin/channel-packages/{packageId}/products`

```json
POST /api/admin/games/{gameId}/products
{
  "productId": "gem_60",
  "productName": "60 Gems",
  "baseAmountMinor": 499,
  "baseCurrency": "USD",
  "priceId": "price_499",
  "enabled": true
}
```

```json
PUT /api/admin/channel-packages/{packageId}/products
{
  "items": [
    {
      "productId": "gem_60",
      "enabled": true,
      "productIdMode": "default",
      "productIdOverride": "",
      "priceIdMode": "override",
      "priceIdOverride": "price_jp_600"
    }
  ]
}
```

### IAP Config

- `GET /api/admin/game-channels/{gameChannelId}/iap-config`
- `PUT /api/admin/game-channels/{gameChannelId}/iap-config`
- `GET /api/admin/channel-packages/{packageId}/iap-override`
- `PUT /api/admin/channel-packages/{packageId}/iap-override`

### Cashier Template and FX Sync

- `GET /api/admin/cashier/templates`
- `POST /api/admin/cashier/templates`
- `GET /api/admin/cashier/templates/{templateId}`
- `POST /api/admin/cashier/templates/{templateId}/versions`
- `GET /api/admin/cashier/templates/{templateId}/versions/{version}/rows`
- `PUT /api/admin/cashier/templates/{templateId}/versions/{version}/rows`
- `POST /api/admin/cashier/templates/{templateId}/versions/{version}/publish`
- `POST /api/admin/cashier/templates/{templateId}/fx-sync/runs`
- `POST /api/admin/cashier/fx-sync-runs/{runId}/approve`

```json
POST /api/admin/cashier/templates
{
  "templateId": "global_default",
  "templateName": "Global Default",
  "fxSyncEnabled": true,
  "fxSyncMode": "manual_confirm",
  "fxSyncSchedule": "monthly"
}
```

### Game Cashier

- `GET /api/admin/games/{gameId}/cashier/profile`
- `PUT /api/admin/games/{gameId}/cashier/profile`
- `GET /api/admin/games/{gameId}/cashier/price-overrides`
- `PUT /api/admin/games/{gameId}/cashier/price-overrides`

### Pay Ways, Providers, Subjects, Merchant Accounts, Routes

- `GET /api/admin/pay-ways`
- `GET /api/admin/cashier/providers`
- `GET /api/admin/billing-subjects`
- `POST /api/admin/billing-subjects`
- `GET /api/admin/cashier/merchant-accounts`
- `POST /api/admin/cashier/merchant-accounts`
- `GET /api/admin/games/{gameId}/payment-routes`
- `PUT /api/admin/games/{gameId}/payment-routes`

```json
PUT /api/admin/games/{gameId}/payment-routes
{
  "items": [
    {
      "marketCode": "GLOBAL",
      "countryCode": "US",
      "currency": "USD",
      "channelId": "google",
      "packageCode": "google-global",
      "payWayId": "credit_card",
      "providerId": "airwallex",
      "merchantAccountId": "merchant_aw_main",
      "priority": 10,
      "enabled": true
    }
  ]
}
```

### Config Snapshots

- `POST /api/admin/games/{gameId}/config-snapshots/generate`
- `GET /api/admin/games/{gameId}/config-snapshots`
- `POST /api/admin/game-config-snapshots/{snapshotId}/publish`
- `GET /api/admin/game-config-snapshots/{snapshotId}/download`

### Sandbox to Production Sync

- `POST /api/admin/games/{gameId}/sync/preview`
- `POST /api/admin/games/{gameId}/sync/execute`
- `GET /api/admin/games/{gameId}/sync-jobs`

```json
POST /api/admin/games/{gameId}/sync/preview
{
  "includeDeletes": false
}
```

```json
200 OK
{
  "gameId": "100001",
  "sourceEnv": "sandbox",
  "targetEnv": "production",
  "sourceHash": "sha256-a",
  "targetHashBefore": "sha256-b",
  "hasDiff": true,
  "sections": [
    {
      "section": "products",
      "changes": [
        {
          "op": "update",
          "entityType": "product",
          "entityKey": "gem_60",
          "fieldName": "price_id",
          "sandboxValue": "price_499",
          "productionValue": "price_599",
          "masked": false
        }
      ]
    }
  ]
}
```

## DTO and Validation Rules

### Template Fields

All `*_templates` tables should drive frontend forms with a shared structure:

- `form_schema_json`: renderable fields and components
- `secret_fields_json`: fields requiring encrypted storage and masked responses
- `file_fields_json`: upload slots and file restrictions
- `validation_rules_json`: server-side and client-side validation hints

### Currency Normalization

Any write path touching amounts must:

1. load `currency_specs`
2. validate fractional precision against `decimal_places`
3. validate lower bound against `min_amount_minor`
4. normalize by `rounding_mode`
5. store normalized integer minor units

## Suggested Repository Interfaces

```go
type GameRepository interface {
    Create(ctx context.Context, game *Game) error
    Update(ctx context.Context, game *Game) error
    FindByGameID(ctx context.Context, gameID string) (*Game, error)
    List(ctx context.Context, filter GameListFilter) ([]Game, error)
}

type SyncRepository interface {
    CreateJob(ctx context.Context, job *SyncJob) error
    AddItems(ctx context.Context, jobID int64, items []SyncJobItem) error
    ListJobsByGame(ctx context.Context, gameID int64) ([]SyncJob, error)
}
```

Keep repositories narrow. Cross-table orchestration belongs in application services, not in repositories.
