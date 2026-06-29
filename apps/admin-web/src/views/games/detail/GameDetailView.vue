<template>
  <div class="page-shell">
    <PageCard>
      <div class="detail-head">
        <div class="detail-head__left">
          <el-button link @click="goBack">← 返回列表</el-button>
          <template v-if="game">
            <h2 class="detail-head__title">{{ game.name }}</h2>
            <PageStatusTag :tone="statusMeta(game.status).tone" :label="statusMeta(game.status).label" />
          </template>
        </div>
        <EnvironmentBadge :environment="app.environment" />
      </div>

      <div v-if="game" class="detail-head__meta">
        <span class="meta-item"><b>Game ID</b> <code>{{ game.gameId }}</code></span>
        <span class="meta-item"><b>代号</b> {{ game.alias }}</span>
        <span class="meta-item"><b>默认市场</b> {{ game.defaultMarketCode }}</span>
        <span class="meta-item"><b>Secret</b> <code class="text-muted">{{ game.gameSecret || "masked" }}</code></span>
      </div>
    </PageCard>

    <PageCard v-loading="loading">
      <div v-if="notFound" class="empty-state">
        <p class="empty-state__title">游戏不存在或已切换环境</p>
        <p class="empty-state__hint">请返回列表确认当前运行环境与游戏 ID。</p>
        <el-button type="primary" @click="goBack">返回列表</el-button>
      </div>

      <el-tabs v-else-if="game" v-model="activeTab">
        <el-tab-pane label="基础信息" name="basic">
          <BasicInfoTab :game="game" @updated="onUpdated" />
        </el-tab-pane>
        <el-tab-pane label="市场" name="markets">
          <MarketsTab :game="game" @updated="onUpdated" />
        </el-tab-pane>
        <el-tab-pane label="法务链接" name="legal">
          <LegalLinksTab :game="game" @updated="onUpdated" />
        </el-tab-pane>
        <el-tab-pane label="自有账号认证" name="account-auth">
          <AccountAuthTab :game-id="game.gameId" />
        </el-tab-pane>
        <el-tab-pane v-for="ph in downstreamTabs" :key="ph.name" :label="ph.label" :name="ph.name" lazy>
          <div class="placeholder">
            <PageStatusTag tone="warning" label="下游模块" />
            <p>{{ ph.hint }}</p>
          </div>
        </el-tab-pane>
      </el-tabs>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import { useAppStore } from "@/stores/app";
import { ApiError } from "@/api/http";
import { getGame, type GameDetail } from "@/api/modules/games";
import BasicInfoTab from "./BasicInfoTab.vue";
import MarketsTab from "./MarketsTab.vue";
import LegalLinksTab from "./LegalLinksTab.vue";
import AccountAuthTab from "./AccountAuthTab.vue";
import { statusMeta } from "../constants";

const route = useRoute();
const router = useRouter();
const app = useAppStore();

const game = ref<GameDetail | null>(null);
const loading = ref(false);
const notFound = ref(false);
const activeTab = ref("basic");

// 下游模块占位（渠道/商品/收银台/同步等由各自模块实现）
const downstreamTabs = [
  { name: "channels", label: "渠道", hint: "渠道实例（GameMarketChannel）由 channel 模块实现。" },
  { name: "packages", label: "渠道包", hint: "渠道包配置由 channel 模块实现。" },
  { name: "products", label: "商品", hint: "商品与 IAP 映射由 product 模块实现。" },
  { name: "channel-login", label: "渠道登录", hint: "渠道登录配置由 channel-login 模块实现。" },
  { name: "iap", label: "IAP", hint: "渠道 IAP 配置由 product 模块实现。" },
  { name: "cashier", label: "收银台", hint: "游戏级收银台由 game-cashier 模块实现。" },
  { name: "payment", label: "支付路由", hint: "支付路由由 payment 模块实现。" },
  { name: "snapshot", label: "配置快照", hint: "配置快照由 snapshot 模块实现。" },
  { name: "sync", label: "同步记录", hint: "sandbox→production 同步由 sync 模块实现。" }
];

function goBack() {
  void router.push({ name: "games" });
}

function onUpdated(next: GameDetail) {
  game.value = next;
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

onMounted(() => {
  const gameId = route.params.gameId;
  if (typeof gameId === "string" && gameId) {
    void load(gameId);
  }
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

.detail-head__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  margin-top: 14px;
  color: var(--text-subtle);
  font-size: 13px;
}

.meta-item b {
  color: var(--text-main);
  margin-right: 6px;
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
</style>
