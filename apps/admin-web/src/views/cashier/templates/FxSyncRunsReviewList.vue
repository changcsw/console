<template>
  <PageCard title="汇率同步审核" description="默认 manual_confirm，需要人工审核差异并选择 approve/ignore。">
    <div class="toolbar">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="manual_confirm 模式下，approve 会在后端同事务完成 apply。"
      />
      <el-button v-perm="'cashier.write'" type="primary" :loading="triggering" @click="triggerRun">触发 FX 同步</el-button>
    </div>

    <el-table :data="runs" border size="small" v-loading="loading">
      <el-table-column prop="runId" label="Run ID" width="100" />
      <el-table-column prop="candidateVersion" label="候选版本" width="120">
        <template #default="{ row }">v{{ row.candidateVersion }}</template>
      </el-table-column>
      <el-table-column label="状态" width="130">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="差异摘要" min-width="280">
        <template #default="{ row }">
          <pre class="diff">{{ formatDiff(row.diffSummary) }}</pre>
        </template>
      </el-table-column>
      <el-table-column label="触发时间" width="180">
        <template #default="{ row }">{{ formatTime(row.triggeredAt) }}</template>
      </el-table-column>
      <el-table-column label="审核信息" min-width="180">
        <template #default="{ row }">
          <div class="review">
            <span>reviewedBy: {{ row.reviewedBy ?? "—" }}</span>
            <span>reviewedAt: {{ formatTime(row.reviewedAt) }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button
            v-perm="'fx.approve'"
            link
            type="success"
            :disabled="!canReview(row.status) || approvingRunId === row.runId"
            @click="review(row.runId, 'approve')"
          >
            approve
          </el-button>
          <el-button
            v-perm="'fx.approve'"
            link
            type="warning"
            :disabled="!canReview(row.status) || approvingRunId === row.runId"
            @click="review(row.runId, 'ignore')"
          >
            ignore
          </el-button>
        </template>
      </el-table-column>
      <template #empty>
        <span class="text-muted">暂无 FX run，点击右上角触发同步。</span>
      </template>
    </el-table>
  </PageCard>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import { ApiError } from "@/api/http";
import { approveFxSyncRun, triggerFxSyncRun, type FxRunStatus, type FxSyncRun } from "@/api/modules/cashier";

const props = defineProps<{
  templateId: string;
  runs: FxSyncRun[];
  loading?: boolean;
}>();

const emit = defineEmits<{
  (e: "refresh"): void;
}>();

const triggering = ref(false);
const approvingRunId = ref<number | null>(null);

function canReview(status: FxRunStatus): boolean {
  return status === "pending_review";
}

function statusType(status: FxRunStatus): "info" | "success" | "warning" | "danger" {
  if (status === "applied") {
    return "success";
  }
  if (status === "approved") {
    return "warning";
  }
  if (status === "ignored" || status === "failed") {
    return "danger";
  }
  return "info";
}

function formatDiff(value: Record<string, unknown>): string {
  try {
    return JSON.stringify(value ?? {}, null, 2);
  } catch {
    return "{}";
  }
}

function formatTime(value?: string | null): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

async function triggerRun() {
  if (!props.templateId) {
    return;
  }
  triggering.value = true;
  try {
    await triggerFxSyncRun(props.templateId);
    ElMessage.success("已触发 FX 同步，候选版本待审核");
    emit("refresh");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "触发 FX 同步失败");
  } finally {
    triggering.value = false;
  }
}

async function review(runId: number, action: "approve" | "ignore") {
  approvingRunId.value = runId;
  try {
    await approveFxSyncRun(runId, {
      action,
      reviewNote: action === "ignore" ? "ignored by reviewer" : undefined
    });
    ElMessage.success(action === "approve" ? "审核通过，已应用候选版本" : "已忽略该运行批次");
    emit("refresh");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "审核操作失败");
  } finally {
    approvingRunId.value = null;
  }
}
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.diff {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  color: #475467;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.review {
  display: grid;
  gap: 4px;
  color: var(--text-subtle);
  font-size: 12px;
}

.text-muted {
  color: var(--text-subtle);
}
</style>
