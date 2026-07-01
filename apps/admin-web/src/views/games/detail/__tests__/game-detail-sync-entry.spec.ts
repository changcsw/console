import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

// sync #21 · GameDetailView「Sync to Production」入口红线测试（L4 vitest）。
// 红线（00 §9 / 01 §2）：仅 sandbox 运行环境渲染同步入口；production 绝不渲染；
// 缺 sync.execute 权限时入口置灰（v-perm 禁用）。

vi.mock("@/router", () => ({
  default: { currentRoute: { value: { path: "/games/100001", fullPath: "/games/100001" } }, push: vi.fn() }
}));

vi.mock("vue-router", () => ({
  useRoute: () => ({ params: { gameId: "100001" } }),
  useRouter: () => ({ push: vi.fn() })
}));

const getGameApi = vi.fn();
vi.mock("@/api/modules/games", () => ({
  getGame: (...args: unknown[]) => getGameApi(...args)
}));

import permDirective from "@/directives/perm";
import { useAppStore } from "@/stores/app";
import { usePermissionStore } from "@/stores/permission";
import GameDetailView from "@/views/games/detail/GameDetailView.vue";
import type { GameDetail } from "@/api/modules/games";

function maskedGame(): GameDetail {
  return {
    gameId: "100001",
    name: "星际远征",
    alias: "starfront",
    iconUrl: "",
    status: "active",
    defaultMarketCode: "GLOBAL",
    gameSecret: "masked",
    secretMasked: true,
    environment: "sandbox",
    markets: [{ marketCode: "GLOBAL", isDefault: true, enabled: true, defaultLocale: "en-US" }],
    legalLinks: [],
    createdAt: "",
    updatedAt: ""
  } as GameDetail;
}

async function mountView(options: { environment: string; perms?: string[] }) {
  setActivePinia(createPinia());
  useAppStore().setEnvironment(options.environment);
  usePermissionStore().setFromUser({ roles: [], permissions: options.perms ?? ["game.read"] });
  getGameApi.mockResolvedValue(maskedGame());

  const wrapper = mount(GameDetailView, {
    global: {
      directives: { perm: permDirective },
      stubs: {
        EnvironmentBadge: true,
        BasicInfoTab: true,
        MarketsTab: true,
        LegalLinksTab: true,
        AccountAuthTab: true,
        ProductTab: true,
        IapConfigTab: true,
        GameCashierTab: true,
        PaymentRoutesTab: true,
        SnapshotTab: true,
        SyncJobsTab: true,
        SyncSectionDrawer: true
      }
    }
  });
  await flushPromises();
  return wrapper;
}

function syncButton(wrapper: Awaited<ReturnType<typeof mountView>>) {
  return wrapper
    .findAll("button")
    .find((btn) => btn.text().includes("Sync to Production"));
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("GameDetailView · Sync 入口红线", () => {
  test("红线：production 运行环境绝不渲染 Sync to Production 入口", async () => {
    const wrapper = await mountView({ environment: "production", perms: ["game.read", "sync.execute"] });
    expect(wrapper.text()).not.toContain("Sync to Production");
    expect(syncButton(wrapper)).toBeUndefined();
  });

  test("红线：develop 运行环境也不渲染 Sync 入口", async () => {
    const wrapper = await mountView({ environment: "develop", perms: ["game.read", "sync.execute"] });
    expect(syncButton(wrapper)).toBeUndefined();
  });

  test("sandbox + sync.execute：渲染入口且可用，点击打开同步抽屉", async () => {
    const wrapper = await mountView({ environment: "sandbox", perms: ["game.read", "sync.execute"] });
    const btn = syncButton(wrapper);
    expect(btn).toBeTruthy();
    expect(btn!.attributes("disabled")).toBeUndefined();
    expect(btn!.classes()).not.toContain("perm-disabled");

    await btn!.trigger("click");
    await flushPromises();
    const drawer = wrapper.findComponent({ name: "SyncSectionDrawer" });
    expect(drawer.props("open")).toBe(true);
  });

  test("sandbox 但缺 sync.execute 权限：入口置灰禁用（v-perm）", async () => {
    const wrapper = await mountView({ environment: "sandbox", perms: ["game.read"] });
    const btn = syncButton(wrapper);
    expect(btn).toBeTruthy();
    expect(btn!.attributes("disabled")).toBe("disabled");
    expect(btn!.classes()).toContain("perm-disabled");
  });
});
