-- 000006 · account-auth 模块：平台表归位 + 游戏维度配置表约束/索引 + seed
-- 追加新文件，不改历史；所有操作保持幂等。

CREATE SCHEMA IF NOT EXISTS platform;

-- 0) 跨模块 bootstrap 归位：channels / channel_policies 落 platform。
--    channel 模块 000005 注明此归位属跨模块 bootstrap、推迟处理；account-auth 是首个硬依赖
--    platform.channels 的消费者（下方 FK 与 seed JOIN、account_auth_repo 运行态裸表名经
--    search_path=<env>,platform 解析），故在此承接归位。幂等：移动后 public.* 不存在则跳过。
ALTER TABLE IF EXISTS public.channels         SET SCHEMA platform;
ALTER TABLE IF EXISTS public.channel_policies SET SCHEMA platform;

-- 1) 平台级三表归位（若仍在 public，则迁移到 platform）
ALTER TABLE IF EXISTS public.account_auth_types          SET SCHEMA platform;
ALTER TABLE IF EXISTS public.channel_account_auth_types  SET SCHEMA platform;
ALTER TABLE IF EXISTS public.account_auth_templates      SET SCHEMA platform;

-- 2) 平台表存在性兜底（新库直接按目标结构创建）
CREATE TABLE IF NOT EXISTS platform.account_auth_types (
  id             BIGSERIAL PRIMARY KEY,
  auth_type_id   VARCHAR(64) NOT NULL,
  auth_type_name VARCHAR(64) NOT NULL,
  enabled        BOOLEAN     NOT NULL DEFAULT TRUE,
  sort           INT         NOT NULL DEFAULT 0,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (auth_type_id)
);

CREATE TABLE IF NOT EXISTS platform.channel_account_auth_types (
  id              BIGSERIAL PRIMARY KEY,
  channel_id_ref  BIGINT      NOT NULL REFERENCES platform.channels(id),
  auth_type_id_ref BIGINT     NOT NULL REFERENCES platform.account_auth_types(id),
  default_enabled BOOLEAN     NOT NULL DEFAULT FALSE,
  locked          BOOLEAN     NOT NULL DEFAULT FALSE,
  sort            INT         NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id_ref, auth_type_id_ref)
);

CREATE TABLE IF NOT EXISTS platform.account_auth_templates (
  id                    BIGSERIAL PRIMARY KEY,
  auth_type_id_ref      BIGINT      NOT NULL REFERENCES platform.account_auth_types(id),
  template_version      VARCHAR(32) NOT NULL,
  form_schema_json      JSONB       NOT NULL DEFAULT '[]'::jsonb,
  secret_fields_json    JSONB       NOT NULL DEFAULT '[]'::jsonb,
  file_fields_json      JSONB       NOT NULL DEFAULT '[]'::jsonb,
  validation_rules_json JSONB       NOT NULL DEFAULT '{}'::jsonb,
  enabled               BOOLEAN     NOT NULL DEFAULT TRUE,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (auth_type_id_ref, template_version)
);

CREATE INDEX IF NOT EXISTS idx_channel_account_auth_types_channel ON platform.channel_account_auth_types(channel_id_ref);
CREATE INDEX IF NOT EXISTS idx_channel_account_auth_types_auth    ON platform.channel_account_auth_types(auth_type_id_ref);

-- 3) 游戏维度配置表（当前 schema，业务表不带 env 列）
CREATE TABLE IF NOT EXISTS game_account_auth_configs (
  id                 BIGSERIAL PRIMARY KEY,
  game_id_ref        BIGINT      NOT NULL REFERENCES games(id),
  auth_type_id_ref   BIGINT      NOT NULL REFERENCES platform.account_auth_types(id),
  enabled            BOOLEAN     NOT NULL DEFAULT FALSE,
  config_json        JSONB       NOT NULL DEFAULT '{}'::jsonb,
  config_status      VARCHAR(16) NOT NULL DEFAULT 'empty',
  last_check_at      TIMESTAMPTZ,
  last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (game_id_ref, auth_type_id_ref)
);

ALTER TABLE game_account_auth_configs
  DROP CONSTRAINT IF EXISTS game_account_auth_configs_auth_type_id_ref_fkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='game_account_auth_configs_auth_type_fk'
      AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_account_auth_configs
      ADD CONSTRAINT game_account_auth_configs_auth_type_fk
      FOREIGN KEY (auth_type_id_ref) REFERENCES platform.account_auth_types(id);
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='game_account_auth_configs_status_check'
      AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_account_auth_configs
      ADD CONSTRAINT game_account_auth_configs_status_check
      CHECK (config_status IN ('empty', 'invalid', 'valid'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_game_account_auth_configs_game_id_ref
  ON game_account_auth_configs(game_id_ref);

-- 4) 平台 seed：认证类型
INSERT INTO platform.account_auth_types (auth_type_id, auth_type_name, enabled, sort)
VALUES
  ('guest',    '游客',            TRUE, 10),
  ('phone',    '手机号',          TRUE, 20),
  ('email',    '邮箱',            TRUE, 30),
  ('google',   'Google 登录',     TRUE, 40),
  ('apple',    'Apple 登录',      TRUE, 50),
  ('facebook', 'Facebook 登录',   TRUE, 60),
  ('line',     'LINE 登录',       TRUE, 70),
  ('kakao',    'Kakao 登录',      TRUE, 80)
ON CONFLICT (auth_type_id) DO UPDATE
SET auth_type_name=EXCLUDED.auth_type_name,
    enabled=EXCLUDED.enabled,
    sort=EXCLUDED.sort,
    updated_at=NOW();

-- 5) 默认渠道认证类型映射：仅对 login_mode=account_system 的渠道生效
INSERT INTO platform.channel_account_auth_types (channel_id_ref, auth_type_id_ref, default_enabled, locked, sort)
SELECT ch.id, at.id,
       CASE WHEN at.auth_type_id IN ('guest') THEN TRUE ELSE FALSE END AS default_enabled,
       FALSE AS locked,
       at.sort
FROM platform.channels ch
JOIN platform.channel_policies cp ON cp.channel_id_ref = ch.id
JOIN platform.account_auth_types at ON at.enabled = TRUE
WHERE cp.login_mode = 'account_system'
ON CONFLICT (channel_id_ref, auth_type_id_ref) DO NOTHING;

-- 6) 模板 seed（最小可用版本）
INSERT INTO platform.account_auth_templates (
  auth_type_id_ref, template_version, form_schema_json, secret_fields_json, file_fields_json, validation_rules_json, enabled
)
SELECT at.id, 'v1',
       CASE at.auth_type_id
         WHEN 'google' THEN
           '[{"key":"clientId","label":"Client ID","component":"input","required":true,"order":10,"scope":"both"},
             {"key":"clientSecret","label":"Client Secret","component":"password","required":true,"order":20,"scope":"server"},
             {"key":"redirectUri","label":"Redirect URI","component":"input","required":true,"order":30,"scope":"both"}]'::jsonb
         ELSE '[]'::jsonb
       END,
       CASE at.auth_type_id
         WHEN 'google' THEN '["clientSecret"]'::jsonb
         ELSE '[]'::jsonb
       END,
       '[]'::jsonb,
       CASE at.auth_type_id
         WHEN 'google' THEN '{"clientId":{"minLen":1},"redirectUri":{"format":"url"}}'::jsonb
         ELSE '{}'::jsonb
       END,
       TRUE
FROM platform.account_auth_types at
ON CONFLICT (auth_type_id_ref, template_version) DO NOTHING;
