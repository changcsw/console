-- 000006 down · account-auth 回滚（best effort）

-- 0) 反向 seed（与 up 步骤 5/6 对称）。channel_account_auth_types / account_auth_templates 仅由 000006 seed，
--    必须先清掉这些 referencing 行，否则后续 000002 down 删除 account_auth_types 会被跨表 FK 阻断。
--    此时这些平台表与 channels/channel_policies 仍在 platform（schema 反向归位在本文件末尾执行）。
DELETE FROM platform.account_auth_templates WHERE template_version = 'v1';

DELETE FROM platform.channel_account_auth_types
WHERE channel_id_ref IN (
  SELECT ch.id
  FROM platform.channels ch
  JOIN platform.channel_policies cp ON cp.channel_id_ref = ch.id
  WHERE cp.login_mode = 'account_system'
);

DROP INDEX IF EXISTS idx_game_account_auth_configs_game_id_ref;
ALTER TABLE IF EXISTS game_account_auth_configs DROP CONSTRAINT IF EXISTS game_account_auth_configs_status_check;
ALTER TABLE IF EXISTS game_account_auth_configs DROP CONSTRAINT IF EXISTS game_account_auth_configs_auth_type_fk;
DROP TABLE IF EXISTS game_account_auth_configs;

DROP INDEX IF EXISTS platform.idx_channel_account_auth_types_auth;
DROP INDEX IF EXISTS platform.idx_channel_account_auth_types_channel;

ALTER TABLE IF EXISTS platform.account_auth_templates      SET SCHEMA public;
ALTER TABLE IF EXISTS platform.channel_account_auth_types  SET SCHEMA public;
ALTER TABLE IF EXISTS platform.account_auth_types          SET SCHEMA public;

-- 反向跨模块 bootstrap 归位：channels / channel_policies 回 public（与 up 的 0) 对称）。
-- 在 account_auth 平台表回 public 之后再移，避免 channel_account_auth_types 的跨 schema FK 中途悬空。
ALTER TABLE IF EXISTS platform.channel_policies SET SCHEMA public;
ALTER TABLE IF EXISTS platform.channels         SET SCHEMA public;
