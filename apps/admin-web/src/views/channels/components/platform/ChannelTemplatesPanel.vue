<template>
  <div class="panel">
    <div class="panel__toolbar">
      <div class="panel__filters">
        <el-select
          v-model="selectedChannelId"
          placeholder="选择渠道"
          filterable
          class="filter-channel"
          :loading="channelsLoading"
          @change="reload"
        >
          <el-option
            v-for="c in channels"
            :key="c.channelId"
            :label="`${c.channelName}（${c.channelId}）`"
            :value="c.channelId"
          />
        </el-select>
        <el-select v-model="filterKind" class="filter-select" @change="reload">
          <el-option label="全部种类" value="" />
          <el-option label="渠道登录" value="login" />
          <el-option label="渠道 IAP" value="iap" />
        </el-select>
        <el-button :disabled="!selectedChannelId" @click="reload">查询</el-button>
      </div>
      <el-button
        v-perm="'channel_template.write'"
        type="primary"
        :disabled="!selectedChannelId"
        @click="openCreate"
      >
        新建模版版本
      </el-button>
    </div>

    <p class="panel__hint">
      模版由系统管理员维护，与具体游戏无关。运行时取「该渠道该类中 enabled 的最新版本」为生效版本；要改已发布版本的字段结构，建议新建一个版本号而非原地改写。
    </p>

    <div v-if="!selectedChannelId" class="empty-state">
      <p class="empty-state__title">请选择渠道</p>
      <p class="empty-state__hint">模版隶属于平台渠道，选择渠道后展示其登录 / IAP 模版版本。</p>
    </div>

    <template v-else>
      <el-table v-loading="loading" :data="rows" border>
        <el-table-column prop="templateVersion" label="版本" min-width="120" />
        <el-table-column label="种类" width="110">
          <template #default="{ row }">{{ templateKindLabel(row.kind) }}</template>
        </el-table-column>
        <el-table-column label="字段数" width="90">
          <template #default="{ row }">{{ row.formSchemaJson.length }}</template>
        </el-table-column>
        <el-table-column label="敏感字段" width="100">
          <template #default="{ row }">{{ row.secretFieldsJson.length }}</template>
        </el-table-column>
        <el-table-column label="文件字段" width="100">
          <template #default="{ row }">{{ row.fileFieldsJson.length }}</template>
        </el-table-column>
        <el-table-column label="启用" width="90">
          <template #default="{ row }">
            <PageStatusTag :tone="row.enabled ? 'success' : 'neutral'" :label="row.enabled ? '已启用' : '已停用'" />
          </template>
        </el-table-column>
        <el-table-column label="是否生效" width="100">
          <template #default="{ row }">
            <PageStatusTag :tone="row.effective ? 'success' : 'neutral'" :label="row.effective ? '生效中' : '未生效'" />
          </template>
        </el-table-column>
        <el-table-column label="更新时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" v-perm="'channel_template.write'" @click="openEdit(row)">编辑</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <span class="text-muted">该渠道暂无模版版本</span>
        </template>
      </el-table>
    </template>

    <el-drawer v-model="drawerVisible" :title="drawerTitle" size="760px">
      <el-form label-position="top">
        <el-form-item label="渠道">
          <el-input :model-value="selectedChannelLabel" disabled />
        </el-form-item>
        <el-form-item label="模版种类">
          <el-select v-model="form.kind" :disabled="editing" class="form-control">
            <el-option label="渠道登录" value="login" />
            <el-option label="渠道 IAP" value="iap" />
          </el-select>
          <p v-if="editing" class="panel__hint">创建后不可改。</p>
        </el-form-item>
        <el-form-item label="版本号">
          <el-input
            v-model="form.templateVersion"
            :disabled="editing"
            placeholder="字母/数字/点/横线/下划线，如 v1 或 2026.01"
          />
          <p v-if="editing" class="panel__hint">创建后不可改：要改版本请新建一个版本号。</p>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>

      <TemplateQuartetEditor :draft="draft" />

      <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>

      <template #footer>
        <el-button @click="drawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitForm">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { ApiError } from "@/api/http";
import {
  listPlatformChannels,
  listChannelTemplates,
  createChannelTemplate,
  updateChannelTemplate,
  type ChannelTemplate,
  type ChannelTemplateKind,
  type PlatformChannel
} from "@/api/modules/platformChannels";
import TemplateQuartetEditor from "./TemplateQuartetEditor.vue";
import { templateKindLabel } from "./labels";
import { draftFromTemplate, draftToPayload, emptyDraft, type TemplateDraft } from "./templateDraft";

const channels = ref<PlatformChannel[]>([]);
const channelsLoading = ref(false);
const selectedChannelId = ref("");
const filterKind = ref<ChannelTemplateKind | "">("");

const rows = ref<ChannelTemplate[]>([]);
const loading = ref(false);

const drawerVisible = ref(false);
const editing = ref(false);
const editingId = ref<number | null>(null);
const saving = ref(false);
const formError = ref("");

const form = reactive({
  kind: "login" as ChannelTemplateKind,
  templateVersion: "",
  enabled: true
});

const draft = ref<TemplateDraft>(emptyDraft());

const selectedChannelLabel = computed(() => {
  const found = channels.value.find((c) => c.channelId === selectedChannelId.value);
  return found ? `${found.channelName}（${found.channelId}）` : selectedChannelId.value;
});

const drawerTitle = computed(() =>
  editing.value ? `编辑模版 ${form.templateVersion}` : "新建模版版本"
);

function reportError(err: unknown, fallback: string, setInline?: (msg: string) => void) {
  if (err instanceof ApiError) {
    const msg = err.message || fallback;
    if (setInline && (err.code === "VALIDATION_FAILED" || err.code === "CONFLICT")) {
      setInline(msg);
    } else {
      ElMessage.error(msg);
    }
    return;
  }
  ElMessage.error(fallback);
}

async function loadChannels() {
  channelsLoading.value = true;
  try {
    const res = await listPlatformChannels({ page: 1, pageSize: 100 });
    channels.value = res.items;
    if (!selectedChannelId.value && res.items.length > 0) {
      selectedChannelId.value = res.items[0].channelId;
      await reload();
    }
  } catch (err) {
    reportError(err, "加载渠道列表失败");
  } finally {
    channelsLoading.value = false;
  }
}

async function reload() {
  if (!selectedChannelId.value) {
    rows.value = [];
    return;
  }
  loading.value = true;
  try {
    rows.value = await listChannelTemplates(selectedChannelId.value, filterKind.value || undefined);
  } catch (err) {
    reportError(err, "加载模版列表失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = false;
  editingId.value = null;
  formError.value = "";
  form.kind = filterKind.value || "login";
  form.templateVersion = "";
  form.enabled = true;
  draft.value = emptyDraft();
  drawerVisible.value = true;
}

function openEdit(row: ChannelTemplate) {
  editing.value = true;
  editingId.value = row.templateId;
  formError.value = "";
  form.kind = row.kind;
  form.templateVersion = row.templateVersion;
  form.enabled = row.enabled;
  draft.value = draftFromTemplate(row);
  drawerVisible.value = true;
}

async function submitForm() {
  formError.value = "";
  const result = draftToPayload(draft.value);
  if ("error" in result) {
    formError.value = result.error;
    return;
  }
  if (!editing.value && form.templateVersion.trim() === "") {
    formError.value = "版本号必填";
    return;
  }

  saving.value = true;
  try {
    if (editing.value && editingId.value != null) {
      await updateChannelTemplate(form.kind, editingId.value, {
        ...result.payload,
        enabled: form.enabled
      });
      ElMessage.success("已更新模版");
    } else {
      await createChannelTemplate(selectedChannelId.value, {
        kind: form.kind,
        templateVersion: form.templateVersion.trim(),
        enabled: form.enabled,
        ...result.payload
      });
      ElMessage.success("已创建模版版本");
    }
    drawerVisible.value = false;
    await reload();
  } catch (err) {
    reportError(err, "保存失败", (msg) => (formError.value = msg));
  } finally {
    saving.value = false;
  }
}

function formatTime(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

onMounted(() => {
  void loadChannels();
});
</script>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.panel__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.panel__filters {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}

.filter-channel {
  width: 260px;
}

.filter-select {
  width: 140px;
}

.form-control {
  width: 100%;
}

.panel__error {
  color: var(--danger);
  font-size: 13px;
  margin: 12px 0 0;
}

.panel__hint {
  color: var(--text-subtle);
  font-size: 12px;
  margin: 0;
}

.empty-state {
  padding: 40px 0;
  text-align: center;
}

.empty-state__title {
  margin: 0;
  font-weight: 600;
  font-size: 16px;
}

.empty-state__hint {
  margin: 8px 0 0;
  color: var(--text-subtle);
}

.text-muted {
  color: var(--text-subtle);
}
</style>
