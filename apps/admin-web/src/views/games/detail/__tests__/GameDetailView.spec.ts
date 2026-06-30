import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

// 避免加载真实路由图（@/api/http 默认导入 @/router）
vi.mock("@/router", () => ({
  default: { currentRoute: { value: { path: "/games/100001", fullPath: "/games/100001" } }, push: vi.fn() }
}));

const pushMock = vi.fn();
vi.mock("vue-router", () => ({
  useRoute: () => ({ params: { gameId: "100001" } }),
  useRouter: () => ({ push: pushMock })
}));

const getGameApi = vi.fn();
vi.mock("@/api/modules/games", () => ({
  getGame: (...args: unknown[]) => getGameApi(...args)
}));

import { ApiError } from "@/api/http";
import GameDetailView from "@/views/games/detail/GameDetailView.vue";
import type { GameDetail } from "@/api/modules/games";

function maskedGame(): GameDetail {
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
    markets: [{ marketCode: "GLOBAL", isDefault: true, enabled: true, defaultLocale: "en-US" }],
    legalLinks: [],
    createdAt: "",
    updatedAt: ""
  };
}

function mountView() {
  setActivePinia(createPinia());
  return mount(GameDetailView, {
    global: {
      stubs: {
        EnvironmentBadge: true,
        BasicInfoTab: true,
        MarketsTab: true,
        LegalLinksTab: true,
        AccountAuthTab: true,
        ProductTab: true,
        IapConfigTab: true,
        GameCashierTab: true
      }
    }
  });
}

describe("GameDetailView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test("详情头部 gameSecret 恒脱敏展示 masked，不出现明文", async () => {
    getGameApi.mockResolvedValue(maskedGame());
    const wrapper = mountView();
    await flushPromises();

    expect(getGameApi).toHaveBeenCalledWith("100001");
    const text = wrapper.text();
    expect(text).toContain("masked");
    // 头部展示 Game ID / 代号，但 Secret 区只显示脱敏值
    expect(text).toContain("100001");
    expect(text).not.toContain("PLAINTEXT");
  });

  test("404 展示『游戏不存在或已切换环境』", async () => {
    getGameApi.mockRejectedValue(new ApiError(404, "NOT_FOUND", "game not found"));
    const wrapper = mountView();
    await flushPromises();

    expect(wrapper.text()).toContain("游戏不存在或已切换环境");
    // 不渲染 Tab 区
    expect(wrapper.findComponent({ name: "BasicInfoTab" }).exists()).toBe(false);
  });

  test("加载成功渲染主 Tab 与下游占位", async () => {
    getGameApi.mockResolvedValue(maskedGame());
    const wrapper = mountView();
    await flushPromises();
    const text = wrapper.text();
    expect(text).toContain("基础信息");
    expect(text).toContain("市场");
    expect(text).toContain("法务链接");
    expect(text).toContain("自有账号认证");
    expect(text).toContain("收银台");
    // 下游占位 Tab（渠道/支付路由等）
    expect(text).toContain("渠道");
    expect(text).toContain("支付路由");
  });
});
