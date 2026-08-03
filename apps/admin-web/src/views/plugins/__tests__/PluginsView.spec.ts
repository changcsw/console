import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import PluginsView from "@/views/plugins/PluginsView.vue";
import FeaturePluginsPanel from "@/views/plugins/components/FeaturePluginsPanel.vue";

const listFeaturePluginsApi = vi.fn();
const listFeaturePluginCategoriesApi = vi.fn();
const listFeaturePluginTemplatesApi = vi.fn();

vi.mock("@/api/modules/featurePlugins", async () => {
  const actual = await vi.importActual<typeof import("@/api/modules/featurePlugins")>(
    "@/api/modules/featurePlugins"
  );
  return {
    ...actual,
    listFeaturePlugins: (...args: unknown[]) => listFeaturePluginsApi(...args),
    listFeaturePluginCategories: (...args: unknown[]) => listFeaturePluginCategoriesApi(...args),
    listFeaturePluginTemplates: (...args: unknown[]) => listFeaturePluginTemplatesApi(...args)
  };
});

type ViewVm = {
  activeTab: string;
  templateFocus: { pluginId: string };
  templatesEpoch: number;
};

async function mountView() {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({
    roles: [],
    permissions: ["feature_plugin.read", "feature_plugin.write"]
  });
  const wrapper = mount(PluginsView, {
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return wrapper;
}

describe("PluginsView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listFeaturePluginsApi.mockResolvedValue({ items: [], page: 1, pageSize: 20, total: 0 });
    listFeaturePluginCategoriesApi.mockResolvedValue([]);
    listFeaturePluginTemplatesApi.mockResolvedValue({ items: [] });
  });

  test("插件面板发出 plugin-deleted 后递增 templatesEpoch 强制重建模板面板", async () => {
    const wrapper = await mountView();
    const vm = wrapper.vm as unknown as ViewVm;

    // 切到插件主数据页签，挂载插件面板
    vm.activeTab = "plugins";
    await flushPromises();
    const panel = wrapper.findComponent(FeaturePluginsPanel);
    expect(panel.exists()).toBe(true);

    expect(vm.templatesEpoch).toBe(0);
    panel.vm.$emit("plugin-deleted", "huawei_push");
    await flushPromises();
    // templatesEpoch 是模板面板的 :key：递增即销毁重建，失效其插件下拉与版本列表缓存
    expect(vm.templatesEpoch).toBe(1);

    panel.vm.$emit("plugin-deleted", "admob");
    await flushPromises();
    expect(vm.templatesEpoch).toBe(2);
  });

  test("焦点插件被删时清空 templateFocus，非焦点删除则保留", async () => {
    const wrapper = await mountView();
    const vm = wrapper.vm as unknown as ViewVm;

    vm.activeTab = "plugins";
    await flushPromises();
    const panel = wrapper.findComponent(FeaturePluginsPanel);

    // 焦点插件本身被级联删除：清空定位，避免模板面板重选一个已不存在的插件
    vm.templateFocus = { pluginId: "huawei_push" };
    panel.vm.$emit("plugin-deleted", "huawei_push");
    await flushPromises();
    expect(vm.templateFocus.pluginId).toBe("");

    // 删的不是焦点插件：保留当前定位，epoch 照常递增
    vm.templateFocus = { pluginId: "admob" };
    panel.vm.$emit("plugin-deleted", "huawei_push");
    await flushPromises();
    expect(vm.templateFocus.pluginId).toBe("admob");
    expect(vm.templatesEpoch).toBe(2);
  });
});
