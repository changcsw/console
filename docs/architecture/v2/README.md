# 管理后台架构文档集 v2（目标架构）

> 本目录是本轮重组后的**目标架构文档集**：树状结构、文档↔代码对齐、front-matter 记录模块关联，固化 7 项关键决策（D1–D7）。
> 旧的 `docs/architecture/*` 与 `docs/architecture/zh-CN/*` 原样保留归档。

## 三种视角，三类文档

| 想看什么 | 看哪 |
| --- | --- |
| 每个功能是什么、做完一步下一步做什么 | [02 · 操作主线](./02-operation-flow.md)（功能/流程视角） |
| 每个模块有哪些表、表里有哪些字段 | [schema-reference](./schema-reference.md)（DB 视角） |
| 单模块的领域/表/API/前端深设计 | `modules/*/README.md`（模块视角） |
| 怎么测、怎么按模块/整体回归、产物在哪 | [03 · 测试体系](./03-testing.md)（测试/回归视角） |
| 文档怎么写/命名/拆分/关联 | [CONVENTIONS](./CONVENTIONS.md) |

## 阅读顺序

1. [00 · 公共部分](./00-common.md) —— 跨模块契约：枚举/默认值、模板四件套（含 `scope`）、currency 归一化、密文/文件、状态机、统一 API 约定、审计、红线、D1–D7。
2. [01 · 整体项目结构](./01-structure.md) —— 技术栈、三环境 schema 模型、monorepo、前后端分层目录、迁移/seed、模块依赖总览、数据流。
3. [02 · 操作主线](./02-operation-flow.md) —— 平台管理员 / 游戏管理员两条端到端操作线与「下一步」规则。
4. [03 · 测试体系](./03-testing.md) —— 测试分层、与代码对齐的测试目录、后端接口场景矩阵标准维度、前端 Playwright 截图、统一 fixtures / 回归入口 / 报告产物。

## 功能模块（端到端：数据模型 + 后端 API + 领域规则 + 前端页面）

> 「关联模块」列取自各文档 front-matter 的 `impacts`（改该模块时需联动核对的下游）。
> 注：`testing` 是所有模块的**通用下游**（改任一模块都需联动核对其测试/场景矩阵），与 `common` 作为通用上游对称，二者均不在本列逐行展示；模块 front-matter 的 `impacts` 仍各自包含 `testing` 以保证反向一致（`CONVENTIONS §3.3`）。

| # | id | 文档 | 关键内容 | 关联模块（impacts） |
| --- | --- | --- | --- | --- |
| 10 | `auth` | [后台鉴权与 RBAC](./modules/10-auth/README.md) | JWT、密码/飞书登录、权限码 `resource.action` | — |
| 11 | `game` | [游戏主数据](./modules/11-game/README.md) | games/markets/legal、game_id/secret、多 market | channel, product, snapshot, sync… |
| 12 | `channel` | [渠道与渠道实例](./modules/12-channel/README.md) | GameMarketChannel(D2)、region(D3)、可见性/复制/隐藏 | account-auth, channel-login, feature-plugin, product, payment, snapshot, sync |
| 13 | `account-auth` | [自有账号认证](./modules/13-account-auth/README.md) | 模板驱动、config_status、按渠道允许集合 | snapshot, sync |
| 14 | `channel-login` | [渠道登录](./modules/14-channel-login/README.md) | channel_only 强制登录、模板驱动 | snapshot, sync |
| 15 | `feature-plugin` | [功能插件](./modules/15-feature-plugin/README.md) | **新增**：国内/海外、必接/可勾选、参数模板、渠道+包级、引导补齐 | snapshot, sync |
| 16 | `product` | [商品与 IAP 映射](./modules/16-product/README.md) | product_id vs price_id、override 解析、currency 归一化 | game-cashier, payment, snapshot, sync |
| 17 | `cashier-template` | [收银台模板与汇率同步](./modules/17-cashier-template/README.md) | 版本生命周期、copy-to-draft、FX 人工确认 | game-cashier, payment, snapshot, sync |
| 18 | `game-cashier` | [游戏级收银台](./modules/18-game-cashier/README.md) | 绑定版本快照、游戏级价格覆盖 | payment, snapshot, sync |
| 19 | `payment` | [支付路由](./modules/19-payment/README.md) | 路由匹配/排序/唯一性、PSP 无感切换 | snapshot, sync |
| 20 | `snapshot` | [配置快照与运行时合并](./modules/20-snapshot/README.md) | per-game 按 market 分区(D4)、scope 过滤、确定性 hash | sync |
| 21 | `sync` | [Sandbox 到 Production 同步](./modules/21-sync/README.md) | section 级、selected_sections、baseline 复核(D6) | — |
| 22 | `audit` | [审计日志](./modules/22-audit/README.md) | 写入规范 + 查询页（横切） | — |
| 23 | `dashboard` | [Dashboard 总览](./modules/23-dashboard/README.md) | 只读聚合（汇率待审/配置异常/同步状态） | — |

## 本轮重组要点

- **树状结构**：每个模块由单文件升级为 `modules/NN-英文短名/README.md`，可在文件夹内继续拆子文档（见 CONVENTIONS §4）。
- **front-matter 关联**：每个文档头部记录 `depends_on` / `impacts`（支持子模块级 `module/sub`），改一处先看 `impacts` 联动核对。
- **新增模块 15-feature-plugin**：与渠道同级；其后模块顺延重编号（16–23，映射见 CONVENTIONS §6）。
- **参数作用域 `scope`**：模板字段标 `client/server/both`（默认 `both`），配置快照只下发 `client/both`（`00 §4.1.1`、`20-snapshot §5.1.1`）。
- **新增 `schema-reference.md` / `02-operation-flow.md`**：分别提供 DB 速查与操作主线。
- **新增 `03-testing.md` 测试体系**：测试结构与代码功能结构对齐（hybrid：单元/集成/组件就近，跨栈 e2e/场景矩阵/fixtures/报告归顶层 `tests/`），后端「接口场景矩阵」标准维度 + 前端 Playwright 截图 + 统一回归入口；各模块「测试要点」升级为「接口场景矩阵」并回链 `03-testing`。
- **引用统一用稳定 `id`**：正文交叉引用改为稳定 `id`（如 `channel-login`/`snapshot`/`sync`），不再用「模块 NN」数字（编号映射留档于 `CONVENTIONS §6`）。

## 七项固化决策（D1–D7）

详见 [00 · 公共部分 §1](./00-common.md)。

- D1 单库 + 每环境独立 schema（`develop`/`sandbox`/`production` 各一套同名业务表，平台数据在 `platform` schema，业务表不带 `env` 列）
- D2 `game_channels` 加 `market_code`，即 GameMarketChannel 落地表
- D3 `channels` 加 `region`（domestic/overseas）
- D4 配置快照 per-game，`config_json` 按 market 分区
- D5 JWT + RBAC（权限码 `resource.action`）
- D6 `sync/execute` 必带 baseline，目标 hash 复核 + nonce 去重（幂等，防重复执行，见 `sync`）
- D7 后端 chi + pgx + golang-migrate
