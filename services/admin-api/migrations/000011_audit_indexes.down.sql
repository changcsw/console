-- 000009 down · 删除 audit 索引（逆序，幂等）。索引随表位于 platform schema。

DROP INDEX IF EXISTS platform.idx_audit_logs_resource;
DROP INDEX IF EXISTS platform.idx_audit_logs_action_created_at;
DROP INDEX IF EXISTS platform.idx_audit_logs_env_created_at;
DROP INDEX IF EXISTS platform.idx_audit_logs_resource_type_created_at;
DROP INDEX IF EXISTS platform.idx_audit_logs_actor_id;
DROP INDEX IF EXISTS platform.idx_audit_logs_created_at;
