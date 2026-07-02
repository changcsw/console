#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)
REPORTS="$ROOT/tests/reports"
mkdir -p "$REPORTS"

log "backend: go test ./... (+ scenario harness)"
cd "$ROOT/services/admin-api"

if [ "${WITH_DB:-0}" = "1" ]; then
  export POSTGRES_DSN="${POSTGRES_DSN:-$DATABASE_URL}"
  export ADMIN_JWT_SECRET="${ADMIN_JWT_SECRET:-regression-scenario-jwt-secret}"
  export APP_ENV="${APP_ENV:-sandbox}"
fi

# 统一跑单元/集成/transport + scenario 入口测试，输出 json 行供汇总
if go test ./... -count=1 -json > "$REPORTS/backend-go-test.json" 2> "$REPORTS/backend-go-test.err"; then
  log "backend tests PASS"
else
  err "backend tests FAILED (see $REPORTS/backend-go-test.*)"
  exit 1
fi
