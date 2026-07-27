#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)
cd "$ROOT"
MIGRATIONS="$ROOT/services/admin-api/migrations"

seed_sql_file() {
  sql_file="$1"
  docker_compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$PGUSER" -d "$PGDATABASE" < "$sql_file"
}

log "migrate up ($MIGRATIONS)"
# migrate up 已包含正式 env schema bootstrap（000017：建 develop/sandbox/production + 业务表结构）。
run_migrate -path "$MIGRATIONS" -database "$DATABASE_URL" up

# seed：000002 已是 seed 迁移；额外 fixtures（如有）按 env 灌入
for f in "$ROOT"/tests/fixtures/common/*.sql; do
  [ -e "$f" ] || continue
  log "seed common: $(basename "$f")"
  seed_sql_file "$f"
done
# sandbox/game.sql 与 sandbox/channel.sql 须先于依赖 game 100001 的 fixtures 灌入（固定 id 渠道实例）
SANDBOX_FIX="$ROOT/tests/fixtures/sandbox"
for early in game.sql channel.sql; do
  f="$SANDBOX_FIX/$early"
  if [ -f "$f" ]; then
    log "seed sandbox (early): $early"
    seed_sql_file "$f"
  fi
done
for f in "$SANDBOX_FIX"/*.sql; do
  [ -e "$f" ] || continue
  base=$(basename "$f")
  if [ "$base" = "game.sql" ] || [ "$base" = "channel.sql" ]; then
    continue
  fi
  log "seed sandbox: $base"
  seed_sql_file "$f"
done
for f in "$ROOT"/tests/fixtures/production/*.sql; do
  [ -e "$f" ] || continue
  log "seed production: $(basename "$f")"
  seed_sql_file "$f"
done
log "db ready (migrated + seeded)"
