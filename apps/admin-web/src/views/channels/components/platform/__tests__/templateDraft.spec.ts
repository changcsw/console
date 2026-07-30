import { describe, expect, test } from "vitest";
import type { ChannelTemplate } from "@/api/modules/platformChannels";
import {
  acceptToText,
  draftFromTemplate,
  draftToPayload,
  emptyDraft,
  fieldKeys,
  parseRules,
  textToAccept,
  type TemplateDraft
} from "../templateDraft";

function draft(overrides: Partial<TemplateDraft> = {}): TemplateDraft {
  return {
    fields: [
      { key: "appId", label: "App ID", component: "input", required: true, order: 10, group: "", scope: "both" }
    ],
    secretFields: [],
    fileFields: [],
    rulesText: "{}",
    ...overrides
  };
}

describe("templateDraft", () => {
  test("emptyDraft 以一个空字段起步，规则文本为空对象", () => {
    const d = emptyDraft();
    expect(d.fields).toHaveLength(1);
    expect(d.fields[0].component).toBe("input");
    expect(d.rulesText).toBe("{}");
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
          { key: " appId ", label: " App ID ", component: "input", required: true, order: 10, placeholder: "  " },
          { key: "appSecret", label: "密钥", component: "password", order: 20 },
          { key: "keystore", label: "签名文件", component: "file", order: 30 }
        ],
        secretFields: ["appSecret"],
        fileFields: [{ key: "keystore", accept: [" .jks ", ""], maxSizeKB: 2048 }],
        rulesText: '{"appId":{"required":true,"maxLen":32}}'
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
    expect(payload.validationRulesJson).toEqual({ appId: { required: true, maxLen: 32 } });
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
      draft({ fields: [{ key: "appSecret", label: "密钥", component: "password" }] }),
      "必须登记为敏感字段"
    ],
    [
      "file 未登记文件字段",
      draft({ fields: [{ key: "keystore", label: "签名", component: "file" }] }),
      "必须登记到文件字段列表"
    ],
    ["规则不是 JSON", draft({ rulesText: "{oops" }), "不是合法 JSON"],
    ["规则不是对象", draft({ rulesText: "[1,2]" }), "必须是 JSON 对象"],
    ["规则字段未声明", draft({ rulesText: '{"ghost":{"required":true}}' }), "校验规则字段未在表单中声明：ghost"]
  ])("前置校验拦截：%s", (_name, input, expected) => {
    const result = draftToPayload(input);
    expect("error" in result).toBe(true);
    if ("error" in result) {
      expect(result.error).toContain(expected);
    }
  });

  test("敏感字段/文件字段里的悬空 key 被剔除而不是报错", () => {
    const result = draftToPayload(
      draft({
        secretFields: ["ghost"],
        fileFields: [{ key: "ghost" }]
      })
    );
    if (!("payload" in result)) {
      throw new Error(result.error);
    }
    expect(result.payload.secretFieldsJson).toEqual([]);
    expect(result.payload.fileFieldsJson).toEqual([]);
  });

  test("draftFromTemplate 回填并深拷贝，编辑不污染原响应", () => {
    const tpl: ChannelTemplate = {
      templateId: 1,
      kind: "login",
      channelId: "huawei_cn",
      templateVersion: "v1",
      formSchemaJson: [
        { key: "mode", label: "模式", component: "select", options: [{ label: "静默", value: "silent" }] }
      ],
      secretFieldsJson: ["appSecret"],
      fileFieldsJson: [{ key: "keystore", accept: [".jks"] }],
      validationRulesJson: { mode: { required: true } },
      enabled: true,
      effective: true,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-02T00:00:00Z"
    };
    const d = draftFromTemplate(tpl);
    expect(d.fields[0].options).toEqual([{ label: "静默", value: "silent" }]);
    expect(d.secretFields).toEqual(["appSecret"]);
    expect(JSON.parse(d.rulesText)).toEqual({ mode: { required: true } });

    d.fields[0].options![0].label = "改了";
    d.secretFields.push("x");
    d.fileFields[0].accept!.push(".p12");
    expect(tpl.formSchemaJson[0].options![0].label).toBe("静默");
    expect(tpl.secretFieldsJson).toEqual(["appSecret"]);
    expect(tpl.fileFieldsJson[0].accept).toEqual([".jks"]);
  });

  test("parseRules 把空文本视为空对象", () => {
    expect(parseRules("   ")).toEqual({ value: {} });
  });

  test("accept 数组与逗号文本互转", () => {
    expect(acceptToText([".jks", ".keystore"])).toBe(".jks, .keystore");
    expect(acceptToText(undefined)).toBe("");
    expect(textToAccept(" .jks , , .p12 ")).toEqual([".jks", ".p12"]);
  });
});
