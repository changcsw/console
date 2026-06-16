# Go 领域模型与 API DTO 草案

这份文档对应后端的 Go 服务实现，目标是让后端 agent 或工程师直接按领域拆分开始落代码。

## 推荐目录结构

```text
internal/
  app/
    command/
    query/
    dto/
  domain/
    admin/
    auth/
    game/
    channel/
    product/
    cashier/
    payment/
    sync/
    common/
  infra/
    persistence/postgres/
    crypto/
    file/
    fx/
  transport/
    http/
      admin/
      auth/
      games/
      channels/
      products/
      cashier/
      payment/
      sync/
```

## 共享枚举和值对象

至少统一这些类型：

- `Environment`
- `LoginMode`
- `PaymentMode`
- `ConfigStatus`
- `OverrideMode`
- `FXSyncMode`

这样可以避免前后端状态值不一致。

## 核心聚合建议

### Game 聚合

负责：
- 游戏基础信息
- 市场
- 法务链接
- 自有账号认证配置
- 游戏渠道
- 商品
- 收银台绑定

### GameChannel 聚合

负责：
- 选中的渠道
- 渠道策略
- 渠道登录配置
- 渠道 IAP 配置
- 包列表

### Product 聚合

负责：
- 逻辑商品
- 包级 IAP 商品 ID 覆盖
- 包级收银台 `price_id` 覆盖

### CashierTemplate 聚合

负责：
- 模板
- 模板版本
- 价格行
- 汇率同步记录

### PaymentRouting 聚合

负责：
- 支付方式
- 支付提供商
- 商户账户
- 路由优先级

### Sync 聚合

负责：
- 差异预览
- 执行同步
- 审计记录

## 应用服务边界建议

不要做一个超大的 `GameService`，建议按领域拆：

- `GameService`
- `GameChannelService`
- `AccountAuthService`
- `ChannelLoginService`
- `ProductService`
- `IAPConfigService`
- `CashierTemplateService`
- `GameCashierService`
- `PaymentRouteService`
- `ConfigSnapshotService`
- `SyncService`
- `AdminAuthService`

## HTTP API 草案

统一前缀：

- `/api/admin`

### 后台登录

- `POST /api/admin/auth/login`
- `POST /api/admin/auth/feishu/callback`
- `GET /api/admin/me`

### 游戏

- `GET /api/admin/games`
- `POST /api/admin/games`
- `GET /api/admin/games/{gameId}`
- `PATCH /api/admin/games/{gameId}`
- `PUT /api/admin/games/{gameId}/markets`
- `PUT /api/admin/games/{gameId}/legal-links`

### 渠道与渠道包

- `GET /api/admin/games/{gameId}/channels`
- `POST /api/admin/games/{gameId}/channels`
- `GET /api/admin/game-channels/{gameChannelId}`
- `PATCH /api/admin/game-channels/{gameChannelId}`
- `POST /api/admin/game-channels/{gameChannelId}/packages`
- `GET /api/admin/game-channels/{gameChannelId}/packages`
- `PATCH /api/admin/channel-packages/{packageId}`

### 自有账号认证

- `GET /api/admin/account-auth/types`
- `GET /api/admin/channels/{channelId}/account-auth-types`
- `PUT /api/admin/games/{gameId}/account-auth-configs`

### 渠道登录

- `GET /api/admin/game-channels/{gameChannelId}/login-config`
- `PUT /api/admin/game-channels/{gameChannelId}/login-config`

### 商品与 IAP 映射

- `GET /api/admin/games/{gameId}/products`
- `POST /api/admin/games/{gameId}/products`
- `PATCH /api/admin/products/{productId}`
- `GET /api/admin/channel-packages/{packageId}/products`
- `PUT /api/admin/channel-packages/{packageId}/products`

### IAP 配置

- `GET /api/admin/game-channels/{gameChannelId}/iap-config`
- `PUT /api/admin/game-channels/{gameChannelId}/iap-config`
- `GET /api/admin/channel-packages/{packageId}/iap-override`
- `PUT /api/admin/channel-packages/{packageId}/iap-override`

### 收银台模板

- `GET /api/admin/cashier/templates`
- `POST /api/admin/cashier/templates`
- `GET /api/admin/cashier/templates/{templateId}`
- `POST /api/admin/cashier/templates/{templateId}/versions`
- `GET /api/admin/cashier/templates/{templateId}/versions/{version}/rows`
- `PUT /api/admin/cashier/templates/{templateId}/versions/{version}/rows`
- `POST /api/admin/cashier/templates/{templateId}/versions/{version}/publish`
- `POST /api/admin/cashier/templates/{templateId}/fx-sync/runs`
- `POST /api/admin/cashier/fx-sync-runs/{runId}/approve`

### 游戏级收银台

- `GET /api/admin/games/{gameId}/cashier/profile`
- `PUT /api/admin/games/{gameId}/cashier/profile`
- `GET /api/admin/games/{gameId}/cashier/price-overrides`
- `PUT /api/admin/games/{gameId}/cashier/price-overrides`

### 支付方式、支付提供商、商户、路由

- `GET /api/admin/pay-ways`
- `GET /api/admin/cashier/providers`
- `GET /api/admin/billing-subjects`
- `POST /api/admin/billing-subjects`
- `GET /api/admin/cashier/merchant-accounts`
- `POST /api/admin/cashier/merchant-accounts`
- `GET /api/admin/games/{gameId}/payment-routes`
- `PUT /api/admin/games/{gameId}/payment-routes`

### 配置快照

- `POST /api/admin/games/{gameId}/config-snapshots/generate`
- `GET /api/admin/games/{gameId}/config-snapshots`
- `POST /api/admin/game-config-snapshots/{snapshotId}/publish`
- `GET /api/admin/game-config-snapshots/{snapshotId}/download`

### `sandbox -> production` 同步

- `POST /api/admin/games/{gameId}/sync/preview`
- `POST /api/admin/games/{gameId}/sync/execute`
- `GET /api/admin/games/{gameId}/sync-jobs`

## 模板类字段统一语义

所有模板表里的四个 JSON 都沿用统一含义：

- `form_schema_json`：前端渲染哪些字段、用什么组件
- `secret_fields_json`：哪些字段是密文
- `file_fields_json`：哪些字段是文件上传
- `validation_rules_json`：服务端和前端共同遵循的校验规则

## 金额写入统一规则

任何涉及金额的写入路径，都必须：

1. 读取 `currency_specs`
2. 根据 `decimal_places` 校验精度
3. 根据 `min_amount_minor` 校验最小金额
4. 根据 `rounding_mode` 做归一化
5. 最终统一存成 `*_amount_minor`

## Repository 接口建议

建议仓储接口保持窄，不要把跨表编排逻辑塞进 repository。

例如：

- `GameRepository`
- `ChannelRepository`
- `ProductRepository`
- `CashierTemplateRepository`
- `PaymentRouteRepository`
- `SyncRepository`

跨表编排、差异计算、模板驱动校验，都应放在应用服务层。

