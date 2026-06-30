<template>
  <div class="runtime-flags">
    <el-tooltip v-if="reason" :content="reason" placement="top">
      <span class="runtime-flags__group is-blocked">
        <PageStatusTag tone="neutral" label="Included in Snapshot" />
        <PageStatusTag tone="neutral" label="Included in Sync" />
        <PageStatusTag tone="warning" label="Included in Runtime Config" />
      </span>
    </el-tooltip>
    <span v-else class="runtime-flags__group">
      <PageStatusTag :tone="includedInSnapshot ? 'success' : 'neutral'" label="Included in Snapshot" />
      <PageStatusTag :tone="includedInSync ? 'success' : 'neutral'" label="Included in Sync" />
      <PageStatusTag :tone="includedInRuntimeConfig ? 'success' : 'warning'" label="Included in Runtime Config" />
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import type { ConfigStatus } from "@/api/modules/channels";
import { runtimeBlockReason } from "../constants";

const props = defineProps<{
  enabled?: boolean;
  hidden: boolean;
  compatible: boolean;
  configStatus: ConfigStatus;
  includedInSnapshot: boolean;
  includedInSync: boolean;
  includedInRuntimeConfig: boolean;
}>();

const reason = computed(() =>
  runtimeBlockReason({
    enabled: props.enabled,
    hidden: props.hidden,
    compatible: props.compatible,
    configStatus: props.configStatus
  })
);
</script>

<style scoped>
.runtime-flags__group {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 6px;
}

.runtime-flags__group.is-blocked {
  opacity: 0.55;
  cursor: help;
}
</style>
