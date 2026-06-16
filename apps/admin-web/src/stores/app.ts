import { defineStore } from "pinia";

export const useAppStore = defineStore("app", {
  state: () => ({
    environment: import.meta.env.VITE_APP_ENV || "develop"
  })
});

