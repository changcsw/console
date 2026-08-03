import { request } from "@/api/http";
import type { Paginated, PluginRegion } from "@/api/modules/channels";
import type {
  TemplateFieldDef,
  TemplateFileFieldDef,
  TemplateRuleDef
} from "@/api/modules/platformChannels";

// 功能插件管理：系统管理员维护的插件分类字典、插件主数据与插件参数模板（四件套）。
// 与具体游戏/渠道无关；游戏渠道实例侧的插件绑定配置见 @/api/modules/channels。

export type { PluginRegion };

/** 插件国内/海外属性下拉顺序固定：国内 → 海外 */
export const PLUGIN_REGION_OPTIONS: PluginRegion[] = ["domestic", "overseas"];

export interface FeaturePluginCategory {
  id: number;
  categoryCode: string;
  categoryName: string;
  enabled: boolean;
  sort: number;
  /** 该分类下的插件数；>0 时删除会被后端 409 拒绝 */
  pluginCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface FeaturePlugin {
  pluginId: string;
  pluginName: string;
  /** 未分类时为 null */
  categoryId: number | null;
  categoryCode: string;
  categoryName: string;
  region: PluginRegion;
  enabled: boolean;
  sort: number;
  templateCount: number;
  updatedAt: string;
}

export interface FeaturePluginTemplate {
  templateId: number;
  pluginId: string;
  templateVersion: string;
  formSchemaJson: TemplateFieldDef[];
  secretFieldsJson: string[];
  fileFieldsJson: TemplateFileFieldDef[];
  validationRulesJson: Record<string, TemplateRuleDef>;
  enabled: boolean;
  /** 是否为当前生效版本：同插件中 enabled 的最新 templateVersion */
  effective: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CreateFeaturePluginCategoryRequest {
  categoryCode: string;
  categoryName: string;
  /** 默认 true */
  enabled?: boolean;
  /** 默认 0 */
  sort?: number;
}

/** categoryCode 创建后不可改，故不在补丁内 */
export interface UpdateFeaturePluginCategoryRequest {
  categoryName?: string;
  enabled?: boolean;
  sort?: number;
}

export interface ListFeaturePluginsQuery {
  /** 模糊匹配 pluginId / pluginName */
  keyword?: string;
  categoryId?: number;
  region?: PluginRegion;
  /** 不限 */
  enabled?: boolean;
  page?: number;
  pageSize?: number;
}

export interface CreateFeaturePluginRequest {
  pluginId: string;
  pluginName: string;
  categoryId?: number;
  region: PluginRegion;
  /** 默认 true */
  enabled?: boolean;
  /** 默认 0 */
  sort?: number;
}

/** pluginId / region 创建后不可改，故不在补丁内；categoryId 传 null 表示清除分类 */
export interface UpdateFeaturePluginRequest {
  pluginName?: string;
  categoryId?: number | null;
  enabled?: boolean;
  sort?: number;
}

export interface CreateFeaturePluginTemplateRequest {
  templateVersion: string;
  formSchemaJson: TemplateFieldDef[];
  secretFieldsJson: string[];
  fileFieldsJson: TemplateFileFieldDef[];
  validationRulesJson: Record<string, TemplateRuleDef>;
  /** 默认 true */
  enabled?: boolean;
}

/** 四件套为整体替换语义：字段省略表示该件不改；templateVersion 与所属插件不可改 */
export interface UpdateFeaturePluginTemplateRequest {
  formSchemaJson?: TemplateFieldDef[];
  secretFieldsJson?: string[];
  fileFieldsJson?: TemplateFileFieldDef[];
  validationRulesJson?: Record<string, TemplateRuleDef>;
  enabled?: boolean;
}

function buildQuery(params: Record<string, unknown>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      search.append(key, String(value));
    }
  }
  const query = search.toString();
  return query ? `?${query}` : "";
}

const enc = encodeURIComponent;

// GET /feature-plugin-categories — 分类字典全量不分页，按 sort 升序（feature_plugin.read）
export async function listFeaturePluginCategories(
  params: { enabled?: boolean } = {}
): Promise<FeaturePluginCategory[]> {
  const res = await request<{ items: FeaturePluginCategory[] }>(
    `/api/admin/feature-plugin-categories${buildQuery({ ...params })}`
  );
  return res.items ?? [];
}

// POST /feature-plugin-categories — 新建分类（feature_plugin.write）
export function createFeaturePluginCategory(
  payload: CreateFeaturePluginCategoryRequest
): Promise<FeaturePluginCategory> {
  return request<FeaturePluginCategory>("/api/admin/feature-plugin-categories", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

// PATCH /feature-plugin-categories/{id} — 改名/启用/排序（feature_plugin.write）
export function updateFeaturePluginCategory(
  id: number,
  payload: UpdateFeaturePluginCategoryRequest
): Promise<FeaturePluginCategory> {
  return request<FeaturePluginCategory>(`/api/admin/feature-plugin-categories/${id}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

// DELETE /feature-plugin-categories/{id} — 删除分类；分类下仍有插件时后端返回 409（feature_plugin.write）
export function deleteFeaturePluginCategory(id: number): Promise<void> {
  return request<void>(`/api/admin/feature-plugin-categories/${id}`, { method: "DELETE" });
}

// GET /feature-plugins — 插件主数据分页（feature_plugin.read）
export function listFeaturePlugins(query: ListFeaturePluginsQuery = {}): Promise<Paginated<FeaturePlugin>> {
  return request<Paginated<FeaturePlugin>>(`/api/admin/feature-plugins${buildQuery({ ...query })}`);
}

// POST /feature-plugins — 新建插件（feature_plugin.write）
export function createFeaturePlugin(payload: CreateFeaturePluginRequest): Promise<FeaturePlugin> {
  return request<FeaturePlugin>("/api/admin/feature-plugins", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

// PATCH /feature-plugins/{pluginId} — 改名/分类/启用/排序（feature_plugin.write）
export function updateFeaturePlugin(
  pluginId: string,
  payload: UpdateFeaturePluginRequest
): Promise<FeaturePlugin> {
  return request<FeaturePlugin>(`/api/admin/feature-plugins/${enc(pluginId)}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

// DELETE /feature-plugins/{pluginId} — 删除插件（feature_plugin.write）
export function deleteFeaturePlugin(pluginId: string): Promise<void> {
  return request<void>(`/api/admin/feature-plugins/${enc(pluginId)}`, { method: "DELETE" });
}

// GET /feature-plugins/{pluginId}/templates — 参数模板版本列表（feature_plugin.read）
export async function listFeaturePluginTemplates(pluginId: string): Promise<FeaturePluginTemplate[]> {
  const res = await request<{ items: FeaturePluginTemplate[] }>(
    `/api/admin/feature-plugins/${enc(pluginId)}/templates`
  );
  return res.items ?? [];
}

// POST /feature-plugins/{pluginId}/templates — 新建模板版本（feature_plugin.write）
export function createFeaturePluginTemplate(
  pluginId: string,
  payload: CreateFeaturePluginTemplateRequest
): Promise<FeaturePluginTemplate> {
  return request<FeaturePluginTemplate>(`/api/admin/feature-plugins/${enc(pluginId)}/templates`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

// PATCH /feature-plugin-templates/{templateId} — 整体替换四件套/启用（feature_plugin.write）
export function updateFeaturePluginTemplate(
  templateId: number,
  payload: UpdateFeaturePluginTemplateRequest
): Promise<FeaturePluginTemplate> {
  return request<FeaturePluginTemplate>(`/api/admin/feature-plugin-templates/${templateId}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}
