<template>
  <PageCard class="metric-card">
    <div class="metric-card__head">
      <h3 class="metric-card__title">{{ title }}</h3>
      <el-tag v-if="!envScoped" size="small" type="warning" effect="light">全环境</el-tag>
      <el-tag v-else size="small" effect="light">当前环境</el-tag>
    </div>

    <div class="metric-card__main">
      <span class="metric-card__value" :class="valueClass">{{ value }}</span>
      <span class="metric-card__suffix">{{ valueSuffix }}</span>
    </div>
    <p class="metric-card__hint" :class="hintClass">
      {{ value > 0 ? busyText : emptyText }}
    </p>

    <div v-if="$slots.secondary" class="metric-card__secondary">
      <slot name="secondary" />
    </div>

    <div class="metric-card__actions">
      <el-button type="primary" link @click="$emit('navigate')">前往处理</el-button>
      <el-button v-if="expandable" link @click="$emit('toggleDetails')">
        {{ detailsExpanded ? "收起明细" : "展开明细" }}
      </el-button>
    </div>

    <div v-if="detailsExpanded" class="metric-card__details">
      <slot name="details" />
    </div>
  </PageCard>
</template>

<script setup lang="ts">
import { computed } from "vue";
import PageCard from "@/components/page/PageCard.vue";

const props = withDefaults(
  defineProps<{
    title: string;
    value: number;
    envScoped: boolean;
    valueSuffix?: string;
    emptyText?: string;
    busyText?: string;
    expandable?: boolean;
    detailsExpanded?: boolean;
  }>(),
  {
    valueSuffix: "条",
    emptyText: "暂无待办",
    busyText: "存在待处理项",
    expandable: false,
    detailsExpanded: false
  }
);

defineEmits<{
  navigate: [];
  toggleDetails: [];
}>();

const valueClass = computed(() => (props.value > 0 ? "metric-card__value--warning" : "metric-card__value--ok"));
const hintClass = computed(() => (props.value > 0 ? "metric-card__hint--warning" : "metric-card__hint--ok"));
</script>

<style scoped>
.metric-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.metric-card__title {
  margin: 0;
  font-size: 16px;
}

.metric-card__main {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  margin-top: 12px;
}

.metric-card__value {
  font-size: 38px;
  font-weight: 700;
  line-height: 1;
}

.metric-card__value--warning {
  color: #d97706;
}

.metric-card__value--ok {
  color: #15803d;
}

.metric-card__suffix {
  color: var(--text-subtle);
  font-size: 13px;
}

.metric-card__hint {
  margin: 8px 0 0;
  font-size: 13px;
}

.metric-card__hint--warning {
  color: #b45309;
}

.metric-card__hint--ok {
  color: #15803d;
}

.metric-card__secondary {
  margin-top: 12px;
}

.metric-card__actions {
  margin-top: 10px;
  display: flex;
  justify-content: space-between;
}

.metric-card__details {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px dashed #dbe3ed;
}
</style>
