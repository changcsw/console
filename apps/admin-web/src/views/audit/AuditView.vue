<template>
  <div class="page-shell">
    <PageCard title="审计日志" description="只读追踪环境变更、同步执行与关键配置变更。">
      <div class="audit-header">
        <h3>审计日志列表</h3>
        <EnvironmentBadge :environment="app.environment" />
      </div>

      <div v-if="pageState !== 'forbidden'" class="filter-bar">
        <el-select v-model="draftFilters.env" clearable filterable placeholder="环境" class="filter-item">
          <el-option v-for="item in envOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-select
          v-model="draftFilters.action"
          clearable
          filterable
          placeholder="动作"
          class="filter-item filter-item--wide"
        >
          <el-option v-for="item in actionOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-select
          v-model="draftFilters.resourceType"
          clearable
          filterable
          placeholder="资源类型"
          class="filter-item filter-item--wide"
        >
          <el-option v-for="item in resourceTypeOptions" :key="item" :label="item" :value="item" />
        </el-select>
        <el-select
          v-model="draftFilters.operator"
          clearable
          filterable
          remote
          reserve-keyword
          placeholder="操作者"
          class="filter-item"
          :remote-method="searchOperators"
          :loading="operatorSearching"
        >
          <el-option
            v-for="item in operatorOptions"
            :key="item.id"
            :label="`${item.displayName || item.userName} (${item.userName})`"
            :value="String(item.id)"
          />
        </el-select>
        <el-date-picker
          v-model="draftTimeRange"
          type="datetimerange"
          start-placeholder="起始时间"
          end-placeholder="结束时间"
          class="filter-item filter-item--range"
          value-format=""
          :clearable="true"
        />
        <el-input
          v-model="draftFilters.keyword"
          clearable
          placeholder="关键词（资源标识/摘要）"
          class="filter-item filter-item--wide"
          @keyup.enter="submitFilters"
        />
        <el-button type="primary" @click="submitFilters">查询</el-button>
        <el-button @click="resetFilters">重置</el-button>
      </div>

      <el-result
        v-if="pageState === 'forbidden'"
        icon="warning"
        title="无权限访问审计日志"
        sub-title="当前账号缺少 audit.read 权限。"
      />

      <el-result
        v-else-if="pageState === 'error'"
        icon="error"
        title="审计日志加载失败"
        :sub-title="errorMessage || '请稍后重试'"
      >
        <template #extra>
          <el-button type="primary" @click="reload(page)">重试</el-button>
        </template>
      </el-result>

      <template v-else>
        <el-skeleton v-if="showSkeleton" animated :rows="6" />

        <template v-else>
          <el-table
            :data="rows"
            border
            :default-sort="{ prop: 'createdAt', order: 'descending' }"
            @sort-change="onSortChange"
            @row-click="openDetail"
          >
            <el-table-column prop="createdAt" label="时间" min-width="190" sortable="custom">
              <template #default="{ row }">
                <el-tooltip :content="formatUtc(row.createdAt)" placement="top">
                  <span>{{ formatLocal(row.createdAt) }}</span>
                </el-tooltip>
              </template>
            </el-table-column>
            <el-table-column label="操作者" min-width="180" show-overflow-tooltip>
              <template #default="{ row }">
                {{ formatOperator(row) }}
              </template>
            </el-table-column>
            <el-table-column label="动作" min-width="150">
              <template #default="{ row }">
                <el-tag :type="actionTagType(row.action)" effect="light">{{ row.action }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="resourceType" label="资源类型" min-width="140" show-overflow-tooltip />
            <el-table-column label="资源标识" min-width="200" show-overflow-tooltip>
              <template #default="{ row }">
                <div class="resource-id">
                  <span>{{ row.resourceId }}</span>
                  <el-button link type="primary" @click.stop="copyText(row.resourceId)">复制</el-button>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="环境" width="130">
              <template #default="{ row }">
                <el-tag :type="envTagType(row.env)" effect="light">{{ row.env }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="摘要" min-width="240" show-overflow-tooltip>
              <template #default="{ row }">
                {{ row.detail?.summary || "—" }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click.stop="openDetail(row)">详情</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="当前条件下无审计记录">
                <el-button @click="resetFilters">重置过滤</el-button>
              </el-empty>
            </template>
          </el-table>

          <div v-if="total > pageSize" class="pager">
            <el-pagination
              background
              layout="prev, pager, next, total"
              :total="total"
              :page-size="pageSize"
              :current-page="page"
              @current-change="reload"
            />
          </div>
        </template>
      </template>
    </PageCard>

    <el-drawer v-model="drawerVisible" title="审计详情" size="62%">
      <div v-loading="detailLoading">
      <template v-if="detailRecord">
        <el-descriptions :column="2" border class="detail-section">
          <el-descriptions-item label="审计ID">{{ detailRecord.id }}</el-descriptions-item>
          <el-descriptions-item label="时间">{{ formatLocal(detailRecord.createdAt) }}</el-descriptions-item>
          <el-descriptions-item label="操作者">{{ formatOperator(detailRecord) }}</el-descriptions-item>
          <el-descriptions-item label="动作">{{ detailRecord.action }}</el-descriptions-item>
          <el-descriptions-item label="资源类型">{{ detailRecord.resourceType }}</el-descriptions-item>
          <el-descriptions-item label="资源标识">{{ detailRecord.resourceId }}</el-descriptions-item>
          <el-descriptions-item label="环境">{{ detailRecord.env }}</el-descriptions-item>
          <el-descriptions-item label="摘要">{{ detailRecord.detail?.summary || "—" }}</el-descriptions-item>
        </el-descriptions>

        <div class="detail-section">
          <div class="compare-header">
            <h4>before / after 对照</h4>
            <el-switch
              v-if="hasBefore && hasAfter"
              v-model="showUnchanged"
              inline-prompt
              active-text="显示未变字段"
              inactive-text="仅看变更"
            />
          </div>

          <el-table v-if="hasBefore && hasAfter" :data="displayCompareRows" border>
            <el-table-column prop="key" label="字段" min-width="180" />
            <el-table-column label="Before" min-width="240">
              <template #default="{ row }">
                <span :class="{ unchanged: !row.changed }">{{ row.beforeText }}</span>
              </template>
            </el-table-column>
            <el-table-column label="After" min-width="240">
              <template #default="{ row }">
                <span :class="{ unchanged: !row.changed, changed: row.changed }">{{ row.afterText }}</span>
              </template>
            </el-table-column>
          </el-table>

          <el-table v-else-if="hasAfter" :data="singleAfterRows" border>
            <el-table-column prop="key" label="字段" min-width="180" />
            <el-table-column prop="valueText" label="After" min-width="320" />
          </el-table>

          <el-table v-else-if="hasBefore" :data="singleBeforeRows" border>
            <el-table-column prop="key" label="字段" min-width="180" />
            <el-table-column prop="valueText" label="Before" min-width="320" />
          </el-table>

          <el-empty v-else description="无 before / after 字段" />
        </div>

        <div class="detail-section">
          <el-collapse>
            <el-collapse-item title="extra" name="extra">
              <pre class="json-block">{{ toPrettyJson(detailRecord.detail?.extra) }}</pre>
            </el-collapse-item>
            <el-collapse-item title="request 元信息" name="request">
              <el-descriptions :column="2" border>
                <el-descriptions-item label="IP">{{ detailRecord.detail?.request?.ip || "—" }}</el-descriptions-item>
                <el-descriptions-item label="Request ID">
                  {{ detailRecord.detail?.request?.requestId || "—" }}
                </el-descriptions-item>
                <el-descriptions-item label="Method">
                  {{ detailRecord.detail?.request?.method || "—" }}
                </el-descriptions-item>
                <el-descriptions-item label="Path">{{ detailRecord.detail?.request?.path || "—" }}</el-descriptions-item>
              </el-descriptions>
            </el-collapse-item>
            <el-collapse-item title="查看原始 JSON" name="raw">
              <pre class="json-block">{{ toPrettyJson(detailRecord.detail) }}</pre>
            </el-collapse-item>
          </el-collapse>
        </div>
      </template>

      <el-empty v-else description="未找到详情数据" />
      <div v-if="detailError" class="detail-error">{{ detailError }}</div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import type { TagProps } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import { ApiError } from "@/api/http";
import { listAdminUsers, type AdminUserListItem } from "@/api/modules/system";
import {
  getAuditLogDetail,
  listAuditLogFacets,
  listAuditLogs,
  type AuditEnvironment,
  type AuditLogItem
} from "@/api/modules/audit";
import { useAppStore } from "@/stores/app";
import { usePermissionStore } from "@/stores/permission";

interface AuditFilters {
  env: string;
  action: string;
  resourceType: string;
  operator: string;
  from: string;
  to: string;
  keyword: string;
}

interface CompareRow {
  key: string;
  beforeText: string;
  afterText: string;
  changed: boolean;
}

interface SortChangeEvent {
  prop: string;
  order: "ascending" | "descending" | null;
}

const KNOWN_ENVS = ["develop", "sandbox", "production"];
const KNOWN_ACTIONS = [
  "admin_user.create",
  "admin_user.update",
  "admin_user.disable",
  "role.create",
  "role.update",
  "role.delete",
  "role.assign_permission",
  "user.assign_role",
  "game.create",
  "game.update",
  "game.enable",
  "game.disable",
  "game.delete",
  "game_channel.create",
  "game_channel.update",
  "game_channel.hide",
  "game_channel.unhide",
  "product.create",
  "product.update",
  "product.delete",
  "cashier_price_template_version.publish",
  "fx.approve",
  "sync.execute"
];
const KNOWN_RESOURCE_TYPES = [
  "admin_user",
  "role",
  "game",
  "game_channel",
  "product",
  "cashier_price_template_version",
  "cashier_fx_sync_run",
  "sync_job"
];

const app = useAppStore();
const permission = usePermissionStore();

const rows = ref<AuditLogItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const sort = ref<"createdAt" | "-createdAt">("-createdAt");
const loading = ref(false);
const pageState = ref<"ready" | "error" | "forbidden">("ready");
const errorMessage = ref("");

const draftFilters = reactive<AuditFilters>({
  env: "",
  action: "",
  resourceType: "",
  operator: "",
  from: "",
  to: "",
  keyword: ""
});
const appliedFilters = reactive<AuditFilters>({
  env: "",
  action: "",
  resourceType: "",
  operator: "",
  from: "",
  to: "",
  keyword: ""
});
const draftTimeRange = ref<[Date, Date] | []>([]);

const envOptions = ref<string[]>([...KNOWN_ENVS]);
const actionOptions = ref<string[]>([...KNOWN_ACTIONS]);
const resourceTypeOptions = ref<string[]>([...KNOWN_RESOURCE_TYPES]);
const operatorOptions = ref<AdminUserListItem[]>([]);
const operatorSearching = ref(false);

const drawerVisible = ref(false);
const detailRecord = ref<AuditLogItem | null>(null);
const detailLoading = ref(false);
const detailError = ref("");
const showUnchanged = ref(false);

const showSkeleton = computed(() => loading.value && rows.value.length === 0 && page.value === 1);

const beforePayload = computed<Record<string, unknown>>(() => {
  const value = detailRecord.value?.detail?.before;
  return value && typeof value === "object" ? value : {};
});
const afterPayload = computed<Record<string, unknown>>(() => {
  const value = detailRecord.value?.detail?.after;
  return value && typeof value === "object" ? value : {};
});
const changedSet = computed(() => new Set(detailRecord.value?.detail?.changed ?? []));
const hasBefore = computed(() => Object.keys(beforePayload.value).length > 0);
const hasAfter = computed(() => Object.keys(afterPayload.value).length > 0);

const compareRows = computed<CompareRow[]>(() => {
  const keys = Array.from(new Set([...Object.keys(beforePayload.value), ...Object.keys(afterPayload.value)]));
  return keys.map((key) => {
    const beforeValue = beforePayload.value[key];
    const afterValue = afterPayload.value[key];
    const changed = changedSet.value.has(key) || serializeValue(beforeValue) !== serializeValue(afterValue);
    return {
      key,
      beforeText: formatDetailValue(key, beforeValue),
      afterText: formatDetailValue(key, afterValue),
      changed
    };
  });
});

const displayCompareRows = computed<CompareRow[]>(() => {
  if (showUnchanged.value) {
    return compareRows.value;
  }
  return compareRows.value.filter((item) => item.changed);
});

const singleBeforeRows = computed(() =>
  Object.entries(beforePayload.value).map(([key, value]) => ({
    key,
    valueText: formatDetailValue(key, value)
  }))
);
const singleAfterRows = computed(() =>
  Object.entries(afterPayload.value).map(([key, value]) => ({
    key,
    valueText: formatDetailValue(key, value)
  }))
);

function mergeUnique(base: string[], extra: string[]): string[] {
  return Array.from(new Set([...base, ...extra.filter(Boolean)]));
}

function buildRequestFilters(): AuditFilters {
  return {
    env: draftFilters.env,
    action: draftFilters.action,
    resourceType: draftFilters.resourceType,
    operator: draftFilters.operator,
    from: draftTimeRange.value.length === 2 ? draftTimeRange.value[0].toISOString() : "",
    to: draftTimeRange.value.length === 2 ? draftTimeRange.value[1].toISOString() : "",
    keyword: draftFilters.keyword.trim()
  };
}

function applyFilters(next: AuditFilters) {
  appliedFilters.env = next.env;
  appliedFilters.action = next.action;
  appliedFilters.resourceType = next.resourceType;
  appliedFilters.operator = next.operator;
  appliedFilters.from = next.from;
  appliedFilters.to = next.to;
  appliedFilters.keyword = next.keyword;
}

async function loadFacets() {
  try {
    const facets = await listAuditLogFacets();
    envOptions.value = mergeUnique(KNOWN_ENVS, facets.envs ?? []);
    actionOptions.value = mergeUnique(KNOWN_ACTIONS, facets.actions ?? []);
    resourceTypeOptions.value = mergeUnique(KNOWN_RESOURCE_TYPES, facets.resourceTypes ?? []);
  } catch {
    envOptions.value = [...KNOWN_ENVS];
    actionOptions.value = [...KNOWN_ACTIONS];
    resourceTypeOptions.value = [...KNOWN_RESOURCE_TYPES];
  }
}

async function searchOperators(keyword: string) {
  operatorSearching.value = true;
  try {
    const res = await listAdminUsers({
      page: 1,
      pageSize: 30,
      keyword: keyword || undefined
    });
    operatorOptions.value = res.items;
  } catch {
    operatorOptions.value = [];
  } finally {
    operatorSearching.value = false;
  }
}

async function reload(targetPage = page.value) {
  loading.value = true;
  errorMessage.value = "";
  pageState.value = "ready";
  try {
    const res = await listAuditLogs({
      env: appliedFilters.env || undefined,
      action: appliedFilters.action || undefined,
      resourceType: appliedFilters.resourceType || undefined,
      operator: appliedFilters.operator || undefined,
      from: appliedFilters.from || undefined,
      to: appliedFilters.to || undefined,
      keyword: appliedFilters.keyword || undefined,
      page: targetPage,
      pageSize: pageSize.value,
      sort: sort.value
    });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
    pageState.value = "ready";
  } catch (err) {
    if (err instanceof ApiError) {
      errorMessage.value = err.message || "加载失败";
      if (err.status === 403 || err.code === "FORBIDDEN" || !permission.hasPerm("audit.read")) {
        pageState.value = "forbidden";
      } else {
        pageState.value = "error";
      }
    } else {
      errorMessage.value = "加载失败";
      pageState.value = "error";
    }
  } finally {
    loading.value = false;
  }
}

function submitFilters() {
  applyFilters(buildRequestFilters());
  void reload(1);
}

function resetFilters() {
  draftFilters.env = "";
  draftFilters.action = "";
  draftFilters.resourceType = "";
  draftFilters.operator = "";
  draftFilters.keyword = "";
  draftTimeRange.value = [];
  submitFilters();
}

function onSortChange(payload: SortChangeEvent) {
  if (payload.prop !== "createdAt") {
    return;
  }
  sort.value = payload.order === "ascending" ? "createdAt" : "-createdAt";
  void reload(1);
}

function formatLocal(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function formatUtc(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toISOString();
}

function formatOperator(item: AuditLogItem): string {
  if (item.actorId === "0") {
    return "System";
  }
  if (item.operator?.displayName) {
    return item.operator.displayName;
  }
  return item.actorId || "—";
}

function parseActionVerb(action: string): string {
  const parts = action.split(".");
  return parts[parts.length - 1] || action;
}

function actionTagType(action: string): TagProps["type"] {
  const verb = parseActionVerb(action);
  if (verb === "create") return "success";
  if (verb === "delete") return "danger";
  if (verb === "publish") return "primary";
  if (verb === "execute") return "warning";
  if (verb === "hide") return "info";
  return "info";
}

function envTagType(env: AuditEnvironment): TagProps["type"] {
  if (env === "production") return "danger";
  if (env === "sandbox") return "warning";
  return "success";
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text);
    ElMessage.success("已复制");
  } catch {
    ElMessage.warning("复制失败，请手动复制");
  }
}

function toPrettyJson(value: unknown): string {
  if (value === undefined) {
    return "—";
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function serializeValue(value: unknown): string {
  if (value === undefined) return "undefined";
  if (value === null) return "null";
  if (typeof value === "object") {
    try {
      return JSON.stringify(value);
    } catch {
      return String(value);
    }
  }
  return String(value);
}

function formatDetailValue(_key: string, value: unknown): string {
  if (value === undefined) {
    return "—";
  }
  if (typeof value === "string") {
    const lower = value.toLowerCase();
    if (lower === "masked" || lower === "******") {
      return "******";
    }
  }
  if (value === null) {
    return "null";
  }
  if (typeof value === "object") {
    return toPrettyJson(value);
  }
  return String(value);
}

async function openDetail(row: AuditLogItem) {
  drawerVisible.value = true;
  detailLoading.value = true;
  detailError.value = "";
  showUnchanged.value = false;
  detailRecord.value = row;
  try {
    detailRecord.value = await getAuditLogDetail(row.id);
  } catch (err) {
    detailRecord.value = row;
    if (err instanceof ApiError && err.status !== 404) {
      detailError.value = err.message || "详情加载失败，已回退展示列表快照";
    }
  } finally {
    detailLoading.value = false;
  }
}

onMounted(() => {
  void Promise.all([loadFacets(), searchOperators("")]).finally(() => {
    submitFilters();
  });
});
</script>

<style scoped>
.audit-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
}

.audit-header h3 {
  margin: 0;
  font-size: 18px;
}

.filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 16px;
}

.filter-item {
  width: 140px;
}

.filter-item--wide {
  width: 180px;
}

.filter-item--range {
  width: 340px;
}

.resource-id {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.detail-section {
  margin-bottom: 18px;
}

.compare-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.compare-header h4 {
  margin: 0;
}

.changed {
  color: #0f766e;
  font-weight: 600;
}

.unchanged {
  color: #94a3b8;
}

.json-block {
  margin: 0;
  padding: 12px;
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 8px;
  max-height: 280px;
  overflow: auto;
}

.detail-error {
  margin-top: 12px;
  color: #b91c1c;
}
</style>

