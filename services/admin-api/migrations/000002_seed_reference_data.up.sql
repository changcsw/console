INSERT INTO channels (channel_id, channel_name, channel_type, enabled, sort)
VALUES
  ('google', 'Google', 'store', TRUE, 10),
  ('apple', 'Apple', 'store', TRUE, 20),
  ('huawei_cn', '华为联运', 'oem', TRUE, 30),
  ('xiaomi_cn', '小米联运', 'oem', TRUE, 40),
  ('oppo_cn', 'OPPO 联运', 'oem', TRUE, 50),
  ('vivo_cn', 'VIVO 联运', 'oem', TRUE, 60),
  ('wechat_mini_game', '微信小游戏', 'mini_game', TRUE, 70),
  ('douyin_mini_game', '抖音小游戏', 'mini_game', TRUE, 80)
ON CONFLICT (channel_id) DO NOTHING;

INSERT INTO channel_policies (channel_id_ref, login_mode, payment_mode, login_locked, payment_locked)
SELECT id,
  CASE channel_id
    WHEN 'huawei_cn' THEN 'channel_only'
    WHEN 'xiaomi_cn' THEN 'channel_only'
    WHEN 'oppo_cn' THEN 'channel_only'
    WHEN 'vivo_cn' THEN 'channel_only'
    ELSE 'account_system'
  END,
  CASE channel_id
    WHEN 'huawei_cn' THEN 'channel_only'
    WHEN 'xiaomi_cn' THEN 'channel_only'
    WHEN 'oppo_cn' THEN 'channel_only'
    WHEN 'vivo_cn' THEN 'channel_only'
    ELSE 'hybrid'
  END,
  CASE channel_id
    WHEN 'huawei_cn' THEN TRUE
    WHEN 'xiaomi_cn' THEN TRUE
    WHEN 'oppo_cn' THEN TRUE
    WHEN 'vivo_cn' THEN TRUE
    ELSE FALSE
  END,
  CASE channel_id
    WHEN 'huawei_cn' THEN TRUE
    WHEN 'xiaomi_cn' THEN TRUE
    WHEN 'oppo_cn' THEN TRUE
    WHEN 'vivo_cn' THEN TRUE
    ELSE FALSE
  END
FROM channels
ON CONFLICT (channel_id_ref) DO NOTHING;

INSERT INTO account_auth_types (auth_type_id, auth_type_name, enabled, sort)
VALUES
  ('guest', '游客', TRUE, 10),
  ('phone', '手机号', TRUE, 20),
  ('email', '邮箱', TRUE, 30),
  ('google', 'Google 登录', TRUE, 40),
  ('apple', 'Apple 登录', TRUE, 50),
  ('facebook', 'Facebook 登录', TRUE, 60),
  ('line', 'LINE 登录', TRUE, 70),
  ('kakao', 'Kakao 登录', TRUE, 80)
ON CONFLICT (auth_type_id) DO NOTHING;

INSERT INTO pay_ways (pay_way_id, pay_way_name, pay_way_type, enabled, sort)
VALUES
  ('credit_card', '信用卡', 'card', TRUE, 10),
  ('paypal', 'PayPal', 'wallet', TRUE, 20),
  ('apple_pay', 'Apple Pay', 'platform', TRUE, 30),
  ('google_pay', 'Google Pay', 'platform', TRUE, 40),
  ('gcash', 'GCash', 'local', TRUE, 50)
ON CONFLICT (pay_way_id) DO NOTHING;

INSERT INTO cashier_providers (provider_id, provider_name, provider_kind, enabled, sort)
VALUES
  ('airwallex', 'Airwallex', 'aggregator', TRUE, 10),
  ('payermax', 'PayerMax', 'aggregator', TRUE, 20),
  ('paypal_direct', 'PayPal Direct', 'wallet_direct', TRUE, 30)
ON CONFLICT (provider_id) DO NOTHING;

INSERT INTO currency_specs (currency_code, currency_name, decimal_places, min_amount_minor, rounding_mode, enabled)
VALUES
  ('USD', 'US Dollar', 2, 1, 'half_up', TRUE),
  ('JPY', 'Japanese Yen', 0, 1, 'half_up', TRUE),
  ('KRW', 'Korean Won', 0, 1, 'half_up', TRUE),
  ('TWD', 'New Taiwan Dollar', 0, 1, 'half_up', TRUE),
  ('EUR', 'Euro', 2, 1, 'half_up', TRUE)
ON CONFLICT (currency_code) DO NOTHING;

