<template>
  <el-table
    v-loading="loading"
    :data="items"
    border
    row-key="gameChannelId"
    :row-class-name="rowClassName"
    @row-click="(row: MarketChannelListItem) => emit('detail', row)"
  >
    <el-table-column label="Market" width="100">
      <template #default="{ row }">
        <el-tag size="small" type="info">{{ row.market }}</el-tag>
      </template>
    </el-table-column>
    <el-table-column label="渠道" min-width="160">
      <template #default="{ row }">
        <div class="cell-channel">
          <span class="cell-channel__id">{{ channelNameMap[row.channelId] || row.channelId }}</span>
          <span class="cell-channel__sub">{{ row.channelId }}</span>
          <span class="cell-channel__key">{{ row.displayKey }}</span>
        </div>
      </template>
    </el-table-column>
    <el-table-column label="区域" width="100">
      <template #default="{ row }">{{ regionLabel(row.region) }}</template>
    </el-table-column>
    <el-table-column label="状态" min-width="180">
      <template #default="{ row }">
        <div class="cell-status">
          <ChannelInstanceStatusTag
            :config-status="row.configStatus"
            :compatible="row.compatible"
            :hidden="row.hidden"
          />
          <span v-if="!row.compatible" class="cell-status__hint">不兼容当前 market，保留配置不删除</span>
        </div>
      </template>
    </el-table-column>
    <el-table-column label="是否进最终配置" min-width="200">
      <template #default="{ row }">
        <ChannelInstanceRuntimeFlags
          :hidden="row.hidden"
          :compatible="row.compatible"
          :config-status="row.configStatus"
          :included-in-snapshot="row.includedInSnapshot"
          :included-in-sync="row.includedInSync"
          :included-in-runtime-config="row.includedInRuntimeConfig"
        />
      </template>
    </el-table-column>
    <el-table-column label="复制来源" width="110">
      <template #default="{ row }">
        <span v-if="row.copiedFromMarket" class="text-muted">{{ row.copiedFromMarket }}</span>
        <span v-else class="text-muted">—</span>
      </template>
    </el-table-column>
    <el-table-column label="最近更新" width="170">
      <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
    </el-table-column>
    <el-table-column label="操作" width="220" fixed="right">
      <template #default="{ row }">
        <el-button link type="primary" @click.stop="emit('detail', row)">详情</el-button>
        <el-button v-perm="'channel.write'" link type="primary" @click.stop="emit('copy', row)">复制创建</el-button>
        <template v-if="!row.hidden">
          <el-tooltip
            v-if="row.configStatus !== 'valid'"
            content="仅 valid 配置可隐藏；invalid/empty 不得隐藏"
            placement="top"
          >
            <span><el-button link disabled @click.stop>隐藏</el-button></span>
          </el-tooltip>
          <el-popconfirm
            v-else
            title="隐藏后将移出快照/同步/客户端最终配置，可恢复。确认隐藏？"
            width="240"
            @confirm="emit('hide', row)"
          >
            <template #reference>
              <el-button v-perm="'channel.write'" link type="warning" @click.stop>隐藏</el-button>
            </template>
          </el-popconfirm>
        </template>
        <el-popconfirm v-else title="恢复后将按规则重新参与生效集。确认恢复？" width="220" @confirm="emit('unhide', row)">
          <template #reference>
            <el-button v-perm="'channel.write'" link type="primary" @click.stop>恢复</el-button>
          </template>
        </el-popconfirm>
      </template>
    </el-table-column>
    <template #empty>
      <div class="empty-state">
        <p class="empty-state__title">暂无渠道实例</p>
        <p class="empty-state__hint">点击右上角「新建渠道实例」按 market 挂载渠道。</p>
      </div>
    </template>
  </el-table>
</template>

<script setup lang="ts">
import { computed } from "vue";
import ChannelInstanceStatusTag from "./ChannelInstanceStatusTag.vue";
import ChannelInstanceRuntimeFlags from "./ChannelInstanceRuntimeFlags.vue";
import type { AvailableChannel, MarketChannelListItem } from "@/api/modules/channels";
import { regionLabel } from "../constants";

const props = defineProps<{
  items: MarketChannelListItem[];
  availableChannels?: AvailableChannel[];
  loading?: boolean;
}>();

const emit = defineEmits<{
  (e: "detail", row: MarketChannelListItem): void;
  (e: "copy", row: MarketChannelListItem): void;
  (e: "hide", row: MarketChannelListItem): void;
  (e: "unhide", row: MarketChannelListItem): void;
}>();

const channelNameMap = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {};
  for (const channel of props.availableChannels || []) {
    map[channel.channelId] = channel.channelName;
  }
  return map;
});

function rowClassName({ row }: { row: MarketChannelListItem }): string {
  if (row.hidden) {
    return "row-hidden";
  }
  if (!row.compatible) {
    return "row-incompatible";
  }
  return "";
}

function formatTime(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}
</script>

<style scoped>
.cell-channel {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.cell-channel__id {
  font-weight: 600;
}

.cell-channel__key {
  color: var(--text-subtle);
  font-size: 12px;
}

.cell-channel__sub {
  color: var(--text-subtle);
  font-size: 12px;
}

.cell-status {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.cell-status__hint {
  color: #b42318;
  font-size: 12px;
}

.text-muted {
  color: var(--text-subtle);
}

.empty-state {
  padding: 24px 0;
}

.empty-state__title {
  margin: 0;
  font-weight: 600;
}

.empty-state__hint {
  margin: 6px 0 0;
  color: var(--text-subtle);
}

:deep(.el-table__row) {
  cursor: pointer;
}

:deep(.el-table__row.row-incompatible) {
  background: #fff4f3;
}

:deep(.el-table__row.row-hidden) {
  background: #f5f7fa;
  color: var(--text-subtle);
}
</style>
