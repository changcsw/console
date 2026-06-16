<template>
  <div class="status-tags">
    <PageStatusTag v-if="hidden" tone="danger" label="已隐藏" />
    <PageStatusTag v-if="incompatibleWithMarket" tone="danger" label="市场不兼容" />
    <PageStatusTag :tone="statusTone" :label="statusLabel" />
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import type { ChannelConfigStatus } from "@/api/gameMarketChannels";

const props = defineProps<{
  status: ChannelConfigStatus;
  hidden: boolean;
  incompatibleWithMarket: boolean;
}>();

const statusTone = computed(() => {
  switch (props.status) {
    case "valid":
      return "success";
    case "invalid":
      return "warning";
    default:
      return "neutral";
  }
});

const statusLabel = computed(() => {
  switch (props.status) {
    case "valid":
      return "配置有效";
    case "invalid":
      return "待补配置";
    default:
      return "尚未配置";
  }
});
</script>

<style scoped>
.status-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
