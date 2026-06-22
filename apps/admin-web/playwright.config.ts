import { defineConfig, devices } from "@playwright/test";

// 专用 e2e 端口（避开本地默认 5173，可能被其它 dev server 占用）。
const PORT = Number(process.env.E2E_PORT ?? 5187);

export default defineConfig({
  testDir: "../../tests/frontend/e2e",
  outputDir: "../../tests/reports/playwright-artifacts",
  snapshotDir: "../../tests/frontend/visual-baseline",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [
    ["list"],
    ["html", { outputFolder: "../../tests/reports/playwright-html", open: "never" }],
    ["json", { outputFile: "../../tests/reports/playwright-results.json" }],
  ],
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: {
    // 强制独立端口 + strictPort，避免复用到其它项目的 5173 dev server。
    command: `pnpm exec vite --port ${PORT} --strictPort`,
    url: `http://127.0.0.1:${PORT}`,
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
