import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

const createGameApi = vi.fn();
vi.mock("@/api/modules/games", () => ({
  createGame: (...args: unknown[]) => createGameApi(...args)
}));

import { ApiError } from "@/api/http";
import CreateGameDrawer from "@/views/games/components/CreateGameDrawer.vue";

interface DrawerVM {
  form: {
    name: string;
    alias: string;
    iconUrl: string;
    markets: string[];
    defaultMarketCode: string;
    status: string;
  };
  defaultMarketChoices: string[];
  aliasError: boolean;
  formError: string;
  visible: boolean;
  submit: () => Promise<void>;
}

function mountDrawer() {
  const wrapper = mount(CreateGameDrawer, {
    props: { open: true }
  });
  return { wrapper, vm: wrapper.vm as unknown as DrawerVM };
}

describe("CreateGameDrawer", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  test("默认市场跟随已选市场联动：默认值不在集合内时回退到第一项", async () => {
    const { wrapper, vm } = mountDrawer();
    vm.form.markets = ["JP", "KR"];
    await wrapper.vm.$nextTick();

    // GLOBAL 已不在已选集合 → 回退到第一项 JP
    expect(vm.form.defaultMarketCode).toBe("JP");
    // 默认市场候选项只来自已选市场
    expect(vm.defaultMarketChoices).toEqual(["JP", "KR"]);
  });

  test("默认市场候选在未选任何市场时回退到全部市场", async () => {
    const { wrapper, vm } = mountDrawer();
    vm.form.markets = [];
    await wrapper.vm.$nextTick();
    expect(vm.defaultMarketChoices).toEqual(["GLOBAL", "JP", "KR", "SEA", "HMT", "CN"]);
  });

  test("alias 非法字符即时校验并阻止提交", async () => {
    const { wrapper, vm } = mountDrawer();
    vm.form.name = "测试游戏";
    vm.form.alias = "bad alias!";
    await wrapper.vm.$nextTick();
    expect(vm.aliasError).toBe(true);

    await vm.submit();
    await flushPromises();
    expect(vm.formError).toContain("alias");
    expect(createGameApi).not.toHaveBeenCalled();
  });

  test("游戏名称为空时阻止提交", async () => {
    const { vm } = mountDrawer();
    vm.form.name = "   ";
    vm.form.alias = "valid_alias";
    await vm.submit();
    await flushPromises();
    expect(vm.formError).toContain("游戏名称");
    expect(createGameApi).not.toHaveBeenCalled();
  });

  test("创建成功后回传明文 gameSecret 并关闭抽屉", async () => {
    const created = {
      gameId: "100001",
      name: "测试游戏",
      alias: "valid_alias",
      iconUrl: "",
      status: "draft",
      defaultMarketCode: "JP",
      gameSecret: "plain-secret-abc123",
      secretMasked: false,
      markets: [],
      legalLinks: [],
      createdAt: "",
      updatedAt: ""
    };
    createGameApi.mockResolvedValue(created);

    const { wrapper, vm } = mountDrawer();
    vm.form.name = "测试游戏";
    vm.form.alias = "valid_alias";
    vm.form.markets = ["JP"];
    await wrapper.vm.$nextTick();

    await vm.submit();
    await flushPromises();

    expect(createGameApi).toHaveBeenCalledWith(
      expect.objectContaining({ name: "测试游戏", alias: "valid_alias", markets: ["JP"], defaultMarketCode: "JP" })
    );
    const emitted = wrapper.emitted("created");
    expect(emitted).toBeTruthy();
    expect((emitted![0][0] as typeof created).gameSecret).toBe("plain-secret-abc123");
    expect((emitted![0][0] as typeof created).secretMasked).toBe(false);
    expect(vm.visible).toBe(false);
  });

  test("alias 冲突（409）展示后端错误消息", async () => {
    createGameApi.mockRejectedValue(new ApiError(409, "CONFLICT", "alias already exists"));
    const { vm } = mountDrawer();
    vm.form.name = "测试游戏";
    vm.form.alias = "dup_alias";
    await vm.submit();
    await flushPromises();
    expect(vm.formError).toBe("alias already exists");
  });
});
