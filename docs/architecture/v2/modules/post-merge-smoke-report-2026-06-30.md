# 合并后冒烟报告 · #14/#16/#17/#22

> 🟪 测试专家 / 合并后集成验证 · 2026-06-30  
> 范围：main 已合并 cashier-template(#17)、channel-login(#14)、product(#16)、audit(#22)  
> migration 000007–000011

---

## 1. 验证命令与结果摘要

| # | 验证项 | 命令 | 结果 |
|---|--------|------|------|
| 1 | Migration 清单 | `ls services/admin-api/migrations/` | **PASS（附注）** |
| 2 | 后端构建 | `cd services/admin-api && go build ./...` | **PASS** |
| 3 | 后端测试 | `cd services/admin-api && go test ./...` | **PASS**（修复后 27 pkg / 0 FAIL） |
| 4 | 前端单测 | `cd apps/admin-web && pnpm test` | **PASS（附注）** 195/196 |
| 5 | 场景矩阵 | `go test ./internal/testkit/scenario/... -v` | **PASS** 四模块 24 PASS + 143 SKIP(requiresDB) |
| 6 | 共享接线 | 人工核对 `admin_wiring.go` / `routes.ts` | **PASS** |

---

## 2. Migration 链（000007–000011）

| 序号 | 文件 | up | down | 归属模块 |
|------|------|:--:|:--:|----------|
| 000007 | `cashier_template_platform_schema` | ✅ | ⚠️ **缺失** | cashier-template |
| 000008 | `channel_login_schema` | ✅ | ✅ | channel-login |
| 000009 | `product_schema` | ✅ | ✅ | product |
| 000010 | `audit_logs_platform_schema` | ✅ | ✅ | audit |
| 000011 | `audit_indexes` | ✅ | ✅ | audit |

- 序号 **无重复**，000007–000011 连续。
- **P3**：000007 仅有 `.up.sql`，无对称 `.down.sql`（其余四组 up/down 成对）。cashier 表结构回滚需手工或后续补 down。

---

## 3. 共享集成点核对

### 3.1 `admin_wiring.go`

| 模块 | 接线项 | 状态 |
|------|--------|------|
| audit | `auditSvc` + `auditSink` + 中间件 `auditSvc` + `audithttp.RegisterRoutes` | ✅ |
| cashier-template | `cashierSvc` + `auditSink` + `cashierhttp.RegisterRoutes(..., auditSvc)` | ✅ |
| product | `productSvc` / `iapSvc` + `auditSink` + `currencySvc` → `gameshttp.WithProductServices` | ✅ |
| channel-login | `channelLoginSvc` + `auditSink` + `channelshttp` 单点注册 login-config | ✅ |

合并冲突热点均已就位：`currency.go` 含 `NormalizeAmountToMinor`（cashier）与 `NormalizeMajorAmount`（product）双入口，无冲突残留。

### 3.2 `routes.ts`

| 模块 | 路由 | 状态 |
|------|------|------|
| cashier-template | `/cashier` | ✅ |
| audit | `/audit` | ✅ |
| channel-login | 复用 `/channels` → `ChannelInstanceDetailDrawer`「渠道登录」页签 | ✅ |
| product | 复用 `/games/:gameId` Tab + `ChannelPackageDetailDrawer` | ✅ |

`ChannelInstanceDetailDrawer.vue`：`v-if="detail.loginMode === 'channel_only'"` + `ChannelLoginConfigPanel` 已挂载。

---

## 4. 场景矩阵（四模块）

`SCENARIO_WITH_DB` 未设（默认 0）→ requiresDB 用例 SKIP，进程内鉴权/校验用例 PASS。

| manifest | PASS | SKIP(requiresDB) | 说明 |
|----------|------|------------------|------|
| `cashier-template.yaml` | 含于合计 | 含于合计 | L3 httptest 等价覆盖 |
| `channel-login.yaml` | 含于合计 | 含于合计 | channels/login_http_test 已绿 |
| `product.yaml` | 含于合计 | 含于合计 | games httptest + domain 单测 |
| `audit.yaml` | 含于合计 | 含于合计 | audit handler + middleware 单测 |

合计：**24 PASS / 143 SKIP / 0 FAIL**（manifest 解析有效）。

---

## 5. P0/P1 阻断项

### 5.1 发现与修复（已本地修复，待提交）

| 级别 | 问题 | 根因 | 修复 |
|------|------|------|------|
| **P1** | `go test ./...` 编译失败 | audit 合并后 `RegisterRoutes` 新增 `AuditWriter` 参数；cashier/channels httptest 未传第 7 参 | `cashier_http_test.go`、`login_http_test.go` 补 `nil` |
| **P1** | product/cashier 测试编译失败 | `AuditSink.Write` 签名改为返回 `error`；测试 spy 未同步 | `product/memstore_test.go`、`cashier/memstore_test.go` 的 `Write` 补 `return nil` |

修复后复跑：`go build ./...` ✅ · `go test ./...` ✅（27 ok / 0 fail）。

### 5.2 当前无未修复 P0/P1

---

## 6. P3 遗留对照 handoff（main 闭环状态）

| 模块 | handoff 遗留 | main 状态 |
|------|-------------|-----------|
| **channel-login** | AuditSink 注入 | ✅ **已闭环**（`admin_wiring.go:127` auditSink） |
| | file 上传占位 | ⏳ P3 未闭环（仍本地文件名引用） |
| | config_status 双轨 | ⏳ P3 未闭环（game_channels vs login-config 聚合待对齐） |
| | mapFormSchema options 未回传 | ⏳ P3（无 select 模板时不影响） |
| **product** | IAP 审计 before/after | ⏳ P3（sink 已注入，detail 无 before/after 快照） |
| | currency-specs 用例 | ✅ **路由已补** `GET /system/currency-specs`；⏳ L3 S2 连库用例待 harness |
| | loadProductsForMapping 1000 硬限 | ⏳ P3 |
| **cashier-template** | audit sink nil | ✅ **已闭环**（auditSink 注入） |
| | FX provider 占位 | ⏳ P3 |
| | 000007 无 down.sql | ⏳ P3（本报告 §2） |
| **audit** | 连库 harness / DSN 注入 | ⏳ P3（`SCENARIO_WITH_DB=1` 未启用） |
| | Playwright 基线 | ⏳ P3（`audit.spec.ts` 未建/未跑） |
| | 历史模块同事务审计回滚 | ⏳ P4 观察项 |

### 前端附注（非四模块阻断）

- `sync-section-drawer.spec.ts` 1/196 FAIL（games 脚手架用例，handoff 已记为既有非本模块问题）。

---

## 7. 对 #18 game-cashier / #15 feature-plugin 开工建议

1. **可并行开工**：`cashier-surface` → `game-cashier(#18)`；`channels-surface` → `feature-plugin(#15)`。两 lane 互不阻塞。
2. **下一 migration 序号**：`000012`（当前最大 000011）。
3. **game-cashier 注意**：依赖 `cashier-template` 平台模板 + `game`；复用 `domain/cashier`、`/cashier` surface；接线点仍为 `admin_wiring.go`。
4. **feature-plugin 注意**：与 `channel-login` 同 `channels-surface`；复用 `/channels` 抽屉模式；勿与 channel-login 路由冲突。
5. **集成前建议**：本地先 `go test ./...` + `pnpm test`；连库阶段设 `SCENARIO_WITH_DB=1` 跑四模块 requiresDB 用例。
6. **共享文件预警**：`admin_wiring.go`、`currency.go`、`ChannelInstanceDetailDrawer.vue`（feature-plugin）、`GameDetailView.vue`（game-cashier）— 改前读各模块 `integration.checklist.md`。

---

## 8. 结论

**合并后主干可构建、可测试；共享接线完整；P1 测试编译漂移已修复。**

✅ **可并行开工 `game-cashier` + `feature-plugin`**（migration 从 `000012` 起）。

---

## 附录：本次修复 diff

```
services/admin-api/internal/app/product/memstore_test.go          spyAudit.Write → return error
services/admin-api/internal/transport/http/cashier/cashier_http_test.go   RegisterRoutes +nil auditMW
services/admin-api/internal/transport/http/cashier/memstore_test.go       fakeAudit.Write → return error
services/admin-api/internal/transport/http/channels/login_http_test.go  RegisterRoutes +nil auditMW
```
