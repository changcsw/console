-- 000008 · audit：audit_logs 平台级表归位到 platform schema
-- 背景：000001 在默认 schema(public) 建了 audit_logs；运行期连接池 search_path=<env>, platform
--       （不含 public，见 internal/infra/persistence/postgres/pool.go），故 audit_logs 必须位于
--       platform 才能被 audit_repository 的未限定 SQL 命中（与 admin_* 平台级表同法，参见 000003）。
-- compact §数据模型：audit_logs 属共享 platform schema（平台级、每行带 env 过滤列）。
-- 幂等：使用 IF EXISTS / IF NOT EXISTS，可重复执行；移动后 public.audit_logs 不再存在则跳过。

CREATE SCHEMA IF NOT EXISTS platform;

ALTER TABLE IF EXISTS public.audit_logs SET SCHEMA platform;
