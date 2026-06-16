DELETE FROM currency_specs WHERE currency_code IN ('USD', 'JPY', 'KRW', 'TWD', 'EUR');
DELETE FROM cashier_providers WHERE provider_id IN ('airwallex', 'payermax', 'paypal_direct');
DELETE FROM pay_ways WHERE pay_way_id IN ('credit_card', 'paypal', 'apple_pay', 'google_pay', 'gcash');
DELETE FROM account_auth_types WHERE auth_type_id IN ('guest', 'phone', 'email', 'google', 'apple', 'facebook', 'line', 'kakao');
DELETE FROM channel_policies WHERE channel_id_ref IN (
  SELECT id FROM channels WHERE channel_id IN ('google', 'apple', 'huawei_cn', 'xiaomi_cn', 'oppo_cn', 'vivo_cn', 'wechat_mini_game', 'douyin_mini_game')
);
DELETE FROM channels WHERE channel_id IN ('google', 'apple', 'huawei_cn', 'xiaomi_cn', 'oppo_cn', 'vivo_cn', 'wechat_mini_game', 'douyin_mini_game');
