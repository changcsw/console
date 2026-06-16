# Admin API

Go backend scaffold for the publishing management console.

## Layout

- `cmd/admin-api`: service entrypoint
- `internal/domain`: domain types and aggregates
- `internal/app`: DTOs and use case layer
- `internal/infra`: config and infrastructure adapters
- `internal/transport/http`: HTTP router and handlers
- `migrations`: SQL migrations

## Notes

- The current scaffold intentionally uses mostly standard-library patterns so the follow-up backend agent can choose concrete libraries with minimal rework.
- The schema draft is duplicated into `migrations/000001_init.up.sql` so migration work can start from this repo directly.

