---
id: channel-login
code: "14"
title: 渠道登录（渠道强制登录配置）— 代码生成精简规格
kind: compact-spec
source: ./README.md
depends_on: [channel, game, common]
code_paths:
  - services/admin-api/internal/domain/channel
  - services/admin-api/internal/transport/http/channels
  - apps/admin-web/src/views/channels
---

# 14 · 渠道登录 — Compact Spec

> 代码生成用精简规格。完整背景/示例/测试矩阵见 `./README.md`。前置契约见 `../../00-common.md`（env 模型 D1 §2、模板四件套 §4、ConfigStatus §3.4、密文 §6、API 包络/错误码 §7、审计 §8）与 `../../01-structure.md`（分层/目录/装配）。
> 一句话定位：当游戏在某 market 接入**联运渠道（`channel_policies.login_mode=channel_only`，如 `huawei_cn`/`xiaomi_cn`/`oppo_cn`/`vivo_cn`）**时，玩家须走渠道自身登录体系。本模块维护**每个 `GameMarketChannel` 实例**的渠道登录参数（appId/appSecret 等），模板驱动渲染校验，配置落 `game_channel_login_configs`。

## 边界 / 红线
- **三套登录体系彻底分离**（红线）：`channel-login`（玩家走渠道登录，本模块）≠ `account-auth`（游戏自有账号体系）≠ `auth`（后台管理员登录）。分表、分领域服务、分前端页，不共享配置实例/密钥/状态。
- 判定是否走本模块的**唯一依据**是 `channel_policies.login_mode == 'channel_only'`，非渠道名硬编码。`account_system` 走 `account-auth`。
- 配置粒度：渠道实例级（挂 `game_channels` 行，按 `(game, market, channel)` 独立），**非** game 级。
- 负责：消费 `channel_login_templates`（只读校验）+ 维护 `game_channel_login_configs` + 单实例读/整体更新 + 模板校验推导 `config_status` + 密文加密脱敏。
- 不负责：渠道实例增删（`channel`）、渠道包/IAP（`channel`/`product`）、模板版本维护（system 模块）、快照合并（`snapshot`）、sandbox→prod 同步（`sync`，本模块数据随 `section=channels` 下游进入同步集）。

## 数据模型
两表：`channel_login_templates`（平台级模板，共享 schema `platform`，**不带 env**）+ `game_channel_login_configs`（配置实例，**游戏维度业务表/每环境独立 schema/不带 env 列**）。公共列约定：`id BIGSERIAL PK`、`created_at/updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`。

### channel_login_templates（平台级，简单模板表，四件套）
结构与其它 `*_templates` 一致（00 §4）。
- `channel_id_ref` BIGINT NOT NULL FK→channels(id)（指向逻辑渠道）
- `template_version` VARCHAR(32) NOT NULL
- 四件套：`form_schema_json`/`secret_fields_json`/`file_fields_json` JSONB NOT NULL DEFAULT `'[]'`；`validation_rules_json` JSONB NOT NULL DEFAULT `'{}'`
- `enabled` BOOLEAN NOT NULL DEFAULT TRUE
- UNIQUE(channel_id_ref, template_version)
- **简单模板表**（00 §4.4.1）：不走 §3.3 三态机；运行时只取该渠道 `enabled=TRUE` 的最新 `template_version`。

### game_channel_login_configs（游戏维度业务表 / 每环境 schema / 不带 env 列）
```sql
CREATE TABLE game_channel_login_configs (
  id                  BIGSERIAL PRIMARY KEY,
  game_channel_id_ref BIGINT NOT NULL REFERENCES game_channels(id),  -- 同 schema 普通 FK；指向 GameMarketChannel（自带 market_code）
  enabled             BOOLEAN NOT NULL DEFAULT FALSE,
  config_json         JSONB NOT NULL DEFAULT '{}'::jsonb,            -- 含加密后密文位
  config_status       VARCHAR(16) NOT NULL DEFAULT 'empty' CHECK (config_status IN ('empty','invalid','valid')),
  last_check_at       TIMESTAMPTZ,
  last_check_message  VARCHAR(255) NOT NULL DEFAULT '',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_channel_id_ref)            -- 每实例 0..1 条；schema 内自成体系，不前置 env
);
```
- `game_channel_id_ref` 已隐含 `(game, market, channel)` 全维度（`game_channels` 唯一键 = `(game_id_ref, market_code, channel_id_ref)`），故唯一键只需 `(game_channel_id_ref)`。
- 运行时连接 `search_path = <env>, platform`；业务表仓储 SQL 不写 schema 前缀、不带 env 谓词。父子行必同 schema=同 env，普通外键即可。

## 枚举与默认
- `ConfigStatus` ∈ {empty, invalid, valid}，默认 empty。
- `LoginMode` ∈ {channel_only, account_system}，默认 account_system（仅 channel_only 需本模块）。
- `Environment` ∈ {develop, sandbox, production}（运行时由 search_path 决定，非表列）。`Market` 经 `game_channels.market_code` 间接体现，本模块不直接存。
- config 实例：enabled 默认 FALSE，config_json `{}`，config_status `empty`，last_check_at NULL，last_check_message `''`。
- 模板：form/secret/file 默认 `[]`，validation 默认 `{}`，enabled 默认 TRUE。

## 业务规则与状态机
1. **适用性**：仅 `channel_policies.login_mode=='channel_only'` 的实例需要/允许配置；对 `account_system` 实例 GET/PUT 均拒绝（`VALIDATION_FAILED`，"该渠道非 channel_only"）；前端不展示页签。
2. **模板驱动校验**（PUT 时按序）：① 由 `game_channel_id_ref` 反查 `game_channels` 得 `channel_id_ref`，取该渠道 `enabled=TRUE` 最新 `template_version`（或显式 `templateVersion`）；模板不存在⇒拒绝。② 未知字段（不在 form_schema）⇒ `VALIDATION_FAILED`。③ 必填校验（含 secret/file 标记字段非空）。④ 逐字段 `validation_rules_json` 校验（minLen/maxLen/min/max/pattern/format/enum，00 §4.3）。⑤ 密文：secret_fields 标记字段 AES-GCM 加密存 config_json 密文位（明文禁止落库）；传哨兵 `"******"`/`"masked"` 表示"未修改"，保留原密文。⑥ 文件：file_fields 存引用（storage key/hash），不存二进制。⑦ 通过则推导 status + 写 last_check_at=NOW()/last_check_message。
3. **config_status 推导**（00 §3.4，后端推导持久化，请求体携带的 configStatus 忽略）：
   - config_json 为空 `{}` 无字段 ⇒ `empty`。
   - 有字段但缺必填（含 secret/file）或任一校验未过 ⇒ `invalid`（last_check_message 写首个失败原因）。
   - 全必填补齐且校验通过 ⇒ `valid`。
   - **复制创建强约束（红线）**：从其它 market 同渠道复制初始值、secret/file 被清空时，**必须 `invalid`，绝不 `empty`**，last_check_message 必为"缺少必填敏感字段或文件字段"。
4. **按 market 实例独立**：登录配置随 `game_channels` 按 `(game, market, channel)` 独立，config_json/密钥/文件/status/enabled 全独立、互不影响。新增 market 实例可"从其它 market 同渠道复制"：仅复制普通字段，**secret/file 必须清空**，新实例初始 invalid，复制后不联动。快照合并按 market：具体 market 实例**整体覆盖** GLOBAL 实例（实例级覆盖，非字段级）；非 valid/未启用/隐藏/不兼容不进合并。
5. **login_locked 语义**：`channel_policies.login_locked`（渠道登录模式锁，平台级）= TRUE 时该渠道实例不能改走自有账号认证，必须配置渠道强制登录（建议在快照/同步前置校验：期望 enabled 但 status!=valid 则告警/拦截）。区分于字段级 `*_locked`（包覆盖锁，无关）。
6. **启用与有效性**：enabled=TRUE 但 status!=valid 允许保存（不阻断编辑），但前端必须显著告警，且该实例**不进快照/同步/客户端最终配置**（00 §9）。隐藏/不兼容实例的登录配置一律不参与快照/同步/运行时。
- 纯领域校验：模板四件套对 config_json 校验 → `(config_status, last_check_message)`（无 IO）。

## 后端 API（前缀 /api/admin，包络 00 §7；读 channel.read / 写 channel.write）
两接口，单实例读取 + 整体 upsert。写操作落当前运行环境对应 schema（不接受前端指定/跨 schema 写）。

### GET `/api/admin/game-channels/{gameChannelId}/login-config`（channel.read）
读取实例配置 + 驱动模板四件套（供前端渲染）。行为：校验 `gameChannelId` 在当前 env schema 存在（否则 404）；校验 `login_mode==channel_only`（否则 400 VALIDATION_FAILED）；实例不存在⇒返回空配置占位（enabled=false/configJson={}/configStatus=empty）+ 模板；密文字段脱敏 `"******"`。
→ data: { gameChannelId, env, channelId, marketCode, loginMode, loginLocked, config:{ enabled, configJson(脱敏), configStatus, lastCheckAt, lastCheckMessage }, template:{ templateVersion, formSchemaJson[], secretFieldsJson[], fileFieldsJson[], validationRulesJson{} } }

form_schema 项结构: { key, label, component, required, order, group }。
示例 huawei: secretFieldsJson=["appSecret"]；validationRulesJson={ appId:{minLen:1,maxLen:64,pattern:"^[0-9A-Za-z_-]+$"}, appSecret:{minLen:8,maxLen:256} }。

### PUT `/api/admin/game-channels/{gameChannelId}/login-config`（channel.write，整体 upsert）
请求 DTO（camelCase；不接受 env/configStatus/lastCheck*）：
| 字段 | 类型 | 必填 | 默认 | 校验/说明 |
| --- | --- | --- | --- | --- |
| enabled | bool | 否 | false | 是否启用渠道强制登录 |
| configJson | object | 是 | {} | key 必须属 form_schema_json；密文字段传明文（将加密）或 `"******"`（保持原值不变） |
| templateVersion | string | 否 | 最新 enabled 版本 | 指定校验所用模板版本 |

→ 成功 200：回显脱敏 configJson + 推导后 configStatus/lastCheckAt/lastCheckMessage。
→ 校验失败（缺必填密钥）400 VALIDATION_FAILED，details[]: { field, rule, message }。**推荐"落库（普通字段 + config_status=invalid + last_check_message）+ 返回 400"**，使前端二次 GET 能看到 invalid 行内态。

错误码（00 §7.4）：`UNAUTHENTICATED`(401)、`FORBIDDEN`(403 无 channel.read/write)、`NOT_FOUND`(404 gameChannelId 不存在)、`VALIDATION_FAILED`(400 非 channel_only/未知字段/必填或规则未过/无模板)、`CONFLICT`(409 唯一键 game_channel_id_ref 冲突)、`INTERNAL`(500)。
审计（00 §8）：PUT 成功写 audit_logs：`action="channel.login_config.update"`、`resource_type="game_channel_login_config"`、`resource_id=gameChannelId`、`env=当前env`、detail_json 记 before/after（密文脱敏，密文只记 "changed: true/false"）。

## 应用服务 / 仓储（ChannelLoginService，internal/app command+query，编排 internal/domain/channel）
- `GetLoginConfig(ctx, env, gameChannelID)`：取实例 + 校验 channel_only + 取最新 enabled 模板 + 取/构造配置 + 脱敏 ⇒ GET 响应。
- `UpsertLoginConfig(ctx, env, gameChannelID, cmd)`：校验 channel_only → 取模板 → 模板驱动校验(§规则2) → 密文加密/文件引用化（含 `"******"` 保留原值）→ 推导 config_status/last_check_* → upsert(UNIQUE(game_channel_id_ref)) → 写审计 → 返回脱敏结果。
- 依赖仓储：`GameChannelRepository`（读 env/market_code/channel_id_ref）、`ChannelPolicyRepository`（login_mode/login_locked）、`ChannelLoginTemplateRepository`、`ChannelLoginConfigRepository`；`infra/crypto`(AES-GCM)、`infra/file`、`AuditWriter`。仓储窄，SQL 不写 schema 前缀/不带 env 谓词。

```go
type ChannelLoginConfigRepository interface {
    GetByGameChannel(ctx context.Context, gameChannelID int64) (*ChannelLoginConfig, error) // 不存在返回 (nil, nil)
    Upsert(ctx context.Context, cfg *ChannelLoginConfig) error                              // 按 (game_channel_id_ref) upsert
}
type ChannelLoginTemplateRepository interface {
    GetPublishedByChannel(ctx context.Context, channelID int64) (*ChannelLoginTemplate, error)
    GetByChannelVersion(ctx context.Context, channelID int64, version string) (*ChannelLoginTemplate, error)
}
```

领域对象（internal/domain/channel，归 `channel.GameChannelAggregate`）：`ChannelLoginConfig{ ID, GameChannelID, Enabled, ConfigJSON, ConfigStatus, LastCheckAt, LastCheckMessage, ... }`、`ChannelLoginTemplate{ ID, ChannelID, TemplateVersion, FormSchemaJSON[], SecretFieldsJSON[], FileFieldsJSON[], ValidationRulesJSON{}, Enabled }`。

## 前端要点（游戏详情 → GameMarketChannel 详情 → "渠道登录"页签）
- 入口可见性：**仅对 `login_mode=channel_only` 实例展示**该页签（与"自有账号认证"页签分开，体现红线）；数据来自 `GET .../login-config`（config + template 一并取回）。
- 模板驱动表单（01 §5.3 统一渲染器，消费四件套）：formSchema 按 order 渲染、component 决控件（input/password/textarea/number/select/switch/file/json）、group 分组、required 标必填；secretFields 用 password + 脱敏（初始 `******`，未改提交 `"******"`）；fileFields 上传控件（受 accept/maxSizeKB）；validationRules 前端即时校验（后端为准）。顶部只读上下文：marketCode/channelId/loginMode/loginLocked/env(EnvironmentBadge)。
- config_status 展示（不得隐藏异常态）：empty(灰)/invalid(红)/valid(绿) + lastCheckMessage/lastCheckAt；enabled=true 但 status!=valid ⇒ 显著告警条"已启用但配置无效，将不进入快照/同步/客户端最终配置"；复制创建 invalid 提示补齐密钥/文件。
- 密文脱敏：默认 `******`，聚焦/点"修改"清空再输入；不展示明文；保存时未改提交 `"******"`、改过提交新明文。
- 运行态只读标记：Included in Snapshot/Sync/Runtime Config；隐藏/不兼容/status!=valid/enabled=false 时标"不生效"+原因。
- API 客户端：`api/modules/channels.ts` 加 `getLoginConfig(gameChannelId)`/`putLoginConfig(gameChannelId, payload)`，经 `http.ts` 解包 `{data}`；写操作挂 `channel.write` 权限指令（无权限置灰）。

## 与公共能力 / 下游
- env(00 D1/§2)：每环境独立 schema、不带 env 列、唯一键 `(game_channel_id_ref)`；写落当前 env schema；普通外键即可。
- 模板四件套(00 §4)：消费 channel_login_templates；简单模板表(§4.4.1)只取 enabled 最新版本，不走三态机；版本维护归 system。
- ConfigStatus(00 §3.4)：后端推导，复制创建强制 invalid。密文(00 §6.1)：AES-GCM 加密落库、响应脱敏 `******`、明文禁落库；文件(00 §6.2)存引用、复制清空。
- API 包络(00 §7) / 鉴权码 channel.read·channel.write(00 §7.5) / 审计(00 §8)。
- `channel`：配置挂 game_channels，随 `section=channels` 下游进同步集。`snapshot`/`sync`：仅 valid+enabled+未隐藏/兼容实例进入；按 market 实例级覆盖合并。

## 关键假设
- 判定 channel_only 以 `channel_policies.login_mode` 为准，不硬编码渠道名；`huawei_cn/xiaomi_cn/oppo_cn/vivo_cn` 仅示例（小游戏 `wechat_mini_game`/`douyin_mini_game` 是否属 channel_only 以 seed 为准）。
- PUT 校验失败时"落库 + 返回 400"（可改纯 400，状态机语义不变）。
- 模板版本运行时取 `enabled=TRUE` 最新 `template_version`；templateVersion 入参仅用于显式指定校验版本。
- 密文"保持原值"哨兵统一 `"******"`（与 GET 脱敏值一致）；多密文字段按"传 `******` 保持、传明文更新"逐字段处理。
- `login_locked=TRUE` 且 status!=valid 的强拦截建议放快照/同步前置（PUT 仍允许保存草稿态）；待与 snapshot/sync 对齐。
- 模板缺失的 channel_only 渠道按"拒绝写入"，不提供无模板兜底自由表单（保持模板强约束）。
- 复制初始值发生在 `channel` 新增渠道实例流程内，本模块当前不单独暴露 `:copy-from` 接口。
