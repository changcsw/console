import { createApp } from "vue";
import { createPinia } from "pinia";
import ElementPlus from "element-plus";
import "element-plus/dist/index.css";
import App from "./App.vue";
import router from "./router";
import "./styles/index.css";
import { registerPermDirective } from "@/directives/perm";
import { useAuthStore } from "@/stores/auth";

const app = createApp(App);

app.use(createPinia());
app.use(router);
app.use(ElementPlus);
registerPermDirective(app);

// 从本地存储恢复会话：安排临期续期，并异步同步当前用户/环境
const auth = useAuthStore();
if (auth.isAuthenticated) {
  auth.scheduleAutoRefresh();
  void auth.loadMe().catch(() => {
    /* 令牌失效时由 http 拦截器统一处理跳转 */
  });
}

app.mount("#app");
