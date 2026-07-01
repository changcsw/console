<template>
  <section class="panel" v-loading="loading">
    <template v-if="model">
      <div class="panel__context">
        <EnvironmentBadge :environment="model.env" />
        <el-tag type="info" size="small">marketCode: {{ model.marketCode }}</el-tag>
        <el-tag type="info" size="small">channelId: {{ model.channelId }}</el-tag>
        <el-tag size="small" :type="model.loginMode === 'channel_only' ? 'warning' : 'success'">
          loginMode: {{ model.loginMode }}
        </el-tag>
        <el-tag size="small" :type="model.loginLocked ? 'danger' : 'info'">
          loginLocked: {{ model.loginLocked ? "locked" : "unlocked" }}
        </el-tag>
      </div>

      <div class="panel__status">
        <PageStatusTag :tone="statusTone(model.config.configStatus)" :label="statusLabel(model.config.configStatus)" />
        <span class="panel__status-message">
          {{ model.config.lastCheckMessage || "最近校验：-" }}
          <template v-if="model.config.lastCheckAt">（{{ formatTime(model.config.lastCheckAt) }}）</template>
        </span>
      </div>

      <el-alert
        v-if="model.config.enabled && model.config.configStatus !== 'valid'"
        type="warning"
        show-icon
        :closable="false"
        title="已启用但配置无效，将不进入快照/同步/客户端最终配置"
        class="panel__alert"
      />
      <el-alert
        v-if="isCopyInvalidHint(model.config.lastCheckMessage)"
        type="info"
        :closable="false"
        title="该实例来自复制创建，请补齐密钥/文件字段后再投入运行"
        class="panel__alert"
      />

      <ChannelInstanceRuntimeFlags
        :enabled="model.config.enabled"
        :hidden="detail.hidden"
        :compatible="detail.compatible"
        :config-status="model.config.configStatus"
        :included-in-snapshot="runtimeIncluded"
        :included-in-sync="runtimeIncluded"
        :included-in-runtime-config="runtimeIncluded"
      />

      <el-form label-position="top" class="panel__form">
        <el-form-item label="启用渠道登录">
          <el-switch v-model="enabled" :disabled="!canWrite" />
        </el-form-item>

        <template v-for="group in groupedFields" :key="group.name">
          <h4 class="group-title">{{ group.name || "默认分组" }}</h4>
          <el-form-item
            v-for="field in group.fields"
            :key="field.key"
            :required="Boolean(field.required)"
            :error="fieldErrors[field.key] || ''"
            :label="field.label || field.key"
          >
            <template v-if="isSecretField(field.key)">
              <div class="secret-row">
                <el-input
                  :model-value="secretInputValue(field.key)"
                  show-password
                  :placeholder="secretPlaceholder(field.key)"
                  :disabled="!canWrite"
                  @focus="beginEditSecret(field.key)"
                  @update:model-value="setSecretValue(field.key, String($event ?? ''))"
                />
                <el-button
                  v-if="hasStoredSecret(field.key) && !secretStates[field.key]?.editing"
                  :disabled="!canWrite"
                  @click="beginEditSecret(field.key)"
                >
                  修改
                </el-button>
              </div>
            </template>

            <template v-else-if="field.component === 'switch'">
              <el-switch
                :model-value="Boolean(draftConfig[field.key])"
                :disabled="!canWrite"
                @update:model-value="setDraftValue(field.key, $event)"
              />
            </template>

            <template v-else-if="field.component === 'number'">
              <el-input-number
                :model-value="toNumberOrNull(draftConfig[field.key])"
                :disabled="!canWrite"
                controls-position="right"
                @update:model-value="setDraftValue(field.key, $event)"
              />
            </template>

            <template v-else-if="field.component === 'select'">
              <el-select
                :model-value="toText(draftConfig[field.key])"
                class="full-width"
                clearable
                :disabled="!canWrite"
                @update:model-value="setDraftValue(field.key, $event)"
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
                :model-value="jsonInputs[field.key] ?? ''"
                type="textarea"
                :rows="4"
                :disabled="!canWrite"
                placeholder="请输入 JSON 对象"
                @update:model-value="setJsonValue(field.key, String($event ?? ''))"
              />
            </template>

            <template v-else-if="isFileField(field.key)">
              <div class="file-row">
                <el-upload
                  :show-file-list="false"
                  :accept="fileAccept(field.key)"
                  :disabled="!canWrite"
                  :http-request="(options: UploadRequestOptions) => onFileUpload(field.key, options)"
                >
                  <el-button :disabled="!canWrite">上传文件</el-button>
                </el-upload>
                <span class="file-row__meta">
                  {{ fileRuleText(field.key) }}
                  <template v-if="toText(draftConfig[field.key])">｜当前：{{ toText(draftConfig[field.key]) }}</template>
                </span>
              </div>
            </template>

            <template v-else>
              <el-input
                :model-value="toText(draftConfig[field.key])"
                :type="field.component === 'textarea' ? 'textarea' : field.component === 'password' ? 'password' : 'text'"
                :rows="field.component === 'textarea' ? 3 : undefined"
                :show-password="field.component === 'password'"
                :placeholder="field.placeholder || ''"
                :disabled="!canWrite"
                @update:model-value="setDraftValue(field.key, $event)"
              />
            </template>
          </el-form-item>
        </template>

        <div class="panel__actions">
          <el-button @click="load">刷新</el-button>
          <el-button v-perm="'channel.write'" type="primary" :loading="saving" :disabled="!canWrite" @click="save">
            保存渠道登录配置
          </el-button>
        </div>
      </el-form>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import type { UploadRequestOptions } from "element-plus";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { ApiError } from "@/api/http";
import {
  getLoginConfig,
  putLoginConfig,
  type ChannelLoginConfigResponse,
  type ChannelLoginTemplateField,
  type ChannelLoginTemplateFileField
} from "@/api/modules/channels";
import type { MarketChannelDetail } from "@/api/modules/channels";
import ChannelInstanceRuntimeFlags from "./ChannelInstanceRuntimeFlags.vue";
import { configStatusMeta, COPY_INVALID_HINT } from "../constants";

interface SecretState {
  editing: boolean;
  value: string;
  hadStored: boolean;
}

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

const props = defineProps<{
  gameChannelId: number;
  detail: MarketChannelDetail;
  canWrite: boolean;
}>();

const emit = defineEmits<{
  (e: "changed"): void;
}>();

const loading = ref(false);
const saving = ref(false);
const model = ref<ChannelLoginConfigResponse | null>(null);
const enabled = ref(false);
const draftConfig = reactive<Record<string, unknown>>({});
const jsonInputs = reactive<Record<string, string>>({});
const secretStates = reactive<Record<string, SecretState>>({});

function resetState() {
  for (const key of Object.keys(draftConfig)) {
    delete draftConfig[key];
  }
  for (const key of Object.keys(jsonInputs)) {
    delete jsonInputs[key];
  }
  for (const key of Object.keys(secretStates)) {
    delete secretStates[key];
  }
}

const sortedFields = computed<ChannelLoginTemplateField[]>(() => {
  if (!model.value) {
    return [];
  }
  return [...model.value.template.formSchemaJson].sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
});

const groupedFields = computed(() => {
  const groups = new Map<string, ChannelLoginTemplateField[]>();
  for (const field of sortedFields.value) {
    const key = field.group || "";
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key)?.push(field);
  }
  return Array.from(groups.entries()).map(([name, fields]) => ({ name, fields }));
});

const fileFieldMap = computed<Record<string, ChannelLoginTemplateFileField>>(() => {
  const map: Record<string, ChannelLoginTemplateFileField> = {};
  for (const item of model.value?.template.fileFieldsJson ?? []) {
    map[item.key] = item;
  }
  return map;
});

const runtimeIncluded = computed(() => {
  if (!model.value) {
    return false;
  }
  if (props.detail.hidden || !props.detail.compatible) {
    return false;
  }
  return model.value.config.enabled && model.value.config.configStatus === "valid";
});

const fieldErrors = computed<Record<string, string>>(() => {
  const errors: Record<string, string> = {};
  for (const field of sortedFields.value) {
    const reason = validateField(field);
    if (reason) {
      errors[field.key] = reason;
    }
  }
  return errors;
});

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

function formatTime(value?: string | null): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function isCopyInvalidHint(message?: string): boolean {
  return Boolean(message && message.includes(COPY_INVALID_HINT));
}

function isSecretField(key: string): boolean {
  return Boolean(model.value?.template.secretFieldsJson.includes(key));
}

function isFileField(key: string): boolean {
  return Boolean(fileFieldMap.value[key]);
}

function getValidationRule(key: string): ValidationRule {
  const rules = model.value?.template.validationRulesJson ?? {};
  const raw = rules[key];
  return (raw && typeof raw === "object" ? raw : {}) as ValidationRule;
}

function hasStoredSecret(key: string): boolean {
  return Boolean(secretStates[key]?.hadStored);
}

function beginEditSecret(key: string) {
  const state = secretStates[key];
  if (!state) {
    return;
  }
  if (!state.editing) {
    state.editing = true;
    state.value = "";
  }
}

function secretInputValue(key: string): string {
  const state = secretStates[key];
  if (!state) {
    return "";
  }
  if (!state.editing && state.hadStored) {
    return "******";
  }
  return state.value;
}

function secretPlaceholder(key: string): string {
  const state = secretStates[key];
  if (!state) {
    return "请输入密钥";
  }
  if (!state.editing && state.hadStored) {
    return "聚焦或点击修改后可重新输入";
  }
  return "请输入密钥，留空则保持原值";
}

function setSecretValue(key: string, value: string) {
  const state = secretStates[key];
  if (!state) {
    return;
  }
  state.editing = true;
  state.value = value;
}

function setDraftValue(key: string, value: unknown) {
  draftConfig[key] = value;
}

function setJsonValue(key: string, value: string) {
  jsonInputs[key] = value;
}

function fileAccept(key: string): string {
  const accept = fileFieldMap.value[key]?.accept ?? [];
  return accept.join(",");
}

function fileRuleText(key: string): string {
  const rule = fileFieldMap.value[key];
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
  return parts.length ? parts.join("；") : "文件字段";
}

function onFileUpload(key: string, options: UploadRequestOptions) {
  const file = options.file;
  const fieldRule = fileFieldMap.value[key];
  if (!fieldRule) {
    options.onError(new Error("文件字段配置缺失") as never);
    return;
  }
  if (fieldRule.maxSizeKB && file.size > fieldRule.maxSizeKB * 1024) {
    const err = new Error(`文件超过 ${fieldRule.maxSizeKB}KB 限制`);
    ElMessage.error(err.message);
    options.onError(err as never);
    return;
  }
  const accepts = fieldRule.accept ?? [];
  if (accepts.length > 0 && !accepts.some((item) => file.name.toLowerCase().endsWith(item.toLowerCase()))) {
    const err = new Error(`文件类型不符合：${accepts.join(", ")}`);
    ElMessage.error(err.message);
    options.onError(err as never);
    return;
  }
  draftConfig[key] = file.name;
  ElMessage.success(`已选择文件：${file.name}`);
  options.onSuccess({ fileName: file.name });
}

function validateField(field: ChannelLoginTemplateField): string | null {
  const key = field.key;
  const required = Boolean(field.required || getValidationRule(key).required);

  if (isSecretField(key)) {
    const state = secretStates[key];
    const trimmed = state?.value.trim() ?? "";
    const nextValue = state?.editing
      ? trimmed || (state.hadStored ? "******" : "")
      : state?.hadStored
        ? "******"
        : "";
    if (required && !nextValue) {
      return "该字段为必填";
    }
    if (nextValue === "******") {
      return null;
    }
    return checkByRule(key, nextValue);
  }

  if (field.component === "json") {
    const raw = (jsonInputs[key] ?? "").trim();
    if (required && raw === "") {
      return "该字段为必填";
    }
    if (raw === "") {
      return null;
    }
    try {
      const parsed = JSON.parse(raw) as unknown;
      return checkByRule(key, parsed);
    } catch {
      return "JSON 格式不合法";
    }
  }

  const value = draftConfig[key];
  const text = toText(value).trim();
  if (required && text === "") {
    return "该字段为必填";
  }
  if (text === "") {
    return null;
  }
  return checkByRule(key, value);
}

function checkByRule(key: string, value: unknown): string | null {
  const rule = getValidationRule(key);
  const text = toText(value);
  if (rule.minLen !== undefined && text.length < rule.minLen) {
    return `长度不能小于 ${rule.minLen}`;
  }
  if (rule.maxLen !== undefined && text.length > rule.maxLen) {
    return `长度不能大于 ${rule.maxLen}`;
  }
  if (rule.pattern) {
    const reg = new RegExp(rule.pattern);
    if (!reg.test(text)) {
      return "格式不符合 pattern 规则";
    }
  }
  if (rule.enum && !rule.enum.some((item) => String(item) === text)) {
    return "值不在枚举范围";
  }
  if (rule.format === "url") {
    try {
      new URL(text);
    } catch {
      return "请输入合法 URL";
    }
  }
  if (rule.format === "email") {
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(text)) {
      return "请输入合法邮箱";
    }
  }
  if (rule.format === "host") {
    if (!/^[a-zA-Z0-9.-]+$/.test(text)) {
      return "请输入合法主机名";
    }
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

function buildPayloadConfig(): Record<string, unknown> {
  const next: Record<string, unknown> = {};
  for (const field of sortedFields.value) {
    const key = field.key;
    if (isSecretField(key)) {
      const state = secretStates[key];
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
      const raw = (jsonInputs[key] ?? "").trim();
      if (raw) {
        next[key] = JSON.parse(raw) as unknown;
      }
      continue;
    }
    const value = draftConfig[key];
    if (value !== undefined) {
      next[key] = value;
    }
  }
  return next;
}

function applyModel(next: ChannelLoginConfigResponse) {
  model.value = next;
  enabled.value = next.config.enabled;
  resetState();

  const config = next.config.configJson || {};
  for (const [key, value] of Object.entries(config)) {
    draftConfig[key] = value;
  }

  for (const field of next.template.formSchemaJson) {
    if (field.component !== "json") {
      continue;
    }
    const raw = config[field.key];
    if (raw === undefined) {
      jsonInputs[field.key] = "";
    } else if (typeof raw === "string") {
      jsonInputs[field.key] = raw;
    } else {
      jsonInputs[field.key] = JSON.stringify(raw, null, 2);
    }
  }

  for (const secretKey of next.template.secretFieldsJson) {
    const raw = toText(config[secretKey]).trim();
    secretStates[secretKey] = {
      editing: false,
      value: "",
      hadStored: raw !== ""
    };
  }
}

async function load() {
  loading.value = true;
  try {
    const res = await getLoginConfig(props.gameChannelId);
    applyModel(res);
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载渠道登录配置失败");
  } finally {
    loading.value = false;
  }
}

async function save() {
  if (!model.value) {
    return;
  }
  const firstError = Object.values(fieldErrors.value).find(Boolean);
  if (firstError) {
    ElMessage.warning(firstError);
    return;
  }
  saving.value = true;
  try {
    const saved = await putLoginConfig(props.gameChannelId, {
      enabled: enabled.value,
      templateVersion: model.value.template.templateVersion,
      configJson: buildPayloadConfig()
    });
    applyModel(saved);
    ElMessage.success("渠道登录配置已保存");
    emit("changed");
  } catch (err) {
    if (err instanceof ApiError) {
      ElMessage.error(err.message || "保存失败");
      if (err.code === "VALIDATION_FAILED") {
        await load();
        emit("changed");
      }
    } else {
      ElMessage.error("保存失败");
    }
  } finally {
    saving.value = false;
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

.panel__context {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.panel__status {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.panel__status-message {
  color: var(--text-subtle);
  font-size: 12px;
}

.panel__alert {
  margin: 0;
}

.panel__form {
  margin-top: 6px;
}

.group-title {
  margin: 14px 0 8px;
  font-size: 14px;
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

.panel__actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}

.full-width {
  width: 100%;
}
</style>
