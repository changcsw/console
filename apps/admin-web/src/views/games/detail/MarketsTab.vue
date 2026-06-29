<template>
  <div class="markets-tab">
    <div class="markets-tab__toolbar">
      <PageStatusTag tone="neutral" :label="`市场数：${game.markets.length}`" />
      <el-button v-perm="'game.write'" type="primary" @click="openEdit">编辑市场</el-button>
    </div>

    <el-table :data="game.markets" border>
      <el-table-column prop="marketCode" label="市场" min-width="120" />
      <el-table-column label="默认市场" width="110">
        <template #default="{ row }">
          <el-tag v-if="row.isDefault" type="success" size="small">默认</el-tag>
          <span v-else class="text-muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="100">
        <template #default="{ row }">
          <PageStatusTag :tone="row.enabled ? 'success' : 'danger'" :label="row.enabled ? '启用' : '停用'" />
        </template>
      </el-table-column>
      <el-table-column prop="defaultLocale" label="默认语言" min-width="120" />
      <template #empty>
        <span class="text-muted">暂无市场</span>
      </template>
    </el-table>

    <el-drawer v-model="drawerVisible" title="编辑市场（全量覆盖）" size="560px">
      <el-form label-position="top">
        <el-form-item label="发行市场">
          <el-select v-model="selectedMarkets" multiple class="full-width" placeholder="选择发行市场">
            <el-option v-for="market in MARKET_OPTIONS" :key="market" :label="market" :value="market" />
          </el-select>
        </el-form-item>

        <el-table v-if="rows.length" :data="rows" border size="small" class="edit-table">
          <el-table-column label="市场" width="100">
            <template #default="{ row }">{{ row.marketCode }}</template>
          </el-table-column>
          <el-table-column label="默认" width="80">
            <template #default="{ row }">
              <el-radio
                v-model="defaultMarketCode"
                :value="row.marketCode"
                :label="row.marketCode"
                :disabled="!row.enabled"
              >
                <span></span>
              </el-radio>
            </template>
          </el-table-column>
          <el-table-column label="启用" width="80">
            <template #default="{ row }">
              <el-switch v-model="row.enabled" @change="onEnabledChange(row)" />
            </template>
          </el-table-column>
          <el-table-column label="默认语言" min-width="140">
            <template #default="{ row }">
              <el-input v-model="row.defaultLocale" placeholder="如 en-US" />
            </template>
          </el-table-column>
        </el-table>

        <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>
        <p class="field-hint">默认市场必须为启用状态；移除已有渠道实例或默认的市场会被后端拒绝（409）。</p>
      </el-form>
      <template #footer>
        <el-button @click="drawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { ApiError } from "@/api/http";
import { replaceMarkets, type GameDetail, type Market, type ReplaceMarketsItem } from "@/api/modules/games";
import { DEFAULT_LOCALE, MARKET_OPTIONS } from "../constants";

const props = defineProps<{ game: GameDetail }>();
const emit = defineEmits<{ updated: [game: GameDetail] }>();

const drawerVisible = ref(false);
const saving = ref(false);
const formError = ref("");

interface MarketRow {
  marketCode: Market;
  enabled: boolean;
  defaultLocale: string;
}

const selectedMarkets = ref<Market[]>([]);
const rows = reactive<MarketRow[]>([]);
const defaultMarketCode = ref<Market>("GLOBAL");

// 跟随多选变化同步行（保留已填值）
watch(selectedMarkets, (markets) => {
  const next: MarketRow[] = markets.map((code) => {
    const existing = rows.find((r) => r.marketCode === code);
    return existing ?? { marketCode: code, enabled: true, defaultLocale: DEFAULT_LOCALE };
  });
  rows.splice(0, rows.length, ...next);
  ensureDefaultEnabled();
});

// 默认市场只能落在已启用的行：当前默认缺失或被停用时，回退到首个启用市场。
function ensureDefaultEnabled() {
  const current = rows.find((r) => r.marketCode === defaultMarketCode.value);
  if (current?.enabled) {
    return;
  }
  const firstEnabled = rows.find((r) => r.enabled);
  defaultMarketCode.value = (firstEnabled?.marketCode ?? rows[0]?.marketCode ?? "GLOBAL") as Market;
}

// 停用某行时若其为默认市场，自动改选其它启用市场（默认市场须 enabled）。
function onEnabledChange(row: MarketRow) {
  if (!row.enabled && row.marketCode === defaultMarketCode.value) {
    ensureDefaultEnabled();
  }
}

function openEdit() {
  formError.value = "";
  rows.splice(
    0,
    rows.length,
    ...props.game.markets.map((m) => ({
      marketCode: m.marketCode,
      enabled: m.enabled,
      defaultLocale: m.defaultLocale || DEFAULT_LOCALE
    }))
  );
  selectedMarkets.value = props.game.markets.map((m) => m.marketCode);
  const current = props.game.markets.find((m) => m.isDefault);
  defaultMarketCode.value = (current?.marketCode ?? props.game.defaultMarketCode) as Market;
  drawerVisible.value = true;
}

async function submit() {
  formError.value = "";
  if (!rows.length) {
    formError.value = "至少保留一个市场";
    return;
  }
  const defaultRow = rows.find((r) => r.marketCode === defaultMarketCode.value);
  if (!defaultRow) {
    formError.value = "请指定一个默认市场（须在已选市场内）";
    return;
  }
  if (!defaultRow.enabled) {
    formError.value = "默认市场必须为启用状态";
    return;
  }
  const payload: ReplaceMarketsItem[] = rows.map((r) => ({
    marketCode: r.marketCode,
    isDefault: r.marketCode === defaultMarketCode.value,
    enabled: r.enabled,
    defaultLocale: r.defaultLocale || DEFAULT_LOCALE
  }));
  saving.value = true;
  try {
    const updated = await replaceMarkets(props.game.gameId, { markets: payload });
    ElMessage.success("市场已更新");
    emit("updated", updated);
    drawerVisible.value = false;
  } catch (err) {
    if (err instanceof ApiError) {
      formError.value = err.message || "保存失败";
    } else {
      formError.value = "保存失败";
    }
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.markets-tab {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.markets-tab__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.edit-table {
  margin-top: 8px;
}

.full-width {
  width: 100%;
}

.text-muted {
  color: var(--text-subtle);
}

.field-hint {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--text-subtle);
}

.panel__error {
  color: var(--danger);
  font-size: 13px;
  margin: 8px 0 0;
}
</style>
