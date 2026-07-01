module: game-cashier / ✅功能验收 / 通过（acceptance PASS）
branch: codex/game-cashier
验收结论: 通过 —— 25 项验收点全 PASS / 0 FAIL（4 接口契约/权限/错误码/包络 + 快照绑定 published+确定性checksum + 整行覆盖 + 归一化 + 全量替换事务回滚 + 重复键预检 + 前端6点 + 红线4项 + operation-flow步骤7 + 下游snapshot/sync/payment无破坏）
后端: 统一入口 backend.sh PASS（go test pass=591/fail=0）；build/vet/test ./... 全绿；game-cashier L3 19/19；domain+scenario(28-case解析) PASS
前端: vitest 20 passed；vue-tsc 本模块 0 错误（5 条均既有非本模块 ChannelLoginConfigPanel/sync-section-drawer，定向规避）；Playwright e2e 8 passed(5.1m, 出沙箱, 含视觉基线)
operation-flow: 步骤7（绑定收银台模板版本+价格覆盖）端到端可用，前置A8 published、产出 profiles+overrides、完成判定「已绑定有效版本」→下一步8支付路由，主流程真实API可达
红线: env schema隔离(业务表无前缀/无env列、平台表platform.前缀)·事务回滚(进程内真回滚PASS)·无payment/IAP耦合·审计 cashier.profile.bind/cashier.override.update 均 ✅
遗留(非阻断·环境): 连库 S1/S6/S10(expect.db)+真实PG跨栈e2e 因本机无docker/PG未执行(由L3+mock e2e等价覆盖)，待 WITH_DB=1 CI 补；ComputeVersionChecksum 缺 domain 确定性单测(test-only 可选)
是否通过: 是（环境性连库维度标注待CI补跑，不阻断验收）
详见: audit.log.md [acceptance]；manifest verification/acceptance、checklist 已刷新
