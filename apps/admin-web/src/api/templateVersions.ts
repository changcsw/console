import { request } from "@/api/http";

export type TemplateVersionStatus = "draft" | "published" | "archived";

export interface TemplateVersion {
  version: number;
  status: TemplateVersionStatus;
}

export async function copyPublishedToDraft(templateId: string, sourceVersion: number): Promise<TemplateVersion> {
  return request<TemplateVersion>(`/api/admin/cashier/templates/${templateId}/versions/${sourceVersion}/copy-to-draft`, {
    method: "POST"
  });
}
