import type { RouteRecordRaw } from "vue-router";
import AdminLayout from "@/layouts/AdminLayout.vue";

export const routes: RouteRecordRaw[] = [
  {
    path: "/login",
    name: "login",
    component: () => import("@/views/login/LoginView.vue"),
    meta: { title: "后台登录", hidden: true, public: true }
  },
  {
    path: "/403",
    name: "forbidden",
    component: () => import("@/views/error/ForbiddenView.vue"),
    meta: { title: "无权限", hidden: true, public: true }
  },
  {
    path: "/",
    component: AdminLayout,
    redirect: "/dashboard",
    children: [
      {
        path: "dashboard",
        name: "dashboard",
        component: () => import("@/views/dashboard/DashboardView.vue"),
        meta: { title: "工作台", icon: "House" }
      },
      {
        path: "games",
        name: "games",
        component: () => import("@/views/games/GamesView.vue"),
        meta: { title: "游戏管理", icon: "Grid", perm: "game.read" }
      },
      {
        path: "games/:gameId",
        name: "game-detail",
        component: () => import("@/views/games/detail/GameDetailView.vue"),
        meta: { title: "游戏详情", hidden: true, perm: "game.read" }
      },
      {
        path: "channels",
        name: "channels",
        component: () => import("@/views/channels/ChannelsView.vue"),
        meta: { title: "渠道管理", icon: "Connection", perm: "channel.read" }
      },
      {
        path: "cashier",
        name: "cashier",
        component: () => import("@/views/cashier/CashierView.vue"),
        meta: { title: "收银台", icon: "CreditCard", perm: "cashier.read" }
      },
      {
        path: "payment",
        component: () => import("@/views/payment/PaymentLayout.vue"),
        redirect: "/payment/pay-ways",
        meta: { title: "支付配置", icon: "Wallet", perm: "payment.read" },
        children: [
          {
            path: "pay-ways",
            name: "payment-pay-ways",
            component: () => import("@/views/payment/PayWaysView.vue"),
            meta: { title: "支付方式", hidden: true, perm: "payment.read" }
          },
          {
            path: "providers",
            name: "payment-providers",
            component: () => import("@/views/payment/ProvidersView.vue"),
            meta: { title: "支付提供商", hidden: true, perm: "payment.read" }
          },
          {
            path: "billing-subjects",
            name: "payment-billing-subjects",
            component: () => import("@/views/payment/BillingSubjectsView.vue"),
            meta: { title: "结算主体", hidden: true, perm: "payment.read" }
          },
          {
            path: "merchant-accounts",
            name: "payment-merchant-accounts",
            component: () => import("@/views/payment/MerchantAccountsView.vue"),
            meta: { title: "商户账户", hidden: true, perm: "payment.read" }
          }
        ]
      },
      {
        path: "audit",
        name: "audit",
        component: () => import("@/views/audit/AuditView.vue"),
        meta: { title: "审计日志", icon: "Document", perm: "audit.read" }
      },
      {
        path: "system",
        name: "system",
        component: () => import("@/views/system/SystemView.vue"),
        meta: { title: "系统设置", icon: "Setting", perm: "system.read" }
      }
    ]
  }
];
