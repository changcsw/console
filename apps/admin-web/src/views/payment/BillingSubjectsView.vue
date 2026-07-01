<template>
  <section>
    <div class="toolbar">
      <el-button @click="load(1)">刷新</el-button>
      <el-button v-perm="'payment.write'" type="primary" @click="drawerOpen = true">新增主体</el-button>
    </div>

    <el-alert v-if="!canWrite" type="info" :closable="false" title="当前账号仅有查看权限，新增入口已置灰。" class="hint" />

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="subjectId" label="Subject ID" min-width="180" />
      <el-table-column prop="subjectName" label="主体名称" min-width="180" />
      <el-table-column prop="legalEntityName" label="法务实体" min-width="260" />
      <el-table-column label="启用" width="100">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? "是" : "否" }}</el-tag>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        background
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="load"
      />
    </div>

    <el-drawer :model-value="drawerOpen" title="新增结算主体" size="520px" @update:model-value="onDrawerChange">
      <el-form label-position="top" :model="form">
        <el-form-item label="subjectId" required>
          <el-input v-model.trim="form.subjectId" :disabled="!canWrite" placeholder="1-64 位小写字母/数字/下划线" />
        </el-form-item>
        <el-form-item label="主体名称" required>
          <el-input v-model.trim="form.subjectName" :disabled="!canWrite" maxlength="128" show-word-limit />
        </el-form-item>
        <el-form-item label="法务实体" required>
          <el-input v-model.trim="form.legalEntityName" :disabled="!canWrite" maxlength="255" show-word-limit />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" :disabled="!canWrite" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="actions">
          <el-button @click="drawerOpen = false">取消</el-button>
          <el-button v-perm="'payment.write'" type="primary" :loading="saving" @click="submit">保存</el-button>
        </div>
      </template>
    </el-drawer>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { ApiError } from "@/api/http";
import { usePermissionStore } from "@/stores/permission";
import { createBillingSubject, listBillingSubjects, type BillingSubjectItem } from "@/api/modules/payment";

const permission = usePermissionStore();
const canWrite = computed(() => permission.hasPerm("payment.write"));

const loading = ref(false);
const saving = ref(false);
const rows = ref<BillingSubjectItem[]>([]);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const drawerOpen = ref(false);

const form = reactive({
  subjectId: "",
  subjectName: "",
  legalEntityName: "",
  enabled: true
});

function resetForm() {
  form.subjectId = "";
  form.subjectName = "";
  form.legalEntityName = "";
  form.enabled = true;
}

function onDrawerChange(next: boolean) {
  drawerOpen.value = next;
  if (!next) {
    resetForm();
  }
}

async function load(targetPage = page.value) {
  loading.value = true;
  try {
    const res = await listBillingSubjects({ page: targetPage, pageSize: pageSize.value });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载结算主体失败");
  } finally {
    loading.value = false;
  }
}

function validate(): boolean {
  if (!/^[a-z0-9_]{1,64}$/.test(form.subjectId)) {
    ElMessage.warning("subjectId 需为 1-64 位小写字母/数字/下划线");
    return false;
  }
  if (!form.subjectName) {
    ElMessage.warning("请填写主体名称");
    return false;
  }
  if (!form.legalEntityName) {
    ElMessage.warning("请填写法务实体");
    return false;
  }
  return true;
}

async function submit() {
  if (!validate()) {
    return;
  }
  saving.value = true;
  try {
    await createBillingSubject({
      subjectId: form.subjectId,
      subjectName: form.subjectName,
      legalEntityName: form.legalEntityName,
      enabled: form.enabled
    });
    ElMessage.success("结算主体已创建");
    drawerOpen.value = false;
    resetForm();
    await load(1);
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "创建结算主体失败");
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void load(1);
});
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 12px;
}

.hint {
  margin-bottom: 12px;
}

.pager {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
