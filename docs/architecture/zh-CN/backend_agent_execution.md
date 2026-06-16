# 后端 Agent 执行清单

这份清单是给后端 agent 直接执行的，按顺序推进即可。

## 目标

基于 Go + PostgreSQL 建立发行管理后台后端，提供清晰的管理 API，并支持 `sandbox -> production` 在线差异同步。

## 阶段 0：仓库和服务初始化

- 建立 Go module 和目录结构
- 选择 router、配置加载、日志、迁移工具、数据库访问层
- 建立 `.env.example`
- 建立健康检查接口

完成标准：
- 服务可以启动
- 配置可加载
- 迁移命令可运行

## 阶段 1：数据库与迁移

- 把 [postgresql_ddl_draft.sql](/Users/csw/gitproject/console/docs/architecture/postgresql_ddl_draft.sql) 拆成有序 migration
- 增加基础 seed 数据：
  - `channels`
  - `channel_policies`
  - `account_auth_types`
  - `pay_ways`
  - `cashier_providers`
  - `currency_specs`

完成标准：
- 空库可从零迁移成功
- seed 重复执行不会报错

## 阶段 2：领域模型和基础设施

- 实现共享枚举和值对象
- 实现仓储层
- 实现币种归一化组件
- 实现密文字段加解密抽象

完成标准：
- 核心领域均有仓储
- 金额归一化有单元测试
- 明文密钥不落库

## 阶段 3：后台登录与权限

- 实现后台密码登录
- 实现飞书后台登录回调
- 实现 `GET /api/admin/me`
- 实现角色和权限中间件

完成标准：
- 后台登录链路可用
- 权限中间件能保护样例接口

## 阶段 4：游戏主数据

- 实现：
  - `GET /api/admin/games`
  - `POST /api/admin/games`
  - `GET /api/admin/games/{gameId}`
  - `PATCH /api/admin/games/{gameId}`
  - `PUT /api/admin/games/{gameId}/markets`
  - `PUT /api/admin/games/{gameId}/legal-links`
- 自动生成：
  - `game_id`
  - `game_secret`
- 默认创建 `GLOBAL` 市场

完成标准：
- 游戏主数据可增删改查
- 法务链接支持默认、按市场、按语言覆盖

## 阶段 5：渠道与渠道包

- 实现：
  - `GET /api/admin/games/{gameId}/channels`
  - `POST /api/admin/games/{gameId}/channels`
  - `GET /api/admin/game-channels/{gameChannelId}`
  - `PATCH /api/admin/game-channels/{gameChannelId}`
  - `POST /api/admin/game-channels/{gameChannelId}/packages`
  - `GET /api/admin/game-channels/{gameChannelId}/packages`
  - `PATCH /api/admin/channel-packages/{packageId}`
- 新增渠道时应用渠道策略默认值

完成标准：
- 游戏下可管理渠道和渠道包
- `inherit_channel_config` 生效

## 阶段 6：自有账号认证与渠道登录

- 实现：
  - `GET /api/admin/account-auth/types`
  - `GET /api/admin/channels/{channelId}/account-auth-types`
  - `PUT /api/admin/games/{gameId}/account-auth-configs`
  - `GET /api/admin/game-channels/{gameChannelId}/login-config`
  - `PUT /api/admin/game-channels/{gameChannelId}/login-config`
- 基于模板元数据做配置校验
- 持久化 `config_status` 和 `last_check_message`

完成标准：
- 自有账号认证配置和渠道强制登录配置都能独立运转
- 未配全、配错时能正确标状态

## 阶段 7：商品、IAP、渠道包覆盖

- 实现：
  - 商品 CRUD
  - 包级商品映射读写
  - 渠道 IAP 配置读写
  - 包级 IAP 覆盖读写
- 实现：
  - `product_id_mode`
  - `price_id_mode`
  的生效解析逻辑

完成标准：
- 同一包可同时具备 IAP 和收银台映射信息
- 覆盖逻辑正确

## 阶段 8：收银台模板和汇率提醒

- 实现价格模板、版本、价格行、发布
- 实现汇率同步运行记录
- 默认模式必须是人工确认，不允许默认自动应用

完成标准：
- 能生成候选版本
- 能展示差异
- 审核后才应用

## 阶段 9：游戏级收银台与支付路由

- 实现：
  - 游戏绑定收银台模板
  - 游戏级价格覆盖
  - 支付方式、支付提供商、主体、商户、支付路由
- 实现支付路由优先级匹配：
  - 包精确命中
  - 渠道+市场+国家+币种
  - 游戏级通配
  - `priority` 越小优先级越高

完成标准：
- 可通过切换路由优先级完成 PSP 无感切换

## 阶段 10：客户端配置快照

- 实现：
  - 生成配置快照
  - 列表查询
  - 发布
  - 下载 JSON

完成标准：
- 快照可预览、可下载、可发布

## 阶段 11：Sandbox 同步 Production

- 实现：
  - `sync/preview`
  - `sync/execute`
  - `sync-jobs`
- 差异按 section 组织
- 敏感字段预览时必须脱敏
- 执行同步前必须再次校验目标 hash

完成标准：
- 预览可靠
- 同步可回溯
- 删除项默认不执行，必须显式打开

## 阶段 12：测试和加固

- 补单元测试：
  - 币种归一化
  - 商品/价格档解析
  - 支付路由解析
  - 同步差异生成
- 补集成测试
- 输出 OpenAPI 或稳定 API 契约

## 不允许踩的坑

- 不要把后台登录和玩家登录配置混在一起
- 不要把渠道 IAP 配置和收银台支付路由混在一起
- 不要绕过 `currency_specs`
- 不要绕过同步预览直接写 production
- 不要存明文密钥

