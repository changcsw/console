import { defineConfig, devices } from "@playwright/test";

const PORT = 5173;

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
    command: "pnpm dev",
    url: `http://127.0.0.1:${PORT}`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
