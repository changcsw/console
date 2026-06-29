import { defineStore } from "pinia";
import type { RouteRecordRaw } from "vue-router";

const SUPER_ADMIN_ROLE = "super_admin";

export const usePermissionStore = defineStore("permission", {
  state: () => ({
    permissions: new Set<string>(),
    roles: [] as string[]
  }),
  getters: {
    isSuperAdmin: (state) => state.roles.includes(SUPER_ADMIN_ROLE),
    permissionList: (state) => Array.from(state.permissions)
  },
  actions: {
    setFromUser(payload: { roles?: string[]; permissions?: string[] }) {
      this.roles = payload.roles ?? [];
      this.permissions = new Set(payload.permissions ?? []);
    },
    clear() {
      this.roles = [];
      this.permissions = new Set();
    },
    /** super_admin 短路放行；否则按权限码集合判断 */
    hasPerm(code?: string): boolean {
      if (!code) {
        return true;
      }
      if (this.isSuperAdmin) {
        return true;
      }
      return this.permissions.has(code);
    },
    hasAnyPerm(codes: string[]): boolean {
      if (this.isSuperAdmin) {
        return true;
      }
      return codes.some((code) => this.permissions.has(code));
    },
    /** 按 meta.perm 过滤可见的路由/菜单项 */
    filterRoutes(routes: RouteRecordRaw[]): RouteRecordRaw[] {
      return routes.filter((route) => {
        if (route.meta?.hidden) {
          return false;
        }
        const perm = route.meta?.perm as string | undefined;
        return this.hasPerm(perm);
      });
    }
  }
});
