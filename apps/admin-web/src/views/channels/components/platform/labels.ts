import type {
  ChannelTemplateKind,
  ChannelType,
  LoginMode,
  PaymentMode,
  TemplateFieldScope
} from "@/api/modules/platformChannels";

// 平台渠道管理页的枚举中文标签。枚举值本身是全局事实源（见 @/api/modules/platformChannels），
// 这里只负责展示层文案。

const CHANNEL_TYPE_LABELS: Record<ChannelType, string> = {
  store: "应用商店",
  oem: "手机厂商",
  web: "网页",
  direct: "直充",
  mini_game: "小游戏"
};

const LOGIN_MODE_LABELS: Record<LoginMode, string> = {
  channel_only: "仅渠道登录",
  account_system: "自有账号体系"
};

const PAYMENT_MODE_LABELS: Record<PaymentMode, string> = {
  channel_only: "仅渠道支付",
  hybrid: "混合支付",
  cashier_only: "仅收银台"
};

const TEMPLATE_KIND_LABELS: Record<ChannelTemplateKind, string> = {
  login: "渠道登录",
  iap: "渠道 IAP"
};

const SCOPE_LABELS: Record<TemplateFieldScope, string> = {
  "": "不区分",
  client: "客户端",
  server: "服务端",
  both: "双端"
};

export function channelTypeLabel(value: string): string {
  return CHANNEL_TYPE_LABELS[value as ChannelType] ?? value;
}

export function loginModeLabel(value: string): string {
  return LOGIN_MODE_LABELS[value as LoginMode] ?? value;
}

export function paymentModeLabel(value: string): string {
  return PAYMENT_MODE_LABELS[value as PaymentMode] ?? value;
}

export function templateKindLabel(value: string): string {
  return TEMPLATE_KIND_LABELS[value as ChannelTemplateKind] ?? value;
}

export function scopeLabel(value: string): string {
  return SCOPE_LABELS[value as TemplateFieldScope] ?? value;
}
