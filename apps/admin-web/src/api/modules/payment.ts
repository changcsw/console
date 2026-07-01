import { ApiError, request } from "@/api/http";

export type PayWayType = "card" | "wallet" | "platform" | "local";
export type ProviderKind = "aggregator" | "gateway" | "wallet_direct";
export type MarketCode = "GLOBAL" | "JP" | "KR" | "SEA" | "HMT" | "CN" | "*";

export interface Paginated<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

export interface PayWayItem {
  payWayId: string;
  payWayName: string;
  payWayType: PayWayType;
  enabled: boolean;
  sort: number;
}

export interface ProviderItem {
  providerId: string;
  providerName: string;
  providerKind: ProviderKind;
  enabled: boolean;
  sort: number;
}

export interface BillingSubjectItem {
  subjectId: string;
  subjectName: string;
  legalEntityName: string;
  enabled: boolean;
}

export interface MerchantAccountItem {
  merchantAccountId: string;
  providerId: string;
  subjectId: string;
  merchantId: string;
  merchantName: string;
  configJson: Record<string, unknown>;
  secret: string;
  enabled: boolean;
}

export interface TemplateFieldOption {
  label: string;
  value: string | number | boolean;
}

export interface TemplateField {
  key: string;
  label: string;
  component: "input" | "password" | "textarea" | "number" | "select" | "switch" | "file" | "json";
  required?: boolean;
  placeholder?: string;
  order?: number;
  group?: string;
  scope?: "client" | "server" | "both";
  options?: TemplateFieldOption[];
}

export interface TemplateFileField {
  key: string;
  accept?: string[];
  maxSizeKB?: number;
}

export interface ProviderTemplate {
  templateVersion: string;
  formSchema: TemplateField[];
  secretFields: string[];
  fileFields: TemplateFileField[];
  validationRules: Record<string, unknown>;
}

export interface RouteSelector {
  packageCode: string | null;
  channelId: string | null;
  marketCode: MarketCode;
  countryCode: string;
  currency: string;
}

export interface GamePaymentRoute {
  id: number;
  selector: RouteSelector;
  providerId: string;
  merchantAccountId: string;
  priority: number;
  enabled: boolean;
  hasDisabledReference?: boolean;
  disabledRefs?: string[];
  payWayEnabled?: boolean;
  providerEnabled?: boolean;
  merchantAccountEnabled?: boolean;
  channelEnabled?: boolean;
  packageEnabled?: boolean;
}

export interface PaymentRouteGroup {
  payWayId: string;
  payWayName: string;
  payWayType: PayWayType;
  routes: GamePaymentRoute[];
}

export interface GamePaymentRoutesResponse {
  gameId: string;
  env: string;
  groups: PaymentRouteGroup[];
}

export interface SaveGamePaymentRouteItem {
  marketCode?: MarketCode;
  countryCode?: string;
  currency?: string;
  channelId?: string | null;
  packageCode?: string | null;
  payWayId: string;
  providerId: string;
  merchantAccountId: string;
  priority?: number;
  enabled?: boolean;
}

export interface SaveGamePaymentRoutesPayload {
  items: SaveGamePaymentRouteItem[];
}

export interface RouteConflictDetail {
  kind: "duplicate_priority" | "duplicate_selector";
  leftRouteId?: number;
  rightRouteId?: number;
  leftIndex?: number;
  rightIndex?: number;
  [key: string]: unknown;
}

export interface BillingSubjectPayload {
  subjectId: string;
  subjectName: string;
  legalEntityName: string;
  enabled?: boolean;
}

export interface CreateMerchantAccountPayload {
  merchantAccountId: string;
  providerId: string;
  subjectId: string;
  merchantId: string;
  merchantName: string;
  configJson?: Record<string, unknown>;
  secrets?: Record<string, string>;
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

export function listPayWays(query: { page?: number; pageSize?: number; enabled?: boolean; type?: PayWayType } = {}) {
  return request<Paginated<PayWayItem>>(`/api/admin/pay-ways${buildQuery(query)}`);
}

export function listProviders(query: { page?: number; pageSize?: number; enabled?: boolean; kind?: ProviderKind } = {}) {
  return request<Paginated<ProviderItem>>(`/api/admin/cashier/providers${buildQuery(query)}`);
}

// 选 provider → 拉该 provider enabled 最新 template_version 四件套（compact §前端）。
// 后端端点：GET /api/admin/cashier/providers/{providerId}/template（权限 payment.read）。
// provider 无可用模板时后端返回 404，前端降级为仅基础字段（见 MerchantAccountsView）。
export function getProviderTemplate(providerId: string) {
  return request<ProviderTemplate>(`/api/admin/cashier/providers/${enc(providerId)}/template`);
}

export function listBillingSubjects(query: { page?: number; pageSize?: number; enabled?: boolean; keyword?: string } = {}) {
  return request<Paginated<BillingSubjectItem>>(`/api/admin/billing-subjects${buildQuery(query)}`);
}

export function createBillingSubject(payload: BillingSubjectPayload) {
  return request<BillingSubjectItem>("/api/admin/billing-subjects", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function listMerchantAccounts(
  query: {
    page?: number;
    pageSize?: number;
    providerId?: string;
    subjectId?: string;
    enabled?: boolean;
    keyword?: string;
  } = {}
) {
  return request<Paginated<MerchantAccountItem>>(`/api/admin/cashier/merchant-accounts${buildQuery(query)}`);
}

export function createMerchantAccount(payload: CreateMerchantAccountPayload) {
  return request<MerchantAccountItem>("/api/admin/cashier/merchant-accounts", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function getGamePaymentRoutes(gameId: string): Promise<GamePaymentRoutesResponse> {
  return request<GamePaymentRoutesResponse>(`/api/admin/games/${enc(gameId)}/payment-routes`);
}

export async function saveGamePaymentRoutes(gameId: string, payload: SaveGamePaymentRoutesPayload): Promise<GamePaymentRoutesResponse> {
  return request<GamePaymentRoutesResponse>(`/api/admin/games/${enc(gameId)}/payment-routes`, {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

export function isRouteConflictError(err: unknown): err is ApiError {
  return err instanceof ApiError && err.code === "ROUTE_CONFLICT";
}
