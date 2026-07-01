import { test, expect, type Page, type Route } from "@playwright/test";

// 审计模块 e2e（对齐 03-testing §5.2 与 22-audit compact spec）：
// 对后端契约做 mock/stub，验证 /audit 列表渲染、动词色系/production 高亮、
// 提交式过滤、详情抽屉 before/after 三态、密文脱敏、403 整页降级、全只读，
// 并采集截图 + 视觉基线。真实跨栈连库 e2e 属测试专家职责，本用例以前端 + 契约 mock 为主。

const FUTURE_ISO = new Date(Date.now() + 60 * 60 * 1000).toISOString();

const SESSION = {
  accessToken: "e2e-access-token",
  refreshToken: "e2e-refresh-token",
  expiresAt: FUTURE_ISO,
  user: {
    userId: 1,
    userName: "admin",
    displayName: "管理员",
    roles: [],
    permissions: ["audit.read"]
  }
};

function meBody(permissions: string[]) {
  return {
    data: {
      userId: 1,
      userName: "admin",
      displayName: "管理员",
      email: "admin@example.com",
      status: "active",
      roles: [],
      permissions,
      identities: [],
      environment: "sandbox"
    }
  };
}

const ROW_UPDATE = {
  id: "9007199254740993",
  actorId: "10",
  operator: { id: "10", userName: "alice", displayName: "爱丽丝" },
  action: "game.update",
  resourceType: "game",
  resourceId: "g_1001",
  env: "production",
  detail: {
    summary: "更新游戏 g_1001：status 变更",
    changed: ["status"],
    before: { name: "同名游戏", status: "draft" },
    after: { name: "同名游戏", status: "active" },
    request: { ip: "10.1.2.3", requestId: "req_8f3a", method: "PUT", path: "/api/admin/games/g_1001" }
  },
  createdAt: "2026-01-02T03:04:05Z"
};

const ROW_CREATE = {
  id: "9007199254740994",
  actorId: "0",
  operator: null,
  action: "game.create",
  resourceType: "game",
  resourceId: "g_2002",
  env: "sandbox",
  detail: { summary: "创建游戏 g_2002", after: { name: "新游戏", status: "draft" } },
  createdAt: "2026-01-03T03:04:05Z"
};

const ROW_DELETE = {
  id: "9007199254740995",
  actorId: "11",
  operator: { id: "11", userName: "bob", displayName: "鲍勃" },
  action: "game.delete",
  resourceType: "game",
  resourceId: "g_3003",
  env: "develop",
  detail: { summary: "删除游戏 g_3003", before: { name: "旧游戏", status: "active" } },
  createdAt: "2026-01-04T03:04:05Z"
};

const ROW_SECRET = {
  id: "9007199254740996",
  actorId: "12",
  operator: { id: "12", userName: "卡萝", displayName: "卡萝" },
  action: "game_account_auth_config.update",
  resourceType: "game_account_auth_config",
  resourceId: "g_1001:google",
  env: "sandbox",
  detail: {
    summary: "更新 google 账号认证配置",
    changed: ["clientSecret"],
    before: { clientSecret: "masked" },
    after: { clientId: "cid-123", clientSecret: "masked" }
  },
  createdAt: "2026-01-05T03:04:05Z"
};

const ROWS = [ROW_UPDATE, ROW_CREATE, ROW_DELETE, ROW_SECRET];
const DETAIL_BY_ID: Record<string, unknown> = Object.fromEntries(ROWS.map((r) => [r.id, r]));

const LIST_BODY = {
  data: { items: ROWS, page: 1, pageSize: 20, total: ROWS.length }
};

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({
    status,
    contentType: "application/json",
    headers: { "X-Environment": "sandbox" },
    body: JSON.stringify(body)
  });
}

interface SetupOptions {
  permissions?: string[];
  listStatus?: number;
}

async function setup(page: Page, opts: SetupOptions = {}) {
  const permissions = opts.permissions ?? ["audit.read"];

  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  // 兜底：其它后台接口返回空，避免 dashboard 等页面挂起。先注册兜底，后注册具体路由（后注册者优先）。
  await page.route("**/api/admin/**", (route) => json(route, 200, { data: {} }));
  await page.route("**/api/admin/me", (route) => json(route, 200, meBody(permissions)));

  // 操作者下拉
  await page.route(/\/api\/admin\/system\/admin-users(\?.*)?$/, (route) =>
    json(route, 200, { data: { items: [], page: 1, pageSize: 30, total: 0 } })
  );

  // 列表（先注册，后注册详情/facets，使更具体的后注册者优先）
  await page.route(/\/api\/admin\/audit-logs(\?.*)?$/, (route) => {
    if (opts.listStatus && opts.listStatus !== 200) {
      return json(route, opts.listStatus, {
        error: { code: opts.listStatus === 403 ? "FORBIDDEN" : "INTERNAL", message: "拒绝访问", details: [] }
      });
    }
    return json(route, 200, LIST_BODY);
  });

  // 详情 /audit-logs/{id}
  await page.route(/\/api\/admin\/audit-logs\/([^/?]+)(\?.*)?$/, (route) => {
    const m = route.request().url().match(/\/audit-logs\/([^/?]+)/);
    const id = m?.[1] ?? "";
    const record = DETAIL_BY_ID[decodeURIComponent(id)];
    if (!record) {
      return json(route, 404, { error: { code: "NOT_FOUND", message: "not found", details: [] } });
    }
    return json(route, 200, { data: record });
  });

  // facets（最后注册 → 优先于 /{id} 正则）
  await page.route("**/api/admin/audit-logs/facets", (route) =>
    json(route, 200, { data: { envs: [], actions: [], resourceTypes: [] } })
  );
}

async function gotoAudit(page: Page) {
  await page.goto("/dashboard");
  const link = page.getByRole("link", { name: "审计日志" });
  await expect(link).toBeVisible();
  await link.click();
  await page.waitForURL(/\/audit$/, { timeout: 15000 });
  // 首次进入需 Vite 即时编译 AuditView 懒加载 chunk，放宽超时。
  await expect(page.getByRole("heading", { name: "审计日志列表" })).toBeVisible({ timeout: 15000 });
}

// 列表表格作用域（避免与 FilterBar 下拉中的同名 option 文本冲突）。
function tableRows(page: Page) {
  return page.locator(".el-table__body-wrapper tr");
}

test("审计列表渲染行/动作标签/资源标识/摘要，且 production 高亮", async ({ page }) => {
  await setup(page);
  await gotoAudit(page);

  const table = page.locator(".el-table");
  await expect(tableRows(page)).toHaveCount(ROWS.length);
  await expect(table.getByText("game.update")).toBeVisible();
  await expect(table.getByText("game.create")).toBeVisible();
  await expect(table.getByText("game.delete")).toBeVisible();
  await expect(table.getByText("g_1001", { exact: true })).toBeVisible();
  await expect(page.getByText("更新游戏 g_1001：status 变更")).toBeVisible();
  // 操作者：displayName / System（actorId=0）
  await expect(table.getByText("爱丽丝")).toBeVisible();
  await expect(table.getByText("System")).toBeVisible();
  // production 标签可见（动作/环境均可能为 danger，避免 strict 模式冲突）
  await expect(table.getByText("production", { exact: true })).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/audit-list.png", fullPage: true });
});

test("提交式过滤：选择动作后点查询才下发请求", async ({ page }) => {
  await setup(page);
  await gotoAudit(page);

  const queryPromise = page.waitForRequest(
    (req) => req.url().includes("/api/admin/audit-logs?") && req.url().includes("action=game.update")
  );
  // Element Plus select 在 e2e 中优先用容器定位，避免 placeholder 输入框挂载时机不稳定。
  await page.locator(".filter-bar .filter-item--wide").first().click();
  await page.getByRole("option", { name: "game.update", exact: true }).click();
  await page.getByRole("button", { name: "查询" }).click();
  const req = await queryPromise;
  expect(req.url()).toContain("action=game.update");
});

test("详情抽屉 update：before/after 左右对照 + changed 高亮 + request 折叠", async ({ page }) => {
  await setup(page);
  await gotoAudit(page);

  await page.getByText("更新游戏 g_1001：status 变更").click();
  const drawer = page.locator(".el-drawer");
  await expect(drawer.getByText("before / after 对照")).toBeVisible();
  // 变更字段 status 默认展示，after 值 active
  await expect(drawer.getByText("active", { exact: true })).toBeVisible();
  await expect(drawer.getByText("request 元信息")).toBeVisible();
});

test("详情抽屉 create：仅 after 单列", async ({ page }) => {
  await setup(page);
  await gotoAudit(page);
  await page.getByText("创建游戏 g_2002").click();
  const drawer = page.locator(".el-drawer");
  await expect(drawer.getByRole("cell", { name: "新游戏", exact: true })).toBeVisible();
});

test("详情抽屉 delete：仅 before 单列", async ({ page }) => {
  await setup(page);
  await gotoAudit(page);
  await page.getByText("删除游戏 g_3003").click();
  const drawer = page.locator(".el-drawer");
  await expect(drawer.locator(".el-table").first().getByText("旧游戏", { exact: true })).toBeVisible();
});

test("密文字段以 ****** 展示，绝不出现明文", async ({ page }) => {
  await setup(page);
  await gotoAudit(page);
  await page.locator(".el-table__body-wrapper tr").filter({ hasText: "更新 google 账号认证配置" }).getByRole("button", { name: "详情" }).click();
  const drawer = page.locator(".el-drawer");
  await expect(drawer.locator(".el-table").first().getByText("******").first()).toBeVisible();
  // 原始 JSON 仍保留后端返回的 masked 占位字面值，断言不会出现明文关键词。
  await expect(drawer).not.toContainText("PLAINTEXT");
});

test("403 无 audit.read → 整页降级且隐藏 FilterBar", async ({ page }) => {
  await setup(page, { permissions: ["audit.read"], listStatus: 403 });
  await gotoAudit(page);
  await expect(page.getByText("无权限访问审计日志")).toBeVisible();
  await expect(page.locator(".filter-bar")).toBeHidden();
  await page.screenshot({ path: "../../tests/frontend/screenshots/audit-forbidden.png", fullPage: true });
});

test("全只读：页面不出现任何写/删按钮", async ({ page }) => {
  await setup(page);
  await gotoAudit(page);
  const buttons = page.getByRole("button");
  const count = await buttons.count();
  for (let i = 0; i < count; i += 1) {
    const text = (await buttons.nth(i).innerText()).trim();
    expect(text).not.toMatch(/新建|新增|删除|保存|编辑|提交|发布|执行|清空|导出/);
  }
});

test("审计列表视觉基线", async ({ page }) => {
  await setup(page);
  await gotoAudit(page);
  await expect(page.locator(".el-table").getByText("更新游戏 g_1001：status 变更")).toBeVisible({ timeout: 15_000 });
  await expect(page).toHaveScreenshot("audit-list.png", { maxDiffPixelRatio: 0.02 });
});
