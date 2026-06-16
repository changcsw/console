<template>
  <span :style="styleObject" class="status-tag">{{ label }}</span>
</template>

<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{
  tone?: "neutral" | "success" | "warning" | "danger";
  label: string;
}>();

const styleObject = computed(() => {
  switch (props.tone) {
    case "success":
      return { background: "#dff5e7", color: "#14532d" };
    case "warning":
      return { background: "#fff1d6", color: "#b54708" };
    case "danger":
      return { background: "#fde7e6", color: "#b42318" };
    default:
      return { background: "#ebeff3", color: "#445469" };
  }
});
</script>

<style scoped>
.status-tag {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 4px 10px;
  font-size: 12px;
  font-weight: 600;
}
</style>

