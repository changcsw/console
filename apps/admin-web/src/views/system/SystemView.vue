<template>
  <div class="page-shell">
    <PageCard title="系统设置" description="后台管理员、角色与权限码维护。读列表需 system.read；写操作按权限码置灰。">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="管理员管理" name="users" lazy>
          <AdminUsersPanel />
        </el-tab-pane>
        <el-tab-pane label="角色管理" name="roles" lazy>
          <RolesPanel />
        </el-tab-pane>
        <el-tab-pane label="权限码" name="permissions" lazy>
          <PermissionsPanel />
        </el-tab-pane>
      </el-tabs>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import PageCard from "@/components/page/PageCard.vue";
import AdminUsersPanel from "./components/AdminUsersPanel.vue";
import RolesPanel from "./components/RolesPanel.vue";
import PermissionsPanel from "./components/PermissionsPanel.vue";

const activeTab = ref<"users" | "roles" | "permissions">("users");
</script>
