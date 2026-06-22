#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)
cd "$ROOT/apps/admin-web"

log "frontend: vitest"
pnpm test

log "frontend: playwright e2e"
# 更新基线请单独跑 pnpm e2e:update
pnpm e2e
