import { beforeEach, describe, expect, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { flushPromises, mount } from "@vue/test-utils";

// 避免加载真实路由图（@/api/http 默认导入 @/router）
vi.mock("@/router", () => ({
  default: { currentRoute: { value: { path: "/login", fullPath: "/login" } }, push: vi.fn() }
}));

const pushMock = vi.fn();
vi.mock("vue-router", () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ push: pushMock })
}));

const loginApi = vi.fn();
const feishuApi = vi.fn();
vi.mock("@/api/modules/auth", () => ({
  login: (...args: unknown[]) => loginApi(...args),
  feishuCallback: (...args: unknown[]) => feishuApi(...args),
  refreshToken: vi.fn(),
  logout: vi.fn(),
  getMe: vi.fn()
}));

import { ApiError } from "@/api/http";
import LoginView from "@/views/login/LoginView.vue";

function mountView() {
  return mount(LoginView, {
    global: {
      stubs: { EnvironmentBadge: true }
    }
  });
}

describe("LoginView", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    localStorage.removeItem("admin-auth");
    vi.clearAllMocks();
  });

  test("shows inline validation when fields empty", async () => {
    const wrapper = mountView();
    await wrapper.get(".login-submit").trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("请输入用户名与密码");
    expect(loginApi).not.toHaveBeenCalled();
  });

  test("shows inline credential error on UNAUTHENTICATED", async () => {
    loginApi.mockRejectedValue(new ApiError(401, "UNAUTHENTICATED", "用户名或密码错误"));
    const wrapper = mountView();
    await wrapper.get('input[aria-label="userName"]').setValue("admin");
    await wrapper.get('input[aria-label="password"]').setValue("wrong-pass");
    await wrapper.get(".login-submit").trigger("click");
    await flushPromises();

    expect(loginApi).toHaveBeenCalledWith({ userName: "admin", password: "wrong-pass" });
    expect(wrapper.find(".login-error").exists()).toBe(true);
    expect(wrapper.text()).toContain("用户名或密码错误");
    expect(pushMock).not.toHaveBeenCalled();
  });

  test("redirects to dashboard on successful login", async () => {
    loginApi.mockResolvedValue({
      accessToken: "acc",
      refreshToken: "ref",
      expiresAt: new Date(Date.now() + 1_800_000).toISOString(),
      user: { userId: 1, userName: "admin", displayName: "Admin", roles: ["super_admin"], permissions: [] }
    });
    const wrapper = mountView();
    await wrapper.get('input[aria-label="userName"]').setValue("admin");
    await wrapper.get('input[aria-label="password"]').setValue("Admin@12345");
    await wrapper.get(".login-submit").trigger("click");
    await flushPromises();

    expect(pushMock).toHaveBeenCalledWith("/dashboard");
    expect(wrapper.find(".login-error").exists()).toBe(false);
  });
});
