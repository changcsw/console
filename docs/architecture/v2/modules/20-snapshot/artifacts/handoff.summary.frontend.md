module: snapshot / frontend-test / pass
worktree: /Users/csw/gitproject/console-snapshot (branch: codex/snapshot)
result: 通过（vitest 22 + Playwright 7 = 29 PASS / 0 FAIL；2 视觉基线通过）
tests: apps/admin-web/src/views/games/detail/__tests__/SnapshotTab.spec.ts（L4，22）; tests/frontend/e2e/snapshot.spec.ts（L5，7，mock 4 接口）
covered: 列表降序渲染/分页、生成、JSON 预览按 market 折叠+密文脱敏***、下载、发布二次确认(draft→published)、权限置灰、空/错/403 态、错误码 NOT_FOUND/VALIDATION_FAILED/VERSION_STATE_INVALID/CONFLICT 提示
baselines: tests/frontend/visual-baseline/snapshot.spec.ts-snapshots/{snapshot-tab,snapshot-tab-readonly}-chromium-darwin.png; screenshots/snapshot-json-preview.png
defects: 无疑似实现缺陷（无需回退前端开发）；沿用 CR 非阻断建议（download 裸 fetch / 空 markets 无 empty hint / download 若改二进制需调整预览）
env: worktree 在工作区根外，测试需关沙箱(all)运行 + pnpm install --force 物化依赖 + npm_config_verify_deps_before_run=false 规避 pnpm exec 误判
run: 前端两车道(开发/CR/测试)均✅，可进入 🟪测试专家集成（前置：后端测试亦✅）
artifacts: docs/architecture/v2/modules/20-snapshot/artifacts/{audit.log.md, module.manifest.json, integration.checklist.md}
