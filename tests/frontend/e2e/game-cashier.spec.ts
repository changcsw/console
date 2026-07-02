import { expect, test, type Page, type Route } from "@playwright/test";

// 游戏级收银台 e2e（对齐 03-testing §5.2 与 18-game-cashier compact spec）：
// 对契约做 mock/stub，验证游戏详情「收银台」Tab 的：绑定快照展示、模板矩阵 vs 游戏覆盖
// 边界高亮、currency_specs 舍入预览、切换/升级版本 PUT、保存覆盖 PUT、未绑定空态、
// 无 cashier.write 置灰，并采集截图 + 视觉基线。
// 真实跨栈联调（连库 e2e）属测试专家职责，本用例以前端 + 契约 mock 为主。
//
// 注：本机 vite dev 冷编译 + Element Plus 首屏渲染较慢，统一抬高单测超时避免误判。

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
    permissions: ["dashboard.read", "game.read", "game.write", "cashier.read", "cashier.write"]
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

const GAME_LIST = {
  data: {
    items: [
      {
        gameId: "100001",
        name: "星际远征",
        alias: "starfront",
        iconUrl: "",
        status: "active",
        defaultMarketCode: "GLOBAL",
        marketCodes: ["GLOBAL"],
        marketCount: 1,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-02T03:04:05Z"
      }
    ],
    page: 1,
    pageSize: 20,
    total: 1
  }
};

const GAME_DETAIL = {
  data: {
    gameId: "100001",
    name: "星际远征",
    alias: "starfront",
    iconUrl: "",
    status: "active",
    defaultMarketCode: "GLOBAL",
    gameSecret: "masked",
    secretMasked: true,
    environment: "sandbox",
    markets: [{ marketCode: "GLOBAL", isDefault: true, enabled: true, defaultLocale: "en-US" }],
    legalLinks: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T03:04:05Z"
  }
};

const CURRENCY_SPECS = {
  data: {
    items: [
      { currencyCode: "USD", currencyName: "US Dollar", decimalPlaces: 2, minAmountMinor: 50, roundingMode: "half_up", enabled: true },
      { currencyCode: "JPY", currencyName: "Japanese Yen", decimalPlaces: 0, minAmountMinor: 1, roundingMode: "half_up", enabled: true }
    ]
  }
};

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
      }
    ],
    page: 1,
    pageSize: 100,
    total: 1
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
    fxSyncRuns: []
  }
};

// 模板公共矩阵行：US 行与覆盖同键（高亮），JP 行为模板独有（不高亮）。
const TEMPLATE_ROWS = {
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
      },
      {
        countryCode: "JP",
        regionCode: "*",
        currency: "JPY",
        priceId: "com.game.pack.001",
        preTaxAmountMinor: 120,
        taxRate: 0,
        taxAmountMinor: 0,
        afterTaxAmountMinor: 120,
        effectiveAt: "2026-01-01T00:00:00Z"
      }
    ]
  }
};

const PROFILE = {
  data: {
    templateId: "global_default",
    appliedTemplateVersion: "1",
    snapshotChecksum: "chk-e2e-001",
    appliedAt: "2026-01-01T00:00:00Z"
  }
};

const PRICE_OVERRIDES = {
  data: {
    items: [
      {
        countryCode: "US",
        regionCode: "*",
        currency: "USD",
        priceId: "com.game.pack.001",
        preTaxAmountMinor: 1999,
        taxRate: "0.1",
        taxAmountMinor: 200,
        afterTaxAmountMinor: 2199,
        reason: "节日促销",
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
  unbound?: boolean;
}

async function setup(page: Page, opts: SetupOptions = {}) {
  const permissions = opts.permissions ?? ["game.read", "game.write", "cashier.read", "cashier.write"];

  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  // 兜底：其它后台接口返回空，避免页面挂起（先注册兜底，后注册具体路由优先）。
  await page.route("**/api/admin/**", (route) => json(route, 200, { data: {} }));
  await page.route("**/api/admin/me", (route) => json(route, 200, meBody(permissions)));
  await page.route("**/api/admin/system/currency-specs", (route) => json(route, 200, CURRENCY_SPECS));

  // 游戏列表 / 详情
  await page.route(/\/api\/admin\/games(\?.*)?$/, (route) => json(route, 200, GAME_LIST));
  await page.route(/\/api\/admin\/games\/[^/]+(\?.*)?$/, (route) => json(route, 200, GAME_DETAIL));

  // 游戏级收银台：profile / price-overrides
  await page.route(/\/api\/admin\/games\/[^/]+\/cashier\/profile$/, (route) => {
    if (route.request().method() === "PUT") {
      return json(route, 200, PROFILE);
    }
    if (opts.unbound) {
      return json(route, 404, { error: { code: "NOT_FOUND", message: "profile not bound", details: [] } });
    }
    return json(route, 200, PROFILE);
  });
  await page.route(/\/api\/admin\/games\/[^/]+\/cashier\/price-overrides$/, (route) => {
    if (route.request().method() === "PUT") {
      return json(route, 200, PRICE_OVERRIDES);
    }
    return json(route, 200, opts.unbound ? { data: { items: [] } } : PRICE_OVERRIDES);
  });

  // cashier 模板契约：列表 / 详情 / 版本价格行
  await page.route("**/api/admin/cashier/**", (route) => {
    const url = route.request().url();
    if (/\/versions\/[^/]+\/rows$/.test(url)) {
      return json(route, 200, TEMPLATE_ROWS);
    }
    if (/\/cashier\/templates\/[^/?]+(\?.*)?$/.test(url)) {
      return json(route, 200, TEMPLATE_DETAIL);
    }
    if (/\/cashier\/templates(\?.*)?$/.test(url)) {
      return json(route, 200, TEMPLATE_LIST);
    }
    return json(route, 200, { data: {} });
  });
}

// 经侧边栏进入游戏详情「收银台」Tab（保证守卫前 /me 已注入权限）。
async function gotoCashierTab(page: Page) {
  await page.goto("/dashboard");
  const link = page.getByRole("link", { name: "游戏管理" });
  await expect(link).toBeVisible();
  await link.click();
  await expect(page.getByRole("cell", { name: "100001" })).toBeVisible({ timeout: 30_000 });
  await page.getByText("星际远征").click();
  // 等待详情加载完成（标题出现）后再切 Tab，规避本机冷编译导致的渲染延迟。
  await expect(page.locator(".detail-head__title")).toContainText("星际远征", { timeout: 45_000 });
  const tab = page.getByRole("tab", { name: "收银台" });
  await expect(tab).toBeVisible({ timeout: 45_000 });
  await tab.click();
  await expect(page.getByText("模板绑定快照")).toBeVisible({ timeout: 30_000 });
}

test.beforeEach(() => {
  test.setTimeout(120_000);
});

test("已绑定模板：展示绑定快照（模板/校验和/已绑定标签）", async ({ page }) => {
  await setup(page);
  await gotoCashierTab(page);

  await expect(page.getByText("已绑定模板")).toBeVisible();
  await expect(page.locator("body")).toContainText("chk-e2e-001");
  await expect(page.getByText("价格边界视图")).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/game-cashier-profile.png", fullPage: true });
});

test("价格边界视图：覆盖行标记『游戏覆盖』、模板独有行『模板公共矩阵』", async ({ page }) => {
  await setup(page);
  await gotoCashierTab(page);

  await expect(page.locator(".el-tag").filter({ hasText: "游戏覆盖" }).first()).toBeVisible();
  await expect(page.locator(".el-tag").filter({ hasText: "模板公共矩阵" }).first()).toBeVisible();
  // 覆盖行高亮（row-class-name）
  await expect(page.locator("tr.matrix-row--overridden").first()).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/game-cashier-matrix.png", fullPage: true });
});

test("游戏级覆盖：currency_specs 舍入预览展示 minor", async ({ page }) => {
  await setup(page);
  await gotoCashierTab(page);

  // 已绑定覆盖（19.99 USD）→ preTax=1999
  await expect(page.locator(".preview__ok").first()).toContainText("preTax=1999");

  await page.screenshot({ path: "../../tests/frontend/screenshots/game-cashier-overrides.png", fullPage: true });
});

test("切换/升级版本触发 PUT profile", async ({ page }) => {
  await setup(page);
  await gotoCashierTab(page);

  const putReq = page.waitForRequest(
    (req) => req.method() === "PUT" && /\/games\/100001\/cashier\/profile$/.test(req.url())
  );
  await page.getByRole("button", { name: "切换/升级版本" }).click();
  await putReq;
});

test("保存覆盖触发 PUT price-overrides", async ({ page }) => {
  await setup(page);
  await gotoCashierTab(page);

  const putReq = page.waitForRequest(
    (req) => req.method() === "PUT" && /\/games\/100001\/cashier\/price-overrides$/.test(req.url())
  );
  await page.getByRole("button", { name: "保存覆盖" }).click();
  const req = await putReq;
  expect(req.postData() ?? "").toContain("items");
});

test("未绑定（profile 404→null）展示空态", async ({ page }) => {
  await setup(page, { unbound: true });
  await gotoCashierTab(page);

  await expect(page.getByText("尚未绑定收银台模板")).toBeVisible();
  await expect(page.getByText("未绑定模板")).toBeVisible();
  await expect(page.getByText("暂无价格矩阵数据")).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/game-cashier-empty.png", fullPage: true });
});

test("无 cashier.write 权限时绑定/覆盖写按钮置灰", async ({ page }) => {
  await setup(page, { permissions: ["game.read", "cashier.read"] });
  await gotoCashierTab(page);

  // 同游戏详情其它 Tab 也有只读 alert，用本 Tab 专属文案精确匹配。
  await expect(page.getByText("当前账号仅有查看权限，绑定版本与覆盖编辑入口已置灰。")).toBeVisible();
  await expect(page.getByRole("button", { name: "切换/升级版本" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "新增覆盖行" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "保存覆盖" })).toBeDisabled();
});

test("游戏级收银台 Tab 视觉基线", async ({ page }) => {
  await setup(page);
  await gotoCashierTab(page);
  await expect(page.getByText("模板绑定快照")).toBeVisible();
  await expect(page.locator(".page-shell")).toHaveScreenshot("game-cashier-tab.png", {
    maxDiffPixelRatio: 0.05,
    timeout: 30_000
  });
});
