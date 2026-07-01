import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { ElMessage, ElMessageBox } from "element-plus";

import { ApiError } from "@/api/http";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import SnapshotTab from "@/views/games/detail/SnapshotTab.vue";
import type {
  DownloadSnapshotResponse,
  GenerateSnapshotResponse,
  SnapshotListItem,
  SnapshotListResponse
} from "@/api/modules/snapshot";

const listApi = vi.fn();
const generateApi = vi.fn();
const publishApi = vi.fn();
const downloadApi = vi.fn();

vi.mock("@/api/modules/snapshot", () => ({
  listGameSnapshots: (...args: unknown[]) => listApi(...args),
  generateGameSnapshot: (...args: unknown[]) => generateApi(...args),
  publishGameSnapshot: (...args: unknown[]) => publishApi(...args),
  downloadGameSnapshot: (...args: unknown[]) => downloadApi(...args)
}));

function makeItem(over: Partial<SnapshotListItem> = {}): SnapshotListItem {
  return {
    id: 1,
    configVersion: "20260615100000-a1b2c3d4",
    status: "draft",
    fileHash: "a1b2c3d4e5f6",
    generatedAt: "2026-06-15T10:00:00Z",
    publishedAt: null,
    ...over
  };
}

// 后端契约保证 generated_at 降序返回；前端不再重排，直接渲染返回顺序。
function descendingItems(): SnapshotListItem[] {
  return [
    makeItem({ id: 3, configVersion: "20260615120000-cccccccc", fileHash: "cccccccc", generatedAt: "2026-06-15T12:00:00Z", status: "published", publishedAt: "2026-06-15T12:30:00Z" }),
    makeItem({ id: 2, configVersion: "20260615110000-bbbbbbbb", fileHash: "bbbbbbbb", generatedAt: "2026-06-15T11:00:00Z", status: "draft", publishedAt: null }),
    makeItem({ id: 1, configVersion: "20260615100000-aaaaaaaa", fileHash: "aaaaaaaa", generatedAt: "2026-06-15T10:00:00Z", status: "draft", publishedAt: null })
  ];
}

function listResponse(items: SnapshotListItem[], total = items.length): SnapshotListResponse {
  return { items, page: 1, pageSize: 20, total };
}

function previewPayload(): DownloadSnapshotResponse {
  return {
    fileName: "game_100001_20260615100000-a1b2c3d4.json",
    blob: new Blob(["{}"], { type: "application/json" }),
    payload: {
      schemaVersion: "1.0",
      gameId: "100001",
      generatedAt: "2026-06-15T10:00:00Z",
      markets: {
        GLOBAL: {
          game: { legalLinks: [] },
          channels: [
            {
              channelId: "google",
              region: "overseas",
              sourceMarket: "GLOBAL",
              login: { appId: "app-global", appSecret: "PLAINTEXT_GLOBAL_SECRET" }
            }
          ],
          paymentRoutes: []
        },
        JP: {
          game: {},
          channels: [
            {
              channelId: "google",
              region: "overseas",
              sourceMarket: "JP",
              iap: { issuerId: "iss-jp", privateKey: "PLAINTEXT_JP_KEY" }
            }
          ],
          paymentRoutes: []
        }
      }
    }
  };
}

interface SnapshotVM {
  rows: SnapshotListItem[];
  page: number;
  total: number;
  pageState: "ready" | "forbidden" | "error";
  errorMessage: string;
  previewData: unknown;
  previewError: string;
  activeMarkets: string[];
  marketEntries: Array<{ market: string; content: unknown }>;
  reload: (targetPage?: number) => Promise<void>;
  generateSnapshot: () => Promise<void>;
  publishSnapshot: (id: number) => Promise<void>;
  downloadSnapshot: (id: number) => Promise<void>;
  previewSnapshot: (row: SnapshotListItem) => Promise<void>;
}

async function mountTab(
  options: { perms?: string[]; items?: SnapshotListItem[]; total?: number; listReject?: unknown } = {}
) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({
    roles: [],
    permissions: options.perms ?? ["game.read", "snapshot.generate", "snapshot.publish"]
  });

  if (options.listReject !== undefined) {
    listApi.mockRejectedValue(options.listReject);
  } else {
    listApi.mockResolvedValue(listResponse(options.items ?? [makeItem()], options.total));
  }

  const wrapper = mount(SnapshotTab, {
    props: { gameId: "100001" },
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return { wrapper, vm: wrapper.vm as unknown as SnapshotVM };
}

function findButtonByText(wrapper: ReturnType<typeof mount>, text: string) {
  return wrapper.findAll("button").find((btn) => btn.text().includes(text));
}

describe("SnapshotTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "warning").mockImplementation(() => ({}) as never);
  });

  test("挂载即按 gameId 拉取快照列表", async () => {
    await mountTab();
    expect(listApi).toHaveBeenCalledTimes(1);
    expect(listApi).toHaveBeenCalledWith("100001", { page: 1, pageSize: 20 });
  });

  test("列表渲染 version/status/hash/时间，并保持后端 generated_at 降序顺序", async () => {
    const { wrapper, vm } = await mountTab({ items: descendingItems() });

    // 组件不重排，直接消费后端降序返回
    expect(vm.rows.map((r) => r.configVersion)).toEqual([
      "20260615120000-cccccccc",
      "20260615110000-bbbbbbbb",
      "20260615100000-aaaaaaaa"
    ]);

    const text = wrapper.text();
    expect(text).toContain("20260615120000-cccccccc");
    expect(text).toContain("cccccccc");
    // status 标签：published / draft 均渲染
    expect(text).toContain("published");
    expect(text).toContain("draft");
    // 渲染顺序与后端一致（最新在最前）
    const html = wrapper.html();
    expect(html.indexOf("cccccccc")).toBeLessThan(html.indexOf("bbbbbbbb"));
    expect(html.indexOf("bbbbbbbb")).toBeLessThan(html.indexOf("aaaaaaaa"));
    // draft 行发布时间显示占位符 —
    expect(text).toContain("—");
  });

  test("total 超过 pageSize 时出现分页器，翻页调用 reload", async () => {
    const { wrapper } = await mountTab({ items: descendingItems(), total: 45 });
    expect(wrapper.find(".snapshot-tab__pager").exists()).toBe(true);
    expect(wrapper.findComponent({ name: "ElPagination" }).exists()).toBe(true);
  });

  test("生成按钮触发 generate 并刷新列表", async () => {
    const created: GenerateSnapshotResponse = {
      id: 9,
      configVersion: "20260615130000-deadbeef",
      fileHash: "deadbeef",
      status: "draft",
      generatedAt: "2026-06-15T13:00:00Z"
    };
    generateApi.mockResolvedValue(created);
    const { wrapper } = await mountTab();

    const genBtn = findButtonByText(wrapper, "生成快照");
    expect(genBtn).toBeTruthy();
    await genBtn!.trigger("click");
    await flushPromises();

    expect(generateApi).toHaveBeenCalledWith("100001");
    expect(ElMessage.success).toHaveBeenCalledWith(expect.stringContaining("20260615130000-deadbeef"));
    // mount 1 次 + 生成后 reload(1) 1 次
    expect(listApi).toHaveBeenCalledTimes(2);
  });

  test("JSON 预览：按 market 分区折叠展示，密文脱敏为 ***", async () => {
    downloadApi.mockResolvedValue(previewPayload());
    const { wrapper, vm } = await mountTab({ items: [makeItem({ id: 1 })] });

    const previewBtn = findButtonByText(wrapper, "预览 JSON");
    expect(previewBtn).toBeTruthy();
    await previewBtn!.trigger("click");
    await flushPromises();

    expect(downloadApi).toHaveBeenCalledWith(1);
    // 每个 market 一个折叠分区
    expect(vm.marketEntries.map((e) => e.market).sort()).toEqual(["GLOBAL", "JP"]);
    expect(vm.activeMarkets.sort()).toEqual(["GLOBAL", "JP"]);

    await flushPromises();
    const html = wrapper.html();
    expect(html).toContain("GLOBAL");
    expect(html).toContain("JP");
    // 密文脱敏
    expect(html).toContain("***");
    expect(html).not.toContain("PLAINTEXT_GLOBAL_SECRET");
    expect(html).not.toContain("PLAINTEXT_JP_KEY");
    // 非密文字段保留
    expect(html).toContain("app-global");
  });

  test("预览下载内容非合法 JSON 时给出错误提示", async () => {
    downloadApi.mockResolvedValue({
      fileName: "x.json",
      blob: new Blob(["not-json"]),
      payload: null
    } as DownloadSnapshotResponse);
    const { vm } = await mountTab({ items: [makeItem({ id: 1 })] });

    await vm.previewSnapshot(makeItem({ id: 1 }));
    await flushPromises();
    expect(vm.previewError).toContain("下载内容不是合法 JSON");
  });

  test("下载入口触发下载并成功提示", async () => {
    const createObjectURL = vi.fn(() => "blob:mock");
    const revokeObjectURL = vi.fn();
    // jsdom 未实现 createObjectURL / anchor 导航
    (URL as unknown as { createObjectURL: unknown }).createObjectURL = createObjectURL;
    (URL as unknown as { revokeObjectURL: unknown }).revokeObjectURL = revokeObjectURL;
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});

    downloadApi.mockResolvedValue(previewPayload());
    const { wrapper } = await mountTab({ items: [makeItem({ id: 7 })] });

    const dlBtn = findButtonByText(wrapper, "下载");
    expect(dlBtn).toBeTruthy();
    await dlBtn!.trigger("click");
    await flushPromises();

    expect(downloadApi).toHaveBeenCalledWith(7);
    expect(createObjectURL).toHaveBeenCalled();
    expect(clickSpy).toHaveBeenCalled();
    expect(ElMessage.success).toHaveBeenCalledWith("快照下载已开始");
    clickSpy.mockRestore();
  });

  test("发布二次确认：确认后触发 publish 并刷新", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    publishApi.mockResolvedValue(makeItem({ id: 1, status: "published", publishedAt: "2026-06-15T14:00:00Z" }));
    const { wrapper } = await mountTab({ items: [makeItem({ id: 1, status: "draft" })] });

    const pubBtn = findButtonByText(wrapper, "发布");
    expect(pubBtn).toBeTruthy();
    await pubBtn!.trigger("click");
    await flushPromises();

    expect(ElMessageBox.confirm).toHaveBeenCalled();
    expect(publishApi).toHaveBeenCalledWith(1);
    expect(ElMessage.success).toHaveBeenCalledWith("快照发布成功");
    expect(listApi).toHaveBeenCalledTimes(2);
  });

  test("发布二次确认：取消则不触发 publish", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockRejectedValue("cancel");
    const { wrapper } = await mountTab({ items: [makeItem({ id: 1, status: "draft" })] });

    const pubBtn = findButtonByText(wrapper, "发布");
    await pubBtn!.trigger("click");
    await flushPromises();

    expect(publishApi).not.toHaveBeenCalled();
  });

  test("非 draft 快照的发布按钮禁用", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    const { wrapper } = await mountTab({ items: [makeItem({ id: 1, status: "published", publishedAt: "2026-06-15T14:00:00Z" })] });
    const pubBtn = findButtonByText(wrapper, "发布");
    // 有 publish 权限但已发布：由 el-button :disabled 生效（is-disabled class），并拦截点击
    expect(pubBtn!.classes()).toContain("is-disabled");
    await pubBtn!.trigger("click");
    await flushPromises();
    expect(publishApi).not.toHaveBeenCalled();
  });

  test("无 snapshot.generate/publish 权限时按钮置灰并显示只读提示", async () => {
    const { wrapper } = await mountTab({ perms: ["game.read"], items: [makeItem({ id: 1, status: "draft" })] });

    const genBtn = findButtonByText(wrapper, "生成快照");
    expect(genBtn!.attributes("disabled")).toBe("disabled");
    expect(genBtn!.classes()).toContain("perm-disabled");

    const pubBtn = findButtonByText(wrapper, "发布");
    expect(pubBtn!.attributes("disabled")).toBe("disabled");
    expect(pubBtn!.classes()).toContain("perm-disabled");

    expect(wrapper.text()).toContain("仅有查看权限");
  });

  test("仅缺 publish 权限时提示 snapshot.publish 置灰", async () => {
    const { wrapper } = await mountTab({ perms: ["game.read", "snapshot.generate"], items: [makeItem()] });
    expect(wrapper.text()).toContain("缺少 snapshot.publish 权限");
  });

  test("空态：无快照时展示空提示与生成首个快照入口", async () => {
    const { wrapper } = await mountTab({ items: [], total: 0 });
    expect(wrapper.text()).toContain("暂无配置快照");
    expect(findButtonByText(wrapper, "生成首个快照")).toBeTruthy();
  });

  test("加载 403 -> 无权限态", async () => {
    const { wrapper, vm } = await mountTab({ listReject: new ApiError(403, "FORBIDDEN", "forbidden") });
    expect(vm.pageState).toBe("forbidden");
    expect(wrapper.text()).toContain("无权限查看配置快照");
  });

  test("加载失败（非 403）-> 错误态并可重试", async () => {
    const { wrapper, vm } = await mountTab({ listReject: new ApiError(500, "INTERNAL", "boom") });
    expect(vm.pageState).toBe("error");
    expect(wrapper.text()).toContain("配置快照加载失败");
    expect(findButtonByText(wrapper, "重试")).toBeTruthy();
  });

  test("无 game.read 权限：直接置为无权限态且不请求列表", async () => {
    const { vm } = await mountTab({ perms: ["snapshot.generate"] });
    expect(vm.pageState).toBe("forbidden");
    expect(listApi).not.toHaveBeenCalled();
  });

  test.each([
    ["NOT_FOUND", "资源不存在，请刷新后重试。"],
    ["VALIDATION_FAILED", "请求参数校验失败，请检查后重试。"],
    ["VERSION_STATE_INVALID", "当前快照状态不允许该操作。"],
    ["CONFLICT", "资源状态冲突，请刷新后重试。"]
  ])("生成失败错误码 %s 映射为可读提示", async (code, message) => {
    generateApi.mockRejectedValue(new ApiError(code === "NOT_FOUND" ? 404 : code === "CONFLICT" ? 409 : 400, code, "raw"));
    const { vm } = await mountTab();
    await vm.generateSnapshot();
    await flushPromises();
    expect(ElMessage.error).toHaveBeenCalledWith(message);
  });

  test("发布失败错误码 VERSION_STATE_INVALID 映射为可读提示", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    publishApi.mockRejectedValue(new ApiError(409, "VERSION_STATE_INVALID", "raw"));
    const { vm } = await mountTab({ items: [makeItem({ id: 1, status: "draft" })] });
    await vm.publishSnapshot(1);
    await flushPromises();
    expect(ElMessage.error).toHaveBeenCalledWith("当前快照状态不允许该操作。");
  });

  test("发布失败错误码 CONFLICT 映射为可读提示", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    publishApi.mockRejectedValue(new ApiError(409, "CONFLICT", "raw"));
    const { vm } = await mountTab({ items: [makeItem({ id: 1, status: "draft" })] });
    await vm.publishSnapshot(1);
    await flushPromises();
    expect(ElMessage.error).toHaveBeenCalledWith("资源状态冲突，请刷新后重试。");
  });
});
