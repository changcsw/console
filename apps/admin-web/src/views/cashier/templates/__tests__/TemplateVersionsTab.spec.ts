import { beforeEach, describe, expect, test } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import TemplateVersionsTab from "@/views/cashier/templates/TemplateVersionsTab.vue";
import type { CashierTemplateVersion, VersionStatus } from "@/api/modules/cashier";

// 说明：el-table 行内单元格（scoped slot）在 jsdom 下不渲染，行内
// 「复制为 draft / 发布 / 编辑矩阵」按钮与权限置灰由 Playwright e2e（真实浏览器）覆盖。
// 本组件级用例聚焦可在 jsdom 下稳定断言的纯逻辑与空态。

function version(overrides: Partial<CashierTemplateVersion> = {}): CashierTemplateVersion {
  return {
    version: "1",
    status: "draft",
    sourceType: "manual",
    publishedAt: null,
    ...overrides
  };
}

function mountTab(versions: CashierTemplateVersion[], perms: string[]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  return mount(TemplateVersionsTab, {
    props: { versions },
    global: { directives: { perm: permDirective } }
  });
}

interface TabVm {
  statusType: (s: VersionStatus) => string;
  formatTime: (v?: string | null) => string;
  onRowClick: (row: CashierTemplateVersion) => void;
}

describe("TemplateVersionsTab", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  test("状态标签类型映射：draft→primary / published→success / archived→info", () => {
    const wrapper = mountTab([version()], ["cashier.read"]);
    const vm = wrapper.vm as unknown as TabVm;
    expect(vm.statusType("draft")).toBe("primary");
    expect(vm.statusType("published")).toBe("success");
    expect(vm.statusType("archived")).toBe("info");
  });

  test("发布时间空值占位为 —，有值时本地化", () => {
    const wrapper = mountTab([version()], ["cashier.read"]);
    const vm = wrapper.vm as unknown as TabVm;
    expect(vm.formatTime(null)).toBe("—");
    expect(vm.formatTime("not-a-date")).toBe("not-a-date");
    expect(vm.formatTime("2026-01-01T00:00:00Z")).not.toBe("—");
  });

  test("行点击 emit select-version 带版本号", () => {
    const wrapper = mountTab([version({ version: "3" })], ["cashier.read"]);
    (wrapper.vm as unknown as TabVm).onRowClick(version({ version: "3" }));
    const emitted = wrapper.emitted("select-version");
    expect(emitted).toBeTruthy();
    expect(emitted![0][0]).toBe("3");
  });

  test("新建 draft 按钮无 cashier.write 时置灰（工具栏按钮，jsdom 可断言）", () => {
    const wrapper = mountTab([], ["cashier.read"]);
    const createBtn = wrapper.findAll("button").find((b) => b.text().includes("新建 draft 版本"));
    expect(createBtn).toBeTruthy();
    expect(createBtn!.attributes("disabled")).toBeDefined();
  });

  test("新建 draft 按钮点击 emit create-version", async () => {
    const wrapper = mountTab([], ["cashier.read", "cashier.write"]);
    const createBtn = wrapper.findAll("button").find((b) => b.text().includes("新建 draft 版本"));
    await createBtn!.trigger("click");
    expect(wrapper.emitted("create-version")).toBeTruthy();
  });

  test("空版本列表展示空态文案", () => {
    const wrapper = mountTab([], ["cashier.read"]);
    expect(wrapper.text()).toContain("暂无版本");
  });
});
