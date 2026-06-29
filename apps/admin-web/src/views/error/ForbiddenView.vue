<template>
  <main class="forbidden-page">
    <div class="forbidden-card">
      <div class="forbidden-code">403</div>
      <h1>无访问权限</h1>
      <p>当前账号未被授予访问该页面所需的权限码，请联系管理员分配相应角色。</p>
      <el-button type="primary" @click="goHome">返回工作台</el-button>
    </div>
  </main>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";

const router = useRouter();

function goHome() {
  void router.push("/dashboard");
}
</script>

<style scoped>
.forbidden-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
}

.forbidden-card {
  text-align: center;
  max-width: 420px;
}

.forbidden-code {
  font-size: 72px;
  font-weight: 800;
  color: var(--danger);
  line-height: 1;
}

.forbidden-card h1 {
  margin: 12px 0 8px;
}

.forbidden-card p {
  color: var(--text-subtle);
  margin: 0 0 20px;
  line-height: 1.6;
}
</style>
