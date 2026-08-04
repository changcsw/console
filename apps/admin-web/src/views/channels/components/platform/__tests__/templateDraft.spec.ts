import { describe, expect, test } from "vitest";
import type { ChannelTemplate } from "@/api/modules/platformChannels";
import {
  acceptToText,
  draftFromTemplate,
  draftToPayload,
  emptyDraft,
  fieldKeys,
  textToAccept,
  type TemplateDraft,
  type EditorField
} from "../templateDraft";

function draft(overrides: { fields?: Partial<EditorField>[] } = {}): TemplateDraft {
  const defaultField: EditorField = {
    key: "appId",
    label: "App ID",
    component: "input",
    required: true,
    order: 10,
    group: "",
    scope: "both",
    _isSecret: false,
    _fileAcceptText: "",
    _rules: {}
  };
  return {
    fields: (overrides.fields ?? [defaultField]).map((f) => ({ ...defaultField, ...f })) as EditorField[]
  };
}

describe("templateDraft", () => {
  test("emptyDraft 以一个空字段起步", () => {
    const d = emptyDraft();
    expect(d.fields).toHaveLength(1);
    expect(d.fields[0].component).toBe("input");
    expect(d.fields[0]._rules).toEqual({ required: false, minLen: undefined, maxLen: undefined, min: undefined, max: undefined, pattern: undefined, format: undefined });
  });

  test("fieldKeys 剔除空白 key", () => {
    const d = draft({
      fields: [
        { key: "appId", label: "A", component: "input" },
        { key: "  ", label: "B", component: "input" }
      ]
    });
    expect(fieldKeys(d)).toEqual(["appId"]);
  });

  test("合法草稿转为提交载荷，并归一化四件套", () => {
    const result = draftToPayload(
      draft({
        fields: [
          { key: " appId ", label: " App ID ", component: "input", required: true, order: 10, placeholder: "  ", _rules: { maxLen: 32 } },
          { key: "appSecret", label: "密钥", component: "password", order: 20, _isSecret: true },
          { key: "keystore", label: "签名文件", component: "file", order: 30, _fileAcceptText: " .jks ", _fileMaxSizeKB: 2048 }
        ]
      })
    );
    expect("payload" in result).toBe(true);
    if (!("payload" in result)) {
      return;
    }
    const { payload } = result;
    expect(payload.formSchemaJson[0].key).toBe("appId");
    expect(payload.formSchemaJson[0].label).toBe("App ID");
    // 空白 placeholder 不下发
    expect(payload.formSchemaJson[0].placeholder).toBeUndefined();
    expect(payload.secretFieldsJson).toEqual(["appSecret"]);
    expect(payload.fileFieldsJson).toEqual([{ key: "keystore", accept: [".jks"], maxSizeKB: 2048 }]);
    expect(payload.validationRulesJson).toEqual({
      appId: { required: true, maxLen: 32 },
      appSecret: { required: true },
      keystore: { required: true }
    });
  });

  test("select 字段下发候选项，非 select 字段不带 options", () => {
    const result = draftToPayload(
      draft({
        fields: [
          { key: "appId", label: "App", component: "input", options: [{ label: "x", value: "y" }] },
          {
            key: "mode",
            label: "模式",
            component: "select",
            options: [
              { label: "静默", value: "silent" },
              { label: "", value: "dropped" }
            ]
          }
        ]
      })
    );
    if (!("payload" in result)) {
      throw new Error(result.error);
    }
    expect(result.payload.formSchemaJson[0].options).toBeUndefined();
    expect(result.payload.formSchemaJson[1].options).toEqual([{ label: "静默", value: "silent" }]);
  });

  test.each([
    ["字段全为空", draft({ fields: [{ key: "", label: "", component: "input" }] }), "至少需要一个表单字段"],
    [
      "key 重复",
      draft({
        fields: [
          { key: "appId", label: "A", component: "input" },
          { key: "appId", label: "B", component: "input" }
        ]
      }),
      "字段 key 重复：appId"
    ],
    ["缺标签", draft({ fields: [{ key: "appId", label: " ", component: "input" }] }), "缺少标签"],
    ["select 无候选项", draft({ fields: [{ key: "mode", label: "模式", component: "select" }] }), "必须配置候选项"],
    [
      "password 未登记敏感字段",
      draft({ fields: [{ key: "appSecret", label: "密钥", component: "password", _isSecret: false }] }),
      "必须登记为敏感字段"
    ]
  ])("前置校验拦截：%s", (_name, input, expected) => {
    const result = draftToPayload(input);
    expect("error" in result).toBe(true);
    if ("error" in result) {
      expect(result.error).toContain(expected);
    }
  });

  test("draftFromTemplate 回填并深拷贝，编辑不污染原响应", () => {
    const tpl: ChannelTemplate = {
      templateId: 1,
      kind: "login",
      channelId: "huawei_cn",
      templateVersion: "v1",
      formSchemaJson: [
        { key: "mode", label: "模式", component: "select", options: [{ label: "静默", value: "silent" }] },
        { key: "appSecret", label: "密钥", component: "password" },
        { key: "keystore", label: "签名", component: "file" }
      ],
      secretFieldsJson: ["appSecret"],
      fileFieldsJson: [{ key: "keystore", accept: [".jks"] }],
      validationRulesJson: { mode: { required: true, maxLen: 10 } },
      enabled: true,
      effective: true,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-02T00:00:00Z"
    };
    const d = draftFromTemplate(tpl);
    expect(d.fields[0].options).toEqual([{ label: "静默", value: "silent" }]);
    expect(d.fields[0]._rules).toEqual({ required: true, maxLen: 10, minLen: undefined, min: undefined, max: undefined, pattern: undefined, format: undefined });
    expect(d.fields[1]._isSecret).toBe(true);
    expect(d.fields[2]._fileAcceptText).toBe(".jks");

    d.fields[0].options![0].label = "改了";
    d.fields[0]._rules.maxLen = 20;
    d.fields[1]._isSecret = false;
    d.fields[2]._fileAcceptText = ".p12";
    expect(tpl.formSchemaJson[0].options![0].label).toBe("静默");
    expect(tpl.secretFieldsJson).toEqual(["appSecret"]);
    expect(tpl.fileFieldsJson[0].accept).toEqual([".jks"]);
    expect(tpl.validationRulesJson["mode"].maxLen).toBe(10);
  });

  test("accept 数组与逗号文本互转", () => {
    expect(acceptToText([".jks", ".keystore"])).toBe(".jks, .keystore");
    expect(acceptToText(undefined)).toBe("");
    expect(textToAccept(" .jks , , .p12 ")).toEqual([".jks", ".p12"]);
  });
});
