import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

vi.mock("@/router", () => ({
  default: { currentRoute: { value: { path: "/dashboard", fullPath: "/dashboard" } }, push: vi.fn() }
}));

const pushMock = vi.fn();
vi.mock("vue-router", () => ({
  useRouter: () => ({ push: pushMock })
}));

const getSummaryApi = vi.fn();
vi.mock("@/api/modules/dashboard", async () => {
  const actual = await vi.importActual<typeof import("@/api/modules/dashboard")>("@/api/modules/dashboard");
  return {
    ...actual,
    getSummary: (...args: unknown[]) => getSummaryApi(...args)
  };
});

import { ApiError } from "@/api/http";
import DashboardIndex from "@/views/dashboard/index.vue";
import { useAppStore } from "@/stores/app";
import {
  DASHBOARD_EMPTY_SUMMARY,
  cloneSummary
} from "@/views/dashboard/__tests__/fixtures";

interface DashboardVM {
  range: "24h" | "7d" | "30d" | "90d";
  summary: unknown;
  loading: boolean;
  errorMessage: string;
  expandedKey: string | null;
  withTopItems: boolean;
  onRangeChange: (next: "24h" | "7d" | "30d" | "90d") => void;
  toggleDetails: (nextKey: string) => Promise<void>;
  reloadSummary: () => Promise<void>;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function mountView() {
  setActivePinia(createPinia());
  const wrapper = mount(DashboardIndex, {
    global: {
      stubs: {
        EnvironmentBadge: true
      }
    }
  });
  return { wrapper, vm: wrapper.vm as unknown as DashboardVM };
}

beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
  getSummaryApi.mockResolvedValue(cloneSummary());
});

describe("DashboardView", () => {
  test("默认按 7d 拉取并渲染 5 张卡片", async () => {
    const { wrapper, vm } = mountView();
    await flushPromises();

    expect(getSummaryApi).toHaveBeenCalledWith({ range: "7d", withTopItems: false, topN: 5 });
    expect(wrapper.findAll(".metric-card").length).toBe(5);
    expect(wrapper.text()).toContain("汇率待审");
    expect(wrapper.text()).toContain("配置异常");
    expect(wrapper.text()).toContain("最近同步");
    expect(wrapper.text()).toContain("待发布快照");
    expect(wrapper.text()).toContain("渠道实例问题");
    expect(useAppStore().environment).toBe("production");
  });

  test("range 切换会写入 localStorage，并以新 range 重新拉取", async () => {
    const { vm } = mountView();
    await flushPromises();
    getSummaryApi.mockClear();

    vm.onRangeChange("30d");
    await flushPromises();

    expect(window.localStorage.getItem("dashboard.summary.range")).toBe("30d");
    expect(getSummaryApi).toHaveBeenCalledWith({ range: "30d", withTopItems: false, topN: 5 });
  });

  test("localStorage 记忆值优先于默认 7d", async () => {
    window.localStorage.setItem("dashboard.summary.range", "24h");
    const { vm } = mountView();
    await flushPromises();
    expect(vm.range).toBe("24h");
    expect(getSummaryApi).toHaveBeenCalledWith({ range: "24h", withTopItems: false, topN: 5 });
  });

  test("range 切换时仅卡片 C 数据变化（其余卡片保持）", async () => {
    const first = cloneSummary();
    const second = cloneSummary();
    second.recentSyncJobs = {
      ...second.recentSyncJobs,
      total: 9,
      byStatus: { previewed: 3, succeeded: 4, failed: 2 }
    };
    getSummaryApi.mockResolvedValueOnce(first).mockResolvedValueOnce(second);

    const { wrapper, vm } = mountView();
    await flushPromises();
    const beforeText = wrapper.text();

    vm.onRangeChange("90d");
    await flushPromises();

    const afterText = wrapper.text();
    expect(beforeText).toContain("汇率待审");
    expect(afterText).toContain("汇率待审");
    expect(afterText).toContain("最近同步");
    expect(afterText).toContain("9");
    expect(afterText).toContain("成功 4");
    expect(afterText).toContain("失败 2");
    expect(afterText).toContain("预览 3");
    expect(afterText).toContain(first.configIssues.invalidTotal.toString());
  });

  test("四态：骨架/错误条+重试/空态/权限裁剪", async () => {
    const pending = deferred<ReturnType<typeof cloneSummary>>();
    getSummaryApi.mockReturnValueOnce(pending.promise);
    const { wrapper, vm } = mountView();
    expect(wrapper.find(".el-skeleton").exists()).toBe(true);
    pending.resolve(cloneSummary());
    await flushPromises();
    expect(wrapper.find(".el-skeleton").exists()).toBe(false);

    getSummaryApi.mockRejectedValueOnce(new ApiError(500, "INTERNAL", "加载失败"));
    await vm.reloadSummary();
    await flushPromises();
    expect(wrapper.text()).toContain("Dashboard 加载失败：加载失败");
    const retryBtn = wrapper.findAll("button").find((btn) => btn.text().includes("重试"));
    expect(retryBtn).toBeTruthy();

    getSummaryApi.mockResolvedValueOnce(cloneSummary(DASHBOARD_EMPTY_SUMMARY));
    await retryBtn!.trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("暂无待办");

    const trimmed = cloneSummary();
    trimmed.fxReview.permitted = false;
    getSummaryApi.mockResolvedValueOnce(trimmed);
    await vm.reloadSummary();
    await flushPromises();
    expect(wrapper.text()).not.toContain("汇率待审");
    expect(wrapper.findAll(".metric-card").length).toBe(4);
  });

  test("count>0 时点击展开明细会触发 withTopItems=true 重拉并渲染 topItems", async () => {
    const first = cloneSummary();
    first.fxReview.topItems = [];
    const second = cloneSummary();
    second.fxReview.topItems = [
      {
        runId: 1202,
        templateId: "asia_price_v1",
        templateName: "Asia Price v1",
        triggeredAt: "2026-06-18T02:00:00Z"
      }
    ];
    getSummaryApi.mockResolvedValueOnce(first).mockResolvedValueOnce(second);

    const { wrapper } = mountView();
    await flushPromises();
    getSummaryApi.mockClear();

    const expandBtn = wrapper.findAll("button").find((btn) => btn.text().includes("展开明细"));
    expect(expandBtn).toBeTruthy();
    await expandBtn!.trigger("click");
    await flushPromises();

    expect(getSummaryApi).toHaveBeenCalledWith({ range: "7d", withTopItems: true, topN: 5 });
    expect(wrapper.text()).toContain("Asia Price v1");
  });

  test("前往处理按钮跳转时透传 link.query", async () => {
    const { wrapper } = mountView();
    await flushPromises();

    const navBtn = wrapper
      .findAll("button")
      .find((btn) => btn.text().includes("前往处理"));
    expect(navBtn).toBeTruthy();
    await navBtn!.trigger("click");

    expect(pushMock).toHaveBeenCalledWith({
      path: "/cashier",
      query: {
        tab: "fx-review",
        status: "pending_review"
      }
    });
  });

});
