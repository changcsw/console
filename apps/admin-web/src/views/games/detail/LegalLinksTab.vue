<template>
  <div class="legal-tab">
    <div class="legal-tab__toolbar">
      <PageStatusTag tone="neutral" :label="`链接数：${game.legalLinks.length}`" />
      <el-button v-perm="'game.write'" type="primary" @click="openEdit">编辑法务链接</el-button>
    </div>

    <el-table :data="game.legalLinks" border>
      <el-table-column label="作用域类型" width="130">
        <template #default="{ row }">{{ scopeTypeLabel(row.scopeType) }}</template>
      </el-table-column>
      <el-table-column prop="scopeValue" label="作用域值" width="120" />
      <el-table-column prop="termsUrl" label="服务条款 URL" min-width="200" show-overflow-tooltip />
      <el-table-column prop="privacyUrl" label="隐私政策 URL" min-width="200" show-overflow-tooltip />
      <el-table-column prop="deleteAccountUrl" label="账号注销 URL" min-width="200" show-overflow-tooltip />
      <template #empty>
        <span class="text-muted">暂无法务链接</span>
      </template>
    </el-table>

    <el-drawer v-model="drawerVisible" title="编辑法务链接（全量覆盖）" size="720px">
      <div class="rows">
        <div v-for="(row, index) in rows" :key="index" class="legal-row">
          <div class="legal-row__head">
            <el-select v-model="row.scopeType" class="scope-type" @change="onScopeTypeChange(row)">
              <el-option v-for="opt in SCOPE_TYPE_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
            <el-input v-if="row.scopeType === 'default'" model-value="*" disabled class="scope-value" />
            <el-select v-else-if="row.scopeType === 'market'" v-model="row.scopeValue" class="scope-value" placeholder="选择市场">
              <el-option v-for="market in MARKET_OPTIONS" :key="market" :label="market" :value="market" />
            </el-select>
            <el-input v-else v-model="row.scopeValue" class="scope-value" placeholder="语言标签，如 zh-CN" />
            <el-button link type="danger" @click="removeRow(index)">移除</el-button>
          </div>
          <el-input v-model="row.termsUrl" placeholder="服务条款 URL（可选）" class="url-input" />
          <el-input v-model="row.privacyUrl" placeholder="隐私政策 URL（可选）" class="url-input" />
          <el-input v-model="row.deleteAccountUrl" placeholder="账号注销 URL（可选）" class="url-input" />
        </div>
      </div>

      <el-button class="add-btn" @click="addRow">+ 新增一行</el-button>
      <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>

      <template #footer>
        <el-button @click="drawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { ApiError } from "@/api/http";
import {
  replaceLegalLinks,
  type GameDetail,
  type LegalScopeType,
  type ReplaceLegalLinksItem
} from "@/api/modules/games";
import { isValidOptionalUrl, MARKET_OPTIONS, SCOPE_TYPE_OPTIONS, scopeTypeLabel } from "../constants";

const props = defineProps<{ game: GameDetail }>();
const emit = defineEmits<{ updated: [game: GameDetail] }>();

const drawerVisible = ref(false);
const saving = ref(false);
const formError = ref("");

interface LegalRow {
  scopeType: LegalScopeType;
  scopeValue: string;
  termsUrl: string;
  privacyUrl: string;
  deleteAccountUrl: string;
}

const rows = reactive<LegalRow[]>([]);

const LOCALE_PATTERN = /^[a-z]{2}(-[A-Z]{2})?$/;

function openEdit() {
  formError.value = "";
  rows.splice(
    0,
    rows.length,
    ...props.game.legalLinks.map((l) => ({
      scopeType: l.scopeType,
      scopeValue: l.scopeValue,
      termsUrl: l.termsUrl,
      privacyUrl: l.privacyUrl,
      deleteAccountUrl: l.deleteAccountUrl
    }))
  );
  drawerVisible.value = true;
}

function addRow() {
  rows.push({ scopeType: "default", scopeValue: "*", termsUrl: "", privacyUrl: "", deleteAccountUrl: "" });
}

function removeRow(index: number) {
  rows.splice(index, 1);
}

function onScopeTypeChange(row: LegalRow) {
  if (row.scopeType === "default") {
    row.scopeValue = "*";
  } else {
    row.scopeValue = "";
  }
}

function validate(): boolean {
  const seen = new Set<string>();
  let defaultCount = 0;
  for (const row of rows) {
    if (row.scopeType === "default") {
      defaultCount += 1;
      row.scopeValue = "*";
    } else if (row.scopeType === "market") {
      if (!MARKET_OPTIONS.includes(row.scopeValue as never)) {
        formError.value = "market 作用域必须选择合法市场";
        return false;
      }
    } else if (row.scopeType === "locale") {
      if (!LOCALE_PATTERN.test(row.scopeValue)) {
        formError.value = "locale 作用域需为合法语言标签（如 zh-CN 或 zh）";
        return false;
      }
    }
    for (const [label, value] of [
      ["服务条款 URL", row.termsUrl],
      ["隐私政策 URL", row.privacyUrl],
      ["账号注销 URL", row.deleteAccountUrl]
    ] as const) {
      if (!isValidOptionalUrl(value.trim())) {
        formError.value = `${label} 需以 http:// 或 https:// 开头（留空则不填）`;
        return false;
      }
    }
    const key = `${row.scopeType}:${row.scopeValue}`;
    if (seen.has(key)) {
      formError.value = `作用域重复：${key}`;
      return false;
    }
    seen.add(key);
  }
  if (defaultCount > 1) {
    formError.value = "default 作用域每个游戏至多一条";
    return false;
  }
  return true;
}

async function submit() {
  formError.value = "";
  if (!validate()) {
    return;
  }
  const payload: ReplaceLegalLinksItem[] = rows.map((r) => ({
    scopeType: r.scopeType,
    scopeValue: r.scopeType === "default" ? "*" : r.scopeValue,
    termsUrl: r.termsUrl.trim(),
    privacyUrl: r.privacyUrl.trim(),
    deleteAccountUrl: r.deleteAccountUrl.trim()
  }));
  saving.value = true;
  try {
    const res = await replaceLegalLinks(props.game.gameId, { legalLinks: payload });
    ElMessage.success("法务链接已更新");
    emit("updated", { ...props.game, legalLinks: res.legalLinks });
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
.legal-tab {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.legal-tab__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.rows {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.legal-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border: 1px solid var(--panel-border);
  border-radius: var(--radius-md);
  background: #f8fbff;
}

.legal-row__head {
  display: flex;
  align-items: center;
  gap: 10px;
}

.scope-type {
  width: 150px;
}

.scope-value {
  width: 180px;
}

.url-input {
  width: 100%;
}

.add-btn {
  margin-top: 14px;
}

.text-muted {
  color: var(--text-subtle);
}

.panel__error {
  color: var(--danger);
  font-size: 13px;
  margin: 8px 0 0;
}
</style>
