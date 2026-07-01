import { expect, test, type Page, type Route } from "@playwright/test";

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
    permissions: ["game.read", "payment.read", "payment.write", "product.read", "product.write"],
  },
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
      environment: "sandbox",
    },
  };
}

function json(route: Route, status: number, body: unknown) {
  return route.fulfill({
    status,
    contentType: "application/json",
    headers: { "X-Environment": "sandbox" },
    body: JSON.stringify(body),
  });
}

async function setup(page: Page, permissions: string[] = SESSION.user.permissions) {
  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  await page.route("**/api/admin/**", (route) => json(route, 200, { data: {} }));
  await page.route("**/api/admin/me", (route) => json(route, 200, meBody(permissions)));
  await page.route("**/api/admin/system/currency-specs", (route) =>
    json(route, 200, {
      data: {
        items: [
          { currencyCode: "USD", currencyName: "US Dollar", decimalPlaces: 2, minAmountMinor: 1, roundingMode: "half_up", enabled: true },
          { currencyCode: "JPY", currencyName: "Japanese Yen", decimalPlaces: 0, minAmountMinor: 1, roundingMode: "half_up", enabled: true },
        ],
      },
    })
  );

  await page.route(/\/api\/admin\/pay-ways(\?.*)?$/, (route) =>
    json(route, 200, {
      data: {
        items: [
          { payWayId: "card", payWayName: "信用卡", payWayType: "card", enabled: true, sort: 10 },
          { payWayId: "wallet", payWayName: "电子钱包", payWayType: "wallet", enabled: true, sort: 20 },
        ],
        page: 1,
        pageSize: 20,
        total: 2,
      },
    })
  );
  await page.route(/\/api\/admin\/cashier\/providers(\?.*)?$/, (route) =>
    json(route, 200, {
      data: {
        items: [
          { providerId: "airwallex", providerName: "Airwallex", providerKind: "gateway", enabled: true, sort: 10 },
          { providerId: "stripe", providerName: "Stripe", providerKind: "gateway", enabled: true, sort: 20 },
        ],
        page: 1,
        pageSize: 20,
        total: 2,
      },
    })
  );
  await page.route(/\/api\/admin\/billing-subjects(\?.*)?$/, (route) =>
    json(route, 200, {
      data: {
        items: [{ subjectId: "sg_main", subjectName: "新加坡主体", legalEntityName: "SG PTE LTD", enabled: true }],
        page: 1,
        pageSize: 20,
        total: 1,
      },
    })
  );
  await page.route(/\/api\/admin\/cashier\/merchant-accounts(\?.*)?$/, (route) =>
    json(route, 200, {
      data: {
        items: [
          {
            merchantAccountId: "ma_aw_01",
            providerId: "airwallex",
            subjectId: "sg_main",
            merchantId: "m001",
            merchantName: "Main",
            configJson: {},
            secret: "masked",
            enabled: true,
          },
        ],
        page: 1,
        pageSize: 20,
        total: 1,
      },
    })
  );

  await page.route(/\/api\/admin\/games(\?.*)?$/, (route) =>
    json(route, 200, {
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
            updatedAt: "2026-01-02T03:04:05Z",
          },
        ],
        page: 1,
        pageSize: 20,
        total: 1,
      },
    })
  );

  await page.route(/\/api\/admin\/games\/100001\/products(\?.*)?$/, (route) =>
    json(route, 200, {
      data: {
        items: [
          {
            id: 1,
            env: "sandbox",
            gameId: "100001",
            productId: "com.demo.gold.10",
            productName: "金币礼包",
            baseAmountMinor: 499,
            baseCurrency: "USD",
            baseAmountDisplay: "4.99",
            priceId: "price_gold_1",
            enabled: true,
            createdAt: "2026-01-01T00:00:00Z",
            updatedAt: "2026-01-02T03:04:05Z",
          },
        ],
        page: 1,
        pageSize: 20,
        total: 1,
      },
    })
  );
  await page.route(/\/api\/admin\/games\/100001\/market-channels(\?.*)?$/, (route) =>
    json(route, 200, {
      data: {
        items: [
          {
            gameChannelId: 1,
            gameId: "100001",
            market: "GLOBAL",
            channelId: "google",
            region: "overseas",
            hidden: false,
            compatible: true,
            configStatus: "valid",
            includedInSnapshot: true,
            includedInSync: true,
            includedInRuntimeConfig: true,
            copiedFromMarket: "",
            updatedAt: "2026-01-02T03:04:05Z",
          },
        ],
        page: 1,
        pageSize: 100,
        total: 1,
      },
    })
  );
  await page.route("**/api/admin/game-channels/1/packages", (route) => json(route, 200, { data: { items: [] } }));
  await page.route("**/api/admin/game-channels/1/iap-config", (route) =>
    json(route, 200, {
      data: {
        gameChannelId: 1,
        channelId: "google",
        template: {
          templateVersion: "v1",
          formSchema: [{ key: "issuerId", label: "Issuer ID", component: "input", required: true, order: 1 }],
          secretFields: [],
          fileFields: [],
          validationRules: {},
        },
        config: {
          enabled: true,
          configStatus: "valid",
          configJson: { issuerId: "iss_001" },
          lastCheckAt: "2026-01-02T03:04:05Z",
          lastCheckMessage: "校验通过",
        },
      },
    })
  );
  await page.route(/\/api\/admin\/games\/100001(\/(markets|legal-links|payment-routes))?(\?.*)?$/, (route) => {
    const url = route.request().url();
    if (url.includes("/payment-routes")) {
      return json(route, 200, {
        data: {
          gameId: "100001",
          env: "sandbox",
          groups: [
            {
              payWayId: "card",
              payWayName: "信用卡",
              payWayType: "card",
              routes: [
                {
                  id: 11,
                  selector: { packageCode: "*", channelId: "*", marketCode: "GLOBAL", countryCode: "*", currency: "*" },
                  providerId: "airwallex",
                  merchantAccountId: "ma_aw_01",
                  priority: 100,
                  enabled: true,
                },
              ],
            },
            {
              payWayId: "wallet",
              payWayName: "电子钱包",
              payWayType: "wallet",
              routes: [
                {
                  id: 12,
                  selector: { packageCode: "pkg_jp", channelId: "google", marketCode: "JP", countryCode: "JP", currency: "JPY" },
                  providerId: "stripe",
                  merchantAccountId: "ma_aw_01",
                  priority: 10,
                  enabled: true,
                },
              ],
            },
          ],
        },
      });
    }
    if (url.includes("/markets")) {
      return json(route, 200, {
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
          updatedAt: "2026-01-02T03:04:05Z",
        },
      });
    }
    if (url.includes("/legal-links")) {
      return json(route, 200, { data: { legalLinks: [] } });
    }
    return json(route, 200, {
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
        updatedAt: "2026-01-02T03:04:05Z",
      },
    });
  });
}

async function gotoPaymentList(page: Page) {
  await page.goto("/dashboard");
  const paymentLink = page.locator('a[href="/payment"]').first();
  await expect(paymentLink).toBeVisible();
  await paymentLink.click();
  await expect(page.getByText("支付配置中心")).toBeVisible();
}

async function gotoPaymentRoutesTab(page: Page) {
  await page.goto("/dashboard");
  await page.getByRole("link", { name: "游戏管理" }).click();
  await page.getByText("星际远征").click();
  await page.getByRole("tab", { name: "支付路由" }).click();
  await expect(page.locator(".payment-routes-tab")).toBeVisible();
}

test("payment 列表页冒烟：支付方式页面可加载", async ({ page }) => {
  await setup(page);
  await gotoPaymentList(page);
  await expect(page.getByRole("tab", { name: "支付方式" })).toBeVisible();
  await expect(page.getByText("card", { exact: true }).first()).toBeVisible();
  await expect(page.getByText("wallet", { exact: true }).first()).toBeVisible();
  await page.screenshot({ path: "../../tests/frontend/screenshots/payment-payways.png", fullPage: true });
});

test("游戏详情支付路由 Tab 冒烟：分组、顺序、兜底标签可见", async ({ page }) => {
  await setup(page);
  await gotoPaymentRoutesTab(page);
  await expect(page.getByText("信用卡 (card)")).toBeVisible();
  await expect(page.getByText("电子钱包 (wallet)")).toBeVisible();
  await expect(page.getByText("airwallex / ma_aw_01")).toBeVisible();
  await expect(page.getByText("兜底")).toBeVisible();
  await expect(page).toHaveScreenshot("payment-routes-tab.png", { maxDiffPixelRatio: 0.02 });
});

test("支付路由与 Product/IAP Tab 共存无回归", async ({ page }) => {
  await setup(page);
  await gotoPaymentRoutesTab(page);
  await expect(page.getByRole("tab", { name: "商品" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "IAP", exact: true })).toBeVisible();

  await page.getByRole("tab", { name: "商品" }).click();
  await expect(page.getByText("IAP 商品 ID (productId)")).toBeVisible();
  await page.getByRole("tab", { name: "IAP", exact: true }).click();
  await expect(page.getByRole("heading", { name: "渠道 IAP 配置" })).toBeVisible();
  await page.getByRole("tab", { name: "支付路由" }).click();
  await expect(page.locator(".payment-routes-tab")).toBeVisible();
});
