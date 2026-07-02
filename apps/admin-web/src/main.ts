import { createApp } from "vue";
import { createPinia } from "pinia";
import ElementPlus from "element-plus";
import "element-plus/dist/index.css";
import App from "./App.vue";
import router from "./router";
import "./styles/index.css";
import { registerPermDirective } from "@/directives/perm";
import { useAuthStore } from "@/stores/auth";
import { usePermissionStore } from "@/stores/permission";

const app = createApp(App);

app.use(createPinia());
app.use(router);
app.use(ElementPlus);
registerPermDirective(app);

// 从本地存储恢复会话：安排临期续期，并异步同步当前用户/环境
const auth = useAuthStore();
if (auth.isAuthenticated) {
  // 挂载前用持久会话中的用户权限同步回填 permission store。
  // 路由首个 beforeEach 会同步读取 permission.hasPerm 判定 meta.perm；此前权限仅由异步 loadMe() 回填，
  // 导致直连/刷新任一受保护路由时守卫先于权限就绪 → 误判 /403。此处保证守卫在权限就绪后再判定。
  if (auth.user) {
    usePermissionStore().setFromUser({ roles: auth.user.roles, permissions: auth.user.permissions });
  }
  auth.scheduleAutoRefresh();
  void auth.loadMe().catch(() => {
    /* 令牌失效时由 http 拦截器统一处理跳转 */
  });
}

app.mount("#app");
