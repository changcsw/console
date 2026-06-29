import type { GameStatus, LegalScopeType, Market } from "@/api/modules/games";

export const MARKET_OPTIONS: Market[] = ["GLOBAL", "JP", "KR", "SEA", "HMT", "CN"];

export const STATUS_OPTIONS: { label: string; value: GameStatus; tone: "neutral" | "success" | "danger" }[] = [
  { label: "草稿", value: "draft", tone: "neutral" },
  { label: "已激活", value: "active", tone: "success" },
  { label: "已停用", value: "disabled", tone: "danger" }
];

export const SCOPE_TYPE_OPTIONS: { label: string; value: LegalScopeType }[] = [
  { label: "默认（兜底）", value: "default" },
  { label: "市场", value: "market" },
  { label: "语言", value: "locale" }
];

export const DEFAULT_LOCALE = "en-US";

// 与后端 IsValidOptionalURL 语义一致：非空时要求 http(s):// 前缀，留空允许（compact：format=url 非空时）。
const OPTIONAL_URL_PATTERN = /^https?:\/\//;

export function isValidOptionalUrl(url: string): boolean {
  return url === "" || OPTIONAL_URL_PATTERN.test(url);
}

export function statusMeta(status: string): { label: string; tone: "neutral" | "success" | "danger" } {
  const found = STATUS_OPTIONS.find((item) => item.value === status);
  return found ? { label: found.label, tone: found.tone } : { label: status, tone: "neutral" };
}

export function scopeTypeLabel(scopeType: string): string {
  return SCOPE_TYPE_OPTIONS.find((item) => item.value === scopeType)?.label ?? scopeType;
}
