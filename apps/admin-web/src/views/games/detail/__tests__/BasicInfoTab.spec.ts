import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

const updateGameApi = vi.fn();
vi.mock("@/api/modules/games", () => ({
  updateGame: (...args: unknown[]) => updateGameApi(...args)
}));

import { ApiError } from "@/api/http";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import BasicInfoTab from "@/views/games/detail/BasicInfoTab.vue";
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
    environment: "sandbox",
    markets: [
      { marketCode: "GLOBAL", isDefault: true, enabled: true, defaultLocale: "en-US" },
      { marketCode: "JP", isDefault: false, enabled: false, defaultLocale: "ja-JP" }
    ],
    legalLinks: [],
    createdAt: "",
    updatedAt: ""
  };
}

interface BasicVM {
  drawerVisible: boolean;
  formError: string;
  form: { name: string; alias: string; iconUrl: string; status: string; defaultMarketCode: string };
  enabledMarketCodes: string[];
  openEdit: () => void;
  submit: () => Promise<void>;
}

function mountTab(perms: string[] = ["game.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  const wrapper = mount(BasicInfoTab, {
    props: { game: makeGame() },
    global: { directives: { perm: permDirective } }
  });
  return { wrapper, vm: wrapper.vm as unknown as BasicVM };
}

describe("BasicInfoTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test("详情展示脱敏 gameSecret 与恒脱敏说明", () => {
    const { wrapper } = mountTab();
    const text = wrapper.text();
    expect(text).toContain("masked");
    expect(text).toContain("恒脱敏");
  });

  test("默认市场下拉仅包含已启用市场", () => {
    const { vm } = mountTab();
    expect(vm.enabledMarketCodes).toEqual(["GLOBAL"]);
  });

  test("编辑保存调用 PATCH 并回传更新结果", async () => {
    const updated = { ...makeGame(), name: "改名后" };
    updateGameApi.mockResolvedValue(updated);
    const { wrapper, vm } = mountTab();
    vm.openEdit();
    vm.form.name = "改名后";
    await vm.submit();
    await flushPromises();

    expect(updateGameApi).toHaveBeenCalledWith("100001", expect.objectContaining({ name: "改名后" }));
    expect(wrapper.emitted("updated")).toBeTruthy();
    expect(vm.drawerVisible).toBe(false);
  });

  test("保存失败展示后端错误消息", async () => {
    updateGameApi.mockRejectedValue(new ApiError(409, "CONFLICT", "alias already exists"));
    const { vm } = mountTab();
    vm.openEdit();
    await vm.submit();
    await flushPromises();
    expect(vm.formError).toBe("alias already exists");
  });

  test("无 game.write 权限时编辑按钮置灰禁用", () => {
    const { wrapper } = mountTab([]);
    const btn = wrapper.find(".basic-tab__toolbar button");
    expect(btn.attributes("disabled")).toBe("disabled");
  });
});
