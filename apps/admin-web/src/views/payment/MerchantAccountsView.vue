<template>
  <section>
    <div class="toolbar">
      <el-select v-model="filter.providerId" clearable placeholder="按 provider 过滤" class="w-220" @change="load(1)">
        <el-option v-for="item in providers" :key="item.providerId" :label="item.providerName" :value="item.providerId" />
      </el-select>
      <el-button @click="load(1)">刷新</el-button>
      <el-button v-perm="'payment.write'" type="primary" @click="openCreate">新增商户</el-button>
    </div>

    <el-alert v-if="!canWrite" type="info" :closable="false" title="当前账号仅有查看权限，新增入口已置灰。" class="hint" />

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="merchantAccountId" label="Merchant Account ID" min-width="180" />
      <el-table-column prop="providerId" label="Provider" min-width="160" />
      <el-table-column prop="subjectId" label="Subject" min-width="160" />
      <el-table-column prop="merchantId" label="Merchant ID" min-width="160" />
      <el-table-column prop="merchantName" label="Merchant Name" min-width="180" />
      <el-table-column label="Secret" width="100">
        <template #default="{ row }">
          <code>{{ row.secret || "masked" }}</code>
        </template>
      </el-table-column>
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

    <el-drawer :model-value="drawerOpen" title="新增商户账户" size="620px" :close-on-click-modal="false" @update:model-value="onDrawerChange">
      <el-form label-position="top" :model="form">
        <el-form-item label="merchantAccountId" required>
          <el-input v-model.trim="form.merchantAccountId" :disabled="!canWrite || saving" />
        </el-form-item>
        <el-form-item label="Provider" required>
          <el-select v-model="form.providerId" class="full" filterable :disabled="!canWrite || saving" @change="onProviderChange">
            <el-option
              v-for="item in providers"
              :key="item.providerId"
              :label="`${item.providerName} (${item.providerId})`"
              :value="item.providerId"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="Billing Subject" required>
          <el-select v-model="form.subjectId" class="full" filterable :disabled="!canWrite || saving">
            <el-option
              v-for="item in subjects"
              :key="item.subjectId"
              :label="`${item.subjectName} (${item.subjectId})`"
              :value="item.subjectId"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="merchantId" required>
          <el-input v-model.trim="form.merchantId" :disabled="!canWrite || saving" />
        </el-form-item>
        <el-form-item label="merchantName" required>
          <el-input v-model.trim="form.merchantName" :disabled="!canWrite || saving" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" :disabled="!canWrite || saving" />
        </el-form-item>
      </el-form>

      <el-alert v-if="templateLoadError" type="warning" :closable="false" :title="templateLoadError" class="hint" />

      <TemplateConfigRenderer
        v-if="activeTemplate"
        v-model="form.configJson"
        v-model:secret-values="secretInputs"
        :template="activeTemplate"
        :disabled="!canWrite || saving"
      />

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
import TemplateConfigRenderer from "@/views/games/detail/components/TemplateConfigRenderer.vue";
import {
  createMerchantAccount,
  getProviderTemplate,
  listBillingSubjects,
  listMerchantAccounts,
  listProviders,
  type BillingSubjectItem,
  type MerchantAccountItem,
  type ProviderItem,
  type ProviderTemplate
} from "@/api/modules/payment";

const permission = usePermissionStore();
const canWrite = computed(() => permission.hasPerm("payment.write"));

const loading = ref(false);
const saving = ref(false);
const rows = ref<MerchantAccountItem[]>([]);
const providers = ref<ProviderItem[]>([]);
const subjects = ref<BillingSubjectItem[]>([]);
const page = ref(1);
const pageSize = ref(20);
const total = ref(0);
const drawerOpen = ref(false);
const activeTemplate = ref<ProviderTemplate | null>(null);
const templateLoadError = ref("");

const filter = reactive({
  providerId: ""
});

const form = reactive({
  merchantAccountId: "",
  providerId: "",
  subjectId: "",
  merchantId: "",
  merchantName: "",
  configJson: {} as Record<string, unknown>,
  enabled: true
});

const secretInputs = ref<Record<string, string>>({});

function resetForm() {
  form.merchantAccountId = "";
  form.providerId = "";
  form.subjectId = "";
  form.merchantId = "";
  form.merchantName = "";
  form.configJson = {};
  form.enabled = true;
  secretInputs.value = {};
  activeTemplate.value = null;
  templateLoadError.value = "";
}

function onDrawerChange(next: boolean) {
  drawerOpen.value = next;
  if (!next) {
    resetForm();
  }
}

async function loadBaseData() {
  const [providerRes, subjectRes] = await Promise.all([
    listProviders({ page: 1, pageSize: 200, enabled: true }),
    listBillingSubjects({ page: 1, pageSize: 200, enabled: true })
  ]);
  providers.value = providerRes.items;
  subjects.value = subjectRes.items;
}

async function load(targetPage = page.value) {
  loading.value = true;
  try {
    const res = await listMerchantAccounts({
      page: targetPage,
      pageSize: pageSize.value,
      providerId: filter.providerId || undefined
    });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载商户账户失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  resetForm();
  drawerOpen.value = true;
}

async function onProviderChange(providerId: string) {
  form.providerId = providerId;
  secretInputs.value = {};
  form.configJson = {};
  activeTemplate.value = null;
  templateLoadError.value = "";
  if (!providerId) {
    return;
  }
  try {
    activeTemplate.value = await getProviderTemplate(providerId);
  } catch (err) {
    // 该 provider 暂无可用模板（后端 404）或拉取失败时降级：
    // 抽屉仍可填写基础字段并提交，仅模板四件套区域不渲染。
    const base = err instanceof ApiError ? err.message : "拉取 provider 模板失败";
    templateLoadError.value = `${base}（该 provider 暂无可用模板，可先填写基础字段保存）`;
  }
}

function validate(): boolean {
  if (!form.merchantAccountId || !form.providerId || !form.subjectId || !form.merchantId || !form.merchantName) {
    ElMessage.warning("请补齐必填字段");
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
    await createMerchantAccount({
      merchantAccountId: form.merchantAccountId,
      providerId: form.providerId,
      subjectId: form.subjectId,
      merchantId: form.merchantId,
      merchantName: form.merchantName,
      configJson: form.configJson,
      secrets: secretInputs.value,
      enabled: form.enabled
    });
    ElMessage.success("商户账户已创建");
    drawerOpen.value = false;
    resetForm();
    await load(1);
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "创建商户账户失败");
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  try {
    await loadBaseData();
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载基础数据失败");
  }
  await load(1);
});
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.hint {
  margin-bottom: 12px;
}

.w-220 {
  width: 220px;
}

.pager {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}

.full {
  width: 100%;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
