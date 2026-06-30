import { request } from "@/api/http";

export type AuditEnvironment = "develop" | "sandbox" | "production" | string;

export interface Paginated<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

export interface AuditOperator {
  id: string;
  userName: string;
  displayName: string;
}

export interface AuditRequestMeta {
  ip?: string;
  userAgent?: string;
  requestId?: string;
  method?: string;
  path?: string;
}

export interface AuditDetailPayload {
  summary?: string;
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
  changed?: string[];
  extra?: Record<string, unknown>;
  request?: AuditRequestMeta;
  [key: string]: unknown;
}

export interface AuditLogItem {
  id: string;
  actorId: string;
  operator: AuditOperator | null;
  action: string;
  resourceType: string;
  resourceId: string;
  env: AuditEnvironment;
  detail: AuditDetailPayload;
  createdAt: string;
}

export interface ListAuditLogsQuery {
  env?: AuditEnvironment;
  action?: string;
  resourceType?: string;
  resourceId?: string;
  operator?: string;
  operatorKeyword?: string;
  from?: string;
  to?: string;
  keyword?: string;
  page?: number;
  pageSize?: number;
  sort?: "createdAt" | "-createdAt";
}

export interface AuditLogFacets {
  envs: string[];
  actions: string[];
  resourceTypes: string[];
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

export function listAuditLogs(query: ListAuditLogsQuery = {}): Promise<Paginated<AuditLogItem>> {
  return request<Paginated<AuditLogItem>>(`/api/admin/audit-logs${buildQuery({ ...query })}`);
}

export function getAuditLogDetail(id: string): Promise<AuditLogItem> {
  return request<AuditLogItem>(`/api/admin/audit-logs/${encodeURIComponent(id)}`);
}

export function listAuditLogFacets(): Promise<AuditLogFacets> {
  return request<AuditLogFacets>("/api/admin/audit-logs/facets");
}
