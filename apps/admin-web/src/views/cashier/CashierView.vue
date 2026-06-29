<template>
  <div class="page-shell">
    <PageCard title="收银台" description="模板列表、版本生命周期、价格矩阵与 FX 审核集中在同一工作台。">
      <div class="toolbar">
        <div class="toolbar__left">
          <el-input
            v-model.trim="keyword"
            clearable
            class="keyword"
            placeholder="模板 ID / 模板名称"
            @keyup.enter="loadTemplates(1)"
            @clear="loadTemplates(1)"
          />
          <el-button @click="loadTemplates(1)">查询</el-button>
        </div>
        <div class="toolbar__right">
          <EnvironmentBadge :environment="app.environment" />
          <el-button v-perm="'cashier.write'" type="primary" @click="createTemplateOpen = true">新建模板</el-button>
        </div>
      </div>

      <el-table
        v-loading="templatesLoading"
        :data="templates"
        border
        highlight-current-row
        :current-row-key="selectedTemplateId"
        row-key="templateId"
        @current-change="onTemplateSelected"
      >
        <el-table-column prop="templateId" label="模板 ID" min-width="150" />
        <el-table-column prop="templateName" label="模板名称" min-width="180" />
        <el-table-column label="FX 模式" width="140">
          <template #default="{ row }">{{ row.fxSyncMode }}</template>
        </el-table-column>
        <el-table-column label="周期" width="120">
          <template #default="{ row }">{{ row.fxSyncSchedule }}</template>
        </el-table-column>
        <el-table-column label="同步开关" width="100">
          <template #default="{ row }">{{ row.fxSyncEnabled ? "开" : "关" }}</template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <template #empty>
          <div class="empty-state">
            <p class="empty-state__title">暂无模板</p>
            <p class="empty-state__hint">点击右上角「新建模板」创建第一条记录。</p>
          </div>
        </template>
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="prev, pager, next, total"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="loadTemplates"
        />
      </div>
    </PageCard>

    <PageCard v-loading="detailLoading" title="模板详情" :description="selectedTemplateId ? selectedTemplateId : '请选择模板查看详情'">
      <template v-if="templateDetail">
        <el-descriptions :column="4" border size="small" class="detail-desc">
          <el-descriptions-item label="模板 ID">{{ templateDetail.templateId }}</el-descriptions-item>
          <el-descriptions-item label="模板名称">{{ templateDetail.templateName }}</el-descriptions-item>
          <el-descriptions-item label="FX 模式">{{ templateDetail.fxSyncMode }}</el-descriptions-item>
          <el-descriptions-item label="FX 周期">{{ templateDetail.fxSyncSchedule }}</el-descriptions-item>
        </el-descriptions>

        <TemplateVersionsTab
          :versions="templateDetail.versions"
          @create-version="createVersionOpen = true"
          @copy-version="openCopyDialog"
          @publish-version="publishVersion"
          @select-version="selectVersion"
        />

        <PriceMatrixEditor
          :template-id="templateDetail.templateId"
          :version="selectedVersion"
          :version-status="selectedVersionStatus"
          @saved="refreshTemplateDetail"
        />

        <FxSyncRunsReviewList
          :template-id="templateDetail.templateId"
          :runs="templateDetail.fxSyncRuns ?? []"
          :loading="detailLoading"
          @refresh="refreshTemplateDetail"
        />
      </template>
      <template v-else>
        <div class="empty-state">
          <p class="empty-state__title">未选择模板</p>
          <p class="empty-state__hint">先在上方列表选择模板，再进行版本、价格矩阵与 FX 审核操作。</p>
        </div>
      </template>
    </PageCard>

    <CreateTemplateDialog :open="createTemplateOpen" @close="createTemplateOpen = false" @created="onTemplateCreated" />
    <CreateVersionDialog
      v-if="selectedTemplateId"
      :open="createVersionOpen"
      :template-id="selectedTemplateId"
      @close="createVersionOpen = false"
      @created="onVersionCreated"
    />
    <CopyPublishedToDraftDialog
      v-if="selectedTemplateId && copySourceVersion"
      :open="copyDialogOpen"
      :template-id="selectedTemplateId"
      :source-version="copySourceVersion"
      @close="copyDialogOpen = false"
      @created="onVersionCreated"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import { useAppStore } from "@/stores/app";
import { ApiError } from "@/api/http";
import {
  getCashierTemplate,
  listCashierTemplates,
  publishCashierVersion,
  type CashierTemplateDetail,
  type CashierTemplateSummary,
  type CashierTemplateVersion,
  type VersionStatus
} from "@/api/modules/cashier";
import TemplateVersionsTab from "./templates/TemplateVersionsTab.vue";
import PriceMatrixEditor from "./templates/PriceMatrixEditor.vue";
import FxSyncRunsReviewList from "./templates/FxSyncRunsReviewList.vue";
import CopyPublishedToDraftDialog from "./templates/components/CopyPublishedToDraftDialog.vue";
import CreateTemplateDialog from "./templates/components/CreateTemplateDialog.vue";
import CreateVersionDialog from "./templates/components/CreateVersionDialog.vue";

const app = useAppStore();

const templates = ref<CashierTemplateSummary[]>([]);
const templatesLoading = ref(false);
const keyword = ref("");
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);

const selectedTemplateId = ref("");
const templateDetail = ref<CashierTemplateDetail | null>(null);
const detailLoading = ref(false);
const selectedVersion = ref("");

const createTemplateOpen = ref(false);
const createVersionOpen = ref(false);
const copyDialogOpen = ref(false);
const copySourceVersion = ref<CashierTemplateVersion | null>(null);

const selectedVersionStatus = computed<VersionStatus>(() => {
  const selected = templateDetail.value?.versions.find((item) => item.version === selectedVersion.value);
  return selected?.status ?? "draft";
});

void loadTemplates(1);

async function loadTemplates(targetPage = page.value) {
  templatesLoading.value = true;
  try {
    const res = await listCashierTemplates({
      page: targetPage,
      pageSize: pageSize.value,
      keyword: keyword.value || undefined
    });
    templates.value = res.items;
    page.value = res.page;
    pageSize.value = res.pageSize;
    total.value = res.total;

    if (!selectedTemplateId.value && res.items[0]) {
      await selectTemplate(res.items[0].templateId);
    } else if (selectedTemplateId.value && !res.items.some((item) => item.templateId === selectedTemplateId.value)) {
      templateDetail.value = null;
      selectedTemplateId.value = "";
      selectedVersion.value = "";
    }
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载模板列表失败");
  } finally {
    templatesLoading.value = false;
  }
}

async function selectTemplate(templateId: string) {
  selectedTemplateId.value = templateId;
  await refreshTemplateDetail();
}

async function refreshTemplateDetail() {
  if (!selectedTemplateId.value) {
    return;
  }
  detailLoading.value = true;
  try {
    const detail = await getCashierTemplate(selectedTemplateId.value);
    templateDetail.value = detail;
    if (!selectedVersion.value || !detail.versions.some((item) => item.version === selectedVersion.value)) {
      const preferred = detail.versions.find((item) => item.status === "draft") ?? detail.versions[0];
      selectedVersion.value = preferred?.version ?? "";
    }
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载模板详情失败");
    templateDetail.value = null;
  } finally {
    detailLoading.value = false;
  }
}

function onTemplateSelected(row: CashierTemplateSummary | null) {
  if (row) {
    void selectTemplate(row.templateId);
  }
}

function selectVersion(version: string) {
  selectedVersion.value = version;
}

function openCopyDialog(version: CashierTemplateVersion) {
  copySourceVersion.value = version;
  copyDialogOpen.value = true;
}

async function publishVersion(version: CashierTemplateVersion) {
  if (!selectedTemplateId.value) {
    return;
  }
  try {
    await publishCashierVersion(selectedTemplateId.value, version.version);
    ElMessage.success(`版本 v${version.version} 已发布`);
    await refreshTemplateDetail();
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "发布版本失败");
  }
}

async function onTemplateCreated() {
  await loadTemplates(1);
}

async function onVersionCreated() {
  copySourceVersion.value = null;
  copyDialogOpen.value = false;
  createVersionOpen.value = false;
  await refreshTemplateDetail();
}
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.toolbar__left,
.toolbar__right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.keyword {
  width: 260px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.empty-state {
  padding: 20px 0;
  text-align: center;
}

.empty-state__title {
  margin: 0;
  font-weight: 600;
}

.empty-state__hint {
  margin: 8px 0 0;
  color: var(--text-subtle);
}

.detail-desc {
  margin-bottom: 16px;
}
</style>
