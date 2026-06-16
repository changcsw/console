<template>
  <span class="env-badge" :data-env="environment">{{ environment.toUpperCase() }}</span>
</template>

<script setup lang="ts">
defineProps<{
  environment: string;
}>();
</script>

<style scoped>
.env-badge {
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
  background: #ebfff1;
  color: #14532d;
}

.env-badge[data-env="sandbox"] {
  background: #fff5db;
  color: #9a6700;
}

.env-badge[data-env="production"] {
  background: #ffe8e6;
  color: #b42318;
}
</style>

