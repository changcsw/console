import { expect, test, type Page, type Route } from "@playwright/test";

// feature-plugin UI 用例（对齐 03-testing §5.2 / 15-feature-plugin spec.compact §前端要点）：
// 对 GET/POST/PATCH game-channel plugins 契约做 mock/stub，验证：渠道实例详情「功能插件」页签
// 渲染必接/国内海外/可勾选/勾选态/config_status/includedInRuntimeConfig、scope=server 提示、
// 必接未配置引导、locked 禁用、plugin.write 权限置灰，并采集关键状态截图基线（toHaveScreenshot）。

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
    permissions: ["dashboard.read", "channel.read", "plugin.read", "plugin.write"]
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

const INSTANCE = {
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

const DETAIL = {
  ...INSTANCE,
  channelName: "华为(中国)",
  channelType: "store",
  // 故意非 channel_only，避免「渠道登录」页签同时渲染 .panel 干扰截图定位
  loginMode: "account_system",
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

// 必接 / 国内 / 不可勾选 / invalid（未配置完成）
const PLUGIN_REQUIRED = {
  id: 501,
  pluginId: "anti_addiction",
  pluginName: "防沉迷",
  region: "domestic",
  required: true,
  selectable: false,
  locked: false,
  enabled: true,
  configStatus: "invalid",
  includedInRuntimeConfig: false,
  configJson: { callback: "https://example.com/aa" },
  lastCheckAt: null,
  lastCheckMessage: "缺少必填敏感字段或文件字段",
  template: {
    templateVersion: "v1",
    formSchemaJson: [
      { key: "appKey", label: "App Key", component: "password", required: true, order: 1, scope: "server" },
      { key: "callback", label: "回调地址", component: "input", required: true, order: 2, scope: "both" }
    ],
    secretFieldsJson: ["appKey"],
    fileFieldsJson: [],
    validationRulesJson: { callback: { format: "url" } }
  }
};

// 可选 / 海外 / 可勾选 / valid（进入最终配置）/ locked
const PLUGIN_OPTIONAL = {
  id: 502,
  pluginId: "push",
  pluginName: "推送",
  region: "overseas",
  required: false,
  selectable: true,
  locked: true,
  enabled: true,
  configStatus: "valid",
  includedInRuntimeConfig: true,
  configJson: { callback: "https://example.com/push" },
  lastCheckAt: "2026-01-01T00:00:00Z",
  lastCheckMessage: "校验通过",
  template: {
    templateVersion: "v1",
    formSchemaJson: [{ key: "callback", label: "回调地址", component: "input", required: true, order: 1, scope: "both" }],
    secretFieldsJson: [],
    fileFieldsJson: [],
    validationRulesJson: { callback: { format: "url" } }
  }
};

interface SetupOptions {
  permissions?: string[];
  plugins?: unknown[];
}

async function setup(page: Page, options: SetupOptions = {}) {
  const permissions = options.permissions ?? ["channel.read", "plugin.read", "plugin.write"];
  const plugins = options.plugins ?? [PLUGIN_REQUIRED, PLUGIN_OPTIONAL];

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
            loginMode: "account_system",
            paymentMode: "channel_only",
            loginLocked: false,
            paymentLocked: false
          }
        ]
      }
    })
  );
  await page.route(/\/api\/admin\/games\/100001\/market-channels(\?.*)?$/, (route) =>
    json(route, 200, { data: { items: [INSTANCE], page: 1, pageSize: 20, total: 1 } })
  );
  await page.route(/\/api\/admin\/game-channels\/101$/, (route) => json(route, 200, { data: DETAIL }));
  await page.route(/\/api\/admin\/game-channels\/101\/packages$/, (route) => json(route, 200, { data: { items: [] } }));
  // GET 插件列表契约 stub
  await page.route(/\/api\/admin\/game-channels\/101\/plugins$/, (route) =>
    json(route, 200, { data: { items: plugins } })
  );
}

async function openPluginTab(page: Page) {
  await page.goto("/dashboard");
  await page.getByRole("link", { name: "渠道管理" }).click();
  await expect(page.getByText("渠道实例管理")).toBeVisible({ timeout: 60_000 });
  const detailBtn = page.getByRole("button", { name: "详情" }).first();
  await expect(detailBtn).toBeVisible({ timeout: 60_000 });
  await detailBtn.click();
  await expect(page.getByText("渠道实例详情")).toBeVisible({ timeout: 60_000 });
  await page.getByRole("tab", { name: "功能插件" }).click();
}

function pluginPanel(page: Page) {
  return page.locator(".panel", { has: page.getByRole("heading", { name: "功能插件" }) });
}

test("功能插件页签渲染必接/区域/勾选态/状态徽标/最终配置标识+scope 提示+引导补齐", async ({ page }) => {
  await setup(page);
  await openPluginTab(page);
  const panel = pluginPanel(page);

  // 必接未配置引导
  await expect(panel.getByText("必接插件未配置完成", { exact: false })).toBeVisible();
  // 徽标：必接 / 国内 / 海外 / 锁定 / 进入最终配置 / 未进入最终配置
  await expect(panel.getByText("必接", { exact: true }).first()).toBeVisible();
  await expect(panel.getByText("国内", { exact: true }).first()).toBeVisible();
  await expect(panel.getByText("海外", { exact: true }).first()).toBeVisible();
  await expect(panel.getByText("锁定", { exact: true }).first()).toBeVisible();
  await expect(panel.getByText("未进入最终配置").first()).toBeVisible();
  // config_status 徽标
  await expect(panel.getByText("配置无效").first()).toBeVisible();
  // scope=server 不下发客户端提示
  await expect(panel.getByText("仅服务端，不下发客户端").first()).toBeVisible();

  await expect(panel).toHaveScreenshot("feature-plugin-list.png", { maxDiffPixelRatio: 0.02 });
});

test("必接 selectable=false 实例启用开关强制选中且禁用", async ({ page }) => {
  await setup(page);
  await openPluginTab(page);
  const panel = pluginPanel(page);
  // 第一个折叠项（必接 anti_addiction）默认展开；其启用开关禁用且为选中态
  const firstSwitch = panel.locator(".el-switch").first();
  await expect(firstSwitch).toHaveClass(/is-disabled/);
  await expect(firstSwitch).toHaveClass(/is-checked/);
});

test("无 plugin.write 权限时保存按钮置灰", async ({ page }) => {
  await setup(page, { permissions: ["channel.read", "plugin.read"] });
  await openPluginTab(page);
  const panel = pluginPanel(page);
  await expect(panel.getByRole("button", { name: "保存插件配置" }).first()).toBeDisabled();
});
