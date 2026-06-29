<template>
  <div class="panel">
    <div class="panel__toolbar">
      <div class="panel__filters">
        <el-input
          v-model="keyword"
          placeholder="权限码 / 名称"
          clearable
          class="filter-keyword"
          @keyup.enter="reload(1)"
          @clear="reload(1)"
        />
        <el-button @click="reload(1)">查询</el-button>
      </div>
      <el-button v-perm="'permission.write'" type="primary" @click="openCreate">新建权限码</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="permissionCode" label="权限码" min-width="200" />
      <el-table-column prop="permissionName" label="名称" min-width="180" />
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="danger" v-perm="'permission.write'" @click="removePermission(row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <span class="text-muted">暂无权限码</span>
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

    <el-dialog v-model="dialogVisible" title="新建权限码" width="460px">
      <el-form label-position="top">
        <el-form-item label="权限码">
          <el-input v-model="form.permissionCode" placeholder="格式 resource.action，如 game.read" />
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="form.permissionName" placeholder="1-128 字符" />
        </el-form-item>
        <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitForm">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { ApiError } from "@/api/http";
import {
  listPermissions,
  createPermission,
  deletePermission,
  type PermissionItem
} from "@/api/modules/system";

const PERMISSION_CODE_PATTERN = /^[a-z0-9_]+\.[a-z0-9_]+$/;

const rows = ref<PermissionItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);
const keyword = ref("");

const dialogVisible = ref(false);
const saving = ref(false);
const formError = ref("");
const form = reactive({ permissionCode: "", permissionName: "" });

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

async function reload(targetPage = page.value) {
  loading.value = true;
  try {
    const res = await listPermissions({ page: targetPage, pageSize: pageSize.value, keyword: keyword.value || undefined });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    reportError(err, "加载权限码失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  form.permissionCode = "";
  form.permissionName = "";
  formError.value = "";
  dialogVisible.value = true;
}

async function submitForm() {
  formError.value = "";
  if (!PERMISSION_CODE_PATTERN.test(form.permissionCode)) {
    formError.value = "权限码格式不合法，应匹配 resource.action（如 game.read）";
    return;
  }
  if (!form.permissionName.trim()) {
    formError.value = "请输入名称";
    return;
  }
  saving.value = true;
  try {
    await createPermission({ permissionCode: form.permissionCode, permissionName: form.permissionName });
    ElMessage.success("已创建权限码");
    dialogVisible.value = false;
    await reload();
  } catch (err) {
    reportError(err, "保存失败", (msg) => (formError.value = msg));
  } finally {
    saving.value = false;
  }
}

async function removePermission(row: PermissionItem) {
  try {
    await ElMessageBox.confirm(`确认删除权限码「${row.permissionCode}」？`, "删除权限码", { type: "warning" });
  } catch {
    return;
  }
  try {
    await deletePermission(row.id);
    ElMessage.success("已删除权限码");
    await reload();
  } catch (err) {
    if (err instanceof ApiError && err.code === "CONFLICT") {
      ElMessage.error(err.message || "该权限码仍被角色引用，请先解绑");
      return;
    }
    reportError(err, "删除权限码失败");
  }
}

onMounted(() => {
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
  width: 240px;
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

.text-muted {
  color: var(--text-subtle);
}
</style>
