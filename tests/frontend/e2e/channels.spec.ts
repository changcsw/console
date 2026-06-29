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
    permissions: ["channel.read", "channel.write"]
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

async function setup(page: Page, permissions: string[]) {
  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  await page.route("**/api/admin/**", (route) => json(route, 200, { data: {} }));
  await page.route("**/api/admin/me", (route) => json(route, 200, meBody(permissions)));
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
            updatedAt: "2026-01-01T00:00:00Z"
          }
        ],
        page: 1,
        pageSize: 20,
        total: 1
      }
    })
  );
  await page.route(/\/api\/admin\/games\/100001\/channels$/, (route) =>
    json(route, 200, {
      data: {
        items: [
          {
            channelId: "google",
            channelName: "Google Play",
            channelType: "store",
            region: "overseas",
            loginMode: "account_system",
            paymentMode: "hybrid",
            loginLocked: false,
            paymentLocked: false
          }
        ]
      }
    })
  );
  await page.route(/\/api\/admin\/games\/100001\/market-channels(\?.*)?$/, (route) =>
    json(route, 200, {
      data: {
        items: [
          {
            gameChannelId: 101,
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
            updatedAt: "2026-01-01T00:00:00Z"
          }
        ],
        page: 1,
        pageSize: 20,
        total: 1
      }
    })
  );
}

async function gotoChannels(page: Page) {
  await page.goto("/dashboard");
  await page.getByRole("link", { name: "渠道管理" }).click();
  await expect(page.getByText("渠道实例管理")).toBeVisible();
}

test("渠道列表行展示渠道名优先（同时保留 channelId / displayKey）", async ({ page }) => {
  await setup(page, ["channel.read", "channel.write"]);
  await gotoChannels(page);

  await expect(page.locator(".cell-channel__id").first()).toHaveText("Google Play");
  await expect(page.locator(".cell-channel__sub").first()).toHaveText("google");
  await expect(page.locator(".cell-channel__key").first()).toHaveText("100001:GLOBAL:google");
});

test("无 channel.write 权限时复制创建按钮置灰", async ({ page }) => {
  await setup(page, ["channel.read"]);
  await gotoChannels(page);
  await expect(page.getByRole("button", { name: "复制创建" })).toBeDisabled();
});
