import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import { useDictionaryStore } from "@/stores/dictionary";
import { ApiError } from "@/api/http";
import GameCashierTab from "@/views/cashier/game/GameCashierTab.vue";
import type {
  CashierPriceRow,
  CashierTemplateDetail,
  CashierTemplateSummary,
  GameCashierPriceOverride,
  GameCashierProfile
} from "@/api/modules/cashier";

const listCashierTemplates = vi.fn();
const getCashierTemplate = vi.fn();
const getGameCashierProfile = vi.fn();
const getGameCashierPriceOverrides = vi.fn();
const getCashierPriceRows = vi.fn();
const putGameCashierProfile = vi.fn();
const putGameCashierPriceOverrides = vi.fn();

vi.mock("@/api/modules/cashier", () => ({
  listCashierTemplates: (...a: unknown[]) => listCashierTemplates(...a),
  getCashierTemplate: (...a: unknown[]) => getCashierTemplate(...a),
  getGameCashierProfile: (...a: unknown[]) => getGameCashierProfile(...a),
  getGameCashierPriceOverrides: (...a: unknown[]) => getGameCashierPriceOverrides(...a),
  getCashierPriceRows: (...a: unknown[]) => getCashierPriceRows(...a),
  putGameCashierProfile: (...a: unknown[]) => putGameCashierProfile(...a),
  putGameCashierPriceOverrides: (...a: unknown[]) => putGameCashierPriceOverrides(...a)
}));

function summary(o: Partial<CashierTemplateSummary> = {}): CashierTemplateSummary {
  return {
    templateId: "global_default",
    templateName: "Global Default",
    fxSyncEnabled: true,
    fxSyncMode: "manual_confirm",
    fxSyncSchedule: "monthly",
    status: "active",
    ...o
  };
}

function detail(o: Partial<CashierTemplateDetail> = {}): CashierTemplateDetail {
  return {
    templateId: "global_default",
    templateName: "Global Default",
    fxSyncEnabled: true,
    fxSyncMode: "manual_confirm",
    fxSyncSchedule: "monthly",
    status: "active",
    versions: [
      { version: "2", status: "draft", publishedAt: null },
      { version: "1", status: "published", publishedAt: "2026-01-01T00:00:00Z" }
    ],
    fxSyncRuns: [],
    ...o
  };
}

function profile(o: Partial<GameCashierProfile> = {}): GameCashierProfile {
  return {
    templateId: "global_default",
    appliedTemplateVersion: "1",
    snapshotChecksum: "chk-abc",
    appliedAt: "2026-01-01T00:00:00Z",
    ...o
  };
}

function templateRow(o: Partial<CashierPriceRow> = {}): CashierPriceRow {
  return {
    countryCode: "US",
    regionCode: "*",
    currency: "USD",
    priceId: "p1",
    preTaxAmountMinor: 999,
    taxRate: 0.085,
    taxAmountMinor: 85,
    afterTaxAmountMinor: 1084,
    effectiveAt: "2026-01-01T00:00:00Z",
    ...o
  };
}

function override(o: Partial<GameCashierPriceOverride> = {}): GameCashierPriceOverride {
  return {
    countryCode: "US",
    regionCode: "*",
    currency: "USD",
    priceId: "p1",
    preTaxAmountMinor: 1999,
    taxRate: "0.1",
    taxAmountMinor: 200,
    afterTaxAmountMinor: 2199,
    reason: "promo",
    effectiveAt: "2026-01-01T00:00:00Z",
    ...o
  };
}

function seedDictionary() {
  const ds = useDictionaryStore();
  // USD: 2 位小数、最小金额 50 minor（用于下限测试）；JPY: 0 位小数
  ds.currencySpecs = [
    { currencyCode: "USD", currencyName: "US Dollar", decimalPlaces: 2, minAmountMinor: 50, roundingMode: "half_up", enabled: true },
    { currencyCode: "JPY", currencyName: "Japanese Yen", decimalPlaces: 0, minAmountMinor: 1, roundingMode: "half_up", enabled: true }
  ];
  ds.loaded = true;
}

interface TabVm {
  bindForm: { templateId: string; templateVersion: string };
  overrideRows: Array<Record<string, unknown>>;
  displayRows: Array<{ key: string; overridden: boolean }>;
  addOverrideRow: () => void;
  removeOverrideRow: (i: number) => void;
  matrixRowClassName: (arg: { row: { overridden: boolean } }) => string;
  previewText: (row: Record<string, unknown>) => string;
  previewIsError: (row: Record<string, unknown>) => boolean;
  saveProfile: () => Promise<void>;
  saveOverrides: () => Promise<void>;
}

async function mountTab(opts: { perms?: string[] } = {}) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: opts.perms ?? ["cashier.read", "cashier.write"] });
  seedDictionary();
  const wrapper = mount(GameCashierTab, {
    props: { gameId: "100001" },
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return wrapper;
}

function vmOf(wrapper: Awaited<ReturnType<typeof mountTab>>): TabVm {
  return wrapper.vm as unknown as TabVm;
}

describe("GameCashierTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listCashierTemplates.mockResolvedValue({ items: [summary()], page: 1, pageSize: 100, total: 1 });
    getGameCashierProfile.mockResolvedValue(profile());
    getCashierTemplate.mockResolvedValue(detail());
    getGameCashierPriceOverrides.mockResolvedValue({ items: [] });
    getCashierPriceRows.mockResolvedValue({ items: [] });
    putGameCashierProfile.mockImplementation((_gid: string, p: { templateVersion: string }) =>
      Promise.resolve(profile({ appliedTemplateVersion: p.templateVersion }))
    );
    putGameCashierPriceOverrides.mockImplementation((_gid: string, p: { items: GameCashierPriceOverride[] }) =>
      Promise.resolve({ items: p.items })
    );
  });

  test("已绑定模板：渲染快照（模板/校验和/已绑定标签）", async () => {
    const w = await mountTab();
    expect(getGameCashierProfile).toHaveBeenCalledWith("100001");
    const t = w.text();
    expect(t).toContain("global_default");
    expect(t).toContain("chk-abc");
    expect(t).toContain("已绑定模板");
  });

  test("未绑定（NOT_FOUND→null）：展示空态且不请求模板价格行", async () => {
    getGameCashierProfile.mockResolvedValue(null);
    const w = await mountTab();
    const t = w.text();
    expect(t).toContain("尚未绑定收银台模板");
    expect(t).toContain("未绑定模板");
    expect(getCashierPriceRows).not.toHaveBeenCalled();
    expect(t).toContain("暂无价格矩阵数据");
  });

  test("加载失败：展示错误 alert", async () => {
    listCashierTemplates.mockRejectedValue(new ApiError(500, "INTERNAL", "加载收银台信息失败-boom"));
    const w = await mountTab();
    expect(w.text()).toContain("加载收银台信息失败-boom");
  });

  test("边界视图：同键覆盖行高亮，模板独有行不高亮", async () => {
    getCashierPriceRows.mockResolvedValue({
      items: [
        templateRow({ priceId: "p1", preTaxAmountMinor: 999 }),
        templateRow({ countryCode: "JP", currency: "JPY", priceId: "p2", preTaxAmountMinor: 120 })
      ]
    });
    getGameCashierPriceOverrides.mockResolvedValue({ items: [override({ priceId: "p1", preTaxAmountMinor: 1999 })] });
    const w = await mountTab();
    const vm = vmOf(w);
    const byKey = Object.fromEntries(vm.displayRows.map((r) => [r.key, r]));
    expect(byKey["US|*|USD|p1"].overridden).toBe(true);
    expect(byKey["JP|*|JPY|p2"].overridden).toBe(false);
    expect(vm.matrixRowClassName({ row: byKey["US|*|USD|p1"] })).toContain("overridden");
    expect(vm.matrixRowClassName({ row: byKey["JP|*|JPY|p2"] })).toBe("");
  });

  test("currency_specs：合法覆盖行舍入预览 minor 正确", async () => {
    const w = await mountTab();
    const vm = vmOf(w);
    vm.addOverrideRow();
    const row = vm.overrideRows[0];
    Object.assign(row, { countryCode: "US", currency: "USD", priceId: "p1", preTaxMajorInput: "9.99", taxRate: 0.1 });
    expect(vm.previewIsError(row)).toBe(false);
    const text = vm.previewText(row);
    expect(text).toContain("preTax=999");
    expect(text).toContain("tax=100");
    expect(text).toContain("afterTax=1099");
  });

  test("currency_specs：币种不在 specs → CURRENCY_NOT_SUPPORTED", async () => {
    const w = await mountTab();
    const vm = vmOf(w);
    vm.addOverrideRow();
    const row = vm.overrideRows[0];
    Object.assign(row, { countryCode: "US", currency: "GBP", priceId: "p1", preTaxMajorInput: "9.99", taxRate: 0 });
    expect(vm.previewIsError(row)).toBe(true);
    expect(vm.previewText(row)).toContain("CURRENCY_NOT_SUPPORTED");
  });

  test("currency_specs：小数位超限与最小金额下限报错", async () => {
    const w = await mountTab();
    const vm = vmOf(w);
    vm.addOverrideRow();
    const row = vm.overrideRows[0];
    Object.assign(row, { countryCode: "US", currency: "USD", priceId: "p1", preTaxMajorInput: "9.999", taxRate: 0 });
    expect(vm.previewText(row)).toContain("最大小数位 2");
    row.preTaxMajorInput = "0.10"; // 10 minor < 50 minAmountMinor
    expect(vm.previewText(row)).toContain("最小金额（minor）为 50");
  });

  test("切换/升级版本：调用 putGameCashierProfile 并刷新价格行", async () => {
    const w = await mountTab();
    const vm = vmOf(w);
    vm.bindForm.templateVersion = "1";
    getGameCashierPriceOverrides.mockClear();
    await vm.saveProfile();
    await flushPromises();
    expect(putGameCashierProfile).toHaveBeenCalledWith("100001", { templateId: "global_default", templateVersion: "1" });
    expect(getGameCashierPriceOverrides).toHaveBeenCalled();
  });

  test("保存覆盖：归一化（大写国家/region 默认*/trim priceId/金额 minor）后下发", async () => {
    const w = await mountTab();
    const vm = vmOf(w);
    vm.addOverrideRow();
    Object.assign(vm.overrideRows[0], {
      countryCode: "us",
      regionCode: "",
      currency: "USD",
      priceId: " p1 ",
      preTaxMajorInput: "9.99",
      taxRate: 0.1,
      reason: "促销"
    });
    await vm.saveOverrides();
    await flushPromises();
    expect(putGameCashierPriceOverrides).toHaveBeenCalledTimes(1);
    const [gid, payload] = putGameCashierPriceOverrides.mock.calls[0] as [string, { items: GameCashierPriceOverride[] }];
    expect(gid).toBe("100001");
    expect(payload.items[0]).toMatchObject({
      countryCode: "US",
      regionCode: "*",
      currency: "USD",
      priceId: "p1",
      preTaxAmountMinor: 999,
      taxAmountMinor: 100,
      afterTaxAmountMinor: 1099,
      reason: "促销"
    });
  });

  test("保存覆盖：存在非法行时不下发保存请求", async () => {
    const w = await mountTab();
    const vm = vmOf(w);
    vm.addOverrideRow();
    Object.assign(vm.overrideRows[0], { countryCode: "US", currency: "USD", priceId: "", preTaxMajorInput: "" });
    await vm.saveOverrides();
    await flushPromises();
    expect(putGameCashierPriceOverrides).not.toHaveBeenCalled();
  });

  test("无 cashier.write：只读提示 + 绑定/覆盖写按钮置灰", async () => {
    const w = await mountTab({ perms: ["cashier.read"] });
    expect(w.text()).toContain("当前账号仅有查看权限");
    const find = (label: string) => w.findAll("button").find((b) => b.text().includes(label));
    expect(find("切换/升级版本")?.attributes("disabled")).toBeDefined();
    expect(find("新增覆盖行")?.attributes("disabled")).toBeDefined();
    expect(find("保存覆盖")?.attributes("disabled")).toBeDefined();
  });
});
