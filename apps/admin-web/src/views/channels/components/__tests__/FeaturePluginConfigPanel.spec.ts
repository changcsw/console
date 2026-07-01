import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { ElMessage } from "element-plus";
import { ApiError } from "@/api/http";
import permDirective from "@/directives/perm";
import FeaturePluginConfigPanel from "@/views/channels/components/FeaturePluginConfigPanel.vue";
import {
  featurePluginItem,
  filePluginItem,
  optionalPluginItem
} from "./fixtures/featurePlugin";
import type { GameChannelPluginItem } from "@/api/modules/channels";

const listGameChannelPluginsApi = vi.fn();
const upsertGameChannelPluginApi = vi.fn();
const patchGameChannelPluginApi = vi.fn();

vi.mock("@/api/modules/channels", () => ({
  listGameChannelPlugins: (...args: unknown[]) => listGameChannelPluginsApi(...args),
  upsertGameChannelPlugin: (...args: unknown[]) => upsertGameChannelPluginApi(...args),
  patchGameChannelPlugin: (...args: unknown[]) => patchGameChannelPluginApi(...args)
}));

function makeDetail() {
  return {
    gameChannelId: 101,
    displayKey: "100001:CN:huawei_cn",
    gameId: "100001",
    market: "CN",
    channelId: "huawei_cn",
    region: "domestic",
    compatible: true,
    hidden: false,
    configStatus: "invalid",
    includedInSnapshot: false,
    includedInSync: false,
    includedInRuntimeConfig: false,
    copiedFromMarket: "",
    updatedAt: "2026-01-01T00:00:00Z",
    enabled: true,
    remark: "",
    hiddenBy: "",
    hiddenAt: null,
    lastCheckAt: null,
    lastCheckMessage: "",
    createdAt: "2026-01-01T00:00:00Z"
  } as const;
}

interface PanelVM {
  items: GameChannelPluginItem[];
  draftOf: (item: GameChannelPluginItem) => { enabled: boolean; config: Record<string, unknown> };
  canEdit: (item: GameChannelPluginItem) => boolean;
  onEnabledChange: (item: GameChannelPluginItem, value: boolean) => void;
  beginEditSecret: (item: GameChannelPluginItem, key: string) => void;
  setSecretValue: (item: GameChannelPluginItem, key: string, value: string) => void;
  onFileUpload: (
    item: GameChannelPluginItem,
    key: string,
    options: { file: { name: string; size: number }; onSuccess: (v: unknown) => void; onError: (e: unknown) => void }
  ) => void;
  saveItem: (item: GameChannelPluginItem) => Promise<void>;
}

async function mountPanel(items: GameChannelPluginItem[], canWrite = true) {
  setActivePinia(createPinia());
  listGameChannelPluginsApi.mockResolvedValue(items);
  const wrapper = mount(FeaturePluginConfigPanel, {
    props: { gameChannelId: 101, detail: makeDetail(), canWrite },
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return { wrapper, vm: wrapper.vm as unknown as PanelVM };
}

describe("FeaturePluginConfigPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "warning").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
  });

  test("加载后展示必接未配置引导与 scope 提示", async () => {
    setActivePinia(createPinia());
    listGameChannelPluginsApi.mockResolvedValue([featurePluginItem()]);
    const wrapper = mount(FeaturePluginConfigPanel, {
      props: {
        gameChannelId: 101,
        detail: makeDetail(),
        canWrite: true
      },
      global: { directives: { perm: permDirective } }
    });
    await flushPromises();

    expect(wrapper.html()).toContain("必接插件未配置完成");
    expect(wrapper.html()).toContain("仅服务端，不下发客户端");
  });

  test("保存走 PATCH，保留未修改密文字段哨兵", async () => {
    setActivePinia(createPinia());
    listGameChannelPluginsApi.mockResolvedValue([featurePluginItem()]);
    patchGameChannelPluginApi.mockResolvedValue(featurePluginItem({ configStatus: "valid", includedInRuntimeConfig: true }));
    const wrapper = mount(FeaturePluginConfigPanel, {
      props: {
        gameChannelId: 101,
        detail: makeDetail(),
        canWrite: true
      },
      global: { directives: { perm: permDirective } }
    });
    await flushPromises();

    const vm = wrapper.vm as unknown as { saveItem: (item: ReturnType<typeof featurePluginItem>) => Promise<void> };
    await vm.saveItem(featurePluginItem());
    await flushPromises();

    expect(patchGameChannelPluginApi).toHaveBeenCalledTimes(1);
    const payload = patchGameChannelPluginApi.mock.calls[0][1] as { config: Record<string, unknown> };
    expect(payload.config.appKey).toBe("******");
    expect(ElMessage.success).toHaveBeenCalled();
  });

  test("徽标渲染：必接/国内海外/进入最终配置/config_status/锁定", async () => {
    const { wrapper } = await mountPanel([
      featurePluginItem(),
      optionalPluginItem({ locked: true })
    ]);
    const html = wrapper.html();
    expect(html).toContain("必接");
    expect(html).toContain("国内");
    expect(html).toContain("海外");
    expect(html).toContain("锁定");
    expect(html).toContain("进入最终配置");
    expect(html).toContain("未进入最终配置");
    expect(html).toContain("配置无效");
    expect(html).toContain("配置有效");
  });

  test("必接引导清单列出未配置完成的插件名", async () => {
    const { wrapper } = await mountPanel([featurePluginItem({ configStatus: "invalid" })]);
    expect(wrapper.html()).toContain("必接插件未配置完成");
    expect(wrapper.html()).toContain("防沉迷(anti_addiction)");
  });

  test("required+selectable=false 强制选中，onEnabledChange 无法取消", async () => {
    const { vm } = await mountPanel([featurePluginItem({ enabled: false, required: true, selectable: false })]);
    const item = vm.items[0];
    expect(vm.draftOf(item).enabled).toBe(true);
    vm.onEnabledChange(item, false);
    expect(vm.draftOf(item).enabled).toBe(true);
  });

  test("locked=true 禁用编辑并展示锁定提示", async () => {
    const { wrapper, vm } = await mountPanel([featurePluginItem({ locked: true })]);
    expect(vm.canEdit(vm.items[0])).toBe(false);
    expect(wrapper.html()).toContain("该插件已锁定，当前实例不可编辑");
  });

  test("无 plugin.write（canWrite=false）置灰编辑", async () => {
    const { vm } = await mountPanel([optionalPluginItem({ locked: false })], false);
    expect(vm.canEdit(vm.items[0])).toBe(false);
  });

  test("空列表展示无可接入插件占位", async () => {
    const { wrapper } = await mountPanel([]);
    expect(wrapper.html()).toContain("暂无可接入插件");
  });

  test("加载失败展示错误提示且列表清空", async () => {
    setActivePinia(createPinia());
    listGameChannelPluginsApi.mockRejectedValue(new ApiError(500, "INTERNAL", "boom"));
    const wrapper = mount(FeaturePluginConfigPanel, {
      props: { gameChannelId: 101, detail: makeDetail(), canWrite: true },
      global: { directives: { perm: permDirective } }
    });
    await flushPromises();
    expect(ElMessage.error).toHaveBeenCalledWith("boom");
    expect((wrapper.vm as unknown as PanelVM).items).toHaveLength(0);
  });

  test("file 上传：超限/类型不符拦截，合法文件回填配置", async () => {
    const { vm } = await mountPanel([filePluginItem()]);
    const item = vm.items[0];

    const tooBig = { file: { name: "a.pdf", size: 200 * 1024 }, onSuccess: vi.fn(), onError: vi.fn() };
    vm.onFileUpload(item, "license", tooBig);
    expect(tooBig.onError).toHaveBeenCalled();
    expect(tooBig.onSuccess).not.toHaveBeenCalled();

    const wrongType = { file: { name: "a.txt", size: 10 * 1024 }, onSuccess: vi.fn(), onError: vi.fn() };
    vm.onFileUpload(item, "license", wrongType);
    expect(wrongType.onError).toHaveBeenCalled();

    const ok = { file: { name: "license.pdf", size: 10 * 1024 }, onSuccess: vi.fn(), onError: vi.fn() };
    vm.onFileUpload(item, "license", ok);
    expect(ok.onSuccess).toHaveBeenCalledWith({ fileName: "license.pdf" });
    expect(vm.draftOf(item).config.license).toBe("license.pdf");
  });

  test("密文重填后下发明文新值", async () => {
    const { vm } = await mountPanel([featurePluginItem()]);
    patchGameChannelPluginApi.mockResolvedValue(featurePluginItem());
    const item = vm.items[0];
    vm.beginEditSecret(item, "appKey");
    vm.setSecretValue(item, "appKey", "new-secret");
    await vm.saveItem(item);
    await flushPromises();
    const payload = patchGameChannelPluginApi.mock.calls[0][1] as { config: Record<string, unknown> };
    expect(payload.config.appKey).toBe("new-secret");
  });

  test("必填字段缺失阻止保存", async () => {
    const { vm } = await mountPanel([featurePluginItem({ id: 501, configJson: {} })]);
    await vm.saveItem(vm.items[0]);
    expect(ElMessage.warning).toHaveBeenCalled();
    expect(patchGameChannelPluginApi).not.toHaveBeenCalled();
    expect(upsertGameChannelPluginApi).not.toHaveBeenCalled();
  });

  test("新实例（id=0）保存走 POST upsert", async () => {
    const { vm } = await mountPanel([optionalPluginItem({ id: 0, configJson: { callback: "https://example.com/push" } })]);
    upsertGameChannelPluginApi.mockResolvedValue(optionalPluginItem({ id: 88 }));
    await vm.saveItem(vm.items[0]);
    await flushPromises();
    expect(upsertGameChannelPluginApi).toHaveBeenCalledTimes(1);
    const [gameChannelId, payload] = upsertGameChannelPluginApi.mock.calls[0] as [number, { pluginId: string }];
    expect(gameChannelId).toBe(101);
    expect(payload.pluginId).toBe("push");
  });
});
