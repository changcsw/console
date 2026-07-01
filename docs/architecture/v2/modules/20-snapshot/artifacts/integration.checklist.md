# snapshot 集成清单

## 模块路由 / API / 页面入口
- 后端 API：
  - `POST /api/admin/games/{gameId}/config-snapshots/generate`
  - `GET /api/admin/games/{gameId}/config-snapshots`
  - `POST /api/admin/game-config-snapshots/{snapshotId}/publish`
  - `GET /api/admin/game-config-snapshots/{snapshotId}/download`
- 后端路由注册：`services/admin-api/internal/transport/http/snapshot/router.go`
- 后端接线入口：`services/admin-api/internal/transport/httpserver/admin_wiring.go`

## 引用模块 / 外部依赖 / 共享 surface
- 依赖模块（读取有效数据视图）：`game/channel/account-auth/channel-login/feature-plugin/product/game-cashier/payment`
- payment 解析契约：`PaymentRouteService.ResolveRoute(ctx, gameID, MatchInput) -> RouteTarget / NOT_FOUND`
- 共享 surface 变更：
  - `services/admin-api/internal/domain/snapshot/*`（runtime-surface）
  - `services/admin-api/internal/transport/http/snapshot/*`
  - `services/admin-api/internal/transport/httpserver/admin_wiring.go`

## 需要统一接入的共享文件
- 已改：`services/admin-api/internal/transport/httpserver/admin_wiring.go`（degraded + ready 两条链都挂 snapshot 路由）
- 下游 `sync` 模块需要复用：
  - `domain/snapshot` 的 canonical/hash/version 口径
  - `game_config_snapshots` 发布态读取逻辑

## 已知问题 / 待完善点
- 当前 `LoadValidData` 对各子模块“有效视图”采用聚合查询实现，若后续各模块暴露标准 query service，可替换为明确端口调用。
- 迁移前向执行在本地未接真实数据库进行 `migrate up` 实跑，仅完成 SQL 幂等实现与构建级验证。

## 集成步骤 / 验证命令 / 风险说明
- 步骤：
  1. 执行迁移 `000015_snapshot_schema`
  2. 确认 `admin_wiring.go` 装配已启用 snapshot service
  3. 通过管理员权限码测试四个接口
- 验证命令：
  - `cd services/admin-api && go build ./... && go vet ./...`
- 风险：
  - 运行态 payment route 按 market+payWay 解析，若后续引入 channel/package 维度输入，需同步扩展生成策略。
# 20-snapshot Integration Checklist

## Frontend

### 模块路由 / API / 页面入口
- 页面入口：`apps/admin-web/src/views/games/detail/GameDetailView.vue` 的「配置快照」Tab（`SnapshotTab`）。
- API（基于 compact 契约）：
  - `POST /api/admin/games/{gameId}/config-snapshots/generate`
  - `GET /api/admin/games/{gameId}/config-snapshots`
  - `POST /api/admin/game-config-snapshots/{snapshotId}/publish`
  - `GET /api/admin/game-config-snapshots/{snapshotId}/download`

### 引用模块 / 外部依赖 / 共享 surface
- 引用：`game.read` 权限（列表与下载）、`snapshot.generate`、`snapshot.publish`。
- 共享 surface：`views/games/detail`（未改路由与菜单）。
- 外部依赖：无新增第三方依赖。

### 需要统一接入的共享文件
- 本次未改 `apps/admin-web/src/router/routes.ts`、菜单与全局导航。
- 若后续将「配置快照」拆分为独立页面，再由集成阶段统一评估路由挂载。

### 已知问题 / 待完善点
- 无阻断问题（前端 CR 2026-07-01 已通过）。
- CR 已修复：JSON 预览 version 字段改用列表行 `configVersion`。
- `downloadGameSnapshot` 使用裸 fetch，未复用 `http.ts` 401 续期与 `X-Environment`（建议项，集成阶段可抽 helper）。
- `download` 接口当前按返回 JSON 解析预览；若后端改为仅流式二进制，前端需改为单独 preview 接口或调整解析逻辑。

### 集成步骤 / 验证命令 / 风险说明
- 步骤：
  1. 打开游戏详情页，切换至「配置快照」Tab。
  2. 验证生成、列表分页、JSON 预览、下载、发布（draft -> published）流程。
  3. 验证无权限时生成/发布按钮置灰、403 显示权限态。
- 验证命令：
  - `pnpm --dir "/Users/csw/gitproject/console-snapshot/apps/admin-web" exec vue-tsc --noEmit`
  - `pnpm --dir "/Users/csw/gitproject/console-snapshot/apps/admin-web" exec vite build`
- 风险说明：依赖后端 `download` 响应与 compact 保持一致（JSON + Content-Disposition）。

## Backend Code Review（2026-07-01）

- CR 结论：**通过**（无阻断项）。
- CR 直接修复：`Publish` 并发竞态下 UPDATE 0 行时由 NOT_FOUND 纠正为 VERSION_STATE_INVALID（`service.go`）。
- 构建验证：`go build ./...` / `go vet ./...` / `go test ./internal/domain/snapshot/...` 均 PASS。
- 待集成：`migrate up 000015` 实跑、四接口权限矩阵联调。

## Backend Test（2026-07-01）

- 新增测试：`internal/domain/snapshot/{merge_matrix,canonical_hash_matrix}_test.go`（L1 纯函数）、`internal/app/snapshot/service_test.go`（L2-lite，内存 fake 无 IO）、`tests/backend/scenarios/snapshot.yaml`（L3 场景矩阵 25 例）、`tests/fixtures/sandbox/snapshot.sql`（连库 fixtures）。
- 结论：**通过**，无疑似实现缺陷。`go test ./...` exit 0（34 PASS / 0 FAIL / 21 连库用例 SKIP 待 PG CI）。
- 统一回归入口：domain/app 测试随 `go test ./...` 自动纳入；`snapshot.yaml` 随 `tests/backend/scenarios/*.yaml` 被 scenario harness 自动收集；`snapshot.sql` 随 `tests/fixtures/sandbox/*.sql` 被回归脚本 seed。
- **连库 CI 待办**：`SCENARIO_WITH_DB=1` + `migrate up 000015` + 上游模块 sandbox fixtures 组合后执行 S1/S3/S4/S5/S6/S7/S8/S9/S10；新增 RBAC 角色 `snapshot_admin`（snapshot.generate+snapshot.publish+game.read）需随 auth seed/装配补齐。
- 新增测试文件均在本模块 code_paths 与统一 tests/ 目录内，未改共享集成点。

## Frontend Test（2026-07-01）

- 新增测试：`apps/admin-web/src/views/games/detail/__tests__/SnapshotTab.spec.ts`（L4 vitest，22 例）、`tests/frontend/e2e/snapshot.spec.ts`（L5 Playwright，7 例，mock 4 接口）。
- 新增视觉基线（git 跟踪）：`tests/frontend/visual-baseline/snapshot.spec.ts-snapshots/{snapshot-tab,snapshot-tab-readonly}-chromium-darwin.png`；截图产物 `tests/frontend/screenshots/snapshot-json-preview.png`。
- 结论：**通过**（29 PASS / 0 FAIL），无疑似实现缺陷，无需回退前端开发。
- 统一回归入口：vitest 随 `pnpm test`（vitest run）自动收集 `src/**/*.spec.ts`；e2e 随 `playwright.config.ts` / `scripts/regression/frontend.sh` 收集 `tests/frontend/e2e/*.spec.ts`。
- **运行前置（CI/本地）**：worktree 需 `pnpm install`（git worktree 不拷贝 node_modules）；Playwright 需 `pnpm exec playwright install chromium` 或本机 Google Chrome（config 使用 `channel: chrome`）。worktree 若在 IDE 工作区根之外，运行测试需关闭沙箱写限制。
- 未改共享集成点（路由/菜单/装配）；沿用前端 CR 非阻断建议（download 裸 fetch / 空 markets 无 empty hint / download 若改二进制需调整预览解析），留待 🟪 集成阶段评估。

## 🟪 集成 / 系统测试（2026-07-01，Opus 4.8 High）

- **契约对账**：前端 `api/modules/snapshot.ts` vs 后端 4 接口（generate/list/publish/download）逐项核对 —— 方法/路径/DTO 字段/权限码/错误码/包络/附件文件名/脱敏 **全部一致，0 漂移**。重点项 generate 201 的 configVersion/fileHash/status/generatedAt、list items 字段、publish `VERSION_STATE_INVALID`(409)、download `Content-Disposition: attachment; filename` 与密文 `"***"` 均对齐。
- **集成 e2e / 回归**：后端 `go test ./...` 全绿；统一回归入口 `WITH_DB=0 scripts/regression/backend.sh` PASS；scenario 4 个 S2 鉴权 401 用例 in-process 通过；21 个连库用例（S1/S3/S4/S5/S6/S7/S8/S9/S10）SKIP。前端车道已 ✅ 29 PASS，集成阶段子 agent 重跑确认。
- **红线**：脱敏 `***`、scope 过滤、权限 403 与置灰、跨 env（业务表 SQL 无 schema 前缀/无 env 谓词、platform 表显式前缀、search_path 决定 schema）、事务回滚（InTx）、production 无可执行 Sync 捷径、审计 generate/publish 落 `platform.audit_logs` —— 代码级 + 单测等价覆盖，全部通过。
- **下游契约抽查**：sync（published+file_hash+config_version，确定性 I4 支撑 diff/去重）可消费；payment `ResolveRoute` per-game per-market 使用一致（已知维度限定：仅 market+payWay，channel/package/country/currency 后续扩展）；feature-plugin `ResolveRuntimeFlags` 三标（RuntimeConfig/Snapshot/Sync）同口径。
- **连库环境状态**：本机无 PostgreSQL、docker daemon 未运行 → 连库跨栈 e2e 与 21 连库场景**环境受限**（非阻断）。
- **CI 复现连库**：`docker compose up -d postgres` → `migrate ... up`（含 `000015_snapshot_schema`，已核对与 compact 一致）→ seed 上游 fixtures + `tests/fixtures/sandbox/snapshot.sql` → `SCENARIO_WITH_DB=1 scripts/regression/backend.sh`；需先在 auth seed/装配补齐角色 `snapshot_admin`/`game_reader`/`no_perm`。前端 e2e 需 `pnpm install` + `playwright install chromium`（或本机 Chrome）。
- **遗留问题清单（移交 🟧）**：**无阻断项**。仅非阻断建议（download 裸 fetch 未复用 401 续期/X-Environment；空 markets 无 empty hint；download 若改二进制需调整预览解析）。
- **通过判定**：**通过，可进入 ✅ 功能验收**（连库维度随 PG CI 补跑）。

## ✅ 功能验收（2026-07-01，Composer 2.5）

- **结论：通过（PASS）**。验收清单 **31/31 PASS**（A 四接口闭环 5 / B 业务规则 6 / C 权限错误码 4 / D 下游契约 3 / E 红线 5 / F 操作主线 3），无 snapshot 归属未达项。详见 `audit.log.md`「✅ 功能验收」章。
- **构建/测试**：后端 gofmt·build·vet·test PASS（snapshot 域+应用 76 子测试 0 FAIL）；前端 vue-tsc·vite build PASS；snapshot vitest 22 + Playwright e2e 7 = 29 PASS；统一回归 `WITH_DB=0 run.sh` 后端 backend=0 PASS。
- **回归发现（需集成/后续关注，非本模块阻断）**：前端全量 Playwright（跨 12 模块 90 例）出现 **8 例失败（games 7 + product 1）**，均为详情页 Tab 交互/权限置灰断言（toContainText/toBeVisible）。
  - **已定位为基线预存/环境态失败**：`git stash` 回退 `GameDetailView.vue` 到基线（无 SnapshotTab）后单独重跑 `games.spec.ts + product.spec.ts`，**同样 8 例失败（集合一致）** → 与 snapshot 改动无关（snapshot 对 GameDetailView 的变更纯增量：+tab-pane +import、移除同名下游占位）。
  - **移交**：🟪 测试专家 / games·product 模块负责人排查（疑本机 Chrome channel 环境态或基线 flaky；基线 b8fb3a9 已含 payment #19）。
- **连库维度**：无 PG / docker 未运行 → 后端连库 21 例（S1/S3-S10）+ 跨栈真实后端 e2e 环境受限（非阻断），沿用上文 CI 复现步骤随 PG 补跑。
- **下一步**：snapshot 验收通过；runtime-surface 内可继续 sync（#21）——请新开 Chat。
