<template>
  <div class="quartet">
    <section class="quartet__section">
      <div class="quartet__head">
        <h4 class="quartet__title">表单字段</h4>
        <el-button link type="primary" @click="addField">添加字段</el-button>
      </div>
      <p class="quartet__hint">
        游戏侧在渠道实例上按此 schema 渲染配置表单。
      </p>

      <div v-for="(field, index) in draft.fields" :key="index" class="field">
        <div class="field__grid">
          <el-input v-model="field.key" placeholder="key（字母开头，字母/数字/下划线）" class="field__key" />
          <el-input v-model="field.label" placeholder="标签" />
          <el-select v-model="field.component" placeholder="组件">
            <el-option v-for="c in TEMPLATE_COMPONENT_OPTIONS" :key="c" :label="c" :value="c" />
          </el-select>
          <el-select :model-value="scopeDisplayValue(field)" @update:model-value="(v: string) => setScopeValue(field, v)">
            <el-option v-for="s in TEMPLATE_SCOPE_OPTIONS" :key="s || SCOPE_SENTINEL" :label="scopeLabel(s)" :value="s || SCOPE_SENTINEL" />
          </el-select>
          <div class="field__order-wrap" title="控制字段在渲染表单里的展示顺序，与字符长度无关，所有组件类型都适用">
            <span class="field__order-label">排序</span>
            <el-input-number
              v-model="field.order"
              :min="0"
              :max="9999"
              controls-position="right"
              class="field__order"
            />
          </div>
          <el-input v-model="field.group" placeholder="分组（可空，同分组的字段渲染时归为一节）" />
          <el-input
            v-if="supportsPlaceholder(field.component)"
            v-model="field.placeholder"
            placeholder="占位提示（可空）"
          />
          <el-input v-else disabled model-value="" placeholder="该组件类型不支持占位提示" />
          <el-checkbox v-model="field.required">必填</el-checkbox>
          <el-checkbox
            v-if="field.component === 'password' || field.component === 'input'"
            v-model="field._isSecret"
            title="登记为敏感字段的值会加密入库并在读接口脱敏"
          >
            敏感字段
          </el-checkbox>
          <el-button link type="danger" @click="removeField(index)">删除</el-button>
        </div>

        <!-- 候选项配置 -->
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

        <!-- 文件约束配置 -->
        <div v-if="field.component === 'file'" class="field__options">
          <div class="quartet__head">
            <span class="field__options-title">文件约束</span>
          </div>
          <div class="file-row">
            <el-input
              v-model="field._fileAcceptText"
              placeholder="允许后缀，逗号分隔，如 .jks, .keystore"
              class="file-row__accept"
            />
            <div class="file-row__maxsize-wrap">
              <span class="file-row__maxsize-label">上限 KB</span>
              <el-input-number
                v-model="field._fileMaxSizeKB"
                :min="1"
                :max="102400"
                controls-position="right"
                class="file-row__maxsize"
              />
            </div>
          </div>
        </div>

        <!-- 校验规则配置 -->
        <div class="field__options">
          <div class="quartet__head">
            <span class="field__options-title">附加校验规则</span>
          </div>
          <div class="rules-grid">
            <div v-if="field.component === 'input' || field.component === 'password' || field.component === 'textarea'" class="rule-item">
              <span class="rule-label">最小长度</span>
              <el-input-number v-model="field._rules.minLen" :min="0" controls-position="right" class="rule-input" />
            </div>
            <div v-if="field.component === 'input' || field.component === 'password' || field.component === 'textarea'" class="rule-item">
              <span class="rule-label">最大长度</span>
              <el-input-number v-model="field._rules.maxLen" :min="0" controls-position="right" class="rule-input" />
            </div>
            <div v-if="field.component === 'number'" class="rule-item">
              <span class="rule-label">最小值</span>
              <el-input-number v-model="field._rules.min" controls-position="right" class="rule-input" />
            </div>
            <div v-if="field.component === 'number'" class="rule-item">
              <span class="rule-label">最大值</span>
              <el-input-number v-model="field._rules.max" controls-position="right" class="rule-input" />
            </div>
            <div v-if="field.component === 'input' || field.component === 'password' || field.component === 'textarea'" class="rule-item">
              <span class="rule-label">正则(pattern)</span>
              <el-input v-model="field._rules.pattern" placeholder="例如 ^[a-z]+$" class="rule-input-text" />
            </div>
            <div v-if="field.component === 'input' || field.component === 'password' || field.component === 'textarea'" class="rule-item">
              <span class="rule-label">格式(format)</span>
              <el-input v-model="field._rules.format" placeholder="例如 url, email" class="rule-input-text" />
            </div>
          </div>
        </div>
      </div>
      <p v-if="draft.fields.length === 0" class="quartet__hint">至少需要一个表单字段。</p>
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
import {
  emptyField,
  supportsPlaceholder,
  type EditorField,
  type TemplateDraft
} from "./templateDraft";

// draft 由抽屉持有的 reactive 草稿，本编辑器就地增删改；提交与整体校验由抽屉负责（draftToPayload）。
const props = defineProps<{
  draft: TemplateDraft;
}>();

const draft = computed(() => props.draft);

// el-select 对空字符串这个"合法但为假值"的绑定有已知坑：即便存在 value="" 的候选项，也会一直显示
// placeholder 而不是该候选项的 label。这里用一个非空占位值只在控件展示层做映射，真实存的 field.scope
// 仍然是 ""（对应"不区分"，与 "both" 语义等价，见 templateDraft.ts）。
const SCOPE_SENTINEL = "__unspecified__";

function scopeDisplayValue(field: TemplateFieldDef): string {
  return field.scope ? field.scope : SCOPE_SENTINEL;
}

function setScopeValue(field: TemplateFieldDef, value: string) {
  field.scope = value === SCOPE_SENTINEL ? "" : (value as TemplateFieldDef["scope"]);
}

function addField() {
  const nextOrder = (props.draft.fields.at(-1)?.order ?? 0) + 10;
  props.draft.fields.push(emptyField(nextOrder));
}

function removeField(index: number) {
  props.draft.fields.splice(index, 1);
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

.field__order-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.field__order-label {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--text-subtle);
  white-space: nowrap;
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

.option-row {
  display: flex;
  gap: 8px;
  align-items: center;
}

.file-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.file-row__accept {
  flex: 1 1 220px;
  min-width: 160px;
}

.file-row__maxsize-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 0 0 auto;
}

.file-row__maxsize-label {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--text-subtle);
  white-space: nowrap;
}

.file-row__maxsize {
  width: 110px;
  flex-shrink: 0;
}

.rules-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.rule-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.rule-label {
  font-size: 12px;
  color: var(--text-subtle);
  white-space: nowrap;
}

.rule-input {
  width: 100px;
}

.rule-input-text {
  width: 160px;
}
</style>
