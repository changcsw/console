<template>
  <div class="snapshot-tab">
    <div class="snapshot-tab__toolbar">
      <el-space wrap>
        <el-button @click="reload(1)">刷新</el-button>
        <el-button
          v-perm="'snapshot.generate'"
          type="primary"
          :disabled="!canGenerate || loading || generating"
          :loading="generating"
          @click="generateSnapshot"
        >
          生成快照
        </el-button>
      </el-space>
    </div>

    <el-alert
      v-if="!canGenerate || !canPublish"
      type="info"
      :closable="false"
      :title="readonlyHint"
      class="snapshot-tab__alert"
    />

    <el-result
      v-if="pageState === 'forbidden'"
      icon="warning"
      title="无权限查看配置快照"
      sub-title="当前账号缺少 game.read 权限。"
    />

    <el-result v-else-if="pageState === 'error'" icon="error" title="配置快照加载失败" :sub-title="errorMessage || '请稍后重试'">
      <template #extra>
        <el-button type="primary" @click="reload(page)">重试</el-button>
      </template>
    </el-result>

    <template v-else>
      <el-table :data="rows" border v-loading="loading">
        <el-table-column prop="configVersion" label="version" min-width="220" show-overflow-tooltip />
        <el-table-column label="status" width="120">
          <template #default="{ row }">
            <el-tag :type="row.status === 'published' ? 'success' : 'warning'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="fileHash" label="hash" min-width="260" show-overflow-tooltip />
        <el-table-column label="生成时间" min-width="180">
          <template #default="{ row }">{{ formatTime(row.generatedAt) }}</template>
        </el-table-column>
        <el-table-column label="发布时间" min-width="180">
          <template #default="{ row }">{{ row.publishedAt ? formatTime(row.publishedAt) : "—" }}</template>
        </el-table-column>
        <el-table-column label="操作" min-width="280" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :loading="previewLoading && previewSnapshotId === row.id" @click="previewSnapshot(row)">
              预览 JSON
            </el-button>
            <el-button link type="primary" :loading="downloadingId === row.id" @click="downloadSnapshot(row.id)">下载</el-button>
            <el-button
              v-perm="'snapshot.publish'"
              link
              type="primary"
              :disabled="!canPublish || row.status !== 'draft' || publishingId === row.id"
              :loading="publishingId === row.id"
              @click="publishSnapshot(row.id)"
            >
              发布
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无配置快照">
            <el-button
              v-perm="'snapshot.generate'"
              type="primary"
              :disabled="!canGenerate || generating"
              :loading="generating"
              @click="generateSnapshot"
            >
              生成首个快照
            </el-button>
          </el-empty>
        </template>
      </el-table>

      <div v-if="total > pageSize" class="snapshot-tab__pager">
        <el-pagination
          background
          layout="prev, pager, next, total"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="reload"
        />
      </div>

      <PageCard
        v-if="previewData"
        title="JSON 预览"
        :description="`version: ${previewVersion || '-'}，按 market 分区折叠展示（密文统一脱敏为 ***）`"
      >
        <el-alert v-if="previewError" type="error" :closable="false" :title="previewError" class="snapshot-tab__alert" />
        <el-collapse v-model="activeMarkets">
          <el-collapse-item v-for="entry in marketEntries" :key="entry.market" :name="entry.market" :title="entry.market">
            <pre class="snapshot-tab__json">{{ toPrettyJson(entry.content) }}</pre>
          </el-collapse-item>
        </el-collapse>
      </PageCard>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import { ApiError } from "@/api/http";
import {
  downloadGameSnapshot,
  generateGameSnapshot,
  listGameSnapshots,
  publishGameSnapshot,
  type SnapshotListItem,
  type SnapshotPreviewPayload
} from "@/api/modules/snapshot";
import { usePermissionStore } from "@/stores/permission";

const props = defineProps<{ gameId: string }>();

const permission = usePermissionStore();
const canRead = computed(() => permission.hasPerm("game.read"));
const canGenerate = computed(() => permission.hasPerm("snapshot.generate"));
const canPublish = computed(() => permission.hasPerm("snapshot.publish"));
const readonlyHint = computed(() => {
  if (!canGenerate.value && !canPublish.value) {
    return "当前账号仅有查看权限，生成/发布入口已置灰。";
  }
  if (!canGenerate.value) {
    return "当前账号缺少 snapshot.generate 权限，生成入口已置灰。";
  }
  return "当前账号缺少 snapshot.publish 权限，发布入口已置灰。";
});

const rows = ref<SnapshotListItem[]>([]);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const loading = ref(false);
const generating = ref(false);
const publishingId = ref<number | null>(null);
const downloadingId = ref<number | null>(null);
const pageState = ref<"ready" | "forbidden" | "error">("ready");
const errorMessage = ref("");

const previewLoading = ref(false);
const previewSnapshotId = ref<number | null>(null);
const previewVersion = ref("");
const previewData = ref<SnapshotPreviewPayload | null>(null);
const previewError = ref("");
const activeMarkets = ref<string[]>([]);

const marketEntries = computed(() => {
  const markets = previewData.value?.markets;
  if (!markets || typeof markets !== "object") {
    return [];
  }
  return Object.entries(markets).map(([market, content]) => ({
    market,
    content: maskSecretValues(content)
  }));
});

function formatTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function normalizeErrorMessage(err: unknown, fallback: string): string {
  if (!(err instanceof ApiError)) {
    return fallback;
  }
  switch (err.code) {
    case "NOT_FOUND":
      return "资源不存在，请刷新后重试。";
    case "VALIDATION_FAILED":
      return "请求参数校验失败，请检查后重试。";
    case "VERSION_STATE_INVALID":
      return "当前快照状态不允许该操作。";
    case "CONFLICT":
      return "资源状态冲突，请刷新后重试。";
    default:
      return err.message || fallback;
  }
}

function toPrettyJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function looksLikeSecretKey(key: string): boolean {
  return /(secret|password|token|cipher|private|access[-_]?key|app[-_]?key)/i.test(key);
}

function maskSecretValues(value: unknown, parentKey = ""): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => maskSecretValues(item, parentKey));
  }
  if (value && typeof value === "object") {
    const result: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
      result[key] = maskSecretValues(item, key);
    }
    return result;
  }
  if (typeof value === "string") {
    const lowered = value.toLowerCase();
    if (looksLikeSecretKey(parentKey) || lowered === "masked" || lowered === "******") {
      return "***";
    }
    return value;
  }
  return value;
}

async function reload(targetPage = page.value) {
  if (!canRead.value) {
    pageState.value = "forbidden";
    rows.value = [];
    total.value = 0;
    return;
  }
  loading.value = true;
  errorMessage.value = "";
  pageState.value = "ready";
  try {
    const res = await listGameSnapshots(props.gameId, { page: targetPage, pageSize: pageSize.value });
    rows.value = res.items;
    page.value = res.page;
    pageSize.value = res.pageSize;
    total.value = res.total;
    pageState.value = "ready";
  } catch (err) {
    if (err instanceof ApiError && (err.status === 403 || err.code === "FORBIDDEN")) {
      pageState.value = "forbidden";
      rows.value = [];
      total.value = 0;
    } else {
      pageState.value = "error";
      errorMessage.value = normalizeErrorMessage(err, "加载配置快照失败");
    }
  } finally {
    loading.value = false;
  }
}

async function generateSnapshot() {
  generating.value = true;
  try {
    const created = await generateGameSnapshot(props.gameId);
    ElMessage.success(`快照生成成功：${created.configVersion}`);
    await reload(1);
  } catch (err) {
    ElMessage.error(normalizeErrorMessage(err, "生成快照失败"));
  } finally {
    generating.value = false;
  }
}

async function publishSnapshot(snapshotId: number) {
  try {
    await ElMessageBox.confirm("发布后该快照将进入 published 状态，确认继续？", "发布快照", {
      type: "warning",
      confirmButtonText: "确认发布",
      cancelButtonText: "取消"
    });
  } catch {
    return;
  }

  publishingId.value = snapshotId;
  try {
    await publishGameSnapshot(snapshotId);
    ElMessage.success("快照发布成功");
    await reload(page.value);
  } catch (err) {
    ElMessage.error(normalizeErrorMessage(err, "发布快照失败"));
  } finally {
    publishingId.value = null;
  }
}

function triggerDownload(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = fileName;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  URL.revokeObjectURL(url);
}

async function downloadSnapshot(snapshotId: number) {
  downloadingId.value = snapshotId;
  try {
    const res = await downloadGameSnapshot(snapshotId);
    triggerDownload(res.blob, res.fileName);
    ElMessage.success("快照下载已开始");
  } catch (err) {
    ElMessage.error(normalizeErrorMessage(err, "下载快照失败"));
  } finally {
    downloadingId.value = null;
  }
}

async function previewSnapshot(row: SnapshotListItem) {
  previewLoading.value = true;
  previewSnapshotId.value = row.id;
  previewError.value = "";
  try {
    const res = await downloadGameSnapshot(row.id);
    if (!res.payload) {
      previewData.value = null;
      previewVersion.value = "";
      previewError.value = "下载内容不是合法 JSON，无法预览。";
      return;
    }
    previewData.value = maskSecretValues(res.payload) as SnapshotPreviewPayload;
    previewVersion.value = row.configVersion;
    activeMarkets.value = Object.keys((res.payload.markets ?? {}) as Record<string, unknown>);
    ElMessage.success("已加载 JSON 预览");
  } catch (err) {
    previewError.value = normalizeErrorMessage(err, "加载 JSON 预览失败");
  } finally {
    previewLoading.value = false;
  }
}

onMounted(() => {
  void reload(1);
});
</script>

<style scoped>
.snapshot-tab {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.snapshot-tab__toolbar {
  display: flex;
  justify-content: flex-end;
}

.snapshot-tab__alert {
  margin-bottom: 2px;
}

.snapshot-tab__pager {
  display: flex;
  justify-content: flex-end;
}

.snapshot-tab__json {
  margin: 0;
  padding: 12px;
  border-radius: 8px;
  background: #0f172a;
  color: #e2e8f0;
  max-height: 420px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
