# PostgreSQL DDL 中文说明

实际可执行 SQL 在：

- [postgresql_ddl_draft.sql](/Users/csw/gitproject/console/docs/architecture/postgresql_ddl_draft.sql)

这里主要给中文解释，避免团队在看纯 SQL 时反复对照聊天记录。

## 1. 游戏主数据

- `games`：游戏主表，存 `game_id`、`game_secret`、名称、代号、默认市场
- `game_markets`：游戏发行市场，例如 `GLOBAL / JP / KR / SEA / HMT / CN`
- `game_legal_links`：用户协议、隐私条款、删号链接，支持默认、按市场、按语言覆盖

## 2. 币种规则

- `currency_specs`：统一管金额输入与计算规则
  - `decimal_places`：支持的小数位数
  - `min_amount_minor`：允许的最小金额
  - `rounding_mode`：归一化时的舍入规则

凡是商品金额、价格模板金额、游戏级价格覆盖，都必须按这里校验和归一化。

## 3. 渠道与渠道包

- `channels`：渠道主数据
  - 例如 `google`、`apple`、`huawei_cn`、`xiaomi_cn`、`wechat_mini_game`
- `channel_policies`：渠道策略
  - `login_mode`：`channel_only / account_system`
  - `payment_mode`：`channel_only / hybrid / cashier_only`
- `game_channels`：某个游戏启用了哪些渠道
- `channel_packages`：某个游戏渠道下的包

## 4. 自有账号认证方式

- `account_auth_types`：游客、手机号、邮箱、Google、Apple 等认证方式主数据
- `channel_account_auth_types`：某个渠道允许哪些自有账号认证方式，以及默认勾选哪些
- `account_auth_templates`：模板驱动表单定义
- `game_account_auth_configs`：某游戏实际启用了哪些认证方式，以及各自配置

## 5. 渠道强制登录

适用于华为、小米、OPPO、VIVO 这类联运渠道。

- `channel_login_templates`
- `game_channel_login_configs`

这部分和“自有账号认证方式”不是一回事，底层分开存。

## 6. 商品、IAP 商品映射、价格档映射

- `products`：逻辑商品主表
- `channel_products`：
  - `product_id_mode/product_id_override`：控制 IAP 商品 ID 是否覆盖
  - `price_id_mode/price_id_override`：控制收银台价格档是否覆盖

这能支持：
- 95% 的默认继承场景
- 少数联运包、地区包的特殊覆盖场景

## 7. 渠道支付 / IAP

- `channel_iap_templates`
- `game_channel_iap_configs`
- `channel_package_iap_overrides`

用于渠道侧支付参数和包级覆盖。

## 8. 收银台价格模板

- `cashier_price_templates`
- `cashier_price_template_versions`
- `cashier_price_rows`
- `cashier_fx_sync_runs`

重点规则：
- 默认汇率同步模式是人工确认
- 只有打开自动应用开关后，才允许自动应用
- 游戏接入收银台时，绑定的是模板快照，不是实时跟随模板

## 9. 游戏级收银台

- `game_cashier_profiles`
- `game_cashier_price_overrides`

全局模板定义的是公共价格矩阵，游戏级表负责绑定快照和做个别覆盖。

## 10. 支付方式、支付提供商、商户、路由

- `pay_ways`：玩家看到的支付方式，例如信用卡、PayPal、GCash
- `cashier_providers`：真实支付提供商，例如 Airwallex、PayerMax
- `billing_subjects`：公司主体
- `cashier_merchant_accounts`：某主体在某 provider 下的商户账户
- `payment_routes`：支付路由表

这是实现“信用卡支付无感切换 PSP”的核心。

## 11. 配置快照与同步

- `game_config_snapshots`：客户端配置 JSON 快照
- `sync_jobs`
- `sync_job_items`

规则：
- `sandbox -> production` 不允许盲写
- 必须先看差异，再确认执行

## 12. 后台管理员与权限

- `admin_users`
- `admin_identities`
- `admin_roles`
- `admin_permissions`
- `admin_user_roles`
- `admin_role_permissions`

飞书登录只在这里，不进入玩家登录配置域。

