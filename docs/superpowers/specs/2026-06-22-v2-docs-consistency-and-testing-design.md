# v2 架构文档一致性修复 + 测试体系设计

> 状态：已确认（2026-06-22）。范围：仅修改 `docs/architecture/v2`，不动旧归档。交付为文档变更本身（docs-only），实际 `tests/` 代码与回归脚本留待后续实现计划。

## 背景

`docs/architecture/v2` 是本轮重组后的目标架构文档集。通读后发现多处一致性问题，且缺少全局测试体系。本设计固化修复口径与新增测试契约。

## 决策摘要（A–G）

### A. 旧编号 → 稳定 id 引用
- 正文所有「模块 NN」改为稳定 `id` 引用（如「模块 14」→ `channel-login`、「模块 19」→ `snapshot`、「模块 20」→ `sync`）。
- `01-structure §7` 依赖图与「依赖要点」的数字标签改为 id 标签。
- `CONVENTIONS §6` 旧→新编号映射表保留留档，说明改为「正文已统一 id 引用，本表仅供历史对照」。
- 不动：章节号（`§5.2`）、market 数量、价格档等真实数字。
- `README` 功能模块表保留 `#` 排序列，正文叙述用 id。

### B. audit_logs 的 env 口径
- `00-common §2.2`：把 `audit_logs` 移出「不带 env 的平台级清单」，单列口径：**有 `env` 列（记录操作发生环境），但不是「游戏维度业务表」——不前置 env 到唯一键、不参与 sandbox→production 同步 diff。**
- `00-common §8`、`schema-reference` 对齐同义措辞。

### C. 单库多 env 跨表引用：复合唯一/复合外键优先
- `00-common §2.2` 末条改为：**优先复合唯一键 +（同 env）复合外键保证引用方/被引用方 env 一致；仅在 PG 复合外键不可行处降级为应用层校验，并在该表注明。**
- `01-structure §4.2`/迁移小节、`schema-reference` 图例对齐。

### D. sync baseline_token 补强 nonce 去重/幂等
- `nonce` 升级为确定性幂等键（非可选增强）。
- execute 成功后落 nonce 去重；同一 token 二次 execute → 新错误码 `SYNC_TOKEN_CONSUMED`(409)。
- 明确「基线 hash 复核 + nonce 去重」双保险，区分幂等重放与目标已变更。
- 落点：`sync §2.3/§4.4/§5.3/§5.4/§11`、`00-common §7.4`、`README` D6 备注。

### E. 新增顶层 `03-testing.md`（全局测试契约）
- 与 00/01/02 同级，`id: testing`，`impacts` 关联全部模块。
- 测试分层：单元(domain)/集成(app+repo 真实 PG)/接口(transport httptest+真实库)/前端组件(vitest)/跨栈 e2e+截图(Playwright)。
- hybrid 目录树：单元/集成/组件就近；顶层 `tests/{frontend/e2e,screenshots,visual-baseline; backend/api,scenarios; fixtures; reports}` + `scripts/regression`。
- 后端接口场景矩阵标准维度：成功/鉴权401/权限403/校验失败400/冲突409/跨env/审计/脱敏/分页/事务回滚。
- 前端截图：Playwright 真实页面截图+trace+HTML report。
- 统一回归入口：启依赖→migrate→seed→后端全场景→前端 e2e+采集→汇总 `tests/reports/summary`。

### F. 各模块「测试要点」→「接口场景矩阵」
- 全部 14 个模块 README 升级/增补接口场景矩阵表（接口 × 标准维度），回链 `03-testing`。
- front-matter：各模块 `impacts` 增加 `testing`；`03-testing.depends_on` 列出各模块，保证反向一致（CONVENTIONS §3.3）。

### G. `01-structure` 技术栈 + README 接入
- 前端测试补 `e2e: Playwright`；后端补「集成/接口真实测试库(PG)+scenario manifest」。
- `README` 三视角表/阅读顺序接入 `03-testing`，重组要点加测试体系说明。

## 校验

- 复扫 `模块\s*\d+` 残留为 0（映射表与必要数字除外）。
- front-matter `impacts`/`depends_on` 反向一致。
- 相对链接不断链。
