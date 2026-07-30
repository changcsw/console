<template>
  <div class="page-shell">
    <PageCard
      title="渠道管理"
      description="系统管理员维护的平台级渠道主数据与渠道模版，与具体游戏无关，且跨运行环境共享。此处新增渠道并维护其登录 / IAP 模版后，游戏在自己的渠道实例上引用模版填写参数（游戏详情页「渠道」页签）。"
    >
      <el-tabs v-model="activeTab">
        <el-tab-pane label="渠道" name="channels" lazy>
          <PlatformChannelsPanel />
        </el-tab-pane>
        <el-tab-pane label="渠道模版" name="templates" lazy>
          <ChannelTemplatesPanel />
        </el-tab-pane>
      </el-tabs>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import PageCard from "@/components/page/PageCard.vue";
import PlatformChannelsPanel from "./components/platform/PlatformChannelsPanel.vue";
import ChannelTemplatesPanel from "./components/platform/ChannelTemplatesPanel.vue";

const activeTab = ref<"channels" | "templates">("channels");
</script>
