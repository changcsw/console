import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { ElMessage } from "element-plus";
import permDirective from "@/directives/perm";
import { ApiError } from "@/api/http";
import { useAppStore } from "@/stores/app";
import { usePermissionStore } from "@/stores/permission";
import PaymentRoutesTab from "@/views/games/detail/PaymentRoutesTab.vue";

const requestApi = vi.fn();
vi.mock("@/api/http", async () => {
  const actual = await vi.importActual<typeof import("@/api/http")>("@/api/http");
  return { ...actual, request: (...args: unknown[]) => requestApi(...args) };
});

const listPayWaysApi = vi.fn();
const listProvidersApi = vi.fn();
const listMerchantAccountsApi = vi.fn();
const getGamePaymentRoutesApi = vi.fn();
const saveGamePaymentRoutesApi = vi.fn();

vi.mock("@/api/modules/payment", () => ({
  listPayWays: (...args: unknown[]) => listPayWaysApi(...args),
  listProviders: (...args: unknown[]) => listProvidersApi(...args),
  listMerchantAccounts: (...args: unknown[]) => listMerchantAccountsApi(...args),
  getGamePaymentRoutes: (...args: unknown[]) => getGamePaymentRoutesApi(...args),
  saveGamePaymentRoutes: (...args: unknown[]) => saveGamePaymentRoutesApi(...args),
  isRouteConflictError: (err: unknown) => err instanceof ApiError && err.code === "ROUTE_CONFLICT",
}));

const listMarketChannelsApi = vi.fn();
const listChannelPackagesApi = vi.fn();
vi.mock("@/api/modules/channels", () => ({
  listMarketChannels: (...args: unknown[]) => listMarketChannelsApi(...args),
  listChannelPackages: (...args: unknown[]) => listChannelPackagesApi(...args),
}));

function makeRoutes() {
  return {
    gameId: "100001",
    env: "sandbox",
    groups: [
      {
        payWayId: "wallet",
        payWayName: "Wallet",
        payWayType: "wallet",
        routes: [
          {
            id: 201,
            selector: { packageCode: "*", channelId: "*", marketCode: "GLOBAL", countryCode: "*", currency: "*" },
            providerId: "p1",
            merchantAccountId: "m1",
            priority: 200,
            enabled: true,
          },
          {
            id: 202,
            selector: { packageCode: "pkg_a", channelId: "ch_a", marketCode: "JP", countryCode: "JP", currency: "JPY" },
            providerId: "p2",
            merchantAccountId: "m2",
            priority: 10,
            enabled: true,
          },
        ],
      },
      {
        payWayId: "card",
        payWayName: "Card",
        payWayType: "card",
        routes: [
          {
            id: 101,
            selector: { packageCode: "*", channelId: "*", marketCode: "GLOBAL", countryCode: "*", currency: "*" },
            providerId: "p1",
            merchantAccountId: "m1",
            priority: 100,
            enabled: true,
            hasDisabledReference: true,
          },
          {
            id: 102,
            selector: { packageCode: "pkg_b", channelId: "ch_b", marketCode: "KR", countryCode: "KR", currency: "KRW" },
            providerId: "p2",
            merchantAccountId: "m2",
            priority: 20,
            enabled: true,
          },
        ],
      },
    ],
  };
}

async function mountTab(options: { perms?: string[]; env?: "sandbox" | "production" } = {}) {
  setActivePinia(createPinia());
  const app = useAppStore();
  app.setEnvironment(options.env ?? "sandbox");
  usePermissionStore().setFromUser({ roles: [], permissions: options.perms ?? ["payment.read", "payment.write"] });

  requestApi.mockResolvedValue({
    items: [{ currencyCode: "USD", currencyName: "US Dollar", decimalPlaces: 2, minAmountMinor: 1, roundingMode: "half_up", enabled: true }],
  });
  listPayWaysApi.mockResolvedValue({
    items: [
      { payWayId: "wallet", payWayName: "Wallet", payWayType: "wallet", enabled: true, sort: 20 },
      { payWayId: "card", payWayName: "Card", payWayType: "card", enabled: true, sort: 10 },
    ],
    page: 1,
    pageSize: 20,
    total: 2,
  });
  listProvidersApi.mockResolvedValue({
    items: [
      { providerId: "p1", providerName: "Provider A", providerKind: "gateway", enabled: true, sort: 1 },
      { providerId: "p2", providerName: "Provider B", providerKind: "gateway", enabled: true, sort: 2 },
    ],
    page: 1,
    pageSize: 20,
    total: 2,
  });
  listMerchantAccountsApi.mockResolvedValue({
    items: [
      {
        merchantAccountId: "m1",
        providerId: "p1",
        subjectId: "s1",
        merchantId: "mid1",
        merchantName: "M-1",
        configJson: {},
        secret: "masked",
        enabled: true,
      },
      {
        merchantAccountId: "m2",
        providerId: "p2",
        subjectId: "s1",
        merchantId: "mid2",
        merchantName: "M-2",
        configJson: {},
        secret: "masked",
        enabled: true,
      },
    ],
    page: 1,
    pageSize: 20,
    total: 2,
  });
  listMarketChannelsApi.mockResolvedValue({
    items: [{ gameChannelId: 1, gameId: "100001", market: "GLOBAL", channelId: "ch_a" }],
    page: 1,
    pageSize: 20,
    total: 1,
  });
  listChannelPackagesApi.mockResolvedValue([{ packageCode: "pkg_a", packageName: "Package A" }]);
  getGamePaymentRoutesApi.mockResolvedValue(makeRoutes());
  saveGamePaymentRoutesApi.mockResolvedValue(makeRoutes());

  const wrapper = mount(PaymentRoutesTab, {
    props: { gameId: "100001" },
    global: { directives: { perm: permDirective } },
  });
  await flushPromises();
  return { wrapper, vm: wrapper.vm as unknown as Record<string, any> };
}

describe("PaymentRoutesTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "warning").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
  });

  test("按 payWay 分组渲染并保持后端返回顺序，兜底/禁用引用状态可见", async () => {
    const { wrapper } = await mountTab();

    const headers = wrapper.findAll(".el-collapse-item__header").map((node) => node.text());
    expect(headers[0]).toContain("Wallet (wallet)");
    expect(headers[1]).toContain("Card (card)");
    expect(wrapper.text()).toContain("兜底");
    expect(wrapper.text()).toContain("引用对象已禁用");
  });

  test("production 隐藏 Sync 且无 payment.write 时写入口置灰", async () => {
    const { wrapper } = await mountTab({ perms: ["payment.read"], env: "production" });

    expect(wrapper.text()).not.toContain("Sync to Production");
    expect(wrapper.text()).toContain("当前账号仅有查看权限，编辑入口已置灰。");
    const createBtn = wrapper.find(".group-head .el-button");
    expect(createBtn.attributes("disabled")).toBe("disabled");
    expect(createBtn.classes()).toContain("perm-disabled");
  });

  test("切 PSP 时 merchant 仅按 provider 过滤", async () => {
    const { vm } = await mountTab();

    vm.openSwitchPsp("card", 0);
    expect(vm.switchMerchants.map((item: { merchantAccountId: string }) => item.merchantAccountId)).toEqual(["m1"]);
    vm.onSwitchProviderChange("p2");
    expect(vm.switchForm.merchantAccountId).toBe("");
    expect(vm.switchMerchants.map((item: { merchantAccountId: string }) => item.merchantAccountId)).toEqual(["m2"]);
  });

  test("ROUTE_CONFLICT 会按 duplicate_priority/duplicate_selector 区分高亮", async () => {
    const conflict = new ApiError(409, "ROUTE_CONFLICT", "conflict", [
      { kind: "duplicate_priority", leftIndex: 0, rightIndex: 1 },
      { kind: "duplicate_selector", leftIndex: 2, rightIndex: 3 },
    ]);
    saveGamePaymentRoutesApi.mockRejectedValueOnce(conflict);
    const { vm } = await mountTab();

    vm.openEdit("wallet", 0);
    await vm.saveRoute();
    await flushPromises();

    const wallet = vm.routesData.groups[0];
    const card = vm.routesData.groups[1];
    expect(vm.rowClassName(wallet, wallet.routes[0])).toBe("row--conflict-priority");
    expect(vm.rowClassName(wallet, wallet.routes[1])).toBe("row--conflict-priority");
    expect(vm.rowClassName(card, card.routes[0])).toBe("row--conflict-selector");
    expect(vm.rowClassName(card, card.routes[1])).toBe("row--conflict-selector");
    expect(ElMessage.error).toHaveBeenCalledWith(
      expect.stringContaining("duplicate_priority")
    );
    expect(ElMessage.error).toHaveBeenCalledWith(
      expect.stringContaining("duplicate_selector")
    );
  });
});
