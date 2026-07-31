import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import { ApiError } from "@/api/http";
import type { ChannelTemplate, PlatformChannel } from "@/api/modules/platformChannels";
import ChannelTemplatesPanel from "@/views/channels/components/platform/ChannelTemplatesPanel.vue";
import type { TemplateDraft } from "@/views/channels/components/platform/templateDraft";

const listPlatformChannelsApi = vi.fn();
const listChannelTemplatesApi = vi.fn();
const createChannelTemplateApi = vi.fn();
const updateChannelTemplateApi = vi.fn();

vi.mock("@/api/modules/platformChannels", async () => {
  const actual = await vi.importActual<typeof import("@/api/modules/platformChannels")>(
    "@/api/modules/platformChannels"
  );
  return {
    ...actual,
    listPlatformChannels: (...args: unknown[]) => listPlatformChannelsApi(...args),
    listChannelTemplates: (...args: unknown[]) => listChannelTemplatesApi(...args),
    createChannelTemplate: (...args: unknown[]) => createChannelTemplateApi(...args),
    updateChannelTemplate: (...args: unknown[]) => updateChannelTemplateApi(...args)
  };
});

function channel(overrides: Partial<PlatformChannel> = {}): PlatformChannel {
  return {
    channelId: "huawei_cn",
    channelName: "华为",
    channelType: "domestic",
    region: "CN",
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

function template(overrides: Partial<ChannelTemplate> = {}): ChannelTemplate {
  return {
    templateId: 1,
    kind: "login",
    channelId: "huawei_cn",
    templateVersion: "v1",
    formSchemaJson: [{ key: "appId", label: "App ID", component: "input", required: true, order: 10 }],
    secretFieldsJson: [],
    fileFieldsJson: [],
    validationRulesJson: {},
    enabled: true,
    effective: true,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T00:00:00Z",
    ...overrides
  };
}

type PanelVm = {
  rows: ChannelTemplate[];
  channels: PlatformChannel[];
  selectedChannelId: string;
  filterKind: string;
  editing: boolean;
  formError: string;
  form: { kind: string; templateVersion: string; enabled: boolean };
  draft: TemplateDraft;
  drawerVisible: boolean;
  reload: () => Promise<void>;
  openCreate: () => void;
  openEdit: (row: ChannelTemplate) => void;
  submitForm: () => Promise<void>;
};

async function mountPanel(
  perms = ["channel_template.read", "channel_template.write"],
  props: { focusChannel?: { channelId: string } } = {}
) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  const wrapper = mount(ChannelTemplatesPanel, {
    props,
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return wrapper;
}

describe("ChannelTemplatesPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listPlatformChannelsApi.mockResolvedValue({ items: [channel()], page: 1, pageSize: 100, total: 1 });
    listChannelTemplatesApi.mockResolvedValue([
      template({ templateId: 2, templateVersion: "v2", enabled: false, effective: false }),
      template()
    ]);
  });

  test("挂载后自动选中第一个渠道并拉取其模版", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;
    expect(vm.selectedChannelId).toBe("huawei_cn");
    // kind 未选时省略参数，后端返回两类
    expect(listChannelTemplatesApi).toHaveBeenCalledWith("huawei_cn", undefined);
    expect(vm.rows).toHaveLength(2);
  });

  test("生效版本与启用状态分别成列展示", async () => {
    const wrapper = await mountPanel();
    const text = wrapper.text();
    expect(text).toContain("v1");
    expect(text).toContain("v2");
    // v1 enabled+effective；v2 停用且未生效
    expect(text).toContain("生效中");
    expect(text).toContain("未生效");
    expect(text).toContain("已启用");
    expect(text).toContain("已停用");
    expect(text).toContain("渠道登录");
  });

  test("kind 切换下发查询参数", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;
    vm.filterKind = "iap";
    await vm.reload();
    expect(listChannelTemplatesApi).toHaveBeenLastCalledWith("huawei_cn", "iap");
  });

  test("新建提交 kind/版本/四件套，并以当前 kind 筛选值为默认种类", async () => {
    createChannelTemplateApi.mockResolvedValue(template({ templateId: 3, templateVersion: "v3" }));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.filterKind = "iap";
    vm.openCreate();
    expect(vm.form.kind).toBe("iap");

    vm.form.templateVersion = " v3 ";
    vm.draft.fields = [
      { key: "appId", label: "App ID", component: "input", required: true, order: 10, scope: "both" },
      { key: "appSecret", label: "密钥", component: "password", required: true, order: 20 }
    ];
    vm.draft.secretFields = ["appSecret"];
    vm.draft.rulesText = '{"appId":{"required":true}}';
    await vm.submitForm();

    const [channelId, payload] = createChannelTemplateApi.mock.calls[0] as [string, Record<string, unknown>];
    expect(channelId).toBe("huawei_cn");
    expect(payload).toMatchObject({
      kind: "iap",
      templateVersion: "v3",
      enabled: true,
      secretFieldsJson: ["appSecret"],
      validationRulesJson: { appId: { required: true } }
    });
    expect(payload.formSchemaJson).toHaveLength(2);
    expect(vm.drawerVisible).toBe(false);
  });

  test("编辑按 kind + templateId 提交，不下发 templateVersion", async () => {
    updateChannelTemplateApi.mockResolvedValue(template({ enabled: false }));
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openEdit(template({ templateId: 7, kind: "login", templateVersion: "v1" }));
    expect(vm.editing).toBe(true);
    expect(vm.form.templateVersion).toBe("v1");
    vm.form.enabled = false;
    await vm.submitForm();

    const [kind, templateId, payload] = updateChannelTemplateApi.mock.calls[0] as [
      string,
      number,
      Record<string, unknown>
    ];
    expect(kind).toBe("login");
    expect(templateId).toBe(7);
    expect(payload.enabled).toBe(false);
    expect(payload).not.toHaveProperty("templateVersion");
    expect(payload).not.toHaveProperty("kind");
  });

  test("四件套前置校验不通过时内联报错且不打接口", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openCreate();
    vm.form.templateVersion = "v3";
    // password 字段未登记敏感字段：后端会 400，前端先拦下
    vm.draft.fields = [{ key: "appSecret", label: "密钥", component: "password", required: true, order: 10 }];
    vm.draft.secretFields = [];
    await vm.submitForm();

    expect(vm.formError).toContain("必须登记为敏感字段");
    expect(createChannelTemplateApi).not.toHaveBeenCalled();
    expect(vm.drawerVisible).toBe(true);
  });

  test("版本号必填在创建时前置拦截", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;
    vm.openCreate();
    vm.draft.fields = [{ key: "appId", label: "App ID", component: "input", order: 10 }];
    vm.form.templateVersion = "  ";
    await vm.submitForm();
    expect(vm.formError).toBe("版本号必填");
    expect(createChannelTemplateApi).not.toHaveBeenCalled();
  });

  test("重复版本 409 走内联报错", async () => {
    createChannelTemplateApi.mockRejectedValueOnce(
      new ApiError(409, "CONFLICT", "该渠道下模版版本 v1 已存在")
    );
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;

    vm.openCreate();
    vm.form.templateVersion = "v1";
    vm.draft.fields = [{ key: "appId", label: "App ID", component: "input", order: 10 }];
    await vm.submitForm();

    expect(vm.formError).toBe("该渠道下模版版本 v1 已存在");
    expect(vm.drawerVisible).toBe(true);
  });

  test("写权限缺失时新建按钮被 v-perm 置灰", async () => {
    const wrapper = await mountPanel(["channel_template.read"]);
    const buttons = wrapper.findAll("button").filter((btn) => btn.text().includes("新建模版版本"));
    expect(buttons).toHaveLength(1);
    expect(buttons[0].attributes("disabled")).toBeDefined();
  });

  test("无渠道可选时给出空态且不拉模版", async () => {
    listPlatformChannelsApi.mockResolvedValue({ items: [], page: 1, pageSize: 100, total: 0 });
    const wrapper = await mountPanel();
    expect(listChannelTemplatesApi).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain("请选择渠道");
  });

  test("挂载时带 focusChannel 预设则优先选中该渠道而非列表首个", async () => {
    listPlatformChannelsApi.mockResolvedValue({
      items: [channel({ channelId: "google", channelName: "Google Play" }), channel()],
      page: 1,
      pageSize: 100,
      total: 2
    });
    const wrapper = await mountPanel(undefined, { focusChannel: { channelId: "huawei_cn" } });
    const vm = wrapper.vm as unknown as PanelVm;
    expect(vm.selectedChannelId).toBe("huawei_cn");
    expect(listChannelTemplatesApi).toHaveBeenCalledWith("huawei_cn", undefined);
  });

  test("focusChannel 换新对象即切换选中渠道并重拉模版（同一渠道重复定位也生效）", async () => {
    listPlatformChannelsApi.mockResolvedValue({
      items: [channel({ channelId: "google", channelName: "Google Play" }), channel()],
      page: 1,
      pageSize: 100,
      total: 2
    });
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;
    expect(vm.selectedChannelId).toBe("google");

    await wrapper.setProps({ focusChannel: { channelId: "huawei_cn" } });
    await flushPromises();
    expect(vm.selectedChannelId).toBe("huawei_cn");
    expect(listChannelTemplatesApi).toHaveBeenLastCalledWith("huawei_cn", undefined);

    // 手动改选其它渠道后，再点同一渠道的「渠道模版」（父级给新对象）仍会切回来
    vm.selectedChannelId = "google";
    await wrapper.setProps({ focusChannel: { channelId: "huawei_cn" } });
    await flushPromises();
    expect(vm.selectedChannelId).toBe("huawei_cn");
    expect(listChannelTemplatesApi).toHaveBeenLastCalledWith("huawei_cn", undefined);
  });

  test("focusChannel 指向不存在的渠道时保持现状不拉模版", async () => {
    const wrapper = await mountPanel();
    const vm = wrapper.vm as unknown as PanelVm;
    expect(vm.selectedChannelId).toBe("huawei_cn");
    listChannelTemplatesApi.mockClear();

    await wrapper.setProps({ focusChannel: { channelId: "ghost" } });
    await flushPromises();
    expect(vm.selectedChannelId).toBe("huawei_cn");
    expect(listChannelTemplatesApi).not.toHaveBeenCalled();
  });
});
