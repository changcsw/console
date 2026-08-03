<template>
  <div class="page-shell">
    <PageCard title="功能插件管理">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="插件分类" name="categories" lazy>
          <PluginCategoriesPanel />
        </el-tab-pane>
        <el-tab-pane label="插件主数据" name="plugins" lazy>
          <FeaturePluginsPanel @view-templates="onViewTemplates" />
        </el-tab-pane>
        <el-tab-pane label="参数模板" name="templates" lazy>
          <FeaturePluginTemplatesPanel :focus-plugin="templateFocus" />
        </el-tab-pane>
      </el-tabs>
    </PageCard>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import PageCard from "@/components/page/PageCard.vue";
import PluginCategoriesPanel from "./components/PluginCategoriesPanel.vue";
import FeaturePluginsPanel from "./components/FeaturePluginsPanel.vue";
import FeaturePluginTemplatesPanel from "./components/FeaturePluginTemplatesPanel.vue";

const activeTab = ref<"categories" | "plugins" | "templates">("categories");
const templateFocus = ref<{ pluginId: string }>({ pluginId: "" });

function onViewTemplates(pluginId: string) {
  // 每次点击都生成新对象：即使同一插件重复点击，模版面板的 watch 也会触发重选
  templateFocus.value = { pluginId };
  activeTab.value = "templates";
}
</script>
