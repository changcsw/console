<template>
  <el-drawer
    :model-value="open"
    title="Sync to Production"
    size="960px"
    :close-on-click-modal="false"
    @update:model-value="onDrawerChange"
  >
    <div class="sync-drawer">
      <div class="sync-drawer__toolbar">
        <el-space>
          <el-button :loading="previewLoading" @click="loadPreview">重新预览</el-button>
          <PageStatusTag
            v-if="preview"
            tone="neutral"
            :label="`baseline: ${preview.targetHashBefore.slice(0, 10)}…`"
          />
        </el-space>
      </div>

      <el-alert
        v-if="previewError"
        type="error"
        :closable="false"
        :title="previewError"
      />

      <el-result
        v-else-if="!preview && !previewLoading"
        icon="info"
        title="暂无可执行预览"
        sub-title="点击“重新预览”加载最新 sandbox→production 差异。"
      />

      <template v-else-if="preview">
        <el-alert
          v-if="!preview.hasDiff"
          type="info"
          :closable="false"
          title="sandbox 与 production 无差异，无需同步。"
        />

        <el-alert
          v-if="executeError"
          type="warning"
          :closable="false"
          :title="executeError"
        />

        <div class="sync-drawer__form">
          <el-switch
            v-model="includeDeletes"
            active-text="执行 delete"
            inactive-text="仅提示 delete"
          />
          <el-input
            v-model="operatorNote"
            maxlength="255"
            show-word-limit
            placeholder="备注（可选）"
          />
        </div>

        <div class="sync-section-list">
          <section v-for="section in preview.sections" :key="section.section" class="sync-section-card">
            <header class="sync-section-card__head">
              <label class="sync-section-card__selector">
                <input
                  :checked="selectedSections.includes(section.section)"
                  type="checkbox"
                  :aria-label="section.section"
                  @change="toggleSection(section.section)"
                />
                <strong>{{ section.section }}</strong>
              </label>
              <el-space size="small">
                <el-tag type="success">add {{ section.summary.add }}</el-tag>
                <el-tag type="warning">update {{ section.summary.update }}</el-tag>
                <el-tag type="danger">delete {{ section.summary.delete }}</el-tag>
              </el-space>
            </header>

            <p v-if="section.dependencies.length" class="sync-section-card__deps">
              依赖：{{ section.dependencies.join(" / ") }}
            </p>

            <div v-if="section.changes.length === 0" class="sync-change sync-change--empty">
              无差异
            </div>

            <div
              v-for="change in section.changes"
              :key="`${section.section}-${change.entityType}-${change.entityKey}-${change.fieldName}-${change.op}`"
              class="sync-change"
              :class="[
                `sync-change--${change.op}`,
                change.op === 'delete' && !includeDeletes ? 'sync-change--delete-muted' : ''
              ]"
            >
              <div class="sync-change__head">
                <el-tag :type="tagType(change.op)" size="small">{{ change.op }}</el-tag>
                <code>{{ change.entityType }} / {{ change.entityKey }} / {{ change.fieldName }}</code>
                <el-tag v-if="change.op === 'delete' && !includeDeletes" size="small" type="info">仅提示，不执行</el-tag>
              </div>
              <div class="sync-change__values">
                <span><b>sandbox:</b> {{ formatDiffValue(change.sandboxValue, change.masked) }}</span>
                <span><b>production:</b> {{ formatDiffValue(change.productionValue, change.masked) }}</span>
              </div>
            </div>
          </section>
        </div>
      </template>
    </div>

    <template #footer>
      <div class="sync-drawer__footer">
        <el-button @click="emit('close')">取消</el-button>
        <el-button
          type="primary"
          :loading="executeLoading"
          :disabled="!preview || !preview.hasDiff || selectedSections.length === 0"
          @click="execute"
        >
          执行同步
        </el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { ApiError } from "@/api/http";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import {
  executeSyncSections,
  previewSyncSections,
  type SyncExecuteResponse,
  type SyncOp,
  type SyncPreviewResponse,
  type SyncSection
} from "@/api/syncSections";

const props = defineProps<{
  open: boolean;
  gameId: string;
}>();

const emit = defineEmits<{
  close: [];
  executed: [result: SyncExecuteResponse];
}>();

const preview = ref<SyncPreviewResponse | null>(null);
const previewLoading = ref(false);
const executeLoading = ref(false);
const includeDeletes = ref(false);
const operatorNote = ref("");
const selectedSections = ref<SyncSection[]>([]);
const previewError = ref("");
const executeError = ref("");

function onDrawerChange(next: boolean) {
  if (!next) {
    emit("close");
  }
}

function tagType(op: SyncOp): "success" | "warning" | "danger" {
  if (op === "add") {
    return "success";
  }
  if (op === "update") {
    return "warning";
  }
  return "danger";
}

function looksMasked(value: unknown): boolean {
  return typeof value === "string" && ["masked", "******", "***", "••••••"].includes(value.toLowerCase());
}

function formatDiffValue(value: unknown, masked: boolean): string {
  if (masked || looksMasked(value)) {
    return "••••••";
  }
  if (value === null || value === undefined || value === "") {
    return "—";
  }
  if (typeof value === "object") {
    try {
      return JSON.stringify(value);
    } catch {
      return "[object]";
    }
  }
  return String(value);
}

function toggleSection(section: SyncSection) {
  if (selectedSections.value.includes(section)) {
    selectedSections.value = selectedSections.value.filter((item) => item !== section);
    return;
  }
  selectedSections.value = [...selectedSections.value, section];
}

async function loadPreview() {
  previewLoading.value = true;
  previewError.value = "";
  executeError.value = "";
  try {
    const res = await previewSyncSections(props.gameId, {});
    preview.value = res;
    selectedSections.value = res.sections.map((item) => item.section);
  } catch (err) {
    preview.value = null;
    previewError.value = err instanceof ApiError ? err.message : "加载同步预览失败";
  } finally {
    previewLoading.value = false;
  }
}

function formatValidationDetails(details: Array<Record<string, unknown>>): string {
  if (!details.length) {
    return "请检查依赖是否齐全。";
  }
  return details
    .map((item) => {
      const section = item.section ? `section=${item.section}` : "";
      const dep = item.missingDependency ? `missing=${item.missingDependency}` : "";
      const key = item.entityKey ? `key=${item.entityKey}` : "";
      return [section, dep, key].filter(Boolean).join(", ");
    })
    .filter(Boolean)
    .join("；");
}

async function handleTokenStale(err: ApiError, title: string) {
  const message = err.message || title;
  try {
    await ElMessageBox.confirm(message, title, {
      confirmButtonText: "重新预览",
      cancelButtonText: "关闭",
      type: "warning"
    });
    await loadPreview();
  } catch {
    // 用户取消时仅保留错误态，不再追加动作。
  }
}

async function execute() {
  if (!preview.value || selectedSections.value.length === 0) {
    return;
  }
  executeLoading.value = true;
  executeError.value = "";
  try {
    const result = await executeSyncSections(props.gameId, {
      selectedSections: selectedSections.value,
      baselineToken: preview.value.baselineToken,
      includeDeletes: includeDeletes.value,
      operatorNote: operatorNote.value.trim()
    });
    ElMessage.success("同步执行成功");
    emit("executed", result);
  } catch (err) {
    if (err instanceof ApiError && err.code === "SYNC_BASELINE_MISMATCH") {
      await handleTokenStale(err, "目标已变更，请重新预览");
      return;
    }
    if (err instanceof ApiError && err.code === "SYNC_TOKEN_CONSUMED") {
      await handleTokenStale(err, "预览凭证已使用，请重新预览");
      return;
    }
    if (err instanceof ApiError && err.code === "UNKNOWN_SECTION") {
      executeError.value = `section 非法：${formatValidationDetails(err.details as Array<Record<string, unknown>>)}`;
      return;
    }
    if (err instanceof ApiError && err.code === "VALIDATION_FAILED") {
      executeError.value = `依赖校验失败：${formatValidationDetails(err.details as Array<Record<string, unknown>>)}`;
      return;
    }
    executeError.value = err instanceof ApiError ? err.message : "执行同步失败";
  } finally {
    executeLoading.value = false;
  }
}

watch(
  () => props.open,
  (next) => {
    if (!next) {
      preview.value = null;
      previewError.value = "";
      executeError.value = "";
      includeDeletes.value = false;
      operatorNote.value = "";
      selectedSections.value = [];
      return;
    }
    void loadPreview();
  },
  { immediate: true }
);
</script>

<style scoped>
.sync-drawer {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sync-drawer__toolbar {
  display: flex;
  justify-content: space-between;
}

.sync-drawer__form {
  display: grid;
  grid-template-columns: 200px minmax(0, 1fr);
  gap: 12px;
}

.sync-section-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 62vh;
  overflow: auto;
  padding-right: 4px;
}

.sync-section-card {
  border: 1px solid var(--panel-border);
  border-radius: 10px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.sync-section-card__head {
  display: flex;
  justify-content: space-between;
  gap: 10px;
}

.sync-section-card__selector {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.sync-section-card__deps {
  margin: 0;
  color: var(--text-subtle);
  font-size: 12px;
}

.sync-change {
  border-left: 3px solid transparent;
  background: #f8fafc;
  border-radius: 8px;
  padding: 8px 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.sync-change--empty {
  border-left-color: var(--panel-border);
  color: var(--text-subtle);
}

.sync-change--add {
  border-left-color: #16a34a;
  background: #effcf3;
}

.sync-change--update {
  border-left-color: #f59e0b;
  background: #fff8eb;
}

.sync-change--delete {
  border-left-color: #ef4444;
  background: #fef2f2;
}

.sync-change--delete-muted {
  opacity: 0.72;
}

.sync-change__head {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.sync-change__values {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  font-size: 13px;
}

.sync-drawer__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
