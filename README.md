# Game Publishing Management Console

Monorepo scaffold for the game publishing management console.

## Structure

- [apps/admin-web](/Users/csw/gitproject/console/apps/admin-web): Vue 3 admin frontend scaffold
- [services/admin-api](/Users/csw/gitproject/console/services/admin-api): Go backend scaffold
- [docs/architecture](/Users/csw/gitproject/console/docs/architecture): architecture, schema, DTO, and execution docs
- [docs/architecture/zh-CN](/Users/csw/gitproject/console/docs/architecture/zh-CN): Chinese architecture and execution docs
- [docs/agents](/Users/csw/gitproject/console/docs/agents): direct agent prompt templates

## 测试与回归

- 全量回归：`sh scripts/regression/run.sh`（启动 Postgres → 迁移/seed → 后端 `go test ./...` + scenario harness → 前端 vitest + Playwright → 汇总 `tests/reports/summary.md`）。若宿主存在多个 Docker 安装，可用 `DOCKER_BIN=/path/to/docker sh scripts/regression/run.sh` 覆盖 CLI。
- 快路径（无需 docker）：`WITH_DB=0 sh scripts/regression/run.sh`（仅进程内场景 + 前端）。
- 仅后端：`sh scripts/regression/backend.sh`；仅前端：`sh scripts/regression/frontend.sh`。
- 首次安装 Playwright 浏览器：`cd apps/admin-web && pnpm e2e:install`。
- 更新前端视觉基线：`cd apps/admin-web && pnpm e2e:update`。
- 测试体系契约见 [docs/architecture/v2/03-testing.md](/Users/csw/gitproject/console/docs/architecture/v2/03-testing.md)。

## Current Status

- v2 全部 14 个业务模块（#10–#23）已合并至 `main`。
- 全量回归：`sh scripts/regression/run.sh`（需 Docker；启动 Postgres → 迁移 → env schema bootstrap → seed → 后端 + 前端）。
- 快路径（无 Docker）：`WITH_DB=0 sh scripts/regression/run.sh`。
- 连库 scenario harness：`SCENARIO_WITH_DB=1` + `POSTGRES_DSN`/`ADMIN_JWT_SECRET`/`APP_ENV=sandbox`；当前 CI 已覆盖 dashboard 模块 scenario；全量 requiresDB 用例随 fixture 补齐逐步启用。
- CI：`.github/workflows/regression.yml`（backend+PG、vitest、Playwright e2e）。

## Next Steps

1. 补齐 `develop/sandbox/production` 正式 bootstrap 迁移（替代回归脚本中的 schema 克隆）。
2. 按模块启用 `SCENARIO_WITH_DB=1` 全量 scenario 矩阵（需 fixtures 与跨模块样本对齐）。
3. 处理跨模块遗留：games 详情 e2e（P-1）、audit sink 统一注入、feature-plugin P2 等。
