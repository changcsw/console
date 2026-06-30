-- 000009 · audit：建议索引（compact §建议索引；原 000007 重排为唯一序号，详见 integration.checklist）
-- 依赖 000008：audit_logs 已位于 platform schema。migrate 连接默认 search_path 不含 platform，
-- 故此处对表显式限定 platform.audit_logs（索引随表落在 platform schema）。
-- 幂等：IF NOT EXISTS 可重复执行。

CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at
  ON platform.audit_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id
  ON platform.audit_logs (actor_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type_created_at
  ON platform.audit_logs (resource_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_env_created_at
  ON platform.audit_logs (env, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created_at
  ON platform.audit_logs (action, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_logs_resource
  ON platform.audit_logs (resource_type, resource_id, created_at DESC);
