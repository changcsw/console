import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import { ApiError } from "@/api/http";
import CreateTemplateDialog from "@/views/cashier/templates/components/CreateTemplateDialog.vue";
import CreateVersionDialog from "@/views/cashier/templates/components/CreateVersionDialog.vue";
import CopyPublishedToDraftDialog from "@/views/cashier/templates/components/CopyPublishedToDraftDialog.vue";

const createCashierTemplate = vi.fn();
const createCashierTemplateVersion = vi.fn();
const copyTemplateVersionToDraft = vi.fn();

vi.mock("@/api/modules/cashier", () => ({
  createCashierTemplate: (...a: unknown[]) => createCashierTemplate(...a),
  createCashierTemplateVersion: (...a: unknown[]) => createCashierTemplateVersion(...a),
  copyTemplateVersionToDraft: (...a: unknown[]) => copyTemplateVersionToDraft(...a)
}));

function withPinia(perms: string[] = ["cashier.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
}

describe("CreateTemplateDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    withPinia();
  });

  test("缺少必填字段时不调用创建接口", async () => {
    const wrapper = mount(CreateTemplateDialog, {
      props: { open: true },
      global: { directives: { perm: permDirective } }
    });
    await (wrapper.vm as unknown as { submit: () => Promise<void> }).submit();
    expect(createCashierTemplate).not.toHaveBeenCalled();
  });

  test("合法填写后创建并 emit created/close", async () => {
    createCashierTemplate.mockResolvedValue({ templateId: "t1" });
    const wrapper = mount(CreateTemplateDialog, {
      props: { open: true },
      global: { directives: { perm: permDirective } }
    });
    const vm = wrapper.vm as unknown as { form: Record<string, unknown>; submit: () => Promise<void> };
    vm.form.templateId = "t1";
    vm.form.templateName = "T1";
    await vm.submit();
    await flushPromises();
    expect(createCashierTemplate).toHaveBeenCalledWith(expect.objectContaining({ templateId: "t1", templateName: "T1" }));
    expect(wrapper.emitted("created")).toBeTruthy();
    expect(wrapper.emitted("close")).toBeTruthy();
  });

  test("CONFLICT 冲突时不 emit created", async () => {
    createCashierTemplate.mockRejectedValue(new ApiError(409, "CONFLICT", "已存在"));
    const wrapper = mount(CreateTemplateDialog, {
      props: { open: true },
      global: { directives: { perm: permDirective } }
    });
    const vm = wrapper.vm as unknown as { form: Record<string, unknown>; submit: () => Promise<void> };
    vm.form.templateId = "dup";
    vm.form.templateName = "Dup";
    await vm.submit();
    await flushPromises();
    expect(wrapper.emitted("created")).toBeFalsy();
  });
});

describe("CreateVersionDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    withPinia();
  });

  test("copy 来源类型缺 sourceVersion 时阻断", async () => {
    const wrapper = mount(CreateVersionDialog, {
      props: { open: true, templateId: "t1" },
      global: { directives: { perm: permDirective } }
    });
    const vm = wrapper.vm as unknown as { form: Record<string, unknown>; submit: () => Promise<void> };
    vm.form.sourceType = "copy_published";
    vm.form.sourceVersion = "";
    await vm.submit();
    expect(createCashierTemplateVersion).not.toHaveBeenCalled();
  });

  test("manual 创建下发 sourceVersion=undefined", async () => {
    createCashierTemplateVersion.mockResolvedValue({ version: "3", status: "draft" });
    const wrapper = mount(CreateVersionDialog, {
      props: { open: true, templateId: "t1" },
      global: { directives: { perm: permDirective } }
    });
    const vm = wrapper.vm as unknown as { form: Record<string, unknown>; submit: () => Promise<void> };
    vm.form.sourceType = "manual";
    await vm.submit();
    await flushPromises();
    expect(createCashierTemplateVersion).toHaveBeenCalledWith("t1", { sourceType: "manual", sourceVersion: undefined });
    expect(wrapper.emitted("created")).toBeTruthy();
  });

  test("copy_published 创建下发来源版本", async () => {
    createCashierTemplateVersion.mockResolvedValue({ version: "9", status: "draft" });
    const wrapper = mount(CreateVersionDialog, {
      props: { open: true, templateId: "t1" },
      global: { directives: { perm: permDirective } }
    });
    const vm = wrapper.vm as unknown as { form: Record<string, unknown>; submit: () => Promise<void> };
    vm.form.sourceType = "copy_published";
    vm.form.sourceVersion = "8";
    await vm.submit();
    await flushPromises();
    expect(createCashierTemplateVersion).toHaveBeenCalledWith("t1", { sourceType: "copy_published", sourceVersion: "8" });
  });
});

describe("CopyPublishedToDraftDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    withPinia();
  });

  test("确认复制调用接口并 emit created/close", async () => {
    copyTemplateVersionToDraft.mockResolvedValue({ version: "10", status: "draft", sourceType: "copy_published" });
    const wrapper = mount(CopyPublishedToDraftDialog, {
      props: { open: true, templateId: "t1", sourceVersion: { version: "8", status: "published" } },
      global: { directives: { perm: permDirective } }
    });
    await (wrapper.vm as unknown as { submit: () => Promise<void> }).submit();
    await flushPromises();
    expect(copyTemplateVersionToDraft).toHaveBeenCalledWith("t1", "8");
    expect(wrapper.emitted("created")).toBeTruthy();
    expect(wrapper.emitted("close")).toBeTruthy();
  });
});
