<template>
  <PageCard title="GameMarketChannel" description="默认展示当前游戏下所有 market 的渠道实例，可按 market 过滤。">
    <div class="channel-toolbar">
      <div class="channel-toolbar__meta">
        <PageStatusTag tone="neutral" :label="`当前游戏：${gameId}`" />
        <PageStatusTag tone="success" :label="`实例数：${items.length}`" />
      </div>
      <div class="channel-toolbar__actions">
        <label class="market-filter">
          <span>市场过滤</span>
          <select v-model="selectedMarket">
            <option value="">全部市场</option>
            <option v-for="market in marketOptions" :key="market" :value="market">{{ market }}</option>
          </select>
        </label>
        <button class="create-button" type="button" @click="openCreateDrawer()">新建渠道实例</button>
      </div>
    </div>

    <ChannelInstanceTable
      :items="items"
      :selected-market="selectedMarket"
      @copy="openCopyDrawer"
      @hide="hideChannel"
      @unhide="unhideChannel"
    />

    <CreateMarketChannelDrawer
      :open="drawerOpen"
      :game-id="gameId"
      :selected-market="selectedMarket"
      :available-markets="availableMarkets"
      :source-instance="drawerSource"
      @close="drawerOpen = false"
      @created="applyCreatedItem"
    />
  </PageCard>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import PageCard from "@/components/page/PageCard.vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import {
  fetchGameMarketChannels,
  hideGameMarketChannel,
  type GameMarketChannelListItem,
  type SourceMarketChannelInstance,
  unhideGameMarketChannel
} from "@/api/gameMarketChannels";
import ChannelInstanceTable from "./components/ChannelInstanceTable.vue";
import CreateMarketChannelDrawer from "./components/CreateMarketChannelDrawer.vue";

const props = defineProps<{
  gameId: string;
}>();

const items = ref<GameMarketChannelListItem[]>([]);
const selectedMarket = ref("");
const drawerOpen = ref(false);
const drawerSource = ref<SourceMarketChannelInstance | null>(null);

const marketOptions = computed(() => {
  return Array.from(new Set(items.value.map((item) => item.market)));
});

const availableMarkets = computed(() => {
  return marketOptions.value.length > 0 ? marketOptions.value : ["GLOBAL", "JP", "KR", "SEA", "HMT", "CN"];
});

onMounted(async () => {
  items.value = await fetchGameMarketChannels(props.gameId);
});

function openCreateDrawer() {
  drawerSource.value = null;
  drawerOpen.value = true;
}

function openCopyDrawer(item: GameMarketChannelListItem) {
  drawerSource.value = {
    market: item.market,
    channelId: item.channelId,
    normalConfig: item.normalConfig ?? { clientId: `${item.market.toLowerCase()}-${item.channelId}` },
    secretConfig: item.secretConfig,
    fileConfig: item.fileConfig
  };
  drawerOpen.value = true;
}

function applyCreatedItem(item: GameMarketChannelListItem) {
  const nextItems = items.value.filter((existing) => existing.id !== item.id);
  items.value = [item, ...nextItems];
}

async function hideChannel(id: string) {
  const hiddenItem = await hideGameMarketChannel(id);
  items.value = items.value.map((item) => {
    if (item.id !== id) {
      return item;
    }

    return {
      ...item,
      ...hiddenItem,
      hidden: true,
      includedInSnapshot: false,
      includedInSync: false,
      includedInRuntimeConfig: false
    };
  });
}

async function unhideChannel(id: string) {
  const visibleItem = await unhideGameMarketChannel(id);
  items.value = items.value.map((item) => {
    if (item.id !== id) {
      return item;
    }

    return {
      ...item,
      ...visibleItem,
      hidden: false,
      includedInSnapshot: true,
      includedInSync: true,
      includedInRuntimeConfig: item.configStatus === "valid"
    };
  });
}
</script>

<style scoped>
.channel-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.channel-toolbar__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.channel-toolbar__actions {
  align-items: flex-end;
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.market-filter {
  display: grid;
  gap: 8px;
}

.market-filter span {
  color: var(--text-subtle);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.market-filter select {
  border: 1px solid var(--panel-border);
  border-radius: 999px;
  font: inherit;
  min-width: 180px;
  padding: 10px 14px;
}

.create-button {
  background: var(--brand);
  border: 1px solid transparent;
  border-radius: 999px;
  color: #ffffff;
  cursor: pointer;
  font: inherit;
  font-weight: 600;
  padding: 10px 16px;
}

@media (max-width: 960px) {
  .channel-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .channel-toolbar__actions {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
