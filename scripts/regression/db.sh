#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)
cd "$ROOT"
MIGRATIONS="$ROOT/services/admin-api/migrations"

log "migrate up ($MIGRATIONS)"
run_migrate -path "$MIGRATIONS" -database "$DATABASE_URL" up

log "bootstrap env schemas (develop/sandbox/production)"
docker_compose exec -T postgres psql -U "$PGUSER" -d "$PGDATABASE" < "$DIR/bootstrap-env-schemas.sql"

# seed：000002 已是 seed 迁移；额外 fixtures（如有）按 env 灌入
for f in "$ROOT"/tests/fixtures/common/*.sql; do
  [ -e "$f" ] || continue
  log "seed common: $(basename "$f")"
  docker_compose exec -T postgres psql -U "$PGUSER" -d "$PGDATABASE" < "$f"
done
for f in "$ROOT"/tests/fixtures/sandbox/*.sql; do
  [ -e "$f" ] || continue
  log "seed sandbox: $(basename "$f")"
  docker_compose exec -T postgres psql -U "$PGUSER" -d "$PGDATABASE" < "$f"
done
for f in "$ROOT"/tests/fixtures/production/*.sql; do
  [ -e "$f" ] || continue
  log "seed production: $(basename "$f")"
  docker_compose exec -T postgres psql -U "$PGUSER" -d "$PGDATABASE" < "$f"
done
log "db ready (migrated + seeded)"
