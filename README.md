# Game Publishing Management Console

Monorepo scaffold for the game publishing management console.

## Structure

- [apps/admin-web](/Users/csw/gitproject/console/apps/admin-web): Vue 3 admin frontend scaffold
- [services/admin-api](/Users/csw/gitproject/console/services/admin-api): Go backend scaffold
- [docs/architecture](/Users/csw/gitproject/console/docs/architecture): architecture, schema, DTO, and execution docs
- [docs/architecture/zh-CN](/Users/csw/gitproject/console/docs/architecture/zh-CN): Chinese architecture and execution docs
- [docs/agents](/Users/csw/gitproject/console/docs/agents): direct agent prompt templates

## Current Status

- Architecture documents are prepared.
- Backend scaffold is created but not compiled in this environment because `go` is not installed locally.
- Frontend scaffold is created but dependencies are not installed in this environment because `pnpm` is not installed locally.

## Next Steps

1. Install Go locally and run backend formatting and compile checks.
2. Install `pnpm` or switch to `npm`, then install frontend dependencies.
3. Hand the docs in `docs/architecture` and `docs/agents` to frontend and backend agents for parallel delivery.
