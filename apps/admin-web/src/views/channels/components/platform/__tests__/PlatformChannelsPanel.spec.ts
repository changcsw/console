import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import { ApiError } from "@/api/http";
import type { PlatformChannel } from "@/api/modules/platformChannels";
import PlatformChannelsPanel from "@/views/channels/components/platform/PlatformChannelsPanel.vue";

const listPlatformChannelsApi = vi.fn();
const createPlatformChannelApi = vi.fn();
const updatePlatformChannelApi = vi.fn();

vi.mock("@/api/modules/platformChannels", async () => {
  const actual = await vi.importActual<typeof import("@/api/modules/platformChannels")>(
    "@/api/modules/platformChannels"
  );
  return {
    ...actual,
    listPlatformChannels: (...args: unknown[]) => listPlatformChannelsApi(...args),
    createPlatformChannel: (...args: unknown[]) => createPlatformChannelApi(...args),
    updatePlatformChannel: (...args: unknown[]) => updatePlatformChannelApi(...args)
  };
});

function channel(overrides: Partial<PlatformChannel> = {}): PlatformChannel {
  return {
    channelId: "huawei_cn",
    channelName: "华为",
    channelType: "oem",
    region: "domestic",
    enabled: true,
    sort: 2,
    loginMode: "channel_only",
    paymentMode: "channel_only",
    loginLocked: false,
    paymentLocked: false,
    loginTemplateCount: 2,
    iapTemplateCount: 0,
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides
  };
}

type PanelVm = {
  rows: PlatformChannel[];
  keyword: string;
  filterRegion: string;
  filterChannelType: string;
  filterEnabled: string;
  editing: boolean;
  formError: string;
  form: Record<string, unknown>;
  drawerVisible: boolean;
  reload: (page?: number) => Promise<void>;
  openCreate: () => void;
  openEdit: (row: PlatformChannel) => void;
  submitForm: () => Promise<void>;
};

async function mountPanel(perms = ["platform_channel.read", "platform_channel.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  const wrapper = mount(PlatformChannelsPanel, {
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return wrapper;
}

describe("PlatformChannelsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listPlatformChannelsApi.mockResolvedValue({ items: [channel()], page: 1, pageSize: 20, total: 1 });
  });

  test("挂载即拉取第一页并渲染渠道主数据与模版计数", async () => {
    const wrapper = await mountPanel();
    expect(listPlatformChannelsApi).toHaveBeenCalledWith(
      expect.objectContaining({ page: 1, pageSize: 20 })
    );
    const text = wrapper.text();
    expect(text).toContain("huawei_cn");
    expect(text).toContain("华为");
    // 枚举以中文标签展示，region 复用渠道页的 regionLabel
    expect(text).toContain("手机厂商");
    expect(text).toContain("国内");
    expect(text).toContain("仅渠道登录");
    expect(text).toContain("登录 2 / IAP 0");
  });

  test("筛选项转成后端查询参数，未选时不下发", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.keyword = "huawei";
    vm.filterRegion = "domestic";
    vm.filterChannelType = "oem";
    vm.filterEnabled = "false";
    await vm.reload(1);

    expect(listPlatformChannelsApi).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: "huawei",
      region: "domestic",
      channelType: "oem",
      enabled: false
    });

    vm.keyword = "";
    vm.filterRegion = "";
    vm.filterChannelType = "";
    vm.filterEnabled = "";
    await vm.reload(1);
    expect(listPlatformChannelsApi).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: undefined,
      region: undefined,
      channelType: undefined,
      enabled: undefined
    });
  });

  test("新建提交全字段", async () => {
    createPlatformChannelApi.mockResolvedValue(channel({ channelId: "vivo_cn" }));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openCreate();
    expect(vm.editing).toBe(false);
    Object.assign(vm.form, {
      channelId: "vivo_cn",
      channelName: "vivo",
      channelType: "oem",
      region: "domestic",
      loginMode: "account_system",
      paymentMode: "hybrid",
      sort: 5,
      loginLocked: false,
      paymentLocked: true,
      enabled: true
    });
    await vm.submitForm();

    expect(createPlatformChannelApi).toHaveBeenCalledWith({
      channelId: "vivo_cn",
      channelName: "vivo",
      channelType: "oem",
      region: "domestic",
      enabled: true,
      sort: 5,
      loginMode: "account_system",
      paymentMode: "hybrid",
      loginLocked: false,
      paymentLocked: true
    });
    expect(vm.drawerVisible).toBe(false);
  });

  test("编辑只提交可变列，不下发 channelId/channelType/region", async () => {
    updatePlatformChannelApi.mockResolvedValue(channel({ channelName: "华为应用市场" }));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openEdit(channel());
    expect(vm.editing).toBe(true);
    vm.form.channelName = "华为应用市场";
    await vm.submitForm();

    const [channelId, payload] = updatePlatformChannelApi.mock.calls[0] as [string, Record<string, unknown>];
    expect(channelId).toBe("huawei_cn");
    expect(payload).toEqual({
      channelName: "华为应用市场",
      enabled: true,
      sort: 2,
      loginMode: "channel_only",
      paymentMode: "channel_only",
      loginLocked: false,
      paymentLocked: false
    });
    expect(payload).not.toHaveProperty("channelId");
    expect(payload).not.toHaveProperty("channelType");
    expect(payload).not.toHaveProperty("region");
  });

  test("编辑抽屉里身份与 region 字段只读且给出不可改说明", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;
    vm.openEdit(channel());
    await flushPromises();

    const hints = wrapper.findAll(".panel__hint").map((node) => node.text());
    expect(hints.join(" ")).toContain("渠道 ID 是渠道实例的引用键");
    expect(hints.join(" ")).toContain("region 决定与 market 的兼容性");
  });

  test("VALIDATION_FAILED / CONFLICT 走抽屉内联报错且不关闭抽屉", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    createPlatformChannelApi.mockRejectedValueOnce(
      new ApiError(409, "CONFLICT", "渠道 ID 已存在：google")
    );
    vm.openCreate();
    await vm.submitForm();
    expect(vm.formError).toBe("渠道 ID 已存在：google");
    expect(vm.drawerVisible).toBe(true);

    createPlatformChannelApi.mockRejectedValueOnce(
      new ApiError(400, "VALIDATION_FAILED", "渠道 ID 只能用小写字母/数字/下划线")
    );
    await vm.submitForm();
    expect(vm.formError).toContain("小写字母");
    expect(vm.drawerVisible).toBe(true);
  });

  test("写权限缺失时写操作按钮被 v-perm 置灰", async () => {
    const wrapper = await mountPanel(["platform_channel.read"]);
    const buttons = wrapper.findAll("button").filter((btn) => btn.text().includes("新建渠道"));
    expect(buttons).toHaveLength(1);
    expect(buttons[0].attributes("disabled")).toBeDefined();
  });
});
