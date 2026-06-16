<template>
  <div class="instance-table">
    <div class="instance-table__head">
      <span>市场</span>
      <span>渠道 ID</span>
      <span>配置状态</span>
      <span>生效范围</span>
    </div>

    <div v-if="filteredItems.length === 0" class="instance-table__empty">当前筛选下暂无渠道实例</div>

    <article v-for="item in filteredItems" :key="item.id" class="instance-table__row">
      <div class="instance-table__market">{{ item.market }}</div>
      <div class="instance-table__channel">{{ item.channelId }}</div>
      <ChannelInstanceStatusTag
        :status="item.configStatus"
        :hidden="item.hidden"
        :incompatible-with-market="item.incompatibleWithMarket"
      />
      <ChannelInstanceRuntimeFlags
        :included-in-runtime-config="item.includedInRuntimeConfig"
        :included-in-snapshot="item.includedInSnapshot"
        :included-in-sync="item.includedInSync"
      />
    </article>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { GameMarketChannelListItem } from "@/api/gameMarketChannels";
import ChannelInstanceRuntimeFlags from "./ChannelInstanceRuntimeFlags.vue";
import ChannelInstanceStatusTag from "./ChannelInstanceStatusTag.vue";

const props = withDefaults(
  defineProps<{
    items: GameMarketChannelListItem[];
    selectedMarket?: string;
  }>(),
  {
    selectedMarket: ""
  }
);

const filteredItems = computed(() => {
  if (!props.selectedMarket) {
    return props.items;
  }

  return props.items.filter((item) => item.market === props.selectedMarket);
});
</script>

<style scoped>
.instance-table {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.instance-table__head,
.instance-table__row {
  display: grid;
  gap: 12px;
  grid-template-columns: 110px 180px minmax(220px, 1fr) minmax(260px, 1.2fr);
}

.instance-table__head {
  color: var(--text-subtle);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
  padding: 0 4px;
  text-transform: uppercase;
}

.instance-table__row {
  align-items: center;
  background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
  border: 1px solid rgba(20, 83, 45, 0.08);
  border-radius: var(--radius-md);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.04);
  padding: 16px;
}

.instance-table__market,
.instance-table__channel {
  font-weight: 700;
}

.instance-table__empty {
  border: 1px dashed var(--panel-border);
  border-radius: var(--radius-md);
  color: var(--text-subtle);
  padding: 20px;
  text-align: center;
}

@media (max-width: 960px) {
  .instance-table__head {
    display: none;
  }

  .instance-table__row {
    grid-template-columns: 1fr;
  }
}
</style>
