<template>
  <div class="panel">
    <div class="panel__toolbar">
      <div class="panel__filters">
        <el-input
          v-model="keyword"
          placeholder="用户名 / 名称 / 邮箱"
          clearable
          class="filter-keyword"
          @keyup.enter="reload(1)"
          @clear="reload(1)"
        />
        <el-select v-model="statusFilter" placeholder="状态" clearable class="filter-status" @change="reload(1)">
          <el-option label="启用" value="active" />
          <el-option label="停用" value="disabled" />
        </el-select>
        <el-button @click="reload(1)">查询</el-button>
      </div>
      <el-button v-perm="'admin_user.write'" type="primary" @click="openCreate">新建管理员</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="userName" label="用户名" min-width="120" />
      <el-table-column prop="displayName" label="显示名" min-width="120" />
      <el-table-column prop="email" label="邮箱" min-width="160" />
      <el-table-column label="角色" min-width="160">
        <template #default="{ row }">
          <span v-if="row.roles?.length">{{ roleNames(row.roles) }}</span>
          <span v-else class="text-muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <PageStatusTag :tone="row.status === 'active' ? 'success' : 'danger'" :label="row.status === 'active' ? '启用' : '停用'" />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="320" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" v-perm="'admin_user.write'" @click="openEdit(row)">编辑</el-button>
          <el-button link type="primary" v-perm="'admin_user.write'" @click="openRoles(row)">角色</el-button>
          <el-button link type="primary" v-perm="'admin_user.write'" @click="openReset(row)">重置密码</el-button>
          <el-button
            link
            :type="row.status === 'active' ? 'danger' : 'success'"
            v-perm="'admin_user.write'"
            @click="toggleStatus(row)"
          >
            {{ row.status === "active" ? "停用" : "启用" }}
          </el-button>
        </template>
      </el-table-column>
      <template #empty>
        <span class="text-muted">暂无管理员</span>
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

    <!-- 新建 / 编辑抽屉 -->
    <el-drawer v-model="drawerVisible" :title="editing ? '编辑管理员' : '新建管理员'" size="460px">
      <el-form label-position="top">
        <el-form-item label="用户名">
          <el-input v-model="form.userName" :disabled="editing" placeholder="1-64 字符，唯一" />
        </el-form-item>
        <el-form-item label="显示名">
          <el-input v-model="form.displayName" placeholder="1-128 字符" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="form.email" placeholder="可选，需为合法邮箱" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status" class="full-width">
            <el-option label="启用" value="active" />
            <el-option label="停用" value="disabled" />
          </el-select>
        </el-form-item>
        <template v-if="!editing">
          <el-form-item label="初始密码">
            <el-input v-model="form.password" type="password" show-password placeholder="可选，8-128 字符，提供则建密码身份" />
          </el-form-item>
          <el-form-item label="飞书 union_id">
            <el-input v-model="form.feishuKey" placeholder="可选，提供则建飞书身份" />
          </el-form-item>
          <el-form-item label="角色">
            <el-select v-model="form.roleIds" multiple class="full-width" placeholder="选择角色">
              <el-option v-for="role in roleOptions" :key="role.id" :label="role.roleName" :value="role.id" />
            </el-select>
          </el-form-item>
        </template>
        <el-form-item v-if="editing && editIdentities.length" label="绑定身份（脱敏）">
          <div class="identities">
            <div v-for="identity in editIdentities" :key="identity.identityType" class="identity-row">
              <span class="identity-type">{{ identity.identityType }}</span>
              <span class="identity-key">{{ identity.identityKey }}</span>
            </div>
          </div>
        </el-form-item>
        <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>
      </el-form>
      <template #footer>
        <el-button @click="drawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitForm">保存</el-button>
      </template>
    </el-drawer>

    <!-- 角色分配抽屉 -->
    <el-drawer v-model="rolesDrawerVisible" title="分配角色（全量覆盖）" size="420px">
      <el-form label-position="top">
        <el-form-item label="角色">
          <el-select v-model="assignRoleIds" multiple class="full-width" placeholder="选择角色">
            <el-option v-for="role in roleOptions" :key="role.id" :label="role.roleName" :value="role.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rolesDrawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitRoles">保存</el-button>
      </template>
    </el-drawer>

    <!-- 重置密码弹窗 -->
    <el-dialog v-model="resetDialogVisible" title="重置密码" width="420px">
      <el-form label-position="top">
        <el-form-item label="新密码">
          <el-input v-model="newPassword" type="password" show-password placeholder="8-128 字符" />
        </el-form-item>
        <p v-if="resetError" class="panel__error" role="alert">{{ resetError }}</p>
      </el-form>
      <template #footer>
        <el-button @click="resetDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitReset">确认重置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { ApiError } from "@/api/http";
import {
  listAdminUsers,
  createAdminUser,
  updateAdminUser,
  getAdminUser,
  assignAdminUserRoles,
  resetAdminUserPassword,
  listRoles,
  type AdminUserListItem,
  type AdminUserStatus,
  type AdminUserIdentity,
  type RoleRef,
  type RoleListItem
} from "@/api/modules/system";

function roleNames(roles: RoleRef[]): string {
  return roles.map((role) => role.roleName).join("、");
}

const rows = ref<AdminUserListItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);

const keyword = ref("");
const statusFilter = ref<AdminUserStatus | "">("");

const roleOptions = ref<RoleListItem[]>([]);

const drawerVisible = ref(false);
const editing = ref(false);
const editingId = ref<number | null>(null);
const saving = ref(false);
const formError = ref("");
const form = reactive({
  userName: "",
  displayName: "",
  email: "",
  status: "active" as AdminUserStatus,
  password: "",
  feishuKey: "",
  roleIds: [] as number[]
});
const editIdentities = ref<AdminUserIdentity[]>([]);

const rolesDrawerVisible = ref(false);
const assignRoleIds = ref<number[]>([]);

const resetDialogVisible = ref(false);
const newPassword = ref("");
const resetError = ref("");

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
    const res = await listAdminUsers({
      page: targetPage,
      pageSize: pageSize.value,
      keyword: keyword.value || undefined,
      status: statusFilter.value || undefined
    });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    reportError(err, "加载管理员列表失败");
  } finally {
    loading.value = false;
  }
}

async function loadRoleOptions() {
  try {
    const res = await listRoles({ pageSize: 100 });
    roleOptions.value = res.items;
  } catch {
    /* 角色选项加载失败不阻断列表 */
  }
}

function resetForm() {
  form.userName = "";
  form.displayName = "";
  form.email = "";
  form.status = "active";
  form.password = "";
  form.feishuKey = "";
  form.roleIds = [];
  formError.value = "";
}

function openCreate() {
  editing.value = false;
  editingId.value = null;
  resetForm();
  drawerVisible.value = true;
}

async function openEdit(row: AdminUserListItem) {
  editing.value = true;
  editingId.value = row.id;
  resetForm();
  editIdentities.value = [];
  form.userName = row.userName;
  form.displayName = row.displayName;
  form.email = row.email;
  form.status = row.status;
  drawerVisible.value = true;
  try {
    const detail = await getAdminUser(row.id);
    editIdentities.value = detail.identities;
  } catch {
    /* 身份脱敏信息加载失败不阻断编辑 */
  }
}

async function submitForm() {
  formError.value = "";
  saving.value = true;
  try {
    if (editing.value && editingId.value != null) {
      await updateAdminUser(editingId.value, {
        displayName: form.displayName,
        email: form.email,
        status: form.status
      });
      ElMessage.success("已更新管理员");
    } else {
      await createAdminUser({
        userName: form.userName,
        displayName: form.displayName,
        email: form.email || undefined,
        status: form.status,
        password: form.password || undefined,
        feishuKey: form.feishuKey || undefined,
        roleIds: form.roleIds.length ? form.roleIds : undefined
      });
      ElMessage.success("已创建管理员");
    }
    drawerVisible.value = false;
    await reload();
  } catch (err) {
    reportError(err, "保存失败", (msg) => (formError.value = msg));
  } finally {
    saving.value = false;
  }
}

async function openRoles(row: AdminUserListItem) {
  editingId.value = row.id;
  rolesDrawerVisible.value = true;
  try {
    const detail = await getAdminUser(row.id);
    assignRoleIds.value = detail.roles.map((role) => role.id);
  } catch (err) {
    reportError(err, "加载用户角色失败");
  }
}

async function submitRoles() {
  if (editingId.value == null) {
    return;
  }
  saving.value = true;
  try {
    await assignAdminUserRoles(editingId.value, assignRoleIds.value);
    ElMessage.success("已更新角色");
    rolesDrawerVisible.value = false;
    await reload();
  } catch (err) {
    reportError(err, "分配角色失败");
  } finally {
    saving.value = false;
  }
}

function openReset(row: AdminUserListItem) {
  editingId.value = row.id;
  newPassword.value = "";
  resetError.value = "";
  resetDialogVisible.value = true;
}

async function submitReset() {
  if (editingId.value == null) {
    return;
  }
  resetError.value = "";
  if (newPassword.value.length < 8) {
    resetError.value = "新密码需至少 8 位";
    return;
  }
  saving.value = true;
  try {
    await resetAdminUserPassword(editingId.value, newPassword.value);
    ElMessage.success("密码已重置");
    resetDialogVisible.value = false;
  } catch (err) {
    reportError(err, "重置密码失败", (msg) => (resetError.value = msg));
  } finally {
    saving.value = false;
  }
}

async function toggleStatus(row: AdminUserListItem) {
  const next: AdminUserStatus = row.status === "active" ? "disabled" : "active";
  try {
    await updateAdminUser(row.id, { status: next });
    ElMessage.success(next === "active" ? "已启用" : "已停用");
    await reload();
  } catch (err) {
    reportError(err, "状态变更失败");
  }
}

onMounted(() => {
  void reload(1);
  void loadRoleOptions();
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
  width: 220px;
}

.filter-status {
  width: 120px;
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

.full-width {
  width: 100%;
}

.text-muted {
  color: var(--text-subtle);
}

.identities {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 100%;
}

.identity-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 6px 10px;
  background: #f1f5f9;
  border-radius: var(--radius-sm);
  font-size: 13px;
}

.identity-type {
  font-weight: 600;
}

.identity-key {
  color: var(--text-subtle);
  font-family: monospace;
}
</style>
