<template>
  <div class="payment-routes-tab">
    <el-alert
      v-if="!canWrite"
      type="info"
      :closable="false"
      title="当前账号仅有查看权限，编辑入口已置灰。"
    />

    <PageCard title="支付路由" description="按 payWay 分组维护优先级链路；组内顺序严格使用后端返回顺序。">
      <div class="toolbar">
        <el-space wrap>
          <el-button @click="loadAll">刷新</el-button>
          <!-- production 隐藏 Sync 入口（00 §9）；执行由 sync 模块负责，此处仅入口占位。 -->
          <el-tooltip v-if="!app.isProduction" content="sandbox→production 同步由 sync 模块统一执行" placement="top">
            <el-button :disabled="true">Sync to Production</el-button>
          </el-tooltip>
        </el-space>
      </div>

      <el-collapse v-model="activeGroups">
        <el-collapse-item
          v-for="group in routesData.groups"
          :key="group.payWayId"
          :name="group.payWayId"
          :title="`${group.payWayName} (${group.payWayId})`"
        >
          <div class="group-head">
            <el-tag size="small" type="info">{{ group.payWayType }}</el-tag>
            <el-button v-perm="'payment.write'" size="small" :disabled="!canWrite" @click="openCreate(group.payWayId)">新增路由</el-button>
          </div>

          <el-table :data="group.routes" border size="small" :row-class-name="rowClassNameFactory(group)">
            <el-table-column label="#" width="56">
              <template #default="{ $index }">{{ $index + 1 }}</template>
            </el-table-column>
            <el-table-column label="作用域" min-width="360">
              <template #default="{ row }">
                <div class="selector-grid">
                  <span>package: {{ displaySelector(row.selector.packageCode) }}</span>
                  <span>channel: {{ displaySelector(row.selector.channelId) }}</span>
                  <span>market: {{ displaySelector(row.selector.marketCode) }}</span>
                  <span>country: {{ displaySelector(row.selector.countryCode) }}</span>
                  <span>currency: {{ displaySelector(row.selector.currency) }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="目标通道" min-width="260">
              <template #default="{ row }">
                <div class="target-line">
                  <span>{{ row.providerId }} / {{ row.merchantAccountId }}</span>
                  <el-tag v-if="isFallbackRoute(row)" size="small" type="warning">兜底</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="priority" label="priority" width="90" />
            <el-table-column label="启用" width="80">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? "是" : "否" }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" min-width="200">
              <template #default="{ row }">
                <el-tag v-if="conflictKindOf(group, row)" size="small" type="danger">
                  {{ conflictLabel(conflictKindOf(group, row)!) }}
                </el-tag>
                <span v-else-if="hasDisabledReference(row)" class="danger">引用对象已禁用</span>
                <span v-else class="ok">正常</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="160" fixed="right">
              <template #default="{ row, $index }">
                <el-button v-perm="'payment.write'" link :disabled="!canWrite" @click="openEdit(group.payWayId, $index)">编辑</el-button>
                <el-dropdown trigger="click" :disabled="!canWrite">
                  <el-button link :disabled="!canWrite">⋯</el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item @click="openSwitchPsp(group.payWayId, $index)">切换通道</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>
        </el-collapse-item>
      </el-collapse>
    </PageCard>

    <el-drawer
      :model-value="drawerOpen"
      title="支付路由"
      size="560px"
      :close-on-click-modal="false"
      @update:model-value="onDrawerChange"
    >
      <el-form label-position="top">
        <el-form-item label="payWay">
          <el-select v-model="editor.payWayId" class="full" :disabled="editor.mode === 'edit'">
            <el-option v-for="item in payWays" :key="item.payWayId" :label="`${item.payWayName} (${item.payWayId})`" :value="item.payWayId" />
          </el-select>
        </el-form-item>

        <el-form-item label="provider" required>
          <el-select v-model="editor.providerId" class="full" filterable @change="onProviderSelect">
            <el-option v-for="item in providers" :key="item.providerId" :label="item.providerName" :value="item.providerId" />
          </el-select>
        </el-form-item>

        <el-form-item label="merchant_account" required>
          <el-select v-model="editor.merchantAccountId" class="full" filterable>
            <el-option
              v-for="item in filteredMerchants"
              :key="item.merchantAccountId"
              :label="`${item.merchantName} (${item.merchantAccountId})`"
              :value="item.merchantAccountId"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="priority" required>
          <el-input-number v-model="editor.priority" :min="1" :max="9999" class="full" />
        </el-form-item>

        <el-form-item label="enabled">
          <el-switch v-model="editor.enabled" />
        </el-form-item>

        <SelectorRow label="package" any-label="任意 *" :any="editor.packageAny" :value="editor.packageCode" :options="packageOptions" @update:any="(v: boolean) => (editor.packageAny = v)" @update:value="(v) => (editor.packageCode = v)" />
        <SelectorRow label="channel" any-label="任意 *" :any="editor.channelAny" :value="editor.channelId" :options="channelOptions" @update:any="(v: boolean) => (editor.channelAny = v)" @update:value="(v) => (editor.channelId = v)" />
        <SelectorRow label="market" any-label="任意 *" :any="editor.marketAny" :value="editor.marketCode" :options="marketOptions" @update:any="(v: boolean) => (editor.marketAny = v)" @update:value="(v) => (editor.marketCode = v)" />
        <SelectorRow label="country" any-label="任意 *" :any="editor.countryAny" :value="editor.countryCode" :free-input="true" @update:any="(v: boolean) => (editor.countryAny = v)" @update:value="(v) => (editor.countryCode = v)" />
        <SelectorRow label="currency" any-label="任意 *" :any="editor.currencyAny" :value="editor.currency" :options="currencyOptions" @update:any="(v: boolean) => (editor.currencyAny = v)" @update:value="(v) => (editor.currency = v)" />
      </el-form>

      <template #footer>
        <div class="actions">
          <el-button @click="drawerOpen = false">取消</el-button>
          <el-button v-perm="'payment.write'" type="primary" :loading="saving" @click="saveRoute">保存</el-button>
        </div>
      </template>
    </el-drawer>

    <el-drawer :model-value="switchDrawerOpen" title="切换通道" size="460px" @update:model-value="onSwitchDrawerChange">
      <el-form label-position="top">
        <el-form-item label="provider" required>
          <el-select v-model="switchForm.providerId" class="full" filterable @change="onSwitchProviderChange">
            <el-option v-for="item in providers" :key="item.providerId" :label="item.providerName" :value="item.providerId" />
          </el-select>
        </el-form-item>
        <el-form-item label="merchant_account" required>
          <el-select v-model="switchForm.merchantAccountId" class="full" filterable>
            <el-option
              v-for="item in switchMerchants"
              :key="item.merchantAccountId"
              :label="`${item.merchantName} (${item.merchantAccountId})`"
              :value="item.merchantAccountId"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="actions">
          <el-button @click="switchDrawerOpen = false">取消</el-button>
          <el-button v-perm="'payment.write'" type="primary" :loading="saving" @click="saveSwitch">保存</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from "vue";
import { ElMessage, ElSwitch } from "element-plus";
import PageCard from "@/components/page/PageCard.vue";
import { ApiError } from "@/api/http";
import { useAppStore } from "@/stores/app";
import { useDictionaryStore } from "@/stores/dictionary";
import { usePermissionStore } from "@/stores/permission";
import { listChannelPackages, listMarketChannels, type ChannelPackage, type MarketChannelListItem } from "@/api/modules/channels";
import {
  getGamePaymentRoutes,
  isRouteConflictError,
  listMerchantAccounts,
  listPayWays,
  listProviders,
  saveGamePaymentRoutes,
  type GamePaymentRoute,
  type GamePaymentRoutesResponse,
  type MerchantAccountItem,
  type PayWayItem,
  type ProviderItem,
  type RouteConflictDetail,
  type SaveGamePaymentRouteItem
} from "@/api/modules/payment";

const SelectorRow = defineComponent({
  props: {
    label: { type: String, required: true },
    anyLabel: { type: String, default: "任意 *" },
    any: { type: Boolean, required: true },
    value: { type: String, default: "" },
    options: { type: Array as () => Array<{ label: string; value: string }>, default: () => [] },
    freeInput: { type: Boolean, default: false }
  },
  emits: ["update:any", "update:value"],
  setup(props, { emit }) {
    return () =>
      h("div", { class: "selector-row" }, [
        h("div", { class: "selector-row__title" }, `${props.label}`),
        h("div", { class: "selector-row__body" }, [
          h(ElSwitch, {
            modelValue: props.any,
            activeText: props.anyLabel,
            inactiveText: "指定",
            "onUpdate:modelValue": (value: string | number | boolean) => emit("update:any", Boolean(value))
          }),
          props.any
            ? null
            : props.freeInput
              ? h("input", {
                  class: "el-input__inner selector-row__input",
                  value: props.value,
                  onInput: (event: Event) => emit("update:value", (event.target as HTMLInputElement).value)
                })
              : h(
                  "select",
                  {
                    class: "selector-row__select",
                    value: props.value,
                    onChange: (event: Event) => emit("update:value", (event.target as HTMLSelectElement).value)
                  },
                  [h("option", { value: "" }, "请选择"), ...props.options.map((item) => h("option", { value: item.value }, item.label))]
                )
        ])
      ]);
  }
});

const props = defineProps<{ gameId: string }>();

const app = useAppStore();
const dictionary = useDictionaryStore();
const permission = usePermissionStore();
const canWrite = computed(() => permission.hasPerm("payment.write"));

const routesData = ref<GamePaymentRoutesResponse>({ gameId: props.gameId, env: app.environment, groups: [] });
const activeGroups = ref<string[]>([]);
const payWays = ref<PayWayItem[]>([]);
const providers = ref<ProviderItem[]>([]);
const merchants = ref<MerchantAccountItem[]>([]);
const marketChannels = ref<MarketChannelListItem[]>([]);
const channelPackages = ref<ChannelPackage[]>([]);

type ConflictKind = "duplicate_priority" | "duplicate_selector";

// 兼容后端两种定位口径：按 route id 或按提交 items 的扁平索引。
const conflictKindById = ref<Map<number, ConflictKind>>(new Map());
const conflictKindByIndex = ref<Map<number, ConflictKind>>(new Map());

function clearConflicts() {
  conflictKindById.value = new Map();
  conflictKindByIndex.value = new Map();
}

// 计算某 group 在扁平 items 序列中的起始偏移（组内顺序 = 后端返回顺序，不重排）。
function groupFlatOffset(payWayId: string): number {
  let offset = 0;
  for (const g of routesData.value.groups) {
    if (g.payWayId === payWayId) {
      return offset;
    }
    offset += g.routes.length;
  }
  return offset;
}

function conflictKindOf(group: { payWayId: string; routes: GamePaymentRoute[] }, row: GamePaymentRoute): ConflictKind | null {
  if (row.id && conflictKindById.value.has(row.id)) {
    return conflictKindById.value.get(row.id) ?? null;
  }
  const localIndex = group.routes.findIndex((item) => item === row);
  if (localIndex >= 0) {
    const flatIndex = groupFlatOffset(group.payWayId) + localIndex;
    if (conflictKindByIndex.value.has(flatIndex)) {
      return conflictKindByIndex.value.get(flatIndex) ?? null;
    }
  }
  return null;
}

function conflictLabel(kind: ConflictKind): string {
  return kind === "duplicate_priority" ? "优先级冲突" : "选择器冲突";
}

const drawerOpen = ref(false);
const switchDrawerOpen = ref(false);
const saving = ref(false);

const editor = reactive({
  mode: "create" as "create" | "edit",
  groupPayWayId: "",
  rowIndex: -1,
  payWayId: "",
  providerId: "",
  merchantAccountId: "",
  priority: 100,
  enabled: true,
  packageAny: true,
  packageCode: "",
  channelAny: true,
  channelId: "",
  marketAny: true,
  marketCode: "GLOBAL",
  countryAny: true,
  countryCode: "",
  currencyAny: true,
  currency: ""
});

const switchForm = reactive({
  groupPayWayId: "",
  rowIndex: -1,
  providerId: "",
  merchantAccountId: ""
});

const channelOptions = computed(() => marketChannels.value.map((item) => ({ label: `${item.market} / ${item.channelId}`, value: item.channelId })));
const packageOptions = computed(() => channelPackages.value.map((item) => ({ label: `${item.packageCode} - ${item.packageName}`, value: item.packageCode })));
const marketOptions = ["GLOBAL", "JP", "KR", "SEA", "HMT", "CN"].map((item) => ({ label: item, value: item }));
const currencyOptions = computed(() => dictionary.enabledCurrencySpecs.map((item) => ({ label: item.currencyCode, value: item.currencyCode })));
const filteredMerchants = computed(() => merchants.value.filter((item) => item.providerId === editor.providerId));
const switchMerchants = computed(() => merchants.value.filter((item) => item.providerId === switchForm.providerId));

function displaySelector(value: string | null | undefined): string {
  if (!value || value === "*") {
    return "*";
  }
  return value;
}

function hasDisabledReference(route: GamePaymentRoute): boolean {
  if (route.hasDisabledReference) {
    return true;
  }
  if (route.disabledRefs && route.disabledRefs.length > 0) {
    return true;
  }
  return [route.payWayEnabled, route.providerEnabled, route.merchantAccountEnabled, route.channelEnabled, route.packageEnabled].some(
    (item) => item === false
  );
}

function isFallbackRoute(route: GamePaymentRoute): boolean {
  const selector = route.selector;
  const allAny =
    displaySelector(selector.packageCode) === "*" &&
    displaySelector(selector.channelId) === "*" &&
    displaySelector(selector.marketCode) === "*" &&
    displaySelector(selector.countryCode) === "*" &&
    displaySelector(selector.currency) === "*";
  const globalFallback =
    displaySelector(selector.packageCode) === "*" &&
    displaySelector(selector.channelId) === "*" &&
    ["GLOBAL", "*"].includes(displaySelector(selector.marketCode)) &&
    displaySelector(selector.countryCode) === "*" &&
    displaySelector(selector.currency) === "*";
  return allAny || globalFallback;
}

function rowClassNameFactory(group: { payWayId: string; routes: GamePaymentRoute[] }) {
  return ({ row }: { row: GamePaymentRoute }) => rowClassName(group, row);
}

function onSwitchDrawerChange(value: boolean) {
  switchDrawerOpen.value = value;
}

function rowClassName(group: { payWayId: string; routes: GamePaymentRoute[] }, row: GamePaymentRoute): string {
  const kind = conflictKindOf(group, row);
  if (kind === "duplicate_priority") {
    return "row--conflict-priority";
  }
  if (kind === "duplicate_selector") {
    return "row--conflict-selector";
  }
  if (hasDisabledReference(row)) {
    return "row--disabled-ref";
  }
  if (isFallbackRoute(row)) {
    return "row--fallback";
  }
  return "";
}

async function loadBaseData() {
  const [payWayRes, providerRes, merchantRes, channelsRes] = await Promise.all([
    listPayWays({ page: 1, pageSize: 200, enabled: true }),
    listProviders({ page: 1, pageSize: 200, enabled: true }),
    listMerchantAccounts({ page: 1, pageSize: 400, enabled: true }),
    listMarketChannels(props.gameId, { page: 1, pageSize: 200, hidden: undefined })
  ]);
  payWays.value = payWayRes.items;
  providers.value = providerRes.items;
  merchants.value = merchantRes.items;
  marketChannels.value = channelsRes.items;

  const packageRows = await Promise.all(channelsRes.items.map((item) => listChannelPackages(item.gameChannelId).catch(() => [])));
  channelPackages.value = packageRows.flat();
}

async function loadRoutes() {
  routesData.value = await getGamePaymentRoutes(props.gameId);
  activeGroups.value = routesData.value.groups.map((item) => item.payWayId);
}

async function loadAll() {
  clearConflicts();
  try {
    await Promise.all([dictionary.ensureCurrencySpecs(), loadBaseData(), loadRoutes()]);
  } catch (err) {
    ElMessage.error(err instanceof ApiError ? err.message : "加载支付路由失败");
  }
}

function resetEditor() {
  editor.mode = "create";
  editor.groupPayWayId = "";
  editor.rowIndex = -1;
  editor.payWayId = payWays.value[0]?.payWayId ?? "";
  editor.providerId = providers.value[0]?.providerId ?? "";
  editor.merchantAccountId = "";
  editor.priority = 100;
  editor.enabled = true;
  editor.packageAny = true;
  editor.packageCode = "";
  editor.channelAny = true;
  editor.channelId = "";
  editor.marketAny = true;
  editor.marketCode = "GLOBAL";
  editor.countryAny = true;
  editor.countryCode = "";
  editor.currencyAny = true;
  editor.currency = "";
}

function openCreate(payWayId: string) {
  resetEditor();
  editor.mode = "create";
  editor.groupPayWayId = payWayId;
  editor.payWayId = payWayId;
  drawerOpen.value = true;
}

function applyFromRoute(route: GamePaymentRoute) {
  editor.providerId = route.providerId;
  editor.merchantAccountId = route.merchantAccountId;
  editor.priority = route.priority;
  editor.enabled = route.enabled;

  editor.packageAny = !route.selector.packageCode || route.selector.packageCode === "*";
  editor.packageCode = route.selector.packageCode ?? "";

  editor.channelAny = !route.selector.channelId || route.selector.channelId === "*";
  editor.channelId = route.selector.channelId ?? "";

  editor.marketAny = !route.selector.marketCode || route.selector.marketCode === "*";
  editor.marketCode = route.selector.marketCode === "*" ? "GLOBAL" : route.selector.marketCode;

  editor.countryAny = !route.selector.countryCode || route.selector.countryCode === "*";
  editor.countryCode = route.selector.countryCode === "*" ? "" : route.selector.countryCode;

  editor.currencyAny = !route.selector.currency || route.selector.currency === "*";
  editor.currency = route.selector.currency === "*" ? "" : route.selector.currency;
}

function openEdit(payWayId: string, index: number) {
  const group = routesData.value.groups.find((item) => item.payWayId === payWayId);
  const route = group?.routes[index];
  if (!group || !route) {
    return;
  }
  resetEditor();
  editor.mode = "edit";
  editor.groupPayWayId = payWayId;
  editor.rowIndex = index;
  editor.payWayId = payWayId;
  applyFromRoute(route);
  drawerOpen.value = true;
}

function openSwitchPsp(payWayId: string, index: number) {
  const group = routesData.value.groups.find((item) => item.payWayId === payWayId);
  const route = group?.routes[index];
  if (!route) {
    return;
  }
  switchForm.groupPayWayId = payWayId;
  switchForm.rowIndex = index;
  switchForm.providerId = route.providerId;
  switchForm.merchantAccountId = route.merchantAccountId;
  switchDrawerOpen.value = true;
}

function onDrawerChange(next: boolean) {
  drawerOpen.value = next;
  if (!next) {
    resetEditor();
  }
}

function onProviderSelect(value: string) {
  editor.providerId = value;
  editor.merchantAccountId = "";
}

function onSwitchProviderChange(value: string) {
  switchForm.providerId = value;
  switchForm.merchantAccountId = "";
}

function toPayloadItem(input: {
  payWayId: string;
  providerId: string;
  merchantAccountId: string;
  priority: number;
  enabled: boolean;
  packageAny: boolean;
  packageCode: string;
  channelAny: boolean;
  channelId: string;
  marketAny: boolean;
  marketCode: string;
  countryAny: boolean;
  countryCode: string;
  currencyAny: boolean;
  currency: string;
}): SaveGamePaymentRouteItem {
  return {
    payWayId: input.payWayId,
    providerId: input.providerId,
    merchantAccountId: input.merchantAccountId,
    priority: input.priority,
    enabled: input.enabled,
    packageCode: input.packageAny ? null : input.packageCode || null,
    channelId: input.channelAny ? null : input.channelId || null,
    marketCode: input.marketAny ? "*" : (input.marketCode as SaveGamePaymentRouteItem["marketCode"]),
    countryCode: input.countryAny ? "*" : input.countryCode || "*",
    currency: input.currencyAny ? "*" : input.currency || "*"
  };
}

function buildCurrentPayloadItems(): SaveGamePaymentRouteItem[] {
  const items: SaveGamePaymentRouteItem[] = [];
  for (const group of routesData.value.groups) {
    for (const row of group.routes) {
      items.push({
        payWayId: group.payWayId,
        providerId: row.providerId,
        merchantAccountId: row.merchantAccountId,
        priority: row.priority,
        enabled: row.enabled,
        packageCode: row.selector.packageCode,
        channelId: row.selector.channelId,
        marketCode: row.selector.marketCode,
        countryCode: row.selector.countryCode,
        currency: row.selector.currency
      });
    }
  }
  return items;
}

function validateEditor(): boolean {
  if (!editor.payWayId || !editor.providerId || !editor.merchantAccountId) {
    ElMessage.warning("请补齐 payWay/provider/merchantAccount");
    return false;
  }
  if (!editor.countryAny && !editor.countryCode.trim()) {
    ElMessage.warning("country 选择“指定”时不能为空");
    return false;
  }
  if (!editor.currencyAny && !editor.currency.trim()) {
    ElMessage.warning("currency 选择“指定”时不能为空");
    return false;
  }
  return true;
}

function applyConflict(err: ApiError): { hasPriority: boolean; hasSelector: boolean } {
  const byId = new Map<number, ConflictKind>();
  const byIndex = new Map<number, ConflictKind>();
  let hasPriority = false;
  let hasSelector = false;

  for (const detail of err.details as RouteConflictDetail[]) {
    const kind: ConflictKind = detail.kind === "duplicate_priority" ? "duplicate_priority" : "duplicate_selector";
    if (kind === "duplicate_priority") {
      hasPriority = true;
    } else {
      hasSelector = true;
    }
    // 高亮两条冲突行：优先用 route id，缺失时回退提交索引。
    for (const id of [detail.leftRouteId, detail.rightRouteId]) {
      if (typeof id === "number") {
        byId.set(id, kind);
      }
    }
    for (const idx of [detail.leftIndex, detail.rightIndex]) {
      if (typeof idx === "number") {
        byIndex.set(idx, kind);
      }
    }
  }

  conflictKindById.value = byId;
  conflictKindByIndex.value = byIndex;
  return { hasPriority, hasSelector };
}

async function doSave(items: SaveGamePaymentRouteItem[]) {
  saving.value = true;
  try {
    clearConflicts();
    routesData.value = await saveGamePaymentRoutes(props.gameId, { items });
    ElMessage.success("支付路由已保存");
    drawerOpen.value = false;
    switchDrawerOpen.value = false;
  } catch (err) {
    if (isRouteConflictError(err)) {
      const { hasPriority, hasSelector } = applyConflict(err);
      const kinds = [hasPriority ? "优先级冲突(duplicate_priority)" : "", hasSelector ? "选择器冲突(duplicate_selector)" : ""]
        .filter(Boolean)
        .join(" / ");
      ElMessage.error(`路由冲突：已高亮冲突行 —— ${kinds || "duplicate_priority / duplicate_selector"}`);
      return;
    }
    ElMessage.error(err instanceof ApiError ? err.message : "保存支付路由失败");
  } finally {
    saving.value = false;
  }
}

async function saveRoute() {
  if (!validateEditor()) {
    return;
  }
  const items = buildCurrentPayloadItems();
  const current = toPayloadItem(editor);

  let targetIndex = -1;
  if (editor.mode === "edit") {
    const group = routesData.value.groups.find((item) => item.payWayId === editor.groupPayWayId);
    if (group) {
      let cursor = 0;
      for (const g of routesData.value.groups) {
        if (g.payWayId === editor.groupPayWayId) {
          targetIndex = cursor + editor.rowIndex;
          break;
        }
        cursor += g.routes.length;
      }
    }
  }

  if (targetIndex >= 0) {
    items[targetIndex] = current;
  } else {
    // 新增路由插入目标 payWay 组末尾，保持扁平 items 与 groups 顺序一致（供冲突索引映射）。
    let cursor = 0;
    let insertAt = items.length;
    for (const g of routesData.value.groups) {
      if (g.payWayId === editor.payWayId) {
        insertAt = cursor + g.routes.length;
        break;
      }
      cursor += g.routes.length;
    }
    items.splice(insertAt, 0, current);
  }

  await doSave(items);
}

async function saveSwitch() {
  if (!switchForm.providerId || !switchForm.merchantAccountId) {
    ElMessage.warning("请选择 provider 和 merchant_account");
    return;
  }
  const items = buildCurrentPayloadItems();
  let targetIndex = -1;
  let cursor = 0;
  for (const g of routesData.value.groups) {
    if (g.payWayId === switchForm.groupPayWayId) {
      targetIndex = cursor + switchForm.rowIndex;
      break;
    }
    cursor += g.routes.length;
  }
  if (targetIndex < 0 || !items[targetIndex]) {
    return;
  }
  items[targetIndex] = {
    ...items[targetIndex],
    providerId: switchForm.providerId,
    merchantAccountId: switchForm.merchantAccountId
  };
  await doSave(items);
}

onMounted(() => {
  void loadAll();
});
</script>

<style scoped>
.payment-routes-tab {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}

.group-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.selector-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 12px;
  font-size: 12px;
}

.target-line {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ok {
  color: var(--text-subtle);
}

.danger {
  color: var(--danger);
  font-weight: 600;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.full {
  width: 100%;
}

.selector-row {
  border: 1px solid var(--panel-border);
  border-radius: 8px;
  padding: 8px;
  margin-bottom: 10px;
}

.selector-row__title {
  font-size: 12px;
  color: var(--text-subtle);
  margin-bottom: 6px;
  text-transform: uppercase;
}

.selector-row__body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.selector-row__select,
.selector-row__input {
  width: 100%;
  min-height: 30px;
  border: 1px solid var(--panel-border);
  border-radius: 6px;
  padding: 4px 8px;
}

:deep(.row--conflict-priority) {
  --el-table-tr-bg-color: #fff1f1;
  box-shadow: inset 3px 0 0 #f56c6c;
}

:deep(.row--conflict-selector) {
  --el-table-tr-bg-color: #fef0f7;
  box-shadow: inset 3px 0 0 #d63384;
}

:deep(.row--disabled-ref) {
  --el-table-tr-bg-color: #fff7ed;
}

:deep(.row--fallback) {
  --el-table-tr-bg-color: #f8fafc;
}
</style>
