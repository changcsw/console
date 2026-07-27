import { defineConfig, devices } from "@playwright/test";

// 专用 e2e 端口（避开本地默认 5173，可能被其它 dev server 占用）。
const PORT = Number(process.env.E2E_PORT ?? 5187);

export default defineConfig({
  testDir: "../../tests/frontend/e2e",
  outputDir: "../../tests/reports/playwright-artifacts",
  snapshotDir: "../../tests/frontend/visual-baseline",
  timeout: 90_000,
  expect: {
    // 本机 fullyParallel 下多个 GameDetailView（含重型 el-tabs/表格）并发渲染会让首帧
    // 断言偶发超过 15s。提到 30s 吸收并发抖动（仍远低于 90s 用例超时）。
    timeout: 30_000,
  },
  fullyParallel: true,
  // 本机验收：默认 workers=CPU 核数在重型 Vue 渲染下会过载导致 flaky（单 worker 100% 稳定）。
  // 收敛到 3 兼顾吞吐与稳定；CI 保留默认（含 retries）。
  workers: process.env.CI ? undefined : 3,
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
    { name: "chromium", use: { ...devices["Desktop Chrome"], channel: "chrome" } },
  ],
  webServer: {
    // e2e 使用生产构建 + preview 静态服务：dev server 的按需编译在 fullyParallel 高并发下
    // 会因首帧编译争抢导致导航超过 15s 而 flaky（单 worker 稳定通过即为佐证）。改为构建后由
    // preview 提供静态产物，页面加载稳定且更快，消除并发抖动。强制独立端口 + strictPort。
    command: `pnpm exec vite build && pnpm exec vite preview --host 127.0.0.1 --port ${PORT} --strictPort`,
    url: `http://127.0.0.1:${PORT}`,
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
