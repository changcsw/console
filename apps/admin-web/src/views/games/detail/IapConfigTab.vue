<template>
  <div class="iap-tab" v-loading="loadingChannels">
    <div class="iap-toolbar">
      <el-select v-model="activeGameChannelId" class="selector" filterable placeholder="选择渠道实例" @change="onGameChannelChange">
        <el-option
          v-for="item in gameChannels"
          :key="item.gameChannelId"
          :label="`${item.market} / ${item.channelId}`"
          :value="item.gameChannelId"
        />
      </el-select>
      <el-select v-model="activePackageId" class="selector" filterable clearable placeholder="选择渠道包（可选）" @change="onPackageChange">
        <el-option
          v-for="pkg in packageOptions"
          :key="pkg.packageId"
          :label="`${pkg.packageCode} - ${pkg.packageName}`"
          :value="pkg.packageId"
        />
      </el-select>
      <el-button @click="reloadCurrent">刷新</el-button>
    </div>

    <el-alert
      v-if="!canWrite"
      type="info"
      :closable="false"
      title="当前账号仅有查看权限，配置项已置灰。"
      class="panel__alert"
    />
    <el-alert
      v-if="!gameChannels.length && !loadingChannels"
      type="info"
      :closable="false"
      title="当前游戏暂无渠道实例，无法配置 IAP。"
    />

    <section v-if="channelConfig" class="panel">
      <header class="panel__head">
        <h4 class="panel__title">渠道 IAP 配置</h4>
        <div class="panel__meta">
          <PageStatusTag :tone="statusTone(channelConfig.config.configStatus)" :label="channelConfig.config.configStatus" />
          <el-switch v-model="channelEnabled" :disabled="!canWrite" active-text="启用" inactive-text="停用" />
        </div>
      </header>
      <p v-if="channelConfig.config.lastCheckMessage" :class="statusClass(channelConfig.config.configStatus)">
        {{ channelConfig.config.lastCheckMessage }}
      </p>

      <TemplateConfigRenderer
        v-model="channelDraftConfig"
        v-model:secret-values="channelSecretInputs"
        :template="channelConfig.template"
        :disabled="!canWrite"
        @json-error-change="(v) => (channelJsonError = v)"
      />

      <div class="panel__actions">
        <el-button v-perm="'product.write'" type="primary" :loading="savingChannel" @click="saveChannelConfig">
          保存渠道 IAP 配置
        </el-button>
      </div>
    </section>

    <section v-if="packageOverride" class="panel">
      <header class="panel__head">
        <h4 class="panel__title">包级 IAP 覆盖</h4>
        <PageStatusTag :tone="statusTone(packageOverride.override.configStatus)" :label="packageOverride.override.configStatus" />
      </header>

      <el-alert
        type="info"
        :closable="false"
        title="未启用时回退渠道配置。"
        class="panel__alert"
      />

      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="渠道基线 enabled">{{ packageOverride.baseConfig.enabled ? "是" : "否" }}</el-descriptions-item>
        <el-descriptions-item label="渠道基线状态">{{ packageOverride.baseConfig.configStatus }}</el-descriptions-item>
        <el-descriptions-item label="渠道基线校验信息" :span="2">
          {{ packageOverride.baseConfig.lastCheckMessage || "—" }}
        </el-descriptions-item>
      </el-descriptions>

      <div class="override-switch">
        <span>启用包级覆盖</span>
        <el-switch v-model="packageEnabled" :disabled="!canWrite" />
      </div>
      <p v-if="packageOverride.override.lastCheckMessage" :class="statusClass(packageOverride.override.configStatus)">
        {{ packageOverride.override.lastCheckMessage }}
      </p>

      <TemplateConfigRenderer
        v-model="packageDraftConfig"
        v-model:secret-values="packageSecretInputs"
        :template="packageOverride.template"
        :disabled="!canWrite"
        @json-error-change="(v) => (packageJsonError = v)"
      />

      <div class="panel__actions">
        <el-button v-perm="'product.write'" type="primary" :loading="savingPackage" @click="savePackageOverride">
          保存包级覆盖
        </el-button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import TemplateConfigRenderer from "./components/TemplateConfigRenderer.vue";
import { ApiError } from "@/api/http";
import { usePermissionStore } from "@/stores/permission";
import { listChannelPackages, listMarketChannels, type ChannelPackage, type MarketChannelListItem } from "@/api/modules/channels";
import {
  getGameChannelIapConfig,
  getPackageIapOverride,
  putGameChannelIapConfig,
  putPackageIapOverride,
  type ConfigStatus,
  type GameChannelIapConfigResponse,
  type PackageIapOverrideResponse
} from "@/api/modules/products";

const props = defineProps<{ gameId: string }>();
const permission = usePermissionStore();
const canWrite = computed(() => permission.hasPerm("product.write"));

const loadingChannels = ref(false);
const gameChannels = ref<MarketChannelListItem[]>([]);
const activeGameChannelId = ref<number | null>(null);
const packages = ref<ChannelPackage[]>([]);
const activePackageId = ref<number | null>(null);

const channelConfig = ref<GameChannelIapConfigResponse | null>(null);
const packageOverride = ref<PackageIapOverrideResponse | null>(null);

const channelEnabled = ref(false);
const channelDraftConfig = ref<Record<string, unknown>>({});
const channelSecretInputs = ref<Record<string, string>>({});
const channelJsonError = ref(false);
const savingChannel = ref(false);

const packageEnabled = ref(false);
const packageDraftConfig = ref<Record<string, unknown>>({});
const packageSecretInputs = ref<Record<string, string>>({});
const packageJsonError = ref(false);
const savingPackage = ref(false);

const packageOptions = computed(() => packages.value.filter((item) => item.enabled));

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

function buildSubmitConfig(
  template: { secretFields: string[] },
  draft: Record<string, unknown>,
  secretInputs: Record<string, string>
): Record<string, unknown> {
  const next: Record<string, unknown> = JSON.parse(JSON.stringify(draft ?? {})) as Record<string, unknown>;
  for (const key of template.secretFields ?? []) {
    delete next[key];
    const value = (secretInputs[key] ?? "").trim();
    if (value) {
      next[key] = value;
    }
  }
  return next;
}

async function loadGameChannels() {
  loadingChannels.value = true;
  try {
    const all: MarketChannelListItem[] = [];
    let currentPage = 1;
    while (true) {
      const res = await listMarketChannels(props.gameId, {
        hidden: undefined,
        page: currentPage,
        pageSize: 100
      });
      all.push(...res.items);
      if (all.length >= res.total || res.items.length === 0) {
        break;
      }
      currentPage += 1;
    }
    gameChannels.value = all;
    if (!activeGameChannelId.value && all.length) {
      activeGameChannelId.value = all[0].gameChannelId;
    }
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载渠道实例失败");
    gameChannels.value = [];
    activeGameChannelId.value = null;
  } finally {
    loadingChannels.value = false;
  }
}

async function loadPackages(gameChannelId: number) {
  try {
    packages.value = await listChannelPackages(gameChannelId);
    if (activePackageId.value && !packages.value.some((item) => item.packageId === activePackageId.value)) {
      activePackageId.value = null;
      packageOverride.value = null;
    }
  } catch {
    packages.value = [];
    activePackageId.value = null;
    packageOverride.value = null;
  }
}

async function loadChannelConfig(gameChannelId: number) {
  try {
    const res = await getGameChannelIapConfig(gameChannelId);
    channelConfig.value = res;
    channelEnabled.value = res.config.enabled;
    channelDraftConfig.value = JSON.parse(JSON.stringify(res.config.configJson ?? {})) as Record<string, unknown>;
    channelSecretInputs.value = {};
    channelJsonError.value = false;
  } catch (err) {
    channelConfig.value = null;
    ElMessage.error(err instanceof ApiError ? err.message : "加载渠道 IAP 配置失败");
  }
}

async function loadPackageOverride(packageId: number) {
  try {
    const res = await getPackageIapOverride(packageId);
    packageOverride.value = res;
    packageEnabled.value = res.override.enabled;
    packageDraftConfig.value = JSON.parse(JSON.stringify(res.override.configJson ?? {})) as Record<string, unknown>;
    packageSecretInputs.value = {};
    packageJsonError.value = false;
  } catch (err) {
    packageOverride.value = null;
    ElMessage.error(err instanceof ApiError ? err.message : "加载包级 IAP 覆盖失败");
  }
}

async function onGameChannelChange() {
  if (!activeGameChannelId.value) {
    return;
  }
  await Promise.all([loadChannelConfig(activeGameChannelId.value), loadPackages(activeGameChannelId.value)]);
}

async function onPackageChange() {
  if (!activePackageId.value) {
    packageOverride.value = null;
    return;
  }
  await loadPackageOverride(activePackageId.value);
}

async function reloadCurrent() {
  await loadGameChannels();
  if (activeGameChannelId.value) {
    await onGameChannelChange();
  }
  if (activePackageId.value) {
    await onPackageChange();
  }
}

async function saveChannelConfig() {
  if (!activeGameChannelId.value || !channelConfig.value) {
    return;
  }
  if (channelJsonError.value) {
    ElMessage.warning("请先修复 JSON 字段格式错误");
    return;
  }
  savingChannel.value = true;
  try {
    const next = await putGameChannelIapConfig(activeGameChannelId.value, {
      enabled: channelEnabled.value,
      configJson: buildSubmitConfig(channelConfig.value.template, channelDraftConfig.value, channelSecretInputs.value)
    });
    channelConfig.value = { ...channelConfig.value, config: next };
    channelDraftConfig.value = JSON.parse(JSON.stringify(next.configJson ?? {})) as Record<string, unknown>;
    channelSecretInputs.value = {};
    ElMessage.success("渠道 IAP 配置已保存");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "保存渠道 IAP 配置失败");
  } finally {
    savingChannel.value = false;
  }
}

async function savePackageOverride() {
  if (!activePackageId.value || !packageOverride.value) {
    return;
  }
  if (packageJsonError.value) {
    ElMessage.warning("请先修复 JSON 字段格式错误");
    return;
  }
  savingPackage.value = true;
  try {
    const next = await putPackageIapOverride(activePackageId.value, {
      enabled: packageEnabled.value,
      configJson: buildSubmitConfig(packageOverride.value.template, packageDraftConfig.value, packageSecretInputs.value)
    });
    packageOverride.value = { ...packageOverride.value, override: next };
    packageDraftConfig.value = JSON.parse(JSON.stringify(next.configJson ?? {})) as Record<string, unknown>;
    packageSecretInputs.value = {};
    ElMessage.success("包级 IAP 覆盖已保存");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "保存包级 IAP 覆盖失败");
  } finally {
    savingPackage.value = false;
  }
}

onMounted(async () => {
  await loadGameChannels();
  if (activeGameChannelId.value) {
    await onGameChannelChange();
  }
});
</script>

<style scoped>
.iap-tab {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.iap-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.selector {
  width: 260px;
}

.panel {
  border: 1px solid var(--panel-border);
  border-radius: var(--radius-sm);
  padding: 14px;
}

.panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.panel__title {
  margin: 0;
  font-size: 15px;
}

.panel__meta {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-text {
  margin: 0 0 10px;
  color: var(--text-subtle);
  font-size: 12px;
}

.status-text--warning {
  color: var(--danger);
}

.panel__actions {
  margin-top: 10px;
  display: flex;
  justify-content: flex-end;
}

.panel__alert {
  margin-bottom: 10px;
}

.override-switch {
  margin: 10px 0;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}
</style>
