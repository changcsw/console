import { defineStore } from "pinia";

export const usePermissionStore = defineStore("permission", {
  state: () => ({
    permissions: [] as string[]
  }),
  actions: {
    setPermissions(nextPermissions: string[]) {
      this.permissions = nextPermissions;
    },
    has(permission: string) {
      return this.permissions.includes(permission);
    }
  }
});

