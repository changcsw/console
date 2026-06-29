<template>
  <div class="account-auth-tab" v-loading="loading">
    <div class="account-auth-tab__toolbar">
      <PageStatusTag tone="neutral" :label="`认证方式：${rows.length}`" />
      <div class="account-auth-tab__actions">
        <el-button @click="load">刷新</el-button>
        <el-button v-perm="'game.write'" type="primary" :loading="saving" @click="saveAll">保存配置</el-button>
      </div>
    </div>

    <el-alert
      v-if="!canWrite"
      type="info"
      :closable="false"
      title="当前账号仅有查看权限，配置项已置灰。"
      class="account-auth-tab__alert"
    />
    <el-alert
      v-if="loadError"
      type="error"
      :closable="false"
      :title="loadError"
      class="account-auth-tab__alert"
    />

    <div v-if="!rows.length && !loading && !loadError" class="empty-state">
      <p class="empty-state__title">暂无可配置认证方式</p>
      <p class="empty-state__hint">请先在渠道实例中启用支持账号体系的渠道后再试。</p>
    </div>

    <section v-for="row in rows" :key="row.authTypeId" class="auth-card" :class="{ 'is-locked': row.locked }">
      <header class="auth-card__head">
        <div class="auth-card__meta">
          <h4 class="auth-card__title">{{ row.authTypeName }}</h4>
          <code class="auth-card__code">{{ row.authTypeId }}</code>
          <el-tag v-if="row.defaultEnabled" size="small" type="success">默认勾选</el-tag>
          <el-tag v-if="row.locked" size="small" type="warning">锁定</el-tag>
        </div>
        <div class="auth-card__state">
          <PageStatusTag :tone="statusTone(row.configStatus)" :label="row.configStatus" />
          <el-switch
            :model-value="row.enabled"
            :disabled="!canWrite || row.locked"
            inline-prompt
            active-text="启用"
            inactive-text="停用"
            @update:model-value="setEnabled(row, $event)"
          />
        </div>
      </header>

      <p v-if="row.lastCheckMessage" class="auth-card__message">{{ row.lastCheckMessage }}</p>
      <el-alert
        v-if="row.enabled && row.configStatus === 'invalid'"
        type="warning"
        :closable="false"
        title="已启用但配置未通过校验，请补齐必填/敏感/文件字段。"
        class="auth-card__alert"
      />

      <el-form label-position="top" class="auth-card__form">
        <el-form-item v-for="field in sortedFields(row.formSchema)" :key="field.key" :label="field.label || field.key">
          <template v-if="isSecretField(row, field.key)">
            <div class="secret-field">
              <code v-if="hasStoredSecret(row, field.key)" class="secret-field__masked">{{ secretMaskedLabel(row, field.key) }}</code>
              <el-input
                :model-value="row.secretInputs[field.key] ?? ''"
                :placeholder="secretPlaceholder(row, field.key)"
                :disabled="!canWrite || row.locked"
                show-password
                @update:model-value="setSecretValue(row, field.key, $event)"
              />
            </div>
          </template>

          <template v-else-if="field.component === 'switch'">
            <el-switch
              :model-value="Boolean(row.draftConfig[field.key])"
              :disabled="!canWrite || row.locked"
              @update:model-value="setDraftValue(row, field.key, $event)"
            />
          </template>

          <template v-else-if="field.component === 'number'">
            <el-input-number
              :model-value="toNumberOrNull(row.draftConfig[field.key])"
              :disabled="!canWrite || row.locked"
              controls-position="right"
              @update:model-value="setDraftValue(row, field.key, $event)"
            />
          </template>

          <template v-else-if="field.component === 'select'">
            <el-select
              :model-value="toText(row.draftConfig[field.key])"
              :disabled="!canWrite || row.locked"
              class="full-width"
              clearable
              @update:model-value="setDraftValue(row, field.key, $event)"
            >
              <el-option
                v-for="opt in field.options ?? []"
                :key="String(opt.value)"
                :label="opt.label"
                :value="String(opt.value)"
              />
            </el-select>
          </template>

          <template v-else-if="field.component === 'json'">
            <el-input
              :model-value="row.jsonInputs[field.key] ?? ''"
              type="textarea"
              :rows="4"
              :disabled="!canWrite || row.locked"
              placeholder="请输入 JSON 对象"
              @update:model-value="setJsonValue(row, field.key, $event)"
            />
          </template>

          <template v-else-if="field.component === 'file' || isFileField(row, field.key)">
            <el-input
              :model-value="toText(row.draftConfig[field.key])"
              :disabled="!canWrite || row.locked"
              placeholder="文件引用（留空不修改）"
              @update:model-value="setDraftValue(row, field.key, $event)"
            />
          </template>

          <template v-else>
            <el-input
              :model-value="toText(row.draftConfig[field.key])"
              :type="field.component === 'textarea' ? 'textarea' : field.component === 'password' ? 'password' : 'text'"
              :rows="field.component === 'textarea' ? 3 : undefined"
              :show-password="field.component === 'password'"
              :disabled="!canWrite || row.locked"
              @update:model-value="setDraftValue(row, field.key, $event)"
            />
          </template>
        </el-form-item>
      </el-form>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { ApiError } from "@/api/http";
import { usePermissionStore } from "@/stores/permission";
import { listMarketChannels } from "@/api/modules/channels";
import {
  listAccountAuthTypes,
  listChannelAccountAuthTypes,
  listGameAccountAuthConfigs,
  replaceGameAccountAuthConfigs,
  type AccountAuthTemplateField,
  type AccountAuthTypeItem,
  type ConfigStatus,
  type GameAccountAuthConfigItem
} from "@/api/modules/accountAuth";

interface AuthRow {
  authTypeId: string;
  authTypeName: string;
  formSchema: AccountAuthTemplateField[];
  secretFields: string[];
  fileFields: string[];
  enabled: boolean;
  defaultEnabled: boolean;
  locked: boolean;
  configStatus: ConfigStatus;
  lastCheckAt: string | null;
  lastCheckMessage: string;
  draftConfig: Record<string, unknown>;
  secretInputs: Record<string, string>;
  jsonInputs: Record<string, string>;
  sort: number;
}

const props = defineProps<{ gameId: string }>();

const permission = usePermissionStore();
const canWrite = computed(() => permission.hasPerm("game.write"));

const loading = ref(false);
const saving = ref(false);
const loadError = ref("");
const rows = ref<AuthRow[]>([]);

function deepClone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value ?? {})) as T;
}

function toText(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  return typeof value === "string" ? value : String(value);
}

function toNumberOrNull(value: unknown): number | null {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  const n = Number(value);
  return Number.isNaN(n) ? null : n;
}

function sortedFields(fields: AccountAuthTemplateField[]): AccountAuthTemplateField[] {
  return [...fields].sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
}

function statusTone(status: ConfigStatus): "neutral" | "success" | "warning" {
  if (status === "valid") {
    return "success";
  }
  if (status === "invalid") {
    return "warning";
  }
  return "neutral";
}

function isSecretField(row: AuthRow, key: string): boolean {
  return row.secretFields.includes(key);
}

function isFileField(row: AuthRow, key: string): boolean {
  return row.fileFields.includes(key);
}

function hasStoredSecret(row: AuthRow, key: string): boolean {
  const value = row.draftConfig[key];
  return value !== null && value !== undefined && String(value).trim() !== "";
}

function secretMaskedLabel(row: AuthRow, key: string): string {
  return toText(row.draftConfig[key]) || "masked";
}

function secretPlaceholder(row: AuthRow, key: string): string {
  return hasStoredSecret(row, key) ? "留空则不修改" : "请输入新值";
}

function setEnabled(row: AuthRow, value: boolean | string | number) {
  row.enabled = Boolean(value);
}

function setDraftValue(row: AuthRow, key: string, value: unknown) {
  row.draftConfig[key] = value;
}

function setSecretValue(row: AuthRow, key: string, value: string | number) {
  row.secretInputs[key] = String(value ?? "");
}

function setJsonValue(row: AuthRow, key: string, value: string | number) {
  row.jsonInputs[key] = String(value ?? "");
}

async function collectChannelIds(gameId: string): Promise<string[]> {
  const channelIds = new Set<string>();
  let page = 1;
  while (true) {
    const res = await listMarketChannels(gameId, { page, pageSize: 100 });
    for (const item of res.items) {
      if (item.channelId) {
        channelIds.add(item.channelId);
      }
    }
    if (res.items.length < res.pageSize || page * res.pageSize >= res.total) {
      break;
    }
    page += 1;
  }
  return Array.from(channelIds);
}

function mergeAllowedFlags(items: Array<{ authTypeId: string; defaultEnabled: boolean; locked: boolean }>) {
  const map = new Map<string, { defaultEnabled: boolean; locked: boolean }>();
  for (const item of items) {
    const prev = map.get(item.authTypeId);
    map.set(item.authTypeId, {
      defaultEnabled: Boolean(prev?.defaultEnabled || item.defaultEnabled),
      locked: Boolean(prev?.locked || item.locked)
    });
  }
  return map;
}

function toJsonInputs(fields: AccountAuthTemplateField[], configJson: Record<string, unknown>): Record<string, string> {
  const result: Record<string, string> = {};
  for (const field of fields) {
    if (field.component !== "json") {
      continue;
    }
    const raw = configJson[field.key];
    if (raw === undefined) {
      result[field.key] = "";
    } else if (typeof raw === "string") {
      result[field.key] = raw;
    } else {
      result[field.key] = JSON.stringify(raw, null, 2);
    }
  }
  return result;
}

function applyConfigs(targetRows: AuthRow[], configs: GameAccountAuthConfigItem[]) {
  const configByType = new Map(configs.map((item) => [item.authTypeId, item] as const));
  for (const row of targetRows) {
    const cfg = configByType.get(row.authTypeId);
    const configJson = deepClone(cfg?.configJson ?? {});
    row.enabled = cfg?.enabled ?? row.defaultEnabled;
    row.configStatus = cfg?.configStatus ?? "empty";
    row.lastCheckAt = cfg?.lastCheckAt ?? null;
    row.lastCheckMessage = cfg?.lastCheckMessage ?? "";
    row.draftConfig = configJson;
    row.secretInputs = {};
    row.jsonInputs = toJsonInputs(row.formSchema, configJson);
  }
}

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const [typeItems, gameConfigs, channelIds] = await Promise.all([
      listAccountAuthTypes(),
      listGameAccountAuthConfigs(props.gameId),
      collectChannelIds(props.gameId)
    ]);

    const channelAllowedItems = (
      await Promise.all(channelIds.map((channelId) => listChannelAccountAuthTypes(channelId)))
    ).flat();
    const allowedMap = mergeAllowedFlags(channelAllowedItems);

    const typeById = new Map<string, AccountAuthTypeItem>(typeItems.map((item) => [item.authTypeId, item]));

    const nextRows: AuthRow[] = gameConfigs.map((cfg) => {
      const type = typeById.get(cfg.authTypeId);
      const flags = allowedMap.get(cfg.authTypeId) ?? { defaultEnabled: false, locked: false };
      const configJson = deepClone(cfg.configJson ?? {});
      const fields = type?.template?.formSchema ?? [];
      return {
        authTypeId: cfg.authTypeId,
        authTypeName: type?.authTypeName ?? cfg.authTypeId,
        formSchema: fields,
        secretFields: type?.template?.secretFields ?? [],
        fileFields: (type?.template?.fileFields ?? []).map((item) => item.key),
        enabled: cfg.enabled,
        defaultEnabled: flags.defaultEnabled,
        locked: flags.locked,
        configStatus: cfg.configStatus,
        lastCheckAt: cfg.lastCheckAt,
        lastCheckMessage: cfg.lastCheckMessage,
        draftConfig: configJson,
        secretInputs: {},
        jsonInputs: toJsonInputs(fields, configJson),
        sort: type?.sort ?? 999
      };
    });

    nextRows.sort((a, b) => a.sort - b.sort || a.authTypeId.localeCompare(b.authTypeId));
    rows.value = nextRows;
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : "加载自有账号认证配置失败";
    rows.value = [];
  } finally {
    loading.value = false;
  }
}

function buildConfigForSubmit(row: AuthRow): Record<string, unknown> {
  const nextConfig = deepClone(row.draftConfig);
  for (const field of row.formSchema) {
    if (field.component !== "json") {
      continue;
    }
    const raw = (row.jsonInputs[field.key] ?? "").trim();
    if (raw === "") {
      delete nextConfig[field.key];
      continue;
    }
    try {
      nextConfig[field.key] = JSON.parse(raw) as unknown;
    } catch {
      throw new Error(`认证方式 ${row.authTypeName} 的字段 ${field.key} 不是合法 JSON`);
    }
  }

  for (const secretKey of row.secretFields) {
    delete nextConfig[secretKey];
    const nextSecret = (row.secretInputs[secretKey] ?? "").trim();
    if (nextSecret) {
      nextConfig[secretKey] = nextSecret;
    }
  }
  return nextConfig;
}

async function saveAll() {
  if (!rows.value.length) {
    return;
  }
  saving.value = true;
  try {
    const payload = {
      items: rows.value.map((row) => ({
        authTypeId: row.authTypeId,
        enabled: row.enabled,
        configJson: buildConfigForSubmit(row)
      }))
    };
    const configs = await replaceGameAccountAuthConfigs(props.gameId, payload);
    applyConfigs(rows.value, configs);
    ElMessage.success("自有账号认证配置已保存");
  } catch (err) {
    if (err instanceof Error && !(err instanceof ApiError)) {
      ElMessage.error(err.message);
    } else if (err instanceof ApiError) {
      ElMessage.error(err.message || "保存失败");
    } else {
      ElMessage.error("保存失败");
    }
  } finally {
    saving.value = false;
  }
}

watch(
  () => props.gameId,
  () => {
    void load();
  },
  { immediate: true }
);
</script>

<style scoped>
.account-auth-tab {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.account-auth-tab__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.account-auth-tab__actions {
  display: flex;
  gap: 8px;
}

.account-auth-tab__alert {
  margin-bottom: 2px;
}

.auth-card {
  border: 1px solid var(--panel-border);
  border-radius: var(--radius-sm);
  padding: 14px;
  background: #fff;
}

.auth-card.is-locked {
  border-color: #f7d794;
}

.auth-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.auth-card__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.auth-card__title {
  margin: 0;
  font-size: 15px;
}

.auth-card__code {
  color: var(--text-subtle);
  font-size: 12px;
}

.auth-card__state {
  display: flex;
  align-items: center;
  gap: 10px;
}

.auth-card__message {
  margin: 10px 0;
  color: var(--text-subtle);
  font-size: 12px;
}

.auth-card__alert {
  margin-bottom: 10px;
}

.auth-card__form {
  margin-top: 8px;
}

.secret-field {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.secret-field__masked {
  flex-shrink: 0;
  color: var(--text-subtle);
  font-size: 12px;
}

.full-width {
  width: 100%;
}

.empty-state {
  padding: 24px 0;
}

.empty-state__title {
  margin: 0;
  font-weight: 600;
}

.empty-state__hint {
  margin: 6px 0 0;
  color: var(--text-subtle);
}
</style>
