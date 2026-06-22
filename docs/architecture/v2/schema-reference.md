---
id: schema-reference
code: "schema"
title: Schema 参考（按模块分组的全表-全字段）
status: target
code_paths:
  - services/admin-api/migrations
depends_on: [common, game, channel, account-auth, channel-login, feature-plugin, product, cashier-template, game-cashier, payment, snapshot, sync, audit, auth]
impacts: []
children: []
---

# Schema 参考（按模块分组）

> **DB 视角速查**：按模块列出全部表与字段，方便一眼看到「每个功能模块有哪些表、表里有哪些字段」。
> 字段细节与约束以各模块文档 §3 数据模型为准；本文件是汇总索引，**不作为可执行 DDL**（可执行 DDL 由 `services/admin-api/migrations/` 承担）。
> 与 `docs/architecture/postgresql_ddl_draft.sql` 的主要差异：带 env 的业务表新增 `env` 列、`game_channels` 新增 `market_code`、`channels` 新增 `region`、新增功能插件 5 张表、模板字段新增 `scope`。

## 图例
- **env 列**：带 `env` 的为「游戏维度业务表」（D1），唯一键统一前置 `env`；其余为平台级（全环境共享）。
- **跨表引用（带 env 表）**：优先**复合唯一键 +（同 env）复合外键**保证引用方/被引用方 env 一致（如 `FOREIGN KEY (env, game_channel_id_ref) REFERENCES game_channels(env, ...)`）；仅在复合外键不可行处降级应用层校验并注明（见 `00` §2.2、`01` §4）。
- **特例 `audit_logs`**：标注为「是(列)」表示**有 env 列但非游戏维度业务表**——env 仅作过滤维度，不前置唯一键、不参与同步 diff（见 `00` §2.2/§8）。`sync_jobs`/`sync_job_items` 不带 env 列，环境维度由 `source_env`/`target_env` 表达。
- **模板四件套**：`form_schema_json` / `secret_fields_json` / `file_fields_json` / `validation_rules_json`；`form_schema_json` 字段含 `scope`（`client/server/both`，默认 `both`）。
- 通用列省略：`id BIGSERIAL PK`、`created_at`/`updated_at TIMESTAMPTZ DEFAULT NOW()`（除特别说明）。

---

## common（公共 / 字典）

### `currency_specs`（平台级）
`currency_code`(UNIQUE) · `currency_name` · `decimal_places`(0–6) · `min_amount_minor`(默认1) · `rounding_mode`(half_up/floor/ceil/truncate) · `enabled`(默认TRUE)

---

## 10 · auth（鉴权与 RBAC）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `admin_users` | 否 | `user_name`(UNIQUE) · `display_name` · `email` · `status`(默认active) |
| `admin_identities` | 否 | `user_id_ref` · `identity_type`(password/feishu) · `identity_key` · `credential_ciphertext` · UNIQUE(identity_type, identity_key) |
| `admin_roles` | 否 | `role_code`(UNIQUE) · `role_name` |
| `admin_permissions` | 否 | `permission_code`(UNIQUE) · `permission_name` |
| `admin_user_roles` | 否 | `user_id_ref` · `role_id_ref` · UNIQUE(user_id_ref, role_id_ref) |
| `admin_role_permissions` | 否 | `role_id_ref` · `permission_id_ref` · UNIQUE(role_id_ref, permission_id_ref) |

---

## 11 · game（游戏主数据）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `games` | 是 | `game_id` · `game_secret` · `name` · `alias` · `icon_url` · `default_market_code`(默认GLOBAL) · `status`(draft/active/disabled) · UNIQUE(env, game_id) |
| `game_markets` | 是 | `game_id_ref` · `market_code` · `is_default` · `enabled` · `default_locale`(默认en-US) · UNIQUE(env, game_id_ref, market_code) |
| `game_legal_links` | 是 | `game_id_ref` · `scope_type`(default/market/locale) · `scope_value`(默认*) · `terms_url` · `privacy_url` · `delete_account_url` · UNIQUE(env, game_id_ref, scope_type, scope_value) |

---

## 12 · channel（渠道与渠道实例）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `channels` | 否 | `channel_id`(UNIQUE) · `channel_name` · `channel_type`(store/oem/web/direct/mini_game) · **`region`(domestic/overseas, D3)** · `enabled` · `sort` |
| `channel_policies` | 否 | `channel_id_ref`(UNIQUE) · `login_mode`(channel_only/account_system) · `payment_mode`(channel_only/hybrid/cashier_only) · `login_locked` · `payment_locked` |
| `game_channels` | 是 | `game_id_ref` · **`market_code`(D2)** · `channel_id_ref` · `enabled` · `hidden`/`hidden_by`/`hidden_at` · `config_status`(empty/invalid/valid) · `last_check_at`/`last_check_message` · `copied_from_market` · `remark` · UNIQUE(env, game_id_ref, market_code, channel_id_ref) |
| `channel_packages` | 是 | `game_channel_id_ref` · `package_code` · `package_name` · `market_code` · `bundle_id` · `inherit_channel_config`(默认TRUE) · `enabled` · `override_json` · UNIQUE(env, game_channel_id_ref, package_code) |

---

## 13 · account-auth（自有账号认证）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `account_auth_types` | 否 | `auth_type_id`(UNIQUE) · `auth_type_name` · `enabled` · `sort` |
| `channel_account_auth_types` | 否 | `channel_id_ref` · `auth_type_id_ref` · `default_enabled` · `locked` · `sort` · UNIQUE(channel_id_ref, auth_type_id_ref) |
| `account_auth_templates` | 否 | `auth_type_id_ref` · `template_version` · 模板四件套 · `enabled` · UNIQUE(auth_type_id_ref, template_version) |
| `game_account_auth_configs` | 是 | `game_id_ref` · `auth_type_id_ref` · `enabled` · `config_json` · `config_status` · `last_check_*` · UNIQUE(env, game_id_ref, auth_type_id_ref) |

---

## 14 · channel-login（渠道登录）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `channel_login_templates` | 否 | `channel_id_ref` · `template_version` · 模板四件套 · `enabled` · UNIQUE(channel_id_ref, template_version) |
| `game_channel_login_configs` | 是 | `game_channel_id_ref` · `enabled` · `config_json` · `config_status` · `last_check_*` · UNIQUE(env, game_channel_id_ref) |

---

## 15 · feature-plugin（功能插件，新增）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `feature_plugins` | 否 | `plugin_id`(UNIQUE) · `plugin_name` · `region`(domestic/overseas) · `enabled` · `sort` |
| `feature_plugin_templates` | 否 | `plugin_id_ref` · `template_version` · 模板四件套(含 scope) · `enabled` · UNIQUE(plugin_id_ref, template_version) |
| `channel_feature_plugins` | 否 | `channel_id_ref` · `plugin_id_ref` · `required`(必接) · `selectable`(可勾选,默认TRUE) · `default_enabled` · `locked` · `sort` · UNIQUE(channel_id_ref, plugin_id_ref) |
| `game_channel_plugin_configs` | 是 | `game_channel_id_ref` · `plugin_id_ref` · `enabled` · `config_json` · `config_status` · `last_check_*` · UNIQUE(env, game_channel_id_ref, plugin_id_ref) |
| `channel_package_plugin_overrides` | 是 | `package_id_ref` · `plugin_id_ref` · `inherit_channel_config`(默认TRUE) · `enabled` · `config_json` · `config_status` · `last_check_*` · UNIQUE(env, package_id_ref, plugin_id_ref) |

---

## 16 · product（商品与 IAP 映射）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `products` | 是 | `game_id_ref` · `product_id` · `product_name` · `base_amount_minor` · `base_currency` · `price_id` · `enabled` · UNIQUE(env, game_id_ref, product_id) |
| `channel_products` | 是 | `product_id_ref` · `package_id_ref` · `product_id_mode`(default/override) · `product_id_override` · `price_id_mode` · `price_id_override` · `enabled` · UNIQUE(env, package_id_ref, product_id_ref) |
| `channel_iap_templates` | 否 | `channel_id_ref` · `template_version` · 模板四件套 · `enabled` · UNIQUE(channel_id_ref, template_version) |
| `game_channel_iap_configs` | 是 | `game_channel_id_ref` · `enabled` · `config_json` · `config_status` · `last_check_*` · UNIQUE(env, game_channel_id_ref) |
| `channel_package_iap_overrides` | 是 | `package_id_ref` · `enabled` · `config_json` · `config_status` · `last_check_*` · UNIQUE(env, package_id_ref) |

---

## 17 · cashier-template（收银台模板与汇率同步）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `cashier_price_templates` | 否 | `template_id`(UNIQUE) · `template_name` · `fx_sync_enabled` · `fx_sync_mode`(manual_confirm/auto_apply) · `fx_sync_schedule`(monthly/quarterly) · `status` |
| `cashier_price_template_versions` | 否 | `template_id_ref` · `version` · `source_type` · `auto_generated` · `fx_base_date` · `status`(draft/published/archived) · `checksum` · `published_at` · UNIQUE(template_id_ref, version) |
| `cashier_price_rows` | 否 | `template_version_id_ref` · `country_code` · `region_code`(默认*) · `currency` · `price_id` · `pre_tax_amount_minor` · `tax_rate` · `tax_amount_minor` · `after_tax_amount_minor` · `effective_at` · UNIQUE(template_version_id_ref, country_code, region_code, currency, price_id) |
| `cashier_fx_sync_runs` | 否 | `template_id_ref` · `candidate_version_id_ref` · `status`(pending_review/approved/applied/ignored/failed) · `diff_summary_json` · `triggered_at` · `reviewed_by`/`reviewed_at`/`review_note` |

---

## 18 · game-cashier（游戏级收银台）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `game_cashier_profiles` | 是 | `game_id_ref` · `template_id_ref` · `applied_template_version_id` · `snapshot_checksum` · `applied_at` · UNIQUE(env, game_id_ref) |
| `game_cashier_price_overrides` | 是 | `game_id_ref` · `country_code` · `region_code`(默认*) · `currency` · `price_id` · `pre_tax_amount_minor` · `tax_rate` · `tax_amount_minor` · `after_tax_amount_minor` · `reason` · `effective_at` · UNIQUE(env, game_id_ref, country_code, region_code, currency, price_id) |

---

## 19 · payment（支付路由）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `pay_ways` | 否 | `pay_way_id`(UNIQUE) · `pay_way_name` · `pay_way_type`(card/wallet/platform/local) · `enabled` · `sort` |
| `cashier_providers` | 否 | `provider_id`(UNIQUE) · `provider_name` · `provider_kind`(aggregator/gateway/wallet_direct) · `enabled` · `sort` |
| `cashier_provider_templates` | 否 | `provider_id_ref` · `template_version` · 模板四件套 · `enabled` · UNIQUE(provider_id_ref, template_version) |
| `billing_subjects` | 否 | `subject_id`(UNIQUE) · `subject_name` · `legal_entity_name` · `enabled` |
| `cashier_merchant_accounts` | 否 | `merchant_account_id`(UNIQUE) · `provider_id_ref` · `subject_id_ref` · `merchant_id` · `merchant_name` · `config_json` · `secret_ciphertext` · `enabled` |
| `payment_routes` | 是 | `game_id_ref` · `market_code`(默认*) · `country_code`(默认*) · `currency`(默认*) · `channel_id_ref` · `package_id_ref` · `pay_way_id_ref` · `provider_id_ref` · `merchant_account_id_ref` · `priority`(默认100) · `enabled` |

---

## 20 · snapshot（配置快照）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `game_config_snapshots` | 是 | `game_id_ref` · `config_schema_version` · `config_version` · `config_json`(按 market 分区, scope 过滤后) · `file_name` · `file_hash` · `storage_key` · `status`(draft/published) · `generated_at`/`published_at` · UNIQUE(env, game_id_ref, config_version) |

---

## 21 · sync（Sandbox → Production 同步）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `sync_jobs` | — | `game_id_ref` · `source_env`/`target_env` · `source_hash` · `target_hash_before`/`target_hash_after`(D6) · `include_deletes` · `operator_id`/`operator_note` · `status`(previewed/succeeded/failed) · `executed_at` |
| `sync_job_items` | — | `sync_job_id_ref` · `section` · `entity_type` · `entity_key` · `op`(add/update/delete) · `field_name` · `sandbox_value_json`/`production_value_json` · `masked` · `applied` |
| `sync_consumed_tokens` | — | `nonce`(UNIQUE) · `sync_job_id_ref` · `consumed_at` —— baseline_token 幂等去重（D 决策，与 execute 同事务写入，见 `sync` §5.6） |

---

## 22 · audit（审计日志）

| 表 | env | 关键字段 |
| --- | --- | --- |
| `audit_logs` | 是(列) | `actor_id` · `action` · `resource_type` · `resource_id` · `env` · `detail_json` · `created_at` |

---

## 23 · dashboard（只读聚合）
无独有表；只读聚合 `cashier_fx_sync_runs` / `game_config_snapshots` / `sync_jobs` / `game_channels` 等的统计视图。

---

## 附：带 env 的业务表清单（唯一键统一前置 env）
`games` · `game_markets` · `game_legal_links` · `game_channels` · `channel_packages` · `game_account_auth_configs` · `game_channel_login_configs` · `game_channel_plugin_configs` · `channel_package_plugin_overrides` · `products` · `channel_products` · `game_channel_iap_configs` · `channel_package_iap_overrides` · `game_cashier_profiles` · `game_cashier_price_overrides` · `payment_routes` · `game_config_snapshots`（`audit_logs` 带 env 列但非游戏维度业务表）。
