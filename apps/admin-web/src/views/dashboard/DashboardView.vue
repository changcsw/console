<template>
  <div class="page-shell">
    <PageCard title="工作台" description="第一版工作台聚焦于环境感知、待处理配置和同步风险。">
      <el-row :gutter="16">
        <el-col :md="8" :sm="24">
          <div class="metric">
            <strong>环境同步</strong>
            <span>Sandbox -> Production 差异确认</span>
          </div>
        </el-col>
        <el-col :md="8" :sm="24">
          <div class="metric">
            <strong>收银台模板</strong>
            <span>汇率更新默认人工确认</span>
          </div>
        </el-col>
        <el-col :md="8" :sm="24">
          <div class="metric">
            <strong>渠道策略</strong>
            <span>联运渠道和小游戏策略分层配置</span>
          </div>
        </el-col>
      </el-row>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import PageCard from "@/components/page/PageCard.vue";
</script>

<style scoped>
.metric {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 16px;
  border-radius: 14px;
  background: #f8fbfd;
  border: 1px solid #e3ebf3;
}

.metric strong {
  font-size: 16px;
}

.metric span {
  color: var(--text-subtle);
}
</style>

