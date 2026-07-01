<template>
  <section>
    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="payWayId" label="Pay Way ID" min-width="180" />
      <el-table-column prop="payWayName" label="名称" min-width="180" />
      <el-table-column prop="payWayType" label="类型" width="120" />
      <el-table-column label="启用" width="100">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? "是" : "否" }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="sort" label="排序" width="100" />
    </el-table>
    <div class="pager">
      <el-pagination
        background
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="load"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { ApiError } from "@/api/http";
import { listPayWays, type PayWayItem } from "@/api/modules/payment";

const loading = ref(false);
const rows = ref<PayWayItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);

async function load(targetPage = page.value) {
  loading.value = true;
  try {
    const res = await listPayWays({ page: targetPage, pageSize: pageSize.value });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载支付方式失败");
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void load(1);
});
</script>

<style scoped>
.pager {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}
</style>
