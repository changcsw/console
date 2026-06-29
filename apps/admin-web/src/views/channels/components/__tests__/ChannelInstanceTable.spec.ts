import { beforeEach, describe, expect, test } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import ChannelInstanceTable from "@/views/channels/components/ChannelInstanceTable.vue";
import type { AvailableChannel, MarketChannelListItem } from "@/api/modules/channels";

function row(overrides: Partial<MarketChannelListItem> = {}): MarketChannelListItem {
  return {
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
    ...overrides
  };
}

const channels: AvailableChannel[] = [
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
];

function mountTable(perms: string[]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  return mount(ChannelInstanceTable, {
    props: { items: [row()], availableChannels: channels, loading: false },
    global: { directives: { perm: permDirective } }
  });
}

describe("ChannelInstanceTable", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  test("渠道名映射优先于 channelId（列表展示优化依赖）", () => {
    const wrapper = mountTable(["channel.read", "channel.write"]);
    const vm = wrapper.vm as unknown as {
      channelNameMap: Record<string, string>;
      rowClassName: (arg: { row: MarketChannelListItem }) => string;
    };
    expect(vm.channelNameMap.google).toBe("Google Play");
    expect(vm.rowClassName({ row: row({ compatible: false }) })).toBe("row-incompatible");
    expect(vm.rowClassName({ row: row({ hidden: true }) })).toBe("row-hidden");
  });
});
