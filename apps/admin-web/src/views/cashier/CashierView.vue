<template>
  <div class="page-shell">
    <PageCard title="收银台管理" description="价格模板、价格矩阵、汇率提醒和路由切换都在这里收口。">
      <el-row :gutter="16">
        <el-col :md="12" :sm="24">
          <div class="notice">
            <strong>汇率同步策略</strong>
            <span>默认人工确认，运营审核后应用。</span>
          </div>
        </el-col>
        <el-col :md="12" :sm="24">
          <div class="notice">
            <strong>支付路由策略</strong>
            <span>同一支付方式可按优先级切换 PSP，客户端无感。</span>
          </div>
        </el-col>
      </el-row>
    </PageCard>

    <PageCard title="价格模板矩阵输入规则" description="金额输入必须受 currency specs 驱动。">
      <el-table :data="currencyRows" width="100%">
        <el-table-column prop="currencyCode" label="币种" min-width="120" />
        <el-table-column prop="decimalPlaces" label="小数位" min-width="120" />
        <el-table-column prop="minAmountMinor" label="最小金额(minor)" min-width="160" />
        <el-table-column prop="roundingMode" label="舍入规则" min-width="140" />
      </el-table>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import PageCard from "@/components/page/PageCard.vue";

const currencyRows = [
  { currencyCode: "USD", decimalPlaces: 2, minAmountMinor: 1, roundingMode: "half_up" },
  { currencyCode: "JPY", decimalPlaces: 0, minAmountMinor: 1, roundingMode: "half_up" }
];
</script>

<style scoped>
.notice {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 16px;
  border-radius: 14px;
  background: #f8fbfd;
  border: 1px solid #e3ebf3;
}

.notice span {
  color: var(--text-subtle);
}
</style>

