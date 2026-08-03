<template>
  <div class="panel">
    <div class="panel__toolbar">
      <div class="panel__filters">
        <el-input
          v-model="keyword"
          placeholder="插件 ID / 插件名"
          clearable
          class="filter-keyword"
          @keyup.enter="reload(1)"
          @clear="reload(1)"
        />
        <el-select v-model="filterCategoryId" class="filter-select" placeholder="插件分类" clearable @change="reload(1)">
          <el-option v-for="c in categories" :key="c.id" :label="c.categoryName" :value="c.id" />
        </el-select>
        <el-select v-model="filterRegion" class="filter-select" placeholder="国内/海外" clearable @change="reload(1)">
          <el-option v-for="r in PLUGIN_REGION_OPTIONS" :key="r" :label="pluginRegionLabel(r)" :value="r" />
        </el-select>
        <el-select v-model="filterEnabled" class="filter-select" placeholder="启用状态" clearable @change="reload(1)">
          <el-option label="已启用" value="true" />
          <el-option label="已停用" value="false" />
        </el-select>
        <el-button @click="reload(1)">查询</el-button>
      </div>
      <el-button v-perm="'feature_plugin.write'" type="primary" @click="openCreate">新建插件</el-button>
    </div>

    <p v-if="listError" class="panel__error" role="alert">{{ listError }}</p>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="pluginId" label="插件 ID" min-width="140" />
      <el-table-column prop="pluginName" label="插件名" min-width="120" />
      <el-table-column label="分类" min-width="100">
        <template #default="{ row }">
          <span v-if="row.categoryName">{{ row.categoryName }}</span>
          <span v-else class="text-muted">未分类</span>
        </template>
      </el-table-column>
      <el-table-column label="国内/海外" min-width="96">
        <template #default="{ row }">{{ pluginRegionLabel(row.region) }}</template>
      </el-table-column>
      <el-table-column label="启用状态" min-width="88">
        <template #default="{ row }">
          <PageStatusTag
            :tone="row.enabled ? 'success' : 'neutral'"
            :label="row.enabled ? '已启用' : '已停用'"
          />
        </template>
      </el-table-column>
      <el-table-column prop="templateCount" label="模板数" width="90" />
      <el-table-column label="更新时间" min-width="170">
        <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="184" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" v-perm="'feature_plugin.write'" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" v-perm="'feature_plugin.read'" @click="emit('view-templates', row.pluginId)">
            参数模板
          </el-button>
          <el-button link type="danger" v-perm="'feature_plugin.write'" @click="removePlugin(row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <span class="text-muted">暂无功能插件</span>
      </template>
    </el-table>

    <div class="panel__pager">
      <el-pagination
        background
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="reload"
      />
    </div>

    <el-drawer v-model="drawerVisible" :title="editing ? '编辑插件' : '新建插件'" size="520px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="插件 ID" prop="pluginId">
          <el-input
            v-model="form.pluginId"
            :disabled="editing"
            placeholder="小写字母/数字/下划线，字母开头，如 huawei_push"
          />
          <p v-if="editing" class="panel__hint">创建后不可改：插件 ID 是插件实例配置的引用键。</p>
        </el-form-item>
        <el-form-item label="插件名" prop="pluginName">
          <el-input v-model="form.pluginName" placeholder="1-64 字符，如 华为推送" />
        </el-form-item>
        <el-form-item label="插件分类">
          <el-select v-model="form.categoryId" class="form-control" clearable placeholder="未分类">
            <el-option v-for="c in categories" :key="c.id" :label="c.categoryName" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="国内/海外" prop="region">
          <el-select v-model="form.region" :disabled="editing" class="form-control">
            <el-option v-for="r in PLUGIN_REGION_OPTIONS" :key="r" :label="pluginRegionLabel(r)" :value="r" />
          </el-select>
          <p v-if="editing" class="panel__hint">
            创建后不可改：国内/海外属性决定与市场（market）的兼容性，改动会让既有插件配置集体失配。
          </p>
        </el-form-item>
        <el-form-item label="排序" prop="sort">
          <el-input-number v-model="form.sort" :min="0" :max="9999" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>
      </el-form>
      <template #footer>
        <el-button @click="drawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitForm">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { ApiError } from "@/api/http";
import type { PluginRegion } from "@/api/modules/channels";
import {
  listFeaturePlugins,
  createFeaturePlugin,
  updateFeaturePlugin,
  deleteFeaturePlugin,
  listFeaturePluginCategories,
  PLUGIN_REGION_OPTIONS,
  type FeaturePlugin,
  type FeaturePluginCategory
} from "@/api/modules/featurePlugins";
import { pluginRegionLabel } from "./labels";

const emit = defineEmits<{
  (e: "view-templates", pluginId: string): void;
  /** 插件删除（含级联删模板版本）成功后发出：父级据此失效参数模板页签的本地缓存 */
  (e: "plugin-deleted", pluginId: string): void;
}>();

const rows = ref<FeaturePlugin[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);
const listError = ref("");

const categories = ref<FeaturePluginCategory[]>([]);

const keyword = ref("");
const filterCategoryId = ref<number | "">("");
const filterRegion = ref<PluginRegion | "">("");
const filterEnabled = ref<"true" | "false" | "">("");

const drawerVisible = ref(false);
const editing = ref(false);
const saving = ref(false);
const formError = ref("");
const formRef = ref<FormInstance>();

const form = reactive({
  pluginId: "",
  pluginName: "",
  categoryId: "" as number | "",
  region: "domestic" as PluginRegion,
  sort: 0,
  enabled: true
});

// 与后端 ValidatePluginID / ValidateFeaturePluginMaster 同口径，文案对齐后端 message。
// 编辑态 pluginId / region disabled 不可改，回填值本身合法，rules 照常通过。
const rules: FormRules = {
  pluginId: [
    { required: true, message: "请输入插件 ID", trigger: "blur" },
    {
      pattern: /^[a-z][a-z0-9_]*$/,
      max: 64,
      message: "插件 ID 只能用小写字母/数字/下划线，且以字母开头，长度不超过 64",
      trigger: "blur"
    }
  ],
  pluginName: [
    { required: true, whitespace: true, message: "插件名称必填且不超过 64 字符", trigger: "blur" },
    { max: 64, message: "插件名称必填且不超过 64 字符", trigger: "blur" }
  ],
  region: [{ required: true, message: "请选择国内/海外", trigger: "change" }],
  sort: [{ type: "integer", min: 0, max: 9999, message: "排序值需在 0-9999 之间", trigger: "change" }]
};

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

async function loadCategories() {
  try {
    categories.value = await listFeaturePluginCategories();
  } catch (err) {
    reportError(err, "加载插件分类失败");
  }
}

async function reload(targetPage = page.value) {
  loading.value = true;
  try {
    const res = await listFeaturePlugins({
      page: targetPage,
      pageSize: pageSize.value,
      keyword: keyword.value || undefined,
      categoryId: filterCategoryId.value === "" ? undefined : filterCategoryId.value,
      region: filterRegion.value || undefined,
      enabled: filterEnabled.value === "" ? undefined : filterEnabled.value === "true"
    });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    reportError(err, "加载功能插件列表失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = false;
  formError.value = "";
  Object.assign(form, {
    pluginId: "",
    pluginName: "",
    categoryId: "" as number | "",
    region: "domestic" as PluginRegion,
    sort: 0,
    enabled: true
  });
  formRef.value?.clearValidate();
  drawerVisible.value = true;
}

function openEdit(row: FeaturePlugin) {
  editing.value = true;
  formError.value = "";
  Object.assign(form, {
    pluginId: row.pluginId,
    pluginName: row.pluginName,
    categoryId: row.categoryId ?? "",
    region: row.region,
    sort: row.sort,
    enabled: row.enabled
  });
  formRef.value?.clearValidate();
  drawerVisible.value = true;
}

async function submitForm() {
  formError.value = "";
  // 先过前端即时校验：不过则不置 saving、不发请求
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  saving.value = true;
  try {
    if (editing.value) {
      // pluginId / region 不在补丁内：创建后不可改。
      // el-select clearable 清空时 v-model 置为 undefined（EP 默认 valueOnClear），与 "" 一样视为「取消归属」，
      // 必须显式下发 null；若原样透传 undefined，JSON 序列化会丢键，后端按「不修改」处理导致清空静默失效。
      await updateFeaturePlugin(form.pluginId, {
        pluginName: form.pluginName,
        categoryId: form.categoryId === "" || form.categoryId == null ? null : form.categoryId,
        enabled: form.enabled,
        sort: form.sort
      });
      ElMessage.success("已更新插件");
    } else {
      await createFeaturePlugin({
        pluginId: form.pluginId,
        pluginName: form.pluginName,
        categoryId: form.categoryId === "" ? undefined : form.categoryId,
        region: form.region,
        enabled: form.enabled,
        sort: form.sort
      });
      ElMessage.success("已创建插件");
    }
    drawerVisible.value = false;
    await reload();
  } catch (err) {
    reportError(err, "保存失败", (msg) => (formError.value = msg));
  } finally {
    saving.value = false;
  }
}

async function removePlugin(row: FeaturePlugin) {
  // 后端删除插件会级联删除其全部参数模板版本，确认文案须说清级联影响
  const message =
    row.templateCount > 0
      ? `确认删除插件「${row.pluginName}」？将同时删除其 ${row.templateCount} 个参数模板版本，删除后不可恢复。`
      : `确认删除插件「${row.pluginName}」？删除后不可恢复。`;
  try {
    await ElMessageBox.confirm(message, "删除插件", {
      type: "warning",
      confirmButtonText: "确认删除",
      cancelButtonText: "取消",
      confirmButtonClass: "el-button--danger"
    });
  } catch {
    return;
  }
  listError.value = "";
  try {
    await deleteFeaturePlugin(row.pluginId);
    ElMessage.success("已删除插件");
    emit("plugin-deleted", row.pluginId);
    await reload();
  } catch (err) {
    // 插件被渠道绑定/渠道实例配置/渠道包覆盖引用时后端返回 409：行内提示，不用 toast，与分类页一致
    if (err instanceof ApiError && err.code === "CONFLICT") {
      listError.value = err.message || "该插件仍被渠道引用，无法删除";
      return;
    }
    reportError(err, "删除插件失败");
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
  void loadCategories();
  void reload(1);
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

.filter-keyword {
  width: 200px;
}

.filter-select {
  width: 140px;
}

.form-control {
  width: 100%;
}

.panel__pager {
  display: flex;
  justify-content: flex-end;
}

.panel__error {
  color: var(--danger);
  font-size: 13px;
  margin: 4px 0 0;
}

.panel__hint {
  color: var(--text-subtle);
  font-size: 12px;
  margin: 4px 0 0;
}

.text-muted {
  color: var(--text-subtle);
}
</style>
