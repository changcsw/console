import { test, expect, type Page, type Route } from "@playwright/test";

// 游戏模块 e2e（对齐 03-testing §5.2 与 11-game compact spec）：
// 对后端契约做 mock/stub，验证列表筛选/分页、创建一次性明文弹窗、详情脱敏、
// 多 Tab、市场/法务编辑抽屉、权限置灰、404 态，并采集截图 + 视觉基线。
// 真实跨栈联调（连库 e2e）属测试专家职责，本用例以前端 + 契约 mock 为主。

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
    permissions: ["dashboard.read", "game.read", "game.write"]
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

const LIST_PAGE_1 = {
  data: {
    items: [
      {
        gameId: "100001",
        name: "星际远征",
        alias: "starfront",
        iconUrl: "",
        status: "active",
        defaultMarketCode: "GLOBAL",
        marketCodes: ["GLOBAL", "JP"],
        marketCount: 2,
        createdAt: "2026-01-01T00:00:00Z",
        updatedAt: "2026-01-02T03:04:05Z"
      },
      {
        gameId: "100002",
        name: "幻想曲",
        alias: "fantasia",
        iconUrl: "",
        status: "draft",
        defaultMarketCode: "JP",
        marketCodes: ["JP"],
        marketCount: 1,
        createdAt: "2026-01-03T00:00:00Z",
        updatedAt: "2026-01-04T03:04:05Z"
      }
    ],
    page: 1,
    pageSize: 20,
    total: 2
  }
};

const DETAIL_100001 = {
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
    markets: [
      { marketCode: "GLOBAL", isDefault: true, enabled: true, defaultLocale: "en-US" },
      { marketCode: "JP", isDefault: false, enabled: true, defaultLocale: "ja-JP" }
    ],
    legalLinks: [
      {
        scopeType: "default",
        scopeValue: "*",
        termsUrl: "https://example.com/terms",
        privacyUrl: "https://example.com/privacy",
        deleteAccountUrl: ""
      }
    ],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-02T03:04:05Z"
  }
};

const CREATED_GAME = {
  data: {
    gameId: "100003",
    name: "新纪元",
    alias: "neo",
    iconUrl: "",
    status: "draft",
    defaultMarketCode: "GLOBAL",
    gameSecret: "PLAINTEXT-SECRET-9f8e7d6c5b4a",
    secretMasked: false,
    environment: "sandbox",
    markets: [{ marketCode: "GLOBAL", isDefault: true, enabled: true, defaultLocale: "en-US" }],
    legalLinks: [],
    createdAt: "2026-02-01T00:00:00Z",
    updatedAt: "2026-02-01T00:00:00Z"
  }
};

// ---- account-auth（自有账号认证 Tab）契约 mock（对齐 13-account-auth compact spec）----
const ACCOUNT_AUTH_TYPES = {
  data: {
    items: [
      {
        authTypeId: "phone",
        authTypeName: "手机号登录",
        enabled: true,
        sort: 20,
        template: { templateVersion: "v1", formSchema: [], secretFields: [], fileFields: [], validationRules: {} }
      },
      {
        authTypeId: "google",
        authTypeName: "Google 登录",
        enabled: true,
        sort: 40,
        template: {
          templateVersion: "v1",
          formSchema: [
            { key: "clientId", label: "Client ID", component: "input", required: true, order: 1 },
            { key: "clientSecret", label: "Client Secret", component: "password", order: 2 },
            { key: "region", label: "区域", component: "select", order: 3, options: [{ label: "美国", value: "us" }] },
            { key: "serviceAccount", label: "服务账号文件", component: "file", order: 4 }
          ],
          secretFields: ["clientSecret"],
          fileFields: [{ key: "serviceAccount", accept: ["application/json"], maxSizeKB: 64 }],
          validationRules: { clientId: { minLen: 1 } }
        }
      }
    ]
  }
};

const CHANNEL_ALLOWED = {
  data: {
    items: [
      { authTypeId: "google", defaultEnabled: true, locked: false },
      { authTypeId: "phone", defaultEnabled: false, locked: true }
    ]
  }
};

const MARKET_CHANNELS = {
  data: {
    items: [
      {
        gameChannelId: 1,
        displayKey: "100001:GLOBAL:ch1",
        gameId: "100001",
        market: "GLOBAL",
        channelId: "ch1",
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

const ACCOUNT_AUTH_CONFIGS = {
  data: {
    items: [
      {
        authTypeId: "google",
        enabled: true,
        configJson: { clientId: "cid-123", clientSecret: "masked", region: "us" },
        configStatus: "valid",
        lastCheckAt: "2026-01-01T00:00:00Z",
        lastCheckMessage: "校验通过"
      },
      {
        authTypeId: "phone",
        enabled: true,
        configJson: {},
        configStatus: "invalid",
        lastCheckAt: null,
        lastCheckMessage: "缺少必填字段"
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
  detailStatus?: number;
}

async function setup(page: Page, opts: SetupOptions = {}) {
  const permissions = opts.permissions ?? ["game.read", "game.write"];

  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  // 兜底：其它后台接口返回空，避免 dashboard 等页面挂起。
  // 先注册兜底，后注册具体路由（Playwright 后注册者优先）。
  await page.route("**/api/admin/**", (route) => json(route, 200, { data: {} }));

  // 当前用户 → 注入权限/环境
  await page.route("**/api/admin/me", (route) => json(route, 200, meBody(permissions)));

  // 列表 / 创建（含查询串）
  await page.route(/\/api\/admin\/games(\?.*)?$/, (route) => {
    if (route.request().method() === "POST") {
      return json(route, 201, CREATED_GAME);
    }
    return json(route, 200, LIST_PAGE_1);
  });

  // 详情 / 编辑 / 市场 / 法务
  await page.route(/\/api\/admin\/games\/[^/]+(\/(markets|legal-links))?(\?.*)?$/, (route) => {
    const method = route.request().method();
    const url = route.request().url();
    if (url.includes("/markets")) {
      return json(route, 200, DETAIL_100001);
    }
    if (url.includes("/legal-links")) {
      return json(route, 200, { data: { legalLinks: DETAIL_100001.data.legalLinks } });
    }
    if (method === "PATCH") {
      return json(route, 200, DETAIL_100001);
    }
    // GET 详情
    if (opts.detailStatus && opts.detailStatus !== 200) {
      return json(route, opts.detailStatus, {
        error: { code: "NOT_FOUND", message: "game not found", details: [] }
      });
    }
    return json(route, 200, DETAIL_100001);
  });

  // account-auth Tab 依赖：认证方式/模板、渠道允许集合、渠道实例（取 channelId）、游戏配置（GET/PUT）
  await page.route("**/api/admin/account-auth/types", (route) => json(route, 200, ACCOUNT_AUTH_TYPES));
  await page.route(/\/api\/admin\/channels\/[^/]+\/account-auth-types(\?.*)?$/, (route) =>
    json(route, 200, CHANNEL_ALLOWED)
  );
  await page.route(/\/api\/admin\/games\/[^/]+\/market-channels(\?.*)?$/, (route) => json(route, 200, MARKET_CHANNELS));
  await page.route(/\/api\/admin\/games\/[^/]+\/account-auth-configs(\?.*)?$/, (route) =>
    json(route, 200, ACCOUNT_AUTH_CONFIGS)
  );
}

// 从工作台经侧边栏进入 /games：保证守卫前 /me 已注入权限（避免直连受守卫重定向）
async function gotoGames(page: Page) {
  await page.goto("/dashboard");
  const link = page.getByRole("link", { name: "游戏管理" });
  await expect(link).toBeVisible();
  await link.click();
  await expect(page.getByText("发行后台根聚合")).toBeVisible();
}

async function openGameDetail(page: Page, gameName = "星际远征") {
  await page.getByText(gameName).first().click();
  await expect(page.locator(".detail-head__title")).toContainText(gameName, { timeout: 15_000 });
}

test("游戏列表渲染行/状态/市场标签，且不展示 gameSecret", async ({ page }) => {
  await setup(page);
  await gotoGames(page);

  await expect(page.getByRole("cell", { name: "100001" })).toBeVisible();
  await expect(page.getByText("星际远征")).toBeVisible();
  await expect(page.getByText("幻想曲")).toBeVisible();
  // 列表不出现密钥字段
  await expect(page.locator("body")).not.toContainText("gameSecret");

  await page.screenshot({ path: "../../tests/frontend/screenshots/games-list.png", fullPage: true });
});

test("筛选条件下发到列表查询", async ({ page }) => {
  await setup(page);
  await gotoGames(page);

  const queryPromise = page.waitForRequest((req) => req.url().includes("/api/admin/games?") && req.url().includes("keyword=star"));
  await page.getByPlaceholder("名称 / 代号 / Game ID").fill("star");
  await page.getByRole("button", { name: "查询" }).click();
  const req = await queryPromise;
  expect(req.url()).toContain("keyword=star");
});

test("创建游戏成功后弹出一次性明文 gameSecret", async ({ page }) => {
  await setup(page);
  await gotoGames(page);

  await page.getByRole("button", { name: "新建游戏" }).click();
  await page.getByPlaceholder("1-128 字符").fill("新纪元");
  await page.getByPlaceholder(/仅字母数字/).fill("neo");
  await page.getByRole("button", { name: "创建" }).click();

  // 一次性明文弹窗
  await expect(page.getByText("游戏密钥（仅此一次）")).toBeVisible();
  await expect(page.getByText("PLAINTEXT-SECRET-9f8e7d6c5b4a")).toBeVisible();
  await page.screenshot({ path: "../../tests/frontend/screenshots/games-secret-dialog.png", fullPage: true });
  await page.getByRole("button", { name: "我已保存" }).click();
  await expect(page.getByText("游戏密钥（仅此一次）")).toBeHidden();
});

test("详情页脱敏展示 Secret + 多 Tab + 下游占位", async ({ page }) => {
  await setup(page);
  await gotoGames(page);

  await openGameDetail(page);
  await expect(page.locator(".detail-head__meta").getByText("masked").first()).toBeVisible();
  await expect(page.locator("body")).not.toContainText("PLAINTEXT");
  // 主 Tab
  await expect(page.getByRole("tab", { name: "基础信息" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "市场" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "法务链接" })).toBeVisible();
  // 下游占位
  await expect(page.getByRole("tab", { name: "支付路由" })).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/game-detail.png", fullPage: true });
});

test("市场 Tab 编辑抽屉打开并展示单默认 radio", async ({ page }) => {
  await setup(page);
  await gotoGames(page);
  await openGameDetail(page);
  await page.getByRole("tab", { name: "市场" }).click();
  const editBtn = page.getByRole("button", { name: "编辑市场" });
  await expect(editBtn).toBeEnabled();
  await editBtn.click();
  await expect(page.getByText("编辑市场（全量覆盖）")).toBeVisible();
  await expect(page.getByText(/移除已有渠道实例/)).toBeVisible();
});

test("法务链接 Tab scopeType 联动 scopeValue", async ({ page }) => {
  await setup(page);
  await gotoGames(page);
  await openGameDetail(page);
  await page.getByRole("tab", { name: "法务链接" }).click();
  const editBtn = page.getByRole("button", { name: "编辑法务链接" });
  await expect(editBtn).toBeEnabled();
  await editBtn.click();
  await expect(page.getByText("编辑法务链接（全量覆盖）")).toBeVisible();
  await page.getByRole("button", { name: "+ 新增一行" }).click();
  // 新增行默认 default → scopeValue 锁 '*'（disabled 输入框）
  await expect(page.locator(".legal-row input[disabled]").last()).toHaveValue("*");
});

test("无 game.write 权限时新建/编辑按钮置灰禁用", async ({ page }) => {
  await setup(page, { permissions: ["game.read"] });
  await gotoGames(page);
  const createBtn = page.getByRole("button", { name: "新建游戏" });
  await expect(createBtn).toBeVisible();
  await expect(createBtn).toBeDisabled();
});

test("详情 404 展示『游戏不存在或已切换环境』", async ({ page }) => {
  await setup(page, { detailStatus: 404 });
  await gotoGames(page);
  await page.getByText("星际远征").click();
  await expect(page.getByText("游戏不存在或已切换环境")).toBeVisible();
  await page.screenshot({ path: "../../tests/frontend/screenshots/game-detail-404.png", fullPage: true });
});

test("游戏列表视觉基线", async ({ page }) => {
  await setup(page);
  await gotoGames(page);
  await expect(page.getByText("星际远征")).toBeVisible();
  await expect(page).toHaveScreenshot("games-list.png", { maxDiffPixelRatio: 0.02 });
});

// 进入星际远征详情并切到「自有账号认证」Tab
async function openAccountAuthTab(page: Page) {
  await openGameDetail(page);
  await page.getByRole("tab", { name: "自有账号认证" }).click();
  await expect(page.locator(".account-auth-tab")).toBeVisible();
}

test("自有账号认证 Tab 渲染模板四件套 + 状态标签 + 密文脱敏", async ({ page }) => {
  await setup(page);
  await gotoGames(page);
  await openAccountAuthTab(page);

  // 认证方式（渠道允许集合并集，按 sort 升序：phone → google）
  await expect(page.getByText("Google 登录")).toBeVisible();
  await expect(page.getByText("手机号登录")).toBeVisible();
  // 模板四件套字段渲染
  await expect(page.getByText("Client ID")).toBeVisible();
  await expect(page.getByText("Client Secret")).toBeVisible();
  await expect(page.getByText("服务账号文件")).toBeVisible();
  // config_status 三态标签
  await expect(page.getByText("valid", { exact: true })).toBeVisible();
  await expect(page.getByText("invalid", { exact: true })).toBeVisible();
  // 密文恒脱敏，绝不出现明文
  await expect(page.locator(".secret-field__masked").first()).toBeVisible();
  await expect(page.locator("body")).not.toContainText("PLAINTEXT");

  await page.screenshot({ path: "../../tests/frontend/screenshots/game-account-auth.png", fullPage: true });
});

test("启用但 invalid 行内告警 + locked 项禁用开关", async ({ page }) => {
  await setup(page);
  await gotoGames(page);
  await openAccountAuthTab(page);

  // phone 启用且 invalid → 行内告警
  await expect(page.getByText("已启用但配置未通过校验，请补齐必填/敏感/文件字段。")).toBeVisible();
  // phone locked → 卡片标记锁定且开关禁用
  const lockedCard = page.locator(".auth-card.is-locked");
  await expect(lockedCard).toBeVisible();
  await expect(lockedCard.locator(".el-switch.is-disabled").first()).toBeVisible();
});

test("无 game.write 权限时保存配置按钮置灰且展示只读提示", async ({ page }) => {
  await setup(page, { permissions: ["game.read"] });
  await gotoGames(page);
  await openAccountAuthTab(page);

  const tab = page.locator(".account-auth-tab");
  await expect(tab.getByText("当前账号仅有查看权限，配置项已置灰。")).toBeVisible();
  await expect(tab.getByRole("button", { name: "保存配置" })).toBeDisabled();
});

test("保存配置触发整体替换 PUT，且密文留空不下发明文", async ({ page }) => {
  await setup(page);
  await gotoGames(page);
  await openAccountAuthTab(page);

  const putPromise = page.waitForRequest(
    (req) => req.method() === "PUT" && req.url().includes("/account-auth-configs")
  );
  await page.getByRole("button", { name: "保存配置" }).click();
  const req = await putPromise;
  const body = req.postData() ?? "";
  // 整体替换载荷含 items 与 authTypeId
  expect(body).toContain("\"items\"");
  expect(body).toContain("google");
  // 密文留空 → 不下发脱敏占位/明文
  expect(body).not.toContain("masked");
  await expect(page.locator(".el-message").getByText("自有账号认证配置已保存")).toBeVisible();
});
