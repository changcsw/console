-- fixtures · 模块 dashboard（sandbox schema 业务样本占位）
-- 本模块为只读聚合，不拥有业务表；实际数据来自：
--   - sandbox/{account-auth,channel-login,feature-plugin,product,snapshot,sync,game-cashier}.sql
--   - common/{cashier-template,channel-login,feature-plugin,product}.sql
--
-- 引用约定：
--   sandbox/dashboard/base                  -> 复用上游模块基线样本，无额外 INSERT
--   sandbox/dashboard/env-isolation         -> 与 production/dashboard/base 组合验证 schema 隔离
--   sandbox/dashboard/masked-and-permission -> 复用含 last_check_message 的 invalid 样本验证脱敏透传

SET search_path TO sandbox, platform;

-- 当前无专属表，不新增写入。
