<template>
  <el-drawer
    :model-value="open"
    :title="`渠道包详情 · ${pkg?.packageCode ?? ''}`"
    size="760px"
    :close-on-click-modal="false"
    @update:model-value="(v: boolean) => !v && emit('close')"
  >
    <div v-loading="loading" class="package-drawer">
      <el-alert
        v-if="!canWrite"
        type="info"
        :closable="false"
        title="当前账号仅有查看权限，配置项已置灰。"
        class="panel__alert"
      />
      <el-descriptions v-if="pkg" :column="2" border size="small">
        <el-descriptions-item label="包标识">{{ pkg.packageCode }}</el-descriptions-item>
        <el-descriptions-item label="包名称">{{ pkg.packageName }}</el-descriptions-item>
        <el-descriptions-item label="Bundle ID">{{ pkg.bundleId || "—" }}</el-descriptions-item>
        <el-descriptions-item label="启用">{{ pkg.enabled ? "是" : "否" }}</el-descriptions-item>
      </el-descriptions>

      <section class="panel">
        <header class="panel__head">
          <h4 class="panel__title">商品映射</h4>
          <el-button v-perm="'product.write'" type="primary" :loading="savingProducts" @click="saveMappings">保存商品映射</el-button>
        </header>
        <el-alert
          type="info"
          :closable="false"
          title="product_id 与 price_id 两维独立，模式互不联动。"
          class="panel__alert"
        />

        <div v-if="!productRows.length" class="empty">该渠道包暂无商品映射。</div>
        <article v-for="row in productRows" :key="row.productId" class="mapping-row">
          <header class="mapping-row__head">
            <div>
              <b>{{ row.productName }}</b>
              <code class="mono">{{ row.productId }}</code>
            </div>
            <el-switch v-model="row.enabled" :disabled="!canWrite" active-text="启用" inactive-text="停用" />
          </header>

          <div class="mapping-grid">
            <section class="mapping-col mapping-col--product">
              <h5>IAP 商品 ID</h5>
              <p class="base-line">base.productId: <code>{{ row.base.productId }}</code></p>
              <el-radio-group v-model="row.productIdMode" :disabled="!canWrite" @change="onModeChange(row, 'product')">
                <el-radio label="default">回退基准</el-radio>
                <el-radio label="override">覆盖</el-radio>
              </el-radio-group>
              <el-input
                v-model="row.productIdOverride"
                maxlength="128"
                show-word-limit
                :disabled="!canWrite || row.productIdMode === 'default'"
                class="input-product"
                placeholder="覆盖 IAP 商品 ID（最多 128）"
              />
              <p class="effective">effective.productId: <code>{{ effectiveProductId(row) }}</code></p>
            </section>

            <section class="mapping-col mapping-col--price">
              <h5>收银台价格档</h5>
              <p class="base-line">base.priceId: <code>{{ row.base.priceId }}</code></p>
              <el-radio-group v-model="row.priceIdMode" :disabled="!canWrite" @change="onModeChange(row, 'price')">
                <el-radio label="default">回退基准</el-radio>
                <el-radio label="override">覆盖</el-radio>
              </el-radio-group>
              <el-input
                v-model="row.priceIdOverride"
                maxlength="64"
                show-word-limit
                :disabled="!canWrite || row.priceIdMode === 'default'"
                class="input-price"
                placeholder="覆盖收银台价格档（最多 64）"
              />
              <p class="effective">effective.priceId: <code>{{ effectivePriceId(row) }}</code></p>
            </section>
          </div>
        </article>
      </section>

      <section class="panel">
        <header class="panel__head">
          <h4 class="panel__title">功能插件覆盖</h4>
        </header>
        <el-alert
          type="info"
          :closable="false"
          title="支持“继承渠道插件 / 自定义覆盖”切换；仅 plugin.write 可编辑。"
          class="panel__alert"
        />
        <div v-if="!packagePlugins.length" class="empty">该渠道包暂无插件覆盖项。</div>
        <article v-for="plugin in packagePlugins" :key="plugin.pluginId" class="plugin-row">
          <header class="plugin-row__head">
            <div class="plugin-row__title">
              <b>{{ plugin.pluginName }}</b>
              <code>{{ plugin.pluginId }}</code>
              <el-tag v-if="plugin.required" size="small" type="danger">必接</el-tag>
              <el-tag v-if="plugin.locked" size="small" type="warning">锁定</el-tag>
              <el-tag size="small" :type="plugin.includedInRuntimeConfig ? 'success' : 'info'">
                {{ plugin.includedInRuntimeConfig ? "进入最终配置" : "未进入最终配置" }}
              </el-tag>
              <PageStatusTag :tone="pluginStatusTone(plugin.configStatus)" :label="pluginStatusLabel(plugin.configStatus)" />
            </div>
            <div class="plugin-row__switch">
              <el-switch
                v-model="plugin.inheritChannelConfig"
                :disabled="!pluginCanEdit(plugin)"
                active-text="继承渠道插件"
                inactive-text="自定义覆盖"
              />
              <el-switch v-model="plugin.enabled" :disabled="!pluginCanEdit(plugin)" active-text="启用" inactive-text="停用" />
            </div>
          </header>

          <el-alert
            v-if="plugin.locked"
            type="info"
            :closable="false"
            title="该插件已锁定，渠道包不可编辑。"
            class="panel__alert"
          />
          <p class="status-text">{{ plugin.lastCheckMessage || "最近校验：-" }}</p>

          <TemplateConfigRenderer
            v-if="!plugin.inheritChannelConfig"
            :model-value="pluginDraft(plugin)"
            :secret-values="pluginSecrets(plugin)"
            :template="pluginTemplateOf(plugin)"
            :disabled="!pluginCanEdit(plugin)"
            @update:model-value="(v) => updatePluginDraft(plugin, v)"
            @update:secret-values="(v) => updatePluginSecrets(plugin, v)"
            @json-error-change="(v) => onPluginJsonError(plugin, v)"
          />

          <el-button
            v-perm="'plugin.write'"
            type="primary"
            :loading="savingPluginId === plugin.pluginId"
            :disabled="!pluginCanEdit(plugin)"
            @click="savePackagePlugin(plugin)"
          >
            保存插件覆盖
          </el-button>
        </article>
      </section>

      <section v-if="iapOverride" class="panel">
        <header class="panel__head">
          <h4 class="panel__title">IAP 覆盖</h4>
          <el-button v-perm="'product.write'" type="primary" :loading="savingIap" @click="saveIapOverride">保存 IAP 覆盖</el-button>
        </header>
        <el-alert
          type="info"
          :closable="false"
          title="未启用时回退渠道配置。"
          class="panel__alert"
        />
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="渠道基线 enabled">{{ iapOverride.baseConfig.enabled ? "是" : "否" }}</el-descriptions-item>
          <el-descriptions-item label="渠道基线状态">{{ iapOverride.baseConfig.configStatus }}</el-descriptions-item>
          <el-descriptions-item label="基线校验信息" :span="2">
            {{ iapOverride.baseConfig.lastCheckMessage || "—" }}
          </el-descriptions-item>
        </el-descriptions>

        <div class="override-switch">
          <span>启用包级 IAP 覆盖</span>
          <el-switch v-model="overrideEnabled" :disabled="!canWrite" />
          <PageStatusTag :tone="statusTone(iapOverride.override.configStatus)" :label="iapOverride.override.configStatus" />
        </div>
        <p v-if="iapOverride.override.lastCheckMessage" :class="statusClass(iapOverride.override.configStatus)">
          {{ iapOverride.override.lastCheckMessage }}
        </p>

        <TemplateConfigRenderer
          v-model="overrideDraftConfig"
          v-model:secret-values="overrideSecretInputs"
          :template="iapOverride.template"
          :disabled="!canWrite"
          @json-error-change="(v) => (jsonError = v)"
        />
      </section>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { usePermissionStore } from "@/stores/permission";
import {
  listChannelPackagePlugins,
  upsertChannelPackagePlugin,
  type ChannelPackage,
  type ChannelPackagePluginItem
} from "@/api/modules/channels";
import {
  getPackageIapOverride,
  getPackageProducts,
  putPackageIapOverride,
  putPackageProducts,
  type ConfigStatus,
  type IapTemplate,
  type PackageIapOverrideResponse,
  type PackageProductItem
} from "@/api/modules/products";
import { ApiError } from "@/api/http";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import TemplateConfigRenderer from "@/views/games/detail/components/TemplateConfigRenderer.vue";
import { configStatusMeta } from "../constants";

type EditableRow = PackageProductItem;

const props = defineProps<{
  open: boolean;
  pkg: ChannelPackage | null;
}>();

const emit = defineEmits<{
  (e: "close"): void;
}>();

const permission = usePermissionStore();
const canWrite = computed(() => permission.hasPerm("product.write"));
const canPluginWrite = computed(() => permission.hasPerm("plugin.write"));

const loading = ref(false);
const savingProducts = ref(false);
const savingIap = ref(false);
const savingPluginId = ref("");
const jsonError = ref(false);

const productRows = ref<EditableRow[]>([]);
const iapOverride = ref<PackageIapOverrideResponse | null>(null);
const packagePlugins = ref<ChannelPackagePluginItem[]>([]);
const overrideEnabled = ref(false);
const overrideDraftConfig = ref<Record<string, unknown>>({});
const overrideSecretInputs = ref<Record<string, string>>({});
const pluginDraftConfig = ref<Record<string, Record<string, unknown>>>({});
const pluginSecretInputs = ref<Record<string, Record<string, string>>>({});
const pluginJsonError = ref<Record<string, boolean>>({});

function statusTone(status: ConfigStatus): "neutral" | "success" | "warning" {
  if (status === "valid") {
    return "success";
  }
  if (status === "invalid") {
    return "warning";
  }
  return "neutral";
}

function statusClass(status: ConfigStatus): string {
  return status === "invalid" ? "status-text status-text--warning" : "status-text";
}

function pluginStatusTone(status: ConfigStatus): "neutral" | "success" | "warning" | "danger" {
  return configStatusMeta(status).tone;
}

function pluginStatusLabel(status: ConfigStatus): string {
  return configStatusMeta(status).label;
}

function pluginTemplateOf(item: ChannelPackagePluginItem): IapTemplate {
  return {
    templateVersion: item.template.templateVersion,
    formSchema: item.template.formSchemaJson,
    secretFields: item.template.secretFieldsJson,
    fileFields: item.template.fileFieldsJson,
    validationRules: item.template.validationRulesJson
  };
}

function pluginCanEdit(item: ChannelPackagePluginItem): boolean {
  return canPluginWrite.value && !item.locked;
}

function buildPluginSubmitConfig(item: ChannelPackagePluginItem): Record<string, unknown> {
  const next: Record<string, unknown> = JSON.parse(JSON.stringify(pluginDraft(item) ?? {})) as Record<string, unknown>;
  for (const key of item.template.secretFieldsJson ?? []) {
    delete next[key];
    const value = (pluginSecrets(item)[key] ?? "").trim();
    if (value) {
      next[key] = value;
    }
  }
  return next;
}

function initPluginDraft(item: ChannelPackagePluginItem) {
  pluginDraftConfig.value[item.pluginId] = JSON.parse(JSON.stringify(item.configJson ?? {})) as Record<string, unknown>;
  pluginSecretInputs.value[item.pluginId] = {};
  pluginJsonError.value[item.pluginId] = false;
}

function pluginDraft(item: ChannelPackagePluginItem): Record<string, unknown> {
  if (!pluginDraftConfig.value[item.pluginId]) {
    initPluginDraft(item);
  }
  return pluginDraftConfig.value[item.pluginId];
}

function pluginSecrets(item: ChannelPackagePluginItem): Record<string, string> {
  if (!pluginSecretInputs.value[item.pluginId]) {
    pluginSecretInputs.value[item.pluginId] = {};
  }
  return pluginSecretInputs.value[item.pluginId];
}

function updatePluginDraft(item: ChannelPackagePluginItem, value: Record<string, unknown>) {
  pluginDraftConfig.value[item.pluginId] = value;
}

function updatePluginSecrets(item: ChannelPackagePluginItem, value: Record<string, string>) {
  pluginSecretInputs.value[item.pluginId] = value;
}

function onPluginJsonError(item: ChannelPackagePluginItem, hasError: boolean) {
  pluginJsonError.value[item.pluginId] = hasError;
}

function effectiveProductId(row: EditableRow): string {
  return row.productIdMode === "override" && row.productIdOverride.trim() ? row.productIdOverride.trim() : row.base.productId;
}

function effectivePriceId(row: EditableRow): string {
  return row.priceIdMode === "override" && row.priceIdOverride.trim() ? row.priceIdOverride.trim() : row.base.priceId;
}

function onModeChange(row: EditableRow, target: "product" | "price") {
  if (target === "product" && row.productIdMode === "default") {
    row.productIdOverride = "";
  }
  if (target === "price" && row.priceIdMode === "default") {
    row.priceIdOverride = "";
  }
}

function buildIapSubmitConfig(): Record<string, unknown> {
  if (!iapOverride.value) {
    return {};
  }
  const next: Record<string, unknown> = JSON.parse(JSON.stringify(overrideDraftConfig.value ?? {})) as Record<string, unknown>;
  for (const key of iapOverride.value.template.secretFields ?? []) {
    delete next[key];
    const value = (overrideSecretInputs.value[key] ?? "").trim();
    if (value) {
      next[key] = value;
    }
  }
  return next;
}

function validateRows(): boolean {
  for (const [index, row] of productRows.value.entries()) {
    if (row.productIdMode === "override") {
      if (!row.productIdOverride.trim()) {
        ElMessage.warning(`第 ${index + 1} 行 IAP 商品 ID 覆盖值不能为空`);
        return false;
      }
      if (row.productIdOverride.trim().length > 128) {
        ElMessage.warning(`第 ${index + 1} 行 IAP 商品 ID 覆盖值不能超过 128 字符`);
        return false;
      }
    }
    if (row.priceIdMode === "override") {
      if (!row.priceIdOverride.trim()) {
        ElMessage.warning(`第 ${index + 1} 行收银台价格档覆盖值不能为空`);
        return false;
      }
      if (row.priceIdOverride.trim().length > 64) {
        ElMessage.warning(`第 ${index + 1} 行收银台价格档覆盖值不能超过 64 字符`);
        return false;
      }
    }
  }
  return true;
}

async function loadData(packageId: number) {
  loading.value = true;
  try {
    const [products, override, plugins] = await Promise.all([
      getPackageProducts(packageId),
      getPackageIapOverride(packageId),
      listChannelPackagePlugins(packageId)
    ]);
    productRows.value = products.map((item) => ({
      ...item,
      productIdOverride: item.productIdMode === "default" ? "" : item.productIdOverride,
      priceIdOverride: item.priceIdMode === "default" ? "" : item.priceIdOverride
    }));
    iapOverride.value = override;
    packagePlugins.value = plugins;
    pluginDraftConfig.value = {};
    pluginSecretInputs.value = {};
    pluginJsonError.value = {};
    for (const item of plugins) {
      initPluginDraft(item);
    }
    overrideEnabled.value = override.override.enabled;
    overrideDraftConfig.value = JSON.parse(JSON.stringify(override.override.configJson ?? {})) as Record<string, unknown>;
    overrideSecretInputs.value = {};
    jsonError.value = false;
  } catch (err) {
    productRows.value = [];
    iapOverride.value = null;
    packagePlugins.value = [];
    ElMessage.error(err instanceof ApiError ? err.message : "加载渠道包详情失败");
  } finally {
    loading.value = false;
  }
}

async function saveMappings() {
  if (!props.pkg) {
    return;
  }
  if (!validateRows()) {
    return;
  }
  savingProducts.value = true;
  try {
    const items = productRows.value.map((row) => ({
      productId: row.productId,
      enabled: row.enabled,
      productIdMode: row.productIdMode,
      productIdOverride: row.productIdMode === "override" ? row.productIdOverride.trim() : "",
      priceIdMode: row.priceIdMode,
      priceIdOverride: row.priceIdMode === "override" ? row.priceIdOverride.trim() : ""
    }));
    const next = await putPackageProducts(props.pkg.packageId, { items });
    productRows.value = next.map((item) => ({
      ...item,
      productIdOverride: item.productIdMode === "default" ? "" : item.productIdOverride,
      priceIdOverride: item.priceIdMode === "default" ? "" : item.priceIdOverride
    }));
    ElMessage.success("商品映射已保存");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "保存商品映射失败");
  } finally {
    savingProducts.value = false;
  }
}

async function saveIapOverride() {
  if (!props.pkg || !iapOverride.value) {
    return;
  }
  if (jsonError.value) {
    ElMessage.warning("请先修复 JSON 字段格式错误");
    return;
  }
  savingIap.value = true;
  try {
    const next = await putPackageIapOverride(props.pkg.packageId, {
      enabled: overrideEnabled.value,
      configJson: buildIapSubmitConfig()
    });
    iapOverride.value = { ...iapOverride.value, override: next };
    overrideDraftConfig.value = JSON.parse(JSON.stringify(next.configJson ?? {})) as Record<string, unknown>;
    overrideSecretInputs.value = {};
    ElMessage.success("包级 IAP 覆盖已保存");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "保存包级 IAP 覆盖失败");
  } finally {
    savingIap.value = false;
  }
}

async function savePackagePlugin(item: ChannelPackagePluginItem) {
  if (!props.pkg) {
    return;
  }
  if (pluginJsonError.value[item.pluginId]) {
    ElMessage.warning("请先修复 JSON 字段格式错误");
    return;
  }
  savingPluginId.value = item.pluginId;
  try {
    const payload = {
      pluginId: item.pluginId,
      inheritChannelConfig: item.inheritChannelConfig,
      enabled: item.enabled,
      config: item.inheritChannelConfig ? {} : buildPluginSubmitConfig(item)
    };
    const saved = await upsertChannelPackagePlugin(props.pkg.packageId, payload);
    packagePlugins.value = packagePlugins.value.map((entry) => (entry.pluginId === item.pluginId ? saved : entry));
    initPluginDraft(saved);
    ElMessage.success("渠道包插件覆盖已保存");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "保存渠道包插件覆盖失败");
  } finally {
    savingPluginId.value = "";
  }
}

watch(
  () => [props.open, props.pkg?.packageId] as const,
  ([open, packageId]) => {
    if (open && packageId) {
      void loadData(packageId);
    }
  }
);
</script>

<style scoped>
.package-drawer {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.panel {
  border: 1px solid var(--panel-border);
  border-radius: var(--radius-sm);
  padding: 12px;
}

.panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.panel__title {
  margin: 0;
  font-size: 15px;
}

.panel__alert {
  margin: 10px 0;
}

.empty {
  color: var(--text-subtle);
  font-size: 13px;
}

.mapping-row {
  border: 1px dashed var(--panel-border);
  border-radius: var(--radius-sm);
  padding: 12px;
  margin-top: 10px;
}

.plugin-row {
  border: 1px dashed var(--panel-border);
  border-radius: var(--radius-sm);
  padding: 12px;
  margin-top: 10px;
}

.plugin-row__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.plugin-row__title {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.plugin-row__switch {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.mapping-row__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.mono {
  margin-left: 8px;
  color: var(--text-subtle);
}

.mapping-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.mapping-col {
  border: 1px solid var(--panel-border);
  border-radius: var(--radius-sm);
  padding: 10px;
}

.mapping-col h5 {
  margin: 0 0 8px;
  font-size: 13px;
}

.mapping-col--product {
  background: #f7fbff;
}

.mapping-col--price {
  background: #fffaf5;
}

.base-line,
.effective {
  margin: 8px 0;
  font-size: 12px;
  color: var(--text-subtle);
}

.input-product :deep(.el-input__wrapper) {
  border-left: 3px solid #3b82f6;
}

.input-price :deep(.el-input__wrapper) {
  border-left: 3px solid #d97706;
}

.override-switch {
  margin: 10px 0;
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-text {
  margin: 0 0 8px;
  color: var(--text-subtle);
  font-size: 12px;
}

.status-text--warning {
  color: var(--danger);
}

@media (max-width: 960px) {
  .mapping-grid {
    grid-template-columns: 1fr;
  }
}
</style>
