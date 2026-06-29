<template>
  <main class="login-page">
    <section class="login-card">
      <div class="login-card__hero">
        <span class="login-card__eyebrow">Publishing Console</span>
        <h1>后台登录</h1>
        <p>发行管理后台 · 仅限授权管理员访问，与玩家登录体系完全隔离。</p>
        <EnvironmentBadge :environment="app.environment" />
      </div>

      <el-tabs v-model="activeTab" class="login-tabs">
        <!-- 密码登录 -->
        <el-tab-pane label="密码登录" name="password">
          <el-form label-position="top" @submit.prevent="onPasswordLogin">
            <el-form-item label="用户名">
              <el-input
                v-model="userName"
                placeholder="请输入用户名"
                autocomplete="username"
                aria-label="userName"
                @input="passwordError = ''"
              />
            </el-form-item>
            <el-form-item label="密码">
              <el-input
                v-model="password"
                type="password"
                placeholder="请输入密码"
                show-password
                autocomplete="current-password"
                aria-label="password"
                @keyup.enter="onPasswordLogin"
                @input="passwordError = ''"
              />
            </el-form-item>
            <p v-if="passwordError" class="login-error" role="alert">{{ passwordError }}</p>
            <el-button type="primary" :loading="passwordLoading" class="login-submit" @click="onPasswordLogin">
              密码登录
            </el-button>
          </el-form>
        </el-tab-pane>

        <!-- 飞书登录 -->
        <el-tab-pane label="飞书登录" name="feishu">
          <el-form label-position="top" @submit.prevent="onFeishuLogin">
            <el-form-item label="飞书授权码">
              <el-input
                v-model="feishuCode"
                placeholder="开发环境可填 mock:用户名"
                aria-label="feishuCode"
                @keyup.enter="onFeishuLogin"
                @input="feishuError = ''"
              />
            </el-form-item>
            <p class="login-hint">未绑定飞书身份的账号无法登录，请先在「系统设置」中为管理员绑定飞书身份。</p>
            <p v-if="feishuError" class="login-error" role="alert">{{ feishuError }}</p>
            <el-button type="primary" :loading="feishuLoading" class="login-submit" @click="onFeishuLogin">
              飞书登录
            </el-button>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </section>
  </main>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { useAppStore } from "@/stores/app";
import { useAuthStore } from "@/stores/auth";
import { ApiError } from "@/api/http";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";

const app = useAppStore();
const auth = useAuthStore();
const router = useRouter();
const route = useRoute();

const activeTab = ref<"password" | "feishu">("password");

const userName = ref("");
const password = ref("");
const passwordLoading = ref(false);
const passwordError = ref("");

const feishuCode = ref("");
const feishuLoading = ref(false);
const feishuError = ref("");

function redirectTarget(): string {
  const redirect = route.query.redirect;
  if (typeof redirect === "string" && redirect.startsWith("/")) {
    return redirect;
  }
  return "/dashboard";
}

function handleError(err: unknown, setInline: (msg: string) => void) {
  if (err instanceof ApiError) {
    if (err.code === "UNAUTHENTICATED" || err.code === "VALIDATION_FAILED") {
      setInline(err.message || "用户名或密码错误");
      return;
    }
    // 飞书不可用 / 服务端错误等
    ElMessage.error(err.message || "登录失败，请稍后再试");
    return;
  }
  // 网络异常
  ElMessage.error("网络异常，请检查连接后重试");
}

async function onPasswordLogin() {
  passwordError.value = "";
  if (!userName.value.trim() || !password.value) {
    passwordError.value = "请输入用户名与密码";
    return;
  }
  passwordLoading.value = true;
  try {
    await auth.login(userName.value.trim(), password.value);
    await router.push(redirectTarget());
  } catch (err) {
    handleError(err, (msg) => (passwordError.value = msg));
  } finally {
    passwordLoading.value = false;
  }
}

async function onFeishuLogin() {
  feishuError.value = "";
  if (!feishuCode.value.trim()) {
    feishuError.value = "请输入飞书授权码";
    return;
  }
  feishuLoading.value = true;
  try {
    await auth.feishuLogin({ code: feishuCode.value.trim() });
    await router.push(redirectTarget());
  } catch (err) {
    handleError(err, (msg) => (feishuError.value = msg));
  } finally {
    feishuLoading.value = false;
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(circle at top left, rgba(22, 163, 74, 0.22), transparent 28%),
    radial-gradient(circle at bottom right, rgba(37, 99, 235, 0.16), transparent 30%),
    linear-gradient(180deg, #f7faf8 0%, #edf2f7 100%);
}

.login-card {
  width: min(100%, 440px);
  padding: 28px;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.96);
  border: 1px solid #d9e2ec;
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.12);
}

.login-card__hero {
  margin-bottom: 12px;
}

.login-card__eyebrow {
  display: inline-block;
  padding: 6px 10px;
  border-radius: 999px;
  background: #dff5e7;
  color: #14532d;
  font-size: 12px;
  font-weight: 700;
}

.login-card h1 {
  margin: 14px 0 8px;
  font-size: 30px;
}

.login-card p {
  margin: 0 0 12px;
  color: var(--text-subtle);
  line-height: 1.6;
}

.login-submit {
  width: 100%;
}

.login-error {
  color: var(--danger);
  font-size: 13px;
  margin: 0 0 12px;
}

.login-hint {
  color: var(--text-subtle);
  font-size: 12px;
  margin: 0 0 12px;
}
</style>
