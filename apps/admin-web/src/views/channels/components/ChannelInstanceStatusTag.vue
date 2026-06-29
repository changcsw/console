<template>
  <span class="status-cluster">
    <PageStatusTag :tone="meta.tone" :label="meta.label" />
    <PageStatusTag v-if="hidden" tone="neutral" label="已隐藏" />
    <PageStatusTag v-else-if="!compatible" tone="danger" label="不兼容" />
  </span>
</template>

<script setup lang="ts">
import { computed } from "vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import type { ConfigStatus } from "@/api/modules/channels";
import { configStatusMeta } from "../constants";

const props = defineProps<{
  configStatus: ConfigStatus;
  compatible: boolean;
  hidden: boolean;
}>();

const meta = computed(() => configStatusMeta(props.configStatus));
</script>

<style scoped>
.status-cluster {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 6px;
}
</style>
