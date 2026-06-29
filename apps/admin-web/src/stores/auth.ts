import { defineStore } from "pinia";
import {
  login as loginApi,
  feishuCallback as feishuCallbackApi,
  refreshToken as refreshApi,
  logout as logoutApi,
  getMe as getMeApi,
  type AuthUser,
  type FeishuCallbackPayload,
  type MeResult
} from "@/api/modules/auth";
import { usePermissionStore } from "@/stores/permission";
import { useAppStore } from "@/stores/app";

const STORAGE_KEY = "admin-auth";
/** access 临期提前续期的安全窗口（毫秒） */
const REFRESH_SKEW_MS = 60_000;

interface PersistedAuth {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
  user: AuthUser | null;
}

let refreshTimer: ReturnType<typeof setTimeout> | null = null;

function loadPersisted(): PersistedAuth {
  if (typeof localStorage === "undefined") {
    return { accessToken: "", refreshToken: "", expiresAt: "", user: null };
  }
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      return JSON.parse(raw) as PersistedAuth;
    }
  } catch {
    /* ignore corrupt storage */
  }
  return { accessToken: "", refreshToken: "", expiresAt: "", user: null };
}

export const useAuthStore = defineStore("auth", {
  state: () => {
    const persisted = loadPersisted();
    return {
      accessToken: persisted.accessToken,
      refreshToken: persisted.refreshToken,
      expiresAt: persisted.expiresAt,
      user: persisted.user as AuthUser | null
    };
  },
  getters: {
    isAuthenticated: (state) => Boolean(state.accessToken)
  },
  actions: {
    persist() {
      if (typeof localStorage === "undefined") {
        return;
      }
      const payload: PersistedAuth = {
        accessToken: this.accessToken,
        refreshToken: this.refreshToken,
        expiresAt: this.expiresAt,
        user: this.user
      };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
    },
    setSession(payload: { accessToken: string; refreshToken: string; expiresAt: string; user?: AuthUser }) {
      this.accessToken = payload.accessToken;
      this.refreshToken = payload.refreshToken;
      this.expiresAt = payload.expiresAt;
      if (payload.user) {
        this.user = payload.user;
        usePermissionStore().setFromUser({ roles: payload.user.roles, permissions: payload.user.permissions });
      }
      this.persist();
      this.scheduleAutoRefresh();
    },
    clearSession() {
      this.accessToken = "";
      this.refreshToken = "";
      this.expiresAt = "";
      this.user = null;
      usePermissionStore().clear();
      if (refreshTimer) {
        clearTimeout(refreshTimer);
        refreshTimer = null;
      }
      if (typeof localStorage !== "undefined") {
        localStorage.removeItem(STORAGE_KEY);
      }
    },
    scheduleAutoRefresh() {
      if (typeof window === "undefined") {
        return;
      }
      if (refreshTimer) {
        clearTimeout(refreshTimer);
        refreshTimer = null;
      }
      if (!this.expiresAt || !this.refreshToken) {
        return;
      }
      const expiry = new Date(this.expiresAt).getTime();
      if (Number.isNaN(expiry)) {
        return;
      }
      const delay = Math.max(expiry - Date.now() - REFRESH_SKEW_MS, 0);
      refreshTimer = setTimeout(() => {
        void this.refresh().catch(() => {
          this.clearSession();
        });
      }, delay);
    },
    async login(userName: string, password: string) {
      const result = await loginApi({ userName, password });
      this.setSession(result);
      return result;
    },
    async feishuLogin(payload: FeishuCallbackPayload) {
      const result = await feishuCallbackApi(payload);
      this.setSession(result);
      return result;
    },
    async refresh() {
      if (!this.refreshToken) {
        throw new Error("missing refresh token");
      }
      const result = await refreshApi(this.refreshToken);
      this.accessToken = result.accessToken;
      this.refreshToken = result.refreshToken || this.refreshToken;
      this.expiresAt = result.expiresAt;
      this.persist();
      this.scheduleAutoRefresh();
      return result;
    },
    async logout() {
      try {
        if (this.refreshToken) {
          await logoutApi(this.refreshToken);
        }
      } catch {
        /* 登出失败也清本地会话 */
      } finally {
        this.clearSession();
      }
    },
    async loadMe(): Promise<MeResult | null> {
      if (!this.accessToken) {
        return null;
      }
      const me = await getMeApi();
      this.user = {
        userId: me.userId,
        userName: me.userName,
        displayName: me.displayName,
        roles: me.roles,
        permissions: me.permissions
      };
      usePermissionStore().setFromUser({ roles: me.roles, permissions: me.permissions });
      if (me.environment) {
        useAppStore().setEnvironment(me.environment);
      }
      this.persist();
      return me;
    }
  }
});
