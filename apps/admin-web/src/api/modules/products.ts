import { request } from "@/api/http";

export type ConfigStatus = "empty" | "invalid" | "valid";
export type OverrideMode = "default" | "override";
export type RoundingMode = "half_up" | "floor" | "ceil" | "truncate";

export interface Paginated<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

export interface ProductItem {
  id: number;
  env: string;
  gameId: string;
  productId: string;
  productName: string;
  baseAmountMinor: number;
  baseCurrency: string;
  baseAmountDisplay: string;
  priceId: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface ListProductsQuery {
  page?: number;
  pageSize?: number;
  sort?: string;
  enabled?: boolean;
  keyword?: string;
}

export interface CreateProductRequest {
  productId: string;
  productName: string;
  baseCurrency: string;
  baseAmountMinor?: number;
  baseAmount?: string | number;
  priceId: string;
  enabled?: boolean;
}

export interface UpdateProductRequest {
  productName?: string;
  baseCurrency?: string;
  baseAmountMinor?: number;
  baseAmount?: string | number;
  priceId?: string;
  enabled?: boolean;
}

export interface PackageProductItem {
  productId: string;
  productName: string;
  enabled: boolean;
  base: {
    productId: string;
    priceId: string;
    baseAmountMinor: number;
    baseCurrency: string;
  };
  productIdMode: OverrideMode;
  productIdOverride: string;
  priceIdMode: OverrideMode;
  priceIdOverride: string;
  effective: {
    productId: string;
    priceId: string;
  };
}

export interface PutPackageProductsItem {
  productId: string;
  enabled?: boolean;
  productIdMode?: OverrideMode;
  productIdOverride?: string;
  priceIdMode?: OverrideMode;
  priceIdOverride?: string;
}

export interface PutPackageProductsRequest {
  items: PutPackageProductsItem[];
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
  scope?: "client" | "server" | "both";
  options?: TemplateFieldOption[];
}

export interface TemplateFileField {
  key: string;
  accept?: string[];
  maxSizeKB?: number;
}

export interface IapTemplate {
  templateVersion: string;
  formSchema: TemplateField[];
  secretFields: string[];
  fileFields: TemplateFileField[];
  validationRules: Record<string, unknown>;
}

export interface IapConfig {
  enabled: boolean;
  configStatus: ConfigStatus;
  configJson: Record<string, unknown>;
  lastCheckAt: string | null;
  lastCheckMessage: string;
}

export interface GameChannelIapConfigResponse {
  gameChannelId: number;
  channelId: string;
  template: IapTemplate;
  config: IapConfig;
}

export interface UpsertIapConfigRequest {
  enabled?: boolean;
  configJson: Record<string, unknown>;
}

export interface PackageIapOverrideResponse {
  packageId: number;
  packageCode: string;
  channelId: string;
  template: IapTemplate;
  baseConfig: IapConfig;
  override: IapConfig;
}

export interface CurrencySpec {
  currencyCode: string;
  currencyName: string;
  decimalPlaces: number;
  minAmountMinor: number;
  roundingMode: RoundingMode;
  enabled: boolean;
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

export function listProducts(gameId: string, query: ListProductsQuery = {}): Promise<Paginated<ProductItem>> {
  return request<Paginated<ProductItem>>(`/api/admin/games/${enc(gameId)}/products${buildQuery({ ...query })}`);
}

export function createProduct(gameId: string, payload: CreateProductRequest): Promise<ProductItem> {
  return request<ProductItem>(`/api/admin/games/${enc(gameId)}/products`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function updateProduct(productId: string, gameId: string, payload: UpdateProductRequest): Promise<ProductItem> {
  return request<ProductItem>(`/api/admin/products/${enc(productId)}${buildQuery({ gameId })}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

export async function getPackageProducts(packageId: number): Promise<PackageProductItem[]> {
  const res = await request<{ items: PackageProductItem[] }>(`/api/admin/channel-packages/${packageId}/products`);
  return res.items ?? [];
}

export async function putPackageProducts(packageId: number, payload: PutPackageProductsRequest): Promise<PackageProductItem[]> {
  const res = await request<{ items: PackageProductItem[] }>(`/api/admin/channel-packages/${packageId}/products`, {
    method: "PUT",
    body: JSON.stringify(payload)
  });
  return res.items ?? [];
}

export function getGameChannelIapConfig(gameChannelId: number): Promise<GameChannelIapConfigResponse> {
  return request<GameChannelIapConfigResponse>(`/api/admin/game-channels/${gameChannelId}/iap-config`);
}

export function putGameChannelIapConfig(gameChannelId: number, payload: UpsertIapConfigRequest): Promise<IapConfig> {
  return request<IapConfig>(`/api/admin/game-channels/${gameChannelId}/iap-config`, {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}

export function getPackageIapOverride(packageId: number): Promise<PackageIapOverrideResponse> {
  return request<PackageIapOverrideResponse>(`/api/admin/channel-packages/${packageId}/iap-override`);
}

export function putPackageIapOverride(packageId: number, payload: UpsertIapConfigRequest): Promise<IapConfig> {
  return request<IapConfig>(`/api/admin/channel-packages/${packageId}/iap-override`, {
    method: "PUT",
    body: JSON.stringify(payload)
  });
}
