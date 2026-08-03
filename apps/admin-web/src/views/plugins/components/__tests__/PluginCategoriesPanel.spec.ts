import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { ElMessageBox } from "element-plus";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import { ApiError } from "@/api/http";
import type { FeaturePluginCategory } from "@/api/modules/featurePlugins";
import PluginCategoriesPanel from "@/views/plugins/components/PluginCategoriesPanel.vue";

const listFeaturePluginCategoriesApi = vi.fn();
const createFeaturePluginCategoryApi = vi.fn();
const updateFeaturePluginCategoryApi = vi.fn();
const deleteFeaturePluginCategoryApi = vi.fn();

vi.mock("@/api/modules/featurePlugins", async () => {
  const actual = await vi.importActual<typeof import("@/api/modules/featurePlugins")>(
    "@/api/modules/featurePlugins"
  );
  return {
    ...actual,
    listFeaturePluginCategories: (...args: unknown[]) => listFeaturePluginCategoriesApi(...args),
    createFeaturePluginCategory: (...args: unknown[]) => createFeaturePluginCategoryApi(...args),
    updateFeaturePluginCategory: (...args: unknown[]) => updateFeaturePluginCategoryApi(...args),
    deleteFeaturePluginCategory: (...args: unknown[]) => deleteFeaturePluginCategoryApi(...args)
  };
});

function category(overrides: Partial<FeaturePluginCategory> = {}): FeaturePluginCategory {
  return {
    id: 1,
    categoryCode: "login",
    categoryName: "登录类",
    enabled: true,
    sort: 2,
    pluginCount: 3,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides
  };
}

type PanelVm = {
  rows: FeaturePluginCategory[];
  filterEnabled: string;
  listError: string;
  editing: boolean;
  formError: string;
  form: Record<string, unknown>;
  drawerVisible: boolean;
  reload: () => Promise<void>;
  openCreate: () => void;
  openEdit: (row: FeaturePluginCategory) => void;
  submitForm: () => Promise<void>;
  removeCategory: (row: FeaturePluginCategory) => Promise<void>;
};

async function mountPanel(perms = ["feature_plugin.read", "feature_plugin.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  const wrapper = mount(PluginCategoriesPanel, {
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return wrapper;
}

describe("PluginCategoriesPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listFeaturePluginCategoriesApi.mockResolvedValue([category()]);
  });

  test("挂载即拉取分类字典并渲染分类主数据", async () => {
    const wrapper = await mountPanel();
    expect(listFeaturePluginCategoriesApi).toHaveBeenCalledWith({ enabled: undefined });
    const text = wrapper.text();
    expect(text).toContain("login");
    expect(text).toContain("登录类");
    // 插件数直接展示计数
    expect(text).toContain("3");
  });

  test("启用状态筛选转成后端查询参数", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.filterEnabled = "false";
    await vm.reload();
    expect(listFeaturePluginCategoriesApi).toHaveBeenLastCalledWith({ enabled: false });

    vm.filterEnabled = "";
    await vm.reload();
    expect(listFeaturePluginCategoriesApi).toHaveBeenLastCalledWith({ enabled: undefined });
  });

  test("新建提交全字段", async () => {
    createFeaturePluginCategoryApi.mockResolvedValue(category({ id: 2, categoryCode: "pay" }));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openCreate();
    expect(vm.editing).toBe(false);
    Object.assign(vm.form, {
      categoryCode: "pay",
      categoryName: "支付类",
      sort: 5,
      enabled: true
    });
    await vm.submitForm();

    expect(createFeaturePluginCategoryApi).toHaveBeenCalledWith({
      categoryCode: "pay",
      categoryName: "支付类",
      enabled: true,
      sort: 5
    });
    expect(vm.drawerVisible).toBe(false);
  });

  test("编辑回填行数据，只提交可变列，不下发 categoryCode", async () => {
    updateFeaturePluginCategoryApi.mockResolvedValue(category({ categoryName: "登录插件" }));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openEdit(category());
    expect(vm.editing).toBe(true);
    // 回填：编码/排序/启用从行数据带入
    expect(vm.form.categoryCode).toBe("login");
    expect(vm.form.sort).toBe(2);
    expect(vm.form.enabled).toBe(true);

    vm.form.categoryName = "登录插件";
    await vm.submitForm();

    const [id, payload] = updateFeaturePluginCategoryApi.mock.calls[0] as [number, Record<string, unknown>];
    expect(id).toBe(1);
    expect(payload).toEqual({
      categoryName: "登录插件",
      enabled: true,
      sort: 2
    });
    expect(payload).not.toHaveProperty("categoryCode");
  });

  test("编辑抽屉里分类编码只读且给出不可改说明", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;
    vm.openEdit(category());
    await flushPromises();

    const hints = wrapper.findAll(".panel__hint").map((node) => node.text());
    expect(hints.join(" ")).toContain("分类编码是分类的引用键");
  });

  test("VALIDATION_FAILED / CONFLICT 走抽屉内联报错且不关闭抽屉", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    createFeaturePluginCategoryApi.mockRejectedValueOnce(
      new ApiError(409, "CONFLICT", "分类编码已存在：login")
    );
    vm.openCreate();
    await vm.submitForm();
    expect(vm.formError).toBe("分类编码已存在：login");
    expect(vm.drawerVisible).toBe(true);

    createFeaturePluginCategoryApi.mockRejectedValueOnce(
      new ApiError(400, "VALIDATION_FAILED", "分类编码只能用小写字母/数字/下划线")
    );
    await vm.submitForm();
    expect(vm.formError).toContain("小写字母");
    expect(vm.drawerVisible).toBe(true);
  });

  test("删除成功后刷新列表", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    deleteFeaturePluginCategoryApi.mockResolvedValue(undefined);
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    await vm.removeCategory(category());
    expect(ElMessageBox.confirm).toHaveBeenCalled();
    expect(deleteFeaturePluginCategoryApi).toHaveBeenCalledWith(1);
    // 挂载 1 次 + 删除后刷新 1 次
    expect(listFeaturePluginCategoriesApi).toHaveBeenCalledTimes(2);
    expect(vm.listError).toBe("");
  });

  test("删除被 409 拒绝时行内提示「该分类下仍有插件，无法删除」", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    deleteFeaturePluginCategoryApi.mockRejectedValue(
      new ApiError(409, "CONFLICT", "该分类下仍有插件，无法删除")
    );
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    await vm.removeCategory(category({ pluginCount: 2 }));
    await flushPromises();

    expect(vm.listError).toBe("该分类下仍有插件，无法删除");
    // 409 不触发刷新，行数据保持
    expect(listFeaturePluginCategoriesApi).toHaveBeenCalledTimes(1);
    expect(wrapper.find(".panel__error").exists()).toBe(true);
    expect(wrapper.find(".panel__error").text()).toContain("该分类下仍有插件，无法删除");
  });

  test("取消删除时不调后端", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockRejectedValue("cancel");
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    await vm.removeCategory(category());
    expect(deleteFeaturePluginCategoryApi).not.toHaveBeenCalled();
  });

  test("写权限缺失时写操作按钮被 v-perm 置灰", async () => {
    const wrapper = await mountPanel(["feature_plugin.read"]);
    const buttons = wrapper.findAll("button").filter((btn) => btn.text().includes("新建分类"));
    expect(buttons).toHaveLength(1);
    expect(buttons[0].attributes("disabled")).toBeDefined();
  });
});
