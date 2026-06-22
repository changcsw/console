---
id: operation-flow
code: "02"
title: 操作主线（功能/流程视角）
status: target
code_paths:
  - apps/admin-web/src/views
depends_on: [game, channel, feature-plugin, account-auth, channel-login, product, cashier-template, game-cashier, payment, snapshot, sync, auth, common]
impacts: []
children: []
---

# 02 · 操作主线（功能/流程视角）

> 跨模块的**端到端操作向导**：让人清楚「每个功能是什么、做完一步该做哪一步、卡在哪、去哪修」。
> 本文不重复各模块深设计（深设计见 `modules/*/README.md`），是前端**配置向导 / 未完成清单 / 红点**的唯一事实来源。
> DB 视角速查见 `schema-reference.md`。

分两类角色：**平台管理员**（基础数据/全局，多为一次性预置）与 **游戏管理员**（游戏维度，按序配置）。

---

## A. 平台管理员主线（基础数据 / 全局）

> 这些是「游戏侧配置的前置依赖」，通常由平台一次性预置、后续少量维护。入口集中在 `system`（基础数据/字典/模板/管理员）。

| 顺序 | 功能 | 产出 | 被谁依赖 | 模块 |
| --- | --- | --- | --- | --- |
| A1 | 管理员与 RBAC | 账号/角色/权限码 | 所有写操作鉴权 | [10-auth](./modules/10-auth/README.md) |
| A2 | 货币 `currency_specs` | 币种精度/下限/舍入 | 所有金额写入 | [common §5](./00-common.md) |
| A3 | 渠道主数据 + 策略 + `region` | `channels`/`channel_policies` | 渠道实例、登录、IAP、插件 | [12-channel](./modules/12-channel/README.md) |
| A4 | 功能插件主数据 + 模板 + 渠道可接策略（必接/可勾选） | `feature_plugins`/`feature_plugin_templates`/`channel_feature_plugins` | 游戏侧加插件 | [15-feature-plugin](./modules/15-feature-plugin/README.md) |
| A5 | 账号认证类型 + 模板 | `account_auth_types`/`account_auth_templates` | 自有账号配置 | [13-account-auth](./modules/13-account-auth/README.md) |
| A6 | 渠道登录模板 | `channel_login_templates` | 渠道强制登录配置 | [14-channel-login](./modules/14-channel-login/README.md) |
| A7 | 渠道 IAP 模板 | `channel_iap_templates` | 渠道 IAP 配置 | [16-product](./modules/16-product/README.md) |
| A8 | 收银台 provider/pay_way/主体/商户/价格模板/汇率 | 支付与价格基础数据 | 收银台绑定、支付路由 | [17-cashier-template](./modules/17-cashier-template/README.md) · [19-payment](./modules/19-payment/README.md) |

> 模板类（A4–A8）字段需标 `scope`（client/server/both，`00 §4.1.1`）；`server` 字段不下发客户端。

---

## B. 游戏管理员主线（游戏维度，按序）

每步给出：**入口 / 前置 / 产出 / 完成判定 / 下一步 / 异常拦截**。

```text
1 建游戏        2 启用 market     3 加渠道+渠道包    4 加功能插件(引导必接)
      └──────────────┴──────────────────┴───────────────────┘
5 账号认证/渠道登录   6 商品+IAP映射    7 收银台绑定+价格覆盖   8 支付路由
      └──────────────┴──────────────────┴───────────────────┘
                9 生成配置快照   →   10 同步到 production   →   11 运营态维护
```

### 1. 新建游戏 + 基础信息 — [11-game](./modules/11-game/README.md)
- 入口：`/games` → 新建。前置：A1 已能登录。
- 产出：`games`（含 icon/别名/默认 market/状态 draft）+ 法务链接 `game_legal_links`。
- 完成判定：游戏创建成功。**下一步 → 2 启用 market**。

### 2. 启用 markets（发行大区） — [11-game](./modules/11-game/README.md)
- 产出：`game_markets`（启用哪些大区 + 默认语言 + 默认市场）。
- 完成判定：至少一个 `enabled=true` 的 market。**下一步 → 3 加渠道**。
- 拦截：移除已有渠道实例的 market、或移除默认市场 ⇒ `409 CONFLICT`。

### 3. 加渠道实例（按 market）+ 渠道包 — [12-channel](./modules/12-channel/README.md)
- 前置：步骤 2 的 market 已启用；A3 渠道主数据就绪。
- 产出：`game_channels`（GameMarketChannel，按 market 兼容性过滤候选）+ `channel_packages`。
- 完成判定：渠道实例 `config_status=valid`（其下登录/IAP/插件齐备）。
- **下一步 → 4 加功能插件（系统引导）**。
- 拦截：market 与渠道 `region` 不兼容 ⇒ `MARKET_CHANNEL_INCOMPATIBLE`；重复 ⇒ `CONFLICT`。

### 4. 加功能插件（引导必接） — [15-feature-plugin](./modules/15-feature-plugin/README.md)
- 入口：渠道实例详情 → 「功能插件」区；**加完渠道后系统引导补齐必接插件**。
- 产出：`game_channel_plugin_configs`（可多个）；渠道包可继承或 `channel_package_plugin_overrides` 覆盖。
- 完成判定：所有 `required` 插件 `enabled=true ∧ config_status=valid`。
- **下一步 → 5 账号认证/渠道登录**。
- 拦截：必接插件未配齐、或勾选后缺必填参数 ⇒ 插件 `invalid` ⇒ 渠道实例异常，挡住快照/同步。

### 5. 配账号认证 / 渠道登录（依渠道策略） — [13-account-auth](./modules/13-account-auth/README.md) · [14-channel-login](./modules/14-channel-login/README.md)
- 分支：`channel_policies.login_mode=account_system` → 配自有账号认证（游戏级）；`channel_only` → 配渠道强制登录（渠道实例级）。
- 产出：`game_account_auth_configs` / `game_channel_login_configs`。
- 完成判定：相关配置 `valid`。**下一步 → 6 商品/IAP**。

### 6. 加商品 + IAP 映射 — [16-product](./modules/16-product/README.md)
- 产出：`products`（金额按 `00 §5` 归一化）+ `channel_products`（product_id/price_id 的 default/override）+ 渠道 IAP 配置 `game_channel_iap_configs`。
- 完成判定：商品与包级 IAP 映射就绪。**下一步 → 7 收银台**。

### 7. 绑定收银台模板版本 + 游戏级价格覆盖 — [17-cashier-template](./modules/17-cashier-template/README.md) · [18-game-cashier](./modules/18-game-cashier/README.md)
- 前置：A8 价格模板存在 `published` 版本。
- 产出：`game_cashier_profiles`（绑定某版本快照）+ `game_cashier_price_overrides`（按需覆盖）。
- 完成判定：已绑定有效版本。**下一步 → 8 支付路由**。

### 8. 配支付路由 — [19-payment](./modules/19-payment/README.md)
- 产出：`payment_routes`（market/country/currency/channel/package → pay_way + provider + 商户，按 priority）。
- 完成判定：目标 market 下关键支付方式有可命中路由且唯一。**下一步 → 9 生成快照**。
- 拦截：路由优先级/选择器冲突 ⇒ `ROUTE_CONFLICT`。

### 9. 生成配置快照 — [20-snapshot](./modules/20-snapshot/README.md)
- 产出：`game_config_snapshots`（per-game，按 market 分区，**按 `scope` 过滤只留 client/both**，确定性 hash）。
- 完成判定：快照生成且无阻塞异常。**下一步 → 10 同步**。
- 拦截：`hidden`/`incompatible`/`invalid`/必接插件缺失的数据不进快照；前端「未完成清单」列出并指向对应步骤修复。

### 10. Sandbox → Production 同步 — [21-sync](./modules/21-sync/README.md)
- 流程：`sync/preview`（按 section 出 add/update/delete + baseline_token，密文 masked）→ 勾选 `selected_sections` → `sync/execute`（复核 `target_hash_before`，不一致 `SYNC_BASELINE_MISMATCH`）。
- 完成判定：`sync_jobs.status=succeeded`。
- 拦截：`production` 视图不出现可执行的 Sync 入口（`00 §9`）。

### 11. 运营态维护
- 删减功能插件 / 收银台绑定；游戏整体停用/启用（`games.status`）；隐藏/恢复渠道实例。
- 任一改动后回到 **步骤 9 重新生成快照 → 10 同步**。

---

## C. 「下一步」驱动规则（前端引导事实来源）

- **完成判定**逐步如上；前端据此点亮「下一步」与进度。
- **阻塞项**统一口径：`config_status=invalid` / 不兼容 / 已隐藏 / 必接插件未配齐 / 路由冲突 / 缺有效收银台版本 ⇒ 列入「未完成/异常清单」，禁止进入快照与同步，并提供「去修复」直达入口（对应上面步骤）。
- **scope**：只有 `client/both` 参数进客户端最终配置；`server` 参数不下发（`00 §4.1.1`）。

## D. 与其它文档的分工
- 本文档：功能/流程视角（做什么、什么顺序、下一步、卡在哪）。
- `schema-reference.md`：DB 视角（哪些表/字段）。
- `modules/*/README.md`：单模块深设计（领域/表/API/前端）。
