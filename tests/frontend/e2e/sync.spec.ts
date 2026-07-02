import { expect, test, type Page, type Route } from "@playwright/test";

// sync #21 · Sandbox→Production 同步 e2e（对齐 03-testing §5.2 与 21-sync compact spec）。
// 对后端契约做 mock/stub，验证：仅 sandbox 渲染 Sync 入口（红线）、production 不渲染、
// 权限 sync.execute 置灰、预览分组/徽标/配色、密文脱敏、include_deletes 默认关、
// 执行成功刷新历史、SYNC_BASELINE_MISMATCH 重新预览、同步历史列表/过滤/失败行展开，
// 并采集截图 + 视觉基线。真实跨栈联调（连库 e2e）属测试专家职责，本用例以前端 + 契约 mock 为主。

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
    permissions: ["dashboard.read", "game.read", "sync.preview", "sync.execute"]
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

const PREVIEW = {
  gameId: "100001",
  sourceEnv: "sandbox",
  targetEnv: "production",
  sourceHash: "sha256-source-aaaaaaaabbbbbbbb",
  targetHashBefore: "sha256-target-ccccccccdddddddd",
  hasDiff: true,
  baselineToken: "baseline-token-e2e.sig",
  previewedAt: "2026-06-17T13:00:00Z",
  expiresAt: "2026-06-17T13:30:00Z",
  sections: [
    {
      section: "channels",
      summary: { add: 1, update: 1, delete: 1 },
      dependencies: ["game", "markets"],
      changes: [
        {
          op: "add",
          entityType: "game_channel",
          entityKey: "JP/google",
          fieldName: "*",
          sandboxValue: { market: "JP", channelId: "google", enabled: true },
          productionValue: null,
          masked: false
        },
        {
          op: "update",
          entityType: "game_channel_login_config",
          entityKey: "JP/google",
          fieldName: "clientSecret",
          sandboxValue: "PLAINTEXT_SANDBOX_SECRET",
          productionValue: "PLAINTEXT_PROD_SECRET",
          masked: true
        },
        {
          op: "delete",
          entityType: "game_channel",
          entityKey: "KR/apple",
          fieldName: "*",
          sandboxValue: null,
          productionValue: { market: "KR", channelId: "apple" },
          masked: false
        }
      ]
    },
    {
      section: "products",
      summary: { add: 0, update: 1, delete: 0 },
      dependencies: ["game"],
      changes: [
        {
          op: "update",
          entityType: "product",
          entityKey: "gem_100",
          fieldName: "price",
          sandboxValue: "9.99",
          productionValue: "8.99",
          masked: false
        }
      ]
    }
  ]
};

const EXECUTE_OK = {
  syncJobId: "9012",
  gameId: "100001",
  sourceEnv: "sandbox",
  targetEnv: "production",
  status: "succeeded",
  selectedSections: ["channels", "products"],
  includeDeletes: false,
  sourceHash: PREVIEW.sourceHash,
  targetHashBefore: PREVIEW.targetHashBefore,
  targetHashAfter: "sha256-target-after-eeeeeeee",
  appliedSummary: {
    channels: { add: 1, update: 1, delete: 0 },
    products: { add: 0, update: 1, delete: 0 }
  },
  skipped: { deletes: [{ section: "channels", entityKey: "KR/apple", reason: "include_deletes=false" }], unselectedSections: [] },
  executedAt: "2026-06-17T13:10:42Z"
};

const JOBS = [
  {
    syncJobId: "9012",
    gameId: "100001",
    sourceEnv: "sandbox",
    targetEnv: "production",
    status: "succeeded",
    selectedSections: ["channels", "products"],
    includeDeletes: false,
    operatorId: 1,
    operatorName: "管理员",
    operatorNote: "首次上线",
    sourceHash: "sha256-source-aaaaaaaabbbbbbbb",
    targetHashBefore: "sha256-target-ccccccccdddddddd",
    targetHashAfter: "sha256-target-after-eeeeeeee",
    executedAt: "2026-06-17T13:10:42Z",
    createdAt: "2026-06-17T13:00:00Z",
    appliedSummary: { channels: { add: 1, update: 1, delete: 0 } }
  },
  {
    syncJobId: "9010",
    gameId: "100001",
    sourceEnv: "sandbox",
    targetEnv: "production",
    status: "failed",
    selectedSections: ["channels"],
    includeDeletes: false,
    operatorId: 1,
    operatorName: "管理员",
    operatorNote: "",
    sourceHash: "sha256-source-11112222",
    targetHashBefore: "sha256-target-33334444",
    targetHashAfter: "",
    executedAt: "2026-06-17T12:00:00Z",
    createdAt: "2026-06-17T11:59:00Z",
    errorSummary: {
      code: "SYNC_BASELINE_MISMATCH",
      message: "目标已在预览后变更",
      details: [{ field: "targetHashBefore", expected: "sha256-...", actual: "sha256-***" }]
    }
  }
];

function meBody(permissions: string[], environment: string) {
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
      environment
    }
  };
}

function makeJson(environment: string) {
  // 使用 Playwright `json` 参数序列化响应体：由 Playwright 计算正确的 UTF-8
  // Content-Length，规避 Chrome 对多字节 body 字符串长度/字节长度不一致时
  // 响应流挂起的环境问题（详见 audit.log 记录）。
  return (route: Route, status: number, body: unknown) =>
    route.fulfill({
      status,
      headers: { "X-Environment": environment },
      json: body
    });
}

interface SetupOptions {
  permissions?: string[];
  environment?: string;
  executeStatus?: number;
  executeError?: { code: string; message: string; details?: unknown[] };
}

async function setup(page: Page, options: SetupOptions = {}) {
  const permissions = options.permissions ?? SESSION.user.permissions;
  const environment = options.environment ?? "sandbox";
  const json = makeJson(environment);

  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  // 兜底优先注册（Playwright 后注册者优先命中）。
  await page.route("**/api/admin/**", (route) => json(route, 200, { data: {} }));
  await page.route("**/api/admin/me", (route) => json(route, 200, meBody(permissions, environment)));

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

  // 同步历史列表
  await page.route(/\/api\/admin\/games\/100001\/sync-jobs(\?.*)?$/, (route) => {
    const url = new URL(route.request().url());
    const statusFilter = url.searchParams.get("status");
    const items = statusFilter ? JOBS.filter((j) => j.status === statusFilter) : JOBS;
    return json(route, 200, { data: { items, page: 1, pageSize: 20, total: items.length } });
  });

  // 预览
  await page.route(/\/api\/admin\/games\/100001\/sync\/preview$/, (route) => json(route, 200, { data: PREVIEW }));

  // 执行（可注入错误码）
  await page.route(/\/api\/admin\/games\/100001\/sync\/execute$/, (route) => {
    if (options.executeStatus && options.executeStatus >= 400) {
      return json(route, options.executeStatus, {
        error: {
          code: options.executeError?.code ?? "INTERNAL",
          message: options.executeError?.message ?? "boom",
          details: options.executeError?.details ?? []
        }
      });
    }
    return json(route, 200, { data: EXECUTE_OK });
  });

  // 游戏详情 + markets/legal
  await page.route(/\/api\/admin\/games\/100001(\/(markets|legal-links))?(\?.*)?$/, (route) => {
    const url = route.request().url();
    if (url.includes("/legal-links")) {
      return json(route, 200, { data: { legalLinks: [] } });
    }
    return json(route, 200, { data: GAME });
  });
}

async function openGameDetail(page: Page) {
  await page.goto("/dashboard");
  await page.getByRole("link", { name: "游戏管理" }).click();
  await page.getByText("星际远征").click();
  // 详情页首次进入需 vite 冷编译整页重型 Tab，放宽等待以容忍本地冷启动。
  await expect(page.locator(".detail-head__title")).toContainText("星际远征", { timeout: 45_000 });
}

async function openSyncDrawer(page: Page) {
  await openGameDetail(page);
  await page.getByRole("button", { name: "Sync to Production" }).click();
  await expect(page.locator(".sync-drawer")).toBeVisible();
  await expect(page.locator(".sync-section-card").first()).toBeVisible();
}

test("红线正向：sandbox 详情页渲染 Sync to Production 入口", async ({ page }) => {
  await setup(page);
  await openGameDetail(page);
  await expect(page.getByRole("button", { name: "Sync to Production" })).toBeVisible();
});

test("红线：production 运行环境绝不渲染 Sync 入口", async ({ page }) => {
  await setup(page, { environment: "production" });
  await openGameDetail(page);
  await expect(page.getByRole("button", { name: "Sync to Production" })).toHaveCount(0);
});

test("权限置灰：sandbox 但无 sync.execute 时入口禁用", async ({ page }) => {
  await setup(page, { permissions: ["game.read", "sync.preview"] });
  await openGameDetail(page);
  await expect(page.getByRole("button", { name: "Sync to Production" })).toBeDisabled();
});

test("预览抽屉：按 section 分组 + 计数徽标 + 差异行配色（截图基线）", async ({ page }) => {
  await setup(page);
  await openSyncDrawer(page);

  // section 分组
  await expect(page.locator(".sync-section-card__selector strong").filter({ hasText: "channels" })).toBeVisible();
  await expect(page.locator(".sync-section-card__selector strong").filter({ hasText: "products" })).toBeVisible();
  // 计数徽标
  await expect(page.locator(".el-tag").filter({ hasText: "add 1" }).first()).toBeVisible();
  await expect(page.locator(".el-tag").filter({ hasText: "update 1" }).first()).toBeVisible();
  await expect(page.locator(".el-tag").filter({ hasText: "delete 1" }).first()).toBeVisible();
  // 差异行配色
  await expect(page.locator(".sync-change--add").first()).toBeVisible();
  await expect(page.locator(".sync-change--update").first()).toBeVisible();
  await expect(page.locator(".sync-change--delete").first()).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/sync-drawer-preview.png", fullPage: true });
  await expect(page.locator(".sync-drawer")).toHaveScreenshot("sync-drawer-preview.png", { maxDiffPixelRatio: 0.02 });
});

test("密文脱敏：masked 行显示 •••••• 且不出现明文", async ({ page }) => {
  await setup(page);
  await openSyncDrawer(page);

  const drawerText = await page.locator(".sync-drawer").innerText();
  expect(drawerText).toContain("••••••");
  expect(drawerText).not.toContain("PLAINTEXT_SANDBOX_SECRET");
  expect(drawerText).not.toContain("PLAINTEXT_PROD_SECRET");
});

test("include_deletes 默认关：delete 行标注「仅提示，不执行」", async ({ page }) => {
  await setup(page);
  await openSyncDrawer(page);

  await expect(page.locator(".sync-change--delete-muted")).toBeVisible();
  await expect(page.getByText("仅提示，不执行")).toBeVisible();
});

test("执行成功：携带 payload 执行 → 成功提示 → 切换到同步记录并展示历史", async ({ page }) => {
  await setup(page);
  await openSyncDrawer(page);

  const execReq = page.waitForRequest(
    (req) => req.method() === "POST" && req.url().includes("/sync/execute")
  );
  await page.getByRole("button", { name: "执行同步" }).click();
  const req = await execReq;
  const body = JSON.parse(req.postData() ?? "{}");
  expect(body.baselineToken).toBe("baseline-token-e2e.sig");
  expect(body.selectedSections).toEqual(["channels", "products"]);
  expect(body.includeDeletes).toBe(false);

  await expect(page.getByText("同步执行成功")).toBeVisible();
  // 执行后切到「同步记录」Tab 并刷新历史
  await expect(page.locator(".sync-jobs-tab")).toBeVisible();
  await expect(page.locator(".sync-jobs-tab").getByText("9012").first()).toBeVisible();
});

test("SYNC_BASELINE_MISMATCH：执行返回 409 → 弹窗提示重新预览", async ({ page }) => {
  await setup(page, {
    executeStatus: 409,
    executeError: { code: "SYNC_BASELINE_MISMATCH", message: "目标已在预览后变更", details: [] }
  });
  await openSyncDrawer(page);

  await page.getByRole("button", { name: "执行同步" }).click();
  await expect(page.getByText("目标已变更，请重新预览")).toBeVisible();
  await expect(page.getByRole("button", { name: "重新预览" }).last()).toBeVisible();
});

test("同步历史：列表渲染 + status 过滤 + 失败行展开错误概要（截图基线）", async ({ page }) => {
  await setup(page);
  await openGameDetail(page);
  await page.getByRole("tab", { name: "同步记录" }).click();
  await expect(page.locator(".sync-jobs-tab")).toBeVisible();

  // 列表渲染任务与状态
  await expect(page.getByText("9012").first()).toBeVisible();
  await expect(page.locator(".el-tag").filter({ hasText: "succeeded" }).first()).toBeVisible();
  await expect(page.locator(".el-tag").filter({ hasText: "failed" }).first()).toBeVisible();

  // 失败行展开错误概要
  await page
    .locator(".el-table__row")
    .filter({ hasText: "9010" })
    .locator(".el-table__expand-icon")
    .click();
  await expect(page.getByText("SYNC_BASELINE_MISMATCH")).toBeVisible();
  await expect(page.getByText("目标已在预览后变更")).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/sync-jobs-tab.png", fullPage: true });

  // status 过滤：仅看 failed
  const filterReq = page.waitForRequest((req) => req.url().includes("/sync-jobs") && req.url().includes("status=failed"));
  await page.locator(".sync-jobs-tab .status-select").click();
  await page.getByRole("option", { name: "failed" }).click();
  await filterReq;
  await expect(page.getByText("9012")).toHaveCount(0);
});
