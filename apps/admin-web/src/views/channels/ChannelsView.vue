<template>
  <div class="page-shell">
    <PageCard title="渠道管理">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="渠道" name="channels" lazy>
          <PlatformChannelsPanel @view-templates="onViewTemplates" />
        </el-tab-pane>
        <el-tab-pane label="渠道模版" name="templates" lazy>
          <ChannelTemplatesPanel :focus-channel="templateFocus" />
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
const templateFocus = ref<{ channelId: string }>({ channelId: "" });

function onViewTemplates(channelId: string) {
  // 每次点击都生成新对象：即使同一渠道重复点击，模版面板的 watch 也会触发重选
  templateFocus.value = { channelId };
  activeTab.value = "templates";
}
</script>
