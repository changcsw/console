<template>
  <PageCard title="版本列表" description="published 版本只读，需复制为 draft 后再修改。">
    <div class="toolbar">
      <el-button v-perm="'cashier.write'" type="primary" @click="emit('create-version')">新建 draft 版本</el-button>
    </div>

    <el-table :data="versions" border size="small" @row-click="onRowClick">
      <el-table-column label="版本" width="110">
        <template #default="{ row }">
          <strong>v{{ row.version }}</strong>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="sourceType" label="来源" min-width="140" />
      <el-table-column label="发布时间" min-width="170">
        <template #default="{ row }">{{ formatTime(row.publishedAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click.stop="emit('select-version', row.version)">编辑矩阵</el-button>
          <el-button
            v-if="row.status === 'published'"
            v-perm="'cashier.write'"
            link
            type="primary"
            @click.stop="emit('copy-version', row)"
          >
            复制为 draft
          </el-button>
          <el-popconfirm
            v-if="row.status === 'draft'"
            title="确认发布该版本？当前 published 将自动归档。"
            width="260"
            @confirm="emit('publish-version', row)"
          >
            <template #reference>
              <el-button v-perm="'cashier.publish'" link type="success" @click.stop>发布</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
      <template #empty>
        <span class="text-muted">暂无版本，请先创建 draft 版本</span>
      </template>
    </el-table>
  </PageCard>
</template>

<script setup lang="ts">
import PageCard from "@/components/page/PageCard.vue";
import type { CashierTemplateVersion, VersionStatus } from "@/api/modules/cashier";

defineProps<{
  versions: CashierTemplateVersion[];
}>();

const emit = defineEmits<{
  (e: "create-version"): void;
  (e: "copy-version", version: CashierTemplateVersion): void;
  (e: "publish-version", version: CashierTemplateVersion): void;
  (e: "select-version", version: string): void;
}>();

function onRowClick(row: CashierTemplateVersion) {
  emit("select-version", row.version);
}

function statusType(status: VersionStatus): "primary" | "success" | "info" {
  if (status === "published") {
    return "success";
  }
  if (status === "draft") {
    return "primary";
  }
  return "info";
}

function formatTime(value?: string | null): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}
</script>

<style scoped>
.toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}

.text-muted {
  color: var(--text-subtle);
}

:deep(.el-table__row) {
  cursor: pointer;
}
</style>
