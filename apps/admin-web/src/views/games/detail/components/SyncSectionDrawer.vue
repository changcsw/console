<template>
  <div v-if="open" class="drawer-backdrop">
    <aside class="drawer-panel">
      <div class="drawer-panel__header">
        <div>
          <h3>Section 同步</h3>
          <p>按 section 选择同步范围，执行时只提交勾选项。</p>
        </div>
        <button class="ghost-button" type="button" @click="$emit('close')">Close</button>
      </div>

      <label v-for="item in preview" :key="item.section" class="section-option">
        <input
          :aria-label="item.section"
          :checked="selectedSections.includes(item.section)"
          type="checkbox"
          @change="toggleSection(item.section)"
        />
        <span>{{ item.section }}</span>
      </label>

      <div class="drawer-actions">
        <button class="primary-button" type="button" @click="execute">Execute</button>
      </div>

      <p v-if="executedLabel" class="payload-preview">{{ executedLabel }}</p>
    </aside>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { executeSyncSections, type SyncPreviewSection, type SyncSection } from "@/api/syncSections";

const props = defineProps<{
  open: boolean;
  gameId?: string;
  preview: SyncPreviewSection[];
}>();

const emit = defineEmits<{
  close: [];
  executed: [payload: { selected_sections: SyncSection[] }];
}>();

const selectedSections = ref<SyncSection[]>([]);
const executedLabel = ref("");

function toggleSection(section: SyncSection) {
  if (selectedSections.value.includes(section)) {
    selectedSections.value = selectedSections.value.filter((item) => item !== section);
    return;
  }

  selectedSections.value = [...selectedSections.value, section];
}

async function execute() {
  const payload = { selected_sections: selectedSections.value };
  executedLabel.value = `selected_sections: ${payload.selected_sections.join(", ")}`;

  if (props.gameId) {
    await executeSyncSections(props.gameId, payload);
  }

  emit("executed", payload);
}
</script>

<style scoped>
.drawer-backdrop {
  align-items: stretch;
  background: rgba(15, 23, 42, 0.22);
  display: flex;
  inset: 0;
  justify-content: flex-end;
  position: fixed;
  z-index: 45;
}

.drawer-panel {
  background: #ffffff;
  box-shadow: -24px 0 48px rgba(15, 23, 42, 0.12);
  display: grid;
  gap: 16px;
  max-width: 420px;
  padding: 24px;
  width: min(100%, 420px);
}

.drawer-panel__header {
  display: flex;
  gap: 16px;
  justify-content: space-between;
}

.drawer-panel__header h3,
.drawer-panel__header p,
.payload-preview {
  margin: 0;
}

.drawer-panel__header p,
.payload-preview {
  color: var(--text-subtle);
}

.section-option {
  align-items: center;
  background: #f8fbff;
  border: 1px solid var(--panel-border);
  border-radius: var(--radius-sm);
  display: flex;
  gap: 10px;
  padding: 10px 12px;
}

.drawer-actions {
  display: flex;
  justify-content: flex-end;
}

.ghost-button,
.primary-button {
  border: 1px solid transparent;
  border-radius: 999px;
  cursor: pointer;
  font: inherit;
  font-weight: 600;
  padding: 10px 14px;
}

.ghost-button {
  background: #ffffff;
  border-color: var(--panel-border);
}

.primary-button {
  background: var(--brand);
  color: #ffffff;
}
</style>
