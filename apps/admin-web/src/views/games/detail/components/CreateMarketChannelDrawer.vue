<template>
  <div v-if="open" class="drawer-backdrop">
    <aside class="drawer-panel">
      <div class="drawer-panel__header">
        <div>
          <h3>新建渠道实例</h3>
          <p>支持空白创建，或从其它 market 的同渠道复制普通字段。</p>
        </div>
        <button class="ghost-button" type="button" @click="$emit('close')">关闭</button>
      </div>

      <div class="mode-switches">
        <button class="mode-button" type="button" @click="resetToEmpty()">空白创建</button>
        <button
          class="mode-button mode-button--primary"
          type="button"
          aria-label="Copy from existing market"
          :disabled="!sourceInstance"
          @click="applyCopy()"
        >
          从现有市场复制
        </button>
      </div>

      <div class="drawer-grid">
        <label class="field">
          <span>目标市场</span>
          <select v-model="targetMarket" aria-label="targetMarket">
            <option v-for="market in availableMarkets" :key="market" :value="market">{{ market }}</option>
          </select>
        </label>

        <label class="field">
          <span>渠道 ID</span>
          <input v-model="channelId" aria-label="channelId" type="text" />
        </label>

        <label class="field">
          <span>clientId</span>
          <input v-model="clientId" aria-label="clientId" type="text" />
        </label>

        <label class="field">
          <span>clientSecret</span>
          <input v-model="clientSecret" aria-label="clientSecret" type="text" />
        </label>

        <label class="field">
          <span>keystoreFile</span>
          <input v-model="keystoreFile" aria-label="keystoreFile" type="text" />
        </label>
      </div>

      <div v-if="sourceInstance" class="drawer-hint">
        当前复制源：{{ sourceInstance.market }} / {{ sourceInstance.channelId }}
      </div>

      <div class="drawer-actions">
        <button class="ghost-button" type="button" @click="$emit('close')">取消</button>
        <button class="primary-button" type="button" @click="submit">保存</button>
      </div>
    </aside>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import {
  createGameMarketChannel,
  type GameMarketChannelListItem,
  type SourceMarketChannelInstance
} from "@/api/gameMarketChannels";

const props = defineProps<{
  open: boolean;
  gameId: string;
  selectedMarket: string;
  availableMarkets: string[];
  sourceInstance?: SourceMarketChannelInstance | null;
}>();

const emit = defineEmits<{
  close: [];
  created: [item: GameMarketChannelListItem];
}>();

const targetMarket = ref("");
const channelId = ref("");
const clientId = ref("");
const clientSecret = ref("");
const keystoreFile = ref("");
const mode = ref<"empty" | "copy">("empty");

watch(
  () => [props.open, props.selectedMarket, props.availableMarkets, props.sourceInstance] as const,
  () => {
    if (!props.open) {
      return;
    }

    resetToEmpty();
  },
  { immediate: true }
);

function resetToEmpty() {
  mode.value = "empty";
  targetMarket.value = props.selectedMarket || props.availableMarkets[0] || "GLOBAL";
  channelId.value = props.sourceInstance?.channelId ?? "";
  clientId.value = "";
  clientSecret.value = "";
  keystoreFile.value = "";
}

function applyCopy() {
  if (!props.sourceInstance) {
    return;
  }

  mode.value = "copy";
  targetMarket.value = props.selectedMarket || props.availableMarkets[0] || "GLOBAL";
  channelId.value = props.sourceInstance.channelId;
  clientId.value = String(props.sourceInstance.normalConfig.clientId ?? "");
  clientSecret.value = "";
  keystoreFile.value = "";
}

async function submit() {
  if (!channelId.value || !targetMarket.value) {
    return;
  }

  const created = await createGameMarketChannel(props.gameId, targetMarket.value, {
    channelId: channelId.value,
    region: targetMarket.value === "CN" ? "domestic" : "overseas",
    copyFromMarket: mode.value === "copy" ? props.sourceInstance?.market : undefined,
    normalConfig: clientId.value ? { clientId: clientId.value } : {},
    secretConfig: clientSecret.value ? { clientSecret: clientSecret.value } : {},
    fileConfig: keystoreFile.value ? { keystoreFile: keystoreFile.value } : {}
  });

  emit("created", {
    ...created,
    normalConfig: clientId.value ? { clientId: clientId.value } : {},
    secretConfig: clientSecret.value ? { clientSecret: clientSecret.value } : {},
    fileConfig: keystoreFile.value ? { keystoreFile: keystoreFile.value } : {}
  });
  emit("close");
}
</script>

<style scoped>
.drawer-backdrop {
  align-items: stretch;
  background: rgba(15, 23, 42, 0.22);
  display: flex;
  inset: 0;
  justify-content: flex-end;
  position: fixed;
  z-index: 40;
}

.drawer-panel {
  background: linear-gradient(180deg, #ffffff 0%, #f6faf8 100%);
  box-shadow: -24px 0 48px rgba(15, 23, 42, 0.12);
  display: flex;
  flex-direction: column;
  gap: 18px;
  max-width: 480px;
  padding: 24px;
  width: min(100%, 480px);
}

.drawer-panel__header {
  display: flex;
  gap: 16px;
  justify-content: space-between;
}

.drawer-panel__header h3 {
  margin: 0;
}

.drawer-panel__header p {
  color: var(--text-subtle);
  margin: 8px 0 0;
}

.mode-switches,
.drawer-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.drawer-grid {
  display: grid;
  gap: 14px;
}

.field {
  display: grid;
  gap: 8px;
}

.field span {
  font-size: 13px;
  font-weight: 600;
}

.field input,
.field select {
  border: 1px solid var(--panel-border);
  border-radius: var(--radius-sm);
  font: inherit;
  padding: 10px 12px;
}

.drawer-hint {
  background: var(--brand-soft);
  border-radius: var(--radius-sm);
  color: var(--brand);
  font-size: 13px;
  padding: 10px 12px;
}

.ghost-button,
.mode-button,
.primary-button {
  border: 1px solid transparent;
  border-radius: 999px;
  cursor: pointer;
  font: inherit;
  font-weight: 600;
  padding: 10px 14px;
}

.ghost-button,
.mode-button {
  background: #ffffff;
  border-color: var(--panel-border);
  color: var(--text-main);
}

.mode-button--primary,
.primary-button {
  background: var(--brand);
  color: #ffffff;
}

.mode-button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}
</style>
