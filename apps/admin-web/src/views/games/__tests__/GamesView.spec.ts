import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

// 避免加载真实路由图（@/api/http 默认导入 @/router）
vi.mock("@/router", () => ({
  default: { currentRoute: { value: { path: "/games", fullPath: "/games" } }, push: vi.fn() }
}));

const pushMock = vi.fn();
vi.mock("vue-router", () => ({
  useRouter: () => ({ push: pushMock })
}));

const listGamesApi = vi.fn();
vi.mock("@/api/modules/games", () => ({
  listGames: (...args: unknown[]) => listGamesApi(...args)
}));

import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import GamesView from "@/views/games/GamesView.vue";
import type { GameDetail, GameListItem } from "@/api/modules/games";

function listItem(overrides: Partial<GameListItem> = {}): GameListItem {
  return {
    gameId: "100001",
    name: "测试游戏",
    alias: "demo",
    iconUrl: "",
    status: "active",
    defaultMarketCode: "GLOBAL",
    marketCodes: ["GLOBAL", "JP"],
    marketCount: 2,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T00:00:00Z",
    ...overrides
  };
}

interface GamesVM {
  rows: GameListItem[];
  total: number;
  page: number;
  keyword: string;
  statusFilter: string;
  marketFilter: string;
  secretDialogVisible: boolean;
  createdGame: GameDetail | null;
  reload: (p?: number) => Promise<void>;
  onCreated: (g: GameDetail) => void;
}

function mountView(perms: string[] = ["game.read", "game.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  const wrapper = mount(GamesView, {
    global: {
      directives: { perm: permDirective },
      stubs: { CreateGameDrawer: true, EnvironmentBadge: true }
    }
  });
  return { wrapper, vm: wrapper.vm as unknown as GamesVM };
}

describe("GamesView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listGamesApi.mockResolvedValue({ items: [listItem()], page: 1, pageSize: 20, total: 1 });
  });

  test("挂载时按第 1 页加载列表并渲染行", async () => {
    const { wrapper, vm } = mountView();
    await flushPromises();
    expect(listGamesApi).toHaveBeenCalledTimes(1);
    expect(listGamesApi.mock.calls[0][0]).toMatchObject({ page: 1, pageSize: 20 });
    expect(vm.rows).toHaveLength(1);
    expect(wrapper.text()).toContain("100001");
    expect(wrapper.text()).toContain("测试游戏");
  });

  test("列表不展示 gameSecret（列表项契约无 secret 字段）", async () => {
    const { wrapper } = mountView();
    await flushPromises();
    // 列表项响应中无 gameSecret/secret 字段，页面不应出现任何密钥相关文案
    expect(wrapper.text().toLowerCase()).not.toContain("secret");
  });

  test("筛选条件（keyword/status/market）透传到列表查询，空值不下发", async () => {
    const { vm } = mountView();
    await flushPromises();
    listGamesApi.mockClear();

    vm.keyword = "war";
    vm.statusFilter = "active";
    vm.marketFilter = "JP";
    await vm.reload(1);
    await flushPromises();

    expect(listGamesApi).toHaveBeenCalledTimes(1);
    expect(listGamesApi.mock.calls[0][0]).toMatchObject({
      page: 1,
      pageSize: 20,
      keyword: "war",
      status: "active",
      marketCode: "JP"
    });

    // 清空筛选 → 不再下发空字符串
    listGamesApi.mockClear();
    vm.keyword = "";
    vm.statusFilter = "";
    vm.marketFilter = "";
    await vm.reload(1);
    await flushPromises();
    const q = listGamesApi.mock.calls[0][0] as Record<string, unknown>;
    expect(q.keyword).toBeUndefined();
    expect(q.status).toBeUndefined();
    expect(q.marketCode).toBeUndefined();
  });

  test("分页切换按目标页加载", async () => {
    const { vm } = mountView();
    await flushPromises();
    listGamesApi.mockResolvedValue({ items: [listItem()], page: 3, pageSize: 20, total: 50 });
    await vm.reload(3);
    await flushPromises();
    expect(listGamesApi.mock.calls.at(-1)?.[0]).toMatchObject({ page: 3 });
    expect(vm.page).toBe(3);
    expect(vm.total).toBe(50);
  });

  test("创建成功后一次性弹窗展示明文 gameSecret 并刷新列表", async () => {
    const { vm } = mountView();
    await flushPromises();
    listGamesApi.mockClear();

    const created: GameDetail = {
      gameId: "100002",
      name: "新游戏",
      alias: "newone",
      iconUrl: "",
      status: "draft",
      defaultMarketCode: "GLOBAL",
      gameSecret: "PLAINTEXT-SECRET-XYZ",
      secretMasked: false,
      markets: [],
      legalLinks: [],
      createdAt: "",
      updatedAt: ""
    };
    vm.onCreated(created);
    await flushPromises();

    expect(vm.secretDialogVisible).toBe(true);
    expect(vm.createdGame?.gameSecret).toBe("PLAINTEXT-SECRET-XYZ");
    expect(vm.createdGame?.secretMasked).toBe(false);
    // 创建后刷新到第 1 页
    expect(listGamesApi).toHaveBeenCalledWith(expect.objectContaining({ page: 1 }));
  });

  test("点击行跳转到详情路由", async () => {
    const { vm } = mountView();
    await flushPromises();
    (vm as unknown as { goDetail: (r: GameListItem) => void }).goDetail(listItem({ gameId: "100009" }));
    expect(pushMock).toHaveBeenCalledWith({ name: "game-detail", params: { gameId: "100009" } });
  });

  test("无 game.write 权限时新建按钮置灰禁用", async () => {
    const { wrapper } = mountView(["game.read"]);
    await flushPromises();
    const createBtn = wrapper
      .findAll("button")
      .find((b) => b.text().includes("新建游戏"));
    expect(createBtn).toBeTruthy();
    expect(createBtn!.attributes("disabled")).toBe("disabled");
  });
});
