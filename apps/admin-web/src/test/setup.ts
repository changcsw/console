import "@testing-library/jest-dom/vitest";
import ElementPlus from "element-plus";
import { config } from "@vue/test-utils";

class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}

Object.defineProperty(globalThis, "ResizeObserver", {
  writable: true,
  value: ResizeObserverMock
});

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false
  })
});

Object.defineProperty(window.HTMLElement.prototype, "scrollTo", {
  writable: true,
  value: () => {}
});

config.global.plugins = [ElementPlus];
