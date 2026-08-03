<template>
  <div class="page-shell">
    <PageCard title="功能插件管理">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="插件分类" name="categories" lazy>
          <PluginCategoriesPanel />
        </el-tab-pane>
        <el-tab-pane label="插件主数据" name="plugins" lazy>
          <FeaturePluginsPanel @view-templates="onViewTemplates" @plugin-deleted="onPluginDeleted" />
        </el-tab-pane>
        <el-tab-pane label="参数模板" name="templates" lazy>
          <FeaturePluginTemplatesPanel :key="templatesEpoch" :focus-plugin="templateFocus" />
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
// 模版面板的插件下拉与版本列表是本地缓存：插件被级联删除后递增 epoch 重建面板，避免残留已删插件
const templatesEpoch = ref(0);

function onViewTemplates(pluginId: string) {
  // 每次点击都生成新对象：即使同一插件重复点击，模版面板的 watch 也会触发重选
  templateFocus.value = { pluginId };
  activeTab.value = "templates";
}

function onPluginDeleted(pluginId: string) {
  if (templateFocus.value.pluginId === pluginId) {
    templateFocus.value = { pluginId: "" };
  }
  templatesEpoch.value += 1;
}
</script>
