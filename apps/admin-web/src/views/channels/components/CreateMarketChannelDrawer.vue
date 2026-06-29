<template>
  <el-drawer
    :model-value="open"
    title="新建渠道实例"
    size="480px"
    :close-on-click-modal="false"
    @update:model-value="(v: boolean) => !v && emit('close')"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="96px" label-position="top">
      <el-form-item label="目标 Market" prop="market">
        <el-select v-model="form.market" placeholder="选择 market" class="full" @change="onMarketChange">
          <el-option v-for="m in MARKET_OPTIONS" :key="m" :label="m" :value="m" />
        </el-select>
        <div class="form-hint">GLOBAL/海外 market 仅显示 overseas 渠道；CN 仅显示 domestic 渠道。</div>
      </el-form-item>

      <el-form-item label="创建方式" prop="mode">
        <el-radio-group v-model="form.mode" @change="onModeChange">
          <el-radio value="empty">空白创建</el-radio>
          <el-radio value="copy">从其它 market 复制</el-radio>
        </el-radio-group>
      </el-form-item>

      <el-form-item label="渠道" prop="channelId">
        <el-select
          v-model="form.channelId"
          placeholder="选择渠道"
          class="full"
          filterable
          @change="onChannelChange"
        >
          <el-option
            v-for="c in channelCandidates"
            :key="c.channelId"
            :label="`${c.channelName}（${c.channelId} · ${regionLabel(c.region)}）`"
            :value="c.channelId"
          />
        </el-select>
        <div v-if="form.market && channelCandidates.length === 0" class="form-hint is-warn">
          该 market 下无可新增的兼容渠道（可能均已存在或不兼容）。
        </div>
      </el-form-item>

      <el-form-item v-if="form.mode === 'copy'" label="复制来源 Market" prop="copyFromMarket">
        <el-select v-model="form.copyFromMarket" placeholder="选择来源 market" class="full">
          <el-option v-for="m in copySourceMarkets" :key="m" :label="m" :value="m" />
        </el-select>
        <div v-if="form.channelId && copySourceMarkets.length === 0" class="form-hint is-warn">
          该渠道在其它 market 暂无可复制的实例。
        </div>
      </el-form-item>

      <el-alert
        v-if="form.mode === 'copy'"
        type="warning"
        :closable="false"
        show-icon
        class="copy-alert"
        title="复制创建说明"
      >
        <p>普通字段将从来源实例带入；<b>敏感字段（secret）与文件字段（file）会被清空</b>，需在详情中补填。</p>
        <p>新实例创建后状态为 <b>invalid</b>，提示：「{{ COPY_INVALID_HINT }}」。复制后新旧实例不再联动。</p>
      </el-alert>

      <el-form-item label="启用">
        <el-switch v-model="form.enabled" />
      </el-form-item>

      <el-form-item label="备注" prop="remark">
        <el-input v-model="form.remark" type="textarea" :rows="2" maxlength="255" show-word-limit />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="emit('close')">取消</el-button>
      <el-button v-perm="'channel.write'" type="primary" :loading="submitting" @click="submit">创建</el-button>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import type { FormInstance, FormRules } from "element-plus";
import { ElMessage } from "element-plus";
import { ApiError } from "@/api/http";
import {
  createMarketChannel,
  type AvailableChannel,
  type CreateChannelMode,
  type CreateMarketChannelResult,
  type Market,
  type MarketChannelListItem
} from "@/api/modules/channels";
import { COPY_INVALID_HINT, MARKET_OPTIONS, isMarketChannelCompatible, regionLabel } from "../constants";

const props = defineProps<{
  open: boolean;
  gameId: string;
  availableChannels: AvailableChannel[];
  existingItems: MarketChannelListItem[];
  defaultMarket?: Market | "";
  presetChannelId?: string;
  presetMode?: CreateChannelMode;
}>();

const emit = defineEmits<{
  (e: "close"): void;
  (e: "created", result: CreateMarketChannelResult): void;
}>();

const formRef = ref<FormInstance>();
const submitting = ref(false);

const form = reactive<{
  market: Market | "";
  mode: CreateChannelMode;
  channelId: string;
  copyFromMarket: string;
  enabled: boolean;
  remark: string;
}>({
  market: "",
  mode: "empty",
  channelId: "",
  copyFromMarket: "",
  enabled: true,
  remark: ""
});

const rules: FormRules = {
  market: [{ required: true, message: "请选择目标 market", trigger: "change" }],
  channelId: [{ required: true, message: "请选择渠道", trigger: "change" }],
  copyFromMarket: [
    {
      validator: (_r, value, cb) => {
        if (form.mode === "copy" && !value) {
          cb(new Error("复制创建须选择来源 market"));
          return;
        }
        cb();
      },
      trigger: "change"
    }
  ]
};

const regionByChannel = computed<Record<string, AvailableChannel>>(() => {
  const map: Record<string, AvailableChannel> = {};
  for (const c of props.availableChannels) {
    map[c.channelId] = c;
  }
  return map;
});

/** 目标 market 下已存在的 channelId 集合（任意 hidden 状态都算冲突，须排除） */
const existingChannelIdsInMarket = computed<Set<string>>(() => {
  const set = new Set<string>();
  if (!form.market) {
    return set;
  }
  for (const item of props.existingItems) {
    if (item.market === form.market) {
      set.add(item.channelId);
    }
  }
  return set;
});

/** 候选渠道：与目标 market 兼容 且 该 market 下尚不存在 */
const channelCandidates = computed<AvailableChannel[]>(() => {
  if (!form.market) {
    return [];
  }
  const market = form.market;
  return props.availableChannels.filter(
    (c) => isMarketChannelCompatible(market, c.region) && !existingChannelIdsInMarket.value.has(c.channelId)
  );
});

/** 复制来源 market：同渠道在其它 market 已存在实例的 market 列表 */
const copySourceMarkets = computed<string[]>(() => {
  if (!form.channelId || !form.market) {
    return [];
  }
  const markets = new Set<string>();
  for (const item of props.existingItems) {
    if (item.channelId === form.channelId && item.market !== form.market) {
      markets.add(item.market);
    }
  }
  return Array.from(markets);
});

function reset() {
  form.market = props.defaultMarket || "";
  form.mode = props.presetMode || "empty";
  form.channelId = props.presetChannelId || "";
  form.copyFromMarket = "";
  form.enabled = true;
  form.remark = "";
  formRef.value?.clearValidate();
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      reset();
    }
  }
);

function onMarketChange() {
  if (form.channelId && !channelCandidates.value.some((c) => c.channelId === form.channelId)) {
    form.channelId = "";
  }
  form.copyFromMarket = "";
}

function onModeChange() {
  form.copyFromMarket = "";
}

function onChannelChange() {
  form.copyFromMarket = "";
}

async function submit() {
  if (!formRef.value) {
    return;
  }
  await formRef.value.validate(async (valid) => {
    if (!valid || !form.market) {
      return;
    }
    submitting.value = true;
    try {
      const result = await createMarketChannel(props.gameId, form.market as Market, {
        channelId: form.channelId,
        mode: form.mode,
        copyFromMarket: form.mode === "copy" ? form.copyFromMarket : undefined,
        enabled: form.enabled,
        remark: form.remark || undefined
      });
      ElMessage.success(
        result.configStatus === "invalid"
          ? `已创建（${result.lastCheckMessage || COPY_INVALID_HINT}）`
          : "渠道实例已创建"
      );
      emit("created", result);
      emit("close");
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === "MARKET_CHANNEL_INCOMPATIBLE") {
          ElMessage.error("渠道与目标 market 不兼容");
        } else if (err.code === "CONFLICT") {
          ElMessage.error("该 market 下已存在同渠道实例");
        } else {
          ElMessage.error(err.message || "创建失败");
        }
      } else {
        ElMessage.error("创建失败");
      }
    } finally {
      submitting.value = false;
    }
  });
}
</script>

<style scoped>
.full {
  width: 100%;
}

.form-hint {
  color: var(--text-subtle);
  font-size: 12px;
  line-height: 1.5;
  margin-top: 4px;
}

.form-hint.is-warn {
  color: #b54708;
}

.copy-alert {
  margin-bottom: 18px;
}

.copy-alert p {
  margin: 4px 0;
  font-size: 12px;
  line-height: 1.6;
}
</style>
