# Backend Agent Execution List

This checklist is meant to be handed to a backend-focused agent and executed in order.

## Goal

Build the Go admin backend for the publishing console using PostgreSQL, with clean APIs for the Vue frontend and environment-aware `sandbox -> production` sync.

## Phase 0: Repo Bootstrap

- Create Go module and service layout under `cmd/` and `internal/`.
- Pick router, config loader, structured logger, migration tool, and DB access layer.
- Add `.env.example`, config loader, and environment selection for `develop`, `sandbox`, and `production`.
- Add DB migration runner and baseline health endpoint.

Done when:
- service boots locally
- config loads cleanly
- migration command runs successfully

## Phase 1: Schema and Migrations

- Translate [postgresql_ddl_draft.sql](/Users/csw/gitproject/console/docs/architecture/postgresql_ddl_draft.sql) into ordered migrations.
- Keep names exactly aligned with the draft.
- Add seed migrations for:
  - `channels`
  - `channel_policies`
  - `account_auth_types`
  - `pay_ways`
  - `cashier_providers`
  - `currency_specs`
- Seed common channels:
  - `google`
  - `apple`
  - `huawei_cn`
  - `xiaomi_cn`
  - `oppo_cn`
  - `vivo_cn`
  - `wechat_mini_game`
  - `douyin_mini_game`

Done when:
- a fresh database can migrate from zero
- seed data is idempotent

## Phase 2: Shared Domain and Persistence Layer

- Implement shared enums and value objects from [go_domain_api_draft.md](/Users/csw/gitproject/console/docs/architecture/go_domain_api_draft.md).
- Add repositories for games, channels, products, cashier, payment routing, config snapshots, sync jobs, and admin auth.
- Add a shared currency normalization helper backed by `currency_specs`.
- Add secret encryption/decryption abstraction for template secret fields and merchant credentials.

Done when:
- repositories cover all main aggregates
- currency normalization is unit tested
- secrets are never stored in plaintext

## Phase 3: Admin Auth and RBAC

- Implement admin password login.
- Implement Feishu admin identity callback flow.
- Implement `GET /api/admin/me`.
- Implement role/permission loading into auth context.
- Add middleware for permission checks and environment display context.

Done when:
- admin login works end to end
- Feishu admin login path is scaffolded even if credentials are mocked in local env
- permission middleware protects sample routes

## Phase 4: Game Core APIs

- Implement:
  - `GET /api/admin/games`
  - `POST /api/admin/games`
  - `GET /api/admin/games/{gameId}`
  - `PATCH /api/admin/games/{gameId}`
  - `PUT /api/admin/games/{gameId}/markets`
  - `PUT /api/admin/games/{gameId}/legal-links`
- Auto-generate:
  - `game_id`
  - `game_secret`
  - default `GLOBAL` market if not supplied
- Add validation for unique alias, market uniqueness, and scope uniqueness.

Done when:
- game create/update/list/detail works
- legal links support `default/market/locale` override

## Phase 5: Channel and Package APIs

- Implement:
  - `GET /api/admin/games/{gameId}/channels`
  - `POST /api/admin/games/{gameId}/channels`
  - `GET /api/admin/game-channels/{gameChannelId}`
  - `PATCH /api/admin/game-channels/{gameChannelId}`
  - `POST /api/admin/game-channels/{gameChannelId}/packages`
  - `GET /api/admin/game-channels/{gameChannelId}/packages`
  - `PATCH /api/admin/channel-packages/{packageId}`
- Enforce channel policy defaults on channel creation.
- For `login_mode=channel_only` and `payment_mode=channel_only`, initialize locked config state.

Done when:
- adding a channel creates a stable game-channel record
- package management works with inherit/override behavior

## Phase 6: Account Auth and Channel Login

- Implement account auth type list and per-channel allowed auth types.
- Implement:
  - `GET /api/admin/account-auth/types`
  - `GET /api/admin/channels/{channelId}/account-auth-types`
  - `PUT /api/admin/games/{gameId}/account-auth-configs`
  - `GET /api/admin/game-channels/{gameChannelId}/login-config`
  - `PUT /api/admin/game-channels/{gameChannelId}/login-config`
- Implement config validation using template metadata.
- Calculate and persist `config_status` and `last_check_message`.

Done when:
- self-account auth configs and channel-only login configs both validate through templates
- invalid configs surface status cleanly

## Phase 7: Products and IAP

- Implement:
  - `GET /api/admin/games/{gameId}/products`
  - `POST /api/admin/games/{gameId}/products`
  - `PATCH /api/admin/products/{productId}`
  - `GET /api/admin/channel-packages/{packageId}/products`
  - `PUT /api/admin/channel-packages/{packageId}/products`
  - `GET /api/admin/game-channels/{gameChannelId}/iap-config`
  - `PUT /api/admin/game-channels/{gameChannelId}/iap-config`
  - `GET /api/admin/channel-packages/{packageId}/iap-override`
  - `PUT /api/admin/channel-packages/{packageId}/iap-override`
- Resolve effective `product_id` and effective `price_id` via mode fields.
- Normalize `base_amount_minor` and IAP-related price inputs via `currency_specs`.

Done when:
- package-level overrides behave correctly
- both IAP config and product mapping APIs are stable

## Phase 8: Cashier Templates and FX Sync

- Implement cashier template CRUD and version lifecycle.
- Implement cashier row bulk upsert.
- Implement FX sync run generation with diff summary.
- Respect:
  - `fx_sync_enabled`
  - `fx_sync_mode`
  - `fx_sync_schedule`
- Default behavior must be reminder only, not auto-apply.

Done when:
- template versions can be created, edited, and published
- FX sync can create candidate versions and wait for manual approval

## Phase 9: Game Cashier and Payment Routing

- Implement:
  - `GET /api/admin/games/{gameId}/cashier/profile`
  - `PUT /api/admin/games/{gameId}/cashier/profile`
  - `GET /api/admin/games/{gameId}/cashier/price-overrides`
  - `PUT /api/admin/games/{gameId}/cashier/price-overrides`
  - `GET /api/admin/games/{gameId}/payment-routes`
  - `PUT /api/admin/games/{gameId}/payment-routes`
- Implement pay ways, providers, billing subjects, and merchant accounts reads/writes as needed by UI.
- Add route resolution logic:
  - exact package match
  - channel + market + country + currency match
  - game-level wildcard fallback
  - lowest priority number wins

Done when:
- a game can bind a cashier template and override specific prices
- a pay way can switch providers by changing route priority only

## Phase 10: Config Snapshots

- Implement:
  - `POST /api/admin/games/{gameId}/config-snapshots/generate`
  - `GET /api/admin/games/{gameId}/config-snapshots`
  - `POST /api/admin/game-config-snapshots/{snapshotId}/publish`
  - `GET /api/admin/game-config-snapshots/{snapshotId}/download`
- Generate final client JSON from effective game/channel/package/product/cashier config.
- Persist `config_json`, `file_hash`, and download metadata.

Done when:
- snapshot generation is deterministic
- published snapshot can be downloaded by ops

## Phase 11: Sandbox to Production Sync

- Implement:
  - `POST /api/admin/games/{gameId}/sync/preview`
  - `POST /api/admin/games/{gameId}/sync/execute`
  - `GET /api/admin/games/{gameId}/sync-jobs`
- Sync must compare sandbox and production snapshots by section.
- Mask secret fields in preview.
- Execute sync in ordered upsert phases.
- Re-check target hash before write.
- Write `sync_jobs`, `sync_job_items`, and `audit_logs`.

Done when:
- preview shows structured diffs
- execute applies confirmed changes safely
- deletes require explicit include flag

## Phase 12: Hardening

- Add unit tests for:
  - currency normalization
  - effective product/price resolution
  - payment route resolution
  - sync diff generation
- Add integration tests for major APIs.
- Add structured audit logging everywhere meaningful.
- Add OpenAPI generation or hand-maintained API schema for frontend alignment.

Done when:
- core flows are tested
- API schema is consumable by frontend

## Non-Negotiable Rules

- Do not merge admin auth with player login config.
- Do not merge channel payment config with cashier routing config.
- Do not bypass `currency_specs` in any amount write path.
- Do not let sync write directly without preview and target re-check.
- Do not store plaintext secrets.
