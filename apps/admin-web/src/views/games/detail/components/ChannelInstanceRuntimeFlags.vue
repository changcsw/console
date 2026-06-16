<template>
  <div class="runtime-flags">
    <PageStatusTag :tone="includedInSnapshot ? 'success' : 'neutral'" :label="includedInSnapshot ? '快照生效' : '未进快照'" />
    <PageStatusTag :tone="includedInSync ? 'success' : 'neutral'" :label="includedInSync ? '同步生效' : '不同步'" />
    <PageStatusTag :tone="includedInRuntimeConfig ? 'success' : 'warning'" :label="includedInRuntimeConfig ? '客户端生效' : '客户端失效'" />
  </div>
</template>

<script setup lang="ts">
import PageStatusTag from "@/components/page/PageStatusTag.vue";

defineProps<{
  includedInSnapshot: boolean;
  includedInSync: boolean;
  includedInRuntimeConfig: boolean;
}>();
</script>

<style scoped>
.runtime-flags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
