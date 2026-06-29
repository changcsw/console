<template>
  <PageCard title="价格矩阵编辑器" description="金额输入受 currency_specs 约束，并实时展示归一化 minor 预览。">
    <template v-if="!templateId || !version">
      <div class="empty-state">
        <p class="empty-state__title">请选择版本</p>
        <p class="empty-state__hint">从上方版本列表选择一个版本后，可编辑并保存价格矩阵。</p>
      </div>
    </template>

    <template v-else>
      <div class="toolbar">
        <el-space>
          <el-tag size="small">模板：{{ templateId }}</el-tag>
          <el-tag size="small">版本：v{{ version }}</el-tag>
          <el-tag size="small" :type="versionStatus === 'published' ? 'success' : 'primary'">
            {{ versionStatus }}
          </el-tag>
          <el-tag v-if="readonlyByStatus" size="small" type="warning">{{ versionStatus }} 只读</el-tag>
        </el-space>
        <el-space>
          <el-button v-perm="'cashier.write'" :disabled="readonly || saving" @click="addRow">新增行</el-button>
          <el-button v-perm="'cashier.write'" type="primary" :disabled="readonly || saving" :loading="saving" @click="saveRows">
            保存矩阵
          </el-button>
        </el-space>
      </div>

      <el-table v-loading="loading" :data="rows" border size="small">
        <el-table-column label="Country" min-width="120">
          <template #default="{ row }">
            <el-input v-model.trim="row.countryCode" :disabled="readonly" placeholder="US" />
          </template>
        </el-table-column>
        <el-table-column label="Region" min-width="120">
          <template #default="{ row }">
            <el-input v-model.trim="row.regionCode" :disabled="readonly" placeholder="*" />
          </template>
        </el-table-column>
        <el-table-column label="Currency" width="120">
          <template #default="{ row }">
            <el-select v-model="row.currency" :disabled="readonly" filterable>
              <el-option v-for="spec in CURRENCY_SPECS" :key="spec.currencyCode" :label="spec.currencyCode" :value="spec.currencyCode" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="Price ID" min-width="130">
          <template #default="{ row }">
            <el-input v-model.trim="row.priceId" :disabled="readonly" placeholder="com.game.pack.001" />
          </template>
        </el-table-column>
        <el-table-column label="税前金额(major)" min-width="160">
          <template #default="{ row }">
            <el-input v-model.trim="row.preTaxMajorInput" :disabled="readonly" placeholder="9.99" />
          </template>
        </el-table-column>
        <el-table-column label="Tax Rate" width="120">
          <template #default="{ row }">
            <el-input-number
              v-model="row.taxRate"
              :disabled="readonly"
              :min="0"
              :max="1"
              :precision="6"
              :step="0.01"
              :controls="false"
            />
          </template>
        </el-table-column>
        <el-table-column label="生效时间" min-width="180">
          <template #default="{ row }">
            <el-date-picker
              v-model="row.effectiveAt"
              :disabled="readonly"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm:ss[Z]"
              placeholder="选择生效时间"
            />
          </template>
        </el-table-column>
        <el-table-column label="归一化预览" min-width="260">
          <template #default="{ row }">
            <div class="preview">
              <p :class="previewIsError(row) ? 'preview__err' : 'preview__ok'">{{ previewText(row) }}</p>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="82" fixed="right">
          <template #default="{ $index }">
            <el-button v-perm="'cashier.write'" :disabled="readonly" link type="danger" @click="removeRow($index)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <p class="hint">提示：精度/下限/舍入规则按 compact 约定的 `currency_specs`（USD/JPY/KRW/TWD/EUR）前端预校验，后端仍为最终裁决。</p>
    </template>
  </PageCard>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import { ApiError } from "@/api/http";
import { getCashierPriceRows, putCashierPriceRows, type CashierPriceRow, type PutPriceRow, type VersionStatus } from "@/api/modules/cashier";
import { usePermissionStore } from "@/stores/permission";

interface CurrencySpec {
  currencyCode: string;
  decimalPlaces: number;
  minAmountMinor: number;
  roundingMode: "half_up" | "floor" | "ceil" | "truncate";
}

const CURRENCY_SPECS: CurrencySpec[] = [
  { currencyCode: "USD", decimalPlaces: 2, minAmountMinor: 1, roundingMode: "half_up" },
  { currencyCode: "JPY", decimalPlaces: 0, minAmountMinor: 1, roundingMode: "half_up" },
  { currencyCode: "KRW", decimalPlaces: 0, minAmountMinor: 1, roundingMode: "half_up" },
  { currencyCode: "TWD", decimalPlaces: 0, minAmountMinor: 1, roundingMode: "half_up" },
  { currencyCode: "EUR", decimalPlaces: 2, minAmountMinor: 1, roundingMode: "half_up" }
];

interface EditableRow {
  countryCode: string;
  regionCode: string;
  currency: string;
  priceId: string;
  preTaxMajorInput: string;
  taxRate: number;
  effectiveAt: string;
}

type NormalizeResult =
  | {
      ok: true;
      value: CashierPriceRow;
    }
  | {
      ok: false;
      message: string;
    };

const props = defineProps<{
  templateId: string;
  version: string;
  versionStatus: VersionStatus;
}>();

const emit = defineEmits<{
  (e: "saved"): void;
}>();

const permission = usePermissionStore();
const loading = ref(false);
const saving = ref(false);
const rows = ref<EditableRow[]>([]);

const readonlyByStatus = computed(
  () => props.versionStatus === "published" || props.versionStatus === "archived"
);
const readonly = computed(() => readonlyByStatus.value || !permission.hasPerm("cashier.write"));

watch(
  () => [props.templateId, props.version] as const,
  ([templateId, version]) => {
    if (templateId && version) {
      void loadRows(templateId, version);
    } else {
      rows.value = [];
    }
  },
  { immediate: true }
);

function fromMinorToMajor(minor: number, decimalPlaces: number): string {
  const value = minor / 10 ** decimalPlaces;
  return value.toFixed(decimalPlaces);
}

function addRow() {
  rows.value.push({
    countryCode: "",
    regionCode: "*",
    currency: "USD",
    priceId: "",
    preTaxMajorInput: "",
    taxRate: 0,
    effectiveAt: new Date().toISOString()
  });
}

function removeRow(index: number) {
  rows.value.splice(index, 1);
}

async function loadRows(templateId: string, version: string) {
  loading.value = true;
  try {
    const data = await getCashierPriceRows(templateId, version);
    const items = data.items ?? [];
    rows.value = items.map((item) => {
      const spec = CURRENCY_SPECS.find((s) => s.currencyCode === item.currency) ?? CURRENCY_SPECS[0];
      return {
        countryCode: item.countryCode,
        regionCode: item.regionCode || "*",
        currency: item.currency,
        priceId: item.priceId,
        preTaxMajorInput: fromMinorToMajor(item.preTaxAmountMinor, spec.decimalPlaces),
        taxRate: typeof item.taxRate === "string" ? Number.parseFloat(item.taxRate) || 0 : item.taxRate,
        effectiveAt: item.effectiveAt
      };
    });
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载价格矩阵失败");
    rows.value = [];
  } finally {
    loading.value = false;
  }
}

function normalizeAmount(value: number, mode: CurrencySpec["roundingMode"]): number {
  if (mode === "floor") {
    return Math.floor(value);
  }
  if (mode === "ceil") {
    return Math.ceil(value);
  }
  if (mode === "truncate") {
    return value < 0 ? Math.ceil(value) : Math.trunc(value);
  }
  return Math.round(value);
}

function normalizeRow(row: EditableRow): NormalizeResult {
  if (!row.countryCode || !row.priceId || !row.currency || !row.preTaxMajorInput) {
    return { ok: false, message: "请填写 country/priceId/currency/金额" };
  }

  const spec = CURRENCY_SPECS.find((item) => item.currencyCode === row.currency);
  if (!spec) {
    return { ok: false, message: "币种不在 currency_specs（CURRENCY_NOT_SUPPORTED）" };
  }

  const amount = Number(row.preTaxMajorInput);
  if (!Number.isFinite(amount) || amount < 0) {
    return { ok: false, message: "金额必须是非负数字" };
  }

  const inputDecimals = row.preTaxMajorInput.includes(".") ? row.preTaxMajorInput.split(".")[1].length : 0;
  if (inputDecimals > spec.decimalPlaces) {
    return { ok: false, message: `${spec.currencyCode} 最大小数位 ${spec.decimalPlaces}` };
  }

  const minorRaw = amount * 10 ** spec.decimalPlaces;
  const preTaxAmountMinor = normalizeAmount(minorRaw, spec.roundingMode);
  if (preTaxAmountMinor < spec.minAmountMinor) {
    return { ok: false, message: `${spec.currencyCode} 最小金额（minor）为 ${spec.minAmountMinor}` };
  }

  const taxAmountMinor = Math.round(preTaxAmountMinor * row.taxRate);
  const afterTaxAmountMinor = preTaxAmountMinor + taxAmountMinor;

  return {
    ok: true,
    value: {
      countryCode: row.countryCode.trim().toUpperCase(),
      regionCode: row.regionCode.trim() || "*",
      currency: row.currency,
      priceId: row.priceId.trim(),
      preTaxAmountMinor,
      taxRate: row.taxRate,
      taxAmountMinor,
      afterTaxAmountMinor,
      effectiveAt: row.effectiveAt || new Date().toISOString()
    }
  };
}

function previewIsError(row: EditableRow): boolean {
  return !normalizeRow(row).ok;
}

function previewText(row: EditableRow): string {
  const result = normalizeRow(row);
  if (!result.ok) {
    return result.message;
  }
  return `preTax=${result.value.preTaxAmountMinor} / tax=${result.value.taxAmountMinor} / afterTax=${result.value.afterTaxAmountMinor} (minor)`;
}

async function saveRows() {
  // 前端仍用 normalizeRow 做 currency_specs 预校验与舍入预览，但下发 major 字符串 + taxRate 字符串，
  // 归一化为 minor 的职责在后端（compact §价格行 / 00 §5）。
  const payloadRows: PutPriceRow[] = [];
  for (let i = 0; i < rows.value.length; i += 1) {
    const row = rows.value[i];
    const result = normalizeRow(row);
    if (!result.ok) {
      ElMessage.warning(`第 ${i + 1} 行：${result.message}`);
      return;
    }
    payloadRows.push({
      countryCode: row.countryCode.trim().toUpperCase(),
      regionCode: row.regionCode.trim() || "*",
      currency: row.currency,
      priceId: row.priceId.trim(),
      preTaxAmount: row.preTaxMajorInput.trim(),
      taxRate: String(row.taxRate),
      effectiveAt: row.effectiveAt || new Date().toISOString()
    });
  }

  saving.value = true;
  try {
    await putCashierPriceRows(props.templateId, props.version, { rows: payloadRows });
    ElMessage.success("价格矩阵已保存");
    emit("saved");
    await loadRows(props.templateId, props.version);
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "保存价格矩阵失败");
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}

.hint {
  margin: 12px 0 0;
  color: var(--text-subtle);
  font-size: 12px;
}

.preview__ok,
.preview__err {
  margin: 0;
  font-size: 12px;
}

.preview__ok {
  color: #0f766e;
}

.preview__err {
  color: #b42318;
}

.empty-state {
  padding: 36px 0;
  text-align: center;
}

.empty-state__title {
  margin: 0;
  font-weight: 600;
}

.empty-state__hint {
  margin: 8px 0 0;
  color: var(--text-subtle);
}
</style>
