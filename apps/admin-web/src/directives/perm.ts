import type { App, Directive, DirectiveBinding } from "vue";
import { usePermissionStore } from "@/stores/permission";

/**
 * v-perm="'role.write'" 或 v-perm="['role.write','admin_user.write']"
 * 无权限时置灰禁用（而非移除），并拦截点击。
 */
function resolve(binding: DirectiveBinding): boolean {
  const store = usePermissionStore();
  const value = binding.value;
  if (!value) {
    return true;
  }
  if (Array.isArray(value)) {
    return store.hasAnyPerm(value);
  }
  return store.hasPerm(String(value));
}

function apply(el: HTMLElement, allowed: boolean) {
  if (allowed) {
    el.removeAttribute("disabled");
    el.classList.remove("perm-disabled");
    el.style.removeProperty("pointer-events");
    return;
  }
  el.setAttribute("disabled", "disabled");
  el.classList.add("perm-disabled");
  el.style.pointerEvents = "none";
  // 同步禁用内部可聚焦控件（Element Plus 按钮渲染为原生 button）
  const focusable = el.querySelectorAll<HTMLElement>("button, input, select, textarea, a");
  focusable.forEach((node) => node.setAttribute("disabled", "disabled"));
}

const permDirective: Directive<HTMLElement, string | string[]> = {
  mounted(el, binding) {
    apply(el, resolve(binding));
  },
  updated(el, binding) {
    apply(el, resolve(binding));
  }
};

export function registerPermDirective(app: App) {
  app.directive("perm", permDirective);
}

export default permDirective;
