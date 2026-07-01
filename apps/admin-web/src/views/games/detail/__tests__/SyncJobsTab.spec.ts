import { beforeEach, describe, expect, test, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import { ApiError } from "@/api/http";
import SyncJobsTab from "@/views/games/detail/SyncJobsTab.vue";
import type { SyncJobListItem, SyncJobsPage } from "@/api/syncSections";

// sync #21 · SyncJobsTab 组件测试（L4 vitest，mock API）。
// 覆盖 compact 前端要点：列项渲染、status 过滤、分页、失败行错误概要展开、成功行 appliedSummary。

const listApi = vi.fn();

vi.mock("@/api/syncSections", () => ({
  listSyncJobs: (...args: unknown[]) => listApi(...args)
}));

function makeJob(over: Partial<SyncJobListItem> = {}): SyncJobListItem {
  return {
    syncJobId: "9012",
    gameId: "100001",
    sourceEnv: "sandbox",
    targetEnv: "production",
    status: "succeeded",
    selectedSections: ["channels", "products"],
    includeDeletes: false,
    operatorId: 7,
    operatorName: "release-admin",
    operatorNote: "首次上线",
    sourceHash: "sha256-aaaaaaaabbbbbbbbcccccccc",
    targetHashBefore: "sha256-tttttttttttttttt",
    targetHashAfter: "sha256-ffffffffffffffff",
    executedAt: "2026-06-17T13:10:42Z",
    createdAt: "2026-06-17T13:00:00Z",
    appliedSummary: { channels: { add: 1, update: 1, delete: 0 } },
    ...over
  };
}

function page(items: SyncJobListItem[], total = items.length, pageNo = 1): SyncJobsPage {
  return { items, page: pageNo, pageSize: 20, total };
}

async function mountTab(options: { items?: SyncJobListItem[]; total?: number; reject?: unknown } = {}) {
  if (options.reject !== undefined) {
    listApi.mockRejectedValue(options.reject);
  } else {
    listApi.mockResolvedValue(page(options.items ?? [makeJob()], options.total));
  }
  const wrapper = mount(SyncJobsTab, { props: { gameId: "100001" } });
  await flushPromises();
  return wrapper;
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("SyncJobsTab", () => {
  test("挂载即按 gameId 拉取历史（page1/pageSize20/sort=-createdAt）", async () => {
    await mountTab();
    expect(listApi).toHaveBeenCalledTimes(1);
    expect(listApi).toHaveBeenCalledWith("100001", {
      page: 1,
      pageSize: 20,
      status: undefined,
      sort: "-createdAt"
    });
  });

  test("列项渲染：任务ID/状态/sections+include_deletes/操作者/备注/hash 截断/时间", async () => {
    const wrapper = await mountTab({
      items: [makeJob({ selectedSections: ["channels", "products"], includeDeletes: true })]
    });
    const text = wrapper.text();
    expect(text).toContain("9012");
    expect(text).toContain("succeeded");
    expect(text).toContain("channels, products");
    expect(text).toContain("include_deletes: true");
    expect(text).toContain("release-admin");
    expect(text).toContain("首次上线");
    // hash 截断展示（前 9 … 后 6），不出现完整原串
    expect(text).toContain("…");
  });

  test("status 过滤切换触发按新 status 的 reload(1)", async () => {
    const wrapper = await mountTab();
    listApi.mockClear();
    listApi.mockResolvedValue(page([makeJob({ status: "failed" })]));

    const vm = wrapper.vm as unknown as { status?: string; reload: (p?: number) => Promise<void> };
    vm.status = "failed";
    await vm.reload(1);
    await flushPromises();

    expect(listApi).toHaveBeenCalledWith("100001", {
      page: 1,
      pageSize: 20,
      status: "failed",
      sort: "-createdAt"
    });
  });

  test("total 超过 pageSize 时渲染分页器", async () => {
    const wrapper = await mountTab({ items: [makeJob()], total: 45 });
    expect(wrapper.findComponent({ name: "ElPagination" }).exists()).toBe(true);
  });

  test("翻页调用 reload 到目标页", async () => {
    const wrapper = await mountTab({ items: [makeJob()], total: 45 });
    listApi.mockClear();
    listApi.mockResolvedValue(page([makeJob()], 45, 2));
    const vm = wrapper.vm as unknown as { reload: (p?: number) => Promise<void> };
    await vm.reload(2);
    await flushPromises();
    expect(listApi).toHaveBeenCalledWith("100001", expect.objectContaining({ page: 2 }));
  });

  test("状态标签映射：succeeded=success / failed=warning / previewed=info", async () => {
    const wrapper = await mountTab({
      items: [
        makeJob({ syncJobId: "1", status: "succeeded" }),
        makeJob({ syncJobId: "2", status: "failed" }),
        makeJob({ syncJobId: "3", status: "previewed", executedAt: null })
      ]
    });
    const html = wrapper.html();
    expect(html).toContain("el-tag--success");
    expect(html).toContain("el-tag--warning");
    expect(html).toContain("el-tag--info");
  });

  test("失败行展开显示错误概要 code+message 与 details", async () => {
    const wrapper = await mountTab({
      items: [
        makeJob({
          status: "failed",
          executedAt: "2026-06-17T13:10:42Z",
          errorSummary: {
            code: "SYNC_BASELINE_MISMATCH",
            message: "目标已变更",
            details: [{ field: "targetHashBefore", expected: "a", actual: "b" }]
          }
        })
      ]
    });
    await wrapper.find(".el-table__expand-icon").trigger("click");
    await flushPromises();
    const text = wrapper.text();
    expect(text).toContain("SYNC_BASELINE_MISMATCH");
    expect(text).toContain("目标已变更");
    expect(text).toContain("targetHashBefore");
  });

  test("成功行展开显示 appliedSummary", async () => {
    const wrapper = await mountTab({
      items: [makeJob({ status: "succeeded", appliedSummary: { products: { add: 0, update: 2, delete: 0 } } })]
    });
    await wrapper.find(".el-table__expand-icon").trigger("click");
    await flushPromises();
    const text = wrapper.text();
    expect(text).toContain("appliedSummary");
    expect(text).toContain("products");
  });

  test("加载失败展示错误态", async () => {
    const wrapper = await mountTab({ reject: new ApiError(500, "INTERNAL", "boom") });
    expect(wrapper.text()).toContain("boom");
  });

  test("空列表不报错且分页 total=0", async () => {
    const wrapper = await mountTab({ items: [], total: 0 });
    const vm = wrapper.vm as unknown as { total: number };
    expect(vm.total).toBe(0);
  });
});
