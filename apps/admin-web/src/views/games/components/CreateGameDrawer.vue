<template>
  <el-drawer v-model="visible" title="新建游戏" size="480px" @closed="onClosed">
    <el-form label-position="top">
      <el-form-item label="游戏名称" required>
        <el-input v-model="form.name" placeholder="1-128 字符" maxlength="128" />
      </el-form-item>
      <el-form-item label="代号 alias" required>
        <el-input v-model="form.alias" placeholder="1-64 字符，仅字母数字 _ -，当前环境唯一" maxlength="64" />
        <p v-if="aliasHint" class="field-hint" :class="{ 'field-hint--error': aliasError }">{{ aliasHint }}</p>
      </el-form-item>
      <el-form-item label="图标 URL">
        <el-input v-model="form.iconUrl" placeholder="可选，需为合法 URL" maxlength="512" />
      </el-form-item>
      <el-form-item label="发行市场">
        <el-select v-model="form.markets" multiple class="full-width" placeholder="默认含 GLOBAL">
          <el-option v-for="market in MARKET_OPTIONS" :key="market" :label="market" :value="market" />
        </el-select>
        <p class="field-hint">默认市场必须在已选市场内；未选则自动建 GLOBAL。</p>
      </el-form-item>
      <el-form-item label="默认市场">
        <el-select v-model="form.defaultMarketCode" class="full-width">
          <el-option v-for="market in defaultMarketChoices" :key="market" :label="market" :value="market" />
        </el-select>
      </el-form-item>
      <el-form-item label="初始状态">
        <el-select v-model="form.status" class="full-width">
          <el-option v-for="opt in STATUS_OPTIONS" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
      </el-form-item>
      <p v-if="formError" class="panel__error" role="alert">{{ formError }}</p>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="submit">创建</el-button>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { ApiError } from "@/api/http";
import { createGame, type CreateGameRequest, type GameDetail, type GameStatus, type Market } from "@/api/modules/games";
import { isValidOptionalUrl, MARKET_OPTIONS, STATUS_OPTIONS } from "../constants";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ "update:open": [value: boolean]; created: [game: GameDetail] }>();

const visible = ref(false);
const saving = ref(false);
const formError = ref("");

const form = reactive({
  name: "",
  alias: "",
  iconUrl: "",
  markets: ["GLOBAL"] as Market[],
  defaultMarketCode: "GLOBAL" as Market,
  status: "draft" as GameStatus
});

watch(
  () => props.open,
  (next) => {
    visible.value = next;
    if (next) {
      resetForm();
    }
  }
);

watch(visible, (next) => {
  if (next !== props.open) {
    emit("update:open", next);
  }
});

// 默认市场只能从已选市场里挑（为空时退回全部）
const defaultMarketChoices = computed<Market[]>(() =>
  form.markets.length ? form.markets : MARKET_OPTIONS
);

// 已选市场变化时，确保默认市场仍在集合内
watch(
  () => form.markets.slice(),
  (markets) => {
    if (markets.length && !markets.includes(form.defaultMarketCode)) {
      form.defaultMarketCode = markets[0];
    }
  }
);

const aliasError = computed(() => {
  if (!form.alias) {
    return false;
  }
  return !/^[a-zA-Z0-9_-]+$/.test(form.alias);
});

const aliasHint = computed(() => (aliasError.value ? "alias 仅允许字母、数字、下划线和连字符" : ""));

function resetForm() {
  form.name = "";
  form.alias = "";
  form.iconUrl = "";
  form.markets = ["GLOBAL"];
  form.defaultMarketCode = "GLOBAL";
  form.status = "draft";
  formError.value = "";
}

function onClosed() {
  resetForm();
}

async function submit() {
  formError.value = "";
  if (!form.name.trim()) {
    formError.value = "请填写游戏名称";
    return;
  }
  if (!form.alias.trim() || aliasError.value) {
    formError.value = "请填写合法的 alias";
    return;
  }
  const iconUrl = form.iconUrl.trim();
  if (!isValidOptionalUrl(iconUrl)) {
    formError.value = "图标 URL 需以 http:// 或 https:// 开头（留空则不设置）";
    return;
  }
  const payload: CreateGameRequest = {
    name: form.name.trim(),
    alias: form.alias.trim(),
    iconUrl: iconUrl || undefined,
    defaultMarketCode: form.defaultMarketCode,
    status: form.status,
    markets: form.markets.length ? form.markets : undefined
  };
  saving.value = true;
  try {
    const game = await createGame(payload);
    ElMessage.success("游戏已创建");
    emit("created", game);
    visible.value = false;
  } catch (err) {
    if (err instanceof ApiError) {
      formError.value = err.message || "创建失败";
    } else {
      formError.value = "创建失败";
    }
  } finally {
    saving.value = false;
  }
}
</script>

<style scoped>
.full-width {
  width: 100%;
}

.field-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-subtle);
}

.field-hint--error {
  color: var(--danger);
}

.panel__error {
  color: var(--danger);
  font-size: 13px;
  margin: 4px 0 0;
}
</style>
