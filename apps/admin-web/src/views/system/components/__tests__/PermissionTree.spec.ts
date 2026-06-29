import { describe, expect, test } from "vitest";
import { mount } from "@vue/test-utils";
import type { PermissionItem } from "@/api/modules/system";
import PermissionTree from "../PermissionTree.vue";

const permissions: PermissionItem[] = [
  { id: 1, permissionCode: "system.read", permissionName: "系统读取", createdAt: "", updatedAt: "" },
  { id: 2, permissionCode: "role.write", permissionName: "角色写入", createdAt: "", updatedAt: "" },
  { id: 3, permissionCode: "system.write", permissionName: "系统写入", createdAt: "", updatedAt: "" },
  { id: 4, permissionCode: "legacy", permissionName: "无点号", createdAt: "", updatedAt: "" }
];

describe("PermissionTree", () => {
  test("groups permissions by resource prefix and falls back to 其它", () => {
    const wrapper = mount(PermissionTree, {
      props: { permissions, checked: [] }
    });
    const text = wrapper.text();
    // 分组父节点：system / role / 其它
    expect(text).toContain("system");
    expect(text).toContain("role");
    expect(text).toContain("其它");
    // 叶子节点展示「code（name）」
    expect(text).toContain("system.read（系统读取）");
  });

  test("getCheckedIds returns only leaf permission ids", async () => {
    const wrapper = mount(PermissionTree, {
      props: { permissions, checked: [1, 3] }
    });
    await wrapper.vm.$nextTick();
    const ids = (wrapper.vm as unknown as { getCheckedIds: () => number[] }).getCheckedIds();
    // 仅返回叶子 permId，排除分组父节点；默认勾选 1、3
    expect(ids.sort()).toEqual([1, 3]);
  });

  test("reacts to checked prop changes", async () => {
    const wrapper = mount(PermissionTree, {
      props: { permissions, checked: [2] }
    });
    await wrapper.vm.$nextTick();
    await wrapper.setProps({ checked: [1, 2] });
    await wrapper.vm.$nextTick();
    const ids = (wrapper.vm as unknown as { getCheckedIds: () => number[] }).getCheckedIds();
    expect(ids.sort()).toEqual([1, 2]);
  });
});
