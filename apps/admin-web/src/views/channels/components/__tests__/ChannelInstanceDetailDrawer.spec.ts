import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { nextTick } from "vue";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import ChannelInstanceDetailDrawer from "@/views/channels/components/ChannelInstanceDetailDrawer.vue";
import ChannelLoginConfigPanel from "@/views/channels/components/ChannelLoginConfigPanel.vue";
import FeaturePluginConfigPanel from "@/views/channels/components/FeaturePluginConfigPanel.vue";

const getMarketChannelApi = vi.fn();
const listChannelPackagesApi = vi.fn();
const updateMarketChannelApi = vi.fn();
const hideMarketChannelApi = vi.fn();
const unhideMarketChannelApi = vi.fn();
const createChannelPackageApi = vi.fn();
const updateChannelPackageApi = vi.fn();
const getLoginConfigApi = vi.fn();
const putLoginConfigApi = vi.fn();
const listGameChannelPluginsApi = vi.fn();
const upsertGameChannelPluginApi = vi.fn();
const patchGameChannelPluginApi = vi.fn();
const listChannelPackagePluginsApi = vi.fn();
const upsertChannelPackagePluginApi = vi.fn();

vi.mock("@/api/modules/channels", () => ({
  getMarketChannel: (...args: unknown[]) => getMarketChannelApi(...args),
  listChannelPackages: (...args: unknown[]) => listChannelPackagesApi(...args),
  updateMarketChannel: (...args: unknown[]) => updateMarketChannelApi(...args),
  hideMarketChannel: (...args: unknown[]) => hideMarketChannelApi(...args),
  unhideMarketChannel: (...args: unknown[]) => unhideMarketChannelApi(...args),
  createChannelPackage: (...args: unknown[]) => createChannelPackageApi(...args),
  updateChannelPackage: (...args: unknown[]) => updateChannelPackageApi(...args),
  getLoginConfig: (...args: unknown[]) => getLoginConfigApi(...args),
  putLoginConfig: (...args: unknown[]) => putLoginConfigApi(...args),
  listGameChannelPlugins: (...args: unknown[]) => listGameChannelPluginsApi(...args),
  upsertGameChannelPlugin: (...args: unknown[]) => upsertGameChannelPluginApi(...args),
  patchGameChannelPlugin: (...args: unknown[]) => patchGameChannelPluginApi(...args),
  listChannelPackagePlugins: (...args: unknown[]) => listChannelPackagePluginsApi(...args),
  upsertChannelPackagePlugin: (...args: unknown[]) => upsertChannelPackagePluginApi(...args)
}));

import { loginConfigResponse } from "./fixtures/channelLogin";

describe("ChannelInstanceDetailDrawer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    document.body.innerHTML = "";
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
    listGameChannelPluginsApi.mockResolvedValue([]);
    listChannelPackagePluginsApi.mockResolvedValue([]);
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

  test("仅 channel_only 实例展示「渠道登录」页签并挂载配置面板", async () => {
    setActivePinia(createPinia());
    usePermissionStore().setFromUser({ roles: [], permissions: ["channel.read", "channel.write"] });
    getMarketChannelApi.mockResolvedValue({
      gameChannelId: 101,
      displayKey: "100001:CN:huawei_cn",
      gameId: "100001",
      market: "CN",
      channelId: "huawei_cn",
      region: "domestic",
      compatible: true,
      hidden: false,
      configStatus: "valid",
      includedInSnapshot: true,
      includedInSync: true,
      includedInRuntimeConfig: true,
      copiedFromMarket: "",
      updatedAt: "2026-01-01T00:00:00Z",
      loginMode: "channel_only",
      loginLocked: false,
      enabled: true,
      remark: "",
      hiddenBy: "",
      hiddenAt: null,
      lastCheckAt: null,
      lastCheckMessage: "",
      createdAt: "2026-01-01T00:00:00Z"
    });
    getLoginConfigApi.mockResolvedValue(loginConfigResponse());

    // 抽屉详情加载由 open false→true 触发（与真实打开行为一致）
    const wrapper = mount(ChannelInstanceDetailDrawer, {
      props: { open: false, gameChannelId: 101 },
      global: { directives: { perm: permDirective } }
    });
    await wrapper.setProps({ open: true });
    await flushPromises();

    // channel_only → 「渠道登录」页签内容（ChannelLoginConfigPanel）已挂载
    expect(wrapper.findComponent(ChannelLoginConfigPanel).exists()).toBe(true);
    expect(getLoginConfigApi).toHaveBeenCalledWith(101);
  });

  test("account_system 实例不展示「渠道登录」页签且不拉取 login-config", async () => {
    setActivePinia(createPinia());
    usePermissionStore().setFromUser({ roles: [], permissions: ["channel.read", "channel.write"] });
    // 默认 mock 的 loginMode 为 undefined（account_system 非 channel_only）
    const wrapper = mount(ChannelInstanceDetailDrawer, {
      props: { open: false, gameChannelId: 1 },
      global: { directives: { perm: permDirective } }
    });
    await wrapper.setProps({ open: true });
    await flushPromises();

    // account_system → 不渲染「渠道登录」页签，亦不拉取 login-config
    expect(wrapper.findComponent(ChannelLoginConfigPanel).exists()).toBe(false);
    expect(getLoginConfigApi).not.toHaveBeenCalled();
  });

  test("详情抽屉始终展示「功能插件」并拉取插件列表", async () => {
    setActivePinia(createPinia());
    usePermissionStore().setFromUser({ roles: [], permissions: ["channel.read", "plugin.read"] });
    getMarketChannelApi.mockResolvedValue({
      gameChannelId: 202,
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
    listGameChannelPluginsApi.mockResolvedValue([]);

    const wrapper = mount(ChannelInstanceDetailDrawer, {
      props: { open: false, gameChannelId: 202 },
      global: { directives: { perm: permDirective } }
    });
    await wrapper.setProps({ open: true });
    await flushPromises();

    expect(wrapper.html()).toContain("功能插件");
    expect(listGameChannelPluginsApi).toHaveBeenCalledWith(202);
  });

  test("无 plugin.write 时功能插件面板按 canPluginWrite=false 置灰", async () => {
    setActivePinia(createPinia());
    usePermissionStore().setFromUser({ roles: [], permissions: ["channel.read", "plugin.read"] });
    listGameChannelPluginsApi.mockResolvedValue([]);

    const wrapper = mount(ChannelInstanceDetailDrawer, {
      props: { open: false, gameChannelId: 1 },
      global: { directives: { perm: permDirective } }
    });
    await wrapper.setProps({ open: true });
    await flushPromises();

    const panel = wrapper.findComponent(FeaturePluginConfigPanel);
    expect(panel.exists()).toBe(true);
    expect(panel.props("canWrite")).toBe(false);
  });
});
