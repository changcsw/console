<template>
  <el-drawer
    :model-value="open"
    title="渠道实例详情"
    size="560px"
    :close-on-click-modal="false"
    @update:model-value="(v: boolean) => !v && emit('close')"
  >
    <div v-loading="loading">
      <template v-if="detail">
        <section class="block">
          <div class="block__head">
            <div class="block__meta">
              <el-tag size="small" type="info">{{ detail.market }}</el-tag>
              <span class="channel-id">{{ detail.channelId }}</span>
              <ChannelInstanceStatusTag
                :config-status="detail.configStatus"
                :compatible="detail.compatible"
                :hidden="detail.hidden"
              />
            </div>
            <code class="display-key">{{ detail.displayKey }}</code>
          </div>

          <ChannelInstanceRuntimeFlags
            class="block__flags"
            :hidden="detail.hidden"
            :compatible="detail.compatible"
            :config-status="detail.configStatus"
            :included-in-snapshot="detail.includedInSnapshot"
            :included-in-sync="detail.includedInSync"
            :included-in-runtime-config="detail.includedInRuntimeConfig"
          />

          <el-descriptions :column="1" border size="small" class="block__desc">
            <el-descriptions-item label="区域">{{ regionLabel(detail.region) }}</el-descriptions-item>
            <el-descriptions-item label="复制来源">{{ detail.copiedFromMarket || "—" }}</el-descriptions-item>
            <el-descriptions-item label="最近校验">{{ detail.lastCheckMessage || "—" }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.hidden" label="隐藏操作人">
              {{ detail.hiddenBy || "—" }}
            </el-descriptions-item>
          </el-descriptions>
        </section>

        <section class="block">
          <h4 class="block__title">基础设置</h4>
          <el-form label-width="80px" label-position="top">
            <el-form-item label="启用">
              <el-switch v-model="editEnabled" :disabled="!canWrite" />
            </el-form-item>
            <el-form-item label="备注">
              <el-input
                v-model="editRemark"
                type="textarea"
                :rows="2"
                maxlength="255"
                show-word-limit
                :disabled="!canWrite"
              />
            </el-form-item>
            <div class="actions-row">
              <el-button v-perm="'channel.write'" type="primary" :loading="savingBasic" @click="saveBasic">
                保存基础设置
              </el-button>
              <el-tooltip
                v-if="!detail.hidden && detail.configStatus !== 'valid'"
                content="仅 valid 配置可隐藏；invalid/empty 不得隐藏"
                placement="top"
              >
                <span><el-button disabled>隐藏</el-button></span>
              </el-tooltip>
              <el-popconfirm
                v-else-if="!detail.hidden"
                title="隐藏后将移出快照/同步/客户端最终配置，可恢复。确认隐藏？"
                width="240"
                @confirm="doHide"
              >
                <template #reference>
                  <el-button v-perm="'channel.write'" type="warning">隐藏</el-button>
                </template>
              </el-popconfirm>
              <el-popconfirm v-else title="恢复后将按规则重新参与生效集。确认恢复？" width="220" @confirm="doUnhide">
                <template #reference>
                  <el-button v-perm="'channel.write'" type="primary" plain>恢复</el-button>
                </template>
              </el-popconfirm>
            </div>
            <p class="form-hint">身份（market/渠道）不可修改；如需变更请删除旧实例后另建。</p>
          </el-form>
        </section>

        <section class="block">
          <div class="block__title-row">
            <h4 class="block__title">渠道包</h4>
            <el-button v-perm="'channel.write'" size="small" @click="toggleCreatePkg">
              {{ creatingPkg ? "取消" : "新建渠道包" }}
            </el-button>
          </div>

          <el-form v-if="creatingPkg" :model="pkgForm" label-width="92px" label-position="top" class="pkg-form">
            <el-form-item label="包标识 packageCode" required>
              <el-input v-model="pkgForm.packageCode" placeholder="同实例下唯一" />
            </el-form-item>
            <el-form-item label="包名称 packageName" required>
              <el-input v-model="pkgForm.packageName" />
            </el-form-item>
            <el-form-item label="Market">
              <el-input :model-value="detail.market" disabled />
              <span class="form-hint">须与所属渠道实例 market 一致。</span>
            </el-form-item>
            <el-form-item label="Bundle ID">
              <el-input v-model="pkgForm.bundleId" />
            </el-form-item>
            <el-form-item label="继承渠道配置">
              <el-switch v-model="pkgForm.inheritChannelConfig" />
            </el-form-item>
            <el-form-item label="启用">
              <el-switch v-model="pkgForm.enabled" />
            </el-form-item>
            <el-button type="primary" :loading="savingPkg" @click="createPkg">提交</el-button>
          </el-form>

          <el-table :data="packages" border size="small" class="pkg-table">
            <el-table-column prop="packageCode" label="包标识" min-width="140" />
            <el-table-column prop="packageName" label="包名称" min-width="140" />
            <el-table-column label="继承" width="80">
              <template #default="{ row }">{{ row.inheritChannelConfig ? "是" : "否" }}</template>
            </el-table-column>
            <el-table-column label="启用" width="90">
              <template #default="{ row }">
                <el-switch
                  :model-value="row.enabled"
                  :disabled="!canWrite"
                  @change="(v: boolean) => togglePkgEnabled(row, v)"
                />
              </template>
            </el-table-column>
            <el-table-column label="操作" width="88" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openPackageDetail(row)">详情</el-button>
              </template>
            </el-table-column>
            <template #empty>
              <span class="text-muted">暂无渠道包</span>
            </template>
          </el-table>
        </section>
      </template>
    </div>
    <ChannelPackageDetailDrawer :open="packageDetailOpen" :pkg="activePackage" @close="packageDetailOpen = false" />
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { usePermissionStore } from "@/stores/permission";
import { ApiError } from "@/api/http";
import {
  createChannelPackage,
  getMarketChannel,
  hideMarketChannel,
  listChannelPackages,
  unhideMarketChannel,
  updateChannelPackage,
  updateMarketChannel,
  type ChannelPackage,
  type MarketChannelDetail
} from "@/api/modules/channels";
import ChannelInstanceStatusTag from "./ChannelInstanceStatusTag.vue";
import ChannelInstanceRuntimeFlags from "./ChannelInstanceRuntimeFlags.vue";
import ChannelPackageDetailDrawer from "./ChannelPackageDetailDrawer.vue";
import { regionLabel } from "../constants";

const props = defineProps<{
  open: boolean;
  gameChannelId: number | null;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "changed"): void;
}>();

const permission = usePermissionStore();
const canWrite = computed(() => permission.hasPerm("channel.write"));

const loading = ref(false);
const detail = ref<MarketChannelDetail | null>(null);
const packages = ref<ChannelPackage[]>([]);

const editEnabled = ref(true);
const editRemark = ref("");
const savingBasic = ref(false);

const creatingPkg = ref(false);
const savingPkg = ref(false);
const packageDetailOpen = ref(false);
const activePackage = ref<ChannelPackage | null>(null);
const pkgForm = reactive({
  packageCode: "",
  packageName: "",
  bundleId: "",
  inheritChannelConfig: true,
  enabled: true
});

async function load(id: number) {
  loading.value = true;
  try {
    const [d, pkgs] = await Promise.all([getMarketChannel(id), listChannelPackages(id)]);
    detail.value = d;
    packages.value = pkgs;
    editEnabled.value = d.enabled;
    editRemark.value = d.remark;
  } catch (err) {
    detail.value = null;
    ElMessage.error(err instanceof ApiError ? err.message : "加载实例详情失败");
  } finally {
    loading.value = false;
  }
}

watch(
  () => [props.open, props.gameChannelId] as const,
  ([open, id]) => {
    creatingPkg.value = false;
    if (open && typeof id === "number") {
      void load(id);
    }
  }
);

function applyDetail(next: MarketChannelDetail) {
  detail.value = next;
  editEnabled.value = next.enabled;
  editRemark.value = next.remark;
}

async function saveBasic() {
  if (!detail.value) {
    return;
  }
  savingBasic.value = true;
  try {
    const next = await updateMarketChannel(detail.value.gameChannelId, {
      enabled: editEnabled.value,
      remark: editRemark.value
    });
    applyDetail(next);
    ElMessage.success("已保存");
    emit("changed");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "保存失败");
  } finally {
    savingBasic.value = false;
  }
}

async function doHide() {
  if (!detail.value) {
    return;
  }
  try {
    const next = await hideMarketChannel(detail.value.gameChannelId);
    applyDetail(next);
    ElMessage.success("已隐藏");
    emit("changed");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "隐藏失败");
  }
}

async function doUnhide() {
  if (!detail.value) {
    return;
  }
  try {
    const next = await unhideMarketChannel(detail.value.gameChannelId);
    applyDetail(next);
    ElMessage.success("已恢复");
    emit("changed");
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "恢复失败");
  }
}

function toggleCreatePkg() {
  creatingPkg.value = !creatingPkg.value;
  if (creatingPkg.value) {
    pkgForm.packageCode = "";
    pkgForm.packageName = "";
    pkgForm.bundleId = "";
    pkgForm.inheritChannelConfig = true;
    pkgForm.enabled = true;
  }
}

function openPackageDetail(row: ChannelPackage) {
  activePackage.value = row;
  packageDetailOpen.value = true;
}

async function createPkg() {
  if (!detail.value) {
    return;
  }
  if (!pkgForm.packageCode || !pkgForm.packageName) {
    ElMessage.warning("请填写包标识与包名称");
    return;
  }
  savingPkg.value = true;
  try {
    const created = await createChannelPackage(detail.value.gameChannelId, {
      packageCode: pkgForm.packageCode,
      packageName: pkgForm.packageName,
      marketCode: detail.value.market,
      bundleId: pkgForm.bundleId || undefined,
      inheritChannelConfig: pkgForm.inheritChannelConfig,
      enabled: pkgForm.enabled
    });
    packages.value = [created, ...packages.value];
    creatingPkg.value = false;
    ElMessage.success("渠道包已创建");
  } catch (err) {
    if (err instanceof ApiError && err.code === "CONFLICT") {
      ElMessage.error("同实例下包标识已存在");
    } else {
      ElMessage.error(err instanceof ApiError ? err.message : "创建渠道包失败");
    }
  } finally {
    savingPkg.value = false;
  }
}

async function togglePkgEnabled(row: ChannelPackage, value: boolean) {
  try {
    const next = await updateChannelPackage(row.packageId, { enabled: value });
    packages.value = packages.value.map((p) => (p.packageId === row.packageId ? next : p));
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "更新渠道包失败");
  }
}
</script>

<style scoped>
.block {
  margin-bottom: 22px;
}

.block__head {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.block__meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.channel-id {
  font-weight: 600;
}

.display-key {
  color: var(--text-subtle);
  font-size: 12px;
}

.block__flags {
  margin: 12px 0;
}

.block__title {
  margin: 0 0 12px;
  font-size: 14px;
}

.block__title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.form-hint {
  color: var(--text-subtle);
  font-size: 12px;
  margin: 6px 0 0;
}

.pkg-form {
  background: #f8fafc;
  border-radius: var(--radius-sm, 8px);
  padding: 14px;
  margin-bottom: 14px;
}

.text-muted {
  color: var(--text-subtle);
}
</style>
