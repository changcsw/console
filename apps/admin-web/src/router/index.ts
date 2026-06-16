import { createRouter, createWebHistory } from "vue-router";
import { routes } from "./routes";

const router = createRouter({
  history: createWebHistory(),
  routes
});

router.beforeEach((to) => {
  document.title = `${String(to.meta.title ?? "Console")} - ${import.meta.env.VITE_APP_TITLE ?? "Publishing Console"}`;
});

export default router;

