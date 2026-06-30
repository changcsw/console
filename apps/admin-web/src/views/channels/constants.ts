import type { ChannelRegion, ConfigStatus, Market } from "@/api/modules/channels";

export const MARKET_OPTIONS: Market[] = ["GLOBAL", "JP", "KR", "SEA", "HMT", "CN"];

export type Tone = "neutral" | "success" | "warning" | "danger";

export const CONFIG_STATUS_OPTIONS: { label: string; value: ConfigStatus; tone: Tone }[] = [
  { label: "未配置", value: "empty", tone: "neutral" },
  { label: "配置无效", value: "invalid", tone: "danger" },
  { label: "配置有效", value: "valid", tone: "success" }
];

export function configStatusMeta(status: string): { label: string; tone: Tone } {
  const found = CONFIG_STATUS_OPTIONS.find((item) => item.value === status);
  return found ? { label: found.label, tone: found.tone } : { label: status, tone: "neutral" };
}

export function regionLabel(region: ChannelRegion | string): string {
  switch (region) {
    case "domestic":
      return "国内";
    case "overseas":
      return "非国内";
    default:
      return region;
  }
}

/**
 * 前端候选过滤用，与后端 domain/channel.ValidateMarketChannelCompatibility 同口径：
 * CN 仅 domestic 兼容；非 CN 仅 overseas 兼容（GLOBAL 仅显示 overseas）。
 * 服务端会二次强制校验，前端仅用于收窄候选与列表标红。
 */
export function isMarketChannelCompatible(market: Market, region: ChannelRegion): boolean {
  if (market === "CN") {
    return region === "domestic";
  }
  return region === "overseas";
}

/** 运行态不生效原因（compact §运行态标识：hidden / incompatible / invalid_config） */
export function runtimeBlockReason(item: {
  hidden: boolean;
  compatible: boolean;
  configStatus: ConfigStatus;
  enabled?: boolean;
}): string | null {
  if (item.enabled === false) {
    return "实例未启用";
  }
  if (item.hidden) {
    return "已隐藏，已移出生效集";
  }
  if (!item.compatible) {
    return "与当前 market 不兼容";
  }
  if (item.configStatus !== "valid") {
    return "配置未通过校验（invalid/empty）";
  }
  return null;
}

export const COPY_INVALID_HINT = "缺少必填敏感字段或文件字段";
