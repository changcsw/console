#!/usr/bin/env sh
# shellcheck shell=sh
set -eu

log()  { printf '\033[1;34m[regression]\033[0m %s\n' "$1"; }
warn() { printf '\033[1;33m[regression]\033[0m %s\n' "$1"; }
err()  { printf '\033[1;31m[regression]\033[0m %s\n' "$1" >&2; }

repo_root() {
  d=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
  while [ "$d" != "/" ]; do
    if [ -d "$d/tests/backend/scenarios" ]; then printf '%s\n' "$d"; return 0; fi
    d=$(dirname -- "$d")
  done
  err "repo root not found (missing tests/backend/scenarios)"; return 1
}

# DB 连接（与 docker-compose.yml 一致）
export PGHOST="${PGHOST:-127.0.0.1}"
export PGPORT="${PGPORT:-55432}"
export PGUSER="${PGUSER:-admin}"
export PGPASSWORD="${PGPASSWORD:-admin}"
export PGDATABASE="${PGDATABASE:-admin_console}"
export DATABASE_URL="${DATABASE_URL:-postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/${PGDATABASE}?sslmode=disable}"

wait_for_pg() {
  log "waiting for postgres at ${PGHOST}:${PGPORT} ..."
  i=0
  while [ "$i" -lt 60 ]; do
    if docker compose exec -T postgres pg_isready -U "$PGUSER" -d "$PGDATABASE" >/dev/null 2>&1; then
      log "postgres ready"; return 0
    fi
    i=$((i+1)); sleep 1
  done
  err "postgres not ready after 60s"; return 1
}
