<template>
  <div class="basic-tab">
    <div class="basic-tab__toolbar">
      <PageStatusTag :tone="statusMeta(game.status).tone" :label="statusMeta(game.status).label" />
      <el-button v-perm="'game.write'" type="primary" @click="openEdit">编辑基础信息</el-button>
    </div>

    <el-descriptions :column="2" border>
      <el-descriptions-item label="Game ID">
        <span class="mono">{{ game.gameId }}</span>
        <el-button link type="primary" class="copy-btn" @click="copy(game.gameId)">复制</el-button>
      </el-descriptions-item>
      <el-descriptions-item label="代号 alias">{{ game.alias }}</el-descriptions-item>
      <el-descriptions-item label="游戏名称">{{ game.name }}</el-descriptions-item>
      <el-descriptions-item label="默认市场">{{ game.defaultMarketCode }}</el-descriptions-item>
      <el-descriptions-item label="状态">
        <PageStatusTag :tone="statusMeta(game.status).tone" :label="statusMeta(game.status).label" />
      </el-descriptions-item>
      <el-descriptions-item label="Game Secret">
        <span class="mono text-muted">{{ game.gameSecret || "masked" }}</span>
        <span class="secret-note">（恒脱敏，仅创建时一次性明文）</span>
      </el-descriptions-item>
      <el-descriptions-item label="图标 URL">
        <a v-if="game.iconUrl" :href="game.iconUrl" target="_blank" rel="noreferrer">{{ game.iconUrl }}</a>
        <span v-else class="text-muted">—</span>
      </el-descriptions-item>
      <el-descriptions-item label="环境">{{ game.environment || app.environment }}</el-descriptions-item>
    </el-descriptions>

    <el-drawer v-model="drawerVisible" title="编辑基础信息" size="460px">
      <el-form label-position="top">
        <el-form-item label="游戏名称">
          <el-input v-model="form.name" maxlength="128" placeholder="1-128 字符" />
        </el-form-item>
        <el-form-item label="代号 alias">
          <el-input v-model="form.alias" maxlength="64" placeholder="仅字母数字 _ -，当前环境唯一" />
        </el-form-item>
        <el-form-item label="图标 URL">
          <el-input v-model="form.iconUrl" maxlength="512" placeholder="可选，需为合法 URL" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status" class="full-width">
            <el-option v-for="opt in STATUS_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="默认市场">
          <el-select v-model="form.defaultMarketCode" class="full-width">
            <el-option
              v-for="market in enabledMarketCodes"
              :key="market"
              :label="market"
              :value="market"
            />
          </el-select>
          <p class="field-hint">只能选择已启用的市场（在「市场」Tab 维护启用集合）。</p>
        </el-form-item>
        <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>
      </el-form>
      <template #footer>
        <el-button @click="drawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import PageStatusTag from "@/components/page/PageStatusTag.vue";
import { useAppStore } from "@/stores/app";
import { ApiError } from "@/api/http";
import { updateGame, type GameDetail, type GameStatus, type Market, type UpdateGameRequest } from "@/api/modules/games";
import { isValidOptionalUrl, STATUS_OPTIONS, statusMeta } from "../constants";

const props = defineProps<{ game: GameDetail }>();
const emit = defineEmits<{ updated: [game: GameDetail] }>();

const app = useAppStore();

const drawerVisible = ref(false);
const saving = ref(false);
const formError = ref("");

const form = reactive({
  name: "",
  alias: "",
  iconUrl: "",
  status: "draft" as GameStatus,
  defaultMarketCode: "GLOBAL" as Market
});

const enabledMarketCodes = computed<Market[]>(() =>
  props.game.markets.filter((m) => m.enabled).map((m) => m.marketCode)
);

function openEdit() {
  form.name = props.game.name;
  form.alias = props.game.alias;
  form.iconUrl = props.game.iconUrl;
  form.status = props.game.status;
  form.defaultMarketCode = props.game.defaultMarketCode as Market;
  formError.value = "";
  drawerVisible.value = true;
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text);
    ElMessage.success("已复制");
  } catch {
    ElMessage.warning("复制失败");
  }
}

async function submit() {
  formError.value = "";
  const iconUrl = form.iconUrl.trim();
  if (!isValidOptionalUrl(iconUrl)) {
    formError.value = "图标 URL 需以 http:// 或 https:// 开头（留空则不设置）";
    return;
  }
  const payload: UpdateGameRequest = {
    name: form.name.trim(),
    alias: form.alias.trim(),
    iconUrl,
    status: form.status,
    defaultMarketCode: form.defaultMarketCode
  };
  saving.value = true;
  try {
    const updated = await updateGame(props.game.gameId, payload);
    ElMessage.success("已更新基础信息");
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
.basic-tab {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.basic-tab__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.mono {
  font-family: monospace;
}

.copy-btn {
  margin-left: 8px;
}

.secret-note {
  margin-left: 8px;
  color: var(--text-subtle);
  font-size: 12px;
}

.text-muted {
  color: var(--text-subtle);
}

.full-width {
  width: 100%;
}

.field-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-subtle);
}

.panel__error {
  color: var(--danger);
  font-size: 13px;
  margin: 4px 0 0;
}
</style>
