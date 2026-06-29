import { request } from "@/api/http";

export interface AuthUser {
  userId: number;
  userName: string;
  displayName: string;
  roles: string[];
  permissions: string[];
}

export interface LoginResult {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
  user: AuthUser;
}

export interface RefreshResult {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
}

export interface MeIdentity {
  identityType: string;
  identityKey: string;
}

export interface MeResult {
  userId: number;
  userName: string;
  displayName: string;
  email: string;
  status: string;
  roles: string[];
  permissions: string[];
  identities: MeIdentity[];
  environment: string;
}

export interface LoginPayload {
  userName: string;
  password: string;
}

export interface FeishuCallbackPayload {
  code: string;
  state?: string;
  redirectUri?: string;
}

export function login(payload: LoginPayload): Promise<LoginResult> {
  return request<LoginResult>("/api/admin/auth/login", {
    method: "POST",
    auth: false,
    skipRefresh: true,
    body: JSON.stringify(payload)
  });
}

export function feishuCallback(payload: FeishuCallbackPayload): Promise<LoginResult> {
  return request<LoginResult>("/api/admin/auth/feishu/callback", {
    method: "POST",
    auth: false,
    skipRefresh: true,
    body: JSON.stringify(payload)
  });
}

export function refreshToken(token: string): Promise<RefreshResult> {
  return request<RefreshResult>("/api/admin/auth/refresh", {
    method: "POST",
    auth: false,
    skipRefresh: true,
    body: JSON.stringify({ refreshToken: token })
  });
}

export function logout(token?: string): Promise<{ loggedOut: boolean }> {
  return request<{ loggedOut: boolean }>("/api/admin/auth/logout", {
    method: "POST",
    skipRefresh: true,
    body: JSON.stringify(token ? { refreshToken: token } : {})
  });
}

export function getMe(): Promise<MeResult> {
  return request<MeResult>("/api/admin/me");
}
