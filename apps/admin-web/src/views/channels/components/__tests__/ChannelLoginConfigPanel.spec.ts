import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { nextTick } from "vue";

// ChannelLoginConfigPanel 组件测试（对齐 03-testing §5.1 与 14-channel-login spec.compact §前端要点）：
// 覆盖统一模板渲染器四件套（order/component/group/required）、密文脱敏与哨兵 "******" 提交语义、
// config_status 三色、enabled=true 但 status!=valid 告警条、复制创建 invalid 提示、
// validationRules 即时校验、channel.write 权限置灰、保存回填与 VALIDATION_FAILED 二次 GET 回显。

const getLoginConfigApi = vi.fn();
const putLoginConfigApi = vi.fn();

vi.mock("@/api/modules/channels", () => ({
  getLoginConfig: (...args: unknown[]) => getLoginConfigApi(...args),
  putLoginConfig: (...args: unknown[]) => putLoginConfigApi(...args)
}));

const messageSuccess = vi.fn();
const messageError = vi.fn();
const messageWarning = vi.fn();
vi.mock("element-plus", async (importOriginal) => {
  const actual = await importOriginal<typeof import("element-plus")>();
  return {
    ...actual,
    ElMessage: {
      success: (...args: unknown[]) => messageSuccess(...args),
      error: (...args: unknown[]) => messageError(...args),
      warning: (...args: unknown[]) => messageWarning(...args)
    }
  };
});

import { ApiError } from "@/api/http";
import permDirective from "@/directives/perm";
import ChannelLoginConfigPanel from "@/views/channels/components/ChannelLoginConfigPanel.vue";
import type { ChannelLoginConfigResponse } from "@/api/modules/channels";
import {
  emptyLoginConfig,
  loginConfigResponse,
  marketChannelDetail
} from "./fixtures/channelLogin";

interface PanelVM {
  model: ChannelLoginConfigResponse | null;
  enabled: boolean;
  draftConfig: Record<string, unknown>;
  jsonInputs: Record<string, string>;
  secretStates: Record<string, { editing: boolean; value: string; hadStored: boolean }>;
  sortedFields: Array<{ key: string }>;
  groupedFields: Array<{ name: string; fields: Array<{ key: string }> }>;
  fieldErrors: Record<string, string>;
  statusTone: (s: string) => string;
  statusLabel: (s: string) => string;
  secretInputValue: (key: string) => string;
  beginEditSecret: (key: string) => void;
  setSecretValue: (key: string, value: string) => void;
  setDraftValue: (key: string, value: unknown) => void;
  save: () => Promise<void>;
  load: () => Promise<void>;
}

async function mountPanel(opts: { response?: ChannelLoginConfigResponse; canWrite?: boolean } = {}) {
  setActivePinia(createPinia());
  const response = opts.response ?? loginConfigResponse();
  getLoginConfigApi.mockResolvedValue(response);
  const wrapper = mount(ChannelLoginConfigPanel, {
    props: {
      gameChannelId: 101,
      detail: marketChannelDetail(),
      canWrite: opts.canWrite ?? true
    },
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return { wrapper, vm: wrapper.vm as unknown as PanelVM };
}

describe("ChannelLoginConfigPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test("四件套渲染：按 order 升序排序、按 group 分组、component 渲染、required 标必填", async () => {
    const { wrapper, vm } = await mountPanel();

    // order 升序：appId(1) region(2) appSecret(3) timeout(4) enableLog(5) extra(6) cert(7)
    expect(vm.sortedFields.map((f) => f.key)).toEqual([
      "appId",
      "region",
      "appSecret",
      "timeout",
      "enableLog",
      "extra",
      "cert"
    ]);

    // group 分组渲染顺序与组名
    expect(vm.groupedFields.map((g) => g.name)).toEqual(["基础", "密钥", "高级"]);

    const html = wrapper.html();
    // form_schema 标签渲染
    expect(html).toContain("App ID");
    expect(html).toContain("区域");
    expect(html).toContain("App Secret");
    expect(html).toContain("证书文件");
    // component 控件可达
    expect(wrapper.find(".el-switch").exists()).toBe(true); // switch
    expect(wrapper.find(".el-select").exists()).toBe(true); // select
    expect(wrapper.find(".el-input-number").exists()).toBe(true); // number
    expect(wrapper.find("textarea").exists()).toBe(true); // json textarea
    expect(wrapper.find(".el-upload").exists()).toBe(true); // file 上传控件
    // required 标必填（el-form-item is-required）
    expect(wrapper.find(".el-form-item.is-required").exists()).toBe(true);
  });

  test("密文脱敏：已存密文初始显示 ****** 占位、未修改提交哨兵 ******", async () => {
    const { vm } = await mountPanel();

    expect(vm.secretStates.appSecret.hadStored).toBe(true);
    expect(vm.secretStates.appSecret.editing).toBe(false);
    expect(vm.secretInputValue("appSecret")).toBe("******");

    putLoginConfigApi.mockResolvedValue(loginConfigResponse());
    await vm.save();
    await flushPromises();

    expect(putLoginConfigApi).toHaveBeenCalledTimes(1);
    const [id, payload] = putLoginConfigApi.mock.calls[0] as [
      number,
      { enabled: boolean; configJson: Record<string, unknown>; templateVersion?: string }
    ];
    expect(id).toBe(101);
    // 未修改 → 提交哨兵 ******（保留原密文，绝不回明文）
    expect(payload.configJson.appSecret).toBe("******");
    expect(payload.templateVersion).toBe("v1");
  });

  test("密文修改：聚焦清空后输入新值，提交新明文而非哨兵", async () => {
    const { vm } = await mountPanel();

    vm.beginEditSecret("appSecret");
    vm.setSecretValue("appSecret", "new-plain-secret-123");
    await nextTick();

    putLoginConfigApi.mockResolvedValue(loginConfigResponse());
    await vm.save();
    await flushPromises();

    const payload = putLoginConfigApi.mock.calls[0][1] as { configJson: Record<string, unknown> };
    expect(payload.configJson.appSecret).toBe("new-plain-secret-123");
  });

  test("config_status 三色展示：empty/invalid/valid → neutral/danger/success", async () => {
    const { vm, wrapper } = await mountPanel({
      response: loginConfigResponse({ config: { configStatus: "invalid", lastCheckMessage: "密钥过短" } })
    });
    expect(vm.statusTone("empty")).toBe("neutral");
    expect(vm.statusTone("invalid")).toBe("danger");
    expect(vm.statusTone("valid")).toBe("success");
    expect(vm.statusLabel("invalid")).toBe("配置无效");
    // 异常态消息不被隐藏
    expect(wrapper.html()).toContain("密钥过短");
  });

  test("enabled=true 但 status!=valid 显著告警条", async () => {
    const { wrapper } = await mountPanel({
      response: loginConfigResponse({ config: { enabled: true, configStatus: "invalid" } })
    });
    expect(wrapper.html()).toContain("已启用但配置无效，将不进入快照/同步/客户端最终配置");
  });

  test("enabled=false 或 valid 时不展示启用告警条", async () => {
    const valid = await mountPanel({
      response: loginConfigResponse({ config: { enabled: true, configStatus: "valid" } })
    });
    expect(valid.wrapper.html()).not.toContain("已启用但配置无效");

    const disabled = await mountPanel({
      response: loginConfigResponse({ config: { enabled: false, configStatus: "invalid" } })
    });
    expect(disabled.wrapper.html()).not.toContain("已启用但配置无效");
  });

  test("复制创建 invalid 提示：lastCheckMessage 含缺少必填敏感字段时提示补齐", async () => {
    const { wrapper } = await mountPanel({
      response: loginConfigResponse({
        config: {
          enabled: false,
          configStatus: "invalid",
          configJson: { appId: "huawei-app-001" },
          lastCheckMessage: "缺少必填敏感字段或文件字段"
        }
      })
    });
    expect(wrapper.html()).toContain("该实例来自复制创建，请补齐密钥/文件字段后再投入运行");
  });

  test("validationRules 即时校验：pattern / minLen / 数值越界", async () => {
    const { vm } = await mountPanel({ response: loginConfigResponse({ config: emptyLoginConfig() }) });

    // appId pattern 不符
    vm.setDraftValue("appId", "bad id!!");
    await nextTick();
    expect(vm.fieldErrors.appId).toBeTruthy();

    // appId 合法 → 无错
    vm.setDraftValue("appId", "huawei_app_001");
    await nextTick();
    expect(vm.fieldErrors.appId).toBeUndefined();

    // appSecret 编辑态太短 minLen=8
    vm.beginEditSecret("appSecret");
    vm.setSecretValue("appSecret", "short");
    await nextTick();
    expect(vm.fieldErrors.appSecret).toBeTruthy();

    // timeout 越界 max=60
    vm.setDraftValue("timeout", 120);
    await nextTick();
    expect(vm.fieldErrors.timeout).toBeTruthy();
  });

  test("校验未过阻断保存：ElMessage.warning 且不调用 PUT", async () => {
    const { vm } = await mountPanel({ response: loginConfigResponse({ config: emptyLoginConfig() }) });
    // 必填 appId 为空 → 必有 fieldError
    await vm.save();
    await flushPromises();
    expect(putLoginConfigApi).not.toHaveBeenCalled();
    expect(messageWarning).toHaveBeenCalled();
  });

  test("无 channel.write 权限：开关与保存按钮置灰禁用", async () => {
    const { wrapper } = await mountPanel({ canWrite: false });
    // 启用开关禁用
    expect(wrapper.find(".el-switch.is-disabled").exists()).toBe(true);
    // 保存按钮禁用（v-perm + :disabled）
    const saveBtn = wrapper.findAll("button").find((b) => b.text().includes("保存渠道登录配置"));
    expect(saveBtn?.attributes("disabled")).toBeDefined();
  });

  test("保存成功：回填模型并 emit changed + success 提示", async () => {
    const { vm, wrapper } = await mountPanel();
    putLoginConfigApi.mockResolvedValue(
      loginConfigResponse({ config: { configStatus: "valid", lastCheckMessage: "保存后校验通过" } })
    );
    await vm.save();
    await flushPromises();

    expect(vm.model?.config.lastCheckMessage).toBe("保存后校验通过");
    expect(messageSuccess).toHaveBeenCalled();
    expect(wrapper.emitted("changed")).toBeTruthy();
  });

  test("PUT VALIDATION_FAILED：触发二次 GET 回显 invalid 行内态并 emit changed", async () => {
    const { vm, wrapper } = await mountPanel();
    expect(getLoginConfigApi).toHaveBeenCalledTimes(1); // 初次加载

    putLoginConfigApi.mockRejectedValue(
      new ApiError(400, "VALIDATION_FAILED", "缺少必填敏感字段或文件字段: appSecret")
    );
    getLoginConfigApi.mockResolvedValue(
      loginConfigResponse({ config: { configStatus: "invalid", lastCheckMessage: "缺少必填敏感字段或文件字段" } })
    );
    await vm.save();
    await flushPromises();

    expect(messageError).toHaveBeenCalledWith("缺少必填敏感字段或文件字段: appSecret");
    // 失败后二次 GET 回显
    expect(getLoginConfigApi).toHaveBeenCalledTimes(2);
    expect(vm.model?.config.configStatus).toBe("invalid");
    expect(wrapper.emitted("changed")).toBeTruthy();
  });

  test("加载失败展示错误提示", async () => {
    setActivePinia(createPinia());
    getLoginConfigApi.mockRejectedValue(new ApiError(500, "INTERNAL", "服务异常"));
    const wrapper = mount(ChannelLoginConfigPanel, {
      props: { gameChannelId: 101, detail: marketChannelDetail(), canWrite: true },
      global: { directives: { perm: permDirective } }
    });
    await flushPromises();
    expect(messageError).toHaveBeenCalledWith("服务异常");
  });
});
