# 前后端并行开发说明

这个文件给你自己或协调 agent 使用，用来让前后端并行推进。

## 后端先行交付的最小接口组

第一批建议优先给前端：

- `GET /api/admin/me`
- `GET /api/admin/games`
- `POST /api/admin/games`
- `GET /api/admin/games/{gameId}`
- `GET /api/admin/games/{gameId}/channels`
- `GET /api/admin/games/{gameId}/products`
- `GET /api/admin/cashier/templates`

这样前端可以先把列表页、详情页骨架和空态全接起来。

## 第二批接口

- 自有账号认证配置
- 渠道登录配置
- IAP 配置
- 渠道包商品映射

这样前端可以把核心业务页闭环。

## 第三批接口

- 收银台价格模板版本与价格矩阵
- 支付路由
- 配置快照
- `sync preview / execute`

这样前端可以把上线、支付和同步能力补齐。

## 并行协作建议

- 前后端都以 [go_domain_api_draft.md](/Users/csw/gitproject/console/docs/architecture/go_domain_api_draft.md) 为准
- DTO 变更必须先更新文档再改代码
- 模板驱动表单结构一旦确定，不要前后端各自发明一套
- 金额字段统一使用 `*_amount_minor`
- 币种精度统一由 `currency_specs` 驱动

