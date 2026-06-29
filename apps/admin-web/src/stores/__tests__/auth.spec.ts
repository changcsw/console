import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";

const loginApi = vi.fn();
const feishuCallbackApi = vi.fn();
const refreshApi = vi.fn();
const logoutApi = vi.fn();
const getMeApi = vi.fn();

vi.mock("@/api/modules/auth", () => ({
  login: (...args: unknown[]) => loginApi(...args),
  feishuCallback: (...args: unknown[]) => feishuCallbackApi(...args),
  refreshToken: (...args: unknown[]) => refreshApi(...args),
  logout: (...args: unknown[]) => logoutApi(...args),
  getMe: (...args: unknown[]) => getMeApi(...args)
}));

import { useAuthStore } from "@/stores/auth";
import { usePermissionStore } from "@/stores/permission";

const STORAGE_KEY = "admin-auth";

function future(ms: number): string {
  return new Date(Date.now() + ms).toISOString();
}

describe("auth store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.removeItem(STORAGE_KEY);
    vi.clearAllMocks();
    vi.useRealTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  test("login stores session, persists, and feeds permission store", async () => {
    loginApi.mockResolvedValue({
      accessToken: "acc",
      refreshToken: "ref",
      expiresAt: future(30 * 60 * 1000),
      user: { userId: 1, userName: "admin", displayName: "Admin", roles: ["super_admin"], permissions: ["system.read"] }
    });
    const auth = useAuthStore();
    await auth.login("admin", "secret");

    expect(auth.isAuthenticated).toBe(true);
    expect(auth.accessToken).toBe("acc");
    expect(usePermissionStore().hasPerm("system.read")).toBe(true);

    const persisted = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(persisted.accessToken).toBe("acc");
    expect(persisted.user.userName).toBe("admin");
  });

  test("clearSession wipes tokens, storage and permissions", async () => {
    loginApi.mockResolvedValue({
      accessToken: "acc",
      refreshToken: "ref",
      expiresAt: future(30 * 60 * 1000),
      user: { userId: 1, userName: "admin", displayName: "Admin", roles: ["super_admin"], permissions: ["system.read"] }
    });
    const auth = useAuthStore();
    await auth.login("admin", "secret");
    auth.clearSession();

    expect(auth.isAuthenticated).toBe(false);
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
    expect(usePermissionStore().permissionList).toEqual([]);
  });

  test("refresh updates access token and keeps old refresh when rotation absent", async () => {
    const auth = useAuthStore();
    auth.refreshToken = "ref-old";
    auth.expiresAt = future(30 * 60 * 1000);
    refreshApi.mockResolvedValue({ accessToken: "acc-new", refreshToken: "", expiresAt: future(30 * 60 * 1000) });

    await auth.refresh();
    expect(auth.accessToken).toBe("acc-new");
    expect(auth.refreshToken).toBe("ref-old");
    expect(refreshApi).toHaveBeenCalledWith("ref-old");
  });

  test("refresh throws when no refresh token present", async () => {
    const auth = useAuthStore();
    auth.refreshToken = "";
    await expect(auth.refresh()).rejects.toThrow();
  });

  test("near-expiry session schedules an automatic refresh", async () => {
    vi.useFakeTimers();
    refreshApi.mockResolvedValue({ accessToken: "acc-2", refreshToken: "ref-2", expiresAt: future(30 * 60 * 1000) });
    const auth = useAuthStore();
    // expiresAt 仅剩 70s，安全窗口 60s → 约 10s 后触发续期
    auth.setSession({
      accessToken: "acc-1",
      refreshToken: "ref-1",
      expiresAt: future(70 * 1000),
      user: { userId: 1, userName: "a", displayName: "A", roles: [], permissions: [] }
    });
    expect(refreshApi).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(11 * 1000);
    expect(refreshApi).toHaveBeenCalledTimes(1);
  });

  test("logout calls api then clears session", async () => {
    logoutApi.mockResolvedValue({ loggedOut: true });
    const auth = useAuthStore();
    auth.refreshToken = "ref";
    auth.accessToken = "acc";
    await auth.logout();
    expect(logoutApi).toHaveBeenCalledWith("ref");
    expect(auth.isAuthenticated).toBe(false);
  });

  test("logout clears session even if api fails", async () => {
    logoutApi.mockRejectedValue(new Error("network"));
    const auth = useAuthStore();
    auth.refreshToken = "ref";
    auth.accessToken = "acc";
    await auth.logout();
    expect(auth.isAuthenticated).toBe(false);
  });

  test("loadMe hydrates user, permissions and environment", async () => {
    getMeApi.mockResolvedValue({
      userId: 9,
      userName: "ops",
      displayName: "Ops",
      email: "",
      status: "active",
      roles: ["ops"],
      permissions: ["audit.read"],
      identities: [{ identityType: "feishu", identityKey: "on_****abcd" }],
      environment: "sandbox"
    });
    const auth = useAuthStore();
    auth.accessToken = "acc";
    const me = await auth.loadMe();
    expect(me?.userName).toBe("ops");
    expect(usePermissionStore().hasPerm("audit.read")).toBe(true);
  });
});
