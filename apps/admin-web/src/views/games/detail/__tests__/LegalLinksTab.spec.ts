import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

const replaceLegalLinksApi = vi.fn();
vi.mock("@/api/modules/games", () => ({
  replaceLegalLinks: (...args: unknown[]) => replaceLegalLinksApi(...args)
}));

import { ApiError } from "@/api/http";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";
import LegalLinksTab from "@/views/games/detail/LegalLinksTab.vue";
import type { GameDetail } from "@/api/modules/games";

function makeGame(): GameDetail {
  return {
    gameId: "100001",
    name: "测试游戏",
    alias: "demo",
    iconUrl: "",
    status: "active",
    defaultMarketCode: "GLOBAL",
    gameSecret: "masked",
    secretMasked: true,
    markets: [{ marketCode: "GLOBAL", isDefault: true, enabled: true, defaultLocale: "en-US" }],
    legalLinks: [
      { scopeType: "default", scopeValue: "*", termsUrl: "https://t", privacyUrl: "https://p", deleteAccountUrl: "" }
    ],
    createdAt: "",
    updatedAt: ""
  };
}

interface LegalRow {
  scopeType: string;
  scopeValue: string;
  termsUrl: string;
  privacyUrl: string;
  deleteAccountUrl: string;
}

interface LegalVM {
  drawerVisible: boolean;
  formError: string;
  rows: LegalRow[];
  openEdit: () => void;
  addRow: () => void;
  removeRow: (i: number) => void;
  onScopeTypeChange: (row: LegalRow) => void;
  submit: () => Promise<void>;
}

function mountTab(perms: string[] = ["game.write"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  const wrapper = mount(LegalLinksTab, {
    props: { game: makeGame() },
    global: { directives: { perm: permDirective } }
  });
  return { wrapper, vm: wrapper.vm as unknown as LegalVM };
}

describe("LegalLinksTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  test("scopeType 联动 scopeValue：default 锁 '*'，切 market/locale 清空", () => {
    const { vm } = mountTab();
    vm.openEdit();
    vm.addRow();
    const row = vm.rows[vm.rows.length - 1];
    expect(row.scopeType).toBe("default");
    expect(row.scopeValue).toBe("*");

    row.scopeType = "market";
    vm.onScopeTypeChange(row);
    expect(row.scopeValue).toBe("");

    row.scopeType = "locale";
    vm.onScopeTypeChange(row);
    expect(row.scopeValue).toBe("");

    row.scopeType = "default";
    vm.onScopeTypeChange(row);
    expect(row.scopeValue).toBe("*");
  });

  test("market 作用域取值非法被前端拦截", async () => {
    const { vm } = mountTab();
    vm.openEdit();
    vm.rows.splice(0, vm.rows.length, {
      scopeType: "market",
      scopeValue: "NOT_A_MARKET",
      termsUrl: "",
      privacyUrl: "",
      deleteAccountUrl: ""
    });
    await vm.submit();
    expect(vm.formError).toContain("合法市场");
    expect(replaceLegalLinksApi).not.toHaveBeenCalled();
  });

  test("locale 作用域需合法语言标签", async () => {
    const { vm } = mountTab();
    vm.openEdit();
    vm.rows.splice(0, vm.rows.length, {
      scopeType: "locale",
      scopeValue: "bad_locale",
      termsUrl: "",
      privacyUrl: "",
      deleteAccountUrl: ""
    });
    await vm.submit();
    expect(vm.formError).toContain("语言标签");
    expect(replaceLegalLinksApi).not.toHaveBeenCalled();
  });

  test("(scopeType, scopeValue) 重复被拦截", async () => {
    const { vm } = mountTab();
    vm.openEdit();
    vm.rows.splice(
      0,
      vm.rows.length,
      { scopeType: "market", scopeValue: "JP", termsUrl: "", privacyUrl: "", deleteAccountUrl: "" },
      { scopeType: "market", scopeValue: "JP", termsUrl: "", privacyUrl: "", deleteAccountUrl: "" }
    );
    await vm.submit();
    expect(vm.formError).toContain("重复");
    expect(replaceLegalLinksApi).not.toHaveBeenCalled();
  });

  test("default 作用域至多一条", async () => {
    const { vm } = mountTab();
    vm.openEdit();
    vm.rows.splice(
      0,
      vm.rows.length,
      { scopeType: "default", scopeValue: "*", termsUrl: "", privacyUrl: "", deleteAccountUrl: "" },
      { scopeType: "default", scopeValue: "*", termsUrl: "", privacyUrl: "", deleteAccountUrl: "" }
    );
    await vm.submit();
    // 先命中重复键（default:*），同样属于非法配置被拦
    expect(vm.formError).toBeTruthy();
    expect(replaceLegalLinksApi).not.toHaveBeenCalled();
  });

  test("合法多行提交：default 强制 '*' 全量覆盖，回传写入结果", async () => {
    const links = [
      { scopeType: "default", scopeValue: "*", termsUrl: "https://t", privacyUrl: "https://p", deleteAccountUrl: "" },
      { scopeType: "market", scopeValue: "JP", termsUrl: "", privacyUrl: "", deleteAccountUrl: "" },
      { scopeType: "locale", scopeValue: "zh-CN", termsUrl: "", privacyUrl: "", deleteAccountUrl: "" }
    ];
    replaceLegalLinksApi.mockResolvedValue({ legalLinks: links });
    const { wrapper, vm } = mountTab();
    vm.openEdit();
    vm.rows.splice(
      0,
      vm.rows.length,
      { scopeType: "default", scopeValue: "ignored", termsUrl: "https://t", privacyUrl: "https://p", deleteAccountUrl: "" },
      { scopeType: "market", scopeValue: "JP", termsUrl: "", privacyUrl: "", deleteAccountUrl: "" },
      { scopeType: "locale", scopeValue: "zh-CN", termsUrl: "", privacyUrl: "", deleteAccountUrl: "" }
    );
    await vm.submit();
    await flushPromises();

    expect(replaceLegalLinksApi).toHaveBeenCalledTimes(1);
    const [gameId, payload] = replaceLegalLinksApi.mock.calls[0];
    expect(gameId).toBe("100001");
    const defaultItem = payload.legalLinks.find((l: { scopeType: string }) => l.scopeType === "default");
    expect(defaultItem.scopeValue).toBe("*");
    const emitted = wrapper.emitted("updated");
    expect(emitted).toBeTruthy();
    expect((emitted![0][0] as GameDetail).legalLinks).toEqual(links);
    expect(vm.drawerVisible).toBe(false);
  });

  test("保存失败展示后端错误消息", async () => {
    replaceLegalLinksApi.mockRejectedValue(new ApiError(400, "VALIDATION_FAILED", "invalid scope value"));
    const { vm } = mountTab();
    vm.openEdit();
    vm.rows.splice(0, vm.rows.length, {
      scopeType: "default",
      scopeValue: "*",
      termsUrl: "",
      privacyUrl: "",
      deleteAccountUrl: ""
    });
    await vm.submit();
    await flushPromises();
    expect(vm.formError).toBe("invalid scope value");
  });

  test("无 game.write 权限时编辑按钮被置灰禁用", () => {
    const { wrapper } = mountTab([]);
    const btn = wrapper.find(".legal-tab__toolbar button");
    expect(btn.attributes("disabled")).toBe("disabled");
  });
});
