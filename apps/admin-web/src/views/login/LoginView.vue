<template>
  <main class="login-page">
    <section class="login-card">
      <div class="login-card__hero">
        <span class="login-card__eyebrow">Publishing Console</span>
        <h1>后台登录</h1>
        <p>这里保留密码登录与飞书登录的入口位置，后续接真实认证流程。</p>
      </div>

      <el-form label-position="top">
        <el-form-item label="用户名">
          <el-input placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input type="password" placeholder="请输入密码" show-password />
        </el-form-item>
        <div class="login-card__actions">
          <el-button type="primary">密码登录</el-button>
          <el-button plain>飞书登录</el-button>
        </div>
      </el-form>
    </section>
  </main>
</template>

<script setup lang="ts"></script>

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
  margin-bottom: 20px;
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
  margin: 0;
  color: var(--text-subtle);
  line-height: 1.6;
}

.login-card__actions {
  display: flex;
  gap: 12px;
}
</style>
