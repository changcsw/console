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
MIGRATE_BIN="${MIGRATE_BIN:-migrate}"
MIGRATE_GO_VERSION="${MIGRATE_GO_VERSION:-v4.19.0}"
MIGRATE_GO_TAGS="${MIGRATE_GO_TAGS:-postgres file}"

default_docker_bin() {
  user_arm_docker="${HOME}/Applications/Docker-arm64.app/Contents/Resources/bin/docker"
  system_docker=$(command -v docker 2>/dev/null || true)

  if [ "$(uname -s)" = "Darwin" ] && [ "$(uname -m)" = "arm64" ] && [ -x "$user_arm_docker" ]; then
    if [ -z "$system_docker" ] || file "$system_docker" 2>/dev/null | grep -q "x86_64"; then
      printf '%s\n' "$user_arm_docker"
      return 0
    fi
  fi

  if [ -n "$system_docker" ]; then
    printf '%s\n' "$system_docker"
    return 0
  fi

  printf '%s\n' docker
}

DOCKER_BIN="${DOCKER_BIN:-$(default_docker_bin)}"

has_command() {
  command -v "$1" >/dev/null 2>&1
}

has_executable() {
  [ -x "$1" ] || has_command "$1"
}

docker_compose() {
  "$DOCKER_BIN" compose "$@"
}

run_migrate() {
  if has_executable "$MIGRATE_BIN"; then
    "$MIGRATE_BIN" "$@"
    return 0
  fi
  if has_command go; then
    log "golang-migrate not found; using 'go run' fallback (${MIGRATE_GO_VERSION}, tags=${MIGRATE_GO_TAGS})"
    go run -tags "$MIGRATE_GO_TAGS" "github.com/golang-migrate/migrate/v4/cmd/migrate@${MIGRATE_GO_VERSION}" "$@"
    return 0
  fi
  err "golang-migrate 'migrate' not installed and 'go' not available"
  return 1
}

wait_for_pg() {
  log "waiting for postgres at ${PGHOST}:${PGPORT} ..."
  i=0
  while [ "$i" -lt 60 ]; do
    if docker_compose exec -T postgres pg_isready -U "$PGUSER" -d "$PGDATABASE" >/dev/null 2>&1; then
      log "postgres ready"; return 0
    fi
    i=$((i+1)); sleep 1
  done
  err "postgres not ready after 60s"; return 1
}
