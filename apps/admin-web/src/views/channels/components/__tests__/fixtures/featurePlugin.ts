import type { ChannelPackagePluginItem, GameChannelPluginItem } from "@/api/modules/channels";

export function featurePluginItem(overrides: Partial<GameChannelPluginItem> = {}): GameChannelPluginItem {
  return {
    id: 501,
    pluginId: "anti_addiction",
    pluginName: "防沉迷",
    region: "domestic",
    required: true,
    selectable: false,
    locked: false,
    enabled: true,
    configStatus: "invalid",
    includedInRuntimeConfig: false,
    configJson: {
      appKey: "masked",
      callback: "https://example.com/callback"
    },
    lastCheckAt: null,
    lastCheckMessage: "缺少必填敏感字段或文件字段",
    template: {
      templateVersion: "v1",
      formSchemaJson: [
        { key: "appKey", label: "App Key", component: "password", required: true, order: 1, scope: "server" },
        { key: "callback", label: "回调地址", component: "input", required: true, order: 2, scope: "both" }
      ],
      secretFieldsJson: ["appKey"],
      fileFieldsJson: [],
      validationRulesJson: {
        callback: { format: "url" }
      }
    },
    ...overrides
  };
}

/** 海外 / 可勾选 / 已生效的可选插件（用于多状态徽标渲染断言）。 */
export function optionalPluginItem(overrides: Partial<GameChannelPluginItem> = {}): GameChannelPluginItem {
  return featurePluginItem({
    id: 0,
    pluginId: "push",
    pluginName: "推送",
    region: "overseas",
    required: false,
    selectable: true,
    locked: false,
    enabled: true,
    configStatus: "valid",
    includedInRuntimeConfig: true,
    configJson: { callback: "https://example.com/push" },
    lastCheckMessage: "校验通过",
    template: {
      templateVersion: "v1",
      formSchemaJson: [{ key: "callback", label: "回调地址", component: "input", required: true, order: 1, scope: "both" }],
      secretFieldsJson: [],
      fileFieldsJson: [],
      validationRulesJson: { callback: { format: "url" } }
    },
    ...overrides
  });
}

/** 含 file 字段的插件（用于上传校验断言）。 */
export function filePluginItem(overrides: Partial<GameChannelPluginItem> = {}): GameChannelPluginItem {
  return featurePluginItem({
    id: 0,
    pluginId: "kyc",
    pluginName: "实名认证",
    region: "domestic",
    required: false,
    selectable: true,
    locked: false,
    enabled: false,
    configStatus: "empty",
    configJson: {},
    lastCheckMessage: "",
    template: {
      templateVersion: "v1",
      formSchemaJson: [{ key: "license", label: "营业执照", component: "file", required: false, order: 1, scope: "server" }],
      secretFieldsJson: [],
      fileFieldsJson: [{ key: "license", accept: [".pdf", ".png"], maxSizeKB: 100 }],
      validationRulesJson: {}
    },
    ...overrides
  });
}

export function channelPackagePluginItem(overrides: Partial<ChannelPackagePluginItem> = {}): ChannelPackagePluginItem {
  return {
    id: 701,
    packageId: 9001,
    pluginId: "anti_addiction",
    pluginName: "防沉迷",
    region: "domestic",
    required: true,
    selectable: false,
    locked: false,
    inheritChannelConfig: true,
    enabled: true,
    configStatus: "valid",
    includedInRuntimeConfig: true,
    configJson: {
      endpoint: "https://example.com/aa",
      apiKey: "masked"
    },
    lastCheckAt: null,
    lastCheckMessage: "校验通过",
    template: {
      templateVersion: "v1",
      formSchemaJson: [
        { key: "endpoint", label: "服务地址", component: "input", required: true, order: 1, scope: "both" },
        { key: "apiKey", label: "API Key", component: "password", required: true, order: 2, scope: "server" }
      ],
      secretFieldsJson: ["apiKey"],
      fileFieldsJson: [],
      validationRulesJson: { endpoint: { format: "url" } }
    },
    ...overrides
  };
}
