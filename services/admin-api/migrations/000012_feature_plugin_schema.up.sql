-- 000012 · feature-plugin 模块：平台主数据/模板/渠道策略 + 渠道实例/渠道包插件配置
-- 说明：
--   - 平台表（schema platform）：feature_plugins / feature_plugin_templates / channel_feature_plugins
--   - 业务表（当前 env schema）：game_channel_plugin_configs / channel_package_plugin_overrides
-- 约束：
--   - 业务表 SQL 不写 schema 前缀（由 search_path=<env>,platform 决定）
--   - 幂等：IF NOT EXISTS + 可重复执行

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.feature_plugins (
  id          BIGSERIAL PRIMARY KEY,
  plugin_id   VARCHAR(64)  NOT NULL UNIQUE,
  plugin_name VARCHAR(64)  NOT NULL,
  region      VARCHAR(16)  NOT NULL CHECK (region IN ('domestic','overseas')),
  enabled     BOOLEAN      NOT NULL DEFAULT TRUE,
  sort        INT          NOT NULL DEFAULT 0,
  created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_feature_plugins_region ON platform.feature_plugins(region);
CREATE INDEX IF NOT EXISTS idx_feature_plugins_enabled_sort ON platform.feature_plugins(enabled, sort);

CREATE TABLE IF NOT EXISTS platform.feature_plugin_templates (
  id                    BIGSERIAL PRIMARY KEY,
  plugin_id_ref         BIGINT      NOT NULL REFERENCES platform.feature_plugins(id),
  template_version      VARCHAR(32) NOT NULL,
  form_schema_json      JSONB       NOT NULL DEFAULT '[]'::jsonb,
  secret_fields_json    JSONB       NOT NULL DEFAULT '[]'::jsonb,
  file_fields_json      JSONB       NOT NULL DEFAULT '[]'::jsonb,
  validation_rules_json JSONB       NOT NULL DEFAULT '{}'::jsonb,
  enabled               BOOLEAN     NOT NULL DEFAULT TRUE,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (plugin_id_ref, template_version)
);

CREATE INDEX IF NOT EXISTS idx_feature_plugin_templates_plugin_enabled_version
  ON platform.feature_plugin_templates(plugin_id_ref, enabled, template_version DESC);

CREATE TABLE IF NOT EXISTS platform.channel_feature_plugins (
  id              BIGSERIAL PRIMARY KEY,
  channel_id_ref  BIGINT      NOT NULL REFERENCES platform.channels(id),
  plugin_id_ref   BIGINT      NOT NULL REFERENCES platform.feature_plugins(id),
  required        BOOLEAN     NOT NULL DEFAULT FALSE,
  selectable      BOOLEAN     NOT NULL DEFAULT TRUE,
  default_enabled BOOLEAN     NOT NULL DEFAULT FALSE,
  locked          BOOLEAN     NOT NULL DEFAULT FALSE,
  sort            INT         NOT NULL DEFAULT 0,
  enabled         BOOLEAN     NOT NULL DEFAULT TRUE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id_ref, plugin_id_ref)
);

CREATE INDEX IF NOT EXISTS idx_channel_feature_plugins_channel_sort
  ON platform.channel_feature_plugins(channel_id_ref, sort);

CREATE TABLE IF NOT EXISTS game_channel_plugin_configs (
  id                 BIGSERIAL PRIMARY KEY,
  game_channel_id_ref BIGINT       NOT NULL REFERENCES game_channels(id),
  plugin_id_ref      BIGINT       NOT NULL REFERENCES platform.feature_plugins(id),
  enabled            BOOLEAN      NOT NULL DEFAULT FALSE,
  config_json        JSONB        NOT NULL DEFAULT '{}'::jsonb,
  config_status      VARCHAR(16)  NOT NULL DEFAULT 'empty',
  last_check_at      TIMESTAMPTZ,
  last_check_message VARCHAR(255) NOT NULL DEFAULT '',
  created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (game_channel_id_ref, plugin_id_ref)
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='game_channel_plugin_configs_status_check'
      AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE game_channel_plugin_configs
      ADD CONSTRAINT game_channel_plugin_configs_status_check
      CHECK (config_status IN ('empty','invalid','valid'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_game_channel_plugin_configs_gc
  ON game_channel_plugin_configs(game_channel_id_ref);

CREATE TABLE IF NOT EXISTS channel_package_plugin_overrides (
  id                     BIGSERIAL PRIMARY KEY,
  package_id_ref         BIGINT       NOT NULL REFERENCES channel_packages(id),
  plugin_id_ref          BIGINT       NOT NULL REFERENCES platform.feature_plugins(id),
  inherit_channel_config BOOLEAN      NOT NULL DEFAULT TRUE,
  enabled                BOOLEAN      NOT NULL DEFAULT FALSE,
  config_json            JSONB        NOT NULL DEFAULT '{}'::jsonb,
  config_status          VARCHAR(16)  NOT NULL DEFAULT 'empty',
  last_check_at          TIMESTAMPTZ,
  last_check_message     VARCHAR(255) NOT NULL DEFAULT '',
  created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  updated_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  UNIQUE (package_id_ref, plugin_id_ref)
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname='channel_package_plugin_overrides_status_check'
      AND connamespace=current_schema()::regnamespace
  ) THEN
    ALTER TABLE channel_package_plugin_overrides
      ADD CONSTRAINT channel_package_plugin_overrides_status_check
      CHECK (config_status IN ('empty','invalid','valid'));
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_channel_package_plugin_overrides_package
  ON channel_package_plugin_overrides(package_id_ref);
