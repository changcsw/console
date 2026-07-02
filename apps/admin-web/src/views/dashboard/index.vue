<template>
  <div class="page-shell dashboard-page">
    <div class="dashboard-toolbar">
      <EnvironmentBadge :environment="app.environment" />
      <el-radio-group v-model="range" size="small" @change="onRangeChange">
        <el-radio-button v-for="item in RANGE_OPTIONS" :key="item.value" :label="item.value">
          {{ item.label }}
        </el-radio-button>
      </el-radio-group>
      <span v-if="summary" class="dashboard-toolbar__time">更新时间：{{ formatTime(summary.generatedAt) }}</span>
    </div>

    <el-alert v-if="errorMessage" type="error" show-icon :closable="false" class="dashboard-error">
      <template #title>Dashboard 加载失败：{{ errorMessage }}</template>
      <template #default>
        <el-button size="small" @click="reloadSummary()">重试</el-button>
      </template>
    </el-alert>

    <div v-if="showInitialSkeleton" class="dashboard-grid">
      <PageCard v-for="item in 5" :key="item">
        <el-skeleton animated :rows="4" />
      </PageCard>
    </div>

    <div v-else class="dashboard-grid">
      <DashboardMetricCard
        v-if="summary?.fxReview?.permitted"
        title="汇率待审"
        :value="summary.fxReview.pendingReviewCount"
        :env-scoped="summary.fxReview.envScoped"
        :expandable="summary.fxReview.pendingReviewCount > 0 || hasTopItems(summary.fxReview.topItems)"
        :details-expanded="expandedKey === 'fxReview'"
        @navigate="goByLink(summary.fxReview.link)"
        @toggle-details="toggleDetails('fxReview')"
      >
        <template #details>
          <ul class="detail-list">
            <li v-for="item in summary.fxReview.topItems" :key="`run-${item.runId}`">
              {{ item.templateName }} · {{ formatTime(item.triggeredAt) }}
            </li>
          </ul>
        </template>
      </DashboardMetricCard>

      <DashboardMetricCard
        v-if="summary?.configIssues?.permitted"
        title="配置异常"
        :value="summary.configIssues.invalidTotal"
        :env-scoped="summary.configIssues.envScoped"
        :expandable="summary.configIssues.invalidTotal > 0 || hasTopItems(summary.configIssues.topItems)"
        :details-expanded="expandedKey === 'configIssues'"
        @navigate="goByLink(summary.configIssues.link)"
        @toggle-details="toggleDetails('configIssues')"
      >
        <template #secondary>
          <div class="source-summary">
            <el-button
              v-for="item in summary.configIssues.bySource"
              :key="item.source"
              class="source-summary__item"
              text
              size="small"
              @click="goByLink(summary.configIssues.link, { source: item.source })"
            >
              {{ formatConfigSource(item.source) }}：{{ item.invalidCount }}
            </el-button>
          </div>
        </template>
        <template #details>
          <ul class="detail-list">
            <li v-for="item in summary.configIssues.topItems" :key="`${item.source}-${item.gameId}-${item.target}`">
              {{ item.gameName || item.gameId }} · {{ formatConfigSource(item.source) }} · {{ item.lastCheckMessage || "配置异常" }}
            </li>
          </ul>
        </template>
      </DashboardMetricCard>

      <DashboardMetricCard
        v-if="summary?.recentSyncJobs?.permitted"
        title="最近同步"
        :value="summary.recentSyncJobs.total"
        :env-scoped="summary.recentSyncJobs.envScoped"
        :expandable="summary.recentSyncJobs.total > 0 || hasTopItems(summary.recentSyncJobs.topItems)"
        :details-expanded="expandedKey === 'recentSyncJobs'"
        @navigate="goByLink(summary.recentSyncJobs.link)"
        @toggle-details="toggleDetails('recentSyncJobs')"
      >
        <template #secondary>
          <div class="sync-summary">
            <span>成功 {{ summary.recentSyncJobs.byStatus.succeeded }}</span>
            <span>失败 {{ summary.recentSyncJobs.byStatus.failed }}</span>
            <span>预览 {{ summary.recentSyncJobs.byStatus.previewed }}</span>
            <span v-if="summary.recentSyncJobs.lastFailedAt">最近失败：{{ formatTime(summary.recentSyncJobs.lastFailedAt) }}</span>
          </div>
        </template>
        <template #details>
          <ul class="detail-list">
            <li v-for="item in summary.recentSyncJobs.topItems" :key="`job-${item.jobId}`">
              {{ item.gameName || item.gameId }} · {{ formatSyncStatus(item.status) }} · {{ formatTime(item.executedAt) }}
            </li>
          </ul>
        </template>
      </DashboardMetricCard>

      <DashboardMetricCard
        v-if="summary?.pendingSnapshots?.permitted"
        title="待发布快照"
        :value="summary.pendingSnapshots.draftCount"
        :env-scoped="summary.pendingSnapshots.envScoped"
        :expandable="summary.pendingSnapshots.draftCount > 0 || hasTopItems(summary.pendingSnapshots.topItems)"
        :details-expanded="expandedKey === 'pendingSnapshots'"
        @navigate="goByLink(summary.pendingSnapshots.link)"
        @toggle-details="toggleDetails('pendingSnapshots')"
      >
        <template #details>
          <ul class="detail-list">
            <li v-for="item in summary.pendingSnapshots.topItems" :key="`snapshot-${item.snapshotId}`">
              {{ item.gameName || item.gameId }} · {{ item.configVersion }} · {{ formatTime(item.generatedAt) }}
            </li>
          </ul>
        </template>
      </DashboardMetricCard>

      <DashboardMetricCard
        v-if="summary?.channelInstanceIssues?.permitted"
        title="渠道实例问题"
        :value="summary.channelInstanceIssues.hiddenCount + summary.channelInstanceIssues.incompatibleCount"
        :env-scoped="summary.channelInstanceIssues.envScoped"
        value-suffix="项"
        :expandable="
          summary.channelInstanceIssues.hiddenCount + summary.channelInstanceIssues.incompatibleCount > 0 ||
          hasTopItems(summary.channelInstanceIssues.topItems)
        "
        :details-expanded="expandedKey === 'channelInstanceIssues'"
        @navigate="goByLink(summary.channelInstanceIssues.link)"
        @toggle-details="toggleDetails('channelInstanceIssues')"
      >
        <template #secondary>
          <div class="sync-summary">
            <span>隐藏 {{ summary.channelInstanceIssues.hiddenCount }}</span>
            <span>不兼容 {{ summary.channelInstanceIssues.incompatibleCount }}</span>
          </div>
        </template>
        <template #details>
          <ul class="detail-list">
            <li v-for="item in summary.channelInstanceIssues.topItems" :key="`channel-${item.gameChannelId}-${item.issue}`">
              {{ item.gameName || item.gameId }} · {{ item.channelId }} · {{ item.marketCode }} · {{ formatIssue(item.issue) }}
            </li>
          </ul>
        </template>
      </DashboardMetricCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import { ApiError } from "@/api/http";
import {
  getSummary,
  type ConfigIssueSource,
  type DashboardRange,
  type DashboardSummary,
  type MetricLink,
  type SyncJobStatus,
  type ChannelIssueType
} from "@/api/modules/dashboard";
import { useAppStore } from "@/stores/app";
import { useDictionaryStore } from "@/stores/dictionary";
import DashboardMetricCard from "./components/DashboardMetricCard.vue";

const router = useRouter();
const app = useAppStore();
const dictionary = useDictionaryStore();

const RANGE_STORAGE_KEY = "dashboard.summary.range";
const RANGE_OPTIONS: Array<{ value: DashboardRange; label: string }> = [
  { value: "24h", label: "24h" },
  { value: "7d", label: "7d" },
  { value: "30d", label: "30d" },
  { value: "90d", label: "90d" }
];

const range = ref<DashboardRange>(readRangeFromStorage());
const summary = ref<DashboardSummary | null>(null);
const loading = ref(false);
const errorMessage = ref("");
const withTopItems = ref(false);
const expandedKey = ref<string | null>(null);

const showInitialSkeleton = computed(() => loading.value && !summary.value);

void reloadSummary();

async function reloadSummary() {
  loading.value = true;
  errorMessage.value = "";
  try {
    summary.value = await getSummary({
      range: range.value,
      withTopItems: withTopItems.value,
      topN: 5
    });
    app.setEnvironment(summary.value.environment);
  } catch (err) {
    const message = err instanceof ApiError ? err.message : "请求失败";
    errorMessage.value = message;
    if (!summary.value) {
      ElMessage.error(message);
    }
  } finally {
    loading.value = false;
  }
}

function onRangeChange(next: DashboardRange) {
  range.value = next;
  window.localStorage.setItem(RANGE_STORAGE_KEY, next);
  void reloadSummary();
}

function readRangeFromStorage(): DashboardRange {
  const raw = window.localStorage.getItem(RANGE_STORAGE_KEY);
  if (raw === "24h" || raw === "7d" || raw === "30d" || raw === "90d") {
    return raw;
  }
  return "7d";
}

function goByLink(link: MetricLink, extraQuery?: Record<string, string>) {
  const normalized: Record<string, string> = {};
  for (const [key, value] of Object.entries(link.query)) {
    if (value !== undefined && value !== null) {
      normalized[key] = String(value);
    }
  }
  void router.push({
    path: link.route,
    query: {
      ...normalized,
      ...extraQuery
    }
  });
}

function hasTopItems(items: unknown[]): boolean {
  return items.length > 0;
}

async function toggleDetails(nextKey: string) {
  if (expandedKey.value === nextKey) {
    expandedKey.value = null;
    return;
  }
  expandedKey.value = nextKey;
  if (!withTopItems.value) {
    withTopItems.value = true;
    await reloadSummary();
  }
}

function formatTime(value?: string | null): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function formatConfigSource(source: ConfigIssueSource): string {
  void dictionary.loaded;
  switch (source) {
    case "account_auth":
      return "自有账号认证";
    case "channel_login":
      return "渠道登录";
    case "channel_iap":
      return "渠道 IAP";
    case "package_iap_override":
      return "分包 IAP 覆盖";
    case "plugin_config":
      return "功能插件";
    case "package_plugin_override":
      return "分包插件覆盖";
    default:
      return source;
  }
}

function formatSyncStatus(status: SyncJobStatus): string {
  switch (status) {
    case "previewed":
      return "已预览";
    case "succeeded":
      return "已成功";
    case "failed":
      return "已失败";
    default:
      return status;
  }
}

function formatIssue(issue: ChannelIssueType): string {
  return issue === "hidden" ? "隐藏" : "不兼容";
}
</script>

<style scoped>
.dashboard-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.dashboard-toolbar {
  position: sticky;
  top: 0;
  z-index: 9;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid #e6ecf4;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.94);
  backdrop-filter: blur(4px);
}

.dashboard-toolbar__time {
  margin-left: auto;
  color: var(--text-subtle);
  font-size: 13px;
}

.dashboard-error {
  align-items: center;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 14px;
}

.source-summary {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
}

.source-summary__item {
  padding-left: 0;
}

.sync-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  color: var(--text-subtle);
  font-size: 13px;
}

.detail-list {
  margin: 0;
  padding-left: 18px;
  color: var(--text-subtle);
  font-size: 13px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

@media (max-width: 768px) {
  .dashboard-toolbar {
    flex-wrap: wrap;
  }

  .dashboard-toolbar__time {
    margin-left: 0;
  }
}
</style>
