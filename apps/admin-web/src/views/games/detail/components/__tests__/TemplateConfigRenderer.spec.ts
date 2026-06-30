import { beforeEach, describe, expect, test } from "vitest";
import { mount } from "@vue/test-utils";
import TemplateConfigRenderer from "@/views/games/detail/components/TemplateConfigRenderer.vue";
import type { IapTemplate } from "@/api/modules/products";

// 模块 16-product · 模板渲染器（四件套消费 + 密文 masked 可重填）
// 对齐 spec.compact §前端要点：消费模板四件套（formSchema/secretFields/fileFields/validationRules）、
// 密文脱敏可重填（留空不修改）、JSON 字段非法阻断、文件字段统一引用输入。

function makeTemplate(): IapTemplate {
  return {
    templateVersion: "v3",
    formSchema: [
      { key: "appId", label: "App ID", component: "input", order: 2 },
      { key: "privateKey", label: "Private Key", component: "password", order: 1 },
      { key: "sandboxFlag", label: "沙箱", component: "switch", order: 3 },
      { key: "retry", label: "重试", component: "number", order: 4 },
      { key: "region", label: "区域", component: "select", order: 5, options: [{ label: "美国", value: "us" }] },
      { key: "extra", label: "扩展", component: "json", order: 6 },
      { key: "certFile", label: "证书文件", component: "file", order: 7 }
    ],
    secretFields: ["privateKey"],
    fileFields: [{ key: "certFile", accept: ["application/json"], maxSizeKB: 64 }],
    validationRules: { appId: { minLen: 1 } }
  };
}

function mountRenderer(overrides: { modelValue?: Record<string, unknown>; secretValues?: Record<string, string> } = {}) {
  const wrapper = mount(TemplateConfigRenderer, {
    props: {
      template: makeTemplate(),
      modelValue: overrides.modelValue ?? { appId: "com.demo.app", privateKey: "masked" },
      secretValues: overrides.secretValues ?? {}
    }
  });
  const items = wrapper.findAll(".el-form-item");
  return { wrapper, items };
}

describe("TemplateConfigRenderer", () => {
  test("按 order 升序渲染四件套字段，组件类型映射正确", () => {
    const { wrapper, items } = mountRenderer();
    // order 升序：Private Key(1) → App ID(2) → 沙箱 → 重试 → 区域 → 扩展 → 证书文件
    const labels = items.map((it) => it.find(".el-form-item__label").text());
    expect(labels).toEqual(["Private Key", "App ID", "沙箱", "重试", "区域", "扩展", "证书文件"]);
    // 各组件类型存在
    expect(wrapper.find(".el-switch").exists()).toBe(true);
    expect(wrapper.find(".el-input-number").exists()).toBe(true);
    expect(wrapper.find(".el-select").exists()).toBe(true);
    expect(wrapper.find("textarea").exists()).toBe(true);
    // 文件字段走统一引用输入（带回填占位）
    expect(items[6].find("input").attributes("placeholder")).toContain("文件引用");
  });

  test("密文字段：已存储值展示 masked 占位且 placeholder 提示留空不修改", () => {
    const { items } = mountRenderer();
    const masked = items[0].find(".secret-input__masked");
    expect(masked.exists()).toBe(true);
    expect(masked.text()).toBe("masked");
    expect(items[0].find("input").attributes("placeholder")).toBe("留空则不修改");
  });

  test("密文字段：无存储值时 placeholder 提示请输入新值", () => {
    const { items } = mountRenderer({ modelValue: { appId: "com.demo.app" } });
    expect(items[0].find(".secret-input__masked").exists()).toBe(false);
    expect(items[0].find("input").attributes("placeholder")).toBe("请输入新值");
  });

  test("密文可重填：输入新值仅 emit update:secretValues，不污染 modelValue", async () => {
    const { wrapper, items } = mountRenderer();
    await items[0].find("input").setValue("brand-new-secret");
    const secretEvents = wrapper.emitted("update:secretValues");
    expect(secretEvents).toBeTruthy();
    expect(secretEvents!.at(-1)![0]).toEqual({ privateKey: "brand-new-secret" });
    // 密文重填不应通过 update:modelValue 落明文
    expect(wrapper.emitted("update:modelValue")).toBeFalsy();
  });

  test("普通字段：编辑 emit update:modelValue；清空则删除该键", async () => {
    const { wrapper, items } = mountRenderer();
    const appInput = items[1].find("input");
    await appInput.setValue("com.demo.next");
    let model = wrapper.emitted("update:modelValue")!.at(-1)![0] as Record<string, unknown>;
    expect(model.appId).toBe("com.demo.next");

    await appInput.setValue("");
    model = wrapper.emitted("update:modelValue")!.at(-1)![0] as Record<string, unknown>;
    expect("appId" in model).toBe(false);
  });

  test("JSON 字段：非法输入 blur → json-error-change(true) 且行内报错可见", async () => {
    const { wrapper, items } = mountRenderer();
    const textarea = items[5].find("textarea");
    await textarea.setValue("{ not valid json ");
    await textarea.trigger("blur");
    const errEvents = wrapper.emitted("json-error-change");
    expect(errEvents!.at(-1)![0]).toBe(true);
    expect(items[5].find(".field-error").exists()).toBe(true);
  });

  test("JSON 字段：合法输入 blur → 解析对象写回 modelValue 且无错误", async () => {
    const { wrapper, items } = mountRenderer();
    const textarea = items[5].find("textarea");
    await textarea.setValue('{"k":1}');
    await textarea.trigger("blur");
    const model = wrapper.emitted("update:modelValue")!.at(-1)![0] as Record<string, unknown>;
    expect(model.extra).toEqual({ k: 1 });
    expect(wrapper.emitted("json-error-change")!.at(-1)![0]).toBe(false);
  });

  test("disabled 透传：只读时所有输入禁用", () => {
    const wrapper = mount(TemplateConfigRenderer, {
      props: {
        template: makeTemplate(),
        modelValue: { appId: "com.demo.app", privateKey: "masked" },
        secretValues: {},
        disabled: true
      }
    });
    const inputs = wrapper.findAll("input");
    expect(inputs.length).toBeGreaterThan(0);
    expect(inputs.every((i) => i.attributes("disabled") !== undefined)).toBe(true);
  });
});
