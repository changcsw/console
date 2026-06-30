import { test, expect, type Page, type Route } from "@playwright/test";

// 模块 16-product e2e（对齐 03-testing §5.2 与 16-product compact spec）：
// 对后端契约做 mock/stub，验证「商品」Tab 列表/编辑抽屉两维独立标注、
// 「IAP」Tab 模板四件套渲染 + configStatus 行内告警 + 密文脱敏 + 权限置灰，并采集截图 + 视觉基线。
// 包级映射两列覆盖（两维正交/必填/effective）由 vitest 组件级深度覆盖；跨栈连库 e2e 属测试专家职责。

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
    permissions: ["game.read", "product.read", "product.write"]
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
      { currencyCode: "USD", currencyName: "US Dollar", decimalPlaces: 2, minAmountMinor: 1, roundingMode: "half_up", enabled: true },
      { currencyCode: "JPY", currencyName: "Japanese Yen", decimalPlaces: 0, minAmountMinor: 1, roundingMode: "half_up", enabled: true }
    ]
  }
};

const PRODUCTS = {
  data: {
    items: [
      {
        id: 1,
        env: "sandbox",
        gameId: "100001",
        productId: "com.demo.gold.10",
        productName: "金币礼包·小",
        baseAmountMinor: 499,
        baseCurrency: "USD",
        baseAmountDisplay: "4.99",
        priceId: "price_gold_small",
        enabled: true,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-02T03:04:05Z"
      }
    ],
    page: 1,
    pageSize: 20,
    total: 1
  }
};

const MARKET_CHANNELS = {
  data: {
    items: [
      {
        gameChannelId: 1,
        displayKey: "100001:GLOBAL:google",
        gameId: "100001",
        market: "GLOBAL",
        channelId: "google",
        region: "overseas",
        compatible: true,
        hidden: false,
        configStatus: "valid",
        includedInSnapshot: true,
        includedInSync: true,
        includedInRuntimeConfig: true,
        copiedFromMarket: "",
        updatedAt: "2026-01-02T03:04:05Z"
      }
    ],
    page: 1,
    pageSize: 100,
    total: 1
  }
};

const IAP_CONFIG = {
  data: {
    gameChannelId: 1,
    channelId: "google",
    template: {
      templateVersion: "v1",
      formSchema: [
        { key: "issuerId", label: "Issuer ID", component: "input", required: true, order: 1 },
        { key: "privateKey", label: "Private Key", component: "password", order: 2 },
        { key: "env", label: "环境", component: "select", order: 3, options: [{ label: "生产", value: "prod" }] }
      ],
      secretFields: ["privateKey"],
      fileFields: [],
      validationRules: { issuerId: { minLen: 1 } }
    },
    config: {
      enabled: true,
      configStatus: "invalid",
      configJson: { issuerId: "iss-123", privateKey: "masked" },
      lastCheckAt: null,
      lastCheckMessage: "缺少必填敏感字段或文件字段"
    }
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

async function setup(page: Page, permissions: string[] = SESSION.user.permissions) {
  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  // 先注册兜底，后注册具体路由（Playwright 后注册者优先）。
  await page.route("**/api/admin/**", (route) => json(route, 200, { data: {} }));
  await page.route("**/api/admin/me", (route) => json(route, 200, meBody(permissions)));
  await page.route("**/api/admin/system/currency-specs", (route) => json(route, 200, CURRENCY_SPECS));

  await page.route(/\/api\/admin\/games(\?.*)?$/, (route) => json(route, 200, GAME_LIST));

  // 商品列表（含查询串）— 在通用 games/:id 之前注册更具体的匹配
  await page.route(/\/api\/admin\/games\/100001\/products(\?.*)?$/, (route) => {
    if (route.request().method() === "POST") {
      return json(route, 201, { data: PRODUCTS.data.items[0] });
    }
    return json(route, 200, PRODUCTS);
  });
  await page.route(/\/api\/admin\/games\/100001\/market-channels(\?.*)?$/, (route) => json(route, 200, MARKET_CHANNELS));

  // 游戏详情 / markets / legal-links
  await page.route(/\/api\/admin\/games\/100001(\/(markets|legal-links))?(\?.*)?$/, (route) => {
    const url = route.request().url();
    if (url.includes("/markets")) {
      return json(route, 200, GAME_DETAIL);
    }
    if (url.includes("/legal-links")) {
      return json(route, 200, { data: { legalLinks: [] } });
    }
    return json(route, 200, GAME_DETAIL);
  });

  // IAP 配置 / 渠道包
  await page.route(/\/api\/admin\/game-channels\/1\/iap-config$/, (route) => {
    if (route.request().method() === "PUT") {
      return json(route, 200, { data: { ...IAP_CONFIG.data.config, configStatus: "valid", lastCheckMessage: "校验通过" } });
    }
    return json(route, 200, IAP_CONFIG);
  });
  await page.route(/\/api\/admin\/game-channels\/1\/packages$/, (route) => json(route, 200, { data: { items: [] } }));
}

async function gotoProductTab(page: Page) {
  await page.goto("/dashboard");
  await page.getByRole("link", { name: "游戏管理" }).click();
  await page.getByText("星际远征").click();
  await page.getByRole("tab", { name: "商品" }).click();
}

async function gotoIapTab(page: Page) {
  await page.goto("/dashboard");
  await page.getByRole("link", { name: "游戏管理" }).click();
  await page.getByText("星际远征").click();
  await page.getByRole("tab", { name: "IAP", exact: true }).click();
}

test("商品 Tab 列表渲染：productId 标注 IAP 商品 ID、priceId 标注收银台价格档", async ({ page }) => {
  await setup(page);
  await gotoProductTab(page);

  await expect(page.getByText("IAP 商品 ID (productId)")).toBeVisible();
  await expect(page.getByText("收银台价格档 (price_id)")).toBeVisible();
  await expect(page.getByRole("cell", { name: "com.demo.gold.10" })).toBeVisible();
  await expect(page.getByRole("cell", { name: "price_gold_small" })).toBeVisible();
  await expect(page.getByText("4.99 USD")).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/product-list.png", fullPage: true });
});

test("商品 Tab 编辑抽屉：两维独立 placeholder 防混填，且 productId 在编辑态锁定", async ({ page }) => {
  await setup(page);
  await gotoProductTab(page);

  await page.getByRole("button", { name: "编辑" }).click();
  await expect(page.getByRole("heading", { name: "编辑商品" })).toBeVisible();
  // productId 身份键：编辑态禁用
  const productIdInput = page.locator(".el-drawer input").first();
  await expect(productIdInput).toBeDisabled();
  // 收银台价格档独立 placeholder（防止与 IAP 商品 ID 混填）
  await expect(page.getByPlaceholder("1-64 字符（与 IAP 商品 ID 独立）")).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/product-edit-drawer.png", fullPage: true });
});

test("商品 Tab 新建抽屉：金额输入按币种给出 minor 预览", async ({ page }) => {
  await setup(page);
  await gotoProductTab(page);

  await page.getByRole("button", { name: "新建商品" }).click();
  await expect(page.getByRole("heading", { name: "新建商品" })).toBeVisible();
  const amount = page.getByPlaceholder("主单位金额字符串，例如 4.99");
  await amount.fill("4.999");
  await amount.blur();
  await expect(page.getByText(/将存储为 500 minor/)).toBeVisible();
});

test("商品 Tab 无 product.write 权限时新建按钮置灰", async ({ page }) => {
  await setup(page, ["game.read", "product.read"]);
  await gotoProductTab(page);
  await expect(page.getByText("当前账号仅有查看权限，编辑入口已置灰。")).toBeVisible();
  await expect(page.getByRole("button", { name: "新建商品" })).toBeDisabled();
});

test("IAP Tab：渲染模板四件套 + configStatus invalid 行内告警 + 密文脱敏", async ({ page }) => {
  await setup(page);
  await gotoIapTab(page);

  const iap = page.locator(".iap-tab");
  await expect(page.getByRole("heading", { name: "渠道 IAP 配置" })).toBeVisible();
  // 模板四件套字段
  await expect(iap.getByText("Issuer ID")).toBeVisible();
  await expect(iap.getByText("Private Key")).toBeVisible();
  await expect(iap.getByText("环境", { exact: true })).toBeVisible();
  // config_status invalid → 行内告警 + lastCheckMessage 不隐藏
  await expect(iap.getByText("缺少必填敏感字段或文件字段")).toBeVisible();
  await expect(iap.locator("p.status-text--warning")).toBeVisible();
  // 密文恒脱敏，绝不出现明文
  await expect(iap.locator(".secret-input__masked").first()).toBeVisible();
  await expect(page.locator("body")).not.toContainText("PLAINTEXT");

  await page.screenshot({ path: "../../tests/frontend/screenshots/product-iap-config.png", fullPage: true });
});

test("IAP Tab 无 product.write 权限时保存按钮置灰且展示只读提示", async ({ page }) => {
  await setup(page, ["game.read", "product.read"]);
  await gotoIapTab(page);
  await expect(page.locator(".iap-tab").getByText("当前账号仅有查看权限，配置项已置灰。")).toBeVisible();
  await expect(page.getByRole("button", { name: "保存渠道 IAP 配置" })).toBeDisabled();
});

test("商品列表视觉基线", async ({ page }) => {
  await setup(page);
  await gotoProductTab(page);
  await expect(page.getByText("com.demo.gold.10")).toBeVisible();
  await expect(page).toHaveScreenshot("product-list.png", { maxDiffPixelRatio: 0.02 });
});
