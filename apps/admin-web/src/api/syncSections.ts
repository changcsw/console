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

export interface SyncPreviewSection {
  section: SyncSection;
}

export interface SyncPreviewResponse {
  gameId: string;
  hasDiff: boolean;
  sections: SyncPreviewSection[];
}

export interface SyncExecutePayload {
  selected_sections: SyncSection[];
}

export async function previewSyncSections(
  gameId: string,
  payload: Partial<SyncExecutePayload> = {}
): Promise<SyncPreviewResponse> {
  return request<SyncPreviewResponse>(`/api/admin/games/${gameId}/sync/preview`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export async function executeSyncSections(gameId: string, payload: SyncExecutePayload): Promise<Record<string, unknown>> {
  return request<Record<string, unknown>>(`/api/admin/games/${gameId}/sync/execute`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}
