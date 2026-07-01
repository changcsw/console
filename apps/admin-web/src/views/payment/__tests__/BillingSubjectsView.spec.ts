import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { ElMessage } from "element-plus";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import BillingSubjectsView from "@/views/payment/BillingSubjectsView.vue";

const listBillingSubjectsApi = vi.fn();
const createBillingSubjectApi = vi.fn();

vi.mock("@/api/modules/payment", () => ({
  listBillingSubjects: (...args: unknown[]) => listBillingSubjectsApi(...args),
  createBillingSubject: (...args: unknown[]) => createBillingSubjectApi(...args),
}));

async function mountView() {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: ["payment.read", "payment.write"] });
  listBillingSubjectsApi.mockResolvedValue({
    items: [{ subjectId: "sub_a", subjectName: "主体A", legalEntityName: "Entity A", enabled: true }],
    page: 1,
    pageSize: 20,
    total: 1,
  });
  createBillingSubjectApi.mockResolvedValue({});

  const wrapper = mount(BillingSubjectsView, {
    global: { directives: { perm: permDirective } },
  });
  await flushPromises();
  return { wrapper, vm: wrapper.vm as unknown as Record<string, any> };
}

describe("BillingSubjectsView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "warning").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
  });

  test("subjectId 非法时阻止提交", async () => {
    const { vm } = await mountView();
    vm.drawerOpen = true;
    vm.form.subjectId = "Bad-ID";
    vm.form.subjectName = "主体";
    vm.form.legalEntityName = "Entity";
    await vm.submit();
    expect(createBillingSubjectApi).not.toHaveBeenCalled();
    expect(ElMessage.warning).toHaveBeenCalledWith("subjectId 需为 1-64 位小写字母/数字/下划线");
  });

  test("合法字段提交后调用创建接口", async () => {
    const { vm } = await mountView();
    vm.drawerOpen = true;
    vm.form.subjectId = "subject_1";
    vm.form.subjectName = "主体1";
    vm.form.legalEntityName = "Entity One";
    vm.form.enabled = true;
    await vm.submit();
    await flushPromises();

    expect(createBillingSubjectApi).toHaveBeenCalledWith({
      subjectId: "subject_1",
      subjectName: "主体1",
      legalEntityName: "Entity One",
      enabled: true,
    });
  });
});
