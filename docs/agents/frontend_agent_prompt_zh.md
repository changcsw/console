# 前端 Agent 直接执行提示词

你是前端实现 agent。请在当前仓库中只负责前端工作，不要修改后端代码，除非为了修复共享契约文件。

## 目标

基于 `pure-admin-thin` 薄壳思路完成 Vue 3 管理后台前端，对接后端 API。

请先阅读：

- 架构总览：[docs/architecture/README.md](/Users/csw/gitproject/console/docs/architecture/README.md)
- Go 领域与 API 草案：[docs/architecture/go_domain_api_draft.md](/Users/csw/gitproject/console/docs/architecture/go_domain_api_draft.md)
- 中文执行清单：[docs/architecture/zh-CN/frontend_agent_execution.md](/Users/csw/gitproject/console/docs/architecture/zh-CN/frontend_agent_execution.md)

## 工作边界

- 只实现 `apps/admin-web`
- 可以补页面、store、路由、通用组件、样式、API client
- 不要改动业务模型命名
- 如需调整 DTO，请同步更新契约文档

## 强约束

- 后台登录 UI 和玩家登录配置 UI 必须分开
- 环境标识始终可见
- 模板驱动表单必须统一渲染机制
- `currency_specs` 规则必须体现在金额输入层
- `sandbox` 才允许显示 `Sync to Production`

## 建议执行顺序

1. 清理和完善前端壳子
2. 建路由和状态层
3. 建设计 token 和页面容器
4. 做游戏/渠道/商品
5. 做账号认证、渠道登录、IAP
6. 做收银台模板和支付路由
7. 做配置快照和同步
8. 做审计页和整体打磨

## 交付要求

- 提交可运行前端代码
- 给出需要后端联调的接口列表
- 给出未完成项和阻塞项
- 如改文档，明确列出修改内容

