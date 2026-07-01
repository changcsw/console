import { ApiError, request } from "@/api/http";
import { useAuthStore } from "@/stores/auth";

export type SnapshotStatus = "draft" | "published";

export interface SnapshotListItem {
  id: number;
  configVersion: string;
  status: SnapshotStatus;
  fileHash: string;
  generatedAt: string;
  publishedAt: string | null;
}

export interface SnapshotListResponse {
  items: SnapshotListItem[];
  page: number;
  pageSize: number;
  total: number;
}

export interface GenerateSnapshotResponse {
  id: number;
  configVersion: string;
  fileHash: string;
  status: SnapshotStatus;
  generatedAt: string;
}

export interface SnapshotPreviewPayload {
  schemaVersion?: string;
  gameId?: string;
  generatedAt?: string;
  markets?: Record<string, unknown>;
  [key: string]: unknown;
}

export interface DownloadSnapshotResponse {
  fileName: string;
  blob: Blob;
  payload: SnapshotPreviewPayload | null;
}

export interface ListSnapshotsQuery {
  page?: number;
  pageSize?: number;
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

function statusToCode(status: number): string {
  switch (status) {
    case 400:
      return "VALIDATION_FAILED";
    case 401:
      return "UNAUTHENTICATED";
    case 403:
      return "FORBIDDEN";
    case 404:
      return "NOT_FOUND";
    case 409:
      return "CONFLICT";
    default:
      return "INTERNAL";
  }
}

function parseFileName(disposition: string | null, fallback: string): string {
  if (!disposition) {
    return fallback;
  }
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]);
    } catch {
      return utf8Match[1];
    }
  }
  const plainMatch = disposition.match(/filename="?([^";]+)"?/i);
  if (plainMatch?.[1]) {
    return plainMatch[1];
  }
  return fallback;
}

// POST /games/{gameId}/config-snapshots/generate — 生成 draft 快照（snapshot.generate）
export function generateGameSnapshot(gameId: string): Promise<GenerateSnapshotResponse> {
  return request<GenerateSnapshotResponse>(`/api/admin/games/${encodeURIComponent(gameId)}/config-snapshots/generate`, {
    method: "POST",
    body: JSON.stringify({})
  });
}

// GET /games/{gameId}/config-snapshots — 快照列表（game.read）
export function listGameSnapshots(gameId: string, query: ListSnapshotsQuery = {}): Promise<SnapshotListResponse> {
  return request<SnapshotListResponse>(
    `/api/admin/games/${encodeURIComponent(gameId)}/config-snapshots${buildQuery({
      page: query.page,
      pageSize: query.pageSize
    })}`
  );
}

// POST /game-config-snapshots/{snapshotId}/publish — 发布快照（snapshot.publish）
export function publishGameSnapshot(snapshotId: number): Promise<SnapshotListItem> {
  return request<SnapshotListItem>(`/api/admin/game-config-snapshots/${snapshotId}/publish`, {
    method: "POST",
    body: JSON.stringify({})
  });
}

// GET /game-config-snapshots/{snapshotId}/download — 下载配置快照（game.read）
export async function downloadGameSnapshot(snapshotId: number): Promise<DownloadSnapshotResponse> {
  const auth = useAuthStore();
  const baseURL = import.meta.env.VITE_API_BASE_URL || "";
  const headers: Record<string, string> = {};
  if (auth.accessToken) {
    headers.Authorization = `Bearer ${auth.accessToken}`;
  }
  const response = await fetch(`${baseURL}/api/admin/game-config-snapshots/${snapshotId}/download`, {
    method: "GET",
    headers
  });

  if (!response.ok) {
    let errorBody: { error?: { code?: string; message?: string; details?: Array<Record<string, unknown>> } } = {};
    try {
      errorBody = (await response.json()) as { error?: { code?: string; message?: string; details?: Array<Record<string, unknown>> } };
    } catch {
      errorBody = {};
    }
    const code = errorBody?.error?.code ?? statusToCode(response.status);
    const message = errorBody?.error?.message ?? `请求失败：${response.status}`;
    throw new ApiError(response.status, code, message, errorBody?.error?.details ?? []);
  }

  const blob = await response.blob();
  const fileName = parseFileName(response.headers.get("Content-Disposition"), `game_snapshot_${snapshotId}.json`);
  let payload: SnapshotPreviewPayload | null = null;
  try {
    payload = (await new Response(blob).json()) as SnapshotPreviewPayload;
  } catch {
    payload = null;
  }

  return { fileName, blob, payload };
}
