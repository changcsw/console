<template>
  <div class="quartet">
    <section class="quartet__section">
      <div class="quartet__head">
        <h4 class="quartet__title">表单字段</h4>
        <el-button link type="primary" @click="addField">添加字段</el-button>
      </div>
      <p class="quartet__hint">
        游戏侧在渠道实例上按此 schema 渲染配置表单。口令字段须登记为敏感字段、文件字段须登记到文件字段列表、下拉字段须配置候选项。
      </p>

      <div v-for="(field, index) in draft.fields" :key="index" class="field">
        <div class="field__grid">
          <el-input v-model="field.key" placeholder="key（字母开头，字母/数字/下划线）" class="field__key" />
          <el-input v-model="field.label" placeholder="标签" />
          <el-select v-model="field.component" placeholder="组件">
            <el-option v-for="c in TEMPLATE_COMPONENT_OPTIONS" :key="c" :label="c" :value="c" />
          </el-select>
          <el-select v-model="field.scope" placeholder="scope">
            <el-option v-for="s in TEMPLATE_SCOPE_OPTIONS" :key="s" :label="scopeLabel(s)" :value="s" />
          </el-select>
          <el-input-number v-model="field.order" :min="0" :max="9999" controls-position="right" class="field__order" />
          <el-input v-model="field.group" placeholder="分组（可空）" />
          <el-input v-model="field.placeholder" placeholder="占位提示（可空）" />
          <el-checkbox v-model="field.required">必填</el-checkbox>
          <el-button link type="danger" @click="removeField(index)">删除</el-button>
        </div>

        <div v-if="field.component === 'select'" class="field__options">
          <div class="quartet__head">
            <span class="field__options-title">候选项</span>
            <el-button link type="primary" @click="addOption(field)">添加候选项</el-button>
          </div>
          <div v-for="(opt, optIndex) in field.options ?? []" :key="optIndex" class="option-row">
            <el-input v-model="opt.label" placeholder="显示文案" />
            <el-input :model-value="String(opt.value)" placeholder="取值" @update:model-value="opt.value = $event" />
            <el-button link type="danger" @click="removeOption(field, optIndex)">删除</el-button>
          </div>
          <p v-if="(field.options ?? []).length === 0" class="quartet__hint">下拉字段至少需要一个候选项。</p>
        </div>
      </div>
      <p v-if="draft.fields.length === 0" class="quartet__hint">至少需要一个表单字段。</p>
    </section>

    <section class="quartet__section">
      <h4 class="quartet__title">敏感字段</h4>
      <p class="quartet__hint">登记为敏感字段的值会加密入库并在读接口脱敏。候选来自上方已填的字段 key。</p>
      <el-select v-model="draft.secretFields" multiple placeholder="选择敏感字段" class="quartet__select">
        <el-option v-for="key in availableKeys" :key="key" :label="key" :value="key" />
      </el-select>
    </section>

    <section class="quartet__section">
      <h4 class="quartet__title">文件字段</h4>
      <p class="quartet__hint">登记为文件字段的值走文件上传通道。候选来自上方已填的字段 key。</p>
      <el-select
        :model-value="fileFieldKeys"
        multiple
        placeholder="选择文件字段"
        class="quartet__select"
        @update:model-value="onFileKeysChange"
      >
        <el-option v-for="key in availableKeys" :key="key" :label="key" :value="key" />
      </el-select>
      <div v-for="file in draft.fileFields" :key="file.key" class="file-row">
        <span class="file-row__key">{{ file.key }}</span>
        <el-input
          :model-value="acceptToText(file.accept)"
          placeholder="允许后缀，逗号分隔，如 .jks, .keystore"
          @update:model-value="file.accept = textToAccept($event)"
        />
        <el-input-number
          v-model="file.maxSizeKB"
          :min="1"
          :max="102400"
          placeholder="上限 KB"
          controls-position="right"
        />
      </div>
    </section>

    <section class="quartet__section">
      <h4 class="quartet__title">校验规则</h4>
      <p class="quartet__hint">
        JSON 对象，键为字段 key，值支持 required / minLen / maxLen / min / max / pattern / format / enum。
      </p>
      <el-input v-model="draft.rulesText" type="textarea" :rows="6" spellcheck="false" />
      <p v-if="rulesError" class="quartet__error" role="alert">{{ rulesError }}</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import {
  TEMPLATE_COMPONENT_OPTIONS,
  TEMPLATE_SCOPE_OPTIONS,
  type TemplateFieldDef
} from "@/api/modules/platformChannels";
import { scopeLabel } from "./labels";
import { acceptToText, emptyField, fieldKeys, parseRules, textToAccept, type TemplateDraft } from "./templateDraft";

// draft 由抽屉持有的 reactive 草稿，本编辑器就地增删改；提交与整体校验由抽屉负责（draftToPayload）。
const props = defineProps<{
  draft: TemplateDraft;
}>();

const draft = computed(() => props.draft);

const availableKeys = computed(() => fieldKeys(props.draft));
const fileFieldKeys = computed(() => props.draft.fileFields.map((file) => file.key));

const rulesError = computed(() => {
  const parsed = parseRules(props.draft.rulesText);
  return "error" in parsed ? parsed.error : "";
});

function addField() {
  const nextOrder = (props.draft.fields.at(-1)?.order ?? 0) + 10;
  props.draft.fields.push(emptyField(nextOrder));
}

function removeField(index: number) {
  const [removed] = props.draft.fields.splice(index, 1);
  if (!removed) {
    return;
  }
  // 字段删掉后，敏感/文件字段里的悬空登记一并清理，避免提交出「未声明的 key」。
  props.draft.secretFields = props.draft.secretFields.filter((key) => key !== removed.key);
  props.draft.fileFields = props.draft.fileFields.filter((file) => file.key !== removed.key);
}

function addOption(field: TemplateFieldDef) {
  if (!field.options) {
    field.options = [];
  }
  field.options.push({ label: "", value: "" });
}

function removeOption(field: TemplateFieldDef, index: number) {
  field.options?.splice(index, 1);
}

function onFileKeysChange(keys: string[]) {
  // 保留已填的 accept / maxSizeKB，仅按选择结果增删。
  props.draft.fileFields = keys.map(
    (key) => props.draft.fileFields.find((file) => file.key === key) ?? { key }
  );
}
</script>

<style scoped>
.quartet {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.quartet__section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.quartet__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.quartet__title {
  margin: 0;
  font-size: 14px;
}

.quartet__hint {
  color: var(--text-subtle);
  font-size: 12px;
  margin: 0;
}

.quartet__error {
  color: var(--danger);
  font-size: 13px;
  margin: 0;
}

.quartet__select {
  width: 100%;
}

.field {
  border: 1px solid var(--border-subtle, #e4e7ed);
  border-radius: 6px;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.field__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  align-items: center;
}

.field__key {
  grid-column: span 2;
}

.field__order {
  width: 100%;
}

.field__options {
  border-top: 1px dashed var(--border-subtle, #e4e7ed);
  padding-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field__options-title {
  font-size: 13px;
  font-weight: 600;
}

.option-row,
.file-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.file-row__key {
  min-width: 96px;
  font-size: 13px;
}
</style>
