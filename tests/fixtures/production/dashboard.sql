-- fixtures · 模块 dashboard（production schema 占位）
-- 用于与 sandbox/dashboard/env-isolation 对照，验证 summary 只统计当前 APP_ENV schema。
-- 具体业务样本由其它模块 production/*.sql 提供（如 sync.sql）。

SET search_path TO production, platform;

-- 当前无 dashboard 专属写入。
