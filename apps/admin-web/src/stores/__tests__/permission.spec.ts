import { beforeEach, describe, expect, test } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import type { RouteRecordRaw } from "vue-router";
import { usePermissionStore } from "@/stores/permission";

describe("permission store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  test("hasPerm: empty code means no requirement -> allow", () => {
    const store = usePermissionStore();
    store.setFromUser({ roles: [], permissions: [] });
    expect(store.hasPerm()).toBe(true);
    expect(store.hasPerm("")).toBe(true);
  });

  test("hasPerm: matches only granted codes", () => {
    const store = usePermissionStore();
    store.setFromUser({ roles: ["editor"], permissions: ["system.read", "role.write"] });
    expect(store.hasPerm("system.read")).toBe(true);
    expect(store.hasPerm("permission.write")).toBe(false);
  });

  test("super_admin short-circuits any permission code", () => {
    const store = usePermissionStore();
    store.setFromUser({ roles: ["super_admin"], permissions: [] });
    expect(store.isSuperAdmin).toBe(true);
    expect(store.hasPerm("anything.dangerous")).toBe(true);
    expect(store.hasAnyPerm(["nope.nope"])).toBe(true);
  });

  test("hasAnyPerm: true when at least one code granted", () => {
    const store = usePermissionStore();
    store.setFromUser({ roles: [], permissions: ["role.write"] });
    expect(store.hasAnyPerm(["admin_user.write", "role.write"])).toBe(true);
    expect(store.hasAnyPerm(["x.y", "z.w"])).toBe(false);
  });

  test("clear resets roles and permissions", () => {
    const store = usePermissionStore();
    store.setFromUser({ roles: ["super_admin"], permissions: ["a.b"] });
    store.clear();
    expect(store.isSuperAdmin).toBe(false);
    expect(store.permissionList).toEqual([]);
    expect(store.hasPerm("a.b")).toBe(false);
  });

  test("filterRoutes hides routes lacking perm or marked hidden", () => {
    const store = usePermissionStore();
    store.setFromUser({ roles: [], permissions: ["system.read"] });
    const routes = [
      { path: "/system", meta: { perm: "system.read" } },
      { path: "/audit", meta: { perm: "audit.read" } },
      { path: "/secret", meta: { hidden: true } },
      { path: "/public", meta: {} }
    ] as unknown as RouteRecordRaw[];
    const visible = store.filterRoutes(routes).map((r) => r.path);
    expect(visible).toEqual(["/system", "/public"]);
  });

  test("super_admin sees every non-hidden route via filterRoutes", () => {
    const store = usePermissionStore();
    store.setFromUser({ roles: ["super_admin"], permissions: [] });
    const routes = [
      { path: "/system", meta: { perm: "system.read" } },
      { path: "/audit", meta: { perm: "audit.read" } },
      { path: "/secret", meta: { hidden: true } }
    ] as unknown as RouteRecordRaw[];
    expect(store.filterRoutes(routes).map((r) => r.path)).toEqual(["/system", "/audit"]);
  });
});
