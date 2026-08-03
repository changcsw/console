<template>
  <div class="panel">
    <div class="panel__toolbar">
      <div class="panel__filters">
        <el-select v-model="filterEnabled" class="filter-select" placeholder="启用状态" clearable @change="reload">
          <el-option label="已启用" value="true" />
          <el-option label="已停用" value="false" />
        </el-select>
        <el-button @click="reload">查询</el-button>
      </div>
      <el-button v-perm="'feature_plugin.write'" type="primary" @click="openCreate">新建分类</el-button>
    </div>

    <p class="panel__hint">
      插件分类是可维护的字典（如登录类 / 支付类 / 推送类 / 广告类），插件主数据按分类归组展示与筛选。
    </p>
    <p v-if="listError" class="panel__error" role="alert">{{ listError }}</p>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="categoryCode" label="分类编码" min-width="140" />
      <el-table-column prop="categoryName" label="分类名" min-width="120" />
      <el-table-column label="启用状态" min-width="88">
        <template #default="{ row }">
          <PageStatusTag
            :tone="row.enabled ? 'success' : 'neutral'"
            :label="row.enabled ? '已启用' : '已停用'"
          />
        </template>
      </el-table-column>
      <el-table-column prop="sort" label="排序" width="80" />
      <el-table-column prop="pluginCount" label="插件数" width="90" />
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" v-perm="'feature_plugin.write'" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" v-perm="'feature_plugin.write'" @click="removeCategory(row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <span class="text-muted">暂无插件分类</span>
      </template>
    </el-table>

    <el-drawer v-model="drawerVisible" :title="editing ? '编辑分类' : '新建分类'" size="480px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="分类编码" prop="categoryCode">
          <el-input
            v-model="form.categoryCode"
            :disabled="editing"
            placeholder="小写字母/数字/下划线，字母开头，如 login"
          />
          <p v-if="editing" class="panel__hint">创建后不可改：分类编码是分类的引用键。</p>
        </el-form-item>
        <el-form-item label="分类名" prop="categoryName">
          <el-input v-model="form.categoryName" placeholder="1-64 字符，如 登录类" />
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
import {
  listFeaturePluginCategories,
  createFeaturePluginCategory,
  updateFeaturePluginCategory,
  deleteFeaturePluginCategory,
  type FeaturePluginCategory
} from "@/api/modules/featurePlugins";

const rows = ref<FeaturePluginCategory[]>([]);
const loading = ref(false);
const filterEnabled = ref<"true" | "false" | "">("");
const listError = ref("");

const drawerVisible = ref(false);
const editing = ref(false);
const editingId = ref<number | null>(null);
const saving = ref(false);
const formError = ref("");
const formRef = ref<FormInstance>();

const form = reactive({
  categoryCode: "",
  categoryName: "",
  sort: 0,
  enabled: true
});

// 与后端 ValidateCategoryCode / ValidateFeaturePluginCategory 同口径，文案对齐后端 message。
// 编辑态 categoryCode disabled 不可改，回填值本身合法，rules 照常通过。
// 注意：pattern 与 max 必须拆成两条 rule——async-validator 对带 RegExp pattern 的规则
// 会把 type 推断为 'pattern'，该校验器只跑正则，同条的 max 会被静默跳过。
const rules: FormRules = {
  categoryCode: [
    { required: true, message: "请输入分类编码", trigger: "blur" },
    {
      pattern: /^[a-z][a-z0-9_]*$/,
      message: "分类编码只能用小写字母/数字/下划线，且以字母开头，长度不超过 64",
      trigger: "blur"
    },
    {
      max: 64,
      message: "分类编码只能用小写字母/数字/下划线，且以字母开头，长度不超过 64",
      trigger: "blur"
    }
  ],
  categoryName: [
    { required: true, whitespace: true, message: "分类名称必填且不超过 64 字符", trigger: "blur" },
    { max: 64, message: "分类名称必填且不超过 64 字符", trigger: "blur" }
  ],
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

async function reload() {
  loading.value = true;
  try {
    rows.value = await listFeaturePluginCategories({
      enabled: filterEnabled.value === "" ? undefined : filterEnabled.value === "true"
    });
  } catch (err) {
    reportError(err, "加载插件分类列表失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = false;
  editingId.value = null;
  formError.value = "";
  Object.assign(form, {
    categoryCode: "",
    categoryName: "",
    sort: 0,
    enabled: true
  });
  formRef.value?.clearValidate();
  drawerVisible.value = true;
}

function openEdit(row: FeaturePluginCategory) {
  editing.value = true;
  editingId.value = row.id;
  formError.value = "";
  Object.assign(form, {
    categoryCode: row.categoryCode,
    categoryName: row.categoryName,
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
    if (editing.value && editingId.value != null) {
      // categoryCode 不在补丁内：创建后不可改。
      await updateFeaturePluginCategory(editingId.value, {
        categoryName: form.categoryName,
        enabled: form.enabled,
        sort: form.sort
      });
      ElMessage.success("已更新分类");
    } else {
      await createFeaturePluginCategory({
        categoryCode: form.categoryCode,
        categoryName: form.categoryName,
        enabled: form.enabled,
        sort: form.sort
      });
      ElMessage.success("已创建分类");
    }
    drawerVisible.value = false;
    await reload();
  } catch (err) {
    reportError(err, "保存失败", (msg) => (formError.value = msg));
  } finally {
    saving.value = false;
  }
}

async function removeCategory(row: FeaturePluginCategory) {
  // 按钮文案与插件删除确认弹窗保持一致：项目未配中文 locale，默认按钮是英文 OK/Cancel
  try {
    await ElMessageBox.confirm(`确认删除分类「${row.categoryName}」？删除后不可恢复。`, "删除分类", {
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
    await deleteFeaturePluginCategory(row.id);
    ElMessage.success("已删除分类");
    await reload();
  } catch (err) {
    // 分类下仍有插件时后端返回 409：行内提示，不用 toast，便于对照当前行。
    if (err instanceof ApiError && err.code === "CONFLICT") {
      listError.value = err.message || "该分类下仍有插件，无法删除";
      return;
    }
    reportError(err, "删除分类失败");
  }
}

onMounted(() => {
  void reload();
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

.filter-select {
  width: 140px;
}

.panel__error {
  color: var(--danger);
  font-size: 13px;
  margin: 0;
}

.panel__hint {
  color: var(--text-subtle);
  font-size: 12px;
  margin: 0;
}

.el-form .panel__hint {
  margin: 4px 0 0;
}

.text-muted {
  color: var(--text-subtle);
}
</style>
