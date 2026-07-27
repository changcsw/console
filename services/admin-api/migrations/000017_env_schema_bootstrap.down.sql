-- 000017 down · 回滚 env schema bootstrap
-- 仅删除本迁移创建的三个环境 schema（连同其中克隆的业务表）；platform 由 000003 创建，保留。

DROP SCHEMA IF EXISTS develop CASCADE;
DROP SCHEMA IF EXISTS sandbox CASCADE;
DROP SCHEMA IF EXISTS production CASCADE;
