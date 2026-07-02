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
    permissions: ["dashboard.read", "game.read", "snapshot.generate", "snapshot.publish"]
  }
};

const GAME = {
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
};

// 后端契约保证 generated_at 降序返回。
const SNAPSHOTS = [
  {
    id: 3,
    configVersion: "20260615120000-cccccccc",
    status: "published",
    fileHash: "cccccccc11112222",
    generatedAt: "2026-06-15T12:00:00Z",
    publishedAt: "2026-06-15T12:30:00Z"
  },
  {
    id: 2,
    configVersion: "20260615110000-bbbbbbbb",
    status: "draft",
    fileHash: "bbbbbbbb33334444",
    generatedAt: "2026-06-15T11:00:00Z",
    publishedAt: null
  },
  {
    id: 1,
    configVersion: "20260615100000-aaaaaaaa",
    status: "draft",
    fileHash: "aaaaaaaa55556666",
    generatedAt: "2026-06-15T10:00:00Z",
    publishedAt: null
  }
];

// 下载/预览返回的 config_json（含 secret 字段以验证前端脱敏）。
const CONFIG_JSON = {
  schemaVersion: "1.0",
  gameId: "100001",
  generatedAt: "2026-06-15T10:00:00Z",
  markets: {
    GLOBAL: {
      game: { legalLinks: [], accountAuth: [], products: [] },
      channels: [
        {
          channelId: "google",
          region: "overseas",
          sourceMarket: "GLOBAL",
          login: { appId: "app-global", appSecret: "PLAINTEXT_GLOBAL_SECRET" },
          iap: {},
          packages: []
        }
      ],
      paymentRoutes: []
    },
    JP: {
      game: {},
      channels: [
        {
          channelId: "google",
          region: "overseas",
          sourceMarket: "JP",
          iap: { issuerId: "iss-jp", privateKey: "PLAINTEXT_JP_KEY" }
        }
      ],
      paymentRoutes: []
    }
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
  snapshots?: typeof SNAPSHOTS;
  listStatus?: number;
  listError?: { code: string; message: string };
}

async function setup(page: Page, options: SetupOptions = {}) {
  const permissions = options.permissions ?? SESSION.user.permissions;
  const snapshots = options.snapshots ?? SNAPSHOTS;

  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  // 通用兜底，需先注册（后注册的更具体路由优先命中）。
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
            updatedAt: "2026-01-02T03:04:05Z"
          }
        ],
        page: 1,
        pageSize: 20,
        total: 1
      }
    })
  );

  // 快照列表（接口 1）
  await page.route(/\/api\/admin\/games\/100001\/config-snapshots(\?.*)?$/, (route) => {
    if (route.request().method() === "GET") {
      if (options.listStatus && options.listStatus >= 400) {
        return json(route, options.listStatus, {
          error: { code: options.listError?.code ?? "INTERNAL", message: options.listError?.message ?? "boom", details: [] }
        });
      }
      return json(route, 200, { data: { items: snapshots, page: 1, pageSize: 20, total: snapshots.length } });
    }
    return json(route, 200, { data: {} });
  });

  // 生成快照（接口 2）
  await page.route(/\/api\/admin\/games\/100001\/config-snapshots\/generate$/, (route) =>
    json(route, 201, {
      data: {
        id: 9,
        configVersion: "20260615130000-deadbeef",
        fileHash: "deadbeef77778888",
        status: "draft",
        generatedAt: "2026-06-15T13:00:00Z"
      }
    })
  );

  // 发布（接口 3）
  await page.route(/\/api\/admin\/game-config-snapshots\/\d+\/publish$/, (route) =>
    json(route, 200, {
      data: {
        id: 2,
        configVersion: "20260615110000-bbbbbbbb",
        status: "published",
        fileHash: "bbbbbbbb33334444",
        generatedAt: "2026-06-15T11:00:00Z",
        publishedAt: "2026-06-15T13:05:00Z"
      }
    })
  );

  // 下载（接口 4）— 返回原始 config_json + Content-Disposition
  await page.route(/\/api\/admin\/game-config-snapshots\/\d+\/download$/, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      headers: {
        "X-Environment": "sandbox",
        "Content-Disposition": 'attachment; filename="game_100001_20260615100000-aaaaaaaa.json"'
      },
      body: JSON.stringify(CONFIG_JSON)
    })
  );

  // 游戏详情（getGame）以及其它 Tab 兜底
  await page.route(/\/api\/admin\/games\/100001(\/(markets|legal-links))?(\?.*)?$/, (route) => {
    const url = route.request().url();
    if (url.includes("/legal-links")) {
      return json(route, 200, { data: { legalLinks: [] } });
    }
    return json(route, 200, { data: GAME });
  });
}

async function gotoSnapshotTab(page: Page) {
  await page.goto("/dashboard");
  await page.getByRole("link", { name: "游戏管理" }).click();
  await page.getByText("星际远征").click();
  await page.getByRole("tab", { name: "配置快照" }).click();
  await expect(page.locator(".snapshot-tab")).toBeVisible();
}

test("快照列表渲染：version/status/hash/时间按降序展示（截图基线）", async ({ page }) => {
  await setup(page);
  await gotoSnapshotTab(page);

  await expect(page.getByText("20260615120000-cccccccc")).toBeVisible();
  await expect(page.getByText("cccccccc11112222")).toBeVisible();
  await expect(page.locator(".el-tag").filter({ hasText: "published" }).first()).toBeVisible();
  await expect(page.locator(".el-tag").filter({ hasText: "draft" }).first()).toBeVisible();

  // 降序：最新版本在最前
  const versions = await page.locator(".el-table__body-wrapper td .cell").allInnerTexts();
  const idxNewest = versions.findIndex((t) => t.includes("cccccccc"));
  const idxOldest = versions.findIndex((t) => t.includes("aaaaaaaa"));
  expect(idxNewest).toBeGreaterThanOrEqual(0);
  expect(idxNewest).toBeLessThan(idxOldest);

  await expect(page).toHaveScreenshot("snapshot-tab.png", { maxDiffPixelRatio: 0.02 });
});

test("生成快照：点击生成按钮触发生成并提示成功", async ({ page }) => {
  await setup(page);
  await gotoSnapshotTab(page);

  await page.getByRole("button", { name: "生成快照" }).click();
  await expect(page.getByText(/快照生成成功/)).toBeVisible();
});

test("JSON 预览：按 market 分区折叠展示且密文脱敏为 ***", async ({ page }) => {
  await setup(page);
  await gotoSnapshotTab(page);

  await page.getByRole("button", { name: "预览 JSON" }).first().click();
  await expect(page.getByRole("heading", { name: "JSON 预览" })).toBeVisible();
  await expect(page.locator(".el-collapse-item__header").filter({ hasText: "GLOBAL" })).toBeVisible();
  await expect(page.locator(".el-collapse-item__header").filter({ hasText: "JP" })).toBeVisible();

  const previewText = await page.locator(".snapshot-tab__json").first().innerText();
  expect(previewText).toContain("***");
  expect(previewText).not.toContain("PLAINTEXT_GLOBAL_SECRET");

  await page.screenshot({ path: "../../tests/frontend/screenshots/snapshot-json-preview.png", fullPage: true });
});

test("发布二次确认：draft → published", async ({ page }) => {
  await setup(page);
  await gotoSnapshotTab(page);

  // 第一行 draft 的发布按钮（id=2 行）
  await page
    .locator(".el-table__row")
    .filter({ hasText: "20260615110000-bbbbbbbb" })
    .getByRole("button", { name: "发布" })
    .click();

  await expect(page.getByText("发布后该快照将进入 published 状态，确认继续？")).toBeVisible();
  await page.getByRole("button", { name: "确认发布" }).click();
  await expect(page.getByText("快照发布成功")).toBeVisible();
});

test("权限置灰：仅 game.read 时生成/发布入口置灰并提示只读", async ({ page }) => {
  await setup(page, { permissions: ["game.read"] });
  await gotoSnapshotTab(page);

  const pane = page.locator("#pane-snapshot");
  // 快照 tab 专属只读文案，避免命中其它 tab 面板的通用只读提示
  await expect(pane.getByText("生成/发布入口已置灰")).toBeVisible();
  const genBtn = pane.getByRole("button", { name: "生成快照" });
  await expect(genBtn).toBeDisabled();
  await expect(genBtn).toHaveClass(/perm-disabled/);

  await expect(page).toHaveScreenshot("snapshot-tab-readonly.png", { maxDiffPixelRatio: 0.02 });
});

test("空态：无快照时展示空提示与生成首个快照入口", async ({ page }) => {
  await setup(page, { snapshots: [] });
  await gotoSnapshotTab(page);

  await expect(page.getByText("暂无配置快照")).toBeVisible();
  await expect(page.getByRole("button", { name: "生成首个快照" })).toBeVisible();
});

test("错误态：列表加载失败展示错误结果并可重试", async ({ page }) => {
  await setup(page, { listStatus: 500, listError: { code: "INTERNAL", message: "boom" } });
  await gotoSnapshotTab(page);

  await expect(page.getByText("配置快照加载失败")).toBeVisible();
  await expect(page.getByRole("button", { name: "重试" })).toBeVisible();
});
