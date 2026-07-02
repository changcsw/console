# Dashboard(#23) Integration Handoff（🟧 修复回填）

- **D-1（已修，唯一修项）**：`fxReview.topItems[].templateId` 前端类型 `number` → `string`。
- 根因：来源 `platform.cashier_price_templates.template_id VARCHAR(64)` 为业务码，后端 DTO 已 `string`，compact 示例 `7` 误导致前端误声明 `number`；裁决以后端/DB `string` 统一。
- 改动文件:行：`apps/admin-web/src/api/modules/dashboard.ts:27`（type）、`views/dashboard/__tests__/fixtures.ts:19`、`views/dashboard/__tests__/DashboardView.spec.ts:171`（夹具改字符串码）。后端无改动（DTO 本就 string）。
- 顺带核对 `fxReview.topItems` 其它字段（runId/templateName/triggeredAt）无漂移。
- 验证：vitest **302/302**；`npm run build`（含 vue-tsc）**pass**；后端未改故不重跑 go build/vet；e2e 未涉 templateId 断言/渲染，沿用 🟪 上轮 dashboard 5/5。
- **P-1 不在本次范围**：games/game-cashier 详情类 e2e 为既有问题（base 复跑同样失败），交 games 模块处理。
- 结论：D-1 已闭环，dashboard 契约前后端 + DB 一致，**可进入 ✅ 功能验收**（功能验收师全量复验）。
- 详见 `artifacts/audit.log.md`（D-1 修复段）。
