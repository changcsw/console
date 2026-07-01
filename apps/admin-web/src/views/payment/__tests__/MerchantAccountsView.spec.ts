import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { ElMessage } from "element-plus";
import permDirective from "@/directives/perm";
import { ApiError } from "@/api/http";
import { usePermissionStore } from "@/stores/permission";
import MerchantAccountsView from "@/views/payment/MerchantAccountsView.vue";
import TemplateConfigRenderer from "@/views/games/detail/components/TemplateConfigRenderer.vue";

const listProvidersApi = vi.fn();
const listBillingSubjectsApi = vi.fn();
const listMerchantAccountsApi = vi.fn();
const getProviderTemplateApi = vi.fn();
const createMerchantAccountApi = vi.fn();

vi.mock("@/api/modules/payment", () => ({
  listProviders: (...args: unknown[]) => listProvidersApi(...args),
  listBillingSubjects: (...args: unknown[]) => listBillingSubjectsApi(...args),
  listMerchantAccounts: (...args: unknown[]) => listMerchantAccountsApi(...args),
  getProviderTemplate: (...args: unknown[]) => getProviderTemplateApi(...args),
  createMerchantAccount: (...args: unknown[]) => createMerchantAccountApi(...args),
}));

const TEMPLATE = {
  templateVersion: "v1",
  formSchema: [
    { key: "appId", label: "App ID", component: "input", order: 1 },
    { key: "privateKey", label: "Private Key", component: "password", order: 2 },
    { key: "region", label: "区域", component: "select", order: 3, options: [{ label: "US", value: "us" }] },
    { key: "certFile", label: "证书文件", component: "file", order: 4 },
  ],
  secretFields: ["privateKey"],
  fileFields: [{ key: "certFile", accept: [".pem"], maxSizeKB: 128 }],
  validationRules: { appId: { minLen: 1 } },
};

async function mountView(perms: string[] = ["payment.read", "payment.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  listProvidersApi.mockResolvedValue({
    items: [{ providerId: "p1", providerName: "Provider A", providerKind: "gateway", enabled: true, sort: 1 }],
    page: 1,
    pageSize: 20,
    total: 1,
  });
  listBillingSubjectsApi.mockResolvedValue({
    items: [{ subjectId: "s1", subjectName: "主体 A", legalEntityName: "Entity A", enabled: true }],
    page: 1,
    pageSize: 20,
    total: 1,
  });
  listMerchantAccountsApi.mockResolvedValue({ items: [], page: 1, pageSize: 20, total: 0 });
  getProviderTemplateApi.mockResolvedValue(TEMPLATE);
  createMerchantAccountApi.mockResolvedValue({});

  const wrapper = mount(MerchantAccountsView, {
    global: { directives: { perm: permDirective } },
  });
  await flushPromises();
  return { wrapper, vm: wrapper.vm as unknown as Record<string, any> };
}

describe("MerchantAccountsView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "warning").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
  });

  test("选择 provider 后渲染模板四件套字段", async () => {
    const { vm, wrapper } = await mountView();
    vm.openCreate();
    await vm.onProviderChange("p1");
    await flushPromises();

    expect(getProviderTemplateApi).toHaveBeenCalledWith("p1");
    expect(wrapper.text()).toContain("App ID");
    expect(wrapper.text()).toContain("Private Key");
    expect(wrapper.text()).toContain("区域");
    expect(wrapper.text()).toContain("证书文件");
  });

  test("secret 留空表示不修改，文件上传写入 configJson 并提交", async () => {
    const { vm, wrapper } = await mountView();
    vm.openCreate();
    await vm.onProviderChange("p1");
    await flushPromises();

    const renderer = wrapper.getComponent(TemplateConfigRenderer);
    await (renderer.vm as any).onFileUpload("certFile", {
      file: { name: "cert.pem", size: 1024 },
      onError: vi.fn(),
      onSuccess: vi.fn(),
    });

    vm.form.merchantAccountId = "ma_001";
    vm.form.providerId = "p1";
    vm.form.subjectId = "s1";
    vm.form.merchantId = "m001";
    vm.form.merchantName = "Merchant 001";
    await vm.submit();
    await flushPromises();

    expect(createMerchantAccountApi).toHaveBeenCalledTimes(1);
    expect(createMerchantAccountApi).toHaveBeenCalledWith(
      expect.objectContaining({
        merchantAccountId: "ma_001",
        configJson: { certFile: "cert.pem" },
        secrets: {},
      })
    );
  });

  test("provider 无可用模板(404)时走降级路径，仍可提交基础字段", async () => {
    getProviderTemplateApi.mockRejectedValueOnce(new ApiError(404, "NOT_FOUND", "资源不存在"));
    const { vm, wrapper } = await mountView();
    vm.openCreate();
    await vm.onProviderChange("p1");
    await flushPromises();

    expect(wrapper.text()).toContain("该 provider 暂无可用模板");
    expect(wrapper.findComponent(TemplateConfigRenderer).exists()).toBe(false);

    vm.form.merchantAccountId = "ma_002";
    vm.form.providerId = "p1";
    vm.form.subjectId = "s1";
    vm.form.merchantId = "m002";
    vm.form.merchantName = "Merchant 002";
    await vm.submit();
    await flushPromises();
    expect(createMerchantAccountApi).toHaveBeenCalledTimes(1);
  });
});
