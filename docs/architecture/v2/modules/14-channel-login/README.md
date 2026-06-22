---
id: channel-login
code: "14"
title: 渠道登录（渠道强制登录配置）
status: target
code_paths:
  - services/admin-api/internal/domain/channel
  - services/admin-api/internal/transport/http/channels
  - apps/admin-web/src/views/channels
depends_on: [channel, game, common]
impacts: [snapshot, sync, testing]
children: []
---

# 14 · 渠道登录（渠道强制登录配置）

> 本文件描述「渠道强制登录」模块。默认遵循 `../../00-common.md`（模板四件套、ConfigStatus、密文、审计、env、API 包络）与 `../../01-structure.md`（分层、目录、装配）。本文与 `00` 冲突处以 `00` 为准，本文只在其基础上追加模块私有约定。
>
> 一句话定位：当某个游戏在某个 market 下接入了**联运渠道（`login_mode=channel_only`，如 `huawei_cn` / `xiaomi_cn` / `oppo_cn` / `vivo_cn`）**时，玩家必须走该渠道自身的登录体系（不能用游戏自有账号体系）。本模块负责维护**每个 `GameMarketChannel` 实例**下的渠道登录参数（appId / appSecret 等），由 `channel_login_templates` 模板驱动渲染与校验，配置实例落在 `game_channel_login_configs`。

---

## 1. 边界（与 `account-auth`「自有账号认证」的区别）

### 1.1 红线：两套登录体系底层分开

`00` §9 与各执行文档反复强调的一条不可触碰红线：**不把"自有账号认证（`account-auth`）"与"渠道强制登录（本模块 `channel-login`）"混在一起**。两者解决的是**互斥的两类登录策略**，物理上分两套表、分两个领域服务、分两个前端页面，互不共享配置实例、密钥、状态。

| 维度 | `account-auth` 自有账号认证 | `channel-login` 渠道强制登录（本模块） |
| --- | --- | --- |
| 触发场景 | 渠道 `login_mode=account_system`（如 `google` / `apple` / web / direct） | 渠道 `login_mode=channel_only`（如 `huawei_cn` / `xiaomi_cn` / `oppo_cn` / `vivo_cn`） |
| 登录主体 | 游戏自有账号体系（游客 / 手机号 / 邮箱 / Google / Apple 第三方登录） | 渠道自身的账号体系（华为账号、小米账号…），玩家被强制走渠道登录 |
| 字典主数据 | `account_auth_types`、`channel_account_auth_types` | 复用 `channels` + `channel_policies`（无独立"登录类型字典"） |
| 模板表 | `account_auth_templates`（按 `auth_type` 维度） | `channel_login_templates`（按 `channel` 维度） |
| 配置实例表 | `game_account_auth_configs`（挂在 **game** 上，多条/按 `auth_type`） | `game_channel_login_configs`（挂在 **game_channel / GameMarketChannel** 上，每实例 0..1 条） |
| 配置粒度 | 游戏级（一个游戏一套自有账号认证） | 渠道实例级（按 market 拆分，每个 `GameMarketChannel` 各自独立） |
| 应用服务 | `AccountAuthService` | `ChannelLoginService`（本模块） |
| 前端页 | 自有账号认证配置页（game 详情 Tab） | 渠道强制登录配置页（仅对 `channel_only` 渠道实例展示） |
| 后端 API | `PUT /api/admin/games/{gameId}/account-auth-configs` | `GET/PUT /api/admin/game-channels/{gameChannelId}/login-config` |

> 红线补充：本模块也**绝不**与"后台管理员登录 / 飞书登录（`auth` 鉴权）"混淆——那是**运营人员登录后台**，本模块是**玩家在客户端登录游戏**。三者（管理员登录 / 自有账号认证 / 渠道强制登录）域名完全独立。

### 1.2 本模块负责 / 不负责

负责：

- 维护 `channel_login_templates`（渠道登录模板四件套，平台级，**不带 env**）的"消费"——本模块只读取/校验模板，模板的版本生命周期维护属于「基础数据/模板管理后台」（见 `00` §3.3 / §4.4，归 system 模块）。
- 维护 `game_channel_login_configs`（渠道登录配置实例，**带 env**）：`enabled`、`config_json`（含密文位）、`config_status`、`last_check_at`、`last_check_message`。
- 提供单实例的读取与整体更新接口（`GET/PUT .../login-config`）。
- 基于模板做配置校验、推导并持久化 `config_status` 与 `last_check_message`。
- 对密文字段（`secret_fields_json` 标记）做加密落库 + 响应脱敏。

不负责：

- 渠道实例本身的增删（属 `channel` 渠道实例）；本模块只在已存在的 `game_channel` 上挂登录配置。
- 渠道包、IAP 配置（`channel` / `product`）。
- 模板版本的发布/归档（属 system 模块，遵循 `00` §3.3 VersionStatus 状态机）。
- 客户端最终配置的合并与快照（`snapshot`），本模块只产出"有效配置实例"供其消费。
- `sandbox -> production` 的 diff/执行（`sync`），本模块数据按 `section=channels` 的下游随渠道实例一起进入同步集（见 §9）。

---

## 2. 领域模型（渠道登录配置，挂在 GameMarketChannel 上）

### 2.1 归属关系

```text
Game (games, 带 env)
  └── GameMarketChannel (game_channels, 带 env, 按 (env, game_id_ref, market_code, channel_id_ref) 唯一)   ← `channel` D2：该表本身即渠道实例落地表
        └── ChannelLoginConfig (game_channel_login_configs, 带 env, 每实例 0..1 条)   ← 本模块
              └─(模板驱动) channel_login_templates (平台级, 不带 env, 按 (channel_id_ref, template_version))
```

关键点：

- **登录配置挂在 `GameMarketChannel`（即 `game_channels` 行）上，不是挂在 game 上。** 因为渠道实例按 market 拆分（`channel` D2：`game_channels` 唯一键含 `market_code`），所以**登录配置天然也是按 market 实例独立的**：`CN / huawei_cn` 与（假想的）另一 market 下同渠道是两条独立 `game_channels` 行，各自挂各自的 `game_channel_login_configs`，**不共享** `config_json` / 密钥 / 文件 / 状态。
- 一个 `game_channel` 最多一条登录配置（`UNIQUE(env, game_channel_id_ref)`，见 §3）。不存在时视为"未配置"（前端按 `empty` 呈现）。
- 模板按**逻辑渠道**维度复用：同一 `channel`（如 `huawei_cn`）在不同 game / 不同 market 下复用同一套 `channel_login_templates` 定义（`form_schema_json` 等四件套），但**配置实例必须各自独立**（`00` §4.4）。

### 2.2 领域聚合内位置

按 `01` §4 与 `go_domain_api_draft.md`，渠道登录配置归属 `channel.GameChannelAggregate`：

```text
channel.GameChannelAggregate 拥有：
  - selected channel（channels 引用）
  - channel policy snapshot（channel_policies：login_mode/payment_mode/locked）
  - channel login config      ← 本模块聚焦
  - channel IAP config（`product`）
  - packages（`channel`）
```

领域对象（概念形态，落在 `internal/domain/channel`）：

```go
// 渠道登录配置实例（带 env，挂在 GameMarketChannel 上）
type ChannelLoginConfig struct {
    ID               int64
    Env              Environment   // develop/sandbox/production
    GameChannelID    int64         // -> game_channels.id（该 game_channel 自带 market_code）
    Enabled          bool          // 是否启用该渠道强制登录
    ConfigJSON       map[string]any // 模板字段值；密文位存密文，不存明文
    ConfigStatus     ConfigStatus  // empty/invalid/valid（推导得到，见 §5）
    LastCheckAt      *time.Time
    LastCheckMessage string
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

// 渠道登录模板（平台级，不带 env；本模块只读消费）
type ChannelLoginTemplate struct {
    ID                  int64
    ChannelID           int64        // -> channels.id
    TemplateVersion     string
    FormSchemaJSON      []FormField  // 四件套：渲染字段
    SecretFieldsJSON    []string     // 四件套：密文字段 key 列表
    FileFieldsJSON      []FileField  // 四件套：文件字段
    ValidationRulesJSON map[string]ValidationRule // 四件套：校验规则
    Enabled             bool
}
```

> 说明：`config_status` 不是前端可任意写入的字段，而是后端依据模板四件套对 `config_json` 校验后**推导并持久化**的结果（见 §5.3）。

---

## 3. 数据模型（逐表逐字段）

本模块涉及两张表：`channel_login_templates`（平台级模板，**不带 env**）与 `game_channel_login_configs`（配置实例，**带 env**）。下列以 `migrations/000001_init.up.sql` 真实列为准，并叠加 `00` D1 的 env 改造。

### 3.1 `channel_login_templates`（平台级模板，全 env 共享，不带 env）

模板四件套表，结构与其它 `*_templates` 完全一致（`00` §4）。按 `00` §2.2 明确列入「不带 env 的平台级模板表」。

| 列 | 类型 | 默认值 | 约束/说明 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | — | 主键 |
| `channel_id_ref` | BIGINT | — | `NOT NULL REFERENCES channels(id)`；指向逻辑渠道（如 `huawei_cn`） |
| `template_version` | VARCHAR(32) | — | `NOT NULL`；模板版本号 |
| `form_schema_json` | JSONB | `'[]'::jsonb` | `NOT NULL`；前端渲染哪些字段（`00` §4.1 单字段结构） |
| `secret_fields_json` | JSONB | `'[]'::jsonb` | `NOT NULL`；密文字段 key 列表（如 `["appSecret"]`） |
| `file_fields_json` | JSONB | `'[]'::jsonb` | `NOT NULL`；文件上传字段定义 |
| `validation_rules_json` | JSONB | `'{}'::jsonb` | `NOT NULL`；前后端共同遵循的校验规则 |
| `enabled` | BOOLEAN | `TRUE` | `NOT NULL` |
| `created_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` |
| `updated_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` |

唯一约束：`UNIQUE (channel_id_ref, template_version)`。

> 注意：模板表**不加 env**，全环境共享同一套模板定义。模板的 `draft/published/archived` 版本生命周期由 system 模块按 `00` §3.3 维护；本模块运行时**只取该渠道当前 `published` 且 `enabled=TRUE` 的模板版本**用于渲染与校验（解析逻辑见 §5.2）。

### 3.2 `game_channel_login_configs`（配置实例，带 env —— 决策 D1）

原始 DDL（`000001_init.up.sql` 第 170–181 行）无 env；按 `00` D1 / `01` §6 的 v2 迁移要求，**新增迁移**给本表加 `env` 列并改唯一键。改造后逐字段如下：

| 列 | 类型 | 默认值 | 约束/说明 |
| --- | --- | --- | --- |
| `id` | BIGSERIAL | — | 主键 |
| `env` | VARCHAR(16) | （取当前运行环境，无 DB 默认） | **新增（D1）**；`NOT NULL CHECK (env IN ('develop','sandbox','production'))` |
| `game_channel_id_ref` | BIGINT | — | `NOT NULL REFERENCES game_channels(id)`；指向 `GameMarketChannel` 实例（该实例自带 `market_code`） |
| `enabled` | BOOLEAN | `FALSE` | `NOT NULL`；是否启用该渠道强制登录（默认关，`00` §10 业务"启用类"默认 false） |
| `config_json` | JSONB | `'{}'::jsonb` | `NOT NULL`；模板字段值；密文位存**密文**（绝不存明文） |
| `config_status` | VARCHAR(16) | `'empty'` | `NOT NULL CHECK (config_status IN ('empty','invalid','valid'))`；推导得到 |
| `last_check_at` | TIMESTAMPTZ | `NULL` | 最近一次校验时间；从未校验则为 `NULL` |
| `last_check_message` | VARCHAR(255) | `''` | `NOT NULL`；最近一次校验信息（失败原因/通过提示） |
| `created_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` |
| `updated_at` | TIMESTAMPTZ | `NOW()` | `NOT NULL` |

唯一约束（D1：唯一键前置 env）：

```text
原: UNIQUE (game_channel_id_ref)
改: UNIQUE (env, game_channel_id_ref)
```

> 解释为何前置 `env` 即足够：`game_channels` 行本身已带 `env` 且唯一键为 `(env, game_id_ref, market_code, channel_id_ref)`（`channel` D2），即 `game_channel_id_ref` 已隐含 `(env, game, market, channel)` 全维度。因此本表只需保证"**同 env 下同一渠道实例至多一条登录配置**"，`UNIQUE(env, game_channel_id_ref)` 即可。

外键 env 一致性（`00` §2.2）：`game_channel_login_configs.env` 必须与其引用的 `game_channels.env` **一致**，由应用层 `ChannelLoginService` 在写入前校验（DB 层保留普通外键到 `game_channels(id)`）。

### 3.3 迁移片段（v2 新增迁移，示意）

```sql
-- up：给 game_channel_login_configs 加 env，并改唯一键（D1）
ALTER TABLE game_channel_login_configs
  ADD COLUMN env VARCHAR(16) NOT NULL DEFAULT 'develop';
ALTER TABLE game_channel_login_configs
  ADD CONSTRAINT game_channel_login_configs_env_check
  CHECK (env IN ('develop','sandbox','production'));
ALTER TABLE game_channel_login_configs
  DROP CONSTRAINT game_channel_login_configs_game_channel_id_ref_key; -- 原 UNIQUE(game_channel_id_ref)
ALTER TABLE game_channel_login_configs
  ADD CONSTRAINT uq_gclc_env_game_channel UNIQUE (env, game_channel_id_ref);
-- 回填历史数据后再去掉列默认值，避免新行漏带 env：
ALTER TABLE game_channel_login_configs ALTER COLUMN env DROP DEFAULT;
```

> `channel_login_templates` 无需改动（平台级、不带 env）。

---

## 4. 枚举与默认值清单（穷尽）

### 4.1 引用的全局枚举（`00` §3，事实来源在 `00`，此处仅列与本模块相关取值）

| 枚举 | 取值 | 默认值 | 本模块用途 |
| --- | --- | --- | --- |
| `Environment` | `develop` / `sandbox` / `production` | `develop` | `game_channel_login_configs.env` |
| `LoginMode` | `channel_only` / `account_system` | `account_system` | 判断渠道是否需要本模块（仅 `channel_only` 需要） |
| `ConfigStatus` | `empty` / `invalid` / `valid` | `empty` | `game_channel_login_configs.config_status` |
| `VersionStatus` | `draft` / `published` / `archived` | `draft` | 模板版本（本模块只消费 `published`） |
| `Market` | `GLOBAL` / `JP` / `KR` / `SEA` / `HMT` / `CN` | `GLOBAL` | 经由 `game_channels.market_code` 间接体现（本模块不直接存 market） |

> `channel_only` 渠道当前 seed 为国内联运渠道（`00`/`01` §6 中 `region=domestic`）：`huawei_cn` / `xiaomi_cn` / `oppo_cn` / `vivo_cn`（小游戏 `wechat_mini_game` / `douyin_mini_game` 的 `login_mode` 以 `channel_policies` seed 为准）。判断是否需要本模块的唯一依据是 `channel_policies.login_mode == 'channel_only'`，而非渠道名硬编码。

### 4.2 本模块字段默认值（穷尽）

| 表 | 字段 | 默认值 | 来源/理由 |
| --- | --- | --- | --- |
| `game_channel_login_configs` | `env` | 当前运行环境（写入时取 `APP_ENV`） | `00` §2.1 / §10 |
| `game_channel_login_configs` | `enabled` | `FALSE` | 启用类默认关；需配置 `valid` 后才建议开 |
| `game_channel_login_configs` | `config_json` | `{}`（空对象） | `00` §10：JSONB 配置类默认 `{}` |
| `game_channel_login_configs` | `config_status` | `empty` | `00` §3.1 ConfigStatus 默认 |
| `game_channel_login_configs` | `last_check_at` | `NULL` | 从未校验 |
| `game_channel_login_configs` | `last_check_message` | `''` | `00` §10：备注类默认 `''` |
| `channel_login_templates` | `form_schema_json` | `[]` | `00` §4：模板四件套列表类默认 `[]` |
| `channel_login_templates` | `secret_fields_json` | `[]` | 同上 |
| `channel_login_templates` | `file_fields_json` | `[]` | 同上 |
| `channel_login_templates` | `validation_rules_json` | `{}` | `00` §4：对象类默认 `{}` |
| `channel_login_templates` | `enabled` | `TRUE` | `00` §10：`enabled` 默认 true |

### 4.3 `config_status` 状态机取值含义（`00` §3.4，本模块完整复述）

| 状态 | 含义 | 进入条件（本模块具体化） |
| --- | --- | --- |
| `empty` | 尚未建立有效配置 | 实例无记录，或有记录但 `config_json` 未填任何字段 |
| `invalid` | 已有结构但校验未过 | 缺必填/密文/文件字段，或值不满足 `validation_rules_json`；**经复制创建且 secret/file 被清空时强制此状态** |
| `valid` | 完整且通过校验 | 全部必填（含 secret/file）补齐且全部校验通过 |

---

## 5. 业务规则

### 5.1 仅 `channel_only` 渠道需要（适用性判定）

- 只有所选渠道的 `channel_policies.login_mode == 'channel_only'` 时，该 `GameMarketChannel` 才**需要/允许**配置渠道强制登录。
- 对 `login_mode == 'account_system'` 的渠道实例：本模块接口应拒绝写入（返回 `VALIDATION_FAILED`，message 提示"该渠道非 channel_only，登录走自有账号认证（`account-auth`）"）；前端不展示渠道强制登录页签/入口（§8）。
- 判定依据是渠道策略，而非渠道名硬编码；当前 `channel_only` 渠道为 `huawei_cn` / `xiaomi_cn` / `oppo_cn` / `vivo_cn`（以 `channel_policies` seed 为准）。
- `channel_policies.login_locked == TRUE` 的语义见 §5.5。

### 5.2 模板驱动校验（template-driven）

写入 `login-config` 时，后端按以下步骤用 `channel_login_templates` 校验 `config_json`：

1. 由 `game_channel_id_ref` 反查 `game_channels` 得 `channel_id_ref`，再取该渠道**当前 `published` 且 `enabled=TRUE`** 的 `channel_login_templates`（按版本生命周期；若按 `template_version` 显式指定则取指定版本）。模板不存在 ⇒ 拒绝（`VALIDATION_FAILED`，"该渠道未配置登录模板"）。
2. **未知字段拒绝**：`config_json` 中出现 `form_schema_json` 未声明的 key ⇒ `VALIDATION_FAILED`。
3. **必填校验**：`form_schema_json` 中 `required=true` 的字段必须有非空值（含 `secret_fields_json` / `file_fields_json` 标记的敏感/文件字段）。
4. **规则校验**：逐字段按 `validation_rules_json` 校验（`minLen/maxLen/min/max/pattern/format/enum`，`00` §4.3）。
5. **密文处理**：`secret_fields_json` 标记字段，落库前用 `infra/crypto`（AES-GCM）加密存入 `config_json` 对应密文位；明文禁止落库（`00` §6.1）。提交时若该密文字段传入哨兵值 `"******"` / `"masked"`（表示"未修改"），则保留库中原密文，不覆盖。
6. **文件处理**：`file_fields_json` 标记字段，存"文件引用（storage key / hash）"，不存二进制（`00` §6.2）。
7. 全部通过 ⇒ 推导 `config_status`（§5.3），写 `last_check_at=NOW()`、`last_check_message`。

### 5.3 `config_status` 推导规则（强约束）

按 `00` §3.4 推导，本模块判定顺序：

- `config_json` 为空对象 `{}` 且无任何字段 ⇒ `empty`。
- 有字段但缺必填（含 secret/file）或任一校验未过 ⇒ `invalid`，`last_check_message` 写明首个失败原因。
- 全部必填补齐且校验通过 ⇒ `valid`。

**复制创建强约束（`00` §3.4 红线）**：当实例经"从其它 market 同渠道复制初始值"创建、且 `secret`/`file` 字段被清空时，**必须显示 `invalid`，绝不显示 `empty`**，且 `last_check_message` 必须为"缺少必填敏感字段或文件字段"。即：只要存在被清空的必填 secret/file，即便普通字段已带入，也不得回落到 `empty`。

> `config_status` 是**后端推导并持久化**的字段，PUT 请求体即使携带 `configStatus` 也以后端推导结果为准（请求中的该字段忽略或仅作客户端提示）。

### 5.4 按 market 实例独立（核心结构性规则）

- 登录配置随 `game_channels`（GameMarketChannel）按 `(env, game, market, channel)` 独立存在。`CN / huawei_cn` 的登录配置与其它 market 下的同渠道实例**互不影响**：各自的 `config_json`、密钥、文件、`config_status`、`enabled` 全部独立。
- 新增某 market 渠道实例时（`channel` 的"新增渠道"流程）允许"从其它 market 同渠道复制初始值"：仅复制普通字段；**secret/file 必须清空**；复制后两实例不再联动；新实例初始 `config_status=invalid`（§5.3）。本模块在被复制写入时按 §5.2 重新校验。
- 客户端最终配置合并（`snapshot`）按 market 进行：具体海外 market 与 `GLOBAL` 存在同一登录配置实例时，**具体 market 实例整体覆盖 GLOBAL 实例**（实例级覆盖，非字段级，见 spec / `00` §3.2）。被隐藏/不兼容/`config_status != valid` 或 `enabled=false` 的实例不进入合并。

> 实务说明：`channel_only` 渠道当前均为国内渠道（`region=domestic`），按 `00` §3.2 可见性规则只在 `market=CN` 出现，因此跨 market 覆盖在本模块多数情况下不发生；但模型层面仍遵循实例级覆盖规则，不做特例。

### 5.5 登录锁定（`login_locked`）语义

- `channel_policies.login_locked`（平台级渠道策略，`00`/DDL）表示该渠道的登录模式被**锁定不可改**。对 `channel_only` 且 `login_locked=TRUE` 的渠道：运营**不能**把该渠道实例改成走自有账号认证，必须配置渠道强制登录；本模块据此**强制要求**该实例填写并通过登录配置（建议在快照/同步前置校验中，若 `enabled` 期望为真而 `config_status != valid` 则告警/拦截）。
- `login_locked=FALSE` 时模式可调整，但只要渠道实为 `channel_only`，仍由本模块承载其登录配置。
- 注意区分两个"锁"：`channel_policies.login_locked` 是**渠道登录模式锁**（平台级、影响是否强制走渠道登录）；与字段级 `*_locked`（如包覆盖锁）无关。

### 5.6 启用与有效性约束

- `enabled=TRUE` 但 `config_status != valid`：允许保存（不阻断编辑），但前端必须显著告警（`00`/前端"不得隐藏异常态"），且该实例**不进入快照 / 同步 / 客户端最终配置**（等价于"无效数据"，`00` §9）。
- 被隐藏（`channel` 的隐藏状态）或与当前 market 不兼容的渠道实例：其登录配置一律不参与快照/同步/运行时（`00` §9 红线）。

---

## 6. 后端 API（遵循 `00` §7 包络）

统一前缀 `/api/admin`；除登录类外均需 `Authorization: Bearer <accessToken>`；请求/响应 JSON 字段 camelCase；写操作作用于**当前运行环境** env（不接受前端指定 env）。本模块两个接口：

```text
GET /api/admin/game-channels/{gameChannelId}/login-config   读取（权限 channel.read）
PUT /api/admin/game-channels/{gameChannelId}/login-config   整体更新（权限 channel.write）
```

### 6.1 GET `/api/admin/game-channels/{gameChannelId}/login-config`

- 用途：读取指定 `GameMarketChannel` 实例的渠道登录配置 + 其驱动模板四件套（供前端渲染）。
- 路径参数：`gameChannelId`（int64，`game_channels.id`）。
- 鉴权权限码：`channel.read`。
- 行为：
  - 校验 `gameChannelId` 在当前 env 存在；不存在 ⇒ 404 `NOT_FOUND`。
  - 校验该渠道 `login_mode == channel_only`；否则 ⇒ 400 `VALIDATION_FAILED`（"该渠道非 channel_only"）。
  - 配置实例不存在 ⇒ 返回 `config` 为"空配置占位"（`enabled=false`、`configJson={}`、`configStatus="empty"`），并附模板，便于前端直接进入编辑态。
  - 响应中密文字段一律脱敏为 `"******"`（`00` §6.1）。

成功响应（实例已存在、含一个密文字段）：

```json
{
  "data": {
    "gameChannelId": 5001,
    "env": "sandbox",
    "channelId": "huawei_cn",
    "marketCode": "CN",
    "loginMode": "channel_only",
    "loginLocked": true,
    "config": {
      "enabled": true,
      "configJson": {
        "appId": "1010101",
        "appSecret": "******"
      },
      "configStatus": "valid",
      "lastCheckAt": "2026-06-16T08:30:00Z",
      "lastCheckMessage": "ok"
    },
    "template": {
      "templateVersion": "v1",
      "formSchemaJson": [
        { "key": "appId",     "label": "App ID",     "component": "input",    "required": true, "order": 10, "group": "basic" },
        { "key": "appSecret", "label": "App Secret", "component": "password", "required": true, "order": 20, "group": "secret" }
      ],
      "secretFieldsJson": ["appSecret"],
      "fileFieldsJson": [],
      "validationRulesJson": {
        "appId":     { "minLen": 1, "maxLen": 64, "pattern": "^[0-9A-Za-z_-]+$" },
        "appSecret": { "minLen": 8, "maxLen": 256 }
      }
    }
  }
}
```

配置实例不存在时（占位）：

```json
{
  "data": {
    "gameChannelId": 5002,
    "env": "sandbox",
    "channelId": "xiaomi_cn",
    "marketCode": "CN",
    "loginMode": "channel_only",
    "loginLocked": false,
    "config": {
      "enabled": false,
      "configJson": {},
      "configStatus": "empty",
      "lastCheckAt": null,
      "lastCheckMessage": ""
    },
    "template": {
      "templateVersion": "v1",
      "formSchemaJson": [
        { "key": "appId",     "label": "App ID",     "component": "input",    "required": true, "order": 10, "group": "basic" },
        { "key": "appSecret", "label": "App Secret", "component": "password", "required": true, "order": 20, "group": "secret" }
      ],
      "secretFieldsJson": ["appSecret"],
      "fileFieldsJson": [],
      "validationRulesJson": {
        "appId":     { "minLen": 1, "maxLen": 64, "pattern": "^[0-9A-Za-z_-]+$" },
        "appSecret": { "minLen": 8, "maxLen": 256 }
      }
    }
  }
}
```

### 6.2 PUT `/api/admin/game-channels/{gameChannelId}/login-config`

- 用途：整体 upsert 指定 `GameMarketChannel` 的渠道登录配置（无则创建，有则覆盖）。
- 路径参数：`gameChannelId`（int64）。
- 鉴权权限码：`channel.write`。
- 请求 DTO（camelCase）：

| 字段 | 类型 | 必填 | 默认 | 校验/说明 |
| --- | --- | --- | --- | --- |
| `enabled` | boolean | 否 | `false` | 是否启用渠道强制登录 |
| `configJson` | object | 是 | `{}` | 模板字段值；key 必须属于 `form_schema_json`；密文字段可传明文（将被加密）或传 `"******"` 表示"保持原值不变" |
| `templateVersion` | string | 否 | 当前 `published` | 指定按哪个模板版本校验；缺省取当前 published |

> 不接受 `env`（由当前运行环境决定）、不接受 `configStatus`/`lastCheck*`（后端推导/写入）。

请求体示例（华为，首次填写明文密钥）：

```json
{
  "enabled": true,
  "configJson": {
    "appId": "1010101",
    "appSecret": "Hx7sR2k9PqLm0Zce"
  }
}
```

请求体示例（仅改 appId，保持密钥不变）：

```json
{
  "enabled": true,
  "configJson": {
    "appId": "1010102",
    "appSecret": "******"
  }
}
```

成功响应（200，回显脱敏 + 推导后的状态）：

```json
{
  "data": {
    "gameChannelId": 5001,
    "env": "sandbox",
    "config": {
      "enabled": true,
      "configJson": {
        "appId": "1010101",
        "appSecret": "******"
      },
      "configStatus": "valid",
      "lastCheckAt": "2026-06-16T09:00:00Z",
      "lastCheckMessage": "ok"
    }
  }
}
```

校验失败响应（400，缺必填密钥；状态落 `invalid`）：

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "缺少必填敏感字段或文件字段",
    "details": [
      { "field": "appSecret", "rule": "required", "message": "appSecret is required" }
    ]
  }
}
```

> 设计取舍：即便校验失败，服务端可选择仍 upsert 已结构化的普通字段并把 `config_status` 落 `invalid`、`last_check_message` 写明原因（与"复制创建后强制 invalid"一致的语义），同时 HTTP 返回 400 提示前端修正——这样前端再次 GET 能看到 `invalid` 行内告警，不会"看似保存成功却无效"。具体是否落库由实现统一约定，本文推荐"落库 + 返回 400"。

### 6.3 错误码（沿用 `00` §7.4，本模块用到）

| code | HTTP | 触发场景 |
| --- | --- | --- |
| `UNAUTHENTICATED` | 401 | 未带/失效 token |
| `FORBIDDEN` | 403 | 无 `channel.read`/`channel.write` |
| `NOT_FOUND` | 404 | `gameChannelId` 在当前 env 不存在 |
| `VALIDATION_FAILED` | 400 | 非 channel_only 渠道 / 未知字段 / 必填或规则未过 / 无登录模板 |
| `CONFLICT` | 409 | 并发写导致唯一键冲突（`(env, game_channel_id_ref)`） |
| `INTERNAL` | 500 | 加密/持久化等内部错误 |

### 6.4 审计（`00` §8）

PUT 成功写 `audit_logs`：`action="channel.login_config.update"`、`resource_type="game_channel_login_config"`、`resource_id=gameChannelId`、`env=当前env`、`detail_json` 记录 before/after（密文脱敏，仅记录 `config_status`/`enabled`/普通字段变更，密文只记"changed: true/false"）。

---

## 7. 应用服务（`ChannelLoginService`）

落在 `internal/app/command` + `internal/app/query`，编排 `internal/domain/channel`，依赖仓储 / `infra/crypto` / `infra/file`。仓储保持窄（`01` §4.2）。

### 7.1 职责

- `GetLoginConfig(ctx, env, gameChannelID)`：取渠道实例 + 校验 `channel_only` + 取 published 模板 + 取/构造配置实例 + 脱敏 ⇒ 组装 GET 响应。
- `UpsertLoginConfig(ctx, env, gameChannelID, cmd)`：校验 `channel_only` → 取模板 → 模板驱动校验（§5.2）→ 密文加密/文件引用化（含 `"******"` 保留原密文逻辑）→ 推导 `config_status`/`last_check_*`（§5.3）→ upsert（`UNIQUE(env, game_channel_id_ref)`）→ 写审计 → 返回脱敏结果。

### 7.2 依赖

- `GameChannelRepository`（读 `game_channels`：env / market_code / channel_id_ref）。
- `ChannelPolicyRepository`（读 `channel_policies`：`login_mode` / `login_locked`）。
- `ChannelLoginTemplateRepository`（读 `channel_login_templates`：按 `channel_id_ref` 取 published 版本）。
- `ChannelLoginConfigRepository`（`game_channel_login_configs` 的 Get/Upsert，方法均带 `ctx` + `env`）。
- `infra/crypto`（AES-GCM 加解密密文字段）、`infra/file`（文件字段引用）。
- `AuditWriter`（写 `audit_logs`）。

### 7.3 仓储接口（示意）

```go
type ChannelLoginConfigRepository interface {
    GetByGameChannel(ctx context.Context, env Environment, gameChannelID int64) (*ChannelLoginConfig, error) // 不存在返回 (nil, nil)
    Upsert(ctx context.Context, cfg *ChannelLoginConfig) error // 按 (env, game_channel_id_ref) upsert
}

type ChannelLoginTemplateRepository interface {
    GetPublishedByChannel(ctx context.Context, channelID int64) (*ChannelLoginTemplate, error)
    GetByChannelVersion(ctx context.Context, channelID int64, version string) (*ChannelLoginTemplate, error)
}
```

---

## 8. 前端（渠道强制登录配置页）

落在 `apps/admin-web`，按 `01` §5 与 frontend Phase 5（"Account Auth and Channel Login UX"）。

### 8.1 入口与可见性

- 位于游戏详情 → 渠道实例（GameMarketChannel）详情下的 **"渠道登录"** 页签（与"自有账号认证"页签**分开**，体现红线）。
- **仅对 `login_mode=channel_only` 的渠道实例展示**该页签；`account_system` 渠道不展示（改展示自有账号认证页签，属 `account-auth`）。
- 数据来源：`GET .../login-config`（同时拿到 `config` 与 `template`）。

### 8.2 模板驱动表单

- 统一走 `01` §5.3 的"模板驱动表单渲染器"，消费四件套：
  - `formSchemaJson`：按 `order` 渲染字段，`component` 决定控件（`input/password/textarea/number/select/switch/file/json`），`group` 分组，`required` 标必填。
  - `secretFieldsJson`：对应字段用 `password` 控件 + 脱敏展示（初始显示 `******`），未改动则提交 `"******"`（保持原值）。
  - `fileFieldsJson`：文件上传控件，受 `accept`/`maxSizeKB` 限制（本模块默认空 `[]`，华为示例无文件字段）。
  - `validationRulesJson`：前端做即时校验（与后端同源规则），后端再次校验为准。
- 顶部展示只读上下文：`marketCode`、`channelId`、`loginMode`、`loginLocked`、当前 `env`（`EnvironmentBadge`）。

### 8.3 config_status 展示（不得隐藏异常态）

- 行内/页头展示 `configStatus` 状态标签：`empty`（灰）/ `invalid`（红）/ `valid`（绿），并展示 `lastCheckMessage`、`lastCheckAt`。
- `enabled=true` 但 `configStatus!=valid` ⇒ 显著告警条："已启用但配置无效，将不会进入快照/同步/客户端最终配置"。
- 复制创建导致的 `invalid`：明确提示"缺少必填敏感字段或文件字段，请补齐密钥/文件后保存"。

### 8.4 密文脱敏交互

- 密文字段默认显示 `******`，聚焦/点击"修改"后清空再输入；不展示明文原值；不在任何前端日志/网络回显中出现明文（后端已脱敏）。
- 保存时：未修改的密文字段提交 `"******"`（后端识别为保持原值）；修改过则提交新明文（后端加密）。

### 8.5 运行态只读标记（`spec` 建议）

展示 `Included in Snapshot` / `Included in Sync` / `Included in Runtime Config` 只读标记；当实例被隐藏 / 不兼容 / `configStatus!=valid` / `enabled=false` 时，这些标记应同步变为"不生效"并给出原因。

### 8.6 API 客户端

`api/modules/channels.ts` 增加 `getLoginConfig(gameChannelId)` / `putLoginConfig(gameChannelId, payload)`，统一经 `http.ts` 解包 `{data}` 包络与错误处理；写操作挂 `channel.write` 权限指令（无权限置灰）。

---

## 9. 与公共能力关系

| 公共能力（`00`） | 本模块如何遵循 |
| --- | --- |
| env（D1 / §2） | `game_channel_login_configs` 加 `env`，唯一键 `(env, game_channel_id_ref)`；写入取当前运行环境；与 `game_channels.env` 一致 |
| 模板四件套（§4） | 消费 `channel_login_templates` 的 `form_schema_json`/`secret_fields_json`/`file_fields_json`/`validation_rules_json`；模板平台级、不带 env |
| ConfigStatus 状态机（§3.4） | `config_status` 后端推导 `empty/invalid/valid`；复制创建强制 `invalid` |
| 模板版本生命周期（§3.3） | 运行时只取该渠道当前 `published` 模板；版本维护归 system 模块 |
| 密文（§6.1） | `secret_fields_json` 字段 AES-GCM 加密落库，响应脱敏 `******`，明文禁止落库 |
| 文件（§6.2） | `file_fields_json` 字段存引用，复制创建清空 |
| API 包络（§7） | `{data}` / `{error{code,message,details}}`；camelCase；Bearer 鉴权 |
| 鉴权权限码（§7.5） | `channel.read` / `channel.write` |
| 审计（§8） | PUT 成功写 `audit_logs`，`action=channel.login_config.update`，before/after 脱敏 |
| 红线（§9） | 与 `account-auth` 自有账号认证、`auth` 管理员登录彻底分离；隐藏/不兼容/无效实例不进快照/同步/运行时 |
| `channel` 渠道实例 | 登录配置挂在 `game_channels`（GameMarketChannel）上；随 `section=channels` 下游进入同步集 |
| `snapshot` 快照 / `sync` 同步 | 仅 `valid` 且 `enabled` 且未隐藏/兼容的实例进入；按 market 实例级覆盖合并 |

---

## 10. 测试要点

## 接口场景矩阵（→ 见 `../../03-testing.md` §4）

> 维度定义见 `03-testing.md §4`（S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S6 跨env / S7 审计 / S8 脱敏 / S9 分页 / S10 事务回滚）。`✓`=覆盖，`—`=不适用。后端 manifest：`tests/backend/scenarios/channel-login.yaml`；前端 e2e：`tests/frontend/e2e/channels.spec.ts`（渠道登录页签）。

| 接口 | S1 | S2 | S3 | S4 | S5 | S6 | S7 | S8 | S9 | S10 | 模块私有维度 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GET /api/admin/game-channels/{gameChannelId}/login-config | ✓ | ✓ | ✓ | ✓ | — | ✓ | — | ✓ | — | — | 非 channel_only 渠道 GET ⇒ VALIDATION_FAILED；secret 回显脱敏（S8）；config_status 回显；实例不存在返回空配置占位 |
| PUT /api/admin/game-channels/{gameChannelId}/login-config | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | ✓ | 仅 channel_only 渠道可写（否则 VALIDATION_FAILED）；与 account_system 隔离；secret 脱敏（S8）；config_status 后端推导；UNIQUE(env, game_channel_id_ref) 冲突→CONFLICT；复制创建强制 invalid；隐藏/不兼容不进快照/同步；upsert+审计同事务回滚 |

前端：channels.spec.ts 渠道登录页签 e2e（empty/invalid/valid 状态标签、enabled 但 invalid 告警条、密文 `******` 脱敏与"修改"重填、仅 channel_only 渠道展示页签） / vitest 组件（模板驱动表单渲染器、config_status 状态标签、密文脱敏交互组件）

### 补充关键用例（后端 / 前端）

### 10.1 数据/迁移

- 迁移后 `game_channel_login_configs` 含 `env` 且唯一键为 `(env, game_channel_id_ref)`；同一 `(env, game_channel_id_ref)` 重复 upsert 不产生重复行。
- 不同 env 下同一 `game_channel_id_ref` 可各存一条（验证 env 维度隔离）。
- `channel_login_templates` 无 env 列（平台级）。

### 10.2 适用性 / 红线

- 对 `account_system` 渠道实例调用 PUT ⇒ `VALIDATION_FAILED`。
- 渠道登录配置写入**不影响** `game_account_auth_configs`（`account-auth`），反之亦然（验证两套表/服务隔离）。

### 10.3 模板驱动校验 + config_status

- 空 `config_json` ⇒ `empty`。
- 缺必填 `appId` 或 `appSecret` ⇒ `invalid`，`last_check_message` 含具体字段。
- `appId` 不满足 `pattern` / `appSecret` 短于 `minLen` ⇒ `invalid`。
- 全字段合规 ⇒ `valid`，`last_check_at` 被更新。
- `config_json` 含未声明 key ⇒ `VALIDATION_FAILED`。

### 10.4 密文

- 首次写明文 `appSecret` ⇒ 库中为密文（非明文），GET 回显 `******`。
- 仅改 `appId`、`appSecret` 传 `"******"` ⇒ 库中密文不变（不被覆盖、不被清空）。
- 任何响应/审计不出现明文密钥。

### 10.5 复制创建强约束

- 从其它 market 同渠道复制创建：普通字段带入、`appSecret` 清空 ⇒ `config_status=invalid`（非 `empty`），`last_check_message="缺少必填敏感字段或文件字段"`。

### 10.6 按 market 独立 / 运行态

- 两个不同 market 的同渠道实例各自配置，互不影响（改一个不影响另一个的 `config_json`/状态）。
- `enabled=true` 但 `config_status!=valid` 的实例：不进入快照/同步/运行时合并（与 `snapshot` / `sync` 联调用例）。
- 隐藏/不兼容实例的登录配置被排除（红线用例）。

### 10.7 API / 鉴权 / 审计

- 无 `channel.write` ⇒ PUT 返回 403。
- 不存在的 `gameChannelId` ⇒ 404。
- PUT 成功写一条 `audit_logs`（`action=channel.login_config.update`，密文脱敏）。

---

## 11. 未决问题与假设

### 11.1 假设（已按以下默认推进）

1. **判定 channel_only 以 `channel_policies.login_mode` 为准**，不硬编码渠道名；`huawei_cn/xiaomi_cn/oppo_cn/vivo_cn` 仅作示例。
2. **PUT 校验失败时落库 + 返回 400**（§6.2 取舍），使前端二次 GET 能看到 `invalid` 行内态；如团队倾向"失败则完全不落库"，可改为纯 400，状态机语义不变。
3. **模板版本运行时取当前 `published`**；`templateVersion` 入参仅用于显式指定校验版本（少见）。模板版本本身的发布/归档不在本模块。
4. **小游戏渠道**（`wechat_mini_game`/`douyin_mini_game`）是否属 `channel_only` 以 `channel_policies` seed 为准；若为 `channel_only` 则同样适用本模块。
5. 密文"保持原值"哨兵统一用 `"******"`；与 GET 脱敏值一致，便于前端透传。

### 11.2 未决问题（待 system / `channel` / `snapshot` 协同确认）

1. **多密文字段的部分更新**：当模板含多个 secret 字段时，是否允许"逐字段独立保持/更新"？本文按"传 `******` 即保持、传明文即更新"逐字段处理，需 system 模块模板规范确认是否所有 secret 控件都遵循该哨兵约定。
2. **登录配置是否需要"复制初始值"的专用接口**：当前复制发生在 `channel` 的"新增渠道实例"流程内（连带复制下游登录配置普通字段、清空 secret/file）。是否需要本模块单独暴露 `POST .../login-config:copy-from` 待定；当前假设不需要。
3. **`login_locked=TRUE` 且 `config_status!=valid` 时的强拦截位置**：在 PUT 即拦截，还是仅在快照/同步前置校验拦截？本文建议放在快照/同步前置（`snapshot` / `sync`），PUT 仍允许保存草稿态以便分步填写；需与 `snapshot` / `sync` 对齐拦截口径。
4. **模板缺失时的降级**：若某 `channel_only` 渠道尚未配置 `channel_login_templates`，当前按"拒绝写入"。是否需要一个"无模板渠道"的兜底自由表单待 system 模块决定（默认不提供，保持模板强约束）。
