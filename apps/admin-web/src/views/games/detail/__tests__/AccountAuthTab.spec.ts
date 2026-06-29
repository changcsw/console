import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

// account-auth Tab 组件测试（对齐 03-testing §5.1 / 13-account-auth compact spec）：
// 覆盖模板四件套消费、secret/file/password/select/switch/json 渲染、
// config_status 三态、启用但 invalid 行内告警、密文恒脱敏 + 留空=不修改、
// locked 禁编、无 game.write 置灰、整体保存 PUT replace 回填。

const listAccountAuthTypesApi = vi.fn();
const listChannelAccountAuthTypesApi = vi.fn();
const listGameAccountAuthConfigsApi = vi.fn();
const replaceGameAccountAuthConfigsApi = vi.fn();
const listMarketChannelsApi = vi.fn();

vi.mock("@/api/modules/accountAuth", () => ({
  listAccountAuthTypes: (...args: unknown[]) => listAccountAuthTypesApi(...args),
  listChannelAccountAuthTypes: (...args: unknown[]) => listChannelAccountAuthTypesApi(...args),
  listGameAccountAuthConfigs: (...args: unknown[]) => listGameAccountAuthConfigsApi(...args),
  replaceGameAccountAuthConfigs: (...args: unknown[]) => replaceGameAccountAuthConfigsApi(...args)
}));

vi.mock("@/api/modules/channels", () => ({
  listMarketChannels: (...args: unknown[]) => listMarketChannelsApi(...args)
}));

const messageSuccess = vi.fn();
const messageError = vi.fn();
vi.mock("element-plus", async (importOriginal) => {
  const actual = await importOriginal<typeof import("element-plus")>();
  return {
    ...actual,
    ElMessage: {
      success: (...args: unknown[]) => messageSuccess(...args),
      error: (...args: unknown[]) => messageError(...args)
    }
  };
});

import { ApiError } from "@/api/http";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import AccountAuthTab from "@/views/games/detail/AccountAuthTab.vue";
import type {
  AccountAuthTypeItem,
  ChannelAccountAuthTypeItem,
  GameAccountAuthConfigItem
} from "@/api/modules/accountAuth";

// ---- 测试数据工厂 ----

function googleType(): AccountAuthTypeItem {
  return {
    authTypeId: "google",
    authTypeName: "Google 登录",
    enabled: true,
    sort: 40,
    template: {
      templateVersion: "v1",
      formSchema: [
        { key: "clientId", label: "Client ID", component: "input", required: true, order: 1 },
        { key: "redirectUri", label: "回调地址", component: "input", order: 2 },
        { key: "clientSecret", label: "Client Secret", component: "password", order: 3 },
        { key: "enableOneTap", label: "启用 One Tap", component: "switch", order: 4 },
        {
          key: "region",
          label: "区域",
          component: "select",
          order: 5,
          options: [
            { label: "美国", value: "us" },
            { label: "欧洲", value: "eu" }
          ]
        },
        { key: "extra", label: "扩展", component: "json", order: 6 },
        { key: "serviceAccount", label: "服务账号文件", component: "file", order: 7 }
      ],
      secretFields: ["clientSecret"],
      fileFields: [{ key: "serviceAccount", accept: ["application/json"], maxSizeKB: 64 }],
      validationRules: { clientId: { minLen: 1 } }
    }
  };
}

function phoneType(): AccountAuthTypeItem {
  return {
    authTypeId: "phone",
    authTypeName: "手机号登录",
    enabled: true,
    sort: 20,
    template: {
      templateVersion: "v1",
      formSchema: [],
      secretFields: [],
      fileFields: [],
      validationRules: {}
    }
  };
}

function googleConfig(over: Partial<GameAccountAuthConfigItem> = {}): GameAccountAuthConfigItem {
  return {
    authTypeId: "google",
    enabled: true,
    configJson: { clientId: "cid-123", clientSecret: "masked", region: "us" },
    configStatus: "valid",
    lastCheckAt: "2026-01-01T00:00:00Z",
    lastCheckMessage: "校验通过",
    ...over
  };
}

function phoneConfig(over: Partial<GameAccountAuthConfigItem> = {}): GameAccountAuthConfigItem {
  return {
    authTypeId: "phone",
    enabled: true,
    configJson: {},
    configStatus: "invalid",
    lastCheckAt: null,
    lastCheckMessage: "缺少必填字段",
    ...over
  };
}

interface SetupData {
  types?: AccountAuthTypeItem[];
  configs?: GameAccountAuthConfigItem[];
  channelAllowed?: ChannelAccountAuthTypeItem[];
}

function setupMocks(data: SetupData = {}) {
  const types = data.types ?? [googleType(), phoneType()];
  const configs = data.configs ?? [googleConfig(), phoneConfig()];
  const channelAllowed =
    data.channelAllowed ??
    ([
      { authTypeId: "google", defaultEnabled: true, locked: false },
      { authTypeId: "phone", defaultEnabled: false, locked: true }
    ] as ChannelAccountAuthTypeItem[]);

  listAccountAuthTypesApi.mockResolvedValue(types);
  listGameAccountAuthConfigsApi.mockResolvedValue(configs);
  listMarketChannelsApi.mockResolvedValue({
    items: [{ channelId: "ch1" }],
    page: 1,
    pageSize: 100,
    total: 1
  });
  listChannelAccountAuthTypesApi.mockResolvedValue(channelAllowed);
}

interface AuthRowVM {
  authTypeId: string;
  authTypeName: string;
  enabled: boolean;
  locked: boolean;
  defaultEnabled: boolean;
  configStatus: string;
  lastCheckMessage: string;
  draftConfig: Record<string, unknown>;
  secretInputs: Record<string, string>;
  jsonInputs: Record<string, string>;
  secretFields: string[];
  fileFields: string[];
}

interface AccountAuthVM {
  rows: AuthRowVM[];
  loadError: string;
  canWrite: boolean;
  loading: boolean;
  saving: boolean;
  load: () => Promise<void>;
  saveAll: () => Promise<void>;
  statusTone: (s: string) => string;
}

async function mountTab(perms: string[] = ["game.read", "game.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  const wrapper = mount(AccountAuthTab, {
    props: { gameId: "100001" },
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return { wrapper, vm: wrapper.vm as unknown as AccountAuthVM };
}

describe("AccountAuthTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setupMocks();
  });

  test("加载后按 sort 升序合并渠道允许集合，行来自游戏配置", async () => {
    const { vm } = await mountTab();
    expect(vm.rows.map((r) => r.authTypeId)).toEqual(["phone", "google"]);
    // 渠道允许标记被并入
    const phone = vm.rows.find((r) => r.authTypeId === "phone")!;
    const google = vm.rows.find((r) => r.authTypeId === "google")!;
    expect(phone.locked).toBe(true);
    expect(google.defaultEnabled).toBe(true);
    expect(google.locked).toBe(false);
  });

  test("模板四件套消费：渲染 input/password/switch/select/json/file 字段", async () => {
    const { wrapper } = await mountTab();
    const html = wrapper.html();
    // 标签来自 formSchema
    expect(html).toContain("Client ID");
    expect(html).toContain("Client Secret");
    expect(html).toContain("启用 One Tap");
    expect(html).toContain("区域");
    expect(html).toContain("扩展");
    expect(html).toContain("服务账号文件");
    // 控件类型可达
    expect(wrapper.find(".el-switch").exists()).toBe(true);
    expect(wrapper.find(".el-select").exists()).toBe(true);
    expect(wrapper.find("textarea").exists()).toBe(true); // json textarea
  });

  test("secret 字段恒显 masked 占位 + 留空则不修改提示", async () => {
    const { wrapper } = await mountTab();
    const html = wrapper.html();
    // 已存在密文 → 显示 masked 标记
    expect(wrapper.find(".secret-field__masked").exists()).toBe(true);
    expect(html).toContain("masked");
    // 已存密文时占位为「留空则不修改」
    expect(html).toContain("留空则不修改");
    // 列表内不出现任何明文密钥
    expect(html).not.toContain("PLAINTEXT");
  });

  test("config_status 三态展示 + 颜色 tone", async () => {
    setupMocks({
      configs: [
        googleConfig({ configStatus: "valid" }),
        phoneConfig({ authTypeId: "phone", configStatus: "empty", enabled: false })
      ]
    });
    const { wrapper, vm } = await mountTab();
    expect(vm.statusTone("empty")).toBe("neutral");
    expect(vm.statusTone("invalid")).toBe("warning");
    expect(vm.statusTone("valid")).toBe("success");
    const html = wrapper.html();
    expect(html).toContain("valid");
    expect(html).toContain("empty");
  });

  test("启用但 invalid 行内告警", async () => {
    setupMocks({
      configs: [phoneConfig({ enabled: true, configStatus: "invalid" })],
      channelAllowed: [{ authTypeId: "phone", defaultEnabled: false, locked: false }]
    });
    const { wrapper } = await mountTab();
    expect(wrapper.html()).toContain("已启用但配置未通过校验");
  });

  test("未启用或 valid 时不展示 invalid 告警", async () => {
    setupMocks({
      configs: [phoneConfig({ enabled: false, configStatus: "invalid" })],
      channelAllowed: [{ authTypeId: "phone", defaultEnabled: false, locked: false }]
    });
    const { wrapper } = await mountTab();
    expect(wrapper.html()).not.toContain("已启用但配置未通过校验");
  });

  test("locked 项卡片标记锁定且开关禁用", async () => {
    const { wrapper, vm } = await mountTab();
    const phone = vm.rows.find((r) => r.authTypeId === "phone")!;
    expect(phone.locked).toBe(true);
    const lockedCard = wrapper.find(".auth-card.is-locked");
    expect(lockedCard.exists()).toBe(true);
    // 锁定卡片内的开关禁用
    expect(lockedCard.find(".el-switch.is-disabled").exists()).toBe(true);
  });

  test("无 game.write 权限时保存按钮置灰且展示只读提示", async () => {
    const { wrapper, vm } = await mountTab(["game.read"]);
    expect(vm.canWrite).toBe(false);
    expect(wrapper.text()).toContain("仅有查看权限");
    const saveBtn = wrapper.find(".account-auth-tab__actions button.perm-disabled");
    expect(saveBtn.exists()).toBe(true);
    expect(saveBtn.attributes("disabled")).toBe("disabled");
  });

  test("整体保存：PUT replace 载荷含 authTypeId/enabled/configJson 并回填 status/message", async () => {
    replaceGameAccountAuthConfigsApi.mockResolvedValue([
      googleConfig({ configStatus: "valid", lastCheckMessage: "保存后校验通过" }),
      phoneConfig({ configStatus: "valid", lastCheckMessage: "" })
    ]);
    const { vm } = await mountTab();
    await vm.saveAll();
    await flushPromises();

    expect(replaceGameAccountAuthConfigsApi).toHaveBeenCalledTimes(1);
    const [gameId, payload] = replaceGameAccountAuthConfigsApi.mock.calls[0] as [
      string,
      { items: Array<{ authTypeId: string; enabled: boolean; configJson: Record<string, unknown> }> }
    ];
    expect(gameId).toBe("100001");
    expect(payload.items.map((i) => i.authTypeId).sort()).toEqual(["google", "phone"]);
    const googleItem = payload.items.find((i) => i.authTypeId === "google")!;
    expect(googleItem.enabled).toBe(true);
    expect(googleItem.configJson).toHaveProperty("clientId", "cid-123");

    // 回填
    const google = vm.rows.find((r) => r.authTypeId === "google")!;
    expect(google.lastCheckMessage).toBe("保存后校验通过");
    const phone = vm.rows.find((r) => r.authTypeId === "phone")!;
    expect(phone.configStatus).toBe("valid");
    expect(messageSuccess).toHaveBeenCalled();
  });

  test("密文留空=不修改：未重填则 configJson 不携带 secret 字段", async () => {
    setupMocks({
      configs: [googleConfig()],
      channelAllowed: [{ authTypeId: "google", defaultEnabled: true, locked: false }]
    });
    replaceGameAccountAuthConfigsApi.mockResolvedValue([googleConfig()]);
    const { vm } = await mountTab();
    await vm.saveAll();
    await flushPromises();

    const payload = replaceGameAccountAuthConfigsApi.mock.calls[0][1] as {
      items: Array<{ authTypeId: string; configJson: Record<string, unknown> }>
    };
    const googleItem = payload.items.find((i) => i.authTypeId === "google")!;
    // 留空 → 不下发 secret（后端保留原密文，绝不回传 masked 明文位）
    expect(googleItem.configJson).not.toHaveProperty("clientSecret");
  });

  test("密文重填：携带新值下发", async () => {
    setupMocks({
      configs: [googleConfig()],
      channelAllowed: [{ authTypeId: "google", defaultEnabled: true, locked: false }]
    });
    replaceGameAccountAuthConfigsApi.mockResolvedValue([googleConfig()]);
    const { vm } = await mountTab();
    const google = vm.rows.find((r) => r.authTypeId === "google")!;
    google.secretInputs.clientSecret = "new-secret-value";
    await vm.saveAll();
    await flushPromises();

    const payload = replaceGameAccountAuthConfigsApi.mock.calls[0][1] as {
      items: Array<{ authTypeId: string; configJson: Record<string, unknown> }>
    };
    const googleItem = payload.items.find((i) => i.authTypeId === "google")!;
    expect(googleItem.configJson).toHaveProperty("clientSecret", "new-secret-value");
  });

  test("json 字段非法时阻断保存并报错", async () => {
    setupMocks({
      configs: [googleConfig()],
      channelAllowed: [{ authTypeId: "google", defaultEnabled: true, locked: false }]
    });
    const { vm } = await mountTab();
    const google = vm.rows.find((r) => r.authTypeId === "google")!;
    google.jsonInputs.extra = "{ not valid json";
    await vm.saveAll();
    await flushPromises();

    expect(replaceGameAccountAuthConfigsApi).not.toHaveBeenCalled();
    expect(messageError).toHaveBeenCalled();
  });

  test("保存失败展示后端错误消息", async () => {
    replaceGameAccountAuthConfigsApi.mockRejectedValue(
      new ApiError(400, "VALIDATION_FAILED", "缺少必填敏感字段或文件字段: clientSecret")
    );
    const { vm } = await mountTab();
    await vm.saveAll();
    await flushPromises();
    expect(messageError).toHaveBeenCalledWith("缺少必填敏感字段或文件字段: clientSecret");
  });

  test("加载失败展示错误且行清空", async () => {
    listGameAccountAuthConfigsApi.mockRejectedValue(new ApiError(500, "INTERNAL", "服务异常"));
    const { vm } = await mountTab();
    expect(vm.loadError).toBe("服务异常");
    expect(vm.rows).toHaveLength(0);
  });

  test("无任何可配置认证方式时展示空态", async () => {
    setupMocks({ configs: [] });
    const { wrapper } = await mountTab();
    expect(wrapper.text()).toContain("暂无可配置认证方式");
  });
});
