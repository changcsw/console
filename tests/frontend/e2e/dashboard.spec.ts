import { expect, test, type Page, type Route } from "@playwright/test";
import {
  DASHBOARD_EMPTY_SUMMARY,
  DASHBOARD_SAMPLE_SUMMARY,
  cloneSummary
} from "../../../apps/admin-web/src/views/dashboard/__tests__/fixtures";

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
    permissions: ["dashboard.read"]
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
  mode?: "normal" | "empty" | "trimmed";
  firstSummaryError?: boolean;
}

async function setup(page: Page, opts: SetupOptions = {}) {
  const requests: string[] = [];
  const permissions = opts.permissions ?? ["dashboard.read"];
  let summaryCalls = 0;

  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  await page.route("**/api/admin/**", (route) => {
    const url = route.request().url();
    if (url.endsWith("/api/admin/me")) {
      return json(route, 200, meBody(permissions));
    }
    if (/\/api\/admin\/dashboard\/summary(\?.*)?$/.test(url)) {
      summaryCalls += 1;
      requests.push(url);

      if (opts.firstSummaryError && summaryCalls === 1) {
        return json(route, 500, { error: { code: "INTERNAL", message: "服务异常", details: [] } });
      }

      if (opts.mode === "empty") {
        return json(route, 200, { data: cloneSummary(DASHBOARD_EMPTY_SUMMARY) });
      }

      if (opts.mode === "trimmed") {
        const trimmed = cloneSummary();
        trimmed.fxReview.permitted = false;
        return json(route, 200, { data: trimmed });
      }

      const query = new URL(url).searchParams;
      const summary = cloneSummary(DASHBOARD_SAMPLE_SUMMARY);
      if (query.get("withTopItems") !== "true") {
        summary.fxReview.topItems = [];
        summary.configIssues.topItems = [];
        summary.recentSyncJobs.topItems = [];
        summary.pendingSnapshots.topItems = [];
        summary.channelInstanceIssues.topItems = [];
      }
      if (query.get("range") === "30d") {
        summary.recentSyncJobs.total = 9;
        summary.recentSyncJobs.byStatus = { previewed: 3, succeeded: 4, failed: 2 };
      }
      return json(route, 200, { data: summary });
    }
    return json(route, 200, { data: {} });
  });

  return { requests };
}

test("dashboard 渲染 5 卡布局、EnvironmentBadge 与截图基线", async ({ page }) => {
  await setup(page);
  await page.goto("/dashboard");

  await expect(page.getByText("汇率待审", { exact: true })).toBeVisible();
  await expect(page.locator(".metric-card")).toHaveCount(5);
  // 顶栏（AdminLayout）与 dashboard 工具栏各有一个 EnvironmentBadge，限定到 dashboard 工具栏避免 strict 冲突。
  await expect(page.locator(".dashboard-toolbar .env-badge")).toContainText("PRODUCTION");

  await page.screenshot({ path: "../../tests/frontend/screenshots/dashboard-main.png", fullPage: true });
  await expect(page).toHaveScreenshot("dashboard-main.png", { maxDiffPixelRatio: 0.02 });
});

test("range 交互与展开明细会按契约参数重拉", async ({ page }) => {
  const { requests } = await setup(page);
  await page.goto("/dashboard");
  await expect(page.getByText("最近同步", { exact: true })).toBeVisible();

  await page.getByText("30d", { exact: true }).click();
  await expect(page.getByText("成功 4")).toBeVisible();
  await expect(page.getByText("失败 2")).toBeVisible();
  await expect(page.getByText("预览 3")).toBeVisible();

  await page.getByRole("button", { name: "展开明细" }).first().click();
  await expect(page.getByText("Global Cashier Price v3")).toBeVisible();

  expect(requests.some((url) => url.includes("range=7d"))).toBe(true);
  expect(requests.some((url) => url.includes("range=30d"))).toBe(true);
  expect(requests.some((url) => url.includes("withTopItems=true"))).toBe(true);
});

test("权限态：permitted=false 卡片隐藏", async ({ page }) => {
  await setup(page, { mode: "trimmed" });
  await page.goto("/dashboard");
  await expect(page.getByText("汇率待审", { exact: true })).toBeHidden();
  await expect(page.locator(".metric-card")).toHaveCount(4);
  await page.screenshot({ path: "../../tests/frontend/screenshots/dashboard-trimmed.png", fullPage: true });
});

test("空态：5 卡显示『暂无待办』", async ({ page }) => {
  await setup(page, { mode: "empty" });
  await page.goto("/dashboard");
  await expect(page.getByText("暂无待办").first()).toBeVisible();
  await page.screenshot({ path: "../../tests/frontend/screenshots/dashboard-empty.png", fullPage: true });
});

test("错误条与重试可恢复页面", async ({ page }) => {
  await setup(page, { firstSummaryError: true });
  await page.goto("/dashboard");

  await expect(page.getByText("Dashboard 加载失败：服务异常")).toBeVisible();
  await page.getByRole("button", { name: "重试" }).click();
  await expect(page.getByText("Dashboard 加载失败：服务异常")).toBeHidden();
  await expect(page.getByText("汇率待审", { exact: true })).toBeVisible();
  await page.screenshot({ path: "../../tests/frontend/screenshots/dashboard-error-retry.png", fullPage: true });
});
