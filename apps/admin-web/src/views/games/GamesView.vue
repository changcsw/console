<template>
  <div class="page-shell">
    <PageCard title="游戏管理" description="发行后台根聚合：管理游戏基础信息、发行市场与法务链接；下游渠道/商品/同步从游戏详情进入。">
      <div class="toolbar">
        <div class="toolbar__filters">
          <el-input
            v-model="keyword"
            placeholder="名称 / 代号 / Game ID"
            clearable
            class="filter-keyword"
            @keyup.enter="reload(1)"
            @clear="reload(1)"
          />
          <el-select v-model="statusFilter" placeholder="状态" clearable class="filter-select" @change="reload(1)">
            <el-option v-for="opt in STATUS_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
          <el-select v-model="marketFilter" placeholder="市场" clearable class="filter-select" @change="reload(1)">
            <el-option v-for="market in MARKET_OPTIONS" :key="market" :label="market" :value="market" />
          </el-select>
          <el-button @click="reload(1)">查询</el-button>
        </div>
        <div class="toolbar__right">
          <EnvironmentBadge :environment="app.environment" />
          <el-button v-perm="'game.write'" type="primary" @click="createOpen = true">新建游戏</el-button>
        </div>
      </div>

      <el-table v-loading="loading" :data="rows" border @row-click="goDetail">
        <el-table-column prop="gameId" label="Game ID" min-width="120" />
        <el-table-column prop="name" label="游戏名称" min-width="180" show-overflow-tooltip />
        <el-table-column prop="alias" label="代号" min-width="120" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <PageStatusTag :tone="statusMeta(row.status).tone" :label="statusMeta(row.status).label" />
          </template>
        </el-table-column>
        <el-table-column prop="defaultMarketCode" label="默认市场" width="110" />
        <el-table-column label="市场" min-width="180">
          <template #default="{ row }">
            <span v-if="row.marketCodes?.length" class="market-tags">
              <el-tag
                v-for="market in row.marketCodes"
                :key="market"
                size="small"
                :type="market === row.defaultMarketCode ? 'success' : 'info'"
              >
                {{ market }}
              </el-tag>
            </span>
            <span v-else class="text-muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="goDetail(row)">详情</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="empty-state">
            <p class="empty-state__title">暂无游戏</p>
            <p class="empty-state__hint">点击右上角「新建游戏」开始配置第一款游戏。</p>
          </div>
        </template>
      </el-table>

      <div class="pager">
        <el-pagination
          background
          layout="prev, pager, next, total"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="reload"
        />
      </div>
    </PageCard>

    <CreateGameDrawer v-model:open="createOpen" @created="onCreated" />

    <!-- gameSecret 一次性明文展示（仅创建后此一次） -->
    <el-dialog v-model="secretDialogVisible" title="游戏密钥（仅此一次）" width="520px" :close-on-click-modal="false">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        title="请立即妥善保存 gameSecret，关闭后将不再以明文展示。"
        class="secret-alert"
      />
      <div class="secret-row">
        <span class="secret-row__label">Game ID</span>
        <code class="secret-row__value">{{ createdGame?.gameId }}</code>
      </div>
      <div class="secret-row">
        <span class="secret-row__label">Game Secret</span>
        <code class="secret-row__value">{{ createdGame?.gameSecret }}</code>
        <el-button size="small" @click="copySecret">复制</el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="closeSecretDialog">我已保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import EnvironmentBadge from "@/components/page/EnvironmentBadge.vue";
import { useAppStore } from "@/stores/app";
import { ApiError } from "@/api/http";
import { listGames, type GameDetail, type GameListItem, type GameStatus, type Market } from "@/api/modules/games";
import CreateGameDrawer from "./components/CreateGameDrawer.vue";
import { MARKET_OPTIONS, STATUS_OPTIONS, statusMeta } from "./constants";

const router = useRouter();
const app = useAppStore();

const rows = ref<GameListItem[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);

const keyword = ref("");
const statusFilter = ref<GameStatus | "">("");
const marketFilter = ref<Market | "">("");

const createOpen = ref(false);
const secretDialogVisible = ref(false);
const createdGame = ref<GameDetail | null>(null);

function formatTime(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

async function reload(targetPage = page.value) {
  loading.value = true;
  try {
    const res = await listGames({
      page: targetPage,
      pageSize: pageSize.value,
      keyword: keyword.value || undefined,
      status: statusFilter.value || undefined,
      marketCode: marketFilter.value || undefined
    });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    if (err instanceof ApiError) {
      ElMessage.error(err.message || "加载游戏列表失败");
    } else {
      ElMessage.error("加载游戏列表失败");
    }
  } finally {
    loading.value = false;
  }
}

function goDetail(row: GameListItem) {
  void router.push({ name: "game-detail", params: { gameId: row.gameId } });
}

function onCreated(game: GameDetail) {
  createdGame.value = game;
  secretDialogVisible.value = true;
  void reload(1);
}

async function copySecret() {
  if (!createdGame.value) {
    return;
  }
  try {
    await navigator.clipboard.writeText(createdGame.value.gameSecret);
    ElMessage.success("已复制到剪贴板");
  } catch {
    ElMessage.warning("复制失败，请手动选择文本");
  }
}

function closeSecretDialog() {
  secretDialogVisible.value = false;
  createdGame.value = null;
}

onMounted(() => {
  void reload(1);
});
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.toolbar__filters {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}

.toolbar__right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.filter-keyword {
  width: 240px;
}

.filter-select {
  width: 130px;
}

.market-tags {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 4px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
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

.secret-alert {
  margin-bottom: 16px;
}

.secret-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
}

.secret-row__label {
  width: 110px;
  color: var(--text-subtle);
  font-size: 13px;
}

.secret-row__value {
  flex: 1;
  font-family: monospace;
  background: #f1f5f9;
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  word-break: break-all;
}

:deep(.el-table__row) {
  cursor: pointer;
}
</style>
