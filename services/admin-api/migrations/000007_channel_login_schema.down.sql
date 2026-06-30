-- 000007 down · channel-login 回滚（best effort，幂等）。
-- 不删除 channel_login_templates / game_channel_login_configs 本体（由 000001 down 负责），
-- 仅回滚本迁移新增的 seed / 索引 / 命名约束，并把模板表归位回 public（与 up 的 1) 对称）。

-- 反向 seed：仅清掉 000007 写入的 huawei_cn v1 模板行。
DELETE FROM platform.channel_login_templates
WHERE template_version = 'v1'
  AND channel_id_ref IN (SELECT id FROM platform.channels WHERE channel_id = 'huawei_cn');

-- 业务表：丢弃本迁移新增的索引与命名约束（当前 env schema）。
DROP INDEX IF EXISTS idx_game_channel_login_configs_game_channel;
ALTER TABLE IF EXISTS game_channel_login_configs
  DROP CONSTRAINT IF EXISTS game_channel_login_configs_status_check;

-- 平台模板表：先丢索引（仍在 platform），再归位回 public（与 up 的 1) 对称）。
DROP INDEX IF EXISTS platform.idx_channel_login_templates_channel_enabled;
ALTER TABLE IF EXISTS platform.channel_login_templates SET SCHEMA public;
