import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import CashierView from "@/views/cashier/CashierView.vue";
import type { CashierTemplateDetail, CashierTemplateSummary } from "@/api/modules/cashier";

const listCashierTemplates = vi.fn();
const getCashierTemplate = vi.fn();
const publishCashierVersion = vi.fn();
const createCashierTemplate = vi.fn();
const createCashierTemplateVersion = vi.fn();
const copyTemplateVersionToDraft = vi.fn();
const getCashierPriceRows = vi.fn();
const putCashierPriceRows = vi.fn();
const approveFxSyncRun = vi.fn();
const triggerFxSyncRun = vi.fn();

vi.mock("@/api/modules/cashier", () => ({
  listCashierTemplates: (...a: unknown[]) => listCashierTemplates(...a),
  getCashierTemplate: (...a: unknown[]) => getCashierTemplate(...a),
  publishCashierVersion: (...a: unknown[]) => publishCashierVersion(...a),
  createCashierTemplate: (...a: unknown[]) => createCashierTemplate(...a),
  createCashierTemplateVersion: (...a: unknown[]) => createCashierTemplateVersion(...a),
  copyTemplateVersionToDraft: (...a: unknown[]) => copyTemplateVersionToDraft(...a),
  getCashierPriceRows: (...a: unknown[]) => getCashierPriceRows(...a),
  putCashierPriceRows: (...a: unknown[]) => putCashierPriceRows(...a),
  approveFxSyncRun: (...a: unknown[]) => approveFxSyncRun(...a),
  triggerFxSyncRun: (...a: unknown[]) => triggerFxSyncRun(...a)
}));

function summary(overrides: Partial<CashierTemplateSummary> = {}): CashierTemplateSummary {
  return {
    templateId: "global_default",
    templateName: "Global Default",
    fxSyncEnabled: true,
    fxSyncMode: "manual_confirm",
    fxSyncSchedule: "monthly",
    status: "active",
    ...overrides
  };
}

function detail(overrides: Partial<CashierTemplateDetail> = {}): CashierTemplateDetail {
  return {
    templateId: "global_default",
    templateName: "Global Default",
    fxSyncEnabled: true,
    fxSyncMode: "manual_confirm",
    fxSyncSchedule: "monthly",
    status: "active",
    versions: [
      { version: "2", status: "draft", sourceType: "manual", publishedAt: null },
      { version: "1", status: "published", sourceType: "manual", publishedAt: "2026-01-01T00:00:00Z" }
    ],
    fxSyncRuns: [],
    ...overrides
  };
}

async function mountView(opts: { perms?: string[]; items?: CashierTemplateSummary[] } = {}) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: opts.perms ?? ["cashier.read", "cashier.write", "cashier.publish", "fx.approve"] });
  listCashierTemplates.mockResolvedValue({
    items: opts.items ?? [summary()],
    page: 1,
    pageSize: 20,
    total: (opts.items ?? [summary()]).length
  });
  const wrapper = mount(CashierView, { global: { directives: { perm: permDirective } } });
  await flushPromises();
  return wrapper;
}

describe("CashierView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getCashierTemplate.mockResolvedValue(detail());
    getCashierPriceRows.mockResolvedValue({ items: [] });
    publishCashierVersion.mockResolvedValue({ version: "2", status: "published" });
  });

  test("初始加载模板列表并自动选中第一项加载详情", async () => {
    const wrapper = await mountView();
    expect(listCashierTemplates).toHaveBeenCalled();
    expect(getCashierTemplate).toHaveBeenCalledWith("global_default");
    expect(wrapper.text()).toContain("Global Default");
  });

  test("查询关键字下发到列表接口", async () => {
    const wrapper = await mountView();
    const vm = wrapper.vm as unknown as { keyword: string; loadTemplates: (p?: number) => Promise<void> };
    vm.keyword = "global";
    await vm.loadTemplates(1);
    await flushPromises();
    expect(listCashierTemplates).toHaveBeenLastCalledWith(expect.objectContaining({ keyword: "global", page: 1 }));
  });

  test("默认优先选中 draft 版本作为编辑目标", async () => {
    const wrapper = await mountView();
    const vm = wrapper.vm as unknown as { selectedVersion: string; selectedVersionStatus: string };
    expect(vm.selectedVersion).toBe("2");
    expect(vm.selectedVersionStatus).toBe("draft");
  });

  test("发布版本调用接口并刷新详情", async () => {
    const wrapper = await mountView();
    const vm = wrapper.vm as unknown as { publishVersion: (v: { version: string }) => Promise<void> };
    getCashierTemplate.mockClear();
    await vm.publishVersion({ version: "2" });
    await flushPromises();
    expect(publishCashierVersion).toHaveBeenCalledWith("global_default", "2");
    expect(getCashierTemplate).toHaveBeenCalledWith("global_default");
  });

  test("空模板列表展示空态文案", async () => {
    const wrapper = await mountView({ items: [] });
    expect(wrapper.text()).toContain("暂无模板");
    expect(getCashierTemplate).not.toHaveBeenCalled();
  });

  test("无 cashier.write 权限时『新建模板』置灰", async () => {
    const wrapper = await mountView({ perms: ["cashier.read"] });
    const createBtn = wrapper.findAll("button").find((b) => b.text().includes("新建模板"));
    expect(createBtn!.attributes("disabled")).toBeDefined();
  });
});
