# Integration Checklist (Frontend) · module 23 dashboard

- [x] Dashboard route kept at `/dashboard`, component remains `views/dashboard/DashboardView.vue` entry.
- [x] Shared surface changed: `apps/admin-web/src/router/routes.ts` added route permission `dashboard.read` (needs integration alignment with backend auth seed/permission list).
- [x] API client added: `apps/admin-web/src/api/modules/dashboard.ts`.
  - `GET /api/admin/dashboard/summary` with params `range/withTopItems/topN` (based on compact contract).
- [x] View layer implemented:
  - `views/dashboard/index.vue` (5 cards, range persistence, four-state behavior, navigation query passthrough).
  - `views/dashboard/components/DashboardMetricCard.vue` (card shell for metric rendering and topItems expansion).
  - `views/dashboard/DashboardView.vue` now delegates to `index.vue` to keep existing route reference stable.
- [x] Permissions behavior:
  - Route-level `dashboard.read` guard via router meta.
  - Metric-level `permitted=false` cards hidden.
- [x] Known gaps / follow-up:
  - Dictionary store currently has no dashboard enum map; source/status display text uses local formatter in view.
  - Card C byStatus buckets are display-only (optional per-bucket navigation with `status` query not implemented; acceptable for MVP).
- [x] Frontend CR (2026-07-01): **通过** — fixed expandable-details gate (`expandable` tied to count>0, not empty topItems); `npm run build` pass.
- [x] Frontend Test (2026-07-01):
  - Vitest: `DashboardMetricCard.spec.ts` + `DashboardView.spec.ts` = pass（12 tests）。
  - Playwright: `tests/frontend/e2e/dashboard.spec.ts` = fail（/dashboard 首屏被路由守卫导向 403，疑似权限回填时序缺陷）。
- [x] Frontend Fix (2026-07-01) · 根因归属【产品缺陷 + 用例 setup】：
  - **产品缺陷（共享面）**：`apps/admin-web/src/main.ts` 在挂载前用持久会话 `auth.user` 同步回填 `permission` store，
    使路由首个 `beforeEach` 在权限就绪后再判定 `meta.perm`；此前权限仅由异步 `loadMe()` 回填，导致直连/刷新任一受保护路由
    （不止 /dashboard）被误判 403。**这是全局 auth/permission 引导修复，改善而非改变既有受保护路由行为**（各路由 perm 语义不变）。
  - **共享面回归防护**：`/dashboard` 现要求 `dashboard.read`（本模块上一轮加，符合 compact），而既有 11 个模块 e2e 均以 `/dashboard`
    作跳板但会话未带 `dashboard.read`。已为这 11 个 spec 的持久 SESSION 补 `dashboard.read`（compact §关键假设：dashboard.read「默认拥有」，
    补齐即对齐既有会话约定）。文件：audit/cashier/channel-login/channels/feature-plugin/game-cashier/games/payment/product/snapshot/sync.spec.ts。
  - **用例 setup**：`dashboard.spec.ts` 的 `.env-badge` 选择器未限定作用域（AdminLayout 顶栏 + dashboard 工具栏各一个），改为 `.dashboard-toolbar .env-badge`；并生成缺失的视觉基线 `visual-baseline/dashboard.spec.ts-snapshots/dashboard-main-chromium-darwin.png`。
  - **防御性渲染**：`views/dashboard/index.vue` 5 张卡 v-if 改用更深可选链（`summary?.x?.permitted`），避免跳板模块以 `{data:{}}` stub 命中 /dashboard 时报错。
  - 验证：vitest 302/302；`dashboard.spec.ts` 5/5；跳板代表 `audit.spec.ts` 9/9；`npm run build` pass。
