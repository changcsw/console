-- 000017 · 正式 env schema bootstrap（替代回归脚本 scripts/regression/bootstrap-env-schemas.sql 的临时克隆）
-- 目标（01-structure.md §6）：bootstrap 迁移创建环境 schema，并把「业务表」结构落到 develop/sandbox/production 三个环境 schema 各一份。
-- 说明：
--   - platform / 平台表由 000003–000016 建立并归位；本迁移只负责三个环境 schema 及其业务表结构。
--   - 业务表 DDL 在 000001+ 以默认 search_path(public) 创建，public 在当前架构中充当「业务表结构模板」；
--     这里按模板把结构克隆到各环境 schema（CREATE TABLE ... LIKE ... INCLUDING ALL），复制列/默认值/CHECK/唯一约束/索引。
--   - 外键不随 LIKE 复制（与既有回归克隆行为一致）；跨 schema 外键补齐留待后续 per-env 迁移运行器方案（§6 理想形态）。
-- 幂等：CREATE SCHEMA IF NOT EXISTS + CREATE TABLE IF NOT EXISTS，可重复执行。

CREATE SCHEMA IF NOT EXISTS platform;
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
