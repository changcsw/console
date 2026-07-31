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

/** 发行市场中文标签（下拉/表格展示用，值为英文代码，如 "全球（GLOBAL）" 场景自行拼接） */
export function regionLabel(region: ChannelRegion | string): string {
  switch (region) {
    case "GLOBAL":
      return "全球";
    case "CN":
      return "中国大陆";
    case "JP":
      return "日本";
    case "KR":
      return "韩国";
    case "SEA":
      return "东南亚";
    case "HMT":
      return "港澳台";
    default:
      return region;
  }
}

/** 发行市场带代码的完整展示：全球（GLOBAL） */
export function regionFullLabel(region: ChannelRegion | string): string {
  const label = regionLabel(region);
  return label === region ? region : `${label}（${region}）`;
}

/**
 * 前端候选过滤用，与后端 domain/channel.ValidateMarketChannelCompatibility 同口径：
 * CN 市场仅允许发行市场为 CN 的渠道；非 CN 市场允许 GLOBAL（全球发行）或与该市场相同的渠道。
 * 服务端会二次强制校验，前端仅用于收窄候选与列表标红。
 */
export function isMarketChannelCompatible(market: Market, region: ChannelRegion): boolean {
  if (market === "CN") {
    return region === "CN";
  }
  return region === "GLOBAL" || region === market;
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
