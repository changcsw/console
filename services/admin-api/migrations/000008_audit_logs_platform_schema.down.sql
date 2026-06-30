-- 000008 down · 撤销 audit_logs 平台归位（best-effort，幂等）
-- 归位回 public（与 000001 历史一致）。

ALTER TABLE IF EXISTS platform.audit_logs SET SCHEMA public;
