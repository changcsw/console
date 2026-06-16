<template>
  <div class="page-shell">
    <PageCard title="游戏管理" description="从这里开始管理游戏、市场、协议链接、渠道、商品与同步。">
      <div class="toolbar">
        <el-button type="primary">新建游戏</el-button>
        <PageStatusTag tone="warning" label="后续接真实接口后展示列表数据" />
      </div>

      <el-table :data="rows" width="100%">
        <el-table-column prop="gameId" label="Game ID" min-width="140" />
        <el-table-column prop="name" label="游戏名称" min-width="200" />
        <el-table-column prop="alias" label="代号" min-width="140" />
        <el-table-column prop="defaultMarketCode" label="默认市场" min-width="120" />
        <el-table-column prop="status" label="状态" min-width="120" />
      </el-table>
    </PageCard>

    <PageCard title="详情页规划" description="建议在真实开发时落成 tabs 结构。">
      <el-tabs>
        <el-tab-pane label="基础信息" />
        <el-tab-pane label="市场与法务" />
        <el-tab-pane label="渠道" />
        <el-tab-pane label="渠道包" />
        <el-tab-pane label="商品" />
        <el-tab-pane label="自有账号认证" />
        <el-tab-pane label="渠道登录" />
        <el-tab-pane label="IAP" />
        <el-tab-pane label="收银台" />
        <el-tab-pane label="同步记录" />
      </el-tabs>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import PageCard from "@/components/page/PageCard.vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";

const rows = ref([
  {
    gameId: "100001",
    name: "Project A",
    alias: "pa",
    defaultMarketCode: "GLOBAL",
    status: "active"
  }
]);
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
</style>

