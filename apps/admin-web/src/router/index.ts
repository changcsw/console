import { createRouter, createWebHistory } from "vue-router";
import { routes } from "./routes";
import { useAuthStore } from "@/stores/auth";
import { usePermissionStore } from "@/stores/permission";

const router = createRouter({
  history: createWebHistory(),
  routes
});

router.beforeEach((to) => {
  document.title = `${String(to.meta.title ?? "Console")} - ${import.meta.env.VITE_APP_TITLE ?? "Publishing Console"}`;

  const auth = useAuthStore();
  const permission = usePermissionStore();

  if (to.meta.public) {
    // 已登录用户访问登录页时直接进工作台
    if (to.path === "/login" && auth.isAuthenticated) {
      return { path: "/dashboard" };
    }
    return true;
  }

  if (!auth.isAuthenticated) {
    return { path: "/login", query: { redirect: to.fullPath } };
  }

  const perm = to.meta.perm;
  if (perm && !permission.hasPerm(perm)) {
    return { path: "/403" };
  }

  return true;
});

export default router;
