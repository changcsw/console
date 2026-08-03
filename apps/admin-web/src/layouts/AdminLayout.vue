<template>
  <div class="app-layout" :class="{ 'app-layout--collapsed': sidebarCollapsed }">
    <aside class="sidebar" :class="{ 'sidebar--collapsed': sidebarCollapsed }">
      <div class="brand">
        <div class="brand__mark">GC</div>
        <div v-show="!sidebarCollapsed">
          <div class="brand__title">Publishing Console</div>
          <div class="brand__sub">发行管理后台</div>
        </div>
      </div>

      <nav class="menu">
        <RouterLink
          v-for="route in visibleRoutes"
          :key="route.path"
          class="menu__item"
          :to="route.path.startsWith('/') ? route.path : `/${route.path}`"
          :title="String(route.meta?.title ?? route.name ?? route.path)"
        >
          <el-icon class="menu__icon"><component :is="resolveMenuIcon(route.meta?.icon)" /></el-icon>
          <span v-show="!sidebarCollapsed" class="menu__label">
            {{ String(route.meta?.title ?? route.name ?? route.path) }}
          </span>
        </RouterLink>
      </nav>

      <div class="sidebar__footer">
        <el-dropdown v-if="auth.user" trigger="click" class="user-dropdown" @command="onUserCommand">
          <button type="button" class="user-profile" :title="auth.user.displayName || auth.user.userName">
            <span class="user-profile__left">
              <span class="user-profile__avatar" aria-hidden="true">
                <img class="user-profile__avatar-img" :src="defaultAvatarUrl" alt="" />
              </span>
              <span v-show="!sidebarCollapsed" class="user-profile__name">
                {{ auth.user.displayName || auth.user.userName }}
              </span>
            </span>
            <el-icon v-show="!sidebarCollapsed" class="user-profile__arrow"><ArrowUp /></el-icon>
          </button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item disabled>{{ auth.user.userName }}</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </aside>

    <main class="content">
      <header class="topbar">
        <div class="topbar__left">
          <el-button
            class="topbar__toggle"
            circle
            size="small"
            :aria-label="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'"
            @click="toggleSidebar"
          >
            <el-icon><component :is="sidebarCollapsed ? Expand : Fold" /></el-icon>
          </el-button>
          <div v-if="topbar.breadcrumb.value" class="topbar__breadcrumb">
            <template v-for="(item, index) in topbar.breadcrumb.value" :key="item.key">
              <el-button
                v-if="item.onClick"
                class="topbar__crumb-btn"
                link
                @click="item.onClick()"
              >
                {{ item.label }}
              </el-button>
              <span v-else class="topbar__breadcrumb-current">{{ item.label }}</span>
              <el-icon v-if="index < topbar.breadcrumb.value.length - 1" class="topbar__crumb-sep"><ArrowRight /></el-icon>
            </template>
          </div>
          <el-tooltip
            content="围绕游戏、渠道、商品、IAP、收银台和生产同步组织页面。"
            placement="bottom-start"
          >
            <el-button class="topbar__hint" circle size="small">
              <el-icon><InfoFilled /></el-icon>
            </el-button>
          </el-tooltip>
        </div>
        <div v-if="topbar.actions.value" class="topbar__right">
          <el-button
            v-if="topbar.actions.value.showSyncButton"
            v-perm="'sync.execute'"
            type="primary"
            :disabled="!topbar.actions.value.canSyncExecute"
            @click="topbar.actions.value.onSyncToProduction"
          >
            同步到生产
          </el-button>
          <el-dropdown trigger="click" @command="onTopbarEnvironmentCommand">
            <span class="topbar__env-trigger">
              {{ topbar.actions.value.environment }}
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="sandbox">sandbox</el-dropdown-item>
                <el-dropdown-item command="production">production</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
        <EnvironmentBadge v-else :environment="app.environment" />
      </header>

      <section class="content__body">
        <RouterView />
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import {
  ArrowDown,
  ArrowUp,
  ArrowRight,
  Connection,
  CreditCard,
  Document,
  Expand,
  Fold,
  Grid,
  House,
  InfoFilled,
  MagicStick,
  Menu as MenuIcon,
  Setting,
  Wallet
} from "@element-plus/icons-vue";
import type { Component } from "vue";
import { useAppStore } from "@/stores/app";
import { useAuthStore } from "@/stores/auth";
import { usePermissionStore } from "@/stores/permission";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import { useTopbarBridge } from "./topbarBridge";

const router = useRouter();
const app = useAppStore();
const auth = useAuthStore();
const permission = usePermissionStore();
const sidebarCollapsed = ref(false);
const topbar = useTopbarBridge();

/** 无上传头像时的默认图（图二风格：圆形风景默认头像） */
const defaultAvatarUrl =
  "data:image/svg+xml;charset=UTF-8," +
  encodeURIComponent(`
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 80 80">
  <defs>
    <linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="#5ec8ff"/>
      <stop offset="55%" stop-color="#2f8fd8"/>
      <stop offset="100%" stop-color="#1c6aa8"/>
    </linearGradient>
  </defs>
  <circle cx="40" cy="40" r="40" fill="url(#sky)"/>
  <ellipse cx="40" cy="62" rx="34" ry="12" fill="#1b3a2a" opacity="0.55"/>
  <path d="M18 58 C22 42 28 34 34 30 C30 40 28 48 30 58 Z" fill="#163826"/>
  <path d="M34 58 C36 40 42 30 50 26 C44 38 42 48 44 58 Z" fill="#1d4a30"/>
  <path d="M48 58 C52 44 58 36 66 32 C60 42 58 50 60 58 Z" fill="#163826"/>
  <circle cx="58" cy="20" r="6" fill="#ffe08a" opacity="0.9"/>
</svg>
`);

onMounted(() => {
  topbar.setConnected(true);
});

onBeforeUnmount(() => {
  topbar.setConnected(false);
  topbar.setBreadcrumb(null);
  topbar.setActions(null);
});

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value;
}

const layoutRoute = computed(() => router.options.routes.find((item) => item.path === "/"));

const visibleRoutes = computed(() =>
  (layoutRoute.value?.children ?? []).filter(
    (item) => !item.meta?.hidden && permission.hasPerm(item.meta?.perm as string | undefined)
  )
);

const routeIconMap: Record<string, Component> = {
  House,
  Grid,
  Connection,
  MagicStick,
  CreditCard,
  Wallet,
  Document,
  Setting
};

function resolveMenuIcon(icon: unknown): Component {
  if (typeof icon !== "string") {
    return MenuIcon;
  }
  return routeIconMap[icon] ?? MenuIcon;
}

async function onUserCommand(command: string) {
  if (command === "logout") {
    await auth.logout();
    void router.push("/login");
  }
}

function onTopbarEnvironmentCommand(command: string) {
  if (command === "sandbox" || command === "production") {
    topbar.actions.value?.onChangeEnvironment(command);
  }
}
</script>

<style scoped>
.app-layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  height: 100vh;
  overflow: hidden;
}

.app-layout--collapsed {
  grid-template-columns: 76px 1fr;
}

.sidebar {
  display: flex;
  flex-direction: column;
  height: 100vh;
  padding: 20px 18px 16px;
  background: linear-gradient(180deg, #0f172a 0%, #13315c 100%);
  color: #eff6ff;
  overflow: hidden;
}

.sidebar--collapsed {
  padding: 16px 10px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 24px;
}

.brand__mark {
  width: 44px;
  height: 44px;
  border-radius: 14px;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #fef08a, #86efac);
  color: #0f172a;
  font-weight: 800;
}

.brand__title {
  font-size: 16px;
  font-weight: 800;
}

.brand__sub {
  color: #c7d2fe;
  font-size: 12px;
}

.menu {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.18) transparent;
}

.menu::-webkit-scrollbar {
  width: 6px;
}

.menu::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.18);
  border-radius: 3px;
}

.menu::-webkit-scrollbar-track {
  background: transparent;
}

/* 用户信息始终固定在菜单栏最下方，不随菜单滚动，也不加分割线 */
.sidebar__footer {
  flex-shrink: 0;
  margin-top: auto;
  padding-top: 8px;
}

.user-dropdown {
  display: block;
  width: 100%;
}

.user-dropdown :deep(.el-tooltip__trigger) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  outline: none;
}

.menu__item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-radius: 12px;
  color: #dbeafe;
  transition: 0.2s ease;
}

.menu__icon {
  font-size: 16px;
}

.menu__label {
  white-space: nowrap;
}

.sidebar--collapsed .menu__item {
  justify-content: center;
  padding: 12px 8px;
}

.menu__item.router-link-active {
  background: rgba(255, 255, 255, 0.12);
  color: #ffffff;
}

.content {
  height: 100vh;
  padding: 20px;
  overflow-y: auto;
}

.content__body {
  margin-top: 10px;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.topbar__left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.topbar__toggle,
.topbar__hint {
  border-color: var(--el-border-color);
  background: #ffffff;
}

.topbar__breadcrumb {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: 4px;
  font-size: 14px;
}

.topbar__crumb-btn {
  padding: 0;
  font-size: 14px;
  color: var(--text-subtle);
}

.topbar__crumb-sep {
  color: var(--text-subtle);
  font-size: 12px;
}

.topbar__crumb-btn:hover {
  color: var(--el-color-primary);
}

.topbar__breadcrumb-parent {
  color: var(--text-subtle);
}

.topbar__breadcrumb-current {
  color: var(--text-main);
  font-weight: 700;
}

.topbar__right {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.topbar__env-trigger {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
  color: #9a6700;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

.user-profile {
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 12px;
  margin: 0;
  padding: 10px 12px;
  border-radius: 10px;
  border: 0;
  background: transparent;
  color: #ffffff;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.user-profile:hover,
.user-profile:focus-visible {
  background: rgba(255, 255, 255, 0.1);
  color: #ffffff;
}

.user-profile:focus-visible {
  outline: 1px solid rgba(125, 211, 252, 0.45);
  outline-offset: 2px;
}

.user-profile__left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
}

.user-profile__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  overflow: hidden;
  border: 1px solid rgba(219, 234, 254, 0.45);
  box-sizing: border-box;
  background: #17489e;
}

.user-profile__avatar-img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.user-profile__name {
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 16px;
  font-weight: 500;
  line-height: 1.25;
  color: inherit;
}

.user-profile__arrow {
  flex-shrink: 0;
  font-size: 14px;
  color: inherit;
}

.sidebar--collapsed .user-profile {
  justify-content: center;
  padding: 8px 0;
}

.sidebar--collapsed .user-profile__left {
  flex: 0;
  justify-content: center;
}
</style>
