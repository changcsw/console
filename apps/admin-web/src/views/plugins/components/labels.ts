import type { PluginRegion } from "@/api/modules/featurePlugins";

// 功能插件管理页的枚举中文标签。枚举值本身是全局事实源（见 @/api/modules/channels），
// 这里只负责展示层文案。

const PLUGIN_REGION_LABELS: Record<PluginRegion, string> = {
  domestic: "国内",
  overseas: "海外"
};

export function pluginRegionLabel(value: string): string {
  return PLUGIN_REGION_LABELS[value as PluginRegion] ?? value;
}
