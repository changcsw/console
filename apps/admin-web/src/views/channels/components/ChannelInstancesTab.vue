<template>
  <div class="instances">
    <div class="toolbar">
      <div class="toolbar__filters">
        <el-select v-model="filterMarket" class="filter-select" @change="reload(1)">
          <el-option label="全部 Market" value="ALL" />
          <el-option v-for="m in MARKET_OPTIONS" :key="m" :label="m" :value="m" />
        </el-select>
        <el-select
          v-model="filterChannelId"
          class="filter-select"
          placeholder="渠道"
          clearable
          filterable
          @change="reload(1)"
        >
          <el-option
            v-for="c in availableChannels"
            :key="c.channelId"
            :label="c.channelName"
            :value="c.channelId"
          />
        </el-select>
        <el-select v-model="filterCompatible" class="filter-select" placeholder="兼容状态" clearable @change="reload(1)">
          <el-option label="兼容" value="true" />
          <el-option label="不兼容" value="false" />
        </el-select>
        <el-select v-model="filterConfigStatus" class="filter-select" placeholder="配置状态" clearable @change="reload(1)">
          <el-option v-for="opt in CONFIG_STATUS_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <el-checkbox v-model="showHidden" @change="reload(1)">显示隐藏项</el-checkbox>
      </div>
      <div class="toolbar__right">
        <el-button v-perm="'channel.write'" type="primary" @click="openCreate()">新建渠道实例</el-button>
      </div>
    </div>

    <ChannelInstanceTable
      :items="rows"
      :loading="loading"
      :available-channels="availableChannels"
      @detail="openDetail"
      @copy="openCopy"
      @hide="onHide"
      @unhide="onUnhide"
    />

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

    <CreateMarketChannelDrawer
      :open="createOpen"
      :game-id="gameId"
      :available-channels="availableChannels"
      :existing-items="allInstancesForCreate"
      :default-market="createDefaultMarket"
      :preset-channel-id="createPresetChannel"
      :preset-mode="createPresetMode"
      @close="createOpen = false"
      @created="onCreated"
    />

    <ChannelInstanceDetailDrawer :open="detailOpen" :game-channel-id="activeId" @close="detailOpen = false" @changed="onChanged" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { ApiError } from "@/api/http";
import {
  hideMarketChannel,
  listGameChannels,
  listMarketChannels,
  unhideMarketChannel,
  type AvailableChannel,
  type ConfigStatus,
  type CreateChannelMode,
  type Market,
  type MarketChannelListItem
} from "@/api/modules/channels";
import ChannelInstanceTable from "./ChannelInstanceTable.vue";
import CreateMarketChannelDrawer from "./CreateMarketChannelDrawer.vue";
import ChannelInstanceDetailDrawer from "./ChannelInstanceDetailDrawer.vue";
import { CONFIG_STATUS_OPTIONS, MARKET_OPTIONS } from "../constants";

const props = defineProps<{
  gameId: string;
}>();

const rows = ref<MarketChannelListItem[]>([]);
const allInstancesForCreate = ref<MarketChannelListItem[]>([]);
const availableChannels = ref<AvailableChannel[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);

const filterMarket = ref<Market | "ALL">("ALL");
const filterChannelId = ref<string>("");
const filterCompatible = ref<"" | "true" | "false">("");
const filterConfigStatus = ref<ConfigStatus | "">("");
const showHidden = ref(false);

const createOpen = ref(false);
const createDefaultMarket = ref<Market | "">("");
const createPresetChannel = ref<string>("");
const createPresetMode = ref<CreateChannelMode>("empty");

const detailOpen = ref(false);
const activeId = ref<number | null>(null);

async function reload(targetPage = page.value) {
  loading.value = true;
  try {
    const res = await listMarketChannels(props.gameId, {
      market: filterMarket.value === "ALL" ? undefined : filterMarket.value,
      channelId: filterChannelId.value || undefined,
      compatible: filterCompatible.value === "" ? undefined : filterCompatible.value === "true",
      hidden: showHidden.value ? undefined : false,
      configStatus: filterConfigStatus.value || undefined,
      page: targetPage,
      pageSize: pageSize.value
    });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载渠道实例失败");
  } finally {
    loading.value = false;
  }
}

async function loadAvailableChannels() {
  try {
    availableChannels.value = await listGameChannels(props.gameId);
  } catch {
    availableChannels.value = [];
  }
}

async function loadAllInstancesForCreate() {
  const pageSizeForCreate = 200;
  let cursor = 1;
  const collected: MarketChannelListItem[] = [];
  try {
    while (true) {
      const res = await listMarketChannels(props.gameId, {
        hidden: undefined,
        page: cursor,
        pageSize: pageSizeForCreate
      });
      collected.push(...res.items);
      if (collected.length >= res.total || res.items.length === 0) {
        break;
      }
      cursor += 1;
    }
    allInstancesForCreate.value = collected;
  } catch {
    allInstancesForCreate.value = rows.value;
  }
}

function openCreate() {
  createDefaultMarket.value = filterMarket.value === "ALL" ? "" : filterMarket.value;
  createPresetChannel.value = "";
  createPresetMode.value = "empty";
  createOpen.value = true;
}

function openCopy(row: MarketChannelListItem) {
  createDefaultMarket.value = "";
  createPresetChannel.value = row.channelId;
  createPresetMode.value = "copy";
  createOpen.value = true;
}

function openDetail(row: MarketChannelListItem) {
  activeId.value = row.gameChannelId;
  detailOpen.value = true;
}

function onCreated() {
  void reload(1);
  void loadAllInstancesForCreate();
}

function onChanged() {
  void reload();
  void loadAllInstancesForCreate();
}

async function onHide(row: MarketChannelListItem) {
  try {
    await hideMarketChannel(row.gameChannelId);
    ElMessage.success("已隐藏");
    void reload();
    void loadAllInstancesForCreate();
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "隐藏失败");
  }
}

async function onUnhide(row: MarketChannelListItem) {
  try {
    await unhideMarketChannel(row.gameChannelId);
    ElMessage.success("已恢复");
    void reload();
    void loadAllInstancesForCreate();
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "恢复失败");
  }
}

onMounted(() => {
  void loadAvailableChannels();
  void loadAllInstancesForCreate();
  void reload(1);
});
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}

.toolbar__filters {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.toolbar__right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.filter-select {
  width: 150px;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
