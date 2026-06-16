# 管理后台架构交付包

这个目录是当前“游戏发行管理后台”的中文交付包，用于给前后端工程师或 agent 直接接手开发。

## 文件

- [../postgresql_ddl_draft.sql](/Users/csw/gitproject/console/docs/architecture/postgresql_ddl_draft.sql)：PostgreSQL DDL 草案
- [postgresql_ddl_guide.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/postgresql_ddl_guide.md)：DDL 中文说明
- [go_domain_api_draft.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/go_domain_api_draft.md)：Go 领域模型、服务边界、HTTP DTO 中文稿
- [backend_agent_execution.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/backend_agent_execution.md)：后端 agent 执行清单
- [frontend_agent_execution.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/frontend_agent_execution.md)：前端 agent 执行清单

## 当前默认前提

- 前端：`pure-admin-thin` 薄壳思路，`Vue 3 + Vite + TypeScript + Pinia + Vue Router + Element Plus`
- 后端：Go + PostgreSQL
- 环境：`develop`、`sandbox`、`production`
- `sandbox -> production`：在线差异预览 + 确认同步
- `market` 当前固定枚举：`GLOBAL / JP / KR / SEA / HMT / CN`
- 一个游戏可以同时选择多个 `market`，默认 `GLOBAL`
- 渠道管理按 `GameMarketChannel` 实例建模：
  - 默认展示当前游戏下所有 market 的所有渠道实例
  - `CN` 仅显示国内渠道
  - 非 `CN` market 仅显示非国内渠道
- 登录分两层：
  - 后台管理员登录
  - 游戏玩家登录/认证配置
- 支付分两层：
  - 渠道支付 / IAP
  - 收银台支付路由

## 推荐阅读顺序

1. 先看 [postgresql_ddl_guide.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/postgresql_ddl_guide.md) 和实际 SQL
2. 再看 [go_domain_api_draft.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/go_domain_api_draft.md)
3. 后端执行看 [backend_agent_execution.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/backend_agent_execution.md)
4. 前端执行看 [frontend_agent_execution.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/frontend_agent_execution.md)

## 当前仓库里已经有的代码骨架

- 后端骨架：[services/admin-api](/Users/csw/gitproject/console/services/admin-api)
- 前端骨架：[apps/admin-web](/Users/csw/gitproject/console/apps/admin-web)
- Agent 提示词：[docs/agents](/Users/csw/gitproject/console/docs/agents)

## 本轮已固化的关键规则

- 海外 market 生成运行时配置时：
  - 先取游戏级默认
  - 再取 `GLOBAL`
  - 最后取具体 market
  - 具体 market 覆盖 `GLOBAL`
- `CN` 不加载 `GLOBAL` 渠道实例
- 渠道被手动隐藏后：
  - 不进入配置快照
  - 不参与同步
  - 不进入客户端最终配置
- 模板版本生命周期固定为：`draft / published / archived`
- `published` 允许直接复制出新的 `draft`，但不允许原地编辑
- `sync/execute` 必须显式传 `selected_sections`
