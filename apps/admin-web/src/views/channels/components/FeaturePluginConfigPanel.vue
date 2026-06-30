<template>
  <section class="panel" v-loading="loading">
    <div class="panel__head">
      <div class="panel__head-left">
        <h4 class="panel__title">功能插件</h4>
        <EnvironmentBadge :environment="app.environment" />
      </div>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-alert
      v-if="requiredMissing.length > 0"
      type="warning"
      show-icon
      :closable="false"
      class="panel__alert"
      :title="`存在 ${requiredMissing.length} 个必接插件未配置完成，请优先补齐`"
      :description="requiredMissingText"
    />

    <div v-if="items.length === 0" class="empty">暂无可接入插件</div>

    <el-collapse v-model="activePanels">
      <el-collapse-item v-for="item in items" :key="item.pluginId" :name="item.pluginId">
        <template #title>
          <div class="plugin-title">
            <span class="plugin-title__name">{{ item.pluginName }}</span>
            <code class="plugin-title__id">{{ item.pluginId }}</code>
            <el-tag size="small" :type="item.region === 'domestic' ? 'success' : 'info'">
              {{ item.region === "domestic" ? "国内" : "海外" }}
            </el-tag>
            <el-tag v-if="item.required" size="small" type="danger">必接</el-tag>
            <el-tag v-if="item.locked" size="small" type="warning">锁定</el-tag>
            <el-tag size="small" :type="item.includedInRuntimeConfig ? 'success' : 'info'">
              {{ item.includedInRuntimeConfig ? "进入最终配置" : "未进入最终配置" }}
            </el-tag>
            <PageStatusTag :tone="statusTone(item.configStatus)" :label="statusLabel(item.configStatus)" />
          </div>
        </template>

        <div class="plugin-body">
          <el-alert
            v-if="item.locked"
            type="info"
            :closable="false"
            title="该插件已锁定，当前实例不可编辑。"
            class="panel__alert"
          />
          <el-alert
            v-else-if="!item.selectable"
            type="info"
            :closable="false"
            title="该插件不可取消勾选。"
            class="panel__alert"
          />

          <div class="plugin-row">
            <el-switch
              :model-value="draftOf(item).enabled"
              :disabled="!canEdit(item) || !item.selectable"
              active-text="已启用"
              inactive-text="未启用"
              @update:model-value="onEnabledChange(item, Boolean($event))"
            />
            <span class="plugin-row__msg">{{ item.lastCheckMessage || "最近校验：-" }}</span>
          </div>

          <template v-for="group in groupedFields(item)" :key="`${item.pluginId}:${group.name}`">
            <h5 class="group-title">{{ group.name || "默认分组" }}</h5>
            <el-form label-position="top">
              <el-form-item
                v-for="field in group.fields"
                :key="`${item.pluginId}:${field.key}`"
                :label="field.label || field.key"
                :required="Boolean(field.required)"
                :error="fieldError(item, field.key)"
              >
                <template #label>
                  <div class="field-label">
                    <span>{{ field.label || field.key }}</span>
                    <el-tag v-if="field.scope === 'server'" size="small" type="warning">仅服务端，不下发客户端</el-tag>
                  </div>
                </template>

                <template v-if="isSecretField(item, field.key)">
                  <div class="secret-row">
                    <el-input
                      :model-value="secretInputValue(item, field.key)"
                      show-password
                      :disabled="!canEdit(item)"
                      :placeholder="secretPlaceholder(item, field.key)"
                      @focus="beginEditSecret(item, field.key)"
                      @update:model-value="setSecretValue(item, field.key, String($event ?? ''))"
                    />
                    <el-button
                      v-if="hasStoredSecret(item, field.key) && !draftOf(item).secretStates[field.key]?.editing"
                      :disabled="!canEdit(item)"
                      @click="beginEditSecret(item, field.key)"
                    >
                      修改
                    </el-button>
                  </div>
                </template>

                <template v-else-if="field.component === 'switch'">
                  <el-switch
                    :model-value="Boolean(draftOf(item).config[field.key])"
                    :disabled="!canEdit(item)"
                    @update:model-value="setDraftValue(item, field.key, $event)"
                  />
                </template>

                <template v-else-if="field.component === 'number'">
                  <el-input-number
                    :model-value="toNumberOrNull(draftOf(item).config[field.key])"
                    :disabled="!canEdit(item)"
                    controls-position="right"
                    @update:model-value="setDraftValue(item, field.key, $event)"
                  />
                </template>

                <template v-else-if="field.component === 'select'">
                  <el-select
                    :model-value="toText(draftOf(item).config[field.key])"
                    class="full-width"
                    :disabled="!canEdit(item)"
                    clearable
                    @update:model-value="setDraftValue(item, field.key, $event)"
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
                    :model-value="draftOf(item).jsonInputs[field.key] ?? ''"
                    type="textarea"
                    :rows="4"
                    :disabled="!canEdit(item)"
                    placeholder="请输入 JSON 对象"
                    @update:model-value="setJsonValue(item, field.key, String($event ?? ''))"
                  />
                </template>

                <template v-else-if="isFileField(item, field.key)">
                  <div class="file-row">
                    <el-upload
                      :show-file-list="false"
                      :accept="fileAccept(item, field.key)"
                      :disabled="!canEdit(item)"
                      :http-request="(options: UploadRequestOptions) => onFileUpload(item, field.key, options)"
                    >
                      <el-button :disabled="!canEdit(item)">上传文件</el-button>
                    </el-upload>
                    <span class="file-row__meta">
                      {{ fileRuleText(item, field.key) }}
                      <template v-if="toText(draftOf(item).config[field.key])">
                        ｜当前：{{ toText(draftOf(item).config[field.key]) }}
                      </template>
                    </span>
                  </div>
                </template>

                <template v-else>
                  <el-input
                    :model-value="toText(draftOf(item).config[field.key])"
                    :type="field.component === 'textarea' ? 'textarea' : field.component === 'password' ? 'password' : 'text'"
                    :rows="field.component === 'textarea' ? 3 : undefined"
                    :show-password="field.component === 'password'"
                    :placeholder="field.placeholder || ''"
                    :disabled="!canEdit(item)"
                    @update:model-value="setDraftValue(item, field.key, $event)"
                  />
                </template>
              </el-form-item>
            </el-form>
          </template>

          <el-button
            v-perm="'plugin.write'"
            type="primary"
            :loading="savingPluginId === item.pluginId"
            :disabled="!canEdit(item)"
            @click="saveItem(item)"
          >
            保存插件配置
          </el-button>
        </div>
      </el-collapse-item>
    </el-collapse>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import type { UploadRequestOptions } from "element-plus";
import { ApiError } from "@/api/http";
import {
  listGameChannelPlugins,
  patchGameChannelPlugin,
  upsertGameChannelPlugin,
  type GameChannelPluginItem,
  type PluginTemplateField,
  type PluginTemplateFileField
} from "@/api/modules/channels";
import type { MarketChannelDetail } from "@/api/modules/channels";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { useAppStore } from "@/stores/app";
import { configStatusMeta } from "../constants";

const app = useAppStore();

type ValidationRule = {
  required?: boolean;
  minLen?: number;
  maxLen?: number;
  min?: number;
  max?: number;
  pattern?: string;
  format?: "url" | "email" | "host";
  enum?: Array<string | number | boolean>;
};

interface SecretState {
  editing: boolean;
  value: string;
  hadStored: boolean;
}

interface PluginDraft {
  enabled: boolean;
  config: Record<string, unknown>;
  jsonInputs: Record<string, string>;
  secretStates: Record<string, SecretState>;
}

const props = defineProps<{
  gameChannelId: number;
  detail: MarketChannelDetail;
  canWrite: boolean;
}>();

const emit = defineEmits<{
  (e: "changed"): void;
}>();

const loading = ref(false);
const items = ref<GameChannelPluginItem[]>([]);
const savingPluginId = ref("");
const activePanels = ref<string[]>([]);
const drafts = reactive<Record<string, PluginDraft>>({});

const requiredMissing = computed(() =>
  items.value.filter((item) => item.required && (!item.enabled || item.configStatus !== "valid"))
);
const requiredMissingText = computed(() =>
  requiredMissing.value.map((item) => `${item.pluginName}(${item.pluginId})`).join("、")
);

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

function statusTone(status: string): "neutral" | "success" | "warning" | "danger" {
  return configStatusMeta(status).tone;
}

function statusLabel(status: string): string {
  return configStatusMeta(status).label;
}

function sortedFields(item: GameChannelPluginItem): PluginTemplateField[] {
  return [...(item.template.formSchemaJson ?? [])].sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
}

function groupedFields(item: GameChannelPluginItem): Array<{ name: string; fields: PluginTemplateField[] }> {
  const groups = new Map<string, PluginTemplateField[]>();
  for (const field of sortedFields(item)) {
    const key = field.group || "";
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key)?.push(field);
  }
  return Array.from(groups.entries()).map(([name, fields]) => ({ name, fields }));
}

function fileFieldMap(item: GameChannelPluginItem): Record<string, PluginTemplateFileField> {
  const map: Record<string, PluginTemplateFileField> = {};
  for (const entry of item.template.fileFieldsJson ?? []) {
    map[entry.key] = entry;
  }
  return map;
}

function getValidationRule(item: GameChannelPluginItem, key: string): ValidationRule {
  const raw = item.template.validationRulesJson?.[key];
  return (raw && typeof raw === "object" ? raw : {}) as ValidationRule;
}

function isSecretField(item: GameChannelPluginItem, key: string): boolean {
  return item.template.secretFieldsJson.includes(key);
}

function isFileField(item: GameChannelPluginItem, key: string): boolean {
  return Boolean(fileFieldMap(item)[key]);
}

function canEdit(item: GameChannelPluginItem): boolean {
  return props.canWrite && !item.locked;
}

function initDraft(item: GameChannelPluginItem) {
  const next: PluginDraft = {
    enabled: item.required && !item.selectable ? true : item.enabled,
    config: JSON.parse(JSON.stringify(item.configJson ?? {})) as Record<string, unknown>,
    jsonInputs: {},
    secretStates: {}
  };
  for (const field of item.template.formSchemaJson ?? []) {
    if (field.component !== "json") {
      continue;
    }
    const raw = next.config[field.key];
    if (raw === undefined) {
      next.jsonInputs[field.key] = "";
    } else if (typeof raw === "string") {
      next.jsonInputs[field.key] = raw;
    } else {
      next.jsonInputs[field.key] = JSON.stringify(raw, null, 2);
    }
  }
  for (const secretKey of item.template.secretFieldsJson ?? []) {
    const raw = toText(next.config[secretKey]).trim();
    next.secretStates[secretKey] = {
      editing: false,
      value: "",
      hadStored: raw !== ""
    };
  }
  drafts[item.pluginId] = next;
}

function draftOf(item: GameChannelPluginItem): PluginDraft {
  if (!drafts[item.pluginId]) {
    initDraft(item);
  }
  return drafts[item.pluginId];
}

function hasStoredSecret(item: GameChannelPluginItem, key: string): boolean {
  return Boolean(draftOf(item).secretStates[key]?.hadStored);
}

function beginEditSecret(item: GameChannelPluginItem, key: string) {
  const state = draftOf(item).secretStates[key];
  if (!state || !canEdit(item)) {
    return;
  }
  if (!state.editing) {
    state.editing = true;
    state.value = "";
  }
}

function secretInputValue(item: GameChannelPluginItem, key: string): string {
  const state = draftOf(item).secretStates[key];
  if (!state) {
    return "";
  }
  if (!state.editing && state.hadStored) {
    return "******";
  }
  return state.value;
}

function secretPlaceholder(item: GameChannelPluginItem, key: string): string {
  const state = draftOf(item).secretStates[key];
  if (!state) {
    return "请输入密钥";
  }
  if (!state.editing && state.hadStored) {
    return "聚焦或点击修改后可重新输入";
  }
  return "请输入密钥，留空则保持原值";
}

function setSecretValue(item: GameChannelPluginItem, key: string, value: string) {
  const state = draftOf(item).secretStates[key];
  if (!state) {
    return;
  }
  state.editing = true;
  state.value = value;
}

function setDraftValue(item: GameChannelPluginItem, key: string, value: unknown) {
  draftOf(item).config[key] = value;
}

function setJsonValue(item: GameChannelPluginItem, key: string, value: string) {
  draftOf(item).jsonInputs[key] = value;
}

function fileAccept(item: GameChannelPluginItem, key: string): string {
  return (fileFieldMap(item)[key]?.accept ?? []).join(",");
}

function fileRuleText(item: GameChannelPluginItem, key: string): string {
  const rule = fileFieldMap(item)[key];
  if (!rule) {
    return "文件字段";
  }
  const parts: string[] = [];
  if (rule.accept?.length) {
    parts.push(`accept: ${rule.accept.join(", ")}`);
  }
  if (rule.maxSizeKB) {
    parts.push(`max: ${rule.maxSizeKB}KB`);
  }
  return parts.length > 0 ? parts.join("；") : "文件字段";
}

function onFileUpload(item: GameChannelPluginItem, key: string, options: UploadRequestOptions) {
  const rule = fileFieldMap(item)[key];
  if (!rule) {
    options.onError(new Error("文件字段配置缺失") as never);
    return;
  }
  const file = options.file;
  if (rule.maxSizeKB && file.size > rule.maxSizeKB * 1024) {
    const err = new Error(`文件超过 ${rule.maxSizeKB}KB 限制`);
    ElMessage.error(err.message);
    options.onError(err as never);
    return;
  }
  const accepts = rule.accept ?? [];
  if (accepts.length > 0 && !accepts.some((ext) => file.name.toLowerCase().endsWith(ext.toLowerCase()))) {
    const err = new Error(`文件类型不符合：${accepts.join(", ")}`);
    ElMessage.error(err.message);
    options.onError(err as never);
    return;
  }
  draftOf(item).config[key] = file.name;
  ElMessage.success(`已选择文件：${file.name}`);
  options.onSuccess({ fileName: file.name });
}

function checkRule(item: GameChannelPluginItem, key: string, value: unknown): string | null {
  const rule = getValidationRule(item, key);
  const text = toText(value);
  if (rule.minLen !== undefined && text.length < rule.minLen) {
    return `长度不能小于 ${rule.minLen}`;
  }
  if (rule.maxLen !== undefined && text.length > rule.maxLen) {
    return `长度不能大于 ${rule.maxLen}`;
  }
  if (rule.pattern && !(new RegExp(rule.pattern).test(text))) {
    return "格式不符合 pattern 规则";
  }
  if (rule.enum && !rule.enum.some((entry) => String(entry) === text)) {
    return "值不在枚举范围";
  }
  if (rule.format === "url") {
    try {
      new URL(text);
    } catch {
      return "请输入合法 URL";
    }
  }
  if (rule.format === "email" && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(text)) {
    return "请输入合法邮箱";
  }
  if (rule.format === "host" && !/^[a-zA-Z0-9.-]+$/.test(text)) {
    return "请输入合法主机名";
  }
  if (rule.min !== undefined || rule.max !== undefined) {
    const num = Number(value);
    if (!Number.isNaN(num)) {
      if (rule.min !== undefined && num < rule.min) {
        return `不能小于 ${rule.min}`;
      }
      if (rule.max !== undefined && num > rule.max) {
        return `不能大于 ${rule.max}`;
      }
    }
  }
  return null;
}

function validateField(item: GameChannelPluginItem, field: PluginTemplateField): string | null {
  const key = field.key;
  const required = Boolean(field.required || getValidationRule(item, key).required);
  const draft = draftOf(item);
  if (isSecretField(item, key)) {
    const state = draft.secretStates[key];
    const trimmed = state?.value.trim() ?? "";
    const nextValue = state?.editing ? trimmed || (state.hadStored ? "******" : "") : state?.hadStored ? "******" : "";
    if (required && !nextValue) {
      return "该字段为必填";
    }
    if (!nextValue || nextValue === "******") {
      return null;
    }
    return checkRule(item, key, nextValue);
  }
  if (field.component === "json") {
    const raw = (draft.jsonInputs[key] ?? "").trim();
    if (required && raw === "") {
      return "该字段为必填";
    }
    if (raw === "") {
      return null;
    }
    try {
      const parsed = JSON.parse(raw) as unknown;
      return checkRule(item, key, parsed);
    } catch {
      return "JSON 格式不合法";
    }
  }
  const value = draft.config[key];
  const text = toText(value).trim();
  if (required && text === "") {
    return "该字段为必填";
  }
  if (text === "") {
    return null;
  }
  return checkRule(item, key, value);
}

function fieldError(item: GameChannelPluginItem, key: string): string {
  const field = sortedFields(item).find((entry) => entry.key === key);
  if (!field) {
    return "";
  }
  return validateField(item, field) ?? "";
}

function buildPayloadConfig(item: GameChannelPluginItem): Record<string, unknown> {
  const next: Record<string, unknown> = {};
  const draft = draftOf(item);
  for (const field of sortedFields(item)) {
    const key = field.key;
    if (isSecretField(item, key)) {
      const state = draft.secretStates[key];
      if (!state) {
        continue;
      }
      if (!state.editing && state.hadStored) {
        next[key] = "******";
      } else {
        const trimmed = state.value.trim();
        if (trimmed) {
          next[key] = trimmed;
        } else if (state.hadStored) {
          next[key] = "******";
        } else {
          next[key] = "";
        }
      }
      continue;
    }
    if (field.component === "json") {
      const raw = (draft.jsonInputs[key] ?? "").trim();
      if (!raw) {
        continue;
      }
      next[key] = JSON.parse(raw) as unknown;
      continue;
    }
    const value = draft.config[key];
    if (value !== undefined) {
      next[key] = value;
    }
  }
  return next;
}

function onEnabledChange(item: GameChannelPluginItem, value: boolean) {
  if (item.required && !item.selectable) {
    draftOf(item).enabled = true;
    return;
  }
  draftOf(item).enabled = value;
}

async function saveItem(item: GameChannelPluginItem) {
  const firstError = sortedFields(item).map((field) => validateField(item, field)).find(Boolean);
  if (firstError) {
    ElMessage.warning(firstError);
    return;
  }
  const draft = draftOf(item);
  savingPluginId.value = item.pluginId;
  try {
    const payload = { enabled: draft.enabled, config: buildPayloadConfig(item) };
    const saved =
      item.id > 0
        ? await patchGameChannelPlugin(item.id, payload)
        : await upsertGameChannelPlugin(props.gameChannelId, {
            pluginId: item.pluginId,
            ...payload
          });
    items.value = items.value.map((entry) => (entry.pluginId === item.pluginId ? saved : entry));
    initDraft(saved);
    ElMessage.success("插件配置已保存");
    emit("changed");
  } catch (err) {
    if (err instanceof ApiError) {
      ElMessage.error(err.message || "插件配置保存失败");
      if (err.code === "VALIDATION_FAILED") {
        await load();
        emit("changed");
      }
      return;
    }
    ElMessage.error("插件配置保存失败");
  } finally {
    savingPluginId.value = "";
  }
}

async function load() {
  loading.value = true;
  try {
    const next = await listGameChannelPlugins(props.gameChannelId);
    items.value = next;
    for (const item of next) {
      initDraft(item);
    }
    activePanels.value = next.slice(0, 1).map((entry) => entry.pluginId);
  } catch (err) {
    items.value = [];
    ElMessage.error(err instanceof ApiError ? err.message : "加载功能插件失败");
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.gameChannelId,
  () => {
    void load();
  },
  { immediate: true }
);
</script>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.panel__head-left {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.panel__title {
  margin: 0;
}

.panel__alert {
  margin: 0;
}

.empty {
  color: var(--text-subtle);
}

.plugin-title {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 13px;
}

.plugin-title__name {
  font-weight: 600;
}

.plugin-title__id {
  color: var(--text-subtle);
}

.plugin-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.plugin-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.plugin-row__msg {
  color: var(--text-subtle);
  font-size: 12px;
}

.group-title {
  margin: 8px 0 0;
  font-size: 13px;
}

.field-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.secret-row {
  display: flex;
  width: 100%;
  gap: 8px;
}

.file-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.file-row__meta {
  color: var(--text-subtle);
  font-size: 12px;
}

.full-width {
  width: 100%;
}
</style>
