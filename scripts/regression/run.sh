#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)

WITH_DB="${WITH_DB:-1}"   # WITH_DB=0 跳过 PG/迁移（smoke 走进程内 httptest 不需库）
MODULE="${1:-all}"        # 预留：按模块过滤（模块场景落地后用）

log "=== regression start (module=$MODULE, with_db=$WITH_DB) ==="

if [ "$WITH_DB" = "1" ]; then
  log "using docker cli: $DOCKER_BIN"
  log "starting postgres"
  (cd "$ROOT" && docker_compose up -d postgres)
  wait_for_pg
  sh "$DIR/db.sh"
else
  warn "WITH_DB=0: skipping postgres/migrate/seed (in-process scenarios only)"
fi

backend_status=0; frontend_status=0
sh "$DIR/backend.sh"  || backend_status=$?
sh "$DIR/frontend.sh" || frontend_status=$?

if [ "$WITH_DB" = "1" ]; then
  log "stopping postgres"
  (cd "$ROOT" && docker_compose down)
fi

sh "$DIR/summarize.sh" || true

log "=== regression done (backend=$backend_status frontend=$frontend_status) ==="
[ "$backend_status" -eq 0 ] && [ "$frontend_status" -eq 0 ]
