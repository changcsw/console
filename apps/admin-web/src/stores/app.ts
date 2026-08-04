import { defineStore } from "pinia";

export type EnvironmentName = "develop" | "sandbox" | "production" | string;

export const useAppStore = defineStore("app", {
  state: () => ({
    environment: (import.meta.env.VITE_APP_ENV || "develop") as EnvironmentName,
    /** 服务端实际运行环境（由 /me 或响应头 X-Environment 同步） */
    serverEnvironment: (import.meta.env.VITE_APP_ENV || "develop") as EnvironmentName,
    /** 用户通过顶栏手动切换后，不再被响应头覆盖 */
    environmentOverride: false
  }),
  getters: {
    isProduction: (state) => state.environment === "production"
  },
  actions: {
    setEnvironment(next: EnvironmentName) {
      if (next && next !== this.environment) {
        this.environment = next;
      }
    },
    /** 从服务端同步环境；未手动切换时同时更新展示环境 */
    syncServerEnvironment(next: EnvironmentName) {
      if (!next) {
        return;
      }
      this.serverEnvironment = next;
      if (!this.environmentOverride) {
        this.environment = next;
      }
    },
    /** 顶栏手动切换环境 */
    switchEnvironment(next: EnvironmentName) {
      if (!next) {
        return;
      }
      this.environmentOverride = true;
      this.environment = next;
    },
    resetEnvironmentOverride() {
      this.environmentOverride = false;
      this.environment = this.serverEnvironment;
    }
  }
});
