<template>
  <div class="app-layout">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand__mark">GC</div>
        <div>
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
        >
          {{ String(route.meta?.title ?? route.name ?? route.path) }}
        </RouterLink>
      </nav>
    </aside>

    <main class="content">
      <header class="topbar page-card">
        <div class="topbar__left">
          <h1>{{ currentTitle }}</h1>
          <p>围绕游戏、渠道、商品、IAP、收银台和生产同步组织页面。</p>
        </div>
        <EnvironmentBadge :environment="app.environment" />
      </header>

      <section class="content__body">
        <RouterView />
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAppStore } from "@/stores/app";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";

const route = useRoute();
const router = useRouter();
const app = useAppStore();

const currentTitle = computed(() => String(route.meta?.title ?? "Publishing Console"));

const visibleRoutes = computed(() =>
  (router.options.routes[0]?.children ?? []).filter((item) => !item.meta?.hidden)
);
</script>

<style scoped>
.app-layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  min-height: 100vh;
}

.sidebar {
  padding: 20px;
  background: linear-gradient(180deg, #0f172a 0%, #13315c 100%);
  color: #eff6ff;
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
}

.menu__item {
  padding: 12px 14px;
  border-radius: 12px;
  color: #dbeafe;
  transition: 0.2s ease;
}

.menu__item.router-link-active {
  background: rgba(255, 255, 255, 0.12);
  color: #ffffff;
}

.content {
  padding: 20px;
}

.content__body {
  margin-top: 16px;
}

.topbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 20px;
}

.topbar h1 {
  margin: 0;
  font-size: 24px;
}

.topbar p {
  margin: 8px 0 0;
  color: var(--text-subtle);
}
</style>
