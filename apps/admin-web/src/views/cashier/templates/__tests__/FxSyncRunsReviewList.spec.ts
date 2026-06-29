import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import FxSyncRunsReviewList from "@/views/cashier/templates/FxSyncRunsReviewList.vue";
import type { FxRunStatus, FxSyncRun } from "@/api/modules/cashier";

// 说明：el-table 行内 approve/ignore 按钮、差异摘要单元格在 jsdom 下不渲染，
// 其禁用态/可视化由 Playwright e2e 覆盖；此处直接调用组件 review()/triggerRun()
// 逻辑与纯函数，验证契约下发与 emit。

const approveFxSyncRun = vi.fn();
const triggerFxSyncRun = vi.fn();

vi.mock("@/api/modules/cashier", () => ({
  approveFxSyncRun: (...args: unknown[]) => approveFxSyncRun(...args),
  triggerFxSyncRun: (...args: unknown[]) => triggerFxSyncRun(...args)
}));

function run(overrides: Partial<FxSyncRun> = {}): FxSyncRun {
  return {
    runId: 1,
    status: "pending_review",
    candidateVersion: "9",
    triggeredAt: "2026-01-01T00:00:00Z",
    reviewedBy: null,
    reviewedAt: null,
    reviewNote: "",
    diffSummary: { added: 2, updated: 1 },
    ...overrides
  };
}

function mountList(runs: FxSyncRun[], perms: string[]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  return mount(FxSyncRunsReviewList, {
    props: { templateId: "global_default", runs, loading: false },
    global: { directives: { perm: permDirective } }
  });
}

interface ListVm {
  statusType: (s: FxRunStatus) => string;
  canReview: (s: FxRunStatus) => boolean;
  formatDiff: (v: Record<string, unknown>) => string;
  formatTime: (v?: string | null) => string;
  review: (runId: number, action: "approve" | "ignore") => Promise<void>;
}

describe("FxSyncRunsReviewList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    approveFxSyncRun.mockResolvedValue(run({ status: "applied" }));
    triggerFxSyncRun.mockResolvedValue(run());
  });

  test("差异摘要 JSON 化", () => {
    const wrapper = mountList([run()], ["cashier.read"]);
    const vm = wrapper.vm as unknown as ListVm;
    expect(vm.formatDiff({ added: 2, updated: 1 })).toContain("\"added\": 2");
    expect(vm.formatDiff(undefined as unknown as Record<string, unknown>)).toBe("{}");
  });

  test("状态标签类型映射", () => {
    const vm = mountList([run()], ["cashier.read"]).vm as unknown as ListVm;
    expect(vm.statusType("applied")).toBe("success");
    expect(vm.statusType("approved")).toBe("warning");
    expect(vm.statusType("ignored")).toBe("danger");
    expect(vm.statusType("failed")).toBe("danger");
    expect(vm.statusType("pending_review")).toBe("info");
  });

  test("仅 pending_review 可审核", () => {
    const vm = mountList([run()], ["cashier.read"]).vm as unknown as ListVm;
    expect(vm.canReview("pending_review")).toBe(true);
    expect(vm.canReview("applied")).toBe(false);
    expect(vm.canReview("ignored")).toBe(false);
  });

  test("approve 调用接口传 action=approve 并 emit refresh", async () => {
    const wrapper = mountList([run({ runId: 42 })], ["fx.approve"]);
    await (wrapper.vm as unknown as ListVm).review(42, "approve");
    await flushPromises();
    expect(approveFxSyncRun).toHaveBeenCalledWith(42, expect.objectContaining({ action: "approve" }));
    expect(wrapper.emitted("refresh")).toBeTruthy();
  });

  test("ignore 调用接口传 action=ignore 并带 reviewNote", async () => {
    const wrapper = mountList([run({ runId: 7 })], ["fx.approve"]);
    await (wrapper.vm as unknown as ListVm).review(7, "ignore");
    await flushPromises();
    expect(approveFxSyncRun).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ action: "ignore", reviewNote: expect.any(String) })
    );
  });

  test("触发 FX 同步按钮（工具栏）无 cashier.write 时置灰", () => {
    const wrapper = mountList([], ["cashier.read"]);
    const btn = wrapper.findAll("button").find((b) => b.text().includes("触发 FX 同步"));
    expect(btn!.attributes("disabled")).toBeDefined();
  });

  test("触发 FX 同步调用接口并 emit refresh", async () => {
    const wrapper = mountList([], ["cashier.write"]);
    const btn = wrapper.findAll("button").find((b) => b.text().includes("触发 FX 同步"));
    await btn!.trigger("click");
    await flushPromises();
    expect(triggerFxSyncRun).toHaveBeenCalledWith("global_default");
    expect(wrapper.emitted("refresh")).toBeTruthy();
  });

  test("无 run 时展示空态文案", () => {
    const wrapper = mountList([], ["cashier.read"]);
    expect(wrapper.text()).toContain("暂无 FX run");
  });
});
