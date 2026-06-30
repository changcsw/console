import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

// 避免加载真实路由图（@/api/http 默认导入 @/router）。
vi.mock("@/router", () => ({
  default: { currentRoute: { value: { path: "/audit", fullPath: "/audit" } }, push: vi.fn() }
}));

// 审计 API 契约 mock/stub（对齐 22-audit compact spec）。
const listAuditLogsApi = vi.fn();
const getAuditLogDetailApi = vi.fn();
const listAuditLogFacetsApi = vi.fn();
vi.mock("@/api/modules/audit", () => ({
  listAuditLogs: (...args: unknown[]) => listAuditLogsApi(...args),
  getAuditLogDetail: (...args: unknown[]) => getAuditLogDetailApi(...args),
  listAuditLogFacets: (...args: unknown[]) => listAuditLogFacetsApi(...args)
}));

// 操作者下拉依赖的管理员列表接口。
const listAdminUsersApi = vi.fn();
vi.mock("@/api/modules/system", () => ({
  listAdminUsers: (...args: unknown[]) => listAdminUsersApi(...args)
}));

import { ApiError } from "@/api/http";
import { usePermissionStore } from "@/stores/permission";
import { useAppStore } from "@/stores/app";
import AuditView from "@/views/audit/AuditView.vue";
import type { AuditLogItem } from "@/api/modules/audit";

function logItem(overrides: Partial<AuditLogItem> = {}): AuditLogItem {
  return {
    id: "9007199254740993", // 大整数：必须以字符串回传，验证 JS 精度无损
    actorId: "10",
    operator: { id: "10", userName: "alice", displayName: "爱丽丝" },
    action: "game.update",
    resourceType: "game",
    resourceId: "g_1001",
    env: "sandbox",
    detail: { summary: "更新游戏 g_1001" },
    createdAt: "2026-01-02T03:04:05Z",
    ...overrides
  };
}

interface AuditVM {
  rows: AuditLogItem[];
  total: number;
  page: number;
  pageSize: number;
  sort: "createdAt" | "-createdAt";
  pageState: "ready" | "error" | "forbidden";
  errorMessage: string;
  draftFilters: {
    env: string;
    action: string;
    resourceType: string;
    operator: string;
    keyword: string;
    from: string;
    to: string;
  };
  draftTimeRange: [Date, Date] | [];
  detailRecord: AuditLogItem | null;
  detailError: string;
  hasBefore: boolean;
  hasAfter: boolean;
  compareRows: { key: string; beforeText: string; afterText: string; changed: boolean }[];
  displayCompareRows: { key: string; beforeText: string; afterText: string; changed: boolean }[];
  singleAfterRows: { key: string; valueText: string }[];
  singleBeforeRows: { key: string; valueText: string }[];
  showUnchanged: boolean;
  reload: (p?: number) => Promise<void>;
  submitFilters: () => void;
  resetFilters: () => void;
  onSortChange: (p: { prop: string; order: "ascending" | "descending" | null }) => void;
  openDetail: (row: AuditLogItem) => Promise<void>;
  actionTagType: (action: string) => string | undefined;
  envTagType: (env: string) => string | undefined;
  formatOperator: (item: AuditLogItem) => string;
}

function mountView(perms: string[] = ["audit.read"]) {
  setActivePinia(createPinia());
  usePermissionStore().setFromUser({ roles: [], permissions: perms });
  useAppStore().environment = "sandbox";
  const wrapper = mount(AuditView, {
    global: {
      stubs: { EnvironmentBadge: true }
    }
  });
  return { wrapper, vm: wrapper.vm as unknown as AuditVM };
}

function pageOf(items: AuditLogItem[], page = 1, pageSize = 20, total = items.length) {
  return { items, page, pageSize, total };
}

beforeEach(() => {
  vi.clearAllMocks();
  listAuditLogsApi.mockResolvedValue(pageOf([logItem()]));
  getAuditLogDetailApi.mockImplementation(async (id: string) => logItem({ id }));
  listAuditLogFacetsApi.mockResolvedValue({ envs: [], actions: [], resourceTypes: [] });
  listAdminUsersApi.mockResolvedValue({ items: [], page: 1, pageSize: 30, total: 0 });
});

describe("AuditView 挂载与列表渲染", () => {
  test("挂载后加载第 1 页并渲染列（动作/资源/摘要/操作者）", async () => {
    const { wrapper, vm } = mountView();
    await flushPromises();

    expect(listAuditLogsApi).toHaveBeenCalledTimes(1);
    expect(listAuditLogsApi.mock.calls[0][0]).toMatchObject({ page: 1, pageSize: 20, sort: "-createdAt" });
    expect(vm.rows).toHaveLength(1);
    expect(wrapper.text()).toContain("game.update");
    expect(wrapper.text()).toContain("g_1001");
    expect(wrapper.text()).toContain("更新游戏 g_1001");
    // 操作者展示 displayName
    expect(wrapper.text()).toContain("爱丽丝");
  });

  test("actorId=0 操作者展示 System；displayName 缺失兜底 actorId", async () => {
    const { vm } = mountView();
    await flushPromises();
    expect(vm.formatOperator(logItem({ actorId: "0", operator: null }))).toBe("System");
    expect(vm.formatOperator(logItem({ actorId: "42", operator: null }))).toBe("42");
    expect(vm.formatOperator(logItem({ actorId: "42", operator: { id: "42", userName: "bob", displayName: "鲍勃" } }))).toBe("鲍勃");
  });

  test("大整数 id 以字符串无损渲染（避免 JS 精度问题）", async () => {
    const { wrapper } = mountView();
    await flushPromises();
    expect(wrapper.text()).not.toContain("9007199254740992");
  });
});

describe("动词色系与 production 高亮", () => {
  test("actionTagType 按动词映射色系", async () => {
    const { vm } = mountView();
    await flushPromises();
    expect(vm.actionTagType("game.create")).toBe("success");
    expect(vm.actionTagType("game.delete")).toBe("danger");
    expect(vm.actionTagType("cashier_price_template_version.publish")).toBe("primary");
    expect(vm.actionTagType("sync.execute")).toBe("warning");
    expect(vm.actionTagType("game_channel.hide")).toBe("info");
    expect(vm.actionTagType("game.update")).toBe("info");
  });

  test("envTagType production 高亮警示（danger）", async () => {
    const { vm } = mountView();
    await flushPromises();
    expect(vm.envTagType("production")).toBe("danger");
    expect(vm.envTagType("sandbox")).toBe("warning");
    expect(vm.envTagType("develop")).toBe("success");
  });

  test("列表中 production 行渲染 danger 标签", async () => {
    listAuditLogsApi.mockResolvedValue(pageOf([logItem({ env: "production" })]));
    const { wrapper } = mountView();
    await flushPromises();
    expect(wrapper.find(".el-tag--danger").exists()).toBe(true);
  });
});

describe("FilterBar 提交式查询 + 切页保留过滤", () => {
  test("仅 submitFilters 才发请求，修改 draft 不发请求", async () => {
    const { vm } = mountView();
    await flushPromises();
    listAuditLogsApi.mockClear();

    vm.draftFilters.env = "production";
    vm.draftFilters.action = "game.update";
    vm.draftFilters.resourceType = "game";
    vm.draftFilters.keyword = "  g_1001  ";
    await flushPromises();
    expect(listAuditLogsApi).not.toHaveBeenCalled();

    vm.submitFilters();
    await flushPromises();
    expect(listAuditLogsApi).toHaveBeenCalledTimes(1);
    expect(listAuditLogsApi.mock.calls[0][0]).toMatchObject({
      env: "production",
      action: "game.update",
      resourceType: "game",
      keyword: "g_1001", // trim 生效
      page: 1
    });
  });

  test("切页(reload)保留已提交过滤条件", async () => {
    const { vm } = mountView();
    await flushPromises();

    vm.draftFilters.action = "game.delete";
    vm.submitFilters();
    await flushPromises();
    listAuditLogsApi.mockClear();
    listAuditLogsApi.mockResolvedValue(pageOf([logItem()], 2, 20, 50));

    await vm.reload(2);
    await flushPromises();
    expect(listAuditLogsApi.mock.calls.at(-1)?.[0]).toMatchObject({ action: "game.delete", page: 2 });
    expect(vm.page).toBe(2);
  });

  test("切换排序(onSortChange)回到第 1 页但保留过滤", async () => {
    const { vm } = mountView();
    await flushPromises();
    vm.draftFilters.resourceType = "game";
    vm.submitFilters();
    await flushPromises();
    listAuditLogsApi.mockClear();

    vm.onSortChange({ prop: "createdAt", order: "ascending" });
    await flushPromises();
    expect(vm.sort).toBe("createdAt");
    expect(listAuditLogsApi.mock.calls.at(-1)?.[0]).toMatchObject({
      resourceType: "game",
      sort: "createdAt",
      page: 1
    });
  });

  test("空过滤条件不下发空字符串参数", async () => {
    const { vm } = mountView();
    await flushPromises();
    const q = listAuditLogsApi.mock.calls[0][0] as Record<string, unknown>;
    expect(q.env).toBeUndefined();
    expect(q.action).toBeUndefined();
    expect(q.resourceType).toBeUndefined();
    expect(q.operator).toBeUndefined();
    expect(q.keyword).toBeUndefined();
    expect(q.from).toBeUndefined();
    expect(q.to).toBeUndefined();
  });

  test("operator 选中后以 actor_id 提交", async () => {
    const { vm } = mountView();
    await flushPromises();
    listAuditLogsApi.mockClear();
    vm.draftFilters.operator = "10";
    vm.submitFilters();
    await flushPromises();
    expect(listAuditLogsApi.mock.calls.at(-1)?.[0]).toMatchObject({ operator: "10", page: 1 });
  });

  test("timeRange 转 ISO-8601 并下发 from/to", async () => {
    const { vm } = mountView();
    await flushPromises();
    listAuditLogsApi.mockClear();
    const start = new Date("2026-01-01T00:00:00Z");
    const end = new Date("2026-01-31T23:59:59Z");
    vm.draftTimeRange = [start, end];
    vm.submitFilters();
    await flushPromises();
    const q = listAuditLogsApi.mock.calls[0][0] as Record<string, unknown>;
    expect(q.from).toBe(start.toISOString());
    expect(q.to).toBe(end.toISOString());
  });

  test("resetFilters 清空 draft 并重新查询第 1 页", async () => {
    const { vm } = mountView();
    await flushPromises();
    vm.draftFilters.action = "game.delete";
    vm.draftFilters.keyword = "x";
    vm.submitFilters();
    await flushPromises();
    listAuditLogsApi.mockClear();

    vm.resetFilters();
    await flushPromises();
    expect(vm.draftFilters.action).toBe("");
    expect(vm.draftFilters.keyword).toBe("");
    const q = listAuditLogsApi.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(q.action).toBeUndefined();
    expect(q.page).toBe(1);
  });
});

describe("详情抽屉 before/after 三态", () => {
  test("create：仅 after 单列展示", async () => {
    const row = logItem({
      action: "game.create",
      detail: { summary: "创建", after: { name: "新游戏", status: "draft" } }
    });
    getAuditLogDetailApi.mockResolvedValue(row);
    const { vm } = mountView();
    await flushPromises();

    await vm.openDetail(row);
    await flushPromises();
    expect(vm.hasAfter).toBe(true);
    expect(vm.hasBefore).toBe(false);
    expect(vm.singleAfterRows.map((r) => r.key)).toEqual(["name", "status"]);
  });

  test("delete：仅 before 单列展示", async () => {
    const row = logItem({
      action: "game.delete",
      detail: { summary: "删除", before: { name: "旧游戏" } }
    });
    getAuditLogDetailApi.mockResolvedValue(row);
    const { vm } = mountView();
    await flushPromises();

    await vm.openDetail(row);
    await flushPromises();
    expect(vm.hasBefore).toBe(true);
    expect(vm.hasAfter).toBe(false);
    expect(vm.singleBeforeRows.map((r) => r.key)).toEqual(["name"]);
  });

  test("update：左右对照 + changed 高亮，默认仅看变更字段", async () => {
    const row = logItem({
      action: "game.update",
      detail: {
        summary: "更新",
        changed: ["status"],
        before: { name: "同名", status: "draft" },
        after: { name: "同名", status: "active" }
      }
    });
    getAuditLogDetailApi.mockResolvedValue(row);
    const { vm } = mountView();
    await flushPromises();

    await vm.openDetail(row);
    await flushPromises();
    expect(vm.hasBefore).toBe(true);
    expect(vm.hasAfter).toBe(true);

    const statusRow = vm.compareRows.find((r) => r.key === "status");
    const nameRow = vm.compareRows.find((r) => r.key === "name");
    expect(statusRow?.changed).toBe(true);
    expect(nameRow?.changed).toBe(false);

    // 默认 showUnchanged=false → 仅展示变更字段
    expect(vm.displayCompareRows.map((r) => r.key)).toEqual(["status"]);
    vm.showUnchanged = true;
    await flushPromises();
    expect(vm.displayCompareRows.map((r) => r.key)).toEqual(["name", "status"]);
  });
});

describe("脱敏展示", () => {
  test("密文字段以 ****** 展示，绝不解密", async () => {
    const row = logItem({
      action: "game_account_auth_config.update",
      detail: {
        summary: "更新配置",
        after: { clientId: "cid-123", clientSecret: "masked", token: "******" }
      }
    });
    getAuditLogDetailApi.mockResolvedValue(row);
    const { vm } = mountView();
    await flushPromises();

    await vm.openDetail(row);
    await flushPromises();
    const secret = vm.singleAfterRows.find((r) => r.key === "clientSecret");
    const token = vm.singleAfterRows.find((r) => r.key === "token");
    expect(secret?.valueText).toBe("******");
    expect(token?.valueText).toBe("******");
    // 非密文字段原样展示
    expect(vm.singleAfterRows.find((r) => r.key === "clientId")?.valueText).toBe("cid-123");
  });

  test("详情抽屉渲染 ****** 文案且不出现明文", async () => {
    const row = logItem({
      action: "game_account_auth_config.update",
      detail: { summary: "更新", after: { clientSecret: "masked" } }
    });
    getAuditLogDetailApi.mockResolvedValue(row);
    const { wrapper, vm } = mountView();
    await flushPromises();
    await vm.openDetail(row);
    await flushPromises();
    expect(wrapper.text()).toContain("******");
  });
});

describe("详情接口异常回退", () => {
  test("详情接口非 404 错误 → 设置 detailError 并回退列表快照", async () => {
    const row = logItem();
    getAuditLogDetailApi.mockRejectedValue(new ApiError(500, "INTERNAL", "boom"));
    const { vm } = mountView();
    await flushPromises();
    await vm.openDetail(row);
    await flushPromises();
    expect(vm.detailError).not.toBe("");
    expect(vm.detailRecord?.id).toBe(row.id);
  });

  test("详情接口 404 → 静默回退列表快照，不报错", async () => {
    const row = logItem();
    getAuditLogDetailApi.mockRejectedValue(new ApiError(404, "NOT_FOUND", "not found"));
    const { vm } = mountView();
    await flushPromises();
    await vm.openDetail(row);
    await flushPromises();
    expect(vm.detailError).toBe("");
    expect(vm.detailRecord?.id).toBe(row.id);
  });
});

describe("状态态：空 / 错误 / 403 整页降级", () => {
  test("空态：渲染『当前条件下无审计记录』", async () => {
    listAuditLogsApi.mockResolvedValue(pageOf([], 1, 20, 0));
    const { wrapper, vm } = mountView();
    await flushPromises();
    expect(vm.rows).toHaveLength(0);
    expect(wrapper.text()).toContain("当前条件下无审计记录");
  });

  test("错误态：非 403 错误展示错误信息与重试入口", async () => {
    listAuditLogsApi.mockRejectedValue(new ApiError(500, "INTERNAL", "服务器开小差"));
    const { wrapper, vm } = mountView();
    await flushPromises();
    expect(vm.pageState).toBe("error");
    expect(vm.errorMessage).toBe("服务器开小差");
    expect(wrapper.text()).toContain("审计日志加载失败");
    const retry = wrapper.findAll("button").find((b) => b.text().includes("重试"));
    expect(retry).toBeTruthy();
  });

  test("403：整页降级，隐藏 FilterBar，提示缺少 audit.read", async () => {
    listAuditLogsApi.mockRejectedValue(new ApiError(403, "FORBIDDEN", "forbidden"));
    const { wrapper, vm } = mountView();
    await flushPromises();
    expect(vm.pageState).toBe("forbidden");
    expect(wrapper.text()).toContain("无权限访问审计日志");
    // FilterBar 整体隐藏
    expect(wrapper.find(".filter-bar").exists()).toBe(false);
  });

  test("无 audit.read 权限时任意错误均降级为 forbidden", async () => {
    listAuditLogsApi.mockRejectedValue(new ApiError(500, "INTERNAL", "boom"));
    const { vm } = mountView([]); // 无权限
    await flushPromises();
    expect(vm.pageState).toBe("forbidden");
  });
});

describe("全只读：无任何写/删按钮", () => {
  test("页面按钮仅含只读交互（查询/重置/复制/详情），无写删", async () => {
    const { wrapper } = mountView();
    await flushPromises();
    const labels = wrapper.findAll("button").map((b) => b.text());
    const forbidden = /新建|新增|删除|保存|编辑|提交|发布|执行|清空|导出/;
    for (const label of labels) {
      expect(label).not.toMatch(forbidden);
    }
    // 含必要只读按钮
    expect(labels.some((l) => l.includes("查询"))).toBe(true);
    expect(labels.some((l) => l.includes("重置"))).toBe(true);
  });
});
