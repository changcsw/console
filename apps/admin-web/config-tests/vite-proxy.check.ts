import { describe, expect, it } from "vitest";
import config from "../vite.config";

describe("vite dev server", () => {
  it("proxies api requests to the admin api", () => {
    const proxy = (config.server?.proxy ?? {}) as Record<string, { target?: string; changeOrigin?: boolean }>;

    expect(proxy["/api"]).toMatchObject({
      target: "http://127.0.0.1:18080",
      changeOrigin: true
    });
  });
});
