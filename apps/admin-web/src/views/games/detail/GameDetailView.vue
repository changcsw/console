<template>
  <div class="page-shell">
    <PageCard v-if="game" class="detail-basic-card">
      <div class="detail-head">
        <div class="detail-head__left">
          <h2 class="detail-head__title">{{ game.name }}</h2>
          <PageStatusTag :tone="statusMeta(game.status).tone" :label="statusMeta(game.status).label" />
        </div>
        <div class="detail-head__actions">
          <el-button v-if="!hasTopbarBridge" link @click="goBack">← 返回列表</el-button>
          <el-button
            v-if="!hasTopbarBridge && app.environment === 'sandbox'"
            v-perm="'sync.execute'"
            type="primary"
            :disabled="!canSyncExecute"
            @click="openSyncDrawer"
          >
            Sync to Production
          </el-button>
          <EnvironmentBadge v-if="!hasTopbarBridge" :environment="app.environment" />
          <el-button v-perm="'game.write'" type="primary" @click="basicInfoRef?.openEdit()">编辑基础信息</el-button>
        </div>
      </div>

      <BasicInfoTab ref="basicInfoRef" :game="game" hide-toolbar @updated="onUpdated" />
    </PageCard>

    <PageCard v-loading="loading">
      <div v-if="notFound" class="empty-state">
        <p class="empty-state__title">游戏不存在或已切换环境</p>
        <p class="empty-state__hint">请返回列表确认当前运行环境与游戏 ID。</p>
        <el-button type="primary" @click="goBack">返回列表</el-button>
      </div>

      <el-tabs v-else-if="game" v-model="activeTab">
        <el-tab-pane label="市场与法务" name="markets" lazy>
          <MarketsTab :game="game" @updated="onUpdated" />
        </el-tab-pane>
        <el-tab-pane label="自有账号认证" name="account-auth" lazy>
          <AccountAuthTab :game-id="game.gameId" />
        </el-tab-pane>
        <el-tab-pane label="商品" name="products" lazy>
          <ProductTab :game-id="game.gameId" />
        </el-tab-pane>
        <el-tab-pane label="IAP" name="iap" lazy>
          <IapConfigTab :game-id="game.gameId" />
        </el-tab-pane>
        <el-tab-pane label="收银台" name="cashier" lazy>
          <GameCashierTab :game-id="game.gameId" />
        </el-tab-pane>
        <el-tab-pane label="支付路由" name="payment-routes" lazy>
          <PaymentRoutesTab :game-id="game.gameId" />
        </el-tab-pane>
        <el-tab-pane label="配置快照" name="snapshot" lazy>
          <SnapshotTab :game-id="game.gameId" />
        </el-tab-pane>
        <el-tab-pane label="同步记录" name="sync" lazy>
          <SyncJobsTab v-if="game" ref="syncJobsTabRef" :game-id="game.gameId" />
        </el-tab-pane>
        <el-tab-pane v-for="ph in downstreamTabs" :key="ph.name" :label="ph.label" :name="ph.name" lazy>
          <div class="placeholder">
            <PageStatusTag tone="warning" label="下游模块" />
            <p>{{ ph.hint }}</p>
          </div>
        </el-tab-pane>
      </el-tabs>
    </PageCard>

    <SyncSectionDrawer
      v-if="game"
      :open="syncDrawerOpen"
      :game-id="game.gameId"
      @close="syncDrawerOpen = false"
      @executed="onSyncExecuted"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import { useAppStore } from "@/stores/app";
import { usePermissionStore } from "@/stores/permission";
import { ApiError } from "@/api/http";
import type { SyncExecuteResponse } from "@/api/syncSections";
import { getGame, type GameDetail } from "@/api/modules/games";
import BasicInfoTab from "./BasicInfoTab.vue";
import MarketsTab from "./MarketsTab.vue";
import AccountAuthTab from "./AccountAuthTab.vue";
import ProductTab from "./ProductTab.vue";
import IapConfigTab from "./IapConfigTab.vue";
import GameCashierTab from "@/views/cashier/game/GameCashierTab.vue";
import PaymentRoutesTab from "./PaymentRoutesTab.vue";
import SnapshotTab from "./SnapshotTab.vue";
import SyncJobsTab from "./SyncJobsTab.vue";
import SyncSectionDrawer from "./components/SyncSectionDrawer.vue";
import { statusMeta } from "../constants";
import { useTopbarBridge } from "@/layouts/topbarBridge";

const route = useRoute();
const router = useRouter();
const app = useAppStore();
const permission = usePermissionStore();

const game = ref<GameDetail | null>(null);
const loading = ref(false);
const notFound = ref(false);
const activeTab = ref("markets");
const syncDrawerOpen = ref(false);
const syncJobsTabRef = ref<{ reload: (page?: number) => Promise<void> } | null>(null);
const basicInfoRef = ref<{ openEdit: () => void } | null>(null);
const canSyncExecute = computed(() => permission.hasPerm("sync.execute"));
const topbar = useTopbarBridge();
const hasTopbarBridge = computed(() => topbar.connected.value);

const downstreamTabs = [
  { name: "channels", label: "渠道", hint: "渠道实例（GameMarketChannel）由 channel 模块实现。" },
  { name: "packages", label: "渠道包", hint: "渠道包配置由 channel 模块实现。" },
  { name: "channel-login", label: "渠道登录", hint: "渠道登录配置由 channel-login 模块实现。" }
];

function goBack() {
  void router.push({ name: "games" });
}

function onUpdated(next: GameDetail) {
  game.value = next;
}

function openSyncDrawer() {
  if (!game.value) {
    return;
  }
  syncDrawerOpen.value = true;
}

async function onSyncExecuted(_: SyncExecuteResponse) {
  syncDrawerOpen.value = false;
  activeTab.value = "sync";
  // 同步记录 Tab 为 lazy 挂载：切换后需等下一帧组件就绪再取 ref 触发刷新。
  // 若此前从未激活过该 Tab，其 onMounted 会自行 reload(1)，此处再显式拉取第 1 页以覆盖已挂载场景。
  await nextTick();
  await syncJobsTabRef.value?.reload(1);
}

function applyTopbarState() {
  topbar.setBreadcrumb([
    {
      key: "games",
      label: "游戏管理",
      onClick: () => {
        goBack();
      }
    },
    {
      key: "game-detail",
      label: game.value?.alias || game.value?.name || "游戏详情"
    }
  ]);
  topbar.setActions({
    environment: app.environment,
    showSyncButton: app.environment === "sandbox",
    canSyncExecute: canSyncExecute.value && Boolean(game.value),
    onChangeEnvironment(next) {
      app.setEnvironment(next);
      const gameId = route.params.gameId;
      if (typeof gameId === "string" && gameId) {
        void load(gameId);
      }
    },
    onSyncToProduction() {
      openSyncDrawer();
    }
  });
}

async function load(gameId: string) {
  loading.value = true;
  notFound.value = false;
  try {
    game.value = await getGame(gameId);
  } catch (err) {
    game.value = null;
    if (err instanceof ApiError && err.status === 404) {
      notFound.value = true;
    } else if (err instanceof ApiError) {
      ElMessage.error(err.message || "加载游戏详情失败");
    } else {
      ElMessage.error("加载游戏详情失败");
    }
  } finally {
    loading.value = false;
  }
}

watch(
  () => route.params.gameId,
  (gameId) => {
    if (typeof gameId === "string" && gameId) {
      void load(gameId);
    }
  }
);

watch(
  [() => game.value?.alias, () => game.value?.name, () => app.environment, canSyncExecute],
  () => {
    applyTopbarState();
  }
);

onMounted(() => {
  applyTopbarState();
  const gameId = route.params.gameId;
  if (typeof gameId === "string" && gameId) {
    void load(gameId);
  }
});

onBeforeUnmount(() => {
  topbar.setBreadcrumb(null);
  topbar.setActions(null);
});
</script>

<style scoped>
.detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.detail-head__left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.detail-head__title {
  margin: 0;
  font-size: 20px;
}

.detail-head__actions {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.placeholder {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 24px;
  color: var(--text-subtle);
}

.text-muted {
  color: var(--text-subtle);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 40px 0;
}

.empty-state__title {
  margin: 0;
  font-weight: 600;
  font-size: 16px;
}

.empty-state__hint {
  margin: 0;
  color: var(--text-subtle);
}

.detail-basic-card :deep(.basic-tab) {
  margin-top: 16px;
}
</style>
