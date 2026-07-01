module: payment(#19) / all-stages / ✅ 通过（验收 36/36 PASS）
lane: payment-surface
worktree: /Users/csw/gitproject/console-payment (branch codex/payment @ origin/main 7a090e9)
changed_paths: services/admin-api/{migrations/000014,internal/domain/payment,internal/app/payment,internal/infra/persistence/postgres/payment_*,internal/transport/http/payment,internal/transport/httpserver/admin_wiring.go}; apps/admin-web/src/{api/modules/payment.ts,views/payment,views/games/detail/PaymentRoutesTab.vue+GameDetailView.vue,router/routes.ts}; tests/backend/scenarios/payment.yaml; tests/fixtures/common/payment.sql; tests/frontend/e2e/payment.spec.ts
api_or_routes: GET /pay-ways,/cashier/providers,/billing-subjects,/cashier/merchant-accounts,/cashier/providers/{id}/template,/games/{id}/payment-routes; POST /billing-subjects,/cashier/merchant-accounts; PUT /games/{id}/payment-routes; 前端 /pay-ways /providers /billing-subjects /merchant-accounts + 游戏详情「支付路由」Tab
depends_on: channel,product,cashier-template,game-cashier,game,common（全部✅已合并main）
integration: docs/architecture/v2/modules/19-payment/artifacts/integration.checklist.md
manifest: docs/architecture/v2/modules/19-payment/artifacts/module.manifest.json
issues: 无阻断；P3 遗留：连库维度(S6/S10/唯一索引)待 PG CI、000014 未 live 执行、全量 regression playwright 端口占用（环境）
downstream: PaymentRouteService.ResolveRoute(ctx,gameID,MatchInput)->RouteTarget/NOT_FOUND，供 #20 snapshot per-game per-market 调用
next: 提交 codex/payment；请新开 Chat 跑集成合并 payment → main，然后可开工 #20 snapshot
