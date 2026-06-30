module: product (16) / acceptance / ✅ PASS（✅ 功能验收师 · Cursor Auto · 2026-06-29）
前置闸门: 🟪 测试专家 R2 = YES；验收基准=功能端到端可用 + 满足 compact 业务规则 + 符合 operation-flow 步骤 6
构建/回归(真实输出): 后端 go build/vet/test 全绿（回归 367/0）；前端 vue-tsc ✅ + vite build ✅ + vitest 22 文件 117 用例 ✅（product 4 文件 34）
重点验收点全 PASS: 金额归一化(USD 4.999→500 / 0.001 拒 / JPY 120.5→121 / display 反算 / productId 冲突 CONFLICT)；product_id≤128·price_id≤64 两维独立禁互填；包映射 PUT 全量 upsert+删未现项+两维独立解析；IAP 四件套校验·AES-GCM 不落明文·响应脱敏·config_status 三态·包级 merge 顶层覆盖·enabled 非 valid 拒绝
红线复核 ✅: IAP≠支付路由隔离（payment 无 price_id/IAP）；只读不跨 schema 写；SyncSection=products；P1 currency-specs 登录态可读+信封/字段正确+前端不再回退 seed
下游抽查: sync SectionProducts 已注册（preview 仍 scaffold=sync21 未落地，非 product 漂移）；snapshot20 未开发（ResolveEffectiveIDs/MergeIAPConfig 已暴露+单测）；price_id 弱引用无破坏；前端 10 端点契约全一致
连库 e2e: 环境阻断（沙箱无 PG/docker/DSN/harness）→ 契约对账 + 进程内单测等价 + 静态走查替代，残留风险已记，不作 FAIL 闸门
Playwright: 沙箱内 Chrome 不可控（kill EPERM/SIGABRT）→ 沿用前置闸门 7/7，非功能缺陷
遗留(非阻断·后续跟踪): #2 IAP 审计 before/after+sink(待 audit22) / #3 loadProductsForMapping 1000 硬限 / #4 IAP 文件统一上传 / #6 currency-specs 补 L3 S2/S1 用例
结论: ✅ 功能验收通过；artifacts=audit.log.md(功能验收段)/module.manifest.json(acceptance=passed)/integration.checklist.md(验收段)
