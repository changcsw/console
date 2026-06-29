import { request } from "@/api/http";

export type ConfigStatus = "empty" | "invalid" | "valid";
export type FormFieldComponent = "input" | "password" | "textarea" | "number" | "select" | "switch" | "file" | "json";
export type FormFieldScope = "client" | "server" | "both";

export interface FormFieldOption {
  label: string;
  value: string | number | boolean;
}

export interface AccountAuthTemplateField {
  key: string;
  label: string;
  component: FormFieldComponent;
  required?: boolean;
  placeholder?: string;
  order?: number;
  scope?: FormFieldScope;
  options?: FormFieldOption[];
}

export interface AccountAuthTemplate {
  templateVersion: string;
  formSchema: AccountAuthTemplateField[];
  secretFields: string[];
  fileFields: Array<{ key: string; accept?: string[]; maxSizeKB?: number }>;
  validationRules: Record<string, unknown>;
}

export interface AccountAuthTypeItem {
  authTypeId: string;
  authTypeName: string;
  enabled: boolean;
  sort: number;
  template: AccountAuthTemplate;
}

export interface ChannelAccountAuthTypeItem {
  authTypeId: string;
  defaultEnabled: boolean;
  locked: boolean;
}

export interface GameAccountAuthConfigItem {
  authTypeId: string;
  enabled: boolean;
  configJson: Record<string, unknown>;
  configStatus: ConfigStatus;
  lastCheckAt: string | null;
  lastCheckMessage: string;
}

export interface ReplaceGameAccountAuthConfigItem {
  authTypeId: string;
  enabled?: boolean;
  configJson?: Record<string, unknown>;
}

export interface ReplaceGameAccountAuthConfigsRequest {
  items: ReplaceGameAccountAuthConfigItem[];
}

const enc = encodeURIComponent;

// GET /account-auth/types — 认证方式与模板定义（game.read）
export async function listAccountAuthTypes(): Promise<AccountAuthTypeItem[]> {
  const res = await request<{ items: AccountAuthTypeItem[] }>("/api/admin/account-auth/types");
  return res.items ?? [];
}

// GET /channels/{channelId}/account-auth-types — 渠道允许集合（game.read）
export async function listChannelAccountAuthTypes(channelId: string): Promise<ChannelAccountAuthTypeItem[]> {
  const res = await request<{ items: ChannelAccountAuthTypeItem[] }>(
    `/api/admin/channels/${enc(channelId)}/account-auth-types`
  );
  return res.items ?? [];
}

// GET /games/{gameId}/account-auth-configs — 游戏认证配置（game.read）
export async function listGameAccountAuthConfigs(gameId: string): Promise<GameAccountAuthConfigItem[]> {
  const res = await request<{ items: GameAccountAuthConfigItem[] }>(`/api/admin/games/${enc(gameId)}/account-auth-configs`);
  return res.items ?? [];
}

// PUT /games/{gameId}/account-auth-configs — 整体替换（game.write）
export async function replaceGameAccountAuthConfigs(
  gameId: string,
  payload: ReplaceGameAccountAuthConfigsRequest
): Promise<GameAccountAuthConfigItem[]> {
  const res = await request<{ items: GameAccountAuthConfigItem[] }>(`/api/admin/games/${enc(gameId)}/account-auth-configs`, {
    method: "PUT",
    body: JSON.stringify(payload)
  });
  return res.items ?? [];
}
