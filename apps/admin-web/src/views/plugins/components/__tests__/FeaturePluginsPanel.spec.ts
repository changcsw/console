import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { ElMessage, ElMessageBox } from "element-plus";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import { ApiError } from "@/api/http";
import type { FeaturePlugin, FeaturePluginCategory } from "@/api/modules/featurePlugins";
import FeaturePluginsPanel from "@/views/plugins/components/FeaturePluginsPanel.vue";

const listFeaturePluginsApi = vi.fn();
const createFeaturePluginApi = vi.fn();
const updateFeaturePluginApi = vi.fn();
const deleteFeaturePluginApi = vi.fn();
const listFeaturePluginCategoriesApi = vi.fn();

vi.mock("@/api/modules/featurePlugins", async () => {
  const actual = await vi.importActual<typeof import("@/api/modules/featurePlugins")>(
    "@/api/modules/featurePlugins"
  );
  return {
    ...actual,
    listFeaturePlugins: (...args: unknown[]) => listFeaturePluginsApi(...args),
    createFeaturePlugin: (...args: unknown[]) => createFeaturePluginApi(...args),
    updateFeaturePlugin: (...args: unknown[]) => updateFeaturePluginApi(...args),
    deleteFeaturePlugin: (...args: unknown[]) => deleteFeaturePluginApi(...args),
    listFeaturePluginCategories: (...args: unknown[]) => listFeaturePluginCategoriesApi(...args)
  };
});

function plugin(overrides: Partial<FeaturePlugin> = {}): FeaturePlugin {
  return {
    pluginId: "huawei_push",
    pluginName: "华为推送",
    categoryId: 1,
    categoryCode: "push",
    categoryName: "推送类",
    region: "domestic",
    enabled: true,
    sort: 2,
    templateCount: 3,
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides
  };
}

function category(overrides: Partial<FeaturePluginCategory> = {}): FeaturePluginCategory {
  return {
    id: 1,
    categoryCode: "push",
    categoryName: "推送类",
    enabled: true,
    sort: 0,
    pluginCount: 1,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides
  };
}

type PanelVm = {
  rows: FeaturePlugin[];
  categories: FeaturePluginCategory[];
  keyword: string;
  filterCategoryId: number | "";
  filterRegion: string;
  filterEnabled: string;
  listError: string;
  editing: boolean;
  saving: boolean;
  formError: string;
  form: Record<string, unknown>;
  drawerVisible: boolean;
  reload: (page?: number) => Promise<void>;
  openCreate: () => void;
  openEdit: (row: FeaturePlugin) => void;
  submitForm: () => Promise<void>;
  removePlugin: (row: FeaturePlugin) => Promise<void>;
};

// EP 2.14 的 form-item 错误展示经 refDebounced(validateState, 100) 防抖：
// validate  settle 后 .el-form-item__error 的出现/消失都要再等 100ms+ 才反映到 DOM
const waitErrorDebounce = () => new Promise((resolve) => setTimeout(resolve, 150));

async function mountPanel(perms = ["feature_plugin.read", "feature_plugin.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  const wrapper = mount(FeaturePluginsPanel, {
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return wrapper;
}

describe("FeaturePluginsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listFeaturePluginsApi.mockResolvedValue({ items: [plugin()], page: 1, pageSize: 20, total: 1 });
    listFeaturePluginCategoriesApi.mockResolvedValue([category()]);
  });

  test("挂载即拉取第一页与分类字典并渲染插件主数据", async () => {
    const wrapper = await mountPanel();
    expect(listFeaturePluginsApi).toHaveBeenCalledWith(
      expect.objectContaining({ page: 1, pageSize: 20 })
    );
    expect(listFeaturePluginCategoriesApi).toHaveBeenCalled();
    const text = wrapper.text();
    expect(text).toContain("huawei_push");
    expect(text).toContain("华为推送");
    expect(text).toContain("推送类");
    // 国内/海外枚举以中文标签展示
    expect(text).toContain("国内");
  });

  test("筛选项转成后端查询参数，未选时不下发", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.keyword = "push";
    vm.filterCategoryId = 1;
    vm.filterRegion = "overseas";
    vm.filterEnabled = "false";
    await vm.reload(1);

    expect(listFeaturePluginsApi).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: "push",
      categoryId: 1,
      region: "overseas",
      enabled: false
    });

    vm.keyword = "";
    vm.filterCategoryId = "";
    vm.filterRegion = "";
    vm.filterEnabled = "";
    await vm.reload(1);
    expect(listFeaturePluginsApi).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: undefined,
      categoryId: undefined,
      region: undefined,
      enabled: undefined
    });
  });

  test("新建提交全字段", async () => {
    createFeaturePluginApi.mockResolvedValue(plugin({ pluginId: "google_fcm" }));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openCreate();
    expect(vm.editing).toBe(false);
    Object.assign(vm.form, {
      pluginId: "google_fcm",
      pluginName: "FCM 推送",
      categoryId: 1,
      region: "overseas",
      sort: 5,
      enabled: true
    });
    await vm.submitForm();

    expect(createFeaturePluginApi).toHaveBeenCalledWith({
      pluginId: "google_fcm",
      pluginName: "FCM 推送",
      categoryId: 1,
      region: "overseas",
      enabled: true,
      sort: 5
    });
    expect(vm.drawerVisible).toBe(false);
  });

  test("新建未选分类时不下发 categoryId", async () => {
    createFeaturePluginApi.mockResolvedValue(plugin({ categoryId: null, categoryCode: "", categoryName: "" }));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openCreate();
    Object.assign(vm.form, {
      pluginId: "apns_push",
      pluginName: "APNs",
      categoryId: "",
      region: "overseas",
      sort: 0,
      enabled: true
    });
    await vm.submitForm();

    const [payload] = createFeaturePluginApi.mock.calls[0] as [Record<string, unknown>];
    // 未选分类时为 undefined：JSON.stringify 会丢弃该键，后端不会收到 categoryId
    expect(payload.categoryId).toBeUndefined();
  });

  test("编辑回填行数据，只提交可变列，不下发 pluginId/region", async () => {
    updateFeaturePluginApi.mockResolvedValue(plugin({ pluginName: "华为推送服务" }));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openEdit(plugin());
    expect(vm.editing).toBe(true);
    // 回填：分类/排序/启用从行数据带入
    expect(vm.form.categoryId).toBe(1);
    expect(vm.form.sort).toBe(2);
    expect(vm.form.enabled).toBe(true);

    vm.form.pluginName = "华为推送服务";
    await vm.submitForm();

    const [pluginId, payload] = updateFeaturePluginApi.mock.calls[0] as [string, Record<string, unknown>];
    expect(pluginId).toBe("huawei_push");
    expect(payload).toEqual({
      pluginName: "华为推送服务",
      categoryId: 1,
      enabled: true,
      sort: 2
    });
    expect(payload).not.toHaveProperty("pluginId");
    expect(payload).not.toHaveProperty("region");
  });

  test("编辑时清空分类下发 categoryId: null（含 el-select clear 置 undefined 的真实路径）", async () => {
    updateFeaturePluginApi.mockResolvedValue(plugin({ categoryId: null }));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    // 路径一：el-select clearable 点击清空时 v-model 被置为 undefined（EP 默认 valueOnClear），
    // 若原样透传会被 JSON 序列化丢键，后端按「不修改」处理，清空操作静默失效。
    vm.openEdit(plugin());
    vm.form.categoryId = undefined;
    await vm.submitForm();
    const [, payloadUndef] = updateFeaturePluginApi.mock.calls[0] as [string, Record<string, unknown>];
    expect(payloadUndef.categoryId).toBeNull();

    // 路径二：行数据本身无分类（categoryId=null 回填为 ""）时同样下发 null
    vm.openEdit(plugin({ categoryId: null, categoryCode: "", categoryName: "" }));
    await vm.submitForm();
    const [, payloadEmpty] = updateFeaturePluginApi.mock.calls[1] as [string, Record<string, unknown>];
    expect(payloadEmpty.categoryId).toBeNull();
  });

  test("编辑抽屉里插件 ID 与国内/海外字段只读且给出不可改说明", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;
    vm.openEdit(plugin());
    await flushPromises();

    const hints = wrapper.findAll(".panel__hint").map((node) => node.text());
    expect(hints.join(" ")).toContain("插件 ID 是插件实例配置的引用键");
    expect(hints.join(" ")).toContain("国内/海外属性决定与市场");
  });

  test("VALIDATION_FAILED / CONFLICT 走抽屉内联报错且不关闭抽屉", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    createFeaturePluginApi.mockRejectedValueOnce(
      new ApiError(409, "CONFLICT", "插件 ID 已存在：huawei_push")
    );
    vm.openCreate();
    // 前端 rules 先跑：赋合法值确保通过即时校验，才轮得到后端返回的错误展示
    Object.assign(vm.form, {
      pluginId: "huawei_push",
      pluginName: "华为推送",
      region: "domestic",
      sort: 0,
      enabled: true
    });
    await vm.submitForm();
    expect(vm.formError).toBe("插件 ID 已存在：huawei_push");
    expect(vm.drawerVisible).toBe(true);

    createFeaturePluginApi.mockRejectedValueOnce(
      new ApiError(400, "VALIDATION_FAILED", "插件 ID 只能用小写字母/数字/下划线")
    );
    await vm.submitForm();
    expect(vm.formError).toContain("小写字母");
    expect(vm.drawerVisible).toBe(true);
  });

  test("写权限缺失时写操作按钮被 v-perm 置灰", async () => {
    const wrapper = await mountPanel(["feature_plugin.read"]);
    const buttons = wrapper.findAll("button").filter((btn) => btn.text().includes("新建插件"));
    expect(buttons).toHaveLength(1);
    expect(buttons[0].attributes("disabled")).toBeDefined();
  });

  test("操作列「参数模板」发出 view-templates 事件直达模板页", async () => {
    const wrapper = await mountPanel();
    const buttons = wrapper.findAll("button").filter((btn) => btn.text().includes("参数模板"));
    expect(buttons).toHaveLength(1);
    await buttons[0].trigger("click");
    expect(wrapper.emitted("view-templates")).toEqual([["huawei_push"]]);
  });

  test("删除成功后刷新列表，确认弹窗说清级联删除的模板版本数", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    deleteFeaturePluginApi.mockResolvedValue(undefined);
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    await vm.removePlugin(plugin());
    // 级联删除二次确认：templateCount>0 时点明版本数与不可恢复，确认键 danger 样式
    expect(ElMessageBox.confirm).toHaveBeenCalledWith(
      "确认删除插件「华为推送」？将同时删除其 3 个参数模板版本，删除后不可恢复。",
      "删除插件",
      expect.objectContaining({
        type: "warning",
        confirmButtonText: "确认删除",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger"
      })
    );
    expect(deleteFeaturePluginApi).toHaveBeenCalledWith("huawei_push");
    // 挂载 1 次 + 删除后刷新 1 次
    expect(listFeaturePluginsApi).toHaveBeenCalledTimes(2);
    expect(vm.listError).toBe("");
    // 通知父级失效参数模板页签缓存
    expect(wrapper.emitted("plugin-deleted")).toEqual([["huawei_push"]]);
  });

  test("无模板版本的插件删除确认文案不带模板数", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    deleteFeaturePluginApi.mockResolvedValue(undefined);
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    await vm.removePlugin(plugin({ templateCount: 0 }));
    expect(ElMessageBox.confirm).toHaveBeenCalledWith(
      "确认删除插件「华为推送」？删除后不可恢复。",
      "删除插件",
      expect.objectContaining({ type: "warning", confirmButtonText: "确认删除" })
    );
    expect(deleteFeaturePluginApi).toHaveBeenCalledWith("huawei_push");
  });

  test("删除被渠道引用的插件返回 409 时行内提示，不刷新也不发 plugin-deleted", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    const errorSpy = vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
    deleteFeaturePluginApi.mockRejectedValue(
      new ApiError(409, "CONFLICT", "该插件仍有关联数据（渠道绑定 2 条、渠道实例配置 3 条），请先删除关联数据")
    );
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    await vm.removePlugin(plugin());
    await flushPromises();

    expect(vm.listError).toBe("该插件仍有关联数据（渠道绑定 2 条、渠道实例配置 3 条），请先删除关联数据");
    // 409 行内提示与全局 toast 互斥：不弹 toast
    expect(errorSpy).not.toHaveBeenCalled();
    // 409 不触发刷新，行数据保持
    expect(listFeaturePluginsApi).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted("plugin-deleted")).toBeUndefined();
    const alert = wrapper.find(".panel__error[role=alert]");
    expect(alert.exists()).toBe(true);
    expect(alert.text()).toContain("渠道绑定 2 条");
  });

  test("删除失败为非 409 错误时走 toast 而非行内提示", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    const errorSpy = vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
    deleteFeaturePluginApi.mockRejectedValue(new ApiError(500, "INTERNAL", "数据库连接失败"));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    await vm.removePlugin(plugin());
    await flushPromises();

    // 非 CONFLICT 不行内化：保持 toast 通道，listError 不留内容
    expect(errorSpy).toHaveBeenCalledWith("数据库连接失败");
    expect(vm.listError).toBe("");
    expect(wrapper.find(".panel__error[role=alert]").exists()).toBe(false);
    expect(wrapper.emitted("plugin-deleted")).toBeUndefined();
  });

  test("再次发起删除时清空上次残留的 409 行内提示", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockResolvedValue("confirm" as never);
    deleteFeaturePluginApi.mockRejectedValueOnce(
      new ApiError(409, "CONFLICT", "该插件仍有关联数据（渠道绑定 2 条），请先删除关联数据")
    );
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    await vm.removePlugin(plugin());
    expect(vm.listError).toContain("渠道绑定 2 条");

    // 引用解除后再次删除成功：行内冲突提示须随新一次删除动作清空，不残留
    deleteFeaturePluginApi.mockResolvedValueOnce(undefined);
    await vm.removePlugin(plugin());
    expect(vm.listError).toBe("");
    expect(wrapper.find(".panel__error[role=alert]").exists()).toBe(false);
    expect(wrapper.emitted("plugin-deleted")).toEqual([["huawei_push"]]);
  });

  test("rules 即时校验：非法插件 ID / 空名称 / 纯空格名称 / 排序越界均拦截提交", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    const cases: Array<Record<string, unknown>> = [
      { pluginId: "1abc" }, // 数字开头
      { pluginId: "ABC_push" }, // 含大写
      { pluginId: "abc-def" }, // 含横线
      { pluginId: `${"a".repeat(65)}` }, // 超 64 字符
      { pluginName: "" }, // 空名称
      { pluginName: "   " } // 纯空格名称
    ];
    for (const patch of cases) {
      vm.openCreate();
      await flushPromises();
      Object.assign(vm.form, {
        pluginId: "good_id",
        pluginName: "合法名称",
        categoryId: "",
        region: "domestic",
        sort: 0,
        enabled: true,
        ...patch
      });
      await flushPromises();
      await vm.submitForm();
      await flushPromises();

      // 校验不过：不发请求、不置 saving、抽屉不关
      expect(createFeaturePluginApi, JSON.stringify(patch)).not.toHaveBeenCalled();
      expect(updateFeaturePluginApi, JSON.stringify(patch)).not.toHaveBeenCalled();
      expect(vm.saving, JSON.stringify(patch)).toBe(false);
      expect(vm.drawerVisible, JSON.stringify(patch)).toBe(true);
    }
    // 最后一次非法提交在对应表单项上渲染出行内校验错误
    await waitErrorDebounce();
    expect(wrapper.find(".el-form-item__error").exists()).toBe(true);
  });

  test("排序越界值被 el-input-number 钳回界内（组件层兜底，rules 的 0-9999 为双保险）", async () => {
    createFeaturePluginApi.mockResolvedValue(plugin());
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openCreate();
    await flushPromises();
    Object.assign(vm.form, {
      pluginId: "good_id",
      pluginName: "合法名称",
      categoryId: "",
      region: "domestic",
      enabled: true
    });
    // 直接赋越界值：el-input-number 的 :min/:max 将其钳回界内，越界值根本到不了 rules
    vm.form.sort = 10000;
    await flushPromises();
    expect(vm.form.sort).toBe(9999);

    await vm.submitForm();
    expect(createFeaturePluginApi).toHaveBeenCalledWith(expect.objectContaining({ sort: 9999 }));
  });

  test("编辑态回填值合法：rules 不误报，可正常保存", async () => {
    updateFeaturePluginApi.mockResolvedValue(plugin());
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    // 编辑态 pluginId/region 禁用不可改，回填值本身合法，rules 应直接通过
    vm.openEdit(plugin());
    await flushPromises();
    await vm.submitForm();
    await flushPromises();

    expect(updateFeaturePluginApi).toHaveBeenCalledWith(
      "huawei_push",
      expect.objectContaining({ pluginName: "华为推送" })
    );
    expect(vm.formError).toBe("");
    expect(wrapper.find(".el-form-item__error").exists()).toBe(false);
    expect(vm.drawerVisible).toBe(false);
  });

  test("rules 校验错误与后端 formError 两套提示各管各的不互相残留", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    // rules 拦截：只出 .el-form-item__error，formError（后端错误位）保持空
    vm.openCreate();
    await flushPromises();
    Object.assign(vm.form, {
      pluginId: "1abc",
      pluginName: "合法名称",
      categoryId: "",
      region: "domestic",
      sort: 0,
      enabled: true
    });
    await vm.submitForm();
    await flushPromises();
    expect(createFeaturePluginApi).not.toHaveBeenCalled();
    expect(vm.formError).toBe("");
    await waitErrorDebounce();
    expect(wrapper.find(".el-form-item__error").exists()).toBe(true);

    // 修正为合法输入后后端返回 CONFLICT：rules 错误清除，后端错误走 formError 行内
    createFeaturePluginApi.mockRejectedValueOnce(new ApiError(409, "CONFLICT", "插件 ID 已存在：good_id"));
    vm.form.pluginId = "good_id";
    await vm.submitForm();
    await flushPromises();
    expect(createFeaturePluginApi).toHaveBeenCalled();
    await waitErrorDebounce();
    expect(wrapper.find(".el-form-item__error").exists()).toBe(false);
    expect(vm.formError).toBe("插件 ID 已存在：good_id");
    expect(vm.drawerVisible).toBe(true);
  });

  test("取消删除时不调后端", async () => {
    vi.spyOn(ElMessageBox, "confirm").mockRejectedValue("cancel");
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    await vm.removePlugin(plugin());
    expect(deleteFeaturePluginApi).not.toHaveBeenCalled();
    expect(listFeaturePluginsApi).toHaveBeenCalledTimes(1);
  });
});
