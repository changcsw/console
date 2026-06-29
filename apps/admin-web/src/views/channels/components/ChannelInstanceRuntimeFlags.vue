<template>
  <div class="runtime-flags">
    <el-tooltip v-if="reason" :content="reason" placement="top">
      <span class="runtime-flags__group is-blocked">
        <PageStatusTag tone="neutral" label="快照" />
        <PageStatusTag tone="neutral" label="同步" />
        <PageStatusTag tone="warning" label="客户端" />
      </span>
    </el-tooltip>
    <span v-else class="runtime-flags__group">
      <PageStatusTag :tone="includedInSnapshot ? 'success' : 'neutral'" label="快照" />
      <PageStatusTag :tone="includedInSync ? 'success' : 'neutral'" label="同步" />
      <PageStatusTag :tone="includedInRuntimeConfig ? 'success' : 'warning'" label="客户端" />
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import type { ConfigStatus } from "@/api/modules/channels";
import { runtimeBlockReason } from "../constants";

const props = defineProps<{
  hidden: boolean;
  compatible: boolean;
  configStatus: ConfigStatus;
  includedInSnapshot: boolean;
  includedInSync: boolean;
  includedInRuntimeConfig: boolean;
}>();

const reason = computed(() =>
  runtimeBlockReason({ hidden: props.hidden, compatible: props.compatible, configStatus: props.configStatus })
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
