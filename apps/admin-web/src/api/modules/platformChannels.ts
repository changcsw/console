import { request } from "@/api/http";
import type {
  ChannelRegion,
  ChannelType,
  FormFieldComponent,
  LoginMode,
  Paginated,
  PaymentMode,
  TemplateFieldOption
} from "@/api/modules/channels";

// 平台渠道主数据与渠道模版：系统管理员维护的平台级基础数据，与具体游戏无关。
// 游戏侧只在渠道实例上引用模版填参（见 @/api/modules/channels）。

export type { ChannelRegion, ChannelType, LoginMode, PaymentMode, TemplateFieldOption };

/** 模版字段作用域，空串表示不区分 */
export type TemplateFieldScope = "" | "client" | "server" | "both";

export const CHANNEL_TYPE_OPTIONS: ChannelType[] = ["store", "domestic", "mini_game"];
/** 发行市场下拉顺序固定：全球 → 中国大陆 → 日本 → 韩国 → 东南亚 → 港澳台 */
export const CHANNEL_REGION_OPTIONS: ChannelRegion[] = ["GLOBAL", "CN", "JP", "KR", "SEA", "HMT"];
export const LOGIN_MODE_OPTIONS: LoginMode[] = ["channel_only", "account_system"];
export const PAYMENT_MODE_OPTIONS: PaymentMode[] = ["channel_only", "hybrid", "cashier_only"];
export const TEMPLATE_COMPONENT_OPTIONS: FormFieldComponent[] = [
  "input",
  "password",
  "textarea",
  "number",
  "select",
  "switch",
  "file",
  "json"
];
export const TEMPLATE_SCOPE_OPTIONS: TemplateFieldScope[] = ["", "client", "server", "both"];

/** 渠道模版种类：登录模版与 IAP 模版分表存储，同渠道下版本号互不冲突 */
export type ChannelTemplateKind = "login" | "iap";

export interface PlatformChannel {
  channelId: string;
  channelName: string;
  channelType: ChannelType;
  region: ChannelRegion;
  enabled: boolean;
  sort: number;
  loginMode: LoginMode;
  paymentMode: PaymentMode;
  loginLocked: boolean;
  paymentLocked: boolean;
  loginTemplateCount: number;
  iapTemplateCount: number;
  updatedAt: string;
}

/** 模版表单字段定义（formSchemaJson 项） */
export interface TemplateFieldDef {
  key: string;
  label: string;
  component: FormFieldComponent;
  required?: boolean;
  order?: number;
  group?: string;
  scope?: TemplateFieldScope;
  placeholder?: string;
  options?: TemplateFieldOption[];
}

/** 模版文件字段约束（fileFieldsJson 项） */
export interface TemplateFileFieldDef {
  key: string;
  accept?: string[];
  maxSizeKB?: number;
}

/** 模版校验规则（validationRulesJson 的值） */
export interface TemplateRuleDef {
  required?: boolean;
  minLen?: number;
  maxLen?: number;
  min?: number;
  max?: number;
  pattern?: string;
  format?: string;
  enum?: string[];
}

export interface ChannelTemplate {
  templateId: number;
  kind: ChannelTemplateKind;
  channelId: string;
  templateVersion: string;
  formSchemaJson: TemplateFieldDef[];
  secretFieldsJson: string[];
  fileFieldsJson: TemplateFileFieldDef[];
  validationRulesJson: Record<string, TemplateRuleDef>;
  enabled: boolean;
  /** 是否为当前生效版本：同渠道同类中 enabled 的最新 templateVersion */
  effective: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface ListPlatformChannelsQuery {
  /** 模糊匹配 channelId / channelName */
  keyword?: string;
  region?: ChannelRegion;
  channelType?: ChannelType;
  /** 不限 */
  enabled?: boolean;
  page?: number;
  pageSize?: number;
}

export interface CreatePlatformChannelRequest {
  channelId: string;
  channelName: string;
  channelType: ChannelType;
  region: ChannelRegion;
  /** 默认 true */
  enabled?: boolean;
  /** 默认 0，取值 0-9999 */
  sort?: number;
  loginMode: LoginMode;
  paymentMode: PaymentMode;
  loginLocked?: boolean;
  paymentLocked?: boolean;
}

/** channelId / channelType / region 创建后不可改，故不在补丁内 */
export interface UpdatePlatformChannelRequest {
  channelName?: string;
  enabled?: boolean;
  sort?: number;
  loginMode?: LoginMode;
  paymentMode?: PaymentMode;
  loginLocked?: boolean;
  paymentLocked?: boolean;
}

export interface CreateChannelTemplateRequest {
  kind: ChannelTemplateKind;
  templateVersion: string;
  formSchemaJson: TemplateFieldDef[];
  secretFieldsJson: string[];
  fileFieldsJson: TemplateFileFieldDef[];
  validationRulesJson: Record<string, TemplateRuleDef>;
  /** 默认 true */
  enabled?: boolean;
}

/** 四件套为整体替换语义：字段省略表示该件不改；templateVersion 与所属渠道不可改 */
export interface UpdateChannelTemplateRequest {
  formSchemaJson?: TemplateFieldDef[];
  secretFieldsJson?: string[];
  fileFieldsJson?: TemplateFileFieldDef[];
  validationRulesJson?: Record<string, TemplateRuleDef>;
  enabled?: boolean;
}

function buildQuery(params: Record<string, unknown>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      search.append(key, String(value));
    }
  }
  const query = search.toString();
  return query ? `?${query}` : "";
}

const enc = encodeURIComponent;

// GET /platform/channels — 平台渠道分页（platform_channel.read）
export function listPlatformChannels(query: ListPlatformChannelsQuery = {}): Promise<Paginated<PlatformChannel>> {
  return request<Paginated<PlatformChannel>>(`/api/admin/platform/channels${buildQuery({ ...query })}`);
}

// GET /platform/channels/{channelId} — 渠道详情（platform_channel.read）
export function getPlatformChannel(channelId: string): Promise<PlatformChannel> {
  return request<PlatformChannel>(`/api/admin/platform/channels/${enc(channelId)}`);
}

// POST /platform/channels — 新建渠道 + 策略（platform_channel.write）
export function createPlatformChannel(payload: CreatePlatformChannelRequest): Promise<PlatformChannel> {
  return request<PlatformChannel>("/api/admin/platform/channels", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

// PATCH /platform/channels/{channelId} — 改名/启用/排序/策略（platform_channel.write）
export function updatePlatformChannel(
  channelId: string,
  payload: UpdatePlatformChannelRequest
): Promise<PlatformChannel> {
  return request<PlatformChannel>(`/api/admin/platform/channels/${enc(channelId)}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

// GET /platform/channels/{channelId}/templates — 模版版本列表，kind 省略返两类（channel_template.read）
export async function listChannelTemplates(
  channelId: string,
  kind?: ChannelTemplateKind
): Promise<ChannelTemplate[]> {
  const res = await request<{ items: ChannelTemplate[] }>(
    `/api/admin/platform/channels/${enc(channelId)}/templates${buildQuery({ kind })}`
  );
  return res.items ?? [];
}

// GET /platform/channel-templates/{kind}/{templateId} — 单个模版版本（channel_template.read）
export function getChannelTemplate(kind: ChannelTemplateKind, templateId: number): Promise<ChannelTemplate> {
  return request<ChannelTemplate>(`/api/admin/platform/channel-templates/${enc(kind)}/${templateId}`);
}

// POST /platform/channels/{channelId}/templates — 新建模版版本（channel_template.write）
export function createChannelTemplate(
  channelId: string,
  payload: CreateChannelTemplateRequest
): Promise<ChannelTemplate> {
  return request<ChannelTemplate>(`/api/admin/platform/channels/${enc(channelId)}/templates`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

// PATCH /platform/channel-templates/{kind}/{templateId} — 整体替换四件套/启用（channel_template.write）
export function updateChannelTemplate(
  kind: ChannelTemplateKind,
  templateId: number,
  payload: UpdateChannelTemplateRequest
): Promise<ChannelTemplate> {
  return request<ChannelTemplate>(`/api/admin/platform/channel-templates/${enc(kind)}/${templateId}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}
