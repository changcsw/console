<template>
  <PageCard title="模板版本" description="支持查看版本状态，并将 published 版本直接复制成新的 draft。">
    <div class="version-list">
      <article v-for="version in versions" :key="version.version" class="version-row">
        <div>
          <strong>v{{ version.version }}</strong>
          <span>{{ version.status }}</span>
        </div>
        <button
          v-if="version.status === 'published'"
          class="copy-button"
          type="button"
          @click="selectedVersion = version"
        >
          复制为 draft
        </button>
      </article>
    </div>

    <CopyPublishedToDraftDialog
      v-if="selectedVersion"
      :open="true"
      :source-version="selectedVersion"
      :template-id="templateId"
      @close="selectedVersion = null"
      @created="appendDraft"
    />
  </PageCard>
</template>

<script setup lang="ts">
import { ref } from "vue";
import PageCard from "@/components/page/PageCard.vue";
import CopyPublishedToDraftDialog from "./components/CopyPublishedToDraftDialog.vue";
import type { TemplateVersion } from "@/api/templateVersions";

const props = defineProps<{
  templateId?: string;
}>();

const versions = ref<TemplateVersion[]>([
  { version: 7, status: "published" },
  { version: 6, status: "archived" }
]);
const selectedVersion = ref<TemplateVersion | null>(null);

function appendDraft(created: TemplateVersion) {
  selectedVersion.value = null;
  versions.value = [created, ...versions.value.filter((item) => item.version !== created.version)];
}
</script>

<style scoped>
.version-list {
  display: grid;
  gap: 12px;
}

.version-row {
  align-items: center;
  background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
  border: 1px solid var(--panel-border);
  border-radius: var(--radius-md);
  display: flex;
  justify-content: space-between;
  padding: 16px;
}

.version-row div {
  display: grid;
  gap: 4px;
}

.version-row span {
  color: var(--text-subtle);
  text-transform: uppercase;
}

.copy-button {
  background: var(--brand);
  border: 1px solid transparent;
  border-radius: 999px;
  color: #ffffff;
  cursor: pointer;
  font: inherit;
  font-weight: 600;
  padding: 10px 14px;
}
</style>
