import { expect, test, type Page, type Route } from "@playwright/test";

// 平台渠道管理 UI 用例（对齐 03-testing §5.2 与 00-common §4.4）：
// 顶部菜单「渠道管理」是系统管理员维护的平台级渠道主数据 + 渠道模版，与游戏无关。
// 对 /api/admin/platform/** 契约做 mock/stub，验证两个页签的列表、新建渠道、新建模版版本、
// 编辑时身份字段只读、生效版本标记与权限置灰。

// dev server 首次按需编译抽屉/面板/element-plus 体积较大，放宽超时避免冷启动误判。
test.describe.configure({ timeout: 120_000 });

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
    permissions: [
      "dashboard.read",
      "platform_channel.read",
      "platform_channel.write",
      "channel_template.read",
      "channel_template.write"
    ]
  }
};

const ALL_PERMS = [
  "dashboard.read",
  "platform_channel.read",
  "platform_channel.write",
  "channel_template.read",
  "channel_template.write"
];

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

const GOOGLE = {
  channelId: "google",
  channelName: "Google Play",
  channelType: "store",
  region: "overseas",
  enabled: true,
  sort: 1,
  loginMode: "channel_only",
  paymentMode: "channel_only",
  loginLocked: false,
  paymentLocked: false,
  loginTemplateCount: 0,
  iapTemplateCount: 0,
  updatedAt: "2026-01-01T00:00:00Z"
};

const HUAWEI = {
  channelId: "huawei_cn",
  channelName: "华为应用市场",
  channelType: "oem",
  region: "domestic",
  enabled: true,
  sort: 2,
  loginMode: "channel_only",
  paymentMode: "channel_only",
  loginLocked: true,
  paymentLocked: false,
  loginTemplateCount: 2,
  iapTemplateCount: 0,
  updatedAt: "2026-01-02T00:00:00Z"
};

const TEMPLATE_V1 = {
  templateId: 1,
  kind: "login",
  channelId: "huawei_cn",
  templateVersion: "v1",
  formSchemaJson: [{ key: "appId", label: "App ID", component: "input", required: true, order: 10, scope: "both" }],
  secretFieldsJson: [],
  fileFieldsJson: [],
  validationRulesJson: {},
  enabled: true,
  effective: true,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z"
};

const TEMPLATE_V2 = {
  ...TEMPLATE_V1,
  templateId: 2,
  templateVersion: "v2",
  enabled: false,
  effective: false
};

interface SetupOptions {
  permissions?: string[];
}

async function setup(page: Page, options: SetupOptions = {}) {
  const permissions = options.permissions ?? ALL_PERMS;

  await page.addInitScript((session) => {
    window.localStorage.setItem("admin-auth", JSON.stringify(session));
  }, SESSION);

  // 兜底：其它后台接口返回空，避免 dashboard 等页面挂起。
  await page.route("**/api/admin/**", (route) => json(route, 200, { data: {} }));
  await page.route("**/api/admin/me", (route) => json(route, 200, meBody(permissions)));

  // 渠道列表 / 新建
  await page.route(/\/api\/admin\/platform\/channels(\?.*)?$/, (route) => {
    if (route.request().method() === "POST") {
      return json(route, 201, { data: { ...GOOGLE, channelId: "vivo_cn", channelName: "vivo" } });
    }
    return json(route, 200, { data: { items: [GOOGLE, HUAWEI], page: 1, pageSize: 20, total: 2 } });
  });

  // 单渠道详情 / 编辑
  await page.route(/\/api\/admin\/platform\/channels\/[^/]+$/, (route) => {
    if (route.request().method() === "PATCH") {
      return json(route, 200, { data: { ...HUAWEI, channelName: "华为(改名)" } });
    }
    return json(route, 200, { data: HUAWEI });
  });

  // 模版列表 / 新建版本
  await page.route(/\/api\/admin\/platform\/channels\/[^/]+\/templates(\?.*)?$/, (route) => {
    if (route.request().method() === "POST") {
      return json(route, 201, { data: { ...TEMPLATE_V1, templateId: 3, templateVersion: "v3" } });
    }
    const url = route.request().url();
    // huawei_cn 有两个登录模版版本；google 无模版
    if (url.includes("/channels/google/")) {
      return json(route, 200, { data: { items: [] } });
    }
    return json(route, 200, { data: { items: [TEMPLATE_V2, TEMPLATE_V1] } });
  });

  await page.route(/\/api\/admin\/platform\/channel-templates\/[^/]+\/\d+$/, (route) =>
    json(route, 200, { data: { ...TEMPLATE_V1, enabled: false } })
  );
}

// 平台渠道入口：工作台 → 侧边栏「渠道管理」。
async function gotoPlatformChannels(page: Page) {
  await page.goto("/dashboard");
  await page.getByRole("link", { name: "渠道管理" }).click();
  await expect(page.getByRole("tab", { name: "渠道", exact: true })).toBeVisible({ timeout: 60_000 });
}

async function gotoTemplatesTab(page: Page) {
  await gotoPlatformChannels(page);
  await page.getByRole("tab", { name: "渠道模版" }).click();
  // 模版隶属于渠道，面板默认选中列表首个渠道（google，无模版），显式切到有模版版本的 huawei_cn。
  await page.locator(".filter-channel").click();
  await page.getByRole("option", { name: "华为应用市场（huawei_cn）" }).click();
}

test("渠道管理页说明其为平台级主数据，并展示渠道列表与模版计数", async ({ page }) => {
  await setup(page);
  await gotoPlatformChannels(page);

  // 页面定位说明：系统管理员维护、与游戏无关
  await expect(page.getByText(/系统管理员维护的平台级渠道主数据/)).toBeVisible();
  await expect(page.getByText(/与具体游戏无关/)).toBeVisible();

  // exact 避免与「Google Play」渠道名单元格撞名（严格模式会报多元素）。
  await expect(page.getByRole("cell", { name: "google", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "huawei_cn", exact: true })).toBeVisible();
  // 枚举中文标签 + 锁定位 + 模版计数
  await expect(page.getByText("应用商店").first()).toBeVisible();
  await expect(page.getByText("手机厂商").first()).toBeVisible();
  await expect(page.getByText("登录锁定")).toBeVisible();
  await expect(page.getByText("登录 2 / IAP 0")).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/platform-channels-list.png", fullPage: true });
});

test("筛选条件下发到平台渠道查询", async ({ page }) => {
  await setup(page);
  await gotoPlatformChannels(page);

  const queryPromise = page.waitForRequest(
    (req) => req.url().includes("/api/admin/platform/channels?") && req.url().includes("region=domestic")
  );
  await page.getByPlaceholder("渠道 ID / 渠道名").fill("huawei");
  await page.locator(".filter-select").first().click();
  // exact 避免匹配到 overseas 的「非国内」选项。
  await page.getByRole("option", { name: "国内", exact: true }).click();
  const req = await queryPromise;
  expect(req.url()).toContain("keyword=huawei");
  expect(req.url()).toContain("region=domestic");
});

test("新建渠道抽屉提交全字段", async ({ page }) => {
  await setup(page);
  await gotoPlatformChannels(page);

  await page.getByRole("button", { name: "新建渠道" }).click();
  await page.getByPlaceholder(/小写字母\/数字\/下划线/).fill("vivo_cn");
  await page.getByPlaceholder(/1-64 字符/).fill("vivo");

  const postPromise = page.waitForRequest(
    (req) => req.method() === "POST" && req.url().endsWith("/api/admin/platform/channels")
  );
  await page.getByRole("button", { name: "保存" }).click();
  const req = await postPromise;
  const body = req.postData() ?? "";
  expect(body).toContain("vivo_cn");
  expect(body).toContain("\"channelType\"");
  expect(body).toContain("\"region\"");
  await expect(page.locator(".el-message").getByText("已创建渠道")).toBeVisible();
});

test("编辑渠道时 channelId / 类型 / region 只读并给出不可改说明", async ({ page }) => {
  await setup(page);
  await gotoPlatformChannels(page);

  await page.getByRole("button", { name: "编辑" }).first().click();
  const drawer = page.locator(".el-drawer");
  await expect(drawer.getByText(/渠道 ID 是渠道实例的引用键/)).toBeVisible();
  await expect(drawer.getByText(/region 决定与 market 的兼容性/)).toBeVisible();
  // 身份字段被禁用
  await expect(drawer.locator("input[disabled]").first()).toBeVisible();

  // PATCH 不下发 channelId / channelType / region
  const patchPromise = page.waitForRequest((req) => req.method() === "PATCH");
  await drawer.getByRole("button", { name: "保存" }).click();
  const body = (await patchPromise).postData() ?? "";
  expect(body).not.toContain("\"channelId\"");
  expect(body).not.toContain("\"channelType\"");
  expect(body).not.toContain("\"region\"");
  expect(body).toContain("\"channelName\"");
});

test("渠道模版页签标记生效版本，停用版本不生效", async ({ page }) => {
  await setup(page);
  await gotoTemplatesTab(page);

  // 限定在模版页签内：渠道页签的「启用状态」下拉把「已停用」选项也留在了 DOM 里，全页匹配会撞严格模式。
  const pane = page.locator("#pane-templates");
  await expect(pane.getByRole("cell", { name: "v1" })).toBeVisible({ timeout: 60_000 });
  await expect(pane.getByRole("cell", { name: "v2" })).toBeVisible();
  // v1 启用且生效；v2 停用且未生效
  await expect(pane.getByText("生效中")).toBeVisible();
  await expect(pane.getByText("未生效")).toBeVisible();
  await expect(pane.getByText("已停用")).toBeVisible();
  // 运行时口径说明
  await expect(pane.getByText(/enabled 的最新版本/)).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/platform-channel-templates.png", fullPage: true });
});

test("新建模版版本提交四件套", async ({ page }) => {
  await setup(page);
  await gotoTemplatesTab(page);

  await page.getByRole("button", { name: "新建模版版本" }).click();
  const drawer = page.locator(".el-drawer");
  await drawer.getByPlaceholder(/字母\/数字\/点\/横线\/下划线/).fill("v3");
  // 四件套编辑器：填第一行表单字段
  await drawer.getByPlaceholder(/^key（/).first().fill("appId");
  await drawer.getByPlaceholder("标签").first().fill("App ID");

  const postPromise = page.waitForRequest((req) => req.method() === "POST" && req.url().includes("/templates"));
  await drawer.getByRole("button", { name: "保存" }).click();
  const body = (await postPromise).postData() ?? "";
  expect(body).toContain("\"templateVersion\":\"v3\"");
  expect(body).toContain("appId");
  expect(body).toContain("\"formSchemaJson\"");
  expect(body).toContain("\"secretFieldsJson\"");
  expect(body).toContain("\"fileFieldsJson\"");
  expect(body).toContain("\"validationRulesJson\"");
  await expect(page.locator(".el-message").getByText("已创建模版版本")).toBeVisible();
});

test("口令字段未登记敏感字段时前端前置拦截，不打接口", async ({ page }) => {
  await setup(page);
  await gotoTemplatesTab(page);

  await page.getByRole("button", { name: "新建模版版本" }).click();
  const drawer = page.locator(".el-drawer");
  await drawer.getByPlaceholder(/字母\/数字\/点\/横线\/下划线/).fill("v9");
  await drawer.getByPlaceholder(/^key（/).first().fill("appSecret");
  await drawer.getByPlaceholder("标签").first().fill("密钥");
  // 组件改为 password（第一行的组件下拉）
  await drawer.locator(".field").first().locator(".el-select").first().click();
  await page.getByRole("option", { name: "password", exact: true }).click();

  let posted = false;
  page.on("request", (req) => {
    if (req.method() === "POST" && req.url().includes("/templates")) {
      posted = true;
    }
  });
  await drawer.getByRole("button", { name: "保存" }).click();
  await expect(drawer.getByText(/必须登记为敏感字段/)).toBeVisible();
  expect(posted).toBe(false);
});

test("无写权限时新建按钮置灰（渠道与模版两个页签）", async ({ page }) => {
  await setup(page, { permissions: ["dashboard.read", "platform_channel.read", "channel_template.read"] });
  await gotoPlatformChannels(page);
  await expect(page.getByRole("button", { name: "新建渠道" })).toBeDisabled();

  await page.getByRole("tab", { name: "渠道模版" }).click();
  await expect(page.getByRole("button", { name: "新建模版版本" })).toBeDisabled();
});
