// channel-login 前端测试 fixtures（模板四件套 mock + config 实例）
// 对齐 spec.compact.md §前端要点 与 GET/PUT login-config 契约。
import type {
  ChannelLoginConfig,
  ChannelLoginConfigResponse,
  ChannelLoginTemplate,
  ChannelLoginTemplateField,
  MarketChannelDetail
} from "@/api/modules/channels";

// 故意打乱 order，用于验证渲染器按 order 升序排序、按 group 分组。
export function huaweiFormSchema(): ChannelLoginTemplateField[] {
  return [
    { key: "extra", label: "扩展参数", component: "json", order: 6, group: "高级" },
    { key: "appId", label: "App ID", component: "input", required: true, order: 1, group: "基础" },
    {
      key: "region",
      label: "区域",
      component: "select",
      order: 2,
      group: "基础",
      options: [
        { label: "中国大陆", value: "cn" },
        { label: "海外", value: "global" }
      ]
    },
    { key: "appSecret", label: "App Secret", component: "password", required: true, order: 3, group: "密钥" },
    { key: "enableLog", label: "启用日志", component: "switch", order: 5, group: "高级" },
    { key: "timeout", label: "超时(秒)", component: "number", order: 4, group: "高级" },
    { key: "cert", label: "证书文件", component: "file", required: true, order: 7, group: "密钥" }
  ];
}

export function huaweiTemplate(over: Partial<ChannelLoginTemplate> = {}): ChannelLoginTemplate {
  return {
    templateVersion: "v1",
    formSchemaJson: huaweiFormSchema(),
    secretFieldsJson: ["appSecret"],
    fileFieldsJson: [{ key: "cert", accept: [".pem"], maxSizeKB: 64 }],
    validationRulesJson: {
      appId: { minLen: 1, maxLen: 64, pattern: "^[0-9A-Za-z_-]+$" },
      appSecret: { minLen: 8, maxLen: 256 },
      timeout: { min: 1, max: 60 }
    },
    ...over
  };
}

export function loginConfig(over: Partial<ChannelLoginConfig> = {}): ChannelLoginConfig {
  return {
    enabled: true,
    configJson: { appId: "huawei-app-001", region: "cn", appSecret: "******", timeout: 30, cert: "cert-ref-001.pem" },
    configStatus: "valid",
    lastCheckAt: "2026-01-01T00:00:00Z",
    lastCheckMessage: "校验通过",
    ...over
  };
}

export function emptyLoginConfig(over: Partial<ChannelLoginConfig> = {}): ChannelLoginConfig {
  return {
    enabled: false,
    configJson: {},
    configStatus: "empty",
    lastCheckAt: null,
    lastCheckMessage: "",
    ...over
  };
}

export function loginConfigResponse(
  over: Partial<Omit<ChannelLoginConfigResponse, "config" | "template">> & {
    config?: Partial<ChannelLoginConfig>;
    template?: Partial<ChannelLoginTemplate>;
  } = {}
): ChannelLoginConfigResponse {
  const { config, template, ...rest } = over;
  return {
    gameChannelId: 101,
    env: "sandbox",
    channelId: "huawei_cn",
    marketCode: "CN",
    loginMode: "channel_only",
    loginLocked: false,
    config: loginConfig(config),
    template: huaweiTemplate(template),
    ...rest
  };
}

// 渠道实例详情（抽屉 props / 页签可见性测试用）
export function marketChannelDetail(over: Partial<MarketChannelDetail> = {}): MarketChannelDetail {
  return {
    gameChannelId: 101,
    displayKey: "100001:CN:huawei_cn",
    gameId: "100001",
    market: "CN",
    channelId: "huawei_cn",
    region: "domestic",
    compatible: true,
    hidden: false,
    configStatus: "valid",
    includedInSnapshot: true,
    includedInSync: true,
    includedInRuntimeConfig: true,
    copiedFromMarket: "",
    updatedAt: "2026-01-01T00:00:00Z",
    channelName: "华为(中国)",
    channelType: "store",
    loginMode: "channel_only",
    paymentMode: "channel_only",
    loginLocked: false,
    paymentLocked: false,
    enabled: true,
    remark: "",
    hiddenBy: "",
    hiddenAt: null,
    lastCheckAt: "2026-01-01T00:00:00Z",
    lastCheckMessage: "校验通过",
    createdAt: "2026-01-01T00:00:00Z",
    ...over
  };
}
