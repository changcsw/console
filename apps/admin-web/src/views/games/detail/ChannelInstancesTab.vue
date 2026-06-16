<template>
  <PageCard title="GameMarketChannel" description="默认展示当前游戏下所有 market 的渠道实例，可按 market 过滤。">
    <div class="channel-toolbar">
      <div class="channel-toolbar__meta">
        <PageStatusTag tone="neutral" :label="`当前游戏：${gameId}`" />
        <PageStatusTag tone="success" :label="`实例数：${items.length}`" />
      </div>

      <el-select v-model="selectedMarket" clearable placeholder="全部市场" style="width: 220px">
        <el-option label="全部市场" value="" />
        <el-option v-for="market in marketOptions" :key="market" :label="market" :value="market" />
      </el-select>
    </div>

    <ChannelInstanceTable :items="items" :selected-market="selectedMarket" />
  </PageCard>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import PageCard from "@/components/page/PageCard.vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { fetchGameMarketChannels, type GameMarketChannelListItem } from "@/api/gameMarketChannels";
import ChannelInstanceTable from "./components/ChannelInstanceTable.vue";

const props = defineProps<{
  gameId: string;
}>();

const items = ref<GameMarketChannelListItem[]>([]);
const selectedMarket = ref("");

const marketOptions = computed(() => {
  return Array.from(new Set(items.value.map((item) => item.market)));
});

onMounted(async () => {
  items.value = await fetchGameMarketChannels(props.gameId);
});
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

@media (max-width: 960px) {
  .channel-toolbar {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
