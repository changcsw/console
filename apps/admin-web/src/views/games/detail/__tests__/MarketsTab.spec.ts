import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

const replaceMarketsApi = vi.fn();
vi.mock("@/api/modules/games", () => ({
  replaceMarkets: (...args: unknown[]) => replaceMarketsApi(...args)
}));

import { ApiError } from "@/api/http";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import MarketsTab from "@/views/games/detail/MarketsTab.vue";
import type { GameDetail } from "@/api/modules/games";

function makeGame(): GameDetail {
  return {
    gameId: "100001",
    name: "测试游戏",
    alias: "demo",
    iconUrl: "",
    status: "active",
    defaultMarketCode: "GLOBAL",
    gameSecret: "masked",
    secretMasked: true,
    markets: [
      { marketCode: "GLOBAL", isDefault: true, enabled: true, defaultLocale: "en-US" },
      { marketCode: "JP", isDefault: false, enabled: true, defaultLocale: "ja-JP" }
    ],
    legalLinks: [],
    createdAt: "",
    updatedAt: ""
  };
}

interface MarketsVM {
  drawerVisible: boolean;
  formError: string;
  selectedMarkets: string[];
  defaultMarketCode: string;
  rows: { marketCode: string; enabled: boolean; defaultLocale: string }[];
  openEdit: () => void;
  submit: () => Promise<void>;
}

function mountTab(perms: string[] = ["game.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  const wrapper = mount(MarketsTab, {
    props: { game: makeGame() },
    global: { directives: { perm: permDirective } }
  });
  return { wrapper, vm: wrapper.vm as unknown as MarketsVM };
}

describe("MarketsTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test("openEdit 用当前聚合预填行与默认市场", async () => {
    const { vm } = mountTab();
    vm.openEdit();
    expect(vm.drawerVisible).toBe(true);
    expect(vm.selectedMarkets).toEqual(["GLOBAL", "JP"]);
    expect(vm.defaultMarketCode).toBe("GLOBAL");
    expect(vm.rows.map((r) => r.marketCode)).toEqual(["GLOBAL", "JP"]);
  });

  test("空市场集合阻止提交", async () => {
    const { vm } = mountTab();
    vm.openEdit();
    vm.selectedMarkets = [];
    await flushPromises();
    await vm.submit();
    expect(vm.formError).toContain("至少保留一个市场");
    expect(replaceMarketsApi).not.toHaveBeenCalled();
  });

  test("默认市场不在已选集合内阻止提交", async () => {
    const { vm } = mountTab();
    vm.openEdit();
    // 强制制造默认市场缺失（绕过 watch 自动纠正）
    vm.defaultMarketCode = "CN";
    await vm.submit();
    expect(vm.formError).toContain("请指定一个默认市场");
    expect(replaceMarketsApi).not.toHaveBeenCalled();
  });

  test("提交构造恰好一条 isDefault=true 的全量覆盖载荷", async () => {
    replaceMarketsApi.mockResolvedValue({ ...makeGame(), defaultMarketCode: "JP" });
    const { wrapper, vm } = mountTab();
    vm.openEdit();
    vm.defaultMarketCode = "JP";
    await vm.submit();
    await flushPromises();

    expect(replaceMarketsApi).toHaveBeenCalledTimes(1);
    const [gameId, payload] = replaceMarketsApi.mock.calls[0];
    expect(gameId).toBe("100001");
    const defaults = payload.markets.filter((m: { isDefault: boolean }) => m.isDefault);
    expect(defaults).toHaveLength(1);
    expect(defaults[0].marketCode).toBe("JP");
    expect(wrapper.emitted("updated")).toBeTruthy();
    expect(vm.drawerVisible).toBe(false);
  });

  test("移除被占用市场返回 409 展示后端错误", async () => {
    replaceMarketsApi.mockRejectedValue(
      new ApiError(409, "CONFLICT", "cannot remove market with existing channels")
    );
    const { vm } = mountTab();
    vm.openEdit();
    await vm.submit();
    await flushPromises();
    expect(vm.formError).toBe("cannot remove market with existing channels");
    expect(vm.drawerVisible).toBe(true);
  });

  test("无 game.write 权限时编辑按钮被置灰禁用", () => {
    const { wrapper } = mountTab([]);
    const btn = wrapper.find(".markets-tab__toolbar button");
    expect(btn.attributes("disabled")).toBe("disabled");
    expect(btn.classes()).toContain("perm-disabled");
  });
});
