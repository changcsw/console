import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { ElMessage } from "element-plus";

const getGameChannelIapConfigApi = vi.fn();
const getPackageIapOverrideApi = vi.fn();
const putGameChannelIapConfigApi = vi.fn();
const putPackageIapOverrideApi = vi.fn();
const listMarketChannelsApi = vi.fn();
const listChannelPackagesApi = vi.fn();

vi.mock("@/api/modules/products", () => ({
  getGameChannelIapConfig: (...a: unknown[]) => getGameChannelIapConfigApi(...a),
  getPackageIapOverride: (...a: unknown[]) => getPackageIapOverrideApi(...a),
  putGameChannelIapConfig: (...a: unknown[]) => putGameChannelIapConfigApi(...a),
  putPackageIapOverride: (...a: unknown[]) => putPackageIapOverrideApi(...a)
}));

vi.mock("@/api/modules/channels", () => ({
  listMarketChannels: (...a: unknown[]) => listMarketChannelsApi(...a),
  listChannelPackages: (...a: unknown[]) => listChannelPackagesApi(...a)
}));

import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import IapConfigTab from "@/views/games/detail/IapConfigTab.vue";

const MARKET_CHANNELS = {
  items: [
    {
      gameChannelId: 1,
      displayKey: "100001:GLOBAL:google",
      gameId: "100001",
      market: "GLOBAL",
      channelId: "google",
      region: "overseas",
      compatible: true,
      hidden: false,
      configStatus: "valid",
      includedInSnapshot: true,
      includedInSync: true,
      includedInRuntimeConfig: true,
      copiedFromMarket: "",
      updatedAt: ""
    }
  ],
  page: 1,
  pageSize: 100,
  total: 1
};

function makeConfig(status: "empty" | "invalid" | "valid", message: string) {
  return {
    gameChannelId: 1,
    channelId: "google",
    template: {
      templateVersion: "v1",
      formSchema: [{ key: "apiKey", label: "API Key", component: "password", order: 1 }],
      secretFields: ["apiKey"],
      fileFields: [],
      validationRules: {}
    },
    config: {
      enabled: true,
      configStatus: status,
      configJson: { apiKey: "masked" },
      lastCheckAt: null,
      lastCheckMessage: message
    }
  };
}

interface IapVM {
  channelConfig: ReturnType<typeof makeConfig> | null;
  channelEnabled: boolean;
  channelDraftConfig: Record<string, unknown>;
  channelSecretInputs: Record<string, string>;
  channelJsonError: boolean;
  statusTone: (s: "empty" | "invalid" | "valid") => string;
  statusClass: (s: "empty" | "invalid" | "valid") => string;
  saveChannelConfig: () => Promise<void>;
}

async function mountTab(perms: string[] = ["product.read", "product.write"], status: "empty" | "invalid" | "valid" = "invalid", message = "缺少必填敏感字段或文件字段") {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  listMarketChannelsApi.mockResolvedValue(MARKET_CHANNELS);
  listChannelPackagesApi.mockResolvedValue([]);
  getGameChannelIapConfigApi.mockResolvedValue(makeConfig(status, message));
  const wrapper = mount(IapConfigTab, {
    props: { gameId: "100001" },
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return { wrapper, vm: wrapper.vm as unknown as IapVM };
}

describe("IapConfigTab · configStatus 行内告警", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(ElMessage, "warning").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "success").mockImplementation(() => ({}) as never);
    vi.spyOn(ElMessage, "error").mockImplementation(() => ({}) as never);
  });

  test("invalid：lastCheckMessage 渲染且不隐藏，使用告警样式", async () => {
    const { wrapper, vm } = await mountTab(["product.write"], "invalid", "缺少必填敏感字段或文件字段");
    const warn = wrapper.find("p.status-text--warning");
    expect(warn.exists()).toBe(true);
    expect(warn.text()).toBe("缺少必填敏感字段或文件字段");
    expect(vm.statusClass("invalid")).toContain("status-text--warning");
    expect(vm.statusTone("invalid")).toBe("warning");
  });

  test("valid：lastCheckMessage 仍展示，但不用告警样式", async () => {
    const { wrapper, vm } = await mountTab(["product.write"], "valid", "校验通过");
    const msg = wrapper.find("p.status-text");
    expect(msg.exists()).toBe(true);
    expect(msg.text()).toBe("校验通过");
    expect(wrapper.find("p.status-text--warning").exists()).toBe(false);
    expect(vm.statusTone("valid")).toBe("success");
  });

  test("empty：lastCheckMessage 为空则不渲染消息段", async () => {
    const { wrapper, vm } = await mountTab(["product.write"], "empty", "");
    expect(wrapper.find("p.status-text").exists()).toBe(false);
    expect(vm.statusTone("empty")).toBe("neutral");
  });

  test("保存渠道配置：密文留空不下发明文，重填则下发", async () => {
    const { vm } = await mountTab();
    putGameChannelIapConfigApi.mockResolvedValue({
      enabled: true,
      configStatus: "valid",
      configJson: { apiKey: "masked" },
      lastCheckAt: null,
      lastCheckMessage: "校验通过"
    });
    vm.channelDraftConfig = { plain: "x", apiKey: "masked" };
    vm.channelSecretInputs = { apiKey: "" };
    await vm.saveChannelConfig();
    await flushPromises();
    let payload = putGameChannelIapConfigApi.mock.calls[0][1];
    expect("apiKey" in payload.configJson).toBe(false);
    expect(payload.configJson.plain).toBe("x");

    vm.channelSecretInputs = { apiKey: "real-secret" };
    await vm.saveChannelConfig();
    await flushPromises();
    payload = putGameChannelIapConfigApi.mock.calls.at(-1)![1];
    expect(payload.configJson.apiKey).toBe("real-secret");
  });

  test("JSON 字段错误时阻止提交", async () => {
    const { vm } = await mountTab();
    vm.channelJsonError = true;
    await vm.saveChannelConfig();
    expect(ElMessage.warning).toHaveBeenCalledWith("请先修复 JSON 字段格式错误");
    expect(putGameChannelIapConfigApi).not.toHaveBeenCalled();
  });

  test("无 product.write 权限时保存按钮置灰且展示只读提示", async () => {
    const { wrapper } = await mountTab(["product.read"]);
    expect(wrapper.text()).toContain("当前账号仅有查看权限");
    const saveBtn = wrapper.find(".panel__actions button");
    expect(saveBtn.attributes("disabled")).toBe("disabled");
  });
});
