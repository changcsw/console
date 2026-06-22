# Game Publishing Management Console

Monorepo scaffold for the game publishing management console.

## Structure

- [apps/admin-web](/Users/csw/gitproject/console/apps/admin-web): Vue 3 admin frontend scaffold
- [services/admin-api](/Users/csw/gitproject/console/services/admin-api): Go backend scaffold
- [docs/architecture](/Users/csw/gitproject/console/docs/architecture): architecture, schema, DTO, and execution docs
- [docs/architecture/zh-CN](/Users/csw/gitproject/console/docs/architecture/zh-CN): Chinese architecture and execution docs
- [docs/agents](/Users/csw/gitproject/console/docs/agents): direct agent prompt templates

## 测试与回归

- 全量回归：`sh scripts/regression/run.sh`（启动 Postgres → 迁移/seed → 后端 `go test ./...` + scenario harness → 前端 vitest + Playwright → 汇总 `tests/reports/summary.md`）。
- 快路径（无需 docker）：`WITH_DB=0 sh scripts/regression/run.sh`（仅进程内场景 + 前端）。
- 仅后端：`sh scripts/regression/backend.sh`；仅前端：`sh scripts/regression/frontend.sh`。
- 更新前端视觉基线：`cd apps/admin-web && pnpm e2e:update`。
- 测试体系契约见 [docs/architecture/v2/03-testing.md](/Users/csw/gitproject/console/docs/architecture/v2/03-testing.md)。

## Current Status

- Architecture documents are prepared.
- Backend scaffold is created but not compiled in this environment because `go` is not installed locally.
- Frontend scaffold is created but dependencies are not installed in this environment because `pnpm` is not installed locally.

## Next Steps

1. Install Go locally and run backend formatting and compile checks.
2. Install `pnpm` or switch to `npm`, then install frontend dependencies.
3. Hand the docs in `docs/architecture` and `docs/agents` to frontend and backend agents for parallel delivery.
