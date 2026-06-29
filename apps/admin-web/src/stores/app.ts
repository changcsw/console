import { defineStore } from "pinia";

export type EnvironmentName = "develop" | "sandbox" | "production" | string;

export const useAppStore = defineStore("app", {
  state: () => ({
    environment: (import.meta.env.VITE_APP_ENV || "develop") as EnvironmentName
  }),
  getters: {
    isProduction: (state) => state.environment === "production"
  },
  actions: {
    setEnvironment(next: EnvironmentName) {
      if (next && next !== this.environment) {
        this.environment = next;
      }
    }
  }
});
