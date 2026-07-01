import { request } from "@/api/http";

export type SyncSection =
  | "game"
  | "markets"
  | "legal"
  | "channels"
  | "packages"
  | "products"
  | "cashier"
  | "payments"
  | "config";

export type SyncOp = "add" | "update" | "delete";
export type SyncJobStatus = "previewed" | "succeeded" | "failed";

export interface SectionSummary {
  add: number;
  update: number;
  delete: number;
}

export interface DiffChange {
  op: SyncOp;
  entityType: string;
  entityKey: string;
  fieldName: string;
  sandboxValue: unknown;
  productionValue: unknown;
  masked: boolean;
}

export interface DiffSection {
  section: SyncSection;
  summary: SectionSummary;
  dependencies: string[];
  changes: DiffChange[];
}

export interface SyncPreviewResponse {
  gameId: string;
  sourceEnv: string;
  targetEnv: string;
  sourceHash: string;
  targetHashBefore: string;
  hasDiff: boolean;
  baselineToken: string;
  previewedAt: string;
  expiresAt: string;
  sections: DiffSection[];
}

export interface SyncPreviewPayload {
  sections?: SyncSection[];
  includeDeletes?: boolean;
}

export interface SkippedDeleteItem {
  section: SyncSection;
  entityKey: string;
  reason: string;
}

export interface ExecuteSkipped {
  deletes: SkippedDeleteItem[];
  unselectedSections: SyncSection[];
}

export interface SyncExecutePayload {
  selectedSections: SyncSection[];
  baselineToken: string;
  includeDeletes?: boolean;
  operatorNote?: string;
}

export interface SyncExecuteResponse {
  syncJobId: string;
  gameId: string;
  sourceEnv: string;
  targetEnv: string;
  status: SyncJobStatus;
  selectedSections: SyncSection[];
  includeDeletes: boolean;
  sourceHash: string;
  targetHashBefore: string;
  targetHashAfter: string;
  appliedSummary: Partial<Record<SyncSection, SectionSummary>>;
  skipped: ExecuteSkipped;
  executedAt: string;
}

export interface SyncJobErrorSummary {
  code: string;
  message: string;
  details: Array<Record<string, unknown>>;
}

export interface SyncJobListItem {
  syncJobId: string;
  gameId: string;
  sourceEnv: string;
  targetEnv: string;
  status: SyncJobStatus;
  selectedSections: SyncSection[];
  includeDeletes: boolean;
  operatorId: number;
  operatorName?: string;
  operatorNote: string;
  sourceHash: string;
  targetHashBefore: string;
  targetHashAfter: string;
  executedAt: string | null;
  createdAt: string;
  appliedSummary?: Partial<Record<SyncSection, SectionSummary>>;
  skipped?: ExecuteSkipped;
  errorSummary?: SyncJobErrorSummary;
}

export interface ListSyncJobsQuery {
  status?: SyncJobStatus;
  page?: number;
  pageSize?: number;
  sort?: string;
}

export interface SyncJobsPage {
  items: SyncJobListItem[];
  page: number;
  pageSize: number;
  total: number;
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

export async function previewSyncSections(
  gameId: string,
  payload: SyncPreviewPayload = {}
): Promise<SyncPreviewResponse> {
  return request<SyncPreviewResponse>(`/api/admin/games/${encodeURIComponent(gameId)}/sync/preview`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function executeSyncSections(gameId: string, payload: SyncExecutePayload): Promise<SyncExecuteResponse> {
  return request<SyncExecuteResponse>(`/api/admin/games/${encodeURIComponent(gameId)}/sync/execute`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function listSyncJobs(gameId: string, query: ListSyncJobsQuery = {}): Promise<SyncJobsPage> {
  return request<SyncJobsPage>(
    `/api/admin/games/${encodeURIComponent(gameId)}/sync-jobs${buildQuery({
      page: query.page,
      pageSize: query.pageSize,
      sort: query.sort,
      status: query.status
    })}`
  );
}
