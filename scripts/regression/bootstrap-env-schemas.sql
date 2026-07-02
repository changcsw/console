-- bootstrap-env-schemas.sql
-- 测试回归用：将 public 中的业务表结构克隆到 develop/sandbox/production 三环境 schema。
-- 架构目标见 docs/architecture/v2/01-structure.md §6；正式 bootstrap 迁移待后续补齐。
-- 幂等：CREATE SCHEMA IF NOT EXISTS + CREATE TABLE IF NOT EXISTS。

CREATE SCHEMA IF NOT EXISTS develop;
CREATE SCHEMA IF NOT EXISTS sandbox;
CREATE SCHEMA IF NOT EXISTS production;

DO $$
DECLARE
  env text;
  tbl text;
BEGIN
  FOR env IN SELECT unnest(ARRAY['develop', 'sandbox', 'production']) LOOP
    FOR tbl IN
      SELECT tablename
      FROM pg_tables
      WHERE schemaname = 'public'
        AND tablename <> 'schema_migrations'
    LOOP
      EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I.%I (LIKE public.%I INCLUDING ALL)',
        env, tbl, tbl
      );
    END LOOP;
  END LOOP;
END $$;
