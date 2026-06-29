import { beforeEach, describe, expect, test } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { defineComponent } from "vue";
import { mount } from "@vue/test-utils";
import permDirective from "@/directives/perm";
import { usePermissionStore } from "@/stores/permission";

describe("v-perm directive", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  test("disables host and inner focusables when permission missing", () => {
    usePermissionStore().setFromUser({ roles: [], permissions: [] });
    const Host = defineComponent({
      directives: { perm: permDirective },
      template: `<span class="host" v-perm="'role.write'"><button class="act">go</button></span>`
    });
    const wrapper = mount(Host);
    const host = wrapper.find(".host");
    expect(host.attributes("disabled")).toBe("disabled");
    expect(host.classes()).toContain("perm-disabled");
    expect(wrapper.find(".act").attributes("disabled")).toBe("disabled");
  });

  test("enables element when permission granted", () => {
    usePermissionStore().setFromUser({ roles: [], permissions: ["role.write"] });
    const Host = defineComponent({
      directives: { perm: permDirective },
      template: `<span class="host" v-perm="'role.write'"><button class="act">go</button></span>`
    });
    const wrapper = mount(Host);
    expect(wrapper.find(".host").attributes("disabled")).toBeUndefined();
    expect(wrapper.find(".host").classes()).not.toContain("perm-disabled");
  });

  test("super_admin keeps element enabled regardless of code", () => {
    usePermissionStore().setFromUser({ roles: ["super_admin"], permissions: [] });
    const Host = defineComponent({
      directives: { perm: permDirective },
      template: `<span class="host" v-perm="['permission.write','role.write']"><button class="act">go</button></span>`
    });
    const wrapper = mount(Host);
    expect(wrapper.find(".host").attributes("disabled")).toBeUndefined();
  });

  test("array form allows when any code granted", () => {
    usePermissionStore().setFromUser({ roles: [], permissions: ["admin_user.write"] });
    const Host = defineComponent({
      directives: { perm: permDirective },
      template: `<span class="host" v-perm="['role.write','admin_user.write']"><button class="act">go</button></span>`
    });
    const wrapper = mount(Host);
    expect(wrapper.find(".host").attributes("disabled")).toBeUndefined();
  });
});
