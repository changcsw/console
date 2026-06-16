<template>
  <div class="page-shell">
    <PageCard title="系统设置" description="后台管理员登录、权限、基础字典与配置模板入口。">
      <el-descriptions :column="1" border>
        <el-descriptions-item label="后台登录">密码 + 飞书</el-descriptions-item>
        <el-descriptions-item label="模板驱动表单">账号认证 / 渠道登录 / IAP / 收银台 Provider</el-descriptions-item>
        <el-descriptions-item label="环境标记">顶部常驻显示 develop / sandbox / production</el-descriptions-item>
      </el-descriptions>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import PageCard from "@/components/page/PageCard.vue";
</script>
