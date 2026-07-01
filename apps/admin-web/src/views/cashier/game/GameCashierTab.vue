<template>
  <div class="game-cashier-tab">
    <el-alert
      v-if="!canWrite"
      type="info"
      :closable="false"
      title="当前账号仅有查看权限，绑定版本与覆盖编辑入口已置灰。"
    />
    <el-alert v-if="loadError" type="error" :closable="false" :title="loadError" />

    <PageCard title="模板绑定快照" description="游戏绑定模板的 published 版本快照；后续模板发布不会自动生效，需手动切换/升级。">
      <div class="profile-toolbar">
        <el-space wrap>
          <PageStatusTag :tone="profile ? 'success' : 'warning'" :label="profile ? '已绑定模板' : '未绑定模板'" />
          <EnvironmentBadge :environment="app.environment" />
        </el-space>
      </div>

      <template v-if="profile">
        <el-descriptions :column="4" border size="small">
          <el-descriptions-item label="模板 ID">{{ profile.templateId }}</el-descriptions-item>
          <el-descriptions-item label="模板版本">{{ profile.appliedTemplateVersion }}</el-descriptions-item>
          <el-descriptions-item label="绑定时间">{{ formatDateTime(profile.appliedAt) }}</el-descriptions-item>
          <el-descriptions-item label="校验和">
            <code>{{ profile.snapshotChecksum || "-" }}</code>
          </el-descriptions-item>
        </el-descriptions>
      </template>
      <template v-else>
        <div class="empty-state">
          <p class="empty-state__title">尚未绑定收银台模板</p>
          <p class="empty-state__hint">先选择模板与发布版本，再执行「切换/升级版本」。</p>
        </div>
      </template>

      <div class="bind-box">
        <el-form inline label-position="top">
          <el-form-item label="模板">
            <el-select v-model="bindForm.templateId" :disabled="savingProfile || !canWrite" filterable class="w-220">
              <el-option
                v-for="item in templates"
                :key="item.templateId"
                :label="`${item.templateId} · ${item.templateName}`"
                :value="item.templateId"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="模板版本（published）">
            <el-select v-model="bindForm.templateVersion" :disabled="savingProfile || !canWrite" filterable class="w-220">
              <el-option
                v-for="item in bindableVersions"
                :key="item.version"
                :label="`v${item.version}`"
                :value="item.version"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="操作">
            <el-button
              v-perm="'cashier.write'"
              type="primary"
              :loading="savingProfile"
              :disabled="savingProfile || !bindForm.templateId || !bindForm.templateVersion"
              @click="saveProfile"
            >
              切换/升级版本
            </el-button>
          </el-form-item>
        </el-form>
      </div>
    </PageCard>

    <PageCard title="价格边界视图" description="清晰区分模板公共矩阵与游戏级覆盖。高亮行表示游戏覆盖已生效。">
      <el-table v-loading="loadingMatrix" :data="displayRows" border size="small" :row-class-name="matrixRowClassName">
        <el-table-column label="来源" width="120">
          <template #default="{ row }">
            <el-tag :type="row.overridden ? 'warning' : 'info'" size="small">
              {{ row.overridden ? "游戏覆盖" : "模板公共矩阵" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Country/Region" min-width="150">
          <template #default="{ row }">{{ row.countryCode }} / {{ row.regionCode }}</template>
        </el-table-column>
        <el-table-column prop="currency" label="Currency" width="110" />
        <el-table-column prop="priceId" label="Price ID" min-width="160" />
        <el-table-column label="税前金额" min-width="150">
          <template #default="{ row }">{{ row.preTaxDisplay }}</template>
        </el-table-column>
        <el-table-column label="税后金额" min-width="150">
          <template #default="{ row }">{{ row.afterTaxDisplay }}</template>
        </el-table-column>
        <el-table-column label="税率" width="110">
          <template #default="{ row }">{{ row.taxRate }}</template>
        </el-table-column>
        <el-table-column label="生效时间" min-width="180">
          <template #default="{ row }">{{ formatDateTime(row.effectiveAt) }}</template>
        </el-table-column>
      </el-table>
      <div v-if="!displayRows.length && !loadingMatrix" class="empty-state">
        <p class="empty-state__title">暂无价格矩阵数据</p>
        <p class="empty-state__hint">绑定模板后可查看公共矩阵与游戏覆盖叠加结果。</p>
      </div>
    </PageCard>

    <PageCard title="游戏级价格覆盖" description="整行覆盖模板同键价格；金额遵循 currency_specs 精度/下限/舍入。">
      <div class="override-toolbar">
        <el-space wrap>
          <el-button v-perm="'cashier.write'" :disabled="!canWrite || savingOverrides" @click="addOverrideRow">新增覆盖行</el-button>
          <el-button
            v-perm="'cashier.write'"
            type="primary"
            :loading="savingOverrides"
            :disabled="!canWrite || savingOverrides"
            @click="saveOverrides"
          >
            保存覆盖
          </el-button>
        </el-space>
      </div>
      <el-table v-loading="loadingOverrides" :data="overrideRows" border size="small">
        <el-table-column label="Country" min-width="120">
          <template #default="{ row }">
            <el-input v-model.trim="row.countryCode" :disabled="!canWrite || savingOverrides" placeholder="US" />
          </template>
        </el-table-column>
        <el-table-column label="Region" min-width="120">
          <template #default="{ row }">
            <el-input v-model.trim="row.regionCode" :disabled="!canWrite || savingOverrides" placeholder="*" />
          </template>
        </el-table-column>
        <el-table-column label="Currency" width="120">
          <template #default="{ row }">
            <el-select v-model="row.currency" :disabled="!canWrite || savingOverrides" filterable>
              <el-option
                v-for="spec in dictionary.enabledCurrencySpecs"
                :key="spec.currencyCode"
                :label="spec.currencyCode"
                :value="spec.currencyCode"
              />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="Price ID" min-width="150">
          <template #default="{ row }">
            <el-input v-model.trim="row.priceId" :disabled="!canWrite || savingOverrides" />
          </template>
        </el-table-column>
        <el-table-column label="税前金额(major)" min-width="160">
          <template #default="{ row }">
            <el-input v-model.trim="row.preTaxMajorInput" :disabled="!canWrite || savingOverrides" placeholder="9.99" />
          </template>
        </el-table-column>
        <el-table-column label="Tax Rate" width="120">
          <template #default="{ row }">
            <el-input-number
              v-model="row.taxRate"
              :disabled="!canWrite || savingOverrides"
              :min="0"
              :max="1"
              :precision="6"
              :step="0.01"
              :controls="false"
            />
          </template>
        </el-table-column>
        <el-table-column label="原因" min-width="150">
          <template #default="{ row }">
            <el-input v-model.trim="row.reason" :disabled="!canWrite || savingOverrides" maxlength="255" show-word-limit />
          </template>
        </el-table-column>
        <el-table-column label="生效时间" min-width="180">
          <template #default="{ row }">
            <el-date-picker
              v-model="row.effectiveAt"
              :disabled="!canWrite || savingOverrides"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm:ss[Z]"
              placeholder="选择生效时间"
            />
          </template>
        </el-table-column>
        <el-table-column label="舍入预览(minor)" min-width="260">
          <template #default="{ row }">
            <p :class="previewIsError(row) ? 'preview__err' : 'preview__ok'">{{ previewText(row) }}</p>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="86" fixed="right">
          <template #default="{ $index }">
            <el-button v-perm="'cashier.write'" :disabled="!canWrite || savingOverrides" link type="danger" @click="removeOverrideRow($index)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <p class="hint">模板公共矩阵与游戏覆盖采用整行覆盖语义；同键覆盖行在上方边界表中高亮展示。</p>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import { ApiError } from "@/api/http";
import {
  getCashierPriceRows,
  getCashierTemplate,
  getGameCashierPriceOverrides,
  getGameCashierProfile,
  listCashierTemplates,
  putGameCashierPriceOverrides,
  putGameCashierProfile,
  type CashierPriceRow,
  type CashierTemplateSummary,
  type CashierTemplateVersion,
  type GameCashierPriceOverride,
  type GameCashierProfile
} from "@/api/modules/cashier";
import { useAppStore } from "@/stores/app";
import { useDictionaryStore } from "@/stores/dictionary";
import { usePermissionStore } from "@/stores/permission";

interface EditableOverrideRow {
  countryCode: string;
  regionCode: string;
  currency: string;
  priceId: string;
  preTaxMajorInput: string;
  taxRate: number;
  reason: string;
  effectiveAt: string;
}

interface DisplayRow {
  key: string;
  countryCode: string;
  regionCode: string;
  currency: string;
  priceId: string;
  preTaxDisplay: string;
  afterTaxDisplay: string;
  taxRate: number;
  effectiveAt: string;
  overridden: boolean;
}

const props = defineProps<{ gameId: string }>();

const app = useAppStore();
const dictionary = useDictionaryStore();
const permission = usePermissionStore();
const canWrite = computed(() => permission.hasPerm("cashier.write"));

const loadError = ref("");
const loadingProfile = ref(false);
const loadingOverrides = ref(false);
const loadingMatrix = ref(false);
const savingProfile = ref(false);
const savingOverrides = ref(false);

const templates = ref<CashierTemplateSummary[]>([]);
const templateVersions = ref<CashierTemplateVersion[]>([]);
const profile = ref<GameCashierProfile | null>(null);
const overrideRows = ref<EditableOverrideRow[]>([]);
const templateRows = ref<CashierPriceRow[]>([]);

const bindForm = reactive({
  templateId: "",
  templateVersion: ""
});

const bindableVersions = computed(() => templateVersions.value.filter((item) => item.status === "published"));

const displayRows = computed<DisplayRow[]>(() => {
  const merged = new Map<string, DisplayRow>();
  for (const row of templateRows.value) {
    const key = toRowKey(row.countryCode, row.regionCode, row.currency, row.priceId);
    merged.set(key, {
      key,
      countryCode: row.countryCode,
      regionCode: row.regionCode || "*",
      currency: row.currency,
      priceId: row.priceId,
      preTaxDisplay: formatMinorWithCurrency(row.preTaxAmountMinor, row.currency),
      afterTaxDisplay: formatMinorWithCurrency(row.afterTaxAmountMinor, row.currency),
      taxRate: row.taxRate,
      effectiveAt: row.effectiveAt,
      overridden: false
    });
  }
  for (const row of overrideRows.value) {
    const normalized = normalizeOverrideRow(row);
    if (!normalized.ok) {
      continue;
    }
    const key = toRowKey(normalized.value.countryCode, normalized.value.regionCode, normalized.value.currency, normalized.value.priceId);
    merged.set(key, {
      key,
      countryCode: normalized.value.countryCode,
      regionCode: normalized.value.regionCode,
      currency: normalized.value.currency,
      priceId: normalized.value.priceId,
      preTaxDisplay: formatMinorWithCurrency(normalized.value.preTaxAmountMinor, normalized.value.currency),
      afterTaxDisplay: formatMinorWithCurrency(normalized.value.afterTaxAmountMinor, normalized.value.currency),
      taxRate: Number(normalized.value.taxRate) || 0,
      effectiveAt: normalized.value.effectiveAt,
      overridden: true
    });
  }
  return Array.from(merged.values()).sort((a, b) => a.key.localeCompare(b.key));
});

watch(
  () => bindForm.templateId,
  async (templateId) => {
    bindForm.templateVersion = "";
    templateVersions.value = [];
    if (!templateId) {
      return;
    }
    try {
      const detail = await getCashierTemplate(templateId);
      templateVersions.value = detail.versions;
      const preferred = detail.versions.find((item) => item.status === "published");
      bindForm.templateVersion = preferred?.version ?? "";
    } catch (err) {
      ElMessage.error(err instanceof ApiError ? err.message : "加载模板版本失败");
    }
  }
);

watch(
  () => props.gameId,
  () => {
    void loadAll();
  },
  { immediate: true }
);

onMounted(async () => {
  await dictionary.ensureCurrencySpecs();
});

function toRowKey(countryCode: string, regionCode: string, currency: string, priceId: string): string {
  return `${countryCode}|${regionCode || "*"}|${currency}|${priceId}`;
}

function formatDateTime(value?: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
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

function formatMinorWithCurrency(minor: number, currencyCode: string): string {
  const spec = dictionary.getCurrencySpec(currencyCode);
  if (!spec) {
    return `${minor} minor`;
  }
  return `${formatMinorToMajor(minor, spec.decimalPlaces)} ${currencyCode}`;
}

function fromMinorToEditable(row: GameCashierPriceOverride): EditableOverrideRow {
  const spec = dictionary.getCurrencySpec(row.currency);
  const major = spec ? formatMinorToMajor(row.preTaxAmountMinor, spec.decimalPlaces) : String(row.preTaxAmountMinor);
  return {
    countryCode: row.countryCode,
    regionCode: row.regionCode || "*",
    currency: row.currency,
    priceId: row.priceId,
    preTaxMajorInput: major,
    taxRate: Number(row.taxRate) || 0,
    reason: row.reason ?? "",
    effectiveAt: row.effectiveAt
  };
}

type NormalizeResult =
  | { ok: true; value: GameCashierPriceOverride }
  | { ok: false; message: string };

function normalizeOverrideRow(row: EditableOverrideRow): NormalizeResult {
  if (!row.countryCode || !row.priceId || !row.currency || !row.preTaxMajorInput) {
    return { ok: false, message: "请填写 country/priceId/currency/金额" };
  }
  const spec = dictionary.getCurrencySpec(row.currency);
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

  const preTaxAmountMinor = roundByMode(amount * 10 ** spec.decimalPlaces, spec.roundingMode);
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
      taxRate: String(row.taxRate),
      taxAmountMinor,
      afterTaxAmountMinor,
      reason: row.reason.trim(),
      effectiveAt: row.effectiveAt || new Date().toISOString()
    }
  };
}

function previewIsError(row: EditableOverrideRow): boolean {
  return !normalizeOverrideRow(row).ok;
}

function previewText(row: EditableOverrideRow): string {
  const result = normalizeOverrideRow(row);
  if (!result.ok) {
    return result.message;
  }
  return `preTax=${result.value.preTaxAmountMinor} / tax=${result.value.taxAmountMinor} / afterTax=${result.value.afterTaxAmountMinor}`;
}

function matrixRowClassName({ row }: { row: DisplayRow }): string {
  return row.overridden ? "matrix-row--overridden" : "";
}

function addOverrideRow() {
  overrideRows.value.push({
    countryCode: "",
    regionCode: "*",
    currency: dictionary.enabledCurrencySpecs[0]?.currencyCode ?? "USD",
    priceId: "",
    preTaxMajorInput: "",
    taxRate: 0,
    reason: "",
    effectiveAt: new Date().toISOString()
  });
}

function removeOverrideRow(index: number) {
  overrideRows.value.splice(index, 1);
}

async function loadTemplatesAndProfile() {
  loadingProfile.value = true;
  try {
    const [templateRes, profileRes] = await Promise.all([
      listCashierTemplates({ page: 1, pageSize: 100 }),
      getGameCashierProfile(props.gameId)
    ]);
    templates.value = templateRes.items ?? [];
    profile.value = profileRes;
    bindForm.templateId = profileRes?.templateId ?? templates.value[0]?.templateId ?? "";
    if (bindForm.templateId) {
      const detail = await getCashierTemplate(bindForm.templateId);
      templateVersions.value = detail.versions;
      if (profileRes?.templateId === bindForm.templateId) {
        bindForm.templateVersion = profileRes.appliedTemplateVersion;
      } else {
        bindForm.templateVersion = detail.versions.find((item) => item.status === "published")?.version ?? "";
      }
    }
  } finally {
    loadingProfile.value = false;
  }
}

async function loadPriceRows() {
  loadingOverrides.value = true;
  loadingMatrix.value = true;
  try {
    const [overridesRes, rowsRes] = await Promise.all([
      getGameCashierPriceOverrides(props.gameId),
      profile.value ? getCashierPriceRows(profile.value.templateId, profile.value.appliedTemplateVersion) : Promise.resolve({ items: [] })
    ]);
    overrideRows.value = (overridesRes.items ?? []).map(fromMinorToEditable);
    templateRows.value = rowsRes.items ?? [];
  } finally {
    loadingOverrides.value = false;
    loadingMatrix.value = false;
  }
}

async function loadAll() {
  loadError.value = "";
  try {
    await loadTemplatesAndProfile();
    await loadPriceRows();
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : "加载收银台信息失败";
    profile.value = null;
    templateRows.value = [];
    overrideRows.value = [];
  }
}

async function saveProfile() {
  if (!bindForm.templateId || !bindForm.templateVersion) {
    ElMessage.warning("请选择模板与发布版本");
    return;
  }
  savingProfile.value = true;
  try {
    profile.value = await putGameCashierProfile(props.gameId, {
      templateId: bindForm.templateId,
      templateVersion: bindForm.templateVersion
    });
    ElMessage.success("模板版本绑定已更新");
    await loadPriceRows();
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "更新模板绑定失败");
  } finally {
    savingProfile.value = false;
  }
}

async function saveOverrides() {
  const payloadItems: GameCashierPriceOverride[] = [];
  for (let i = 0; i < overrideRows.value.length; i += 1) {
    const result = normalizeOverrideRow(overrideRows.value[i]);
    if (!result.ok) {
      ElMessage.warning(`第 ${i + 1} 行：${result.message}`);
      return;
    }
    payloadItems.push(result.value);
  }
  savingOverrides.value = true;
  try {
    const res = await putGameCashierPriceOverrides(props.gameId, { items: payloadItems });
    overrideRows.value = (res.items ?? []).map(fromMinorToEditable);
    ElMessage.success("游戏级覆盖已保存");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "保存覆盖失败");
  } finally {
    savingOverrides.value = false;
  }
}
</script>

<style scoped>
.game-cashier-tab {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.profile-toolbar {
  margin-bottom: 12px;
}

.bind-box {
  margin-top: 12px;
}

.override-toolbar {
  margin-bottom: 12px;
}

.empty-state {
  padding: 24px 0;
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

.w-220 {
  width: 220px;
}

:deep(.matrix-row--overridden) {
  --el-table-tr-bg-color: #fff7ed;
}
</style>
