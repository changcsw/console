import type {
  ChannelTemplate,
  TemplateFieldDef,
  TemplateFileFieldDef,
  TemplateRuleDef
} from "@/api/modules/platformChannels";

// 渠道模版四件套的编辑态草稿与提交载荷之间的纯转换。
// 与后端 domain/channel.ValidateChannelTemplate 同口径做前置提示，服务端仍会二次强制校验。

/** 抽屉里的模版四件套编辑态。validationRulesJson 用 JSON 文本直填，提交前解析。 */
export interface TemplateDraft {
  fields: TemplateFieldDef[];
  secretFields: string[];
  fileFields: TemplateFileFieldDef[];
  rulesText: string;
}

export interface TemplateQuartetPayload {
  formSchemaJson: TemplateFieldDef[];
  secretFieldsJson: string[];
  fileFieldsJson: TemplateFileFieldDef[];
  validationRulesJson: Record<string, TemplateRuleDef>;
}

export function emptyField(order: number): TemplateFieldDef {
  return { key: "", label: "", component: "input", required: false, order, group: "", scope: "", placeholder: "" };
}

export function emptyDraft(): TemplateDraft {
  return { fields: [emptyField(10)], secretFields: [], fileFields: [], rulesText: "{}" };
}

export function draftFromTemplate(tpl: ChannelTemplate): TemplateDraft {
  return {
    fields: tpl.formSchemaJson.map((field) => ({
      key: field.key,
      label: field.label,
      component: field.component,
      required: field.required ?? false,
      order: field.order ?? 0,
      group: field.group ?? "",
      scope: field.scope ?? "",
      placeholder: field.placeholder ?? "",
      options: field.options ? field.options.map((opt) => ({ ...opt })) : undefined
    })),
    secretFields: [...tpl.secretFieldsJson],
    fileFields: tpl.fileFieldsJson.map((file) => ({ ...file, accept: file.accept ? [...file.accept] : undefined })),
    rulesText: JSON.stringify(tpl.validationRulesJson ?? {}, null, 2)
  };
}

/** 已填的字段 key，供敏感字段/文件字段的候选与越界剔除使用。 */
export function fieldKeys(draft: TemplateDraft): string[] {
  return draft.fields.map((field) => field.key.trim()).filter((key) => key !== "");
}

/**
 * 把草稿转为提交载荷。校验失败返回 error 文案（前置提示，避免无谓往返）。
 */
export function draftToPayload(draft: TemplateDraft): { payload: TemplateQuartetPayload } | { error: string } {
  const keys = fieldKeys(draft);
  if (keys.length === 0) {
    return { error: "至少需要一个表单字段，且字段 key 必填" };
  }
  const duplicated = keys.find((key, index) => keys.indexOf(key) !== index);
  if (duplicated) {
    return { error: `字段 key 重复：${duplicated}` };
  }
  const missingLabel = draft.fields.find((field) => field.key.trim() !== "" && field.label.trim() === "");
  if (missingLabel) {
    return { error: `字段「${missingLabel.key}」缺少标签` };
  }
  const selectWithoutOptions = draft.fields.find(
    (field) => field.component === "select" && (field.options ?? []).filter((opt) => opt.label !== "").length === 0
  );
  if (selectWithoutOptions) {
    return { error: `下拉字段「${selectWithoutOptions.key}」必须配置候选项` };
  }

  const secretFieldsJson = draft.secretFields.filter((key) => keys.includes(key));
  const fileFieldsJson = draft.fileFields.filter((file) => keys.includes(file.key));
  const secretSet = new Set(secretFieldsJson);
  const fileSet = new Set(fileFieldsJson.map((file) => file.key));

  const passwordUnregistered = draft.fields.find(
    (field) => field.component === "password" && !secretSet.has(field.key.trim())
  );
  if (passwordUnregistered) {
    return { error: `口令字段「${passwordUnregistered.key}」必须登记为敏感字段` };
  }
  const fileUnregistered = draft.fields.find((field) => field.component === "file" && !fileSet.has(field.key.trim()));
  if (fileUnregistered) {
    return { error: `文件字段「${fileUnregistered.key}」必须登记到文件字段列表` };
  }

  const rules = parseRules(draft.rulesText);
  if ("error" in rules) {
    return { error: rules.error };
  }
  const unknownRuleKey = Object.keys(rules.value).find((key) => !keys.includes(key));
  if (unknownRuleKey) {
    return { error: `校验规则字段未在表单中声明：${unknownRuleKey}` };
  }

  return {
    payload: {
      formSchemaJson: draft.fields
        .filter((field) => field.key.trim() !== "")
        .map((field) => normalizeField(field)),
      secretFieldsJson,
      fileFieldsJson: fileFieldsJson.map((file) => normalizeFileField(file)),
      validationRulesJson: rules.value
    }
  };
}

function normalizeField(field: TemplateFieldDef): TemplateFieldDef {
  const out: TemplateFieldDef = {
    key: field.key.trim(),
    label: field.label.trim(),
    component: field.component,
    required: field.required ?? false,
    order: field.order ?? 0,
    group: (field.group ?? "").trim(),
    scope: field.scope ?? ""
  };
  const placeholder = (field.placeholder ?? "").trim();
  if (placeholder !== "") {
    out.placeholder = placeholder;
  }
  if (field.component === "select") {
    out.options = (field.options ?? [])
      .filter((opt) => opt.label !== "")
      .map((opt) => ({ label: opt.label, value: opt.value }));
  }
  return out;
}

function normalizeFileField(file: TemplateFileFieldDef): TemplateFileFieldDef {
  const out: TemplateFileFieldDef = { key: file.key.trim() };
  const accept = (file.accept ?? []).map((item) => item.trim()).filter((item) => item !== "");
  if (accept.length > 0) {
    out.accept = accept;
  }
  if (file.maxSizeKB != null && file.maxSizeKB > 0) {
    out.maxSizeKB = file.maxSizeKB;
  }
  return out;
}

/** 解析 validationRulesJson 文本。空文本视为 {}。 */
export function parseRules(text: string): { value: Record<string, TemplateRuleDef> } | { error: string } {
  const trimmed = text.trim();
  if (trimmed === "") {
    return { value: {} };
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return { error: "校验规则不是合法 JSON" };
  }
  if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
    return { error: "校验规则必须是 JSON 对象（形如 { 字段key: { required: true } }）" };
  }
  for (const [key, rule] of Object.entries(parsed as Record<string, unknown>)) {
    if (rule === null || typeof rule !== "object" || Array.isArray(rule)) {
      return { error: `校验规则「${key}」必须是对象` };
    }
  }
  return { value: parsed as Record<string, TemplateRuleDef> };
}

/** accept 数组与逗号分隔文本的互转（编辑器里用文本填写）。 */
export function acceptToText(accept?: string[]): string {
  return (accept ?? []).join(", ");
}

export function textToAccept(text: string): string[] {
  return text
    .split(",")
    .map((item) => item.trim())
    .filter((item) => item !== "");
}
