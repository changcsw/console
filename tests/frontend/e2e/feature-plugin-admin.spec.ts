import { expect, test, type Page, type Route } from "@playwright/test";

// 功能插件管理（平台级）UI 用例：左侧菜单「功能插件管理」进入，覆盖插件分类字典、插件主数据、
// 参数模板（四件套）三个页签的列表/新增/编辑/删除成功路径，分类删除 409 行内报错、
// 编辑时 pluginId/region 身份字段禁用、categoryId 清空语义（显式 null），以及
// feature_plugin.read / feature_plugin.write 权限对菜单与写按钮的控制。
// 与 feature-plugin.spec.ts（游戏渠道实例侧的插件绑定配置）不同域，故独立成文件。
// e2e 跑在 vite build + preview 静态产物上，不连真实后端：对 /api/admin/feature-plugin* 契约做 mock/stub。

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
    permissions: ["dashboard.read", "feature_plugin.read", "feature_plugin.write"]
  }
};

const ALL_PERMS = ["dashboard.read", "feature_plugin.read", "feature_plugin.write"];

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

// login 分类下挂有插件（pluginCount>0）：删除会被后端 409 拒绝
const CATEGORY_LOGIN = {
  id: 1,
  categoryCode: "login",
  categoryName: "登录类",
  enabled: true,
  sort: 1,
  pluginCount: 1,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z"
};

const CATEGORY_PUSH = {
  id: 2,
  categoryCode: "push",
  categoryName: "推送类",
  enabled: true,
  sort: 2,
  pluginCount: 0,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z"
};

const PLUGIN_HUAWEI = {
  pluginId: "huawei_push",
  pluginName: "华为推送",
  categoryId: 2,
  categoryCode: "push",
  categoryName: "推送类",
  region: "domestic",
  enabled: true,
  sort: 1,
  templateCount: 2,
  updatedAt: "2026-01-01T00:00:00Z"
};

// 未分类 / 海外 / 已停用 的边界行
const PLUGIN_ADMOB = {
  pluginId: "admob",
  pluginName: "AdMob 广告",
  categoryId: null,
  categoryCode: "",
  categoryName: "",
  region: "overseas",
  enabled: false,
  sort: 2,
  templateCount: 0,
  updatedAt: "2026-01-02T00:00:00Z"
};

const TEMPLATE_V1 = {
  templateId: 1,
  pluginId: "huawei_push",
  templateVersion: "v1",
  formSchemaJson: [
    { key: "appId", label: "App ID", component: "input", required: true, order: 10, scope: "both" },
    { key: "appSecret", label: "App Secret", component: "password", required: true, order: 20, scope: "server" }
  ],
  secretFieldsJson: ["appSecret"],
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

  // 分类字典：全量不分页 / 新建
  await page.route(/\/api\/admin\/feature-plugin-categories(\?.*)?$/, (route) => {
    if (route.request().method() === "POST") {
      return json(route, 201, {
        data: { ...CATEGORY_PUSH, id: 3, categoryCode: "ad", categoryName: "广告类", pluginCount: 0 }
      });
    }
    return json(route, 200, { data: { items: [CATEGORY_LOGIN, CATEGORY_PUSH] } });
  });

  // 单分类：编辑 / 删除
  await page.route(/\/api\/admin\/feature-plugin-categories\/\d+$/, (route) => {
    if (route.request().method() === "DELETE") {
      return json(route, 200, { data: {} });
    }
    return json(route, 200, { data: { ...CATEGORY_LOGIN, categoryName: "登录插件类" } });
  });

  // 插件主数据：分页列表 / 新建
  await page.route(/\/api\/admin\/feature-plugins(\?.*)?$/, (route) => {
    if (route.request().method() === "POST") {
      return json(route, 201, {
        data: { ...PLUGIN_ADMOB, pluginId: "google_fcm", pluginName: "FCM 推送", region: "domestic", enabled: true }
      });
    }
    return json(route, 200, {
      data: { items: [PLUGIN_HUAWEI, PLUGIN_ADMOB], page: 1, pageSize: 20, total: 2 }
    });
  });

  // 插件参数模板：版本列表 / 新建版本（admob 无模板）
  await page.route(/\/api\/admin\/feature-plugins\/[^/]+\/templates(\?.*)?$/, (route) => {
    if (route.request().method() === "POST") {
      return json(route, 201, { data: { ...TEMPLATE_V1, templateId: 3, templateVersion: "v3" } });
    }
    const url = route.request().url();
    if (url.includes("/feature-plugins/admob/")) {
      return json(route, 200, { data: { items: [] } });
    }
    return json(route, 200, { data: { items: [TEMPLATE_V2, TEMPLATE_V1] } });
  });

  // 单插件：编辑 / 删除
  await page.route(/\/api\/admin\/feature-plugins\/[^/]+$/, (route) => {
    if (route.request().method() === "DELETE") {
      return json(route, 200, { data: {} });
    }
    return json(route, 200, { data: { ...PLUGIN_HUAWEI, pluginName: "华为推送服务" } });
  });

  // 单模板版本：整体替换四件套 / 启用
  await page.route(/\/api\/admin\/feature-plugin-templates\/\d+$/, (route) =>
    json(route, 200, { data: { ...TEMPLATE_V1, enabled: false } })
  );
}

// 功能插件管理入口：工作台 → 侧边栏「功能插件管理」。
async function gotoPlugins(page: Page) {
  await page.goto("/dashboard");
  await page.getByRole("link", { name: "功能插件管理" }).click();
  await expect(page.getByRole("tab", { name: "插件分类" })).toBeVisible({ timeout: 60_000 });
}

async function gotoTab(page: Page, tab: "插件分类" | "插件主数据" | "参数模板") {
  await gotoPlugins(page);
  await page.getByRole("tab", { name: tab }).click();
}

test("左侧菜单出现「功能插件管理」入口（MagicStick 图标渲染）并导航进入分类列表", async ({ page }) => {
  await setup(page);
  await page.goto("/dashboard");

  const menuLink = page.getByRole("link", { name: "功能插件管理" });
  await expect(menuLink).toBeVisible();
  // MagicStick 已注册进 routeIconMap：菜单项内渲染出 svg 图标（导入缺失会在构建期失败，未注册会回退 MenuIcon）
  await expect(menuLink.locator(".menu__icon svg")).toBeVisible();

  await menuLink.click();
  await expect(page).toHaveURL(/\/plugins$/);
  await expect(page.getByRole("tab", { name: "插件分类" })).toBeVisible({ timeout: 60_000 });

  // 分类字典列表渲染：编码 / 名称 / 插件数
  await expect(page.getByRole("cell", { name: "login", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "登录类", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "push", exact: true })).toBeVisible();
  await expect(page.getByRole("cell", { name: "推送类", exact: true })).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/feature-plugin-admin-categories.png", fullPage: true });
});

test("三个页签切换正常", async ({ page }) => {
  await setup(page);
  await gotoPlugins(page);
  await expect(page.getByRole("button", { name: "新建分类" })).toBeVisible();

  await page.getByRole("tab", { name: "插件主数据" }).click();
  await expect(page.getByRole("button", { name: "新建插件" })).toBeVisible();
  await expect(page.locator("#pane-plugins").getByRole("cell", { name: "huawei_push", exact: true })).toBeVisible();

  await page.getByRole("tab", { name: "参数模板" }).click();
  // 模板面板挂载后自动选中首个插件（huawei_push）并拉取其模板版本
  await expect(page.locator("#pane-templates").getByRole("cell", { name: "v1", exact: true })).toBeVisible({
    timeout: 60_000
  });

  // 切回分类页签（lazy 页签保活，列表仍在）
  await page.getByRole("tab", { name: "插件分类" }).click();
  await expect(page.getByRole("cell", { name: "login", exact: true })).toBeVisible();
});

test("新建分类提交全字段", async ({ page }) => {
  await setup(page);
  await gotoPlugins(page);

  await page.getByRole("button", { name: "新建分类" }).click();
  const drawer = page.locator(".el-drawer");
  await drawer.getByPlaceholder(/字母开头，如 login/).fill("ad");
  await drawer.getByPlaceholder(/如 登录类/).fill("广告类");

  const postPromise = page.waitForRequest(
    (req) => req.method() === "POST" && req.url().endsWith("/api/admin/feature-plugin-categories")
  );
  await drawer.getByRole("button", { name: "保存" }).click();
  const body = (await postPromise).postData() ?? "";
  expect(body).toContain("\"categoryCode\":\"ad\"");
  expect(body).toContain("\"categoryName\":\"广告类\"");
  expect(body).toContain("\"enabled\":true");
  expect(body).toContain("\"sort\"");
  await expect(page.locator(".el-message").getByText("已创建分类")).toBeVisible();
});

test("编辑分类时编码只读且给出不可改说明，PATCH 只下发可变列", async ({ page }) => {
  await setup(page);
  await gotoPlugins(page);

  await page.getByRole("row", { name: /login/ }).getByRole("button", { name: "编辑" }).click();
  const drawer = page.locator(".el-drawer");
  await expect(drawer.getByText("编辑分类")).toBeVisible();
  await expect(drawer.getByPlaceholder(/字母开头，如 login/)).toBeDisabled();
  await expect(drawer.getByText(/分类编码是分类的引用键/)).toBeVisible();
  await drawer.getByPlaceholder(/如 登录类/).fill("登录插件类");

  const patchPromise = page.waitForRequest(
    (req) => req.method() === "PATCH" && req.url().endsWith("/api/admin/feature-plugin-categories/1")
  );
  await drawer.getByRole("button", { name: "保存" }).click();
  const body = (await patchPromise).postData() ?? "";
  expect(body).toContain("\"categoryName\":\"登录插件类\"");
  expect(body).not.toContain("categoryCode");
  await expect(page.locator(".el-message").getByText("已更新分类")).toBeVisible();
});

test("删除分类成功路径", async ({ page }) => {
  await setup(page);
  await gotoPlugins(page);

  await page.getByRole("row", { name: /push/ }).getByRole("button", { name: "删除" }).click();
  // 与插件删除确认同口径：中文按钮 + 不可恢复说明
  const box = page.locator(".el-message-box");
  await expect(box).toContainText("确认删除分类「推送类」？删除后不可恢复。");
  await expect(box.getByRole("button", { name: "确认删除" })).toBeVisible();
  await expect(box.getByRole("button", { name: "取消" })).toBeVisible();

  const deletePromise = page.waitForRequest(
    (req) => req.method() === "DELETE" && req.url().endsWith("/api/admin/feature-plugin-categories/2")
  );
  await box.getByRole("button", { name: "确认删除" }).click();
  await deletePromise;
  await expect(page.locator(".el-message").getByText("已删除分类")).toBeVisible();
});

test("删除被引用分类返回 409 时展示行内可读错误提示", async ({ page }) => {
  await setup(page);
  // 后注册的路由优先匹配：login 分类（id=1，pluginCount=1）删除被后端 409 拒绝
  await page.route(/\/api\/admin\/feature-plugin-categories\/1$/, (route) => {
    if (route.request().method() === "DELETE") {
      return json(route, 409, { error: { code: "CONFLICT", message: "该分类下仍有插件，无法删除" } });
    }
    return json(route, 200, { data: CATEGORY_LOGIN });
  });
  await gotoPlugins(page);

  await page.getByRole("row", { name: /login/ }).getByRole("button", { name: "删除" }).click();
  await page.locator(".el-message-box").getByRole("button", { name: "确认删除" }).click();

  // 行内报错（role=alert），不用 toast，便于对照当前行；且不出现成功提示
  const alert = page.locator(".panel__error[role=alert]");
  await expect(alert).toBeVisible();
  await expect(alert).toContainText("该分类下仍有插件，无法删除");
  await expect(page.locator(".el-message").getByText("已删除分类")).toBeHidden();
});

test("插件主数据页签渲染列表（含未分类/海外/已停用边界行）", async ({ page }) => {
  await setup(page);
  await gotoTab(page, "插件主数据");

  const pane = page.locator("#pane-plugins");
  await expect(pane.getByRole("cell", { name: "huawei_push", exact: true })).toBeVisible();
  await expect(pane.getByRole("cell", { name: "华为推送", exact: true })).toBeVisible();
  await expect(pane.getByRole("cell", { name: "推送类", exact: true })).toBeVisible();
  await expect(pane.getByRole("cell", { name: "admob", exact: true })).toBeVisible();
  // 未分类 / 国内海外中文标签 / 启用状态
  await expect(pane.getByText("未分类")).toBeVisible();
  await expect(pane.getByText("国内", { exact: true })).toBeVisible();
  await expect(pane.getByText("海外", { exact: true })).toBeVisible();
  await expect(pane.getByText("已启用").first()).toBeVisible();
  await expect(pane.getByText("已停用")).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/feature-plugin-admin-plugins.png", fullPage: true });
});

test("新建插件提交全字段（默认国内，未选分类不下发 categoryId）", async ({ page }) => {
  await setup(page);
  await gotoTab(page, "插件主数据");

  await page.getByRole("button", { name: "新建插件" }).click();
  const drawer = page.locator(".el-drawer");
  await drawer.getByPlaceholder(/如 huawei_push/).fill("google_fcm");
  await drawer.getByPlaceholder(/如 华为推送/).fill("FCM 推送");

  const postPromise = page.waitForRequest(
    (req) => req.method() === "POST" && req.url().endsWith("/api/admin/feature-plugins")
  );
  await drawer.getByRole("button", { name: "保存" }).click();
  const body = (await postPromise).postData() ?? "";
  expect(body).toContain("\"pluginId\":\"google_fcm\"");
  expect(body).toContain("\"pluginName\":\"FCM 推送\"");
  // 未改动时下发的默认值：国内/海外=国内
  expect(body).toContain("\"region\":\"domestic\"");
  // 未选分类：省略该键（后端语义=不设置），而不是 null
  expect(body).not.toContain("categoryId");
  await expect(page.locator(".el-message").getByText("已创建插件")).toBeVisible();
});

test("编辑插件时 pluginId/region 禁用且 PATCH 不下发身份字段", async ({ page }) => {
  await setup(page);
  await gotoTab(page, "插件主数据");

  await page
    .locator("#pane-plugins")
    .getByRole("row", { name: /huawei_push/ })
    .getByRole("button", { name: "编辑" })
    .click();
  const drawer = page.locator(".el-drawer");
  await expect(drawer.getByText("编辑插件")).toBeVisible();
  // 身份字段禁用 + 不可改说明
  await expect(drawer.getByPlaceholder(/如 huawei_push/)).toBeDisabled();
  await expect(drawer.getByText(/插件 ID 是插件实例配置的引用键/)).toBeVisible();
  await expect(drawer.getByText(/国内\/海外属性决定与市场/)).toBeVisible();
  // EP 2.14 的 el-select 禁用态落在内部 combobox input 的 disabled 属性上（根节点不再挂 is-disabled）
  const regionItem = drawer.locator(".el-form-item").filter({ hasText: "国内/海外" });
  await expect(regionItem.getByRole("combobox")).toBeDisabled();

  await drawer.getByPlaceholder(/如 华为推送/).fill("华为推送服务");
  const patchPromise = page.waitForRequest(
    (req) => req.method() === "PATCH" && req.url().endsWith("/api/admin/feature-plugins/huawei_push")
  );
  await drawer.getByRole("button", { name: "保存" }).click();
  const body = (await patchPromise).postData() ?? "";
  expect(body).toContain("\"pluginName\":\"华为推送服务\"");
  expect(body).toContain("\"categoryId\":2");
  expect(body).not.toContain("\"pluginId\"");
  expect(body).not.toContain("\"region\"");
  await expect(page.locator(".el-message").getByText("已更新插件")).toBeVisible();
});

test("编辑插件清空分类后 PATCH 显式下发 categoryId: null", async ({ page }) => {
  await setup(page);
  await gotoTab(page, "插件主数据");

  await page
    .locator("#pane-plugins")
    .getByRole("row", { name: /huawei_push/ })
    .getByRole("button", { name: "编辑" })
    .click();
  const drawer = page.locator(".el-drawer");
  await expect(drawer.getByText("编辑插件")).toBeVisible();

  // el-select clearable：悬停后出现清空图标，点击后 v-model 置为 undefined（真实用户路径）
  const categoryItem = drawer.locator(".el-form-item").filter({ hasText: "插件分类" });
  await categoryItem.locator(".el-select").hover();
  const clearIcon = categoryItem.locator(".el-select__clear");
  await expect(clearIcon).toBeVisible();
  await clearIcon.click();

  const patchPromise = page.waitForRequest(
    (req) => req.method() === "PATCH" && req.url().endsWith("/api/admin/feature-plugins/huawei_push")
  );
  await drawer.getByRole("button", { name: "保存" }).click();
  const body = (await patchPromise).postData() ?? "";
  // 契约语义：categoryId 传 null = 取消归属分类；若丢键则后端按「不修改」处理
  expect(body).toContain("\"categoryId\":null");
  await expect(page.locator(".el-message").getByText("已更新插件")).toBeVisible();
});

test("删除插件成功路径（无模板版本，确认文案不带模板数）", async ({ page }) => {
  await setup(page);
  await gotoTab(page, "插件主数据");

  await page
    .locator("#pane-plugins")
    .getByRole("row", { name: /admob/ })
    .getByRole("button", { name: "删除" })
    .click();
  // templateCount=0：文案只说不可恢复，不提模板版本
  const box = page.locator(".el-message-box");
  await expect(box).toContainText("确认删除插件「AdMob 广告」？删除后不可恢复。");
  await expect(box).not.toContainText("参数模板");

  const deletePromise = page.waitForRequest(
    (req) => req.method() === "DELETE" && req.url().endsWith("/api/admin/feature-plugins/admob")
  );
  await box.getByRole("button", { name: "确认删除" }).click();
  await deletePromise;
  await expect(page.locator(".el-message").getByText("已删除插件")).toBeVisible();
});

test("删除有模板的插件：确认弹窗说清级联删除的模板版本数，确认后删除并刷新列表", async ({ page }) => {
  await setup(page);
  // 列表请求计数：删除成功后面板应重新拉取列表
  let listCalls = 0;
  await page.route(/\/api\/admin\/feature-plugins(\?.*)?$/, (route) => {
    if (route.request().method() === "GET") {
      listCalls += 1;
    }
    return json(route, 200, {
      data: { items: [PLUGIN_HUAWEI, PLUGIN_ADMOB], page: 1, pageSize: 20, total: 2 }
    });
  });
  await gotoTab(page, "插件主数据");
  const listCallsBefore = listCalls;

  await page
    .locator("#pane-plugins")
    .getByRole("row", { name: /huawei_push/ })
    .getByRole("button", { name: "删除" })
    .click();
  // huawei_push 带 2 个模板版本：确认文案须点明级联删除数量与不可恢复
  const box = page.locator(".el-message-box");
  await expect(box).toContainText("确认删除插件「华为推送」？将同时删除其 2 个参数模板版本，删除后不可恢复。");

  const deletePromise = page.waitForRequest(
    (req) => req.method() === "DELETE" && req.url().endsWith("/api/admin/feature-plugins/huawei_push")
  );
  await box.getByRole("button", { name: "确认删除" }).click();
  await deletePromise;
  await expect(page.locator(".el-message").getByText("已删除插件")).toBeVisible();
  // 删除成功后列表重新拉取
  await expect.poll(() => listCalls).toBe(listCallsBefore + 1);
});

test("删除被渠道引用的插件返回 409 时行内提示且不弹全局 toast", async ({ page }) => {
  await setup(page);
  // 后注册的路由优先匹配：huawei_push 删除被后端 409 拒绝（渠道绑定引用）
  await page.route(/\/api\/admin\/feature-plugins\/huawei_push$/, (route) => {
    if (route.request().method() === "DELETE") {
      return json(route, 409, {
        error: { code: "CONFLICT", message: "该插件仍有关联数据（渠道绑定 2 条），请先删除关联数据" }
      });
    }
    return json(route, 200, { data: PLUGIN_HUAWEI });
  });
  await gotoTab(page, "插件主数据");

  await page
    .locator("#pane-plugins")
    .getByRole("row", { name: /huawei_push/ })
    .getByRole("button", { name: "删除" })
    .click();
  await page.locator(".el-message-box").getByRole("button", { name: "确认删除" }).click();

  // 409 行内提示（role=alert），不弹全局 toast，也不出现成功提示
  const alert = page.locator("#pane-plugins .panel__error[role=alert]");
  await expect(alert).toBeVisible();
  await expect(alert).toContainText("该插件仍有关联数据（渠道绑定 2 条），请先删除关联数据");
  await expect(page.locator(".el-message")).toHaveCount(0);
  // 行数据保持（未刷新删除）
  await expect(page.locator("#pane-plugins").getByRole("cell", { name: "huawei_push", exact: true })).toBeVisible();
});

test("新建插件填非法插件 ID 时被即时校验拦截，不发 POST", async ({ page }) => {
  await setup(page);
  // 后注册的路由优先匹配：统计 POST 是否发出
  let postCalls = 0;
  await page.route(/\/api\/admin\/feature-plugins(\?.*)?$/, (route) => {
    if (route.request().method() === "POST") {
      postCalls += 1;
      return json(route, 201, { data: PLUGIN_ADMOB });
    }
    return json(route, 200, {
      data: { items: [PLUGIN_HUAWEI, PLUGIN_ADMOB], page: 1, pageSize: 20, total: 2 }
    });
  });
  await gotoTab(page, "插件主数据");

  await page.getByRole("button", { name: "新建插件" }).click();
  const drawer = page.locator(".el-drawer");
  await drawer.getByPlaceholder(/如 huawei_push/).fill("1abc");
  await drawer.getByPlaceholder(/如 华为推送/).fill("合法名称");
  await drawer.getByRole("button", { name: "保存" }).click();

  // 行内校验错误出现（插件 ID 只能小写字母/数字/下划线且字母开头），抽屉不关。
  // 注意：分类页签的抽屉外壳常驻 DOM，直接断言 .el-drawer 会撞 strict mode，须按标题收窄
  await expect(drawer.locator(".el-form-item__error").first()).toBeVisible();
  await expect(page.locator(".el-drawer").filter({ hasText: "新建插件" })).toBeVisible();
  // 校验拦截：POST 未发出
  expect(postCalls).toBe(0);
});

test("插件行「参数模板」直达模板页签并渲染版本列表与生效标记", async ({ page }) => {
  await setup(page);
  await gotoTab(page, "插件主数据");

  await page
    .locator("#pane-plugins")
    .getByRole("row", { name: /huawei_push/ })
    .getByRole("button", { name: "参数模板" })
    .click();

  const pane = page.locator("#pane-templates");
  // 插件选择器定位到 huawei_push 并自动拉取其模板版本
  await expect(pane.locator(".filter-plugin")).toContainText("华为推送", { timeout: 60_000 });
  await expect(pane.getByRole("cell", { name: "v1", exact: true })).toBeVisible();
  await expect(pane.getByRole("cell", { name: "v2", exact: true })).toBeVisible();
  // v1 启用且生效；v2 停用且未生效
  await expect(pane.getByText("生效中")).toBeVisible();
  await expect(pane.getByText("未生效")).toBeVisible();
  await expect(pane.getByText("已停用")).toBeVisible();
  // 字段数 / 敏感字段计数
  await expect(pane.getByRole("cell", { name: "2", exact: true }).first()).toBeVisible();

  await page.screenshot({ path: "../../tests/frontend/screenshots/feature-plugin-admin-templates.png", fullPage: true });
});

test("新建模板版本提交四件套", async ({ page }) => {
  await setup(page);
  await gotoTab(page, "参数模板");

  const pane = page.locator("#pane-templates");
  await expect(pane.getByRole("cell", { name: "v1", exact: true })).toBeVisible({ timeout: 60_000 });

  await pane.getByRole("button", { name: "新建模板版本" }).click();
  const drawer = page.locator(".el-drawer");
  await drawer.getByPlaceholder(/如 v1 或 2026.01/).fill("v3");
  // 四件套编辑器：填第一行表单字段
  await drawer.getByPlaceholder(/^key（/).first().fill("appId");
  await drawer.getByPlaceholder("标签").first().fill("App ID");

  const postPromise = page.waitForRequest(
    (req) => req.method() === "POST" && req.url().includes("/feature-plugins/huawei_push/templates")
  );
  await drawer.getByRole("button", { name: "保存" }).click();
  const body = (await postPromise).postData() ?? "";
  expect(body).toContain("\"templateVersion\":\"v3\"");
  expect(body).toContain("appId");
  expect(body).toContain("\"formSchemaJson\"");
  expect(body).toContain("\"secretFieldsJson\"");
  expect(body).toContain("\"fileFieldsJson\"");
  expect(body).toContain("\"validationRulesJson\"");
  await expect(page.locator(".el-message").getByText("已创建模板版本")).toBeVisible();
});

test("仅有 feature_plugin.read 时三个页签的写操作按钮置灰", async ({ page }) => {
  await setup(page, { permissions: ["dashboard.read", "feature_plugin.read"] });
  await gotoPlugins(page);
  await expect(page.getByRole("button", { name: "新建分类" })).toBeDisabled();

  await page.getByRole("tab", { name: "插件主数据" }).click();
  await expect(page.getByRole("button", { name: "新建插件" })).toBeDisabled();

  await page.getByRole("tab", { name: "参数模板" }).click();
  await expect(page.locator("#pane-templates").getByRole("button", { name: "新建模板版本" })).toBeDisabled();
});

test("无 feature_plugin.read 权限时左侧菜单不出现「功能插件管理」入口", async ({ page }) => {
  await setup(page, { permissions: ["dashboard.read"] });
  await page.goto("/dashboard");

  await expect(page.locator("nav.menu")).toBeVisible();
  // visibleRoutes 按 perm 过滤，入口不渲染
  await expect(page.getByRole("link", { name: "功能插件管理" })).toHaveCount(0);
});
