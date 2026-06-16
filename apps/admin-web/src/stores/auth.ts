import { defineStore } from "pinia";

interface CurrentUser {
  userId: number;
  displayName: string;
  roles: string[];
  permissions: string[];
}

export const useAuthStore = defineStore("auth", {
  state: () => ({
    accessToken: "",
    refreshToken: "",
    currentUser: null as CurrentUser | null
  }),
  actions: {
    setSession(payload: { accessToken: string; refreshToken?: string; currentUser: CurrentUser }) {
      this.accessToken = payload.accessToken;
      this.refreshToken = payload.refreshToken ?? "";
      this.currentUser = payload.currentUser;
    },
    clearSession() {
      this.accessToken = "";
      this.refreshToken = "";
      this.currentUser = null;
    }
  }
});

