import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { ElMessage } from "element-plus";

const getPackageProductsApi = vi.fn();
const getPackageIapOverrideApi = vi.fn();
const putPackageProductsApi = vi.fn();
const putPackageIapOverrideApi = vi.fn();
const listChannelPackagePluginsApi = vi.fn();
const upsertChannelPackagePluginApi = vi.fn();

vi.mock("@/api/modules/products", () => ({
  getPackageProducts: (...a: unknown[]) => getPackageProductsApi(...a),
  getPackageIapOverride: (...a: unknown[]) => getPackageIapOverrideApi(...a),
  putPackageProducts: (...a: unknown[]) => putPackageProductsApi(...a),
  putPackageIapOverride: (...a: unknown[]) => putPackageIapOverrideApi(...a)
}));

vi.mock("@/api/modules/channels", () => ({
  listChannelPackagePlugins: (...a: unknown[]) => listChannelPackagePluginsApi(...a),
  upsertChannelPackagePlugin: (...a: unknown[]) => upsertChannelPackagePluginApi(...a)
}));

import { ApiError } from "@/api/http";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import ChannelPackageDetailDrawer from "@/views/channels/components/ChannelPackageDetailDrawer.vue";
import TemplateConfigRenderer from "@/views/games/detail/components/TemplateConfigRenderer.vue";
import type { ChannelPackage, ChannelPackagePluginItem } from "@/api/modules/channels";
import type { PackageProductItem } from "@/api/modules/products";
import { channelPackagePluginItem } from "./fixtures/featurePlugin";

function makePkg(): ChannelPackage {
  return {
    packageId: 9001,
    gameChannelId: 1,
    packageCode: "PKG-GP",
    packageName: "Google Play 包",
    marketCode: "GLOBAL",
    bundleId: "com.demo",
    inheritChannelConfig: true,
    enabled: true,
    overrideJson: {},
    createdAt: "",
    updatedAt: ""
  };
}

function makeProduct(over: Partial<PackageProductItem> = {}): PackageProductItem {
  return {
    productId: "com.demo.gold",
    productName: "金币礼包",
    enabled: true,
    base: { productId: "com.demo.gold", priceId: "price_gold_1", baseAmountMinor: 499, baseCurrency: "USD" },
    productIdMode: "default",
    productIdOverride: "",
    priceIdMode: "default",
    priceIdOverride: "",
    effective: { productId: "com.demo.gold", priceId: "price_gold_1" },
    ...over
  };
}

const OVERRIDE_RESP = {
  packageId: 9001,
  packageCode: "PKG-GP",
  channelId: "google",
  template: { templateVersion: "v1", formSchema: [], secretFields: ["apiKey"], fileFields: [], validationRules: {} },
  baseConfig: { enabled: true, configStatus: "valid" as const, configJson: {}, lastCheckAt: null, lastCheckMessage: "基线 OK" },
  override: { enabled: false, configStatus: "empty" as const, configJson: {}, lastCheckAt: null, lastCheckMessage: "" }
};

interface DrawerVM {
  productRows: PackageProductItem[];
  overrideEnabled: boolean;
  overrideDraftConfig: Record<string, unknown>;
  overrideSecretInputs: Record<string, string>;
  packagePlugins: ChannelPackagePluginItem[];
  effectiveProductId: (row: PackageProductItem) => string;
  effectivePriceId: (row: PackageProductItem) => string;
  onModeChange: (row: PackageProductItem, target: "product" | "price") => void;
  saveMappings: () => Promise<void>;
  saveIapOverride: () => Promise<void>;
  pluginCanEdit: (item: ChannelPackagePluginItem) => boolean;
  updatePluginSecrets: (item: ChannelPackagePluginItem, value: Record<string, string>) => void;
  savePackagePlugin: (item: ChannelPackagePluginItem) => Promise<void>;
}

async function mountDrawer(
  perms: string[] = ["product.read", "product.write"],
  rows: PackageProductItem[] = [makeProduct()],
  plugins: ChannelPackagePluginItem[] = []
) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  getPackageProductsApi.mockResolvedValue(rows);
  getPackageIapOverrideApi.mockResolvedValue(JSON.parse(JSON.stringify(OVERRIDE_RESP)));
  listChannelPackagePluginsApi.mockResolvedValue(plugins);
  // 抽屉用非 immediate watch（open false→true 才加载），故先关后开触发 loadData。
  const wrapper = mount(ChannelPackageDetailDrawer, {
    props: { open: false, pkg: makePkg() },
    global: { directives: { perm: permDirective } }
  });
  await wrapper.setProps({ open: true });
  await flushPromises();
  return { wrapper, vm: wrapper.vm as unknown as DrawerVM };
}

describe("ChannelPackageDetailDrawer · 商品映射两组覆盖", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "warning").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
  });

  test("加载后 default 模式清空 override 残值", async () => {
    const { vm } = await mountDrawer(["product.read", "product.write"], [
      makeProduct({ productIdMode: "default", productIdOverride: "残值", priceIdMode: "default", priceIdOverride: "残值" })
    ]);
    expect(vm.productRows[0].productIdOverride).toBe("");
    expect(vm.productRows[0].priceIdOverride).toBe("");
  });

  test("mode=default 切换时清空对应 override（onModeChange）", async () => {
    const { vm } = await mountDrawer();
    const row = vm.productRows[0];
    row.productIdMode = "override";
    row.productIdOverride = "ov-prod";
    row.productIdMode = "default";
    vm.onModeChange(row, "product");
    expect(row.productIdOverride).toBe("");
  });

  test("effective 实时显示：override 非空取覆盖值，否则回退基准", async () => {
    const { vm } = await mountDrawer();
    const row = vm.productRows[0];
    // 默认回退基准
    expect(vm.effectiveProductId(row)).toBe("com.demo.gold");
    expect(vm.effectivePriceId(row)).toBe("price_gold_1");
    // 仅覆盖 productId
    row.productIdMode = "override";
    row.productIdOverride = "com.demo.OVERRIDE";
    expect(vm.effectiveProductId(row)).toBe("com.demo.OVERRIDE");
    // priceId 不联动，仍回退基准
    expect(vm.effectivePriceId(row)).toBe("price_gold_1");
  });

  test("两组互不联动：覆盖 priceId 不改变 effective.productId", async () => {
    const { vm } = await mountDrawer();
    const row = vm.productRows[0];
    row.priceIdMode = "override";
    row.priceIdOverride = "price_OVERRIDE";
    expect(vm.effectivePriceId(row)).toBe("price_OVERRIDE");
    expect(vm.effectiveProductId(row)).toBe("com.demo.gold");
    expect(row.productIdMode).toBe("default");
  });

  test("override 必填空值阻止提交（productId 组）", async () => {
    const { vm } = await mountDrawer();
    vm.productRows[0].productIdMode = "override";
    vm.productRows[0].productIdOverride = "   ";
    await vm.saveMappings();
    expect(ElMessage.warning).toHaveBeenCalledWith(expect.stringContaining("IAP 商品 ID 覆盖值不能为空"));
    expect(putPackageProductsApi).not.toHaveBeenCalled();
  });

  test("override 必填空值阻止提交（priceId 组）", async () => {
    const { vm } = await mountDrawer();
    vm.productRows[0].priceIdMode = "override";
    vm.productRows[0].priceIdOverride = "";
    await vm.saveMappings();
    expect(ElMessage.warning).toHaveBeenCalledWith(expect.stringContaining("收银台价格档覆盖值不能为空"));
    expect(putPackageProductsApi).not.toHaveBeenCalled();
  });

  test("override 超长阻止提交（productId>128 / priceId>64）", async () => {
    const { vm } = await mountDrawer();
    vm.productRows[0].productIdMode = "override";
    vm.productRows[0].productIdOverride = "x".repeat(129);
    await vm.saveMappings();
    expect(ElMessage.warning).toHaveBeenCalledWith(expect.stringContaining("不能超过 128 字符"));
    expect(putPackageProductsApi).not.toHaveBeenCalled();
  });

  test("保存映射：default 组强制下发空 override，override 组下发 trim 值", async () => {
    const { vm } = await mountDrawer();
    const row = vm.productRows[0];
    row.productIdMode = "override";
    row.productIdOverride = "  com.demo.OV  ";
    row.priceIdMode = "default";
    row.priceIdOverride = "脏值"; // default 组应被强制清空
    putPackageProductsApi.mockResolvedValue([makeProduct()]);
    await vm.saveMappings();
    await flushPromises();
    const [packageId, payload] = putPackageProductsApi.mock.calls[0];
    expect(packageId).toBe(9001);
    expect(payload.items[0].productIdMode).toBe("override");
    expect(payload.items[0].productIdOverride).toBe("com.demo.OV");
    expect(payload.items[0].priceIdMode).toBe("default");
    expect(payload.items[0].priceIdOverride).toBe("");
  });

  test("保存 IAP 覆盖：密文留空不下发明文，仅重填值下发", async () => {
    const { vm } = await mountDrawer();
    vm.overrideEnabled = true;
    vm.overrideDraftConfig = { plain: "v" };
    vm.overrideSecretInputs = { apiKey: "" }; // 留空
    putPackageIapOverrideApi.mockResolvedValue({ ...OVERRIDE_RESP.override, enabled: true });
    await vm.saveIapOverride();
    await flushPromises();
    const [, payload] = putPackageIapOverrideApi.mock.calls[0];
    expect("apiKey" in payload.configJson).toBe(false);
    expect(payload.configJson.plain).toBe("v");

    // 重填后下发
    vm.overrideSecretInputs = { apiKey: "new-secret" };
    await vm.saveIapOverride();
    await flushPromises();
    const lastCall = putPackageIapOverrideApi.mock.calls.at(-1)!;
    expect(lastCall[1].configJson.apiKey).toBe("new-secret");
  });

  test("加载失败展示错误提示", async () => {
    setActivePinia(createPinia());
    usePermissionStore().setFromUser({ roles: [], permissions: ["product.write"] });
    getPackageProductsApi.mockRejectedValue(new ApiError(500, "INTERNAL", "boom"));
    getPackageIapOverrideApi.mockResolvedValue(JSON.parse(JSON.stringify(OVERRIDE_RESP)));
    const wrapper = mount(ChannelPackageDetailDrawer, {
      props: { open: false, pkg: makePkg() },
      global: { directives: { perm: permDirective } }
    });
    await wrapper.setProps({ open: true });
    await flushPromises();
    expect(ElMessage.error).toHaveBeenCalledWith("boom");
  });
});

describe("ChannelPackageDetailDrawer · 功能插件继承/覆盖", () => {
  const PLUGIN_PERMS = ["product.read", "product.write", "plugin.write"];

  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "warning").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
  });

  // IAP 覆盖区也固定挂载一个 TemplateConfigRenderer，用 formSchema 长度区分插件渲染器。
  function pluginRenderers(wrapper: Awaited<ReturnType<typeof mountDrawer>>["wrapper"]) {
    return wrapper
      .findAllComponents(TemplateConfigRenderer)
      .filter((r) => (r.props("template").formSchema?.length ?? 0) === 2);
  }

  test("inherit=true 时不渲染自定义模板，保存下发空 config 且继承标记为真", async () => {
    const { wrapper, vm } = await mountDrawer(PLUGIN_PERMS, [makeProduct()], [channelPackagePluginItem()]);
    // 继承态：插件模板渲染器不挂载（IAP 覆盖区的渲染器除外）
    expect(pluginRenderers(wrapper)).toHaveLength(0);
    expect(wrapper.html()).toContain("继承渠道插件");

    upsertChannelPackagePluginApi.mockResolvedValue(channelPackagePluginItem());
    await vm.savePackagePlugin(vm.packagePlugins[0]);
    await flushPromises();
    const [packageId, payload] = upsertChannelPackagePluginApi.mock.calls[0] as [
      number,
      { inheritChannelConfig: boolean; config: Record<string, unknown> }
    ];
    expect(packageId).toBe(9001);
    expect(payload.inheritChannelConfig).toBe(true);
    expect(payload.config).toEqual({});
  });

  test("自定义覆盖：模板渲染器消费四件套并渲染 scope=server 提示", async () => {
    const { wrapper } = await mountDrawer(PLUGIN_PERMS, [makeProduct()], [
      channelPackagePluginItem({ inheritChannelConfig: false })
    ]);
    const renderers = pluginRenderers(wrapper);
    expect(renderers).toHaveLength(1);
    expect(renderers[0].props("template").formSchema).toHaveLength(2);
    expect(renderers[0].props("template").secretFields).toContain("apiKey");
    expect(wrapper.html()).toContain("仅服务端，不下发客户端");
  });

  test("自定义覆盖保存：密文留空不下发，重填下发明文", async () => {
    const { vm } = await mountDrawer(PLUGIN_PERMS, [makeProduct()], [
      channelPackagePluginItem({ inheritChannelConfig: false })
    ]);
    const plugin = vm.packagePlugins[0];

    upsertChannelPackagePluginApi.mockResolvedValue(channelPackagePluginItem({ inheritChannelConfig: false }));
    vm.updatePluginSecrets(plugin, { apiKey: "" });
    await vm.savePackagePlugin(plugin);
    await flushPromises();
    const firstConfig = (upsertChannelPackagePluginApi.mock.calls[0][1] as { config: Record<string, unknown> }).config;
    expect("apiKey" in firstConfig).toBe(false);
    expect(firstConfig.endpoint).toBe("https://example.com/aa");

    vm.updatePluginSecrets(plugin, { apiKey: "new-key" });
    await vm.savePackagePlugin(plugin);
    await flushPromises();
    const lastConfig = (upsertChannelPackagePluginApi.mock.calls.at(-1)![1] as { config: Record<string, unknown> }).config;
    expect(lastConfig.apiKey).toBe("new-key");
  });

  test("locked 插件展示锁定提示且不可编辑", async () => {
    const { wrapper, vm } = await mountDrawer(PLUGIN_PERMS, [makeProduct()], [
      channelPackagePluginItem({ locked: true })
    ]);
    expect(vm.pluginCanEdit(vm.packagePlugins[0])).toBe(false);
    expect(wrapper.html()).toContain("该插件已锁定，渠道包不可编辑");
  });

  test("无 plugin.write 权限插件覆盖置灰", async () => {
    const { vm } = await mountDrawer(["product.read", "product.write"], [makeProduct()], [
      channelPackagePluginItem()
    ]);
    expect(vm.pluginCanEdit(vm.packagePlugins[0])).toBe(false);
  });
});
