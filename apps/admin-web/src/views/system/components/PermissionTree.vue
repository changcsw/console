<template>
  <el-tree
    ref="treeRef"
    :data="treeData"
    show-checkbox
    node-key="key"
    :default-checked-keys="defaultCheckedKeys"
    :default-expand-all="true"
    :props="{ label: 'label', children: 'children' }"
  />
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { ElTree } from "element-plus";
import type { PermissionItem } from "@/api/modules/system";

interface TreeNode {
  key: string;
  label: string;
  permId?: number;
  children?: TreeNode[];
}

const props = defineProps<{
  permissions: PermissionItem[];
  checked: number[];
}>();

const treeRef = ref<InstanceType<typeof ElTree>>();

const treeData = computed<TreeNode[]>(() => {
  const groups = new Map<string, TreeNode[]>();
  for (const perm of props.permissions) {
    const resource = perm.permissionCode.includes(".") ? perm.permissionCode.split(".")[0] : "其它";
    const leaf: TreeNode = {
      key: `perm-${perm.id}`,
      label: `${perm.permissionCode}（${perm.permissionName}）`,
      permId: perm.id
    };
    const list = groups.get(resource) ?? [];
    list.push(leaf);
    groups.set(resource, list);
  }
  return Array.from(groups.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([resource, children]) => ({
      key: `group-${resource}`,
      label: resource,
      children
    }));
});

const defaultCheckedKeys = computed(() => props.checked.map((id) => `perm-${id}`));

watch(
  () => props.checked,
  (next) => {
    treeRef.value?.setCheckedKeys(next.map((id) => `perm-${id}`));
  }
);

function getCheckedIds(): number[] {
  const tree = treeRef.value;
  if (!tree) {
    return [];
  }
  // leaf-only：排除分组父节点
  const keys = tree.getCheckedNodes(true) as TreeNode[];
  return keys.filter((node) => node.permId != null).map((node) => node.permId as number);
}

defineExpose({ getCheckedIds });
</script>
