<template>
  <div class="product-tab">
    <div class="product-tab__toolbar">
      <div class="product-tab__meta">
        <PageStatusTag tone="neutral" :label="`商品数：${total}`" />
      </div>
      <div class="product-tab__actions">
        <el-input
          v-model="keyword"
          class="keyword-input"
          clearable
          placeholder="搜索 productId / productName"
          @keyup.enter="reload(1)"
        />
        <el-select v-model="enabledFilter" clearable placeholder="启用状态" class="state-select" @change="reload(1)">
          <el-option label="启用" :value="true" />
          <el-option label="停用" :value="false" />
        </el-select>
        <el-button @click="reload(page)">刷新</el-button>
        <el-button v-perm="'product.write'" type="primary" @click="openCreate">新建商品</el-button>
      </div>
    </div>

    <el-alert
      v-if="!canWrite"
      type="info"
      :closable="false"
      title="当前账号仅有查看权限，编辑入口已置灰。"
      class="product-tab__alert"
    />
    <el-alert
      v-if="loadError"
      class="product-tab__alert"
      type="error"
      :closable="false"
      :title="loadError"
    />

    <el-table v-loading="loading" :data="rows" border size="small">
      <el-table-column prop="productId" min-width="180">
        <template #header>IAP 商品 ID (productId)</template>
      </el-table-column>
      <el-table-column prop="productName" label="商品名" min-width="160" />
      <el-table-column label="基准金额" min-width="180">
        <template #default="{ row }">{{ row.baseAmountDisplay }} {{ row.baseCurrency }}</template>
      </el-table-column>
      <el-table-column prop="priceId" min-width="220">
        <template #header>收银台价格档 (price_id)</template>
      </el-table-column>
      <el-table-column label="启用" width="88">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? "是" : "否" }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="updatedAt" label="更新时间" min-width="180" />
      <el-table-column label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button v-perm="'product.write'" link type="primary" @click="openEdit(row)">编辑</el-button>
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
        @current-change="reload"
      />
    </div>

    <el-drawer
      :model-value="drawerOpen"
      :title="editingProductId ? '编辑商品' : '新建商品'"
      size="540px"
      :close-on-click-modal="false"
      @update:model-value="(v: boolean) => !v && closeDrawer()"
    >
      <el-form label-position="top" :model="form">
        <el-form-item label="IAP 商品 ID (productId)" required>
          <el-input
            v-model="form.productId"
            maxlength="128"
            show-word-limit
            :disabled="!canWrite || Boolean(editingProductId)"
            placeholder="1-128 字符"
          />
        </el-form-item>
        <el-form-item label="商品名称 (productName)" required>
          <el-input v-model="form.productName" maxlength="128" show-word-limit :disabled="!canWrite" />
        </el-form-item>
        <el-form-item label="基准币种 (baseCurrency)" required>
          <el-select v-model="form.baseCurrency" class="full-width" filterable :disabled="!canWrite" @change="onCurrencyChanged">
            <el-option
              v-for="currency in dictionary.enabledCurrencySpecs"
              :key="currency.currencyCode"
              :label="`${currency.currencyCode} - ${currency.currencyName}`"
              :value="currency.currencyCode"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="基准金额 (baseAmount)" required>
          <el-input
            v-model="form.baseAmount"
            placeholder="主单位金额字符串，例如 4.99"
            :disabled="!canWrite"
            @blur="previewMinorAmount"
          />
          <div v-if="selectedCurrencySpec" class="amount-hint">
            <span>小数位限制：{{ selectedCurrencySpec.decimalPlaces }}</span>
            <span>最小值：{{ minAmountDisplay }}</span>
            <span>舍入：{{ selectedCurrencySpec.roundingMode }}</span>
          </div>
          <p v-if="minorPreview" class="amount-preview">{{ minorPreview }}</p>
        </el-form-item>
        <el-form-item label="收银台价格档 (price_id)" required>
          <el-input
            v-model="form.priceId"
            maxlength="64"
            show-word-limit
            :disabled="!canWrite"
            placeholder="1-64 字符（与 IAP 商品 ID 独立）"
          />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" :disabled="!canWrite" />
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="drawer-actions">
          <el-button @click="closeDrawer">取消</el-button>
          <el-button v-perm="'product.write'" type="primary" :loading="saving" @click="submitProduct">保存</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { ApiError } from "@/api/http";
import { useDictionaryStore } from "@/stores/dictionary";
import { usePermissionStore } from "@/stores/permission";
import {
  createProduct,
  listProducts,
  updateProduct,
  type ListProductsQuery,
  type ProductItem
} from "@/api/modules/products";

const props = defineProps<{ gameId: string }>();

const dictionary = useDictionaryStore();
const permission = usePermissionStore();
const canWrite = computed(() => permission.hasPerm("product.write"));

const loading = ref(false);
const saving = ref(false);
const loadError = ref("");
const rows = ref<ProductItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const keyword = ref("");
const enabledFilter = ref<boolean | undefined>(undefined);

const drawerOpen = ref(false);
const editingProductId = ref("");
const minorPreview = ref("");
const form = reactive({
  productId: "",
  productName: "",
  baseCurrency: "USD",
  baseAmount: "",
  priceId: "",
  enabled: true
});

const selectedCurrencySpec = computed(() => dictionary.getCurrencySpec(form.baseCurrency));

const minAmountDisplay = computed(() => {
  const spec = selectedCurrencySpec.value;
  if (!spec) {
    return "-";
  }
  return formatMinorToMajor(spec.minAmountMinor, spec.decimalPlaces);
});

function resetForm() {
  form.productId = "";
  form.productName = "";
  form.baseCurrency = dictionary.enabledCurrencySpecs[0]?.currencyCode ?? "USD";
  form.baseAmount = "";
  form.priceId = "";
  form.enabled = true;
  minorPreview.value = "";
}

function closeDrawer() {
  drawerOpen.value = false;
  editingProductId.value = "";
  resetForm();
}

function openCreate() {
  editingProductId.value = "";
  resetForm();
  drawerOpen.value = true;
}

function openEdit(row: ProductItem) {
  editingProductId.value = row.productId;
  form.productId = row.productId;
  form.productName = row.productName;
  form.baseCurrency = row.baseCurrency;
  form.baseAmount = row.baseAmountDisplay;
  form.priceId = row.priceId;
  form.enabled = row.enabled;
  minorPreview.value = "";
  drawerOpen.value = true;
}

function formatMinorToMajor(minor: number, decimalPlaces: number): string {
  if (!Number.isFinite(minor)) {
    return "-";
  }
  const sign = minor < 0 ? "-" : "";
  const absolute = Math.abs(Math.trunc(minor));
  const factor = 10 ** decimalPlaces;
  const integer = Math.trunc(absolute / factor);
  const fraction = String(absolute % factor).padStart(decimalPlaces, "0");
  return decimalPlaces > 0 ? `${sign}${integer}.${fraction}` : `${sign}${integer}`;
}

function roundByMode(value: number, mode: string): number {
  switch (mode) {
    case "floor":
      return Math.floor(value);
    case "ceil":
      return Math.ceil(value);
    case "truncate":
      return value < 0 ? Math.ceil(value) : Math.floor(value);
    default:
      return Math.round(value);
  }
}

function previewMinorAmount() {
  const spec = selectedCurrencySpec.value;
  if (!spec) {
    minorPreview.value = "";
    return;
  }
  const amount = Number(form.baseAmount);
  if (!Number.isFinite(amount)) {
    minorPreview.value = "";
    return;
  }
  const scaled = amount * 10 ** spec.decimalPlaces;
  const minor = roundByMode(scaled, spec.roundingMode);
  minorPreview.value = `将存储为 ${minor} minor（仅前端预览，最终以后端归一化为准）`;
}

function onCurrencyChanged() {
  minorPreview.value = "";
}

function validateForm(): boolean {
  if (!form.productId.trim()) {
    ElMessage.warning("请填写 IAP 商品 ID");
    return false;
  }
  if (form.productId.trim().length > 128) {
    ElMessage.warning("IAP 商品 ID 不能超过 128 字符");
    return false;
  }
  if (!form.productName.trim()) {
    ElMessage.warning("请填写商品名称");
    return false;
  }
  if (!form.baseCurrency) {
    ElMessage.warning("请选择基准币种");
    return false;
  }
  if (!form.baseAmount.trim()) {
    ElMessage.warning("请填写基准金额");
    return false;
  }
  if (!form.priceId.trim()) {
    ElMessage.warning("请填写收银台价格档");
    return false;
  }
  if (form.priceId.trim().length > 64) {
    ElMessage.warning("收银台价格档不能超过 64 字符");
    return false;
  }
  return true;
}

async function submitProduct() {
  if (!validateForm()) {
    return;
  }
  saving.value = true;
  try {
    if (!editingProductId.value) {
      await createProduct(props.gameId, {
        productId: form.productId.trim(),
        productName: form.productName.trim(),
        baseCurrency: form.baseCurrency,
        baseAmount: form.baseAmount.trim(),
        priceId: form.priceId.trim(),
        enabled: form.enabled
      });
      ElMessage.success("商品已创建");
    } else {
      await updateProduct(editingProductId.value, props.gameId, {
        productName: form.productName.trim(),
        baseCurrency: form.baseCurrency,
        baseAmount: form.baseAmount.trim(),
        priceId: form.priceId.trim(),
        enabled: form.enabled
      });
      ElMessage.success("商品已更新");
    }
    closeDrawer();
    await reload(page.value);
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "保存商品失败");
  } finally {
    saving.value = false;
  }
}

async function reload(targetPage = page.value) {
  loading.value = true;
  loadError.value = "";
  try {
    const query: ListProductsQuery = {
      page: targetPage,
      pageSize: pageSize.value,
      keyword: keyword.value.trim() || undefined,
      enabled: enabledFilter.value
    };
    const res = await listProducts(props.gameId, query);
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    rows.value = [];
    total.value = 0;
    loadError.value = err instanceof ApiError ? err.message : "加载商品列表失败";
  } finally {
    loading.value = false;
  }
}

onMounted(async () => {
  await dictionary.ensureCurrencySpecs();
  resetForm();
  await reload(1);
});
</script>

<style scoped>
.product-tab {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.product-tab__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.product-tab__meta,
.product-tab__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.keyword-input {
  width: 260px;
}

.state-select {
  width: 120px;
}

.product-tab__alert {
  margin-bottom: 2px;
}

.pager {
  display: flex;
  justify-content: flex-end;
}

.full-width {
  width: 100%;
}

.amount-hint {
  margin-top: 8px;
  color: var(--text-subtle);
  font-size: 12px;
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.amount-preview {
  margin: 6px 0 0;
  color: var(--text-subtle);
  font-size: 12px;
}

.drawer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
