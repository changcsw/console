import { beforeEach, describe, expect, test, vi } from "vitest";
import { mount } from "@vue/test-utils";
import { ElMessage } from "element-plus";
import TemplateConfigRenderer from "@/views/games/detail/components/TemplateConfigRenderer.vue";
import type { IapTemplate } from "@/api/modules/products";

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
      { key: "certFile", label: "证书文件", component: "file", order: 7 },
    ],
    secretFields: ["privateKey"],
    fileFields: [{ key: "certFile", accept: [".json"], maxSizeKB: 64 }],
    validationRules: { appId: { minLen: 1 } },
  };
}

function mountRenderer(overrides: { modelValue?: Record<string, unknown>; secretValues?: Record<string, string> } = {}) {
  const wrapper = mount(TemplateConfigRenderer, {
    props: {
      template: makeTemplate(),
      modelValue: overrides.modelValue ?? { appId: "com.demo.app", privateKey: "masked" },
      secretValues: overrides.secretValues ?? {},
    },
  });
  const items = wrapper.findAll(".el-form-item");
  return { wrapper, items };
}

describe("TemplateConfigRenderer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
  });

  test("按 order 升序渲染四件套字段，组件类型映射正确", () => {
    const { wrapper, items } = mountRenderer();
    const labels = items.map((it) => it.find(".el-form-item__label").text());
    expect(labels).toEqual(["Private Key", "App ID", "沙箱", "重试", "区域", "扩展", "证书文件"]);
    expect(wrapper.find(".el-switch").exists()).toBe(true);
    expect(wrapper.find(".el-input-number").exists()).toBe(true);
    expect(wrapper.find(".el-select").exists()).toBe(true);
    expect(wrapper.find("textarea").exists()).toBe(true);
    expect(items[6].find("button").text()).toContain("上传文件");
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
        disabled: true,
      },
    });
    const inputs = wrapper.findAll("input");
    expect(inputs.length).toBeGreaterThan(0);
    expect(inputs.every((i) => i.attributes("disabled") !== undefined)).toBe(true);
  });

  test("文件上传成功：写入文件名到 modelValue，secret 留空不变", async () => {
    const { wrapper } = mountRenderer({ modelValue: { privateKey: "masked" } });
    const onSuccess = vi.fn();
    const onError = vi.fn();
    await (wrapper.vm as any).onFileUpload("certFile", {
      file: { name: "cert.json", size: 1024 },
      onSuccess,
      onError,
    });

    expect(onError).not.toHaveBeenCalled();
    const model = wrapper.emitted("update:modelValue")!.at(-1)![0] as Record<string, unknown>;
    expect(model.certFile).toBe("cert.json");
    expect(model.privateKey).toBe("masked");
    expect(ElMessage.success).toHaveBeenCalledWith("已选择文件：cert.json");
  });

  test("文件上传失败：超出大小限制时返回错误", async () => {
    const { wrapper } = mountRenderer();
    const onSuccess = vi.fn();
    const onError = vi.fn();
    await (wrapper.vm as any).onFileUpload("certFile", {
      file: { name: "cert.json", size: 70 * 1024 },
      onSuccess,
      onError,
    });

    expect(onSuccess).not.toHaveBeenCalled();
    expect(onError).toHaveBeenCalled();
    expect(ElMessage.error).toHaveBeenCalledWith("文件超过 64KB 限制");
  });
});
