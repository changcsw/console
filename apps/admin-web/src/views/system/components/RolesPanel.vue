<template>
  <div class="panel">
    <div class="panel__toolbar">
      <div class="panel__filters">
        <el-input
          v-model="keyword"
          placeholder="角色码 / 角色名"
          clearable
          class="filter-keyword"
          @keyup.enter="reload(1)"
          @clear="reload(1)"
        />
        <el-button @click="reload(1)">查询</el-button>
      </div>
      <el-button v-perm="'role.write'" type="primary" @click="openCreate">新建角色</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="roleCode" label="角色码" min-width="160" />
      <el-table-column prop="roleName" label="角色名" min-width="160" />
      <el-table-column prop="permissionCount" label="权限数" width="100" />
      <el-table-column label="操作" width="280" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" v-perm="'role.write'" @click="openEdit(row)">改名</el-button>
          <el-button link type="primary" v-perm="'role.write'" @click="openPermissions(row)">配置权限</el-button>
          <el-button link type="danger" v-perm="'role.write'" @click="removeRole(row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <span class="text-muted">暂无角色</span>
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

    <!-- 新建 / 改名抽屉 -->
    <el-drawer v-model="drawerVisible" :title="editing ? '编辑角色' : '新建角色'" size="460px">
      <el-form label-position="top">
        <el-form-item label="角色码">
          <el-input v-model="form.roleCode" :disabled="editing" placeholder="如 super_admin，建议 [a-z0-9_]+，不可改" />
        </el-form-item>
        <el-form-item label="角色名">
          <el-input v-model="form.roleName" placeholder="1-128 字符" />
        </el-form-item>
        <el-form-item v-if="!editing" label="初始权限">
          <PermissionTree ref="createTreeRef" :permissions="allPermissions" :checked="[]" />
        </el-form-item>
        <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>
      </el-form>
      <template #footer>
        <el-button @click="drawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitForm">保存</el-button>
      </template>
    </el-drawer>

    <!-- 权限配置抽屉 -->
    <el-drawer v-model="permDrawerVisible" title="配置角色权限（全量覆盖）" size="480px">
      <p class="panel__hint">
        权限按 resource 分组。保存为<strong>全量覆盖</strong>：当前以空选起步（后端暂无单角色权限读取接口），请勾选该角色应有的<strong>全部</strong>权限后再保存；生效需用户令牌刷新（≤30 分钟）。
      </p>
      <PermissionTree ref="assignTreeRef" :permissions="allPermissions" :checked="assignCheckedIds" />
      <template #footer>
        <el-button @click="permDrawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitPermissions">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { ApiError } from "@/api/http";
import {
  listRoles,
  createRole,
  updateRole,
  deleteRole,
  assignRolePermissions,
  listPermissions,
  type RoleListItem,
  type PermissionItem
} from "@/api/modules/system";
import PermissionTree from "./PermissionTree.vue";

const rows = ref<RoleListItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);
const keyword = ref("");

const allPermissions = ref<PermissionItem[]>([]);

const drawerVisible = ref(false);
const editing = ref(false);
const editingId = ref<number | null>(null);
const saving = ref(false);
const formError = ref("");
const form = reactive({ roleCode: "", roleName: "" });
const createTreeRef = ref<InstanceType<typeof PermissionTree> | null>(null);

const permDrawerVisible = ref(false);
const assignCheckedIds = ref<number[]>([]);
const assignTreeRef = ref<InstanceType<typeof PermissionTree> | null>(null);

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
    const res = await listRoles({ page: targetPage, pageSize: pageSize.value, keyword: keyword.value || undefined });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    reportError(err, "加载角色列表失败");
  } finally {
    loading.value = false;
  }
}

async function loadAllPermissions() {
  try {
    const res = await listPermissions({ all: true });
    allPermissions.value = res.items;
  } catch {
    /* 忽略 */
  }
}

function openCreate() {
  editing.value = false;
  editingId.value = null;
  form.roleCode = "";
  form.roleName = "";
  formError.value = "";
  drawerVisible.value = true;
}

function openEdit(row: RoleListItem) {
  editing.value = true;
  editingId.value = row.id;
  form.roleCode = row.roleCode;
  form.roleName = row.roleName;
  formError.value = "";
  drawerVisible.value = true;
}

async function submitForm() {
  formError.value = "";
  saving.value = true;
  try {
    if (editing.value && editingId.value != null) {
      await updateRole(editingId.value, { roleName: form.roleName });
      ElMessage.success("已更新角色");
    } else {
      await createRole({
        roleCode: form.roleCode,
        roleName: form.roleName,
        permissionIds: createTreeRef.value?.getCheckedIds() ?? []
      });
      ElMessage.success("已创建角色");
    }
    drawerVisible.value = false;
    await reload();
  } catch (err) {
    reportError(err, "保存失败", (msg) => (formError.value = msg));
  } finally {
    saving.value = false;
  }
}

function openPermissions(row: RoleListItem) {
  editingId.value = row.id;
  // 后端未提供「角色详情/单角色权限读取」接口，无法回填现有勾选；
  // PUT 为全量覆盖，故以空选起步并在抽屉内明确提示（见 handoff 偏差）。
  assignCheckedIds.value = [];
  permDrawerVisible.value = true;
}

async function submitPermissions() {
  if (editingId.value == null) {
    return;
  }
  saving.value = true;
  try {
    await assignRolePermissions(editingId.value, assignTreeRef.value?.getCheckedIds() ?? []);
    ElMessage.success("已更新角色权限");
    permDrawerVisible.value = false;
    await reload();
  } catch (err) {
    reportError(err, "配置权限失败");
  } finally {
    saving.value = false;
  }
}

async function removeRole(row: RoleListItem) {
  try {
    await ElMessageBox.confirm(`确认删除角色「${row.roleName}」？`, "删除角色", { type: "warning" });
  } catch {
    return;
  }
  try {
    await deleteRole(row.id);
    ElMessage.success("已删除角色");
    await reload();
  } catch (err) {
    if (err instanceof ApiError && err.code === "CONFLICT") {
      const userCount = (err.details?.[0] as { userCount?: number })?.userCount;
      ElMessage.error(userCount != null ? `该角色仍被 ${userCount} 个用户引用，请先解绑` : err.message);
      return;
    }
    reportError(err, "删除角色失败");
  }
}

onMounted(() => {
  void reload(1);
  void loadAllPermissions();
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

.panel__hint {
  color: var(--text-subtle);
  font-size: 12px;
  margin: 0 0 12px;
}

.text-muted {
  color: var(--text-subtle);
}
</style>
