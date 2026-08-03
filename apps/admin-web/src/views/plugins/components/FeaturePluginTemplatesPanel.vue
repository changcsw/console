<template>
  <div class="panel">
    <div class="panel__toolbar">
      <div class="panel__filters">
        <el-select
          v-model="selectedPluginId"
          placeholder="选择插件"
          filterable
          class="filter-plugin"
          :loading="pluginsLoading"
          @change="reload"
        >
          <el-option
            v-for="p in plugins"
            :key="p.pluginId"
            :label="`${p.pluginName}（${p.pluginId}）`"
            :value="p.pluginId"
          />
        </el-select>
        <el-button :disabled="!selectedPluginId" @click="reload">查询</el-button>
      </div>
      <el-button
        v-perm="'feature_plugin.write'"
        type="primary"
        :disabled="!selectedPluginId"
        @click="openCreate"
      >
        新建模板版本
      </el-button>
    </div>

    <p class="panel__hint">
      参数模板由系统管理员维护，与具体游戏/渠道无关。运行时取「该插件中 enabled 的最新版本」为生效版本；要改已发布版本的字段结构，建议新建一个版本号而非原地改写。
    </p>

    <div v-if="!selectedPluginId" class="empty-state">
      <p class="empty-state__title">请选择插件</p>
      <p class="empty-state__hint">参数模板隶属于功能插件，选择插件后展示其模板版本。</p>
    </div>

    <template v-else>
      <el-table v-loading="loading" :data="rows" border>
        <el-table-column prop="templateVersion" label="版本" min-width="120" />
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
            <el-button link type="primary" v-perm="'feature_plugin.write'" @click="openEdit(row)">编辑</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <span class="text-muted">该插件暂无模板版本</span>
        </template>
      </el-table>
    </template>

    <el-drawer v-model="drawerVisible" :title="drawerTitle" size="760px">
      <el-form label-position="top">
        <el-form-item label="插件">
          <el-input :model-value="selectedPluginLabel" disabled />
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
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { ApiError } from "@/api/http";
import {
  listFeaturePlugins,
  listFeaturePluginTemplates,
  createFeaturePluginTemplate,
  updateFeaturePluginTemplate,
  type FeaturePlugin,
  type FeaturePluginTemplate
} from "@/api/modules/featurePlugins";
import TemplateQuartetEditor from "@/views/channels/components/platform/TemplateQuartetEditor.vue";
import { draftFromTemplate, draftToPayload, emptyDraft, type PluginTemplateDraft } from "./pluginTemplateDraft";

const props = defineProps<{
  /** 插件主数据页签「参数模板」操作带入的目标插件；新对象引用即视为一次新的定位请求 */
  focusPlugin?: { pluginId: string };
}>();

const plugins = ref<FeaturePlugin[]>([]);
const pluginsLoading = ref(false);
const selectedPluginId = ref("");

const rows = ref<FeaturePluginTemplate[]>([]);
const loading = ref(false);

const drawerVisible = ref(false);
const editing = ref(false);
const editingId = ref<number | null>(null);
const saving = ref(false);
const formError = ref("");

const form = reactive({
  templateVersion: "",
  enabled: true
});

const draft = ref<PluginTemplateDraft>(emptyDraft());

const selectedPluginLabel = computed(() => {
  const found = plugins.value.find((p) => p.pluginId === selectedPluginId.value);
  return found ? `${found.pluginName}（${found.pluginId}）` : selectedPluginId.value;
});

const drawerTitle = computed(() =>
  editing.value ? `编辑模板 ${form.templateVersion}` : "新建模板版本"
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

async function loadPlugins() {
  pluginsLoading.value = true;
  try {
    const res = await listFeaturePlugins({ page: 1, pageSize: 100 });
    plugins.value = res.items;
    const preset = props.focusPlugin?.pluginId;
    if (preset && res.items.some((p) => p.pluginId === preset)) {
      selectedPluginId.value = preset;
      await reload();
    } else if (!selectedPluginId.value && res.items.length > 0) {
      selectedPluginId.value = res.items[0].pluginId;
      await reload();
    }
  } catch (err) {
    reportError(err, "加载插件列表失败");
  } finally {
    pluginsLoading.value = false;
  }
}

async function reload() {
  if (!selectedPluginId.value) {
    rows.value = [];
    return;
  }
  loading.value = true;
  try {
    rows.value = await listFeaturePluginTemplates(selectedPluginId.value);
  } catch (err) {
    reportError(err, "加载模板列表失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = false;
  editingId.value = null;
  formError.value = "";
  form.templateVersion = "";
  form.enabled = true;
  draft.value = emptyDraft();
  drawerVisible.value = true;
}

function openEdit(row: FeaturePluginTemplate) {
  editing.value = true;
  editingId.value = row.templateId;
  formError.value = "";
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
      await updateFeaturePluginTemplate(editingId.value, {
        ...result.payload,
        enabled: form.enabled
      });
      ElMessage.success("已更新模板");
    } else {
      await createFeaturePluginTemplate(selectedPluginId.value, {
        templateVersion: form.templateVersion.trim(),
        enabled: form.enabled,
        ...result.payload
      });
      ElMessage.success("已创建模板版本");
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

// 页签 lazy 挂载后保活：从插件主数据页签点「参数模板」时父级会换一个新的 focusPlugin 对象，
// 这里按引用变化把选中插件切过去并重拉模板列表。
watch(
  () => props.focusPlugin,
  (focus) => {
    if (!focus?.pluginId || !plugins.value.some((p) => p.pluginId === focus.pluginId)) {
      return;
    }
    selectedPluginId.value = focus.pluginId;
    void reload();
  }
);

onMounted(() => {
  void loadPlugins();
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

.filter-plugin {
  width: 260px;
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
