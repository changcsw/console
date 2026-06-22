import { test, expect } from "@playwright/test";

test("app shell loads and renders", async ({ page }) => {
  const response = await page.goto("/");
  expect(response?.status()).toBeLessThan(400);

  // 标题校验同时兜底：避免误连到其它项目的 dev server（曾占用 5173）。
  // 路由会追加页面名（如「工作台 - Publishing Console」），故用子串匹配。
  await expect(page).toHaveTitle(/Publishing Console/);

  // 应用挂载点应出现内容（Vue 挂在 #app）
  await expect(page.locator("#app")).toBeVisible();

  // 采集真实页面截图到统一产物目录（git 不跟踪正本）
  await page.screenshot({
    path: "../../tests/frontend/screenshots/app-shell.png",
    fullPage: true,
  });
});

test("app shell visual baseline", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator("#app")).toBeVisible();
  // 首次运行需 e2e:update 生成基线（visual-baseline/，git 跟踪）
  await expect(page).toHaveScreenshot("app-shell.png", { maxDiffPixelRatio: 0.02 });
});
