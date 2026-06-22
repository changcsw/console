# 架构文档集 v2 重组设计（树状 + 模块关联 + 功能插件 + 参数作用域）

> 目标：把 `docs/architecture/v2` 重组为**树状、可逐层拆子文档、可记录模块间关联**的文档集；新增「功能插件」模块；引入「参数作用域（client/server/both）」跨模块约定；新增按模块分组的 `schema-reference.md`。
> 本 spec 是后续执行（writing-plans / 直接执行）的唯一依据。

---

## 1. 背景与现状

- 现状：`docs/architecture/v2/` 下有 `README.md`、`00-公共部分.md`、`01-整体项目结构.md`，以及 `modules/` 内 13 个模块文档（编号 10–22）。
- 代码侧 domain 划分：`cashier / channel / common / game / payment / product / sync`；前端 views：`audit / cashier / channels / dashboard / games / login / system`。
- 现有模块文档精细度可接受，但是**单层平铺**，无法支撑「每个模块再向下拆子文档」，也没有机器可读的「模块间关联」信息。

## 2. 目标与非目标

### 2.1 目标
1. 每个模块由「单 .md」升级为「文件夹 + `README.md`（模块功能总纲）」，可在文件夹内继续拆子文档，形成树。
2. 全部文档加 **YAML front-matter**，记录 `depends_on`（上游）与 `impacts`（下游联动），支持**子模块级**关联。
3. 命名：数字前缀 + 英文短名，尽量对齐代码 domain。
4. 新增「功能插件（feature-plugin）」模块，与渠道同级。
5. 引入「参数作用域 `scope`（client/server/both）」跨模块约定，配置快照只下发 client/both。
6. 新增 `schema-reference.md`：按模块分组列出所有表与字段（DB 视角）。
7. 新增 `02-operation-flow.md`：跨模块操作主线（功能/流程视角，回答「每个功能是什么、做完一步下一步做什么」），分平台管理员 / 游戏管理员两条线。

### 2.2 非目标（本轮不做）
- 不拆分各模块的子文档（仅建立结构与规范，正文原样保留）。
- 不改写/精简各模块现有正文（除被新约定直接影响的小段）。
- 不文件夹化 `00`、`01`（保持单文件，但加 front-matter）。
- 不改动 `docs/architecture/*` 旧文档与 `docs/architecture/zh-CN/*` 归档。
- 不落地任何代码 / 迁移。

---

## 3. 目标目录树

```text
docs/architecture/v2/
  README.md                         # 导航索引（阅读顺序 + 模块表 + 依赖图）
  CONVENTIONS.md                    # 新增：命名 / front-matter / 拆分 / 关联维护规范
  schema-reference.md               # 新增：按模块分组的全表-全字段参考（DB 视角）
  00-common.md                      # 保持单文件（加 front-matter）
  01-structure.md                   # 保持单文件（加 front-matter；原 01-整体项目结构）
  02-operation-flow.md              # 新增：跨模块操作主线（平台/游戏两角色，含「下一步」）
  modules/
    10-auth/            README.md   # 后台鉴权与 RBAC      code: domain/admin,auth
    11-game/            README.md   # 游戏主数据           code: domain/game
    12-channel/         README.md   # 渠道与渠道实例        code: domain/channel
    13-account-auth/    README.md   # 自有账号认证
    14-channel-login/   README.md   # 渠道登录
    15-feature-plugin/  README.md   # 新增：功能插件        code: domain/plugin(待建)
    16-product/         README.md   # 商品与 IAP 映射       code: domain/product
    17-cashier-template/README.md   # 收银台模板与汇率同步    code: domain/cashier
    18-game-cashier/    README.md   # 游戏级收银台
    19-payment/         README.md   # 支付路由             code: domain/payment
    20-snapshot/        README.md   # 配置快照与运行时合并    code: domain/snapshot
    21-sync/            README.md   # Sandbox→Production    code: domain/sync
    22-audit/           README.md   # 审计日志（横切）
    23-dashboard/       README.md   # Dashboard 总览
```

> 说明：`00-公共部分.md` 重命名为 `00-common.md`，`01-整体项目结构.md` → `01-structure.md`（仅路径英文化，标题可保留中文）。

### 3.1 命名映射（旧 → 新）

| 旧文件 | 新位置 | id | 备注 |
| --- | --- | --- | --- |
| `00-公共部分.md` | `00-common.md` | `common` | 单文件 |
| `01-整体项目结构.md` | `01-structure.md` | `structure` | 单文件 |
| `modules/10-后台鉴权与RBAC.md` | `modules/10-auth/README.md` | `auth` | |
| `modules/11-游戏主数据.md` | `modules/11-game/README.md` | `game` | |
| `modules/12-渠道与渠道实例.md` | `modules/12-channel/README.md` | `channel` | |
| `modules/13-自有账号认证.md` | `modules/13-account-auth/README.md` | `account-auth` | |
| `modules/14-渠道登录.md` | `modules/14-channel-login/README.md` | `channel-login` | |
| （新增） | `modules/15-feature-plugin/README.md` | `feature-plugin` | 新模块 |
| `modules/15-商品与IAP映射.md` | `modules/16-product/README.md` | `product` | 编号 15→16 |
| `modules/16-收银台模板与汇率同步.md` | `modules/17-cashier-template/README.md` | `cashier-template` | 16→17 |
| `modules/17-游戏级收银台.md` | `modules/18-game-cashier/README.md` | `game-cashier` | 17→18 |
| `modules/18-支付路由.md` | `modules/19-payment/README.md` | `payment` | 18→19 |
| `modules/19-配置快照与运行时配置合并.md` | `modules/20-snapshot/README.md` | `snapshot` | 19→20 |
| `modules/20-Sandbox到Production同步.md` | `modules/21-sync/README.md` | `sync` | 20→21 |
| `modules/21-审计日志.md` | `modules/22-audit/README.md` | `audit` | 21→22 |
| `modules/22-Dashboard总览.md` | `modules/23-dashboard/README.md` | `dashboard` | 22→23 |

---

## 4. front-matter 规范

### 4.1 模块总纲（`README.md` / `00`、`01`）

```yaml
---
id: channel                       # 稳定唯一标识（kebab，对齐代码 domain），重命名/移动不变
code: "12"                        # 排序编号（与文件夹前缀一致）
title: 渠道与渠道实例 (GameMarketChannel)
status: target                    # target | draft | deprecated
code_paths:                       # 文档 ↔ 代码对齐
  - services/admin-api/internal/domain/channel
  - apps/admin-web/src/views/channels
depends_on: [game, common]        # 我依赖的上游（改它们可能影响我）
impacts: [account-auth, channel-login, feature-plugin, product, payment, snapshot, sync]  # 改我时需联动核对的下游
children: []                      # 已拆出的子文档 id（如 [channel/visibility, channel/copy]）
---
```

### 4.2 子文档（后续拆分时）

```yaml
---
id: channel/visibility            # 路径式：<module>/<sub>
parent: channel
title: 可见性与兼容性规则
code_paths: [services/admin-api/internal/domain/channel/visibility.go]
depends_on: [common]
impacts: [snapshot/merge, sync/diff]   # 可精确到其它模块的子文档
---
```

### 4.3 字段语义

- `id`：稳定标识，是 `depends_on`/`impacts`/`parent`/`children` 引用的目标。模块用 `name`，子文档用 `name/sub`。
- `depends_on`：本文档**依赖**的对象（上游）。
- `impacts`：修改本文档时**需要连带核对/修改**的对象（下游联动）。可填模块 id，也可填子文档 id（满足「子模块改→联动其它模块子模块」）。
- `depends_on` 与 `impacts` 互为反向，原则上应保持一致（A.impacts 含 B ⇔ B.depends_on 含 A）；以**显式记录**为准，后续可加脚本校验断链与反向一致性。
- `code_paths`：对应代码目录/文件，体现「文档结构与代码结构对齐」。

---

## 5. 关联维护规则（写入 CONVENTIONS.md）

1. 改任一文档前，先读其 front-matter 的 `impacts`，逐项打开（含子文档粒度）核对是否需要同步修改。
2. 新增/删除模块或子文档时，同步更新相关文档的 `depends_on`/`impacts` 与父文档 `children`。
3. `00`、`01` 将来拆成文件夹时：`id` 不变（仍为 `common`/`structure`），原大文件内容下沉为子文档并各自补 `parent`/`id`，所有 `depends_on: [common]` 等引用因 `id` 不变而不断链；拆分 PR 必须同时更新 `children` 与受影响文档关联。
4. 命名规范：模块文件夹 `NN-英文短名`，模块总纲固定 `README.md`，子文档 `英文短名.md`（对齐代码文件名优先）。

---

## 6. 新模块：功能插件（feature-plugin，15）

### 6.1 定位
与「渠道」同级的平台能力：游戏在某渠道实例上接入若干「功能插件」（如实名、客服、推送、防沉迷等）。完全复用渠道 + IAP 的成熟模式：平台主数据 + 模板四件套 + 渠道级配置实例 + 渠道包覆盖。

### 6.2 数据模型

**`feature_plugins`（平台级，无 env）** — 对标 `channels`

| 列 | 类型 | 默认 | 约束/说明 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK |
| `plugin_id` | VARCHAR(64) | — | 业务键，UNIQUE |
| `plugin_name` | VARCHAR(64) | — | |
| `region` | VARCHAR(16) | — | CHECK in `domestic/overseas` |
| `enabled` | BOOLEAN | `TRUE` | |
| `sort` | INT | `0` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | `NOW()` | |

**`feature_plugin_templates`（平台级）** — 对标 `account_auth_templates`，含模板四件套
- `id`、`plugin_id_ref`(FK)、`template_version`、`form_schema_json`、`secret_fields_json`、`file_fields_json`、`validation_rules_json`、`enabled`，`UNIQUE(plugin_id_ref, template_version)`。
- `form_schema_json` 字段携带 `scope`（见 §7）。

**`channel_feature_plugins`（平台级）** — 对标 `channel_account_auth_types`：定义某渠道下可接哪些插件及其必接/可勾选属性

| 列 | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK |
| `channel_id_ref` | BIGINT | — | FK→channels(id) |
| `plugin_id_ref` | BIGINT | — | FK→feature_plugins(id) |
| `required` | BOOLEAN | `FALSE` | **必接**；必接未配置 ⇒ 渠道实例标异常 |
| `selectable` | BOOLEAN | `TRUE` | **前端是否可勾选**；必接项一般 `FALSE`（强制接入） |
| `default_enabled` | BOOLEAN | `FALSE` | 新建渠道实例时是否默认勾选 |
| `locked` | BOOLEAN | `FALSE` | 锁定后游戏侧不可改勾选状态 |
| `sort` | INT | `0` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | `NOW()` | |

`UNIQUE(channel_id_ref, plugin_id_ref)`。约束：`plugin.region` 必须与渠道使用场景的 market 兼容（CN⇒domestic，其它⇒overseas）。

**`game_channel_plugin_configs`（游戏级，带 env）** — 对标 `game_channel_iap_configs`，但**一个渠道实例可有多个插件**

| 列 | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK |
| `env` | VARCHAR(16) | — | CHECK in `develop/sandbox/production`（D1） |
| `game_channel_id_ref` | BIGINT | — | FK→game_channels(id) |
| `plugin_id_ref` | BIGINT | — | FK→feature_plugins(id) |
| `enabled` | BOOLEAN | `FALSE` | 是否勾选接入 |
| `config_json` | JSONB | `{}` | 插件参数 |
| `config_status` | VARCHAR(16) | `empty` | `empty/invalid/valid`；勾选但缺必填 ⇒ `invalid` |
| `last_check_at` | TIMESTAMPTZ | NULL | |
| `last_check_message` | VARCHAR(255) | `''` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | `NOW()` | |

`UNIQUE(env, game_channel_id_ref, plugin_id_ref)`。

**`channel_package_plugin_overrides`（游戏级，带 env）** — 对标 `channel_package_iap_overrides`

| 列 | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | — | PK |
| `env` | VARCHAR(16) | — | （D1） |
| `package_id_ref` | BIGINT | — | FK→channel_packages(id) |
| `plugin_id_ref` | BIGINT | — | FK→feature_plugins(id) |
| `inherit_channel_config` | BOOLEAN | `TRUE` | **默认与渠道用同一套插件及配置** |
| `enabled` | BOOLEAN | `FALSE` | |
| `config_json` | JSONB | `{}` | 仅存与渠道的差异 |
| `config_status` | VARCHAR(16) | `empty` | |
| `last_check_at` | TIMESTAMPTZ | NULL | |
| `last_check_message` | VARCHAR(255) | `''` | |
| `created_at`/`updated_at` | TIMESTAMPTZ | `NOW()` | |

`UNIQUE(env, package_id_ref, plugin_id_ref)`。

### 6.3 业务规则
- **国内/海外可见性**：`plugin.region` 与 market 兼容性校验同渠道规则，服务端二次校验。
- **必接/非必接**：`channel_feature_plugins.required=true` 的插件，渠道实例若未配置为 `valid` ⇒ 渠道实例运行态异常并提示补齐。
- **前端可勾选**：`selectable=false`（典型为必接项）前端置为强制选中、不可取消。
- **勾选必填参数**：`enabled=true` 且模板必填字段缺失 ⇒ `config_status=invalid`，遵循 `00 §3.4`。
- **加渠道后引导加插件**：游戏新建/进入渠道实例后，前端引导补齐该渠道下所有 `required` 插件（流程写入 12-channel 与本模块前端信息架构）。
- **渠道级 vs 渠道包级**：默认在渠道实例（GameMarketChannel）上配置；渠道包 `inherit_channel_config=true` 时沿用渠道的插件集合与配置，置 `false` 时用包级覆盖。
- **运行态/快照纳入**：与渠道一致——`hidden/incompatible/invalid` 的插件不进快照、不参与同步、不进客户端最终配置。

### 6.4 后端 API（草案，遵循 `00 §7`，权限码 `plugin.read/plugin.write`）
- `GET /api/admin/games/{gameId}/channels/{gameChannelId}/plugins` —— 列出该渠道实例可接插件 + 当前配置态。
- `POST /api/admin/game-channels/{gameChannelId}/plugins` —— 勾选/配置插件。
- `PATCH /api/admin/game-channel-plugins/{id}` —— 改配置/启停。
- `GET/POST /api/admin/channel-packages/{packageId}/plugins` —— 包级覆盖。
- 平台侧：插件主数据、模板、`channel_feature_plugins` 走 system/基础数据后台。

### 6.5 前端信息架构
- 游戏详情 → 渠道实例详情内新增「功能插件」区：列出可接插件、必接标记、勾选态、`config_status`、参数表单（模板驱动渲染器）。
- 渠道实例创建/保存后引导补齐 `required` 插件。
- 渠道包详情提供「继承渠道插件 / 自定义覆盖」开关。

### 6.6 关联（front-matter）
- `id: feature-plugin`，`code: "15"`。
- `depends_on: [channel, game, common]`。
- `impacts: [snapshot, sync]`（插件配置进快照、参与同步）。
- `channel`、`snapshot`、`sync` 的 `impacts`/`depends_on` 反向补 `feature-plugin`。

### 6.7 代码对齐
- 后端建议新增 domain `services/admin-api/internal/domain/plugin`（`code_paths` 先登记，代码后续落地）。

---

## 7. 跨模块约定：参数作用域 scope（client/server/both）

### 7.1 模板四件套扩展（`00 §4`）
`form_schema_json` 每个字段新增 `scope`：

```json
{ "key": "appId", "label": "App ID", "component": "input", "required": true, "scope": "both" }
```
- `scope ∈ client | server | both`，**默认 `both`**。
- 含义：标记该参数最终用于客户端、仅服务端、还是两端共用。

### 7.2 配置生成过滤（`20-snapshot`）
- 配置快照/客户端最终配置生成时，**只纳入 `scope ∈ {client, both}` 的参数**；`server` 仅用于服务端、不下发客户端。
- 合并算法在解析模板配置时按字段 `scope` 过滤。

### 7.3 影响范围（需在对应文档追加说明）
- `00-common §4`：模板字段结构加 `scope` 定义与默认值。
- `20-snapshot`：生成过滤规则 + 该模块 `depends_on` 与各模板模块关联。
- 所有使用模板四件套的模块：`account-auth`、`channel-login`、`product/IAP`、`cashier-template`(provider 模板)、`feature-plugin`——在其文档注明字段需标 `scope`。

---

## 8. schema-reference.md（新增）

- 位置：`docs/architecture/v2/schema-reference.md`。
- 结构：按模块分章（与目录树同序）。每章列出该模块拥有的表；每表一个小节，列出字段（列名 / 类型 / 默认 / 约束 / 作用域或备注）+ 唯一键/索引。
- 内容来源：汇总各模块文档 §3 数据模型 + `00 §2.2` 的 env 规则 + 本 spec §6 新增 plugin 表；并标注 v2 相对 `postgresql_ddl_draft.sql` 的差异（env 列、`market_code`、`region`、plugin 表、`scope`）。
- 定位：人读「全表-全字段」索引，与各模块细表互为对照；不替代各模块文档，也不作为可执行 DDL（可执行 DDL 仍由迁移文件承担）。
- 表→模块归属（关键）：
  - auth(10)：`admin_users/admin_identities/admin_roles/admin_permissions/admin_user_roles/admin_role_permissions`
  - game(11)：`games/game_markets/game_legal_links`
  - channel(12)：`channels/channel_policies/game_channels/channel_packages`
  - account-auth(13)：`account_auth_types/channel_account_auth_types/account_auth_templates/game_account_auth_configs`
  - channel-login(14)：`channel_login_templates/game_channel_login_configs`
  - feature-plugin(15)：`feature_plugins/feature_plugin_templates/channel_feature_plugins/game_channel_plugin_configs/channel_package_plugin_overrides`
  - product(16)：`products/channel_products/channel_iap_templates/game_channel_iap_configs/channel_package_iap_overrides`
  - cashier-template(17)：`cashier_price_templates/cashier_price_template_versions/cashier_price_rows/cashier_fx_sync_runs`
  - game-cashier(18)：`game_cashier_profiles/game_cashier_price_overrides`
  - payment(19)：`pay_ways/cashier_providers/cashier_provider_templates/billing_subjects/cashier_merchant_accounts/payment_routes`
  - snapshot(20)：`game_config_snapshots`
  - sync(21)：`sync_jobs/sync_job_items`
  - audit(22)：`audit_logs`
  - common：`currency_specs`
  - dashboard(23)：无独有表（只读聚合）

---

## 8b. 02-operation-flow.md（新增，操作主线）

- 位置：`docs/architecture/v2/02-operation-flow.md`（单文件，顶层）。
- 定位：跨模块**端到端操作向导**，不重复各模块深设计；回答「每个功能是什么、做完一步下一步做什么、卡在哪」。
- front-matter：`id: operation-flow`，`code: "02"`，`depends_on: [game, channel, feature-plugin, account-auth, channel-login, product, cashier-template, game-cashier, payment, snapshot, sync, common]`，`impacts: []`（聚合视图，被各模块驱动）。
- 结构：

### 8b.1 角色一：平台管理员（基础数据 / 全局，多为一次性预置）
列出可调全部功能与建议预置顺序：
- 管理员与 RBAC（10）→ 货币 specs（common）→ 渠道主数据+策略+region（12）→ 功能插件主数据+模板+`channel_feature_plugins`(必接/可勾选)（15）→ 账号认证类型+模板（13）→ 渠道登录模板（14）→ IAP 模板（16）→ 收银台 provider/pay_way/主体/商户/价格模板/汇率（17/19）。
- 每项标注：入口、产出、被哪些游戏侧操作依赖。

### 8b.2 角色二：游戏管理员（游戏维度，按序）
主线（每步含「入口 / 前置依赖 / 产出 / 完成后下一步 / 异常拦截」）：

```text
1. 新建游戏 → 填基础信息(icon/别名/默认 market/法务链接)         [11-game]
2. 启用 markets(发行大区)                                       [11-game]
3. 按 market 加渠道实例(兼容性过滤) → 加渠道包                    [12-channel]
4. 【引导】加功能插件：必接强制接入 → 填参数(缺则 invalid)        [15-feature-plugin]
5. 配账号认证 / 渠道登录(依渠道策略)                             [13/14]
6. 加商品 → IAP 映射(product↔package)                          [16-product]
7. 绑定收银台模板版本 → 游戏级价格覆盖                            [17/18-cashier]
8. 配支付路由                                                   [19-payment]
9. 生成配置快照(挡住 invalid/incompatible/hidden、按 scope 过滤)  [20-snapshot]
10. sandbox → production 同步                                  [21-sync]
11. 运营态：删减功能插件/收银台、游戏整体停用/启用                  [对应模块]
```

### 8b.3 「下一步」驱动规则
- 每步给出**完成判定**（如「渠道实例 config_status=valid 且必接插件已配齐」）。
- 给出**阻塞项**：`invalid`/`incompatible`/`hidden`/必接插件未配置 → 阻止进入快照/同步，并指明去哪修。
- 前端的引导（配置向导 / 未完成清单 / 红点）即据本节实现；本文档是其唯一事实来源。

### 8b.4 与 schema-reference 的分工
- 本文档=功能/流程视角（做什么、什么顺序）；`schema-reference.md`=DB 视角（哪些表/字段）；模块 README=单模块深设计。

---

## 9. CONVENTIONS.md（新增）

包含：
1. 目录与命名规范（模块文件夹 `NN-英文短名`、总纲 `README.md`、子文档命名）。
2. front-matter 字段表（§4）与示例。
3. 关联维护规则（§5）。
4. 文档拆分规则：何时把 README 内容下沉为子文档、拆分后如何维护 `children`/`parent`/关联、`00`/`01` 将来文件夹化的迁移规则。
5. 「文档 ↔ 代码」对齐约定（`code_paths`）。

---

## 10. README.md 更新

- 更新阅读顺序与模块表链接到新路径（含 15-feature-plugin、重编号 16–23）。
- 模块表新增「关联模块」列（取自 `impacts`，人读速览）。
- 依赖图补 feature-plugin 节点与 scope 说明。
- 增加指向 `CONVENTIONS.md` 与 `schema-reference.md` 的入口。

---

## 11. 工作分解（执行阶段）

1. 移动/重命名：13 模块 → `modules/NN-英文短名/README.md`（按 §3.1 含重编号）；`00`/`01` 重命名为英文单文件。
2. 给全部文档加 front-matter（§4），补齐 `depends_on`/`impacts`/`code_paths`。
3. 新建 `modules/15-feature-plugin/README.md`（§6 全文）。
4. `00-common.md §4` 加 `scope` 定义；`20-snapshot` 加生成过滤；相关模板模块注明 `scope`。
5. 新建 `schema-reference.md`（§8）。
6. 新建 `02-operation-flow.md`（§8b，两角色主线）。
7. 新建 `CONVENTIONS.md`（§9）。
8. 更新 `README.md`（§10）。
9. 自检：链接可达、front-matter `depends_on`/`impacts` 反向一致、编号与目录一致。

## 12. 显式假设
- 功能插件代码 domain 命名定为 `plugin`（`code_paths` 先登记，代码后续实现）。
- `scope` 默认 `both`，存量模板未标处按 `both` 解释（即默认仍下发客户端，保持向后兼容）。
- 重编号一次性完成；外部对旧编号的引用以 front-matter `id`（稳定）为准。
