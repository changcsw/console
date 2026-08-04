import type {
  ChannelTemplate,
  TemplateFieldDef,
  TemplateFileFieldDef,
  TemplateRuleDef
} from "@/api/modules/platformChannels";

// 渠道模版四件套的编辑态草稿与提交载荷之间的纯转换。
// 与后端 domain/channel.ValidateChannelTemplate 同口径做前置提示，服务端仍会二次强制校验。

/** 抽屉里的模版四件套编辑态。合并为单个字段列表以便于 UI 呈现与编辑。 */
export interface EditorField extends TemplateFieldDef {
  /** 是否登记为敏感字段 */
  _isSecret: boolean;
  /** 文件字段的后缀限制（逗号分隔字符串） */
  _fileAcceptText: string;
  /** 文件字段的大小限制（KB） */
  _fileMaxSizeKB?: number;
  /** 该字段的校验规则 */
  _rules: TemplateRuleDef;
}

export interface TemplateDraft {
  fields: EditorField[];
}

export interface TemplateQuartetPayload {
  formSchemaJson: TemplateFieldDef[];
  secretFieldsJson: string[];
  fileFieldsJson: TemplateFileFieldDef[];
  validationRulesJson: Record<string, TemplateRuleDef>;
}

/** 占位提示只对文本类输入生效；select/switch/number/file/json 渲染时不读取 placeholder，编辑器与提交都应忽略。 */
const PLACEHOLDER_COMPONENTS = new Set(["input", "textarea", "password"]);

export function supportsPlaceholder(component: string): boolean {
  return PLACEHOLDER_COMPONENTS.has(component);
}

export function emptyField(order: number): EditorField {
  return {
    key: "",
    label: "",
    component: "input",
    required: false,
    order,
    group: "",
    scope: "",
    placeholder: "",
    _isSecret: false,
    _fileAcceptText: "",
    _rules: { required: false, minLen: undefined, maxLen: undefined, min: undefined, max: undefined, pattern: undefined, format: undefined }
  };
}

export function emptyDraft(): TemplateDraft {
  return { fields: [emptyField(10)] };
}

export function draftFromTemplate(tpl: ChannelTemplate): TemplateDraft {
  const secretSet = new Set(tpl.secretFieldsJson ?? []);
  const fileMap = new Map((tpl.fileFieldsJson ?? []).map((file) => [file.key, file]));
  const rulesMap = tpl.validationRulesJson ?? {};

  const fields: EditorField[] = tpl.formSchemaJson.map((field) => {
    const fileDef = fileMap.get(field.key);
    const ruleDef = rulesMap[field.key] ?? { required: field.required ?? false };
    return {
      key: field.key,
      label: field.label,
      component: field.component,
      required: field.required ?? false,
      order: field.order ?? 0,
      group: field.group ?? "",
      // 历史数据里的 "both" 归一到 ""：两者语义等价，编辑器不再区分展示。
      scope: field.scope === "both" ? "" : (field.scope ?? ""),
      placeholder: field.placeholder ?? "",
      options: field.options ? field.options.map((opt) => ({ ...opt })) : undefined,
      _isSecret: secretSet.has(field.key),
      _fileAcceptText: acceptToText(fileDef?.accept),
      _fileMaxSizeKB: fileDef?.maxSizeKB,
      _rules: { minLen: undefined, maxLen: undefined, min: undefined, max: undefined, pattern: undefined, format: undefined, ...ruleDef }
    };
  });

  return { fields };
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

  const passwordUnregistered = draft.fields.find(
    (field) => field.component === "password" && !field._isSecret
  );
  if (passwordUnregistered) {
    return { error: `口令字段「${passwordUnregistered.key}」必须登记为敏感字段` };
  }

  const validFields = draft.fields.filter((field) => field.key.trim() !== "");
  const secretFieldsJson = validFields.filter((field) => field._isSecret).map((field) => field.key.trim());
  const fileFieldsJson = validFields
    .filter((field) => field.component === "file")
    .map((field) => {
      const out: TemplateFileFieldDef = { key: field.key.trim() };
      const accept = textToAccept(field._fileAcceptText);
      if (accept.length > 0) {
        out.accept = accept;
      }
      if (field._fileMaxSizeKB != null && field._fileMaxSizeKB > 0) {
        out.maxSizeKB = field._fileMaxSizeKB;
      }
      return out;
    });

  const validationRulesJson: Record<string, TemplateRuleDef> = {};
  for (const field of validFields) {
    const cleanRules: TemplateRuleDef = { required: field.required ?? false };
    if (field._rules.minLen != null) cleanRules.minLen = field._rules.minLen;
    if (field._rules.maxLen != null) cleanRules.maxLen = field._rules.maxLen;
    if (field._rules.min != null) cleanRules.min = field._rules.min;
    if (field._rules.max != null) cleanRules.max = field._rules.max;
    if (field._rules.pattern) cleanRules.pattern = field._rules.pattern;
    if (field._rules.format) cleanRules.format = field._rules.format;
    validationRulesJson[field.key.trim()] = cleanRules;
  }

  return {
    payload: {
      formSchemaJson: validFields.map((field) => normalizeField(field)),
      secretFieldsJson,
      fileFieldsJson,
      validationRulesJson
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
    // "both" 与 "" 语义等价，提交前统一归一到 ""，避免新旧数据混用两种写法。
    scope: field.scope === "both" ? "" : (field.scope ?? "")
  };
  const placeholder = (field.placeholder ?? "").trim();
  if (placeholder !== "" && supportsPlaceholder(field.component)) {
    out.placeholder = placeholder;
  }
  if (field.component === "select") {
    out.options = (field.options ?? [])
      .filter((opt) => opt.label !== "")
      .map((opt) => ({ label: opt.label, value: opt.value }));
  }
  return out;
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
