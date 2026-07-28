<template>
  <div class="markets-tab">
    <div class="markets-tab__toolbar">
      <PageStatusTag tone="neutral" :label="`市场数：${game.markets.length}`" />
      <el-button v-perm="'game.write'" type="primary" @click="openEdit">编辑市场与法务链接</el-button>
    </div>

    <el-table :data="game.markets" border>
      <el-table-column label="市场" min-width="140">
        <template #default="{ row }">
          <span>{{ row.marketCode }}</span>
          <el-tag v-if="row.isDefault" type="success" size="small" class="market-default-tag">默认市场</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="启用" width="100">
        <template #default="{ row }">
          <PageStatusTag :tone="row.enabled ? 'success' : 'danger'" :label="row.enabled ? '启用' : '停用'" />
        </template>
      </el-table-column>
      <el-table-column prop="defaultLocale" label="默认语言" width="130" />
      <el-table-column label="服务条款 URL" min-width="210" show-overflow-tooltip>
        <template #default="{ row }">
          <a
            v-if="effectiveLegalByMarket(row.marketCode).termsUrl"
            :href="effectiveLegalByMarket(row.marketCode).termsUrl"
            target="_blank"
            rel="noreferrer"
          >
            {{ effectiveLegalByMarket(row.marketCode).termsUrl }}
          </a>
          <span v-else class="text-muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="隐私政策 URL" min-width="210" show-overflow-tooltip>
        <template #default="{ row }">
          <a
            v-if="effectiveLegalByMarket(row.marketCode).privacyUrl"
            :href="effectiveLegalByMarket(row.marketCode).privacyUrl"
            target="_blank"
            rel="noreferrer"
          >
            {{ effectiveLegalByMarket(row.marketCode).privacyUrl }}
          </a>
          <span v-else class="text-muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="账号注销 URL" min-width="210" show-overflow-tooltip>
        <template #default="{ row }">
          <a
            v-if="effectiveLegalByMarket(row.marketCode).deleteAccountUrl"
            :href="effectiveLegalByMarket(row.marketCode).deleteAccountUrl"
            target="_blank"
            rel="noreferrer"
          >
            {{ effectiveLegalByMarket(row.marketCode).deleteAccountUrl }}
          </a>
          <span v-else class="text-muted">—</span>
        </template>
      </el-table-column>
      <template #empty>
        <span class="text-muted">暂无市场</span>
      </template>
    </el-table>

    <el-drawer v-model="drawerVisible" title="编辑市场与法务链接（全量覆盖）" size="860px">
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

        <div class="legal-editor">
          <h4>法务链接（按市场）</h4>
          <div v-for="row in rows" :key="row.marketCode" class="legal-card">
            <div class="legal-card__head">
              <strong>{{ row.marketCode }}</strong>
              <el-switch
                v-if="row.marketCode !== 'GLOBAL'"
                v-model="row.inheritGlobal"
                inline-prompt
                active-text="引用 GLOBAL"
                inactive-text="独立配置"
              />
            </div>
            <el-input
              v-model="row.termsUrl"
              placeholder="服务条款 URL"
              :disabled="row.marketCode !== 'GLOBAL' && row.inheritGlobal"
            />
            <el-input
              v-model="row.privacyUrl"
              placeholder="隐私政策 URL"
              :disabled="row.marketCode !== 'GLOBAL' && row.inheritGlobal"
            />
            <el-input
              v-model="row.deleteAccountUrl"
              placeholder="账号注销 URL"
              :disabled="row.marketCode !== 'GLOBAL' && row.inheritGlobal"
            />
          </div>
        </div>

        <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>
        <p class="field-hint">新增市场默认引用 GLOBAL 法务链接；关闭“引用 GLOBAL”后可为该市场单独编辑。</p>
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
import {
  replaceLegalLinks,
  replaceMarkets,
  type GameDetail,
  type GameLegalLink,
  type Market,
  type ReplaceLegalLinksItem,
  type ReplaceMarketsItem
} from "@/api/modules/games";
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
  inheritGlobal: boolean;
  termsUrl: string;
  privacyUrl: string;
  deleteAccountUrl: string;
}

const selectedMarkets = ref<Market[]>([]);
const rows = reactive<MarketRow[]>([]);
const defaultMarketCode = ref<Market>("GLOBAL");

const DEFAULT_GLOBAL_LEGAL = {
  termsUrl: "https://legal.example.com/global/terms",
  privacyUrl: "https://legal.example.com/global/privacy",
  deleteAccountUrl: "https://legal.example.com/global/delete-account"
};

watch(selectedMarkets, (markets) => {
  const globalRow = rows.find((item) => item.marketCode === "GLOBAL");
  const next: MarketRow[] = markets.map((code) => {
    const existing = rows.find((r) => r.marketCode === code);
    if (existing) {
      return existing;
    }
    return {
      marketCode: code,
      enabled: true,
      defaultLocale: DEFAULT_LOCALE,
      inheritGlobal: code !== "GLOBAL",
      termsUrl: globalRow?.termsUrl || "",
      privacyUrl: globalRow?.privacyUrl || "",
      deleteAccountUrl: globalRow?.deleteAccountUrl || ""
    };
  });
  rows.splice(0, rows.length, ...next);
  ensureDefaultEnabled();
  syncInheritedRows();
});

function ensureDefaultEnabled() {
  const current = rows.find((r) => r.marketCode === defaultMarketCode.value);
  if (current?.enabled) {
    return;
  }
  const firstEnabled = rows.find((r) => r.enabled);
  defaultMarketCode.value = (firstEnabled?.marketCode ?? rows[0]?.marketCode ?? "GLOBAL") as Market;
}

function onEnabledChange(row: MarketRow) {
  if (!row.enabled && row.marketCode === defaultMarketCode.value) {
    ensureDefaultEnabled();
  }
}

function normalizeLink(raw?: Partial<GameLegalLink>) {
  return {
    termsUrl: raw?.termsUrl || "",
    privacyUrl: raw?.privacyUrl || "",
    deleteAccountUrl: raw?.deleteAccountUrl || ""
  };
}

function resolveGameGlobalLegal() {
  const defaultScope = props.game.legalLinks.find((item) => item.scopeType === "default");
  const globalMarket = props.game.legalLinks.find((item) => item.scopeType === "market" && item.scopeValue === "GLOBAL");
  return normalizeLink(globalMarket || defaultScope || DEFAULT_GLOBAL_LEGAL);
}

function resolveGameMarketLegal(marketCode: Market) {
  const marketScope = props.game.legalLinks.find((item) => item.scopeType === "market" && item.scopeValue === marketCode);
  return normalizeLink(marketScope);
}

function syncInheritedRows() {
  const globalRow = rows.find((item) => item.marketCode === "GLOBAL");
  if (!globalRow) {
    return;
  }
  rows.forEach((row) => {
    if (row.marketCode !== "GLOBAL" && row.inheritGlobal) {
      row.termsUrl = globalRow.termsUrl;
      row.privacyUrl = globalRow.privacyUrl;
      row.deleteAccountUrl = globalRow.deleteAccountUrl;
    }
  });
}

function openEdit() {
  formError.value = "";
  const globalLegal = resolveGameGlobalLegal();
  rows.splice(
    0,
    rows.length,
    ...props.game.markets.map((m) => {
      const marketLegal = resolveGameMarketLegal(m.marketCode);
      const inheritGlobal =
        m.marketCode !== "GLOBAL" &&
        !props.game.legalLinks.some((item) => item.scopeType === "market" && item.scopeValue === m.marketCode);
      return {
        marketCode: m.marketCode,
        enabled: m.enabled,
        defaultLocale: m.defaultLocale || DEFAULT_LOCALE,
        inheritGlobal,
        termsUrl: m.marketCode === "GLOBAL" ? globalLegal.termsUrl : inheritGlobal ? globalLegal.termsUrl : marketLegal.termsUrl,
        privacyUrl:
          m.marketCode === "GLOBAL" ? globalLegal.privacyUrl : inheritGlobal ? globalLegal.privacyUrl : marketLegal.privacyUrl,
        deleteAccountUrl:
          m.marketCode === "GLOBAL"
            ? globalLegal.deleteAccountUrl
            : inheritGlobal
              ? globalLegal.deleteAccountUrl
              : marketLegal.deleteAccountUrl
      };
    })
  );
  selectedMarkets.value = props.game.markets.map((m) => m.marketCode);
  const current = props.game.markets.find((m) => m.isDefault);
  defaultMarketCode.value = (current?.marketCode ?? props.game.defaultMarketCode) as Market;
  drawerVisible.value = true;
}

function validateUrl(value: string) {
  return !value || /^https?:\/\//.test(value);
}

function buildLegalPayload(): ReplaceLegalLinksItem[] | null {
  const globalRow = rows.find((row) => row.marketCode === "GLOBAL");
  if (!globalRow) {
    formError.value = "必须包含 GLOBAL 市场，用于兜底法务链接";
    return null;
  }
  const allRows = rows.map((row) => ({
    ...row,
    termsUrl: row.termsUrl.trim(),
    privacyUrl: row.privacyUrl.trim(),
    deleteAccountUrl: row.deleteAccountUrl.trim()
  }));
  for (const row of allRows) {
    for (const [label, value] of [
      ["服务条款 URL", row.termsUrl],
      ["隐私政策 URL", row.privacyUrl],
      ["账号注销 URL", row.deleteAccountUrl]
    ] as const) {
      if (!validateUrl(value)) {
        formError.value = `${row.marketCode} ${label} 需以 http:// 或 https:// 开头（留空则不填）`;
        return null;
      }
    }
  }
  const payload: ReplaceLegalLinksItem[] = [
    {
      scopeType: "default",
      scopeValue: "*",
      termsUrl: globalRow.termsUrl.trim(),
      privacyUrl: globalRow.privacyUrl.trim(),
      deleteAccountUrl: globalRow.deleteAccountUrl.trim()
    }
  ];
  allRows.forEach((row) => {
    if (row.marketCode === "GLOBAL" || row.inheritGlobal) {
      return;
    }
    payload.push({
      scopeType: "market",
      scopeValue: row.marketCode,
      termsUrl: row.termsUrl,
      privacyUrl: row.privacyUrl,
      deleteAccountUrl: row.deleteAccountUrl
    });
  });
  return payload;
}

function effectiveLegalByMarket(marketCode: Market) {
  const marketScope = props.game.legalLinks.find((item) => item.scopeType === "market" && item.scopeValue === marketCode);
  if (marketScope) {
    return normalizeLink(marketScope);
  }
  return resolveGameGlobalLegal();
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
  const marketsPayload: ReplaceMarketsItem[] = rows.map((r) => ({
    marketCode: r.marketCode,
    isDefault: r.marketCode === defaultMarketCode.value,
    enabled: r.enabled,
    defaultLocale: r.defaultLocale || DEFAULT_LOCALE
  }));
  const legalPayload = buildLegalPayload();
  if (!legalPayload) {
    return;
  }
  saving.value = true;
  try {
    const marketUpdated = await replaceMarkets(props.game.gameId, { markets: marketsPayload });
    const legalUpdated = await replaceLegalLinks(props.game.gameId, { legalLinks: legalPayload });
    ElMessage.success("市场与法务链接已更新");
    emit("updated", { ...marketUpdated, legalLinks: legalUpdated.legalLinks });
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

.market-default-tag {
  margin-left: 8px;
}

.legal-editor {
  margin-top: 16px;
}

.legal-editor h4 {
  margin: 0 0 10px;
}

.legal-card {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 10px;
}

.legal-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
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
