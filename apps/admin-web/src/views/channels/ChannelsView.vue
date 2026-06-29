<template>
  <div class="page-shell">
    <PageCard
      title="渠道实例管理"
      description="按 market 拆分的渠道实例（GameMarketChannel）：可见性/复制创建/隐藏/运行态标识/渠道包。先选择游戏再管理其渠道实例。"
    >
      <div class="toolbar">
        <div class="toolbar__left">
          <el-select
            v-model="selectedGameId"
            placeholder="选择游戏"
            filterable
            class="game-select"
            :loading="gamesLoading"
            @change="onGameChange"
          >
            <el-option
              v-for="g in games"
              :key="g.gameId"
              :label="`${g.name}（${g.gameId}）`"
              :value="g.gameId"
            />
          </el-select>
        </div>
        <EnvironmentBadge :environment="app.environment" />
      </div>

      <ChannelInstancesTab v-if="selectedGameId" :key="selectedGameId" :game-id="selectedGameId" />
      <div v-else class="empty-state">
        <p class="empty-state__title">请选择游戏</p>
        <p class="empty-state__hint">渠道实例隶属于具体游戏与 market，选择游戏后展示其全部渠道实例。</p>
      </div>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import { useAppStore } from "@/stores/app";
import { ApiError } from "@/api/http";
import { listGames, type GameListItem } from "@/api/modules/games";
import ChannelInstancesTab from "./components/ChannelInstancesTab.vue";

const app = useAppStore();
const route = useRoute();
const router = useRouter();

const games = ref<GameListItem[]>([]);
const gamesLoading = ref(false);
const selectedGameId = ref<string>("");

async function loadGames() {
  gamesLoading.value = true;
  try {
    const res = await listGames({ page: 1, pageSize: 100 });
    games.value = res.items;
    const fromQuery = typeof route.query.gameId === "string" ? route.query.gameId : "";
    if (fromQuery && res.items.some((g) => g.gameId === fromQuery)) {
      selectedGameId.value = fromQuery;
    } else if (!selectedGameId.value && res.items.length > 0) {
      selectedGameId.value = res.items[0].gameId;
    }
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载游戏列表失败");
  } finally {
    gamesLoading.value = false;
  }
}

function onGameChange(gameId: string) {
  void router.replace({ query: { ...route.query, gameId } });
}

onMounted(() => {
  void loadGames();
});
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.game-select {
  width: 280px;
}

.empty-state {
  padding: 48px 0;
  text-align: center;
}

.empty-state__title {
  margin: 0;
  font-weight: 600;
  font-size: 16px;
}

.empty-state__hint {
  margin: 8px 0 0;
  color: var(--text-subtle);
}
</style>
