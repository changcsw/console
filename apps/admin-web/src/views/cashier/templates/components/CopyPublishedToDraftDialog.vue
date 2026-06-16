<template>
  <div v-if="open" class="dialog-backdrop">
    <section class="dialog-panel">
      <h3>Published v{{ sourceVersion.version }} draft copy</h3>
      <p>已发布版本不可原地编辑，但可以一键复制为新的 draft 继续修改。</p>

      <div class="dialog-actions">
        <button class="ghost-button" type="button" @click="$emit('close')">Cancel</button>
        <button class="primary-button" type="button" @click="submit">Create draft from published v{{ sourceVersion.version }}</button>
      </div>

      <p v-if="createdLabel" class="dialog-result">{{ createdLabel }}</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { copyPublishedToDraft, type TemplateVersion } from "@/api/templateVersions";

const props = defineProps<{
  open: boolean;
  templateId?: string;
  sourceVersion: TemplateVersion;
}>();

const emit = defineEmits<{
  close: [];
  created: [version: TemplateVersion];
}>();

const createdLabel = ref("");

async function submit() {
  const created = await copyPublishedToDraft(props.templateId ?? "template-1", props.sourceVersion.version);
  createdLabel.value = `draft v${created.version} created`;
  emit("created", created);
}
</script>

<style scoped>
.dialog-backdrop {
  align-items: center;
  background: rgba(15, 23, 42, 0.18);
  display: flex;
  inset: 0;
  justify-content: center;
  position: fixed;
  z-index: 50;
}

.dialog-panel {
  background: #ffffff;
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-soft);
  display: grid;
  gap: 16px;
  max-width: 520px;
  padding: 24px;
  width: min(100%, 520px);
}

.dialog-panel h3,
.dialog-panel p {
  margin: 0;
}

.dialog-panel p {
  color: var(--text-subtle);
}

.dialog-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

.dialog-result {
  color: var(--brand);
  font-weight: 700;
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
