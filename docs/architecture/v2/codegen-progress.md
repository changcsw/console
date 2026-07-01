# 代码生成进度账本

> 由「🎯 总负责 Agent（瘦身版 / 单模块 scope）」维护（见 `codegen-workflow.md` §12）。
> 本文件是总 Agent 允许持续读取的唯一进度账本；详细执行日志已迁移至 `docs/architecture/v2/codegen-progress.audit.md`，仅供人类审计。
> 总 Agent 只读本文件，不读审计归档正文、不读 handoff 全文、不读 diff。

**状态图例**：⬜ 未开始 ｜ 🔄 进行中 ｜ ✅ 完成 ｜ ❌ 打回（需返工）

**单模块流水线（DAG）**：
```text
   ┌ 🟦开发 → 🟦CR → 🟦测试 ┐
 M ┤                         ├─(均✅)→ 🟪测试专家 ⇄ 🟧全栈修复 → ✅验收
   └ 🟩开发 → 🟩CR → 🟩测试 ┘
```

**并行原则**：同 `lane` 串行，跨 `lane` 在 `depends_on` 满足时可并行。

**总 Agent 输入**：`docs/architecture/v2/index.json` + 本文件 + 当前模块 `spec.compact.md`

---

## 进度总表

> 列含义：🟦开发/CR/测试 = 后端车道三阶段；🟩开发/CR/测试 = 前端车道三阶段；🟪 = 测试专家（集成）；🟧 = 高级全栈工程师修复回路（有修复时标 🔄，回路结束标 ✅，无需修复留 ⬜）；✅ = 功能验收。

| # | 模块 id | lane | depends_on（需先完成后端） | 🟦开发 | 🟦CR | 🟦测试 | 🟩开发 | 🟩CR | 🟩测试 | 🟪测试专家 | 🟧全栈修复 | ✅验收 | 备注 |
| --- | --- | --- | --- | :--: | :--: | :--: | :--: | :--: | :--: | :--: | :--: | :--: | --- |
| 10 | `auth` | `platform-surface` | common | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 旧模块，详细审计已归档 |
| 11 | `game` | `games-surface` | common | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 旧模块，详细审计已归档 |
| 12 | `channel` | `channels-surface` | game | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 旧模块，详细审计已归档 |
| 13 | `account-auth` | `games-surface` | channel, game | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 旧模块，详细审计已归档 |
| 14 | `channel-login` | `channels-surface` | channel, game | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ 验收 15/15 PASS；4 项 P3 移交集成阶段；已合并至 main |
| 15 | `feature-plugin` | `channels-surface` | channel, game | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ 验收 29/29 PASS；🟪2轮(I-1契约漂移+P3)已闭合；遗留 P2/审计拆分/迁移000012协调 非阻断移交集成 |
| 16 | `product` | `games-surface` | channel, game | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ 已合并至 main |
| 17 | `cashier-template` | `cashier-surface` | common | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ 全流程完成（验收 29/29 PASS）；遗留非阻断·跨模块 |
| 18 | `game-cashier` | `cashier-surface` | cashier-template, game | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ 验收 25/25 PASS；集成2轮(taxRate漂移/checksum/重复键)修复；含跨模块 #17 发布 checksum 改动(checklist 已标注)；遗留连库维度待 PG CI |
| 19 | `payment` | `payment-surface` | channel, product, cashier-template, game-cashier, game | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ 验收 36/36 PASS；🟪2轮(R1发现I1-I4契约漂移→🟧修复→R2闭环)；含新增 `GET /cashier/providers/{id}/template`；遗留 P3 连库维度待 PG CI；已合并至 main |
| 20 | `snapshot` | `runtime-surface` | channel, account-auth, channel-login, feature-plugin, product, cashier-template, game-cashier, payment, game | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⬜ | ✅ | ✅ 验收 31/31 PASS；契约0漂移·无需🟧修复；连库21例待PG CI(非阻断)；已合并至 main |
| 21 | `sync` | `runtime-surface` | snapshot, +上游全部 | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | 上游已全部合并，可开工 |
| 22 | `audit` | `audit-surface` | common | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ 验收 26 项全 PASS；P0(schema→platform) 经 🟧 修复 + 🟪 复测；已合并至 main |
| 23 | `dashboard` | `dashboard-surface` | cashier-template, snapshot, sync | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | ⬜ | 等待 `sync` |

> 注：`artifacts_dir` 统一在 `index.json.docs[].artifacts_dir` 中定义；旧模块若尚无 `artifacts` 目录，仅在后续续作时回填。

---

## 可并行开工建议（按当前进度）

| lane | 推荐起始模块 | 当前状态 | 说明 |
| --- | --- | --- | --- |
| `games-surface` | （已全部完成） | ✅ 完成 | game / account-auth / product 三模块均 ✅，本 lane 收官 |
| `channels-surface` | （已全部完成） | ✅ 完成 | channel / channel-login / feature-plugin 三模块均 ✅，本 lane 收官 |
| `cashier-surface` | （已全部完成） | ✅ 完成 | cashier-template / game-cashier 均 ✅，本 lane 收官（含 #17 发布 checksum 跨模块修复） |
| `audit-surface` | （已全部完成） | ✅ 完成 | audit 已合并至 main |
| `payment-surface` | （已全部完成） | ✅ 完成 | payment 已合并至 main |
| `runtime-surface` | `sync` | 🔄 可开工 | #20 snapshot 已合并 main；可开工 sync #21（同 lane 串行） |
| `dashboard-surface` | `dashboard` | ⛔ 阻塞 | 等待 `sync` |

> 如果你选择 `feature-plugin` 而不是 `channel-login` 先开，也可以；但 **`channels-surface` 同时只能跑一个模块**。

---

## 在制看板（只放当前仍在推进的模块）

| 模块 | lane | 当前阶段 | worktree / branch | handoff.summary.md | 说明 |
| --- | --- | --- | --- | --- | --- |
| `sync`(#21) | runtime-surface | ⬜ 待开工 | （待分配 worktree） / `codex/sync` | — | #20 snapshot 已合并 main；runtime-surface 下一模块 |

> 总 Agent 每次只维护当前模块这一行；模块完成或阻塞后即清空或转历史，不把长日志写回本文件。

---

## 产物路径约定（速记）

- 模块产物目录：`docs/architecture/v2/modules/<code>-<module-id>/artifacts`
- 标准文件：
  - `bootstrap.module.txt`
  - `bootstrap.integration.txt`
  - `module.manifest.json`
  - `integration.checklist.md`
  - `handoff.summary.md`
  - `audit.log.md`

---

## 审计归档

- 详细执行日志：`docs/architecture/v2/codegen-progress.audit.md`
- 默认只给人类审计使用；总 Agent、集成 Agent、后续模块 Agent **默认不读**
- 旧模块 `10~13` 的历史日志已迁入归档；后续若续作这些模块，再按新协议回填各自 `artifacts` 目录
