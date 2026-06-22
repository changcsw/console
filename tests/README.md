# tests — 跨栈测试与统一回归

本目录承载**跨栈 / 顶层**测试资产；单元/集成/组件测试就近放在各自代码侧（见 `docs/architecture/v2/03-testing.md`）。

- `backend/scenarios/*.yaml` — 后端接口场景矩阵 manifest（数据）。harness 代码在 `services/admin-api/internal/testkit/scenario`。
- `frontend/e2e/*.spec.ts` — Playwright 用例（真实页面截图 / trace）。
- `frontend/screenshots/` — 运行期采集截图（git 忽略）。
- `frontend/visual-baseline/` — 截图基线（git 跟踪）。
- `fixtures/{common,sandbox,production}/` — 统一 seed/fixtures（按 env 维度）。
- `reports/` — 测试产物：junit / html / summary（git 忽略）。

运行：`scripts/regression/run.sh`（全量）/ `backend.sh` / `frontend.sh`。
