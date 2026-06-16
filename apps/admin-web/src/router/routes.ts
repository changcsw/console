import type { RouteRecordRaw } from "vue-router";
import AdminLayout from "@/layouts/AdminLayout.vue";

export const routes: RouteRecordRaw[] = [
  {
    path: "/login",
    name: "login",
    component: () => import("@/views/login/LoginView.vue"),
    meta: { title: "后台登录", hidden: true }
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
        meta: { title: "游戏管理", icon: "Grid" }
      },
      {
        path: "channels",
        name: "channels",
        component: () => import("@/views/channels/ChannelsView.vue"),
        meta: { title: "渠道管理", icon: "Connection" }
      },
      {
        path: "cashier",
        name: "cashier",
        component: () => import("@/views/cashier/CashierView.vue"),
        meta: { title: "收银台管理", icon: "CreditCard" }
      },
      {
        path: "audit",
        name: "audit",
        component: () => import("@/views/audit/AuditView.vue"),
        meta: { title: "审计日志", icon: "Document" }
      },
      {
        path: "system",
        name: "system",
        component: () => import("@/views/system/SystemView.vue"),
        meta: { title: "系统设置", icon: "Setting" }
      }
    ]
  }
];
