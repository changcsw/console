# 后端 Agent 直接执行提示词

你是后端实现 agent。请在当前仓库中只负责后端工作，不要修改前端代码，除非为了修复共享契约文件。

## 目标

基于以下文档实现 Go + PostgreSQL 后端：

- 架构总览：[docs/architecture/README.md](/Users/csw/gitproject/console/docs/architecture/README.md)
- SQL 草案：[docs/architecture/postgresql_ddl_draft.sql](/Users/csw/gitproject/console/docs/architecture/postgresql_ddl_draft.sql)
- Go 领域与 API 草案：[docs/architecture/go_domain_api_draft.md](/Users/csw/gitproject/console/docs/architecture/go_domain_api_draft.md)
- 中文执行清单：[docs/architecture/zh-CN/backend_agent_execution.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/backend_agent_execution.md)

## 工作边界

- 只实现 `services/admin-api`
- 可以补充或修正 migration、Go 代码、API 契约
- 不要擅自改业务模型命名
- 如果需要调整 DTO，先同步更新对应文档

## 强约束

- 后台管理员登录和玩家登录配置必须分开
- 渠道 IAP 配置和收银台支付路由必须分开
- 所有金额写入必须经过 `currency_specs`
- `sandbox -> production` 同步必须先预览差异再执行
- 敏感字段不得明文存储

## 建议执行顺序

1. 完善 migration
2. 完善领域模型与 repository
3. 实现后台登录与权限
4. 实现游戏、渠道、商品
5. 实现账号认证、渠道登录、IAP
6. 实现收银台模板、支付路由、配置快照
7. 实现同步预览与执行
8. 补测试与 OpenAPI

## 交付要求

- 提交可运行代码
- 提交必要 migration
- 给出未完成项和阻塞项
- 如改动文档，明确列出修改内容

