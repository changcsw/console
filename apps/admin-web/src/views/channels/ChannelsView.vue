<template>
  <div class="page-shell">
    <PageCard title="渠道管理" description="维护渠道定义、策略、联运能力与小游戏差异。">
      <el-table :data="rows" width="100%">
        <el-table-column prop="channelId" label="渠道 ID" min-width="180" />
        <el-table-column prop="channelName" label="渠道名称" min-width="180" />
        <el-table-column prop="channelType" label="渠道类型" min-width="120" />
        <el-table-column prop="loginMode" label="登录模式" min-width="140" />
        <el-table-column prop="paymentMode" label="支付模式" min-width="140" />
      </el-table>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import PageCard from "@/components/page/PageCard.vue";

const rows = [
  { channelId: "google", channelName: "Google", channelType: "store", loginMode: "account_system", paymentMode: "hybrid" },
  { channelId: "huawei_cn", channelName: "华为联运", channelType: "oem", loginMode: "channel_only", paymentMode: "channel_only" },
  { channelId: "wechat_mini_game", channelName: "微信小游戏", channelType: "mini_game", loginMode: "account_system", paymentMode: "hybrid" }
];
</script>

