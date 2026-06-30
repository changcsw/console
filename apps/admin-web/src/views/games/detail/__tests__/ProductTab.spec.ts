import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { ElMessage } from "element-plus";

const listProductsApi = vi.fn();
const createProductApi = vi.fn();
const updateProductApi = vi.fn();

vi.mock("@/api/modules/products", () => ({
  listProducts: (...args: unknown[]) => listProductsApi(...args),
  createProduct: (...args: unknown[]) => createProductApi(...args),
  updateProduct: (...args: unknown[]) => updateProductApi(...args)
}));

// dictionary store 走 @/api/http.request 拉 currency-specs；mock 成功返回，保留 ApiError 真实类。
const requestApi = vi.fn();
vi.mock("@/api/http", async () => {
  const actual = await vi.importActual<typeof import("@/api/http")>("@/api/http");
  return { ...actual, request: (...args: unknown[]) => requestApi(...args) };
});

import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import ProductTab from "@/views/games/detail/ProductTab.vue";
import type { ProductItem } from "@/api/modules/products";

const CURRENCY_SPECS = {
  items: [
    { currencyCode: "USD", currencyName: "US Dollar", decimalPlaces: 2, minAmountMinor: 1, roundingMode: "half_up", enabled: true },
    { currencyCode: "JPY", currencyName: "Japanese Yen", decimalPlaces: 0, minAmountMinor: 1, roundingMode: "half_up", enabled: true }
  ]
};

function makeRow(over: Partial<ProductItem> = {}): ProductItem {
  return {
    id: 1,
    env: "sandbox",
    gameId: "100001",
    productId: "com.demo.gold",
    productName: "金币礼包",
    baseAmountMinor: 499,
    baseCurrency: "USD",
    baseAmountDisplay: "4.99",
    priceId: "price_gold_1",
    enabled: true,
    createdAt: "",
    updatedAt: "2026-01-02T03:04:05Z",
    ...over
  };
}

interface ProductVM {
  form: {
    productId: string;
    productName: string;
    baseCurrency: string;
    baseAmount: string;
    priceId: string;
    enabled: boolean;
  };
  editingProductId: string;
  drawerOpen: boolean;
  minorPreview: string;
  openCreate: () => void;
  openEdit: (row: ProductItem) => void;
  submitProduct: () => Promise<void>;
  previewMinorAmount: () => void;
}

async function mountTab(perms: string[] = ["product.read", "product.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  requestApi.mockResolvedValue(CURRENCY_SPECS);
  listProductsApi.mockResolvedValue({ items: [makeRow()], page: 1, pageSize: 20, total: 1 });
  const wrapper = mount(ProductTab, {
    props: { gameId: "100001" },
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return { wrapper, vm: wrapper.vm as unknown as ProductVM };
}

function fillValid(vm: ProductVM) {
  vm.openCreate();
  vm.form.productId = "com.demo.gem";
  vm.form.productName = "宝石";
  vm.form.baseCurrency = "USD";
  vm.form.baseAmount = "1.99";
  vm.form.priceId = "price_gem_1";
}

describe("ProductTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "warning").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
  });

  test("挂载即拉取币种字典与商品列表", async () => {
    await mountTab();
    expect(requestApi).toHaveBeenCalledWith("/api/admin/system/currency-specs");
    expect(listProductsApi).toHaveBeenCalledTimes(1);
  });

  test("productId 必填：为空阻止提交", async () => {
    const { vm } = await mountTab();
    fillValid(vm);
    vm.form.productId = "  ";
    await vm.submitProduct();
    expect(ElMessage.warning).toHaveBeenCalledWith("请填写 IAP 商品 ID");
    expect(createProductApi).not.toHaveBeenCalled();
  });

  test("productId 长度上限 128：超长阻止提交（与 priceId 独立校验）", async () => {
    const { vm } = await mountTab();
    fillValid(vm);
    vm.form.productId = "x".repeat(129);
    vm.form.priceId = "price_ok"; // priceId 合法，仍因 productId 超长被拦
    await vm.submitProduct();
    expect(ElMessage.warning).toHaveBeenCalledWith("IAP 商品 ID 不能超过 128 字符");
    expect(createProductApi).not.toHaveBeenCalled();
  });

  test("priceId 必填：为空阻止提交（productId 合法）", async () => {
    const { vm } = await mountTab();
    fillValid(vm);
    vm.form.priceId = "   ";
    await vm.submitProduct();
    expect(ElMessage.warning).toHaveBeenCalledWith("请填写收银台价格档");
    expect(createProductApi).not.toHaveBeenCalled();
  });

  test("priceId 长度上限 64：超长阻止提交（productId 合法）", async () => {
    const { vm } = await mountTab();
    fillValid(vm);
    vm.form.priceId = "p".repeat(65);
    await vm.submitProduct();
    expect(ElMessage.warning).toHaveBeenCalledWith("收银台价格档不能超过 64 字符");
    expect(createProductApi).not.toHaveBeenCalled();
  });

  test("priceId 允许 128 长度的 productId（两维独立，priceId 自身仍受 64 限制）", async () => {
    const { vm } = await mountTab();
    fillValid(vm);
    vm.form.productId = "x".repeat(128); // 恰好 128，合法
    vm.form.priceId = "p".repeat(64); // 恰好 64，合法
    createProductApi.mockResolvedValue(makeRow());
    await vm.submitProduct();
    await flushPromises();
    expect(createProductApi).toHaveBeenCalledTimes(1);
    const [, payload] = createProductApi.mock.calls[0];
    expect(payload.productId).toHaveLength(128);
    expect(payload.priceId).toHaveLength(64);
  });

  test("合法表单创建：下发 trim 后的 productId/priceId 且互不混填", async () => {
    const { vm } = await mountTab();
    fillValid(vm);
    vm.form.productId = "  com.demo.gem  ";
    vm.form.priceId = "  price_gem_1  ";
    createProductApi.mockResolvedValue(makeRow());
    await vm.submitProduct();
    await flushPromises();
    const [gameId, payload] = createProductApi.mock.calls[0];
    expect(gameId).toBe("100001");
    expect(payload.productId).toBe("com.demo.gem");
    expect(payload.priceId).toBe("price_gem_1");
  });

  test("编辑模式：openEdit 预填且走 updateProduct（productId 身份键不变）", async () => {
    const { vm } = await mountTab();
    vm.openEdit(makeRow());
    expect(vm.editingProductId).toBe("com.demo.gold");
    expect(vm.form.productId).toBe("com.demo.gold");
    updateProductApi.mockResolvedValue(makeRow());
    await vm.submitProduct();
    await flushPromises();
    expect(updateProductApi).toHaveBeenCalledTimes(1);
    expect(createProductApi).not.toHaveBeenCalled();
    const [productId, gameId] = updateProductApi.mock.calls[0];
    expect(productId).toBe("com.demo.gold");
    expect(gameId).toBe("100001");
  });

  test("金额预览按币种精度与舍入换算 minor", async () => {
    const { vm } = await mountTab();
    vm.openCreate();
    vm.form.baseCurrency = "USD";
    vm.form.baseAmount = "4.999";
    vm.previewMinorAmount();
    expect(vm.minorPreview).toContain("500 minor");
  });

  test("无 product.write 权限时新建按钮置灰禁用", async () => {
    const { wrapper } = await mountTab(["product.read"]);
    const createBtn = wrapper.find(".product-tab__actions button.el-button--primary");
    expect(createBtn.attributes("disabled")).toBe("disabled");
    expect(createBtn.classes()).toContain("perm-disabled");
  });
});
