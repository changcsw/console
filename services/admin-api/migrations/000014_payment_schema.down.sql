-- 000014 down · payment 回滚（best effort）

DROP INDEX IF EXISTS uq_payment_routes_selector;
DROP INDEX IF EXISTS uq_payment_routes_priority;

ALTER TABLE IF EXISTS payment_routes DROP CONSTRAINT IF EXISTS payment_routes_merchant_fk;
ALTER TABLE IF EXISTS payment_routes DROP CONSTRAINT IF EXISTS payment_routes_provider_fk;
ALTER TABLE IF EXISTS payment_routes DROP CONSTRAINT IF EXISTS payment_routes_pay_way_fk;
ALTER TABLE IF EXISTS payment_routes DROP CONSTRAINT IF EXISTS payment_routes_channel_fk;

ALTER TABLE IF EXISTS platform.cashier_merchant_accounts SET SCHEMA public;
ALTER TABLE IF EXISTS platform.billing_subjects SET SCHEMA public;
ALTER TABLE IF EXISTS platform.cashier_provider_templates SET SCHEMA public;
ALTER TABLE IF EXISTS platform.cashier_providers SET SCHEMA public;
ALTER TABLE IF EXISTS platform.pay_ways SET SCHEMA public;
