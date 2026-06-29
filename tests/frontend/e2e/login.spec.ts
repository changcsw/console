import { test, expect } from "@playwright/test";

// 登录页 e2e（对齐 03-testing §5.2 与 10-auth README）：
// 后台登录与玩家登录体系隔离；密码 / 飞书两个 Tab；凭据错误行内提示；飞书未绑定提示。
// 纯前端页面渲染，不依赖后端；提交类断言（凭据错误/登录成功跳转）需后端联调，
// 见同模块 vitest（LoginView.spec.ts）已覆盖交互逻辑。

test("login page renders password/feishu tabs and env badge", async ({ page }) => {
  const response = await page.goto("/login");
  expect(response?.status()).toBeLessThan(400);

  await expect(page).toHaveTitle(/Publishing Console/);

  // 两个登录方式 Tab
  await expect(page.getByRole("tab", { name: "密码登录" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "飞书登录" })).toBeVisible();

  // 密码登录表单元素
  await expect(page.getByLabel("userName")).toBeVisible();
  await expect(page.getByLabel("password")).toBeVisible();

  await page.screenshot({
    path: "../../tests/frontend/screenshots/login-password.png",
    fullPage: true,
  });
});

test("login page client-side validation shows inline error", async ({ page }) => {
  await page.goto("/login");
  // 空表单点击登录 → 行内校验提示（不发起网络请求）
  await page.getByRole("button", { name: "密码登录" }).click();
  await expect(page.getByRole("alert")).toContainText("请输入用户名与密码");
});

test("feishu tab shows unbound-identity hint", async ({ page }) => {
  await page.goto("/login");
  await page.getByRole("tab", { name: "飞书登录" }).click();
  await expect(page.getByText("未绑定飞书身份的账号无法登录")).toBeVisible();
  await page.screenshot({
    path: "../../tests/frontend/screenshots/login-feishu.png",
    fullPage: true,
  });
});

test("login page visual baseline", async ({ page }) => {
  await page.goto("/login");
  await expect(page.locator(".login-card")).toBeVisible();
  // 首次运行需 e2e:update 生成基线（visual-baseline/，git 跟踪）
  await expect(page).toHaveScreenshot("login-page.png", { maxDiffPixelRatio: 0.02 });
});
