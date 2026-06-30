import { defineStore } from "pinia";
import { request } from "@/api/http";
import type { CurrencySpec } from "@/api/modules/products";

const fallbackCurrencySpecs: CurrencySpec[] = [
  {
    currencyCode: "USD",
    currencyName: "US Dollar",
    decimalPlaces: 2,
    minAmountMinor: 1,
    roundingMode: "half_up",
    enabled: true
  },
  {
    currencyCode: "JPY",
    currencyName: "Japanese Yen",
    decimalPlaces: 0,
    minAmountMinor: 1,
    roundingMode: "half_up",
    enabled: true
  },
  {
    currencyCode: "KRW",
    currencyName: "Korean Won",
    decimalPlaces: 0,
    minAmountMinor: 1,
    roundingMode: "half_up",
    enabled: true
  },
  {
    currencyCode: "TWD",
    currencyName: "New Taiwan Dollar",
    decimalPlaces: 0,
    minAmountMinor: 1,
    roundingMode: "half_up",
    enabled: true
  },
  {
    currencyCode: "EUR",
    currencyName: "Euro",
    decimalPlaces: 2,
    minAmountMinor: 1,
    roundingMode: "half_up",
    enabled: true
  }
];

export const useDictionaryStore = defineStore("dictionary", {
  state: () => ({
    currencySpecs: [] as CurrencySpec[],
    loaded: false,
    loading: false
  }),
  getters: {
    enabledCurrencySpecs: (state): CurrencySpec[] => {
      const source = state.currencySpecs.length ? state.currencySpecs : fallbackCurrencySpecs;
      return source.filter((item) => item.enabled !== false);
    }
  },
  actions: {
    getCurrencySpec(code: string): CurrencySpec | undefined {
      return this.enabledCurrencySpecs.find((item) => item.currencyCode === code);
    },
    async ensureCurrencySpecs(force = false) {
      if (this.loading) {
        return;
      }
      if (this.loaded && !force) {
        return;
      }
      this.loading = true;
      try {
        const res = await request<{ items: CurrencySpec[] }>("/api/admin/system/currency-specs");
        this.currencySpecs = (res.items ?? []).map((item) => ({
          ...item,
          currencyCode: String(item.currencyCode || "").toUpperCase()
        }));
      } catch {
        this.currencySpecs = [...fallbackCurrencySpecs];
      } finally {
        this.loaded = true;
        this.loading = false;
      }
    }
  }
});
