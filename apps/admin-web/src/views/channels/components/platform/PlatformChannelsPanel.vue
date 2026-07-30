<template>
  <div class="panel">
    <div class="panel__toolbar">
      <div class="panel__filters">
        <el-input
          v-model="keyword"
          placeholder="渠道 ID / 渠道名"
          clearable
          class="filter-keyword"
          @keyup.enter="reload(1)"
          @clear="reload(1)"
        />
        <el-select v-model="filterRegion" class="filter-select" placeholder="region" clearable @change="reload(1)">
          <el-option v-for="r in CHANNEL_REGION_OPTIONS" :key="r" :label="regionLabel(r)" :value="r" />
        </el-select>
        <el-select v-model="filterChannelType" class="filter-select" placeholder="渠道类型" clearable @change="reload(1)">
          <el-option v-for="t in CHANNEL_TYPE_OPTIONS" :key="t" :label="channelTypeLabel(t)" :value="t" />
        </el-select>
        <el-select v-model="filterEnabled" class="filter-select" placeholder="启用状态" clearable @change="reload(1)">
          <el-option label="已启用" value="true" />
          <el-option label="已停用" value="false" />
        </el-select>
        <el-button @click="reload(1)">查询</el-button>
      </div>
      <el-button v-perm="'platform_channel.write'" type="primary" @click="openCreate">新建渠道</el-button>
    </div>

    <el-table v-loading="loading" :data="rows" border>
      <el-table-column prop="channelId" label="渠道 ID" min-width="140" />
      <el-table-column prop="channelName" label="渠道名" min-width="140" />
      <el-table-column label="类型" width="110">
        <template #default="{ row }">{{ channelTypeLabel(row.channelType) }}</template>
      </el-table-column>
      <el-table-column label="region" width="100">
        <template #default="{ row }">{{ regionLabel(row.region) }}</template>
      </el-table-column>
      <el-table-column label="登录模式" min-width="130">
        <template #default="{ row }">{{ loginModeLabel(row.loginMode) }}</template>
      </el-table-column>
      <el-table-column label="支付模式" min-width="120">
        <template #default="{ row }">{{ paymentModeLabel(row.paymentMode) }}</template>
      </el-table-column>
      <el-table-column label="锁定位" width="140">
        <template #default="{ row }">
          <span v-if="!row.loginLocked && !row.paymentLocked" class="text-muted">无</span>
          <template v-else>
            <PageStatusTag v-if="row.loginLocked" tone="warning" label="登录锁定" />
            <PageStatusTag v-if="row.paymentLocked" tone="warning" label="支付锁定" />
          </template>
        </template>
      </el-table-column>
      <el-table-column label="模版数" width="130">
        <template #default="{ row }">登录 {{ row.loginTemplateCount }} / IAP {{ row.iapTemplateCount }}</template>
      </el-table-column>
      <el-table-column label="启用状态" width="100">
        <template #default="{ row }">
          <PageStatusTag
            :tone="row.enabled ? 'success' : 'neutral'"
            :label="row.enabled ? '已启用' : '已停用'"
          />
        </template>
      </el-table-column>
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" v-perm="'platform_channel.write'" @click="openEdit(row)">编辑</el-button>
        </template>
      </el-table-column>
      <template #empty>
        <span class="text-muted">暂无平台渠道</span>
      </template>
    </el-table>

    <div class="panel__pager">
      <el-pagination
        background
        layout="prev, pager, next, total"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="reload"
      />
    </div>

    <el-drawer v-model="drawerVisible" :title="editing ? '编辑渠道' : '新建渠道'" size="520px">
      <el-form label-position="top">
        <el-form-item label="渠道 ID">
          <el-input
            v-model="form.channelId"
            :disabled="editing"
            placeholder="小写字母/数字/下划线，字母或数字开头，如 huawei_cn"
          />
          <p v-if="editing" class="panel__hint">创建后不可改：渠道 ID 是渠道实例的引用键。</p>
        </el-form-item>
        <el-form-item label="渠道名">
          <el-input v-model="form.channelName" placeholder="1-64 字符，如 华为应用市场" />
        </el-form-item>
        <el-form-item label="渠道类型">
          <el-select v-model="form.channelType" :disabled="editing" class="form-control">
            <el-option v-for="t in CHANNEL_TYPE_OPTIONS" :key="t" :label="channelTypeLabel(t)" :value="t" />
          </el-select>
          <p v-if="editing" class="panel__hint">创建后不可改。</p>
        </el-form-item>
        <el-form-item label="region">
          <el-select v-model="form.region" :disabled="editing" class="form-control">
            <el-option v-for="r in CHANNEL_REGION_OPTIONS" :key="r" :label="regionLabel(r)" :value="r" />
          </el-select>
          <p v-if="editing" class="panel__hint">
            创建后不可改：region 决定与 market 的兼容性，改动会让既有渠道实例集体失配。
          </p>
        </el-form-item>
        <el-form-item label="登录模式">
          <el-select v-model="form.loginMode" class="form-control">
            <el-option v-for="m in LOGIN_MODE_OPTIONS" :key="m" :label="loginModeLabel(m)" :value="m" />
          </el-select>
        </el-form-item>
        <el-form-item label="支付模式">
          <el-select v-model="form.paymentMode" class="form-control">
            <el-option v-for="m in PAYMENT_MODE_OPTIONS" :key="m" :label="paymentModeLabel(m)" :value="m" />
          </el-select>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :max="9999" />
        </el-form-item>
        <el-form-item label="锁定位">
          <div class="form-switches">
            <el-checkbox v-model="form.loginLocked">登录锁定（游戏侧不可改登录模式）</el-checkbox>
            <el-checkbox v-model="form.paymentLocked">支付锁定（游戏侧不可改支付模式）</el-checkbox>
          </div>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
        <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>
      </el-form>
      <template #footer>
        <el-button @click="drawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submitForm">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { ApiError } from "@/api/http";
import {
  listPlatformChannels,
  createPlatformChannel,
  updatePlatformChannel,
  CHANNEL_REGION_OPTIONS,
  CHANNEL_TYPE_OPTIONS,
  LOGIN_MODE_OPTIONS,
  PAYMENT_MODE_OPTIONS,
  type ChannelRegion,
  type ChannelType,
  type LoginMode,
  type PaymentMode,
  type PlatformChannel
} from "@/api/modules/platformChannels";
import { regionLabel } from "../../constants";
import { channelTypeLabel, loginModeLabel, paymentModeLabel } from "./labels";

const rows = ref<PlatformChannel[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);

const keyword = ref("");
const filterRegion = ref<ChannelRegion | "">("");
const filterChannelType = ref<ChannelType | "">("");
const filterEnabled = ref<"true" | "false" | "">("");

const drawerVisible = ref(false);
const editing = ref(false);
const saving = ref(false);
const formError = ref("");

const form = reactive({
  channelId: "",
  channelName: "",
  channelType: "store" as ChannelType,
  region: "overseas" as ChannelRegion,
  loginMode: "channel_only" as LoginMode,
  paymentMode: "channel_only" as PaymentMode,
  sort: 0,
  loginLocked: false,
  paymentLocked: false,
  enabled: true
});

function reportError(err: unknown, fallback: string, setInline?: (msg: string) => void) {
  if (err instanceof ApiError) {
    const msg = err.message || fallback;
    if (setInline && (err.code === "VALIDATION_FAILED" || err.code === "CONFLICT")) {
      setInline(msg);
    } else {
      ElMessage.error(msg);
    }
    return;
  }
  ElMessage.error(fallback);
}

async function reload(targetPage = page.value) {
  loading.value = true;
  try {
    const res = await listPlatformChannels({
      page: targetPage,
      pageSize: pageSize.value,
      keyword: keyword.value || undefined,
      region: filterRegion.value || undefined,
      channelType: filterChannelType.value || undefined,
      enabled: filterEnabled.value === "" ? undefined : filterEnabled.value === "true"
    });
    rows.value = res.items;
    total.value = res.total;
    page.value = res.page;
    pageSize.value = res.pageSize;
  } catch (err) {
    reportError(err, "加载平台渠道列表失败");
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = false;
  formError.value = "";
  Object.assign(form, {
    channelId: "",
    channelName: "",
    channelType: "store" as ChannelType,
    region: "overseas" as ChannelRegion,
    loginMode: "channel_only" as LoginMode,
    paymentMode: "channel_only" as PaymentMode,
    sort: 0,
    loginLocked: false,
    paymentLocked: false,
    enabled: true
  });
  drawerVisible.value = true;
}

function openEdit(row: PlatformChannel) {
  editing.value = true;
  formError.value = "";
  Object.assign(form, {
    channelId: row.channelId,
    channelName: row.channelName,
    channelType: row.channelType,
    region: row.region,
    loginMode: row.loginMode,
    paymentMode: row.paymentMode,
    sort: row.sort,
    loginLocked: row.loginLocked,
    paymentLocked: row.paymentLocked,
    enabled: row.enabled
  });
  drawerVisible.value = true;
}

async function submitForm() {
  formError.value = "";
  saving.value = true;
  try {
    if (editing.value) {
      // channelId / channelType / region 不在补丁内：创建后不可改。
      await updatePlatformChannel(form.channelId, {
        channelName: form.channelName,
        enabled: form.enabled,
        sort: form.sort,
        loginMode: form.loginMode,
        paymentMode: form.paymentMode,
        loginLocked: form.loginLocked,
        paymentLocked: form.paymentLocked
      });
      ElMessage.success("已更新渠道");
    } else {
      await createPlatformChannel({
        channelId: form.channelId,
        channelName: form.channelName,
        channelType: form.channelType,
        region: form.region,
        enabled: form.enabled,
        sort: form.sort,
        loginMode: form.loginMode,
        paymentMode: form.paymentMode,
        loginLocked: form.loginLocked,
        paymentLocked: form.paymentLocked
      });
      ElMessage.success("已创建渠道");
    }
    drawerVisible.value = false;
    await reload();
  } catch (err) {
    reportError(err, "保存失败", (msg) => (formError.value = msg));
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void reload(1);
});
</script>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.panel__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.panel__filters {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}

.filter-keyword {
  width: 200px;
}

.filter-select {
  width: 140px;
}

.form-control {
  width: 100%;
}

.form-switches {
  display: flex;
  flex-direction: column;
}

.panel__pager {
  display: flex;
  justify-content: flex-end;
}

.panel__error {
  color: var(--danger);
  font-size: 13px;
  margin: 4px 0 0;
}

.panel__hint {
  color: var(--text-subtle);
  font-size: 12px;
  margin: 4px 0 0;
}

.text-muted {
  color: var(--text-subtle);
}
</style>
