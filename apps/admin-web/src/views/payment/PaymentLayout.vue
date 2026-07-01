<template>
  <PageCard title="支付配置中心" description="管理支付方式、PSP、结算主体与商户账户（平台级基础数据）。">
    <div class="payment-layout__head">
      <el-tabs :model-value="activePath" @tab-change="onTabChange">
        <el-tab-pane label="支付方式" name="/payment/pay-ways" />
        <el-tab-pane label="支付提供商" name="/payment/providers" />
        <el-tab-pane label="结算主体" name="/payment/billing-subjects" />
        <el-tab-pane label="商户账户" name="/payment/merchant-accounts" />
      </el-tabs>
      <EnvironmentBadge :environment="app.environment" />
    </div>
    <RouterView />
  </PageCard>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import PageCard from "@/components/page/PageCard.vue";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import { useAppStore } from "@/stores/app";

const route = useRoute();
const router = useRouter();
const app = useAppStore();

const activePath = computed(() => {
  const path = route.path;
  if (path.startsWith("/payment/")) {
    return path;
  }
  return "/payment/pay-ways";
});

function onTabChange(name: string | number) {
  if (typeof name === "string") {
    void router.push(name);
  }
}
</script>

<style scoped>
.payment-layout__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
</style>
