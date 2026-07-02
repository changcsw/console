import { expect, test, type Page, type Route } from "@playwright/test";

// channel-login UI 用例（对齐 03-testing §5.2 / 14-channel-login spec.compact §前端要点）：
// 对 GET/PUT login-config 契约做 mock/stub，验证：仅 channel_only 展示「渠道登录」页签、
// 模板四件套渲染、密文脱敏、config_status 三色与 enabled+invalid 告警、channel.write 权限置灰，
// 并采集关键状态截图基线（toHaveScreenshot）。

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
    permissions: ["dashboard.read", "channel.read", "channel.write"]
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

const HUAWEI_INSTANCE = {
  gameChannelId: 101,
  displayKey: "100001:CN:huawei_cn",
  gameId: "100001",
  market: "CN",
  channelId: "huawei_cn",
  region: "domestic",
  compatible: true,
  hidden: false,
  configStatus: "valid",
  includedInSnapshot: true,
  includedInSync: true,
  includedInRuntimeConfig: true,
  copiedFromMarket: "",
  updatedAt: "2026-01-01T00:00:00Z"
};

const HUAWEI_DETAIL = {
  ...HUAWEI_INSTANCE,
  channelName: "华为(中国)",
  channelType: "store",
  loginMode: "channel_only",
  paymentMode: "channel_only",
  loginLocked: false,
  paymentLocked: false,
  enabled: true,
  remark: "",
  hiddenBy: "",
  hiddenAt: null,
  lastCheckAt: "2026-01-01T00:00:00Z",
  lastCheckMessage: "校验通过",
  createdAt: "2026-01-01T00:00:00Z"
};

const TEMPLATE = {
  templateVersion: "v1",
  formSchemaJson: [
    { key: "appId", label: "App ID", component: "input", required: true, order: 1, group: "基础" },
    {
      key: "region",
      label: "区域",
      component: "select",
      order: 2,
      group: "基础",
      options: [
        { label: "中国大陆", value: "cn" },
        { label: "海外", value: "global" }
      ]
    },
    { key: "appSecret", label: "App Secret", component: "password", required: true, order: 3, group: "密钥" }
  ],
  secretFieldsJson: ["appSecret"],
  fileFieldsJson: [],
  validationRulesJson: {
    appId: { minLen: 1, maxLen: 64, pattern: "^[0-9A-Za-z_-]+$" },
    appSecret: { minLen: 8, maxLen: 256 }
  }
};

function loginConfigBody(over: Partial<{ enabled: boolean; configStatus: string; lastCheckMessage: string }> = {}) {
  return {
    data: {
      gameChannelId: 101,
      env: "sandbox",
      channelId: "huawei_cn",
      marketCode: "CN",
      loginMode: "channel_only",
      loginLocked: false,
      config: {
        enabled: over.enabled ?? true,
        configJson: { appId: "huawei-app-001", region: "cn", appSecret: "******" },
        configStatus: over.configStatus ?? "valid",
        lastCheckAt: "2026-01-01T00:00:00Z",
        lastCheckMessage: over.lastCheckMessage ?? "校验通过"
      },
      template: TEMPLATE
    }
  };
}

interface SetupOptions {
  permissions?: string[];
  loginConfig?: Partial<{ enabled: boolean; configStatus: string; lastCheckMessage: string }>;
}

async function setup(page: Page, options: SetupOptions = {}) {
  const permissions = options.permissions ?? ["channel.read", "channel.write"];

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
            defaultMarketCode: "CN",
            marketCodes: ["CN"],
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
            channelId: "huawei_cn",
            channelName: "华为(中国)",
            channelType: "store",
            region: "domestic",
            loginMode: "channel_only",
            paymentMode: "channel_only",
            loginLocked: false,
            paymentLocked: false
          }
        ]
      }
    })
  );
  await page.route(/\/api\/admin\/games\/100001\/market-channels(\?.*)?$/, (route) =>
    json(route, 200, {
      data: { items: [HUAWEI_INSTANCE], page: 1, pageSize: 20, total: 1 }
    })
  );
  await page.route(/\/api\/admin\/game-channels\/101$/, (route) => json(route, 200, { data: HUAWEI_DETAIL }));
  await page.route(/\/api\/admin\/game-channels\/101\/packages$/, (route) =>
    json(route, 200, { data: { items: [] } })
  );
  // GET/PUT login-config 契约 stub
  await page.route(/\/api\/admin\/game-channels\/101\/login-config$/, (route) => {
    if (route.request().method() === "PUT") {
      return json(route, 200, loginConfigBody(options.loginConfig));
    }
    return json(route, 200, loginConfigBody(options.loginConfig));
  });
}

async function openLoginTab(page: Page) {
  await page.goto("/dashboard");
  await page.getByRole("link", { name: "渠道管理" }).click();
  await expect(page.getByText("渠道实例管理")).toBeVisible({ timeout: 60_000 });
  // 打开实例详情抽屉（等待行渲染后再点击，规避列表加载/冷编译竞态）
  const detailBtn = page.getByRole("button", { name: "详情" }).first();
  await expect(detailBtn).toBeVisible({ timeout: 60_000 });
  await detailBtn.click();
  await expect(page.getByText("渠道实例详情")).toBeVisible({ timeout: 60_000 });
  // 切到「渠道登录」页签
  await page.getByRole("tab", { name: "渠道登录" }).click();
  await expect(page.getByText("启用渠道登录")).toBeVisible();
}

test("channel_only 实例展示渠道登录页签并渲染模板四件套+脱敏+valid 状态", async ({ page }) => {
  await setup(page);
  await openLoginTab(page);
  const panel = page.locator(".panel").first();

  // 四件套字段渲染（限定 panel 内的表单标签，避开表头/描述列同名文本）
  await expect(panel.locator(".el-form-item__label", { hasText: "App ID" })).toBeVisible();
  await expect(panel.locator(".el-form-item__label", { hasText: "区域" })).toBeVisible();
  await expect(panel.locator(".el-form-item__label", { hasText: "App Secret" })).toBeVisible();
  // 分组渲染
  await expect(panel.getByRole("heading", { name: "基础" })).toBeVisible();
  await expect(panel.getByRole("heading", { name: "密钥" })).toBeVisible();
  // 顶部只读上下文
  await expect(panel.getByText("marketCode: CN")).toBeVisible();
  await expect(panel.getByText("loginMode: channel_only")).toBeVisible();
  // config_status valid 绿
  await expect(panel.locator(".panel__status")).toContainText("配置有效");
  // 密文脱敏占位（绝不回明文）
  const secretInput = panel.locator('input[type="password"]').first();
  await expect(secretInput).toHaveValue("******");

  await expect(panel).toHaveScreenshot("channel-login-valid.png", { maxDiffPixelRatio: 0.02 });
});

test("enabled=true 但 invalid 展示告警条与红色状态", async ({ page }) => {
  await setup(page, { loginConfig: { enabled: true, configStatus: "invalid", lastCheckMessage: "App Secret 校验失败" } });
  await openLoginTab(page);
  const panel = page.locator(".panel").first();

  await expect(panel.getByText("已启用但配置无效，将不进入快照/同步/客户端最终配置")).toBeVisible();
  await expect(panel.locator(".panel__status")).toContainText("配置无效");
  await expect(panel.locator(".panel__status-message")).toContainText("App Secret 校验失败");

  await expect(panel).toHaveScreenshot("channel-login-invalid.png", {
    maxDiffPixelRatio: 0.02
  });
});

test("无 channel.write 权限时保存按钮与输入置灰", async ({ page }) => {
  await setup(page, { permissions: ["channel.read"] });
  await openLoginTab(page);
  const panel = page.locator(".panel").first();

  await expect(panel.getByRole("button", { name: "保存渠道登录配置" })).toBeDisabled();
  // 启用开关也置灰
  await expect(panel.locator(".el-switch.is-disabled").first()).toBeVisible();
});
