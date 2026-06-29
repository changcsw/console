import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { nextTick } from "vue";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import ChannelInstanceDetailDrawer from "@/views/channels/components/ChannelInstanceDetailDrawer.vue";

const getMarketChannelApi = vi.fn();
const listChannelPackagesApi = vi.fn();
const updateMarketChannelApi = vi.fn();
const hideMarketChannelApi = vi.fn();
const unhideMarketChannelApi = vi.fn();
const createChannelPackageApi = vi.fn();
const updateChannelPackageApi = vi.fn();

vi.mock("@/api/modules/channels", () => ({
  getMarketChannel: (...args: unknown[]) => getMarketChannelApi(...args),
  listChannelPackages: (...args: unknown[]) => listChannelPackagesApi(...args),
  updateMarketChannel: (...args: unknown[]) => updateMarketChannelApi(...args),
  hideMarketChannel: (...args: unknown[]) => hideMarketChannelApi(...args),
  unhideMarketChannel: (...args: unknown[]) => unhideMarketChannelApi(...args),
  createChannelPackage: (...args: unknown[]) => createChannelPackageApi(...args),
  updateChannelPackage: (...args: unknown[]) => updateChannelPackageApi(...args)
}));

describe("ChannelInstanceDetailDrawer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setActivePinia(createPinia());
    getMarketChannelApi.mockResolvedValue({
      gameChannelId: 1,
      displayKey: "100001:GLOBAL:google",
      gameId: "100001",
      market: "GLOBAL",
      channelId: "google",
      region: "overseas",
      compatible: true,
      hidden: false,
      configStatus: "valid",
      includedInSnapshot: true,
      includedInSync: true,
      includedInRuntimeConfig: true,
      copiedFromMarket: "",
      updatedAt: "2026-01-01T00:00:00Z",
      enabled: true,
      remark: "",
      hiddenBy: "",
      hiddenAt: null,
      lastCheckAt: null,
      lastCheckMessage: "",
      createdAt: "2026-01-01T00:00:00Z"
    });
    listChannelPackagesApi.mockResolvedValue([]);
  });

  test("canWrite 基于 computed，权限变更后输入禁用态可响应更新", async () => {
    const permission = usePermissionStore();
    permission.setFromUser({ roles: [], permissions: ["channel.read"] });

    const wrapper = mount(ChannelInstanceDetailDrawer, {
      props: { open: true, gameChannelId: 1 },
      global: { directives: { perm: permDirective } }
    });
    await flushPromises();

    const vm = wrapper.vm as unknown as { canWrite: boolean };
    expect(vm.canWrite).toBe(false);

    permission.setFromUser({ roles: [], permissions: ["channel.read", "channel.write"] });
    await nextTick();
    await flushPromises();

    expect(vm.canWrite).toBe(true);
  });
});
