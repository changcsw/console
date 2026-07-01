<template>
  <div class="sync-jobs-tab">
    <div class="sync-jobs-tab__toolbar">
      <el-space wrap>
        <el-select v-model="status" clearable class="status-select" placeholder="状态过滤" @change="reload(1)">
          <el-option label="previewed" value="previewed" />
          <el-option label="succeeded" value="succeeded" />
          <el-option label="failed" value="failed" />
        </el-select>
        <el-button @click="reload(page)">刷新</el-button>
      </el-space>
    </div>

    <el-alert v-if="loadError" type="error" :closable="false" :title="loadError" />

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="sync-jobs-tab__expand">
            <template v-if="row.status === 'failed'">
              <div class="expand-title">错误概要</div>
              <p class="expand-block">
                {{ row.errorSummary?.code || "INTERNAL" }}: {{ row.errorSummary?.message || "执行失败" }}
              </p>
              <pre v-if="row.errorSummary?.details?.length" class="expand-block">{{
                toPrettyJson(row.errorSummary.details)
              }}</pre>
            </template>

            <template v-if="row.status === 'succeeded'">
              <div class="expand-title">appliedSummary</div>
              <pre class="expand-block">{{ toPrettyJson(row.appliedSummary || {}) }}</pre>
            </template>
          </div>
        </template>
      </el-table-column>

      <el-table-column prop="syncJobId" label="任务ID" min-width="120" />
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="sections / include_deletes" min-width="220">
        <template #default="{ row }">
          <div>{{ row.selectedSections?.join(", ") || "-" }}</div>
          <div class="subtle">include_deletes: {{ row.includeDeletes ? "true" : "false" }}</div>
        </template>
      </el-table-column>
      <el-table-column label="操作者" min-width="140">
        <template #default="{ row }">
          {{ row.operatorName || row.operatorId }}
        </template>
      </el-table-column>
      <el-table-column prop="operatorNote" label="备注" min-width="180" show-overflow-tooltip />
      <el-table-column label="source-target hash" min-width="280">
        <template #default="{ row }">
          <div>{{ truncateHash(row.sourceHash) }}</div>
          <div class="subtle">→ {{ truncateHash(row.targetHashAfter || row.targetHashBefore) }}</div>
        </template>
      </el-table-column>
      <el-table-column label="executedAt" min-width="170">
        <template #default="{ row }">{{ row.executedAt ? formatTime(row.executedAt) : "—" }}</template>
      </el-table-column>
      <el-table-column label="createdAt" min-width="170">
        <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
      </el-table-column>
    </el-table>

    <div class="sync-jobs-tab__pager">
      <el-pagination
        background
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="reload"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { ApiError } from "@/api/http";
import { listSyncJobs, type SyncJobListItem, type SyncJobStatus } from "@/api/syncSections";

const props = defineProps<{ gameId: string }>();

const loading = ref(false);
const loadError = ref("");
const rows = ref<SyncJobListItem[]>([]);
const status = ref<SyncJobStatus | undefined>(undefined);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);

function toPrettyJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function statusTagType(value: SyncJobStatus): "info" | "success" | "warning" {
  if (value === "succeeded") {
    return "success";
  }
  if (value === "failed") {
    return "warning";
  }
  return "info";
}

function formatTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function truncateHash(hash: string): string {
  if (!hash) {
    return "—";
  }
  return hash.length > 18 ? `${hash.slice(0, 9)}…${hash.slice(-6)}` : hash;
}

async function reload(targetPage = page.value) {
  loading.value = true;
  loadError.value = "";
  try {
    const res = await listSyncJobs(props.gameId, {
      page: targetPage,
      pageSize: pageSize.value,
      status: status.value,
      sort: "-createdAt"
    });
    rows.value = res.items;
    page.value = res.page;
    pageSize.value = res.pageSize;
    total.value = res.total;
  } catch (err) {
    rows.value = [];
    total.value = 0;
    loadError.value = err instanceof ApiError ? err.message : "加载同步历史失败";
  } finally {
    loading.value = false;
  }
}

defineExpose({
  reload
});

watch(
  () => props.gameId,
  () => {
    void reload(1);
  }
);

onMounted(() => {
  void reload(1);
});
</script>

<style scoped>
.sync-jobs-tab {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sync-jobs-tab__toolbar {
  display: flex;
  justify-content: flex-end;
}

.status-select {
  width: 160px;
}

.sync-jobs-tab__pager {
  display: flex;
  justify-content: flex-end;
}

.sync-jobs-tab__expand {
  padding: 8px 12px;
}

.expand-title {
  font-weight: 600;
  margin-bottom: 6px;
}

.expand-block {
  margin: 0;
  color: var(--text-subtle);
  white-space: pre-wrap;
  word-break: break-word;
}

.subtle {
  color: var(--text-subtle);
  font-size: 12px;
}
</style>
