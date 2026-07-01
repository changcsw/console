<template>
  <el-form label-position="top" class="template-form">
    <el-form-item v-for="field in sortedFields" :key="field.key" :label="field.label || field.key">
      <template #label>
        <div class="field-label">
          <span>{{ field.label || field.key }}</span>
          <el-tag v-if="field.scope === 'server'" size="small" type="warning">仅服务端，不下发客户端</el-tag>
        </div>
      </template>
      <template v-if="isSecretField(field.key)">
        <div class="secret-input">
          <code v-if="hasStoredSecret(field.key)" class="secret-input__masked">{{ maskedText(field.key) }}</code>
          <el-input
            :model-value="secretText(field.key)"
            :disabled="disabled"
            show-password
            :placeholder="hasStoredSecret(field.key) ? '留空则不修改' : '请输入新值'"
            @update:model-value="setSecretValue(field.key, $event)"
          />
        </div>
      </template>

      <template v-else-if="field.component === 'switch'">
        <el-switch :model-value="Boolean(draft[field.key])" :disabled="disabled" @update:model-value="setValue(field.key, $event)" />
      </template>

      <template v-else-if="field.component === 'number'">
        <el-input-number
          :model-value="toNumberOrNull(draft[field.key])"
          :disabled="disabled"
          controls-position="right"
          @update:model-value="setValue(field.key, $event)"
        />
      </template>

      <template v-else-if="field.component === 'select'">
        <el-select
          :model-value="toText(draft[field.key])"
          :disabled="disabled"
          class="full-width"
          clearable
          @update:model-value="setValue(field.key, $event)"
        >
          <el-option v-for="opt in field.options ?? []" :key="String(opt.value)" :label="opt.label" :value="String(opt.value)" />
        </el-select>
      </template>

      <template v-else-if="field.component === 'json'">
        <el-input
          :model-value="jsonInputs[field.key] ?? ''"
          type="textarea"
          :rows="4"
          :disabled="disabled"
          placeholder="请输入 JSON 对象"
          @update:model-value="setJsonText(field.key, $event)"
          @blur="commitJson(field.key)"
        />
        <p v-if="jsonErrors[field.key]" class="field-error">{{ jsonErrors[field.key] }}</p>
      </template>

      <template v-else-if="field.component === 'file' || isFileField(field.key)">
        <div class="file-input">
          <el-upload
            :show-file-list="false"
            :accept="fileAccept(field.key)"
            :disabled="disabled"
            :http-request="(options: UploadRequestOptions) => onFileUpload(field.key, options)"
          >
            <el-button :disabled="disabled">上传文件</el-button>
          </el-upload>
          <span class="file-input__meta">
            {{ fileRuleText(field.key) }}
            <template v-if="toText(draft[field.key])">｜当前：{{ toText(draft[field.key]) }}</template>
          </span>
        </div>
      </template>

      <template v-else>
        <el-input
          :model-value="toText(draft[field.key])"
          :type="field.component === 'textarea' ? 'textarea' : field.component === 'password' ? 'password' : 'text'"
          :rows="field.component === 'textarea' ? 3 : undefined"
          :show-password="field.component === 'password'"
          :disabled="disabled"
          @update:model-value="setValue(field.key, $event)"
        />
      </template>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { ElMessage } from "element-plus";
import type { UploadRequestOptions } from "element-plus";
import { computed, ref, watch } from "vue";
import type { IapTemplate } from "@/api/modules/products";

const props = withDefaults(
  defineProps<{
    template: IapTemplate;
    modelValue: Record<string, unknown>;
    secretValues: Record<string, string>;
    disabled?: boolean;
  }>(),
  {
    disabled: false
  }
);

const emit = defineEmits<{
  (e: "update:modelValue", value: Record<string, unknown>): void;
  (e: "update:secretValues", value: Record<string, string>): void;
  (e: "json-error-change", hasError: boolean): void;
}>();

const jsonInputs = ref<Record<string, string>>({});
const jsonErrors = ref<Record<string, string>>({});

const sortedFields = computed(() => {
  return [...(props.template.formSchema ?? [])].sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
});

const draft = computed(() => props.modelValue ?? {});

watch(
  () => [props.modelValue, props.template.formSchema] as const,
  () => {
    const next: Record<string, string> = {};
    for (const field of props.template.formSchema ?? []) {
      if (field.component !== "json") {
        continue;
      }
      const raw = props.modelValue?.[field.key];
      if (raw === undefined) {
        next[field.key] = "";
      } else if (typeof raw === "string") {
        next[field.key] = raw;
      } else {
        next[field.key] = JSON.stringify(raw, null, 2);
      }
    }
    jsonInputs.value = next;
    jsonErrors.value = {};
    emit("json-error-change", false);
  },
  { immediate: true }
);

function isSecretField(key: string): boolean {
  return (props.template.secretFields ?? []).includes(key);
}

function isFileField(key: string): boolean {
  return (props.template.fileFields ?? []).some((item) => item.key === key);
}

function hasStoredSecret(key: string): boolean {
  const value = props.modelValue?.[key];
  return value !== null && value !== undefined && String(value).trim() !== "";
}

function maskedText(key: string): string {
  const value = props.modelValue?.[key];
  return value === undefined || value === null || value === "" ? "masked" : String(value);
}

function secretText(key: string): string {
  return props.secretValues?.[key] ?? "";
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
  const parsed = Number(value);
  return Number.isNaN(parsed) ? null : parsed;
}

function setValue(key: string, value: unknown) {
  const next = { ...(props.modelValue ?? {}) };
  if (value === undefined || value === null || value === "") {
    delete next[key];
  } else {
    next[key] = value;
  }
  emit("update:modelValue", next);
}

function setSecretValue(key: string, value: string | number) {
  const next = { ...(props.secretValues ?? {}) };
  next[key] = String(value ?? "");
  emit("update:secretValues", next);
}

function setJsonText(key: string, value: string | number) {
  jsonInputs.value = { ...jsonInputs.value, [key]: String(value ?? "") };
}

function commitJson(key: string) {
  const raw = (jsonInputs.value[key] ?? "").trim();
  const nextErrors = { ...jsonErrors.value };
  const nextModel = { ...(props.modelValue ?? {}) };
  if (!raw) {
    delete nextModel[key];
    delete nextErrors[key];
    jsonErrors.value = nextErrors;
    emit("update:modelValue", nextModel);
    emit("json-error-change", Object.keys(nextErrors).length > 0);
    return;
  }
  try {
    nextModel[key] = JSON.parse(raw) as unknown;
    delete nextErrors[key];
    jsonErrors.value = nextErrors;
    emit("update:modelValue", nextModel);
    emit("json-error-change", Object.keys(nextErrors).length > 0);
  } catch {
    nextErrors[key] = `${key} 不是合法 JSON`;
    jsonErrors.value = nextErrors;
    emit("json-error-change", true);
  }
}

function fileRule(key: string) {
  return (props.template.fileFields ?? []).find((item) => item.key === key);
}

function fileAccept(key: string): string {
  return (fileRule(key)?.accept ?? []).join(",");
}

function fileRuleText(key: string): string {
  const rule = fileRule(key);
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

function onFileUpload(key: string, options: UploadRequestOptions) {
  const rule = fileRule(key);
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
  setValue(key, file.name);
  ElMessage.success(`已选择文件：${file.name}`);
  options.onSuccess({ fileName: file.name });
}
</script>

<style scoped>
.template-form {
  width: 100%;
}

.field-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.secret-input {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.secret-input__masked {
  flex-shrink: 0;
  color: var(--text-subtle);
  font-size: 12px;
}

.file-input {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.file-input__meta {
  color: var(--text-subtle);
  font-size: 12px;
}

.field-error {
  margin: 6px 0 0;
  color: var(--danger);
  font-size: 12px;
}

.full-width {
  width: 100%;
}
</style>
