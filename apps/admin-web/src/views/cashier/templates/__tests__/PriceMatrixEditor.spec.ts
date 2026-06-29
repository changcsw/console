import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";
import { nextTick } from "vue";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import PriceMatrixEditor from "@/views/cashier/templates/PriceMatrixEditor.vue";
import type { VersionStatus } from "@/api/modules/cashier";

const getCashierPriceRows = vi.fn();
const putCashierPriceRows = vi.fn();

vi.mock("@/api/modules/cashier", () => ({
  getCashierPriceRows: (...args: unknown[]) => getCashierPriceRows(...args),
  putCashierPriceRows: (...args: unknown[]) => putCashierPriceRows(...args)
}));

interface EditableRow {
  countryCode: string;
  regionCode: string;
  currency: string;
  priceId: string;
  preTaxMajorInput: string;
  taxRate: number;
  effectiveAt: string;
}

interface EditorVm {
  rows: EditableRow[];
  readonly: boolean;
  readonlyByStatus: boolean;
  normalizeRow: (row: EditableRow) => { ok: boolean; message?: string; value?: Record<string, number | string> };
  normalizeAmount: (value: number, mode: "half_up" | "floor" | "ceil" | "truncate") => number;
  previewText: (row: EditableRow) => string;
  previewIsError: (row: EditableRow) => boolean;
}

function editable(overrides: Partial<EditableRow> = {}): EditableRow {
  return {
    countryCode: "US",
    regionCode: "*",
    currency: "USD",
    priceId: "com.game.pack.001",
    preTaxMajorInput: "9.99",
    taxRate: 0,
    effectiveAt: "2026-01-01T00:00:00Z",
    ...overrides
  };
}

async function mountEditor(opts: {
  perms?: string[];
  version?: string;
  versionStatus?: VersionStatus;
} = {}) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: opts.perms ?? ["cashier.read", "cashier.write"] });
  const wrapper = mount(PriceMatrixEditor, {
    props: {
      templateId: "global_default",
      version: opts.version ?? "1",
      versionStatus: opts.versionStatus ?? "draft"
    },
    global: { directives: { perm: permDirective } }
  });
  await flushPromises();
  return wrapper;
}

describe("PriceMatrixEditor", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getCashierPriceRows.mockResolvedValue({ items: [] });
    putCashierPriceRows.mockResolvedValue({ items: [] });
  });

  test("USD half_up 精度内金额归一化预览正确", async () => {
    const wrapper = await mountEditor();
    const vm = wrapper.vm as unknown as EditorVm;
    const result = vm.normalizeRow(editable({ currency: "USD", preTaxMajorInput: "9.99", taxRate: 0.1 }));
    expect(result.ok).toBe(true);
    // 9.99 * 100 = 999 minor；tax = round(999*0.1)=100；afterTax=1099
    expect(result.value).toMatchObject({ preTaxAmountMinor: 999, taxAmountMinor: 100, afterTaxAmountMinor: 1099 });
    expect(vm.previewText(editable({ preTaxMajorInput: "9.99", taxRate: 0.1 }))).toContain("preTax=999");
  });

  test("JPY 小数位超限报错（decimalPlaces=0）", async () => {
    const wrapper = await mountEditor();
    const vm = wrapper.vm as unknown as EditorVm;
    const result = vm.normalizeRow(editable({ currency: "JPY", preTaxMajorInput: "9.99" }));
    expect(result.ok).toBe(false);
    expect(result.message).toContain("JPY 最大小数位 0");
    expect(vm.previewIsError(editable({ currency: "JPY", preTaxMajorInput: "9.99" }))).toBe(true);
  });

  test("低于币种最小金额下限报错", async () => {
    const wrapper = await mountEditor();
    const vm = wrapper.vm as unknown as EditorVm;
    const result = vm.normalizeRow(editable({ currency: "USD", preTaxMajorInput: "0.00" }));
    expect(result.ok).toBe(false);
    expect(result.message).toContain("最小金额");
  });

  test("不支持的币种报 CURRENCY_NOT_SUPPORTED", async () => {
    const wrapper = await mountEditor();
    const vm = wrapper.vm as unknown as EditorVm;
    const result = vm.normalizeRow(editable({ currency: "GBP" }));
    expect(result.ok).toBe(false);
    expect(result.message).toContain("CURRENCY_NOT_SUPPORTED");
  });

  test("舍入模式：floor/ceil/truncate/half_up", async () => {
    const wrapper = await mountEditor();
    const vm = wrapper.vm as unknown as EditorVm;
    expect(vm.normalizeAmount(10.9, "floor")).toBe(10);
    expect(vm.normalizeAmount(10.1, "ceil")).toBe(11);
    expect(vm.normalizeAmount(10.9, "truncate")).toBe(10);
    expect(vm.normalizeAmount(-10.9, "truncate")).toBe(-10);
    expect(vm.normalizeAmount(10.5, "half_up")).toBe(11);
  });

  test("published 版本只读（不依赖权限）", async () => {
    const wrapper = await mountEditor({ versionStatus: "published" });
    const vm = wrapper.vm as unknown as EditorVm;
    expect(vm.readonlyByStatus).toBe(true);
    expect(vm.readonly).toBe(true);
  });

  test("archived 版本只读（CR 修复点回归）", async () => {
    const wrapper = await mountEditor({ versionStatus: "archived" });
    const vm = wrapper.vm as unknown as EditorVm;
    expect(vm.readonlyByStatus).toBe(true);
    expect(vm.readonly).toBe(true);
  });

  test("draft 版本无 cashier.write 权限时只读", async () => {
    const wrapper = await mountEditor({ versionStatus: "draft", perms: ["cashier.read"] });
    const vm = wrapper.vm as unknown as EditorVm;
    expect(vm.readonlyByStatus).toBe(false);
    expect(vm.readonly).toBe(true);
  });

  test("加载行时 taxRate 字符串响应被解析为数字（CR 修复点回归）", async () => {
    getCashierPriceRows.mockResolvedValue({
      items: [
        {
          countryCode: "US",
          regionCode: "*",
          currency: "USD",
          priceId: "p1",
          preTaxAmountMinor: 999,
          taxRate: "0.085",
          taxAmountMinor: 85,
          afterTaxAmountMinor: 1084,
          effectiveAt: "2026-01-01T00:00:00Z"
        }
      ]
    });
    const wrapper = await mountEditor();
    const vm = wrapper.vm as unknown as EditorVm;
    expect(vm.rows[0].taxRate).toBeCloseTo(0.085, 6);
    expect(vm.rows[0].preTaxMajorInput).toBe("9.99");
  });

  test("空版本/空模板时清空行且不请求接口", async () => {
    setActivePinia(createPinia());
    usePermissionStore().setFromUser({ roles: [], permissions: ["cashier.write"] });
    const wrapper = mount(PriceMatrixEditor, {
      props: { templateId: "", version: "", versionStatus: "draft" as VersionStatus },
      global: { directives: { perm: permDirective } }
    });
    await flushPromises();
    expect(getCashierPriceRows).not.toHaveBeenCalled();
    expect((wrapper.vm as unknown as EditorVm).rows).toEqual([]);
  });

  test("非法行阻断保存，不发起 PUT", async () => {
    const wrapper = await mountEditor();
    const vm = wrapper.vm as unknown as EditorVm & { rows: EditableRow[]; saveRows: () => Promise<void> };
    vm.rows.push(editable({ currency: "JPY", preTaxMajorInput: "9.99" }));
    await (wrapper.vm as unknown as { saveRows: () => Promise<void> }).saveRows();
    expect(putCashierPriceRows).not.toHaveBeenCalled();
  });

  test("合法行保存时下发 major 金额字符串 + taxRate 字符串（归一化交后端）并刷新", async () => {
    const wrapper = await mountEditor();
    const vm = wrapper.vm as unknown as { rows: EditableRow[]; saveRows: () => Promise<void> };
    vm.rows.push(editable({ countryCode: "us", priceId: " com.game.pack.001 ", preTaxMajorInput: "9.99", taxRate: 0.1 }));
    await nextTick();
    await vm.saveRows();
    await flushPromises();
    expect(putCashierPriceRows).toHaveBeenCalledTimes(1);
    const payload = putCashierPriceRows.mock.calls[0][2];
    // 契约对齐后端 upsertRowsRequest：preTaxAmount(major 字符串) + taxRate(字符串)，无 *_minor 字段。
    expect(payload.rows[0]).toMatchObject({
      countryCode: "US",
      regionCode: "*",
      currency: "USD",
      priceId: "com.game.pack.001",
      preTaxAmount: "9.99",
      taxRate: "0.1"
    });
    expect(payload.rows[0].preTaxAmountMinor).toBeUndefined();
    expect(wrapper.emitted("saved")).toBeTruthy();
  });
});
