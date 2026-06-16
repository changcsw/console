# Management Console Architecture Pack

This folder is the current handoff pack for building the game publishing management console.

## Files

- [postgresql_ddl_draft.sql](/Users/csw/gitproject/console/docs/architecture/postgresql_ddl_draft.sql): PostgreSQL-first schema draft
- [go_domain_api_draft.md](/Users/csw/gitproject/console/docs/architecture/go_domain_api_draft.md): Go domain model, service boundaries, and HTTP DTO draft
- [backend_agent_execution.md](/Users/csw/gitproject/console/docs/architecture/backend_agent_execution.md): backend implementation checklist for an agent
- [frontend_agent_execution.md](/Users/csw/gitproject/console/docs/architecture/frontend_agent_execution.md): frontend implementation checklist for an agent
- [zh-CN/README.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/README.md): Chinese version of the architecture pack

## Scope

This pack assumes:

- Frontend: `pure-admin-thin` base, `Vue 3 + Vite + TypeScript + Pinia + Vue Router + Element Plus`
- Backend: Go service with PostgreSQL
- Environments: `develop`, `sandbox`, `production`
- `sandbox -> production` sync is online diff-and-confirm sync, not export/import
- Login is split into:
  - admin login for the console itself
  - player login/account configuration for each game/channel
- Payment is split into:
  - channel payment / IAP
  - cashier payment routing

## PostgreSQL vs MySQL

The SQL file targets PostgreSQL 15+ on purpose because:

- `JSONB` is more ergonomic for template/config payloads
- `BIGSERIAL` and `TIMESTAMPTZ` are convenient defaults
- check constraints are stronger

If a MySQL version is needed later, the main mapping is straightforward:

- `BIGSERIAL` -> `BIGINT AUTO_INCREMENT`
- `JSONB` -> `JSON`
- `TIMESTAMPTZ` -> `TIMESTAMP`
- `CHECK` constraints -> keep in service validation if the MySQL flavor is permissive

## Recommended Build Order

1. Backend agent starts with schema + migrations.
2. Backend agent implements core read/write APIs for games, channels, products, cashier templates, and sync preview.
3. Frontend agent builds shell, route guards, environment indicator, and game management pages first.
4. Frontend agent integrates APIs incrementally in the same order as backend delivery.
5. Both agents align on DTOs in `go_domain_api_draft.md` before coding generated clients or form schemas.
