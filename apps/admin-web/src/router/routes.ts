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
