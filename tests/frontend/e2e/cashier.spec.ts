import { expect, test, type Page, type Route } from "@playwright/test";

// 收银台模块 e2e（对齐 03-testing §5.2 与 17-cashier-template compact spec）：
// 对 /api/admin/cashier 契约做 mock/stub，验证模板列表/详情、版本生命周期入口、
// 价格矩阵 published 只读、FX 审核 approve/ignore、权限置灰、空态，并采集截图 + 视觉基线。
// 真实跨栈联调（连库 e2e）属测试专家职责，本用例以前端 + 契约 mock 为主。
//
// 注：本机 vite dev 冷编译 + Element Plus 首屏渲染较慢（单页 ~20-30s），
// 故统一抬高单测超时，避免把环境慢当成断言失败。

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
    permissions: ["dashboard.read", "cashier.read", "cashier.write", "cashier.publish", "fx.approve"]
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

const TEMPLATE_LIST = {
  data: {
    items: [
      {
        templateId: "global_default",
        templateName: "Global Default",
        fxSyncEnabled: true,
        fxSyncMode: "manual_confirm",
        fxSyncSchedule: "monthly",
        status: "active"
      },
      {
        templateId: "promo_q1",
        templateName: "Promo Q1",
        fxSyncEnabled: false,
        fxSyncMode: "auto_apply",
        fxSyncSchedule: "quarterly",
        status: "active"
      }
    ],
    page: 1,
    pageSize: 20,
    total: 2
  }
};

const TEMPLATE_DETAIL = {
  data: {
    templateId: "global_default",
    templateName: "Global Default",
    fxSyncEnabled: true,
    fxSyncMode: "manual_confirm",
    fxSyncSchedule: "monthly",
    status: "active",
    versions: [
      { version: "2", status: "draft", sourceType: "manual", publishedAt: null },
      { version: "1", status: "published", sourceType: "manual", publishedAt: "2026-01-01T00:00:00Z" }
    ],
    fxSyncRuns: [
      {
        runId: 501,
        status: "pending_review",
        candidateVersion: "3",
        triggeredAt: "2026-02-01T00:00:00Z",
        reviewedBy: null,
        reviewedAt: null,
        reviewNote: "",
        diffSummary: { added: 2, updated: 1, removed: 0, currencies: ["USD", "JPY"] }
      },
      {
        runId: 500,
        status: "applied",
        candidateVersion: "4",
        triggeredAt: "2026-01-15T00:00:00Z",
        reviewedBy: 1,
        reviewedAt: "2026-01-16T00:00:00Z",
        reviewNote: "ok",
        diffSummary: { added: 1 }
      }
    ]
  }
};

const ROWS_V2 = {
  data: {
    items: [
      {
        countryCode: "US",
        regionCode: "*",
        currency: "USD",
        priceId: "com.game.pack.001",
        preTaxAmountMinor: 999,
        taxRate: 0.085,
        taxAmountMinor: 85,
        afterTaxAmountMinor: 1084,
        effectiveAt: "2026-01-01T00:00:00Z"
      }
    ]
  }
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
  emptyList?: boolean;
  listError?: number;
}

async function setup(page: Page, opts: SetupOptions = {}) {
  const permissions = opts.permissions ?? ["cashier.read", "cashier.write", "cashier.publish", "fx.approve"];

  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  // 兜底：其它后台接口返回空，避免 dashboard 等页面挂起。
  await page.route("**/api/admin/**", (route) => json(route, 200, { data: {} }));
  await page.route("**/api/admin/me", (route) => json(route, 200, meBody(permissions)));

  // cashier 契约：单处理器按 url/method 分支。
  await page.route("**/api/admin/cashier/**", (route) => {
    const url = route.request().url();
    const method = route.request().method();

    if (/\/fx-sync-runs\/\d+\/approve$/.test(url)) {
      return json(route, 200, { data: { runId: 501, status: "applied" } });
    }
    if (/\/fx-sync\/runs$/.test(url)) {
      return json(route, 201, { data: { runId: 502, status: "pending_review", candidateVersion: "5" } });
    }
    if (/\/versions\/[^/]+\/rows$/.test(url)) {
      return json(route, 200, ROWS_V2);
    }
    if (/\/versions\/[^/]+\/copy-to-draft$/.test(url)) {
      return json(route, 201, { data: { version: "9", status: "draft", sourceType: "copy_published" } });
    }
    if (/\/versions\/[^/]+\/publish$/.test(url)) {
      return json(route, 200, { data: { version: "2", status: "published" } });
    }
    if (/\/versions$/.test(url) && method === "POST") {
      return json(route, 201, { data: { version: "3", status: "draft", sourceType: "manual" } });
    }
    // GET /templates/{id}（详情）
    if (/\/cashier\/templates\/[^/?]+(\?.*)?$/.test(url)) {
      return json(route, 200, TEMPLATE_DETAIL);
    }
    // GET /templates（列表）/ POST 创建
    if (/\/cashier\/templates(\?.*)?$/.test(url)) {
      if (method === "POST") {
        return json(route, 201, { data: { templateId: "new_tpl", templateName: "New" } });
      }
      if (opts.listError) {
        return json(route, opts.listError, { error: { code: "INTERNAL", message: "boom", details: [] } });
      }
      if (opts.emptyList) {
        return json(route, 200, { data: { items: [], page: 1, pageSize: 20, total: 0 } });
      }
      return json(route, 200, TEMPLATE_LIST);
    }
    return json(route, 200, { data: {} });
  });
}

// 经侧边栏进入 /cashier（保证守卫前 /me 已注入权限）。
async function gotoCashier(page: Page) {
  await page.goto("/dashboard");
  const link = page.getByRole("link", { name: "收银台" });
  await expect(link).toBeVisible();
  await link.click();
  await expect(page.getByRole("button", { name: "新建模板" })).toBeVisible();
}

// 进入并等待详情区（含 FX 审核卡片）加载完成，便于后续断言稳定。
async function gotoCashierDetail(page: Page) {
  await gotoCashier(page);
  // 详情区含 3 个 Element Plus 表格（版本/价格矩阵/FX），本机渲染较慢，放宽等待。
  await expect(page.getByText("汇率同步审核")).toBeVisible({ timeout: 30_000 });
}

test.beforeEach(() => {
  // 本机 vite dev 冷编译较慢，抬高单测超时避免误判。
  test.setTimeout(120_000);
});

test("模板列表渲染行/FX 模式/周期，并自动选中首行加载详情", async ({ page }) => {
  await setup(page);
  await gotoCashier(page);

  await expect(page.getByRole("cell", { name: "global_default" }).first()).toBeVisible();
  await expect(page.getByRole("cell", { name: "Promo Q1" })).toBeVisible();
  // 自动选中第一行 → 详情区展示
  await expect(page.getByText("模板详情")).toBeVisible();
  await expect(page.locator(".detail-desc")).toContainText("Global Default");

  await page.screenshot({ path: "../../tests/frontend/screenshots/cashier-list-detail.png", fullPage: true });
});

test("版本列表展示状态，published 行有『复制为 draft』，draft 行有『发布』", async ({ page }) => {
  await setup(page);
  await gotoCashierDetail(page);

  await expect(page.getByText("版本列表")).toBeVisible();
  // draft v2 → 发布；published v1 → 复制为 draft（均为版本列表内唯一按钮）
  await expect(page.getByRole("button", { name: "复制为 draft" })).toBeVisible();
  await expect(page.getByRole("button", { name: "发布", exact: true })).toBeVisible();
  // 状态标签
  await expect(page.locator(".el-tag").filter({ hasText: "draft" }).first()).toBeVisible();
  await expect(page.locator(".el-tag").filter({ hasText: "published" }).first()).toBeVisible();
});

test("点击 published 行『复制为 draft』打开复制对话框", async ({ page }) => {
  await setup(page);
  await gotoCashierDetail(page);

  await page.getByRole("button", { name: "复制为 draft" }).click();
  const dialog = page.locator(".el-dialog");
  await expect(dialog.getByText("复制 published 为 draft")).toBeVisible();
  await expect(dialog.getByText(/published 版本只读/)).toBeVisible();
});

test("发布 draft 触发 publish 请求", async ({ page }) => {
  await setup(page);
  await gotoCashierDetail(page);

  const publishReq = page.waitForRequest(
    (req) => req.method() === "POST" && /\/versions\/2\/publish$/.test(req.url())
  );
  await page.getByRole("button", { name: "发布", exact: true }).click();
  // el-popconfirm 确认按钮（locale 无关：弹层内 primary 按钮）
  await page.locator(".el-popconfirm__action .el-button--primary").click();
  await publishReq;
});

test("价格矩阵：draft 版本可编辑并展示归一化预览", async ({ page }) => {
  await setup(page);
  await gotoCashierDetail(page);

  await expect(page.getByText("价格矩阵编辑器")).toBeVisible();
  // 归一化预览（minor）渲染
  await expect(page.locator(".preview__ok").first()).toContainText("preTax=999");
  await expect(page.getByRole("button", { name: "新增行" })).toBeEnabled();
  await page.screenshot({ path: "../../tests/frontend/screenshots/cashier-price-matrix.png", fullPage: true });
});

test("FX 审核：差异摘要展示，pending_review 行可 approve、已 applied 行禁用", async ({ page }) => {
  await setup(page);
  await gotoCashierDetail(page);

  await expect(page.locator(".diff").first()).toContainText("added");
  const approveBtns = page.getByRole("button", { name: "approve" });
  await expect(approveBtns.first()).toBeEnabled();
  await expect(approveBtns.nth(1)).toBeDisabled();

  await page.screenshot({ path: "../../tests/frontend/screenshots/cashier-fx-review.png", fullPage: true });
});

test("approve 触发审核请求并带 action=approve", async ({ page }) => {
  await setup(page);
  await gotoCashierDetail(page);

  const approveReq = page.waitForRequest(
    (req) => req.method() === "POST" && /\/fx-sync-runs\/501\/approve$/.test(req.url())
  );
  await page.getByRole("button", { name: "approve" }).first().click();
  const req = await approveReq;
  expect(req.postData() ?? "").toContain("approve");
});

test("无 cashier.write 权限时新建模板/触发 FX 同步置灰", async ({ page }) => {
  await setup(page, { permissions: ["cashier.read"] });
  await gotoCashierDetail(page);

  await expect(page.getByRole("button", { name: "新建模板" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "触发 FX 同步" })).toBeDisabled();
});

test("无 fx.approve 权限时 approve/ignore 置灰", async ({ page }) => {
  await setup(page, { permissions: ["cashier.read", "cashier.write"] });
  await gotoCashierDetail(page);

  await expect(page.getByRole("button", { name: "approve" }).first()).toBeDisabled();
  await expect(page.getByRole("button", { name: "ignore" }).first()).toBeDisabled();
});

test("空模板列表展示空态文案", async ({ page }) => {
  await setup(page, { emptyList: true });
  await gotoCashier(page);

  await expect(page.getByText("暂无模板")).toBeVisible();
  await page.screenshot({ path: "../../tests/frontend/screenshots/cashier-empty.png", fullPage: true });
});

test("列表加载失败展示错误提示", async ({ page }) => {
  await setup(page, { listError: 500 });
  await gotoCashier(page);

  await expect(page.getByText("加载模板列表失败").or(page.getByText("boom"))).toBeVisible();
});

test("收银台列表视觉基线", async ({ page }) => {
  await setup(page);
  await gotoCashier(page);
  await expect(page.getByRole("cell", { name: "global_default" }).first()).toBeVisible();
  await expect(page.locator(".page-shell")).toHaveScreenshot("cashier-list.png", { maxDiffPixelRatio: 0.05, timeout: 30_000 });
});
