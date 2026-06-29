import { ElMessage } from "element-plus";
import { useAuthStore } from "@/stores/auth";
import { useAppStore } from "@/stores/app";
import router from "@/router";

export interface ApiErrorDetail {
  field?: string;
  message?: string;
  [key: string]: unknown;
}

export class ApiError extends Error {
  code: string;
  status: number;
  details: ApiErrorDetail[];

  constructor(status: number, code: string, message: string, details: ApiErrorDetail[] = []) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export interface RequestOptions extends RequestInit {
  /** 是否注入 Authorization 头，默认 true */
  auth?: boolean;
  /** 跳过 401 自动续期（用于 refresh 自身），默认 false */
  skipRefresh?: boolean;
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

let refreshPromise: Promise<boolean> | null = null;

function ensureRefreshed(): Promise<boolean> {
  const auth = useAuthStore();
  if (!auth.refreshToken) {
    return Promise.resolve(false);
  }
  if (!refreshPromise) {
    refreshPromise = auth
      .refresh()
      .then(() => true)
      .catch(() => false)
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
}

function redirectToLogin() {
  const current = router.currentRoute.value;
  if (current.path === "/login") {
    return;
  }
  void router.push({ path: "/login", query: { redirect: current.fullPath } });
}

async function doRequest<T>(path: string, options: RequestOptions | undefined, allowRefresh: boolean): Promise<T> {
  const auth = useAuthStore();
  const baseURL = import.meta.env.VITE_API_BASE_URL || "";

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options?.headers as Record<string, string>) ?? {})
  };
  if (options?.auth !== false && auth.accessToken) {
    headers.Authorization = `Bearer ${auth.accessToken}`;
  }

  const response = await fetch(`${baseURL}${path}`, { ...options, headers });

  const environment = response.headers.get("X-Environment");
  if (environment) {
    useAppStore().setEnvironment(environment);
  }

  let body: unknown = null;
  const text = await response.text();
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = null;
    }
  }

  if (response.ok) {
    const envelope = body as { data?: T } | null;
    return (envelope && "data" in envelope ? (envelope.data as T) : (body as T)) as T;
  }

  const errorBody = (body as { error?: { code?: string; message?: string; details?: ApiErrorDetail[] } } | null)?.error;
  const code = errorBody?.code ?? statusToCode(response.status);
  const message = errorBody?.message ?? `请求失败：${response.status}`;
  const details = errorBody?.details ?? [];

  if (response.status === 401 && allowRefresh && options?.skipRefresh !== true) {
    const ok = await ensureRefreshed();
    if (ok) {
      return doRequest<T>(path, options, false);
    }
    auth.clearSession();
    redirectToLogin();
    throw new ApiError(response.status, code, message, details);
  }

  if (response.status === 403) {
    ElMessage.error(message || "无权限执行该操作");
  }

  throw new ApiError(response.status, code, message, details);
}

export async function request<T>(path: string, options?: RequestOptions): Promise<T> {
  return doRequest<T>(path, options, true);
}
