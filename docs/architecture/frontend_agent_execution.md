# Frontend Agent Execution List

This checklist is meant to be handed to a frontend-focused agent building on `pure-admin-thin`.

## Goal

Build a clean, maintainable admin console for game publishing operations using the backend contract in [go_domain_api_draft.md](/Users/csw/gitproject/console/docs/architecture/go_domain_api_draft.md).

## Phase 0: Shell Cleanup and Base Setup

- Initialize frontend from `pure-admin-thin`.
- Remove demo routes, demo pages, mock data, and unrelated showcase components.
- Keep:
  - auth shell
  - route guards
  - menu system
  - tabs
  - upload wrapper
  - permission directive/component
  - table and form primitives
- Add top-level environment indicator in the app chrome.
- Add base API client and auth interceptors.

Done when:
- app boots with a minimal shell
- current environment is visible globally
- no demo pages remain in active menus

## Phase 1: Route and State Architecture

- Build route groups:
  - `/dashboard`
  - `/games`
  - `/channels`
  - `/cashier`
  - `/audit`
  - `/system`
- Add Pinia stores:
  - `auth`
  - `permission`
  - `app`
  - `dictionary`
- Add typed API layer matching backend DTOs.

Done when:
- route structure is stable
- typed request/response modules exist

## Phase 2: Design System and Page Containers

- Create shared page shells:
  - list page
  - detail page
  - drawer form
  - diff drawer
  - status badge
  - masked secret display
- Add a restrained design token layer:
  - color
  - spacing
  - radius
  - typography
  - table density
- Keep logic and visual layers separate.

Done when:
- list/detail/drawer patterns are reusable
- styling changes stay mostly inside shared UI layers

## Phase 3: Game Management

- Build pages:
  - game list
  - create game drawer/page
  - game detail
  - markets tab
  - legal links tab
- Show generated `game_id` and masked `game_secret`.
- Default market handling should be clear in the UI.

Done when:
- games can be created and edited from the frontend
- legal links support default and override scopes

## Phase 4: Channel and Package Management

- In game detail, add tabs for:
  - channels
  - packages
  - products
  - account auth
  - channel login
  - IAP
  - cashier
  - sync history
- Build add-channel flow from seeded `channels`.
- When a channel is added, surface its policy:
  - `channel_only` or `account_system`
  - `channel_only` / `hybrid` / `cashier_only`
- Build package list and package editor with inherit/override behavior.

Done when:
- ops can understand a channel's login/payment mode directly from the page
- packages are manageable without navigating away from the game flow

## Phase 5: Account Auth and Channel Login UX

- Build self-account auth config page for games.
- Build channel-login config page for `channel_only` channels.
- Render forms dynamically from template metadata:
  - `form_schema_json`
  - `secret_fields_json`
  - `file_fields_json`
  - `validation_rules_json`
- Show `config_status` clearly:
  - empty
  - invalid
  - valid
- Add warnings if a config is enabled but not valid.

Done when:
- enabling guest, phone, email, Google, Apple, etc. works from one unified flow
- channel-only login configs have their own isolated form flow

## Phase 6: Products and IAP

- Build product list/editor in the game detail.
- Build package-level product mapping editor.
- Expose both override pairs cleanly:
  - `product_id_mode` + `product_id_override`
  - `price_id_mode` + `price_id_override`
- Build IAP config panel and package-level IAP override panel.

Done when:
- the UI makes the difference between IAP product ID and cashier price ID obvious
- package overrides are easy to review

## Phase 7: Cashier Management

- Build top-level cashier pages:
  - template list
  - template detail
  - version list
  - price matrix editor
  - FX sync review list
- Build game-level cashier tab:
  - applied template
  - template version
  - game overrides
- Honor `currency_specs` in all amount inputs:
  - decimal precision
  - minimum amount
  - rounding preview when relevant

Done when:
- price matrix editing is precise and guarded
- FX sync review is visible and requires approval by default

## Phase 8: Payment Routing

- Build pages/components for:
  - pay ways
  - cashier providers
  - billing subjects
  - merchant accounts
  - game payment routes
- Present routing as a priority list per pay way.
- Make fallback order obvious.
- Support package/channel/market/country/currency scoping in one editor without becoming confusing.

Done when:
- ops can re-route `credit_card` from one provider to another without ambiguity

## Phase 9: Config Snapshots and Sync

- Build snapshot list and publish flow in game detail.
- Add JSON preview and manual download entry.
- In sandbox only, show `Sync to Production` action.
- Build sync preview drawer:
  - section grouping
  - add/update/delete labels
  - masked secrets
  - confirm with optional include-deletes toggle
- Build sync history tab.

Done when:
- sync flow is safe and readable
- snapshot and sync concepts are visible but not overbearing

## Phase 10: Audit and Polish

- Build audit log page with filters:
  - env
  - action
  - resource type
  - operator
  - time range
- Add consistent empty, warning, and error states.
- Add permission gates to every mutating action.
- Review mobile layout enough to keep the console usable on narrower widths.

Done when:
- core actions are discoverable
- dangerous actions are visually distinct
- permission failures degrade gracefully

## Shared UI Rules

- Do not mix admin login UI with player login configuration UI.
- Do not hide invalid config states; surface them inline.
- Keep page interactions drawer-first where possible.
- Use one consistent pattern for template-driven forms.
- Keep environment context visible at all times.
- Keep sandbox-only sync action impossible to trigger in production views.

## High-Risk Areas to Treat Carefully

- Dynamic form rendering from backend templates
- Precision-safe money inputs driven by `currency_specs`
- Distinguishing:
  - `product_id`
  - `product_id_override`
  - `price_id`
  - `price_id_override`
- Explaining channel-only vs account-system login clearly
- Presenting payment routing priority without confusing operations users
