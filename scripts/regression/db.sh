#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)
cd "$ROOT"
MIGRATIONS="$ROOT/services/admin-api/migrations"

# golang-migrate：本机需安装 `migrate`（brew install golang-migrate）
if ! command -v migrate >/dev/null 2>&1; then
  err "golang-migrate 'migrate' not installed; see https://github.com/golang-migrate/migrate"
  exit 1
fi

log "migrate up ($MIGRATIONS)"
migrate -path "$MIGRATIONS" -database "$DATABASE_URL" up

# seed：000002 已是 seed 迁移；额外 fixtures（如有）按 env 灌入
for f in "$ROOT"/tests/fixtures/common/*.sql; do
  [ -e "$f" ] || continue
  log "seed common: $(basename "$f")"
  docker compose exec -T postgres psql -U "$PGUSER" -d "$PGDATABASE" < "$f"
done
log "db ready (migrated + seeded)"
