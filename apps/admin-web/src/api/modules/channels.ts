import { request } from "@/api/http";

// 枚举（与 00-common §3 / 12-channel compact 全局事实源一致；本模块本地引用，待 dictionary store 落地后可切换）
export type Market = "GLOBAL" | "JP" | "KR" | "SEA" | "HMT" | "CN";
export type ChannelRegion = "domestic" | "overseas";
export type ChannelType = "store" | "oem" | "web" | "direct" | "mini_game";
export type LoginMode = "channel_only" | "account_system";
export type PaymentMode = "channel_only" | "hybrid" | "cashier_only";
export type ConfigStatus = "empty" | "invalid" | "valid";
export type CreateChannelMode = "empty" | "copy";

export interface Paginated<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

// GET /games/{gameId}/channels → 该游戏可用渠道主数据（新增时筛选）
export interface AvailableChannel {
  channelId: string;
  channelName: string;
  channelType: ChannelType;
  region: ChannelRegion;
  loginMode: LoginMode;
  paymentMode: PaymentMode;
  loginLocked: boolean;
  paymentLocked: boolean;
}

// GET /games/{gameId}/market-channels → 渠道实例列表行
export interface MarketChannelListItem {
  /** 路径参数统一口径：int64 game_channels.id，前端从行回传不解析复合串 */
  gameChannelId: number;
  /** 复合串 gameId:market:channelId，仅作展示/行 key */
  displayKey: string;
  gameId: string;
  market: Market;
  channelId: string;
  region: ChannelRegion;
  compatible: boolean;
  hidden: boolean;
  configStatus: ConfigStatus;
  includedInSnapshot: boolean;
  includedInSync: boolean;
  includedInRuntimeConfig: boolean;
  copiedFromMarket: string;
  updatedAt: string;
}

// 实例详情（GET /game-channels/{gameChannelId}；hide/unhide/patch 返回更新后实例）
export interface MarketChannelDetail extends MarketChannelListItem {
  channelName?: string;
  channelType?: ChannelType;
  loginMode?: LoginMode;
  paymentMode?: PaymentMode;
  loginLocked?: boolean;
  paymentLocked?: boolean;
  enabled: boolean;
  remark: string;
  hiddenBy: string;
  hiddenAt: string | null;
  lastCheckAt: string | null;
  lastCheckMessage: string;
  createdAt: string;
}

export interface ListMarketChannelsQuery {
  /** 默认 ALL */
  market?: Market | "ALL";
  channelId?: string;
  /** 不限 */
  compatible?: boolean;
  /** 默认 false（不含隐藏） */
  hidden?: boolean;
  /** 不限 */
  configStatus?: ConfigStatus;
  page?: number;
  pageSize?: number;
}

export interface CreateMarketChannelRequest {
  channelId: string;
  /** 默认 empty */
  mode?: CreateChannelMode;
  /** mode=copy 时必填 */
  copyFromMarket?: string;
  enabled?: boolean;
  remark?: string;
}

export interface CreateMarketChannelResult {
  gameChannelId: number;
  displayKey: string;
  market: Market;
  channelId: string;
  configStatus: ConfigStatus;
  /** 复制创建时提示"缺少必填敏感字段或文件字段" */
  lastCheckMessage: string;
  copiedFromMarket: string;
}

export interface UpdateMarketChannelRequest {
  enabled?: boolean;
  remark?: string;
}

export interface ChannelPackage {
  packageId: number;
  gameChannelId: number;
  packageCode: string;
  packageName: string;
  marketCode: Market;
  bundleId: string;
  inheritChannelConfig: boolean;
  enabled: boolean;
  overrideJson: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface CreateChannelPackageRequest {
  packageCode: string;
  packageName: string;
  /** 须等于所属实例 market */
  marketCode: Market;
  bundleId?: string;
  inheritChannelConfig?: boolean;
  enabled?: boolean;
}

export interface UpdateChannelPackageRequest {
  packageName?: string;
  bundleId?: string;
  inheritChannelConfig?: boolean;
  enabled?: boolean;
  overrideJson?: Record<string, unknown>;
}

export type FormFieldComponent = "input" | "password" | "textarea" | "number" | "select" | "switch" | "file" | "json";

export interface TemplateFieldOption {
  label: string;
  value: string | number | boolean;
}

export interface ChannelLoginTemplateField {
  key: string;
  label: string;
  component: FormFieldComponent;
  required?: boolean;
  placeholder?: string;
  order?: number;
  group?: string;
  options?: TemplateFieldOption[];
}

export interface ChannelLoginTemplateFileField {
  key: string;
  accept?: string[];
  maxSizeKB?: number;
}

export interface ChannelLoginTemplate {
  templateVersion: string;
  formSchemaJson: ChannelLoginTemplateField[];
  secretFieldsJson: string[];
  fileFieldsJson: ChannelLoginTemplateFileField[];
  validationRulesJson: Record<string, unknown>;
}

export interface ChannelLoginConfig {
  enabled: boolean;
  configJson: Record<string, unknown>;
  configStatus: ConfigStatus;
  lastCheckAt: string | null;
  lastCheckMessage: string;
}

export interface ChannelLoginConfigResponse {
  gameChannelId: number;
  env: string;
  channelId: string;
  marketCode: Market;
  loginMode: LoginMode;
  loginLocked: boolean;
  config: ChannelLoginConfig;
  template: ChannelLoginTemplate;
}

export interface PutChannelLoginConfigPayload {
  enabled?: boolean;
  configJson: Record<string, unknown>;
  templateVersion?: string;
}

export type TemplateScope = "client" | "server" | "both";

export interface PluginTemplateField {
  key: string;
  label: string;
  component: FormFieldComponent;
  required?: boolean;
  placeholder?: string;
  order?: number;
  group?: string;
  scope?: TemplateScope;
  options?: TemplateFieldOption[];
}

export interface PluginTemplateFileField {
  key: string;
  accept?: string[];
  maxSizeKB?: number;
}

export interface FeaturePluginTemplate {
  templateVersion: string;
  formSchemaJson: PluginTemplateField[];
  secretFieldsJson: string[];
  fileFieldsJson: PluginTemplateFileField[];
  validationRulesJson: Record<string, unknown>;
}

export interface FeaturePluginConfig {
  id: number;
  enabled: boolean;
  configJson: Record<string, unknown>;
  configStatus: ConfigStatus;
  lastCheckAt: string | null;
  lastCheckMessage: string;
}

export interface GameChannelPluginItem {
  id: number;
  pluginId: string;
  pluginName: string;
  region: ChannelRegion;
  required: boolean;
  selectable: boolean;
  locked: boolean;
  enabled: boolean;
  configStatus: ConfigStatus;
  includedInRuntimeConfig: boolean;
  configJson: Record<string, unknown>;
  lastCheckAt: string | null;
  lastCheckMessage: string;
  template: FeaturePluginTemplate;
}

export interface UpsertGameChannelPluginPayload {
  pluginId: string;
  enabled?: boolean;
  config?: Record<string, unknown>;
}

export interface PatchGameChannelPluginPayload {
  enabled?: boolean;
  config?: Record<string, unknown>;
}

export interface ChannelPackagePluginItem {
  id: number;
  packageId: number;
  pluginId: string;
  pluginName: string;
  region: ChannelRegion;
  required: boolean;
  selectable: boolean;
  locked: boolean;
  inheritChannelConfig: boolean;
  enabled: boolean;
  configStatus: ConfigStatus;
  includedInRuntimeConfig: boolean;
  configJson: Record<string, unknown>;
  lastCheckAt: string | null;
  lastCheckMessage: string;
  template: FeaturePluginTemplate;
}

export interface UpsertChannelPackagePluginPayload {
  pluginId: string;
  inheritChannelConfig?: boolean;
  enabled?: boolean;
  config?: Record<string, unknown>;
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

// GET /games/{gameId}/channels — 可用渠道主数据（channel.read）
export async function listGameChannels(gameId: string): Promise<AvailableChannel[]> {
  const res = await request<{ items: AvailableChannel[] }>(`/api/admin/games/${enc(gameId)}/channels`);
  return res.items ?? [];
}

// GET /games/{gameId}/market-channels — 渠道实例分页（channel.read）
export function listMarketChannels(
  gameId: string,
  query: ListMarketChannelsQuery = {}
): Promise<Paginated<MarketChannelListItem>> {
  return request<Paginated<MarketChannelListItem>>(
    `/api/admin/games/${enc(gameId)}/market-channels${buildQuery({ ...query })}`
  );
}

// POST /games/{gameId}/markets/{market}/channels — 创建空白/复制（channel.write）
export function createMarketChannel(
  gameId: string,
  market: Market,
  payload: CreateMarketChannelRequest
): Promise<CreateMarketChannelResult> {
  return request<CreateMarketChannelResult>(`/api/admin/games/${enc(gameId)}/markets/${enc(market)}/channels`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

// GET /game-channels/{gameChannelId} — 实例详情（channel.read）
export function getMarketChannel(gameChannelId: number): Promise<MarketChannelDetail> {
  return request<MarketChannelDetail>(`/api/admin/game-channels/${gameChannelId}`);
}

// PATCH /game-channels/{gameChannelId} — 改 enabled/remark（channel.write）
export function updateMarketChannel(
  gameChannelId: number,
  payload: UpdateMarketChannelRequest
): Promise<MarketChannelDetail> {
  return request<MarketChannelDetail>(`/api/admin/game-channels/${gameChannelId}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

// POST /game-channels/{gameChannelId}/hide — 隐藏（channel.write）
export function hideMarketChannel(gameChannelId: number, reason?: string): Promise<MarketChannelDetail> {
  return request<MarketChannelDetail>(`/api/admin/game-channels/${gameChannelId}/hide`, {
    method: "POST",
    body: JSON.stringify(reason ? { reason } : {})
  });
}

// POST /game-channels/{gameChannelId}/unhide — 恢复（channel.write）
export function unhideMarketChannel(gameChannelId: number): Promise<MarketChannelDetail> {
  return request<MarketChannelDetail>(`/api/admin/game-channels/${gameChannelId}/unhide`, {
    method: "POST",
    body: JSON.stringify({})
  });
}

// GET /game-channels/{gameChannelId}/packages — 渠道包列表（channel.read）
export async function listChannelPackages(gameChannelId: number): Promise<ChannelPackage[]> {
  const res = await request<{ items: ChannelPackage[] }>(`/api/admin/game-channels/${gameChannelId}/packages`);
  return res.items ?? [];
}

// POST /game-channels/{gameChannelId}/packages — 新建渠道包（channel.write）
export function createChannelPackage(
  gameChannelId: number,
  payload: CreateChannelPackageRequest
): Promise<ChannelPackage> {
  return request<ChannelPackage>(`/api/admin/game-channels/${gameChannelId}/packages`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

// PATCH /channel-packages/{packageId} — 改包（channel.write）
export function updateChannelPackage(
  packageId: number,
  payload: UpdateChannelPackageRequest
): Promise<ChannelPackage> {
  return request<ChannelPackage>(`/api/admin/channel-packages/${packageId}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

// GET /game-channels/{gameChannelId}/login-config — 渠道登录配置与模板（channel.read）
export function getLoginConfig(gameChannelId: number): Promise<ChannelLoginConfigResponse> {
  return request<ChannelLoginConfigResponse>(`/api/admin/game-channels/${gameChannelId}/login-config`);
}

// PUT /game-channels/{gameChannelId}/login-config — 渠道登录整体 upsert（channel.write）
export function putLoginConfig(
  gameChannelId: number,
  payload: PutChannelLoginConfigPayload
): Promise<ChannelLoginConfigResponse> {
  return request<ChannelLoginConfigResponse>(`/api/admin/game-channels/${gameChannelId}/login-config`, {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

// GET /game-channels/{gameChannelId}/plugins — 列渠道实例可接入插件（plugin.read）
export async function listGameChannelPlugins(gameChannelId: number): Promise<GameChannelPluginItem[]> {
  const res = await request<{ items: GameChannelPluginItem[] }>(`/api/admin/game-channels/${gameChannelId}/plugins`);
  return res.items ?? [];
}

// POST /game-channels/{gameChannelId}/plugins — 勾选/配置插件（plugin.write）
export function upsertGameChannelPlugin(
  gameChannelId: number,
  payload: UpsertGameChannelPluginPayload
): Promise<GameChannelPluginItem> {
  return request<GameChannelPluginItem>(`/api/admin/game-channels/${gameChannelId}/plugins`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

// PATCH /game-channel-plugins/{id} — 修改单插件配置（plugin.write）
export function patchGameChannelPlugin(
  id: number,
  payload: PatchGameChannelPluginPayload
): Promise<GameChannelPluginItem> {
  return request<GameChannelPluginItem>(`/api/admin/game-channel-plugins/${id}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

// GET /channel-packages/{packageId}/plugins — 渠道包插件覆盖列表（plugin.read）
export async function listChannelPackagePlugins(packageId: number): Promise<ChannelPackagePluginItem[]> {
  const res = await request<{ items: ChannelPackagePluginItem[] }>(`/api/admin/channel-packages/${packageId}/plugins`);
  return res.items ?? [];
}

// POST /channel-packages/{packageId}/plugins — 渠道包插件覆盖写入（plugin.write）
export function upsertChannelPackagePlugin(
  packageId: number,
  payload: UpsertChannelPackagePluginPayload
): Promise<ChannelPackagePluginItem> {
  return request<ChannelPackagePluginItem>(`/api/admin/channel-packages/${packageId}/plugins`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}
