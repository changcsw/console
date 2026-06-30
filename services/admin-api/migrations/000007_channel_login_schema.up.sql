-- 000007 · channel-login 模块：渠道登录模板平台归位 + 渠道实例登录配置表存在性/约束兜底 + seed
-- 追加新文件、不改历史；全部 IF EXISTS / IF NOT EXISTS / 存在性判断，可重复前向执行。
-- 说明：channel_login_templates（平台模板）与 game_channel_login_configs（游戏维度业务表）
--   已在 000001_init 建于默认 schema。本迁移负责：
--     1) 把 channel_login_templates 归位到 platform（与 channels 同 schema，承接其跨 schema FK，
--        对齐 000006 对 channels/account_auth 平台表的归位）。
--     2) 对业务表 game_channel_login_configs 做存在性/CHECK/索引兜底（幂等）。
--     3) 写入 huawei_cn v1 模板 seed 示例。
-- schema-per-env 下业务表在每环境 schema 各一份：迁移运行器按 search_path 落到当前 schema，
--   业务表 DDL 不写 schema 前缀、不带 env 谓词（01 §4.2/§6，对齐 000004/000005/000006）。
-- 约束名在 PostgreSQL 中按 namespace 隔离，存在性判断按 current_schema() 限定 connamespace。

CREATE SCHEMA IF NOT EXISTS platform;

-- ============ 1) 平台模板归位：channel_login_templates 落 platform ============
ALTER TABLE IF EXISTS public.channel_login_templates SET SCHEMA platform;

-- 平台表存在性兜底（新库直接按目标结构创建于 platform；四件套见 00 §4）。
CREATE TABLE IF NOT EXISTS platform.channel_login_templates (
  id                    BIGSERIAL PRIMARY KEY,
  channel_id_ref        BIGINT      NOT NULL REFERENCES platform.channels(id),
  template_version      VARCHAR(32) NOT NULL,
  form_schema_json      JSONB       NOT NULL DEFAULT '[]'::jsonb,
  secret_fields_json    JSONB       NOT NULL DEFAULT '[]'::jsonb,
  file_fields_json      JSONB       NOT NULL DEFAULT '[]'::jsonb,
  validation_rules_json JSONB       NOT NULL DEFAULT '{}'::jsonb,
  enabled               BOOLEAN     NOT NULL DEFAULT TRUE,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id_ref, template_version)
);

-- 运行时取「该渠道 enabled=TRUE 最新 template_version」的热路径索引。
CREATE INDEX IF NOT EXISTS idx_channel_login_templates_channel_enabled
  ON platform.channel_login_templates(channel_id_ref, enabled);

-- ============ 2) 业务表存在性/约束/索引兜底（游戏维度，当前 env schema 一份） ============
CREATE TABLE IF NOT EXISTS game_channel_login_configs (
  id                  BIGSERIAL    PRIMARY KEY,
  game_channel_id_ref BIGINT       NOT NULL REFERENCES game_channels(id),
  enabled             BOOLEAN      NOT NULL DEFAULT FALSE,
  config_json         JSONB        NOT NULL DEFAULT '{}'::jsonb,
  config_status       VARCHAR(16)  NOT NULL DEFAULT 'empty',
  last_check_at       TIMESTAMPTZ,
  last_check_message  VARCHAR(255) NOT NULL DEFAULT '',
  created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (game_channel_id_ref)
);

-- config_status 命名 CHECK 兜底（按 current_schema 限定，幂等；对齐 000005/000006）。
-- 000001_init 建表时已带内联 CHECK（自动命名 *_config_status_check）；仅当两种命名都缺失
-- （例如表由本迁移 CREATE TABLE 兜底新建、无内联 CHECK）时才补加，避免重复约束。
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname IN (
        'game_channel_login_configs_status_check',
        'game_channel_login_configs_config_status_check'
      )
      AND connamespace = current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_channel_login_configs
      ADD CONSTRAINT game_channel_login_configs_status_check
      CHECK (config_status IN ('empty', 'invalid', 'valid'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_game_channel_login_configs_game_channel
  ON game_channel_login_configs(game_channel_id_ref);

-- ============ 3) 平台模板 seed：huawei_cn v1 示例（appId 普通字段 + appSecret 密文） ============
INSERT INTO platform.channel_login_templates (
  channel_id_ref, template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json, enabled
)
SELECT ch.id, 'v1',
  '[{"key":"appId","label":"App ID","component":"input","required":true,"order":10,"group":"basic","scope":"both"},
    {"key":"appSecret","label":"App Secret","component":"password","required":true,"order":20,"group":"secret","scope":"server"}]'::jsonb,
  '["appSecret"]'::jsonb,
  '[]'::jsonb,
  '{"appId":{"minLen":1,"maxLen":64,"pattern":"^[0-9A-Za-z_-]+$"},"appSecret":{"minLen":8,"maxLen":256}}'::jsonb,
  TRUE
FROM platform.channels ch
WHERE ch.channel_id = 'huawei_cn'
ON CONFLICT (channel_id_ref, template_version) DO NOTHING;
