import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import ChannelInstancesTab from "@/views/channels/components/ChannelInstancesTab.vue";

const listGameChannelsApi = vi.fn();
const listMarketChannelsApi = vi.fn();
const hideMarketChannelApi = vi.fn();
const unhideMarketChannelApi = vi.fn();

vi.mock("@/api/modules/channels", () => ({
  listGameChannels: (...args: unknown[]) => listGameChannelsApi(...args),
  listMarketChannels: (...args: unknown[]) => listMarketChannelsApi(...args),
  hideMarketChannel: (...args: unknown[]) => hideMarketChannelApi(...args),
  unhideMarketChannel: (...args: unknown[]) => unhideMarketChannelApi(...args)
}));

describe("ChannelInstancesTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setActivePinia(createPinia());
    usePermissionStore().setFromUser({ roles: [], permissions: ["channel.read", "channel.write"] });
  });

  test("创建抽屉使用全量实例（跨分页、含隐藏）作为去重输入", async () => {
    listGameChannelsApi.mockResolvedValue([
      {
        channelId: "google",
        channelName: "Google Play",
        channelType: "store",
        region: "overseas",
        loginMode: "account_system",
        paymentMode: "hybrid",
        loginLocked: false,
        paymentLocked: false
      }
    ]);

    listMarketChannelsApi.mockImplementation((_gameId: string, query?: Record<string, unknown>) => {
      // reload() 默认仅查非隐藏列表
      if (query?.hidden === false) {
        return Promise.resolve({
          items: [
            {
              gameChannelId: 101,
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
              updatedAt: "2026-01-01T00:00:00Z"
            }
          ],
          page: 1,
          pageSize: 20,
          total: 1
        });
      }

      // loadAllInstancesForCreate()：hidden=undefined，按 200 分页拉全量
      const page = Number(query?.page ?? 1);
      if (page === 1) {
        return Promise.resolve({
          items: [
            {
              gameChannelId: 101,
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
              updatedAt: "2026-01-01T00:00:00Z"
            }
          ],
          page: 1,
          pageSize: 200,
          total: 2
        });
      }

      return Promise.resolve({
        items: [
          {
            gameChannelId: 202,
            displayKey: "100001:CN:huawei_cn",
            gameId: "100001",
            market: "CN",
            channelId: "huawei_cn",
            region: "domestic",
            compatible: true,
            hidden: true,
            configStatus: "invalid",
            includedInSnapshot: false,
            includedInSync: false,
            includedInRuntimeConfig: false,
            copiedFromMarket: "",
            updatedAt: "2026-01-01T00:00:00Z"
          }
        ],
        page: 2,
        pageSize: 200,
        total: 2
      });
    });

    const wrapper = mount(ChannelInstancesTab, {
      props: { gameId: "100001" },
      global: {
        directives: { perm: permDirective },
        stubs: {
          ChannelInstanceTable: true,
          ChannelInstanceDetailDrawer: true
        }
      }
    });
    await flushPromises();

    const drawer = wrapper.findComponent({ name: "CreateMarketChannelDrawer" });
    const existingItems = drawer.props("existingItems") as Array<{ channelId: string; hidden: boolean }>;
    expect(existingItems).toHaveLength(2);
    expect(existingItems.map((item) => item.channelId)).toEqual(["google", "huawei_cn"]);
    expect(existingItems.some((item) => item.hidden)).toBe(true);

    // 确认创建侧全量拉取参数（hidden=undefined + pageSize=200）被触发
    expect(
      listMarketChannelsApi.mock.calls.some(
        ([, query]) => query?.hidden === undefined && query?.pageSize === 200 && query?.page === 1
      )
    ).toBe(true);
    expect(
      listMarketChannelsApi.mock.calls.some(
        ([, query]) => query?.hidden === undefined && query?.pageSize === 200 && query?.page === 2
      )
    ).toBe(true);
  });
});
