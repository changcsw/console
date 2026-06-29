import { request } from "@/api/http";

export type AdminUserStatus = "active" | "disabled";

export interface Paginated<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}

export interface RoleRef {
  id: number;
  roleCode: string;
  roleName: string;
}

export interface AdminUserIdentity {
  identityType: string;
  identityKey: string;
}

export interface AdminUserListItem {
  id: number;
  userName: string;
  displayName: string;
  email: string;
  status: AdminUserStatus;
  roles: RoleRef[];
  createdAt: string;
  updatedAt: string;
}

export interface AdminUserDetail {
  id: number;
  userName: string;
  displayName: string;
  email: string;
  status: AdminUserStatus;
  roles: RoleRef[];
  identities: AdminUserIdentity[];
  permissions: string[];
  createdAt: string;
  updatedAt: string;
}

export interface RoleListItem {
  id: number;
  roleCode: string;
  roleName: string;
  permissionCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface PermissionItem {
  id: number;
  permissionCode: string;
  permissionName: string;
  createdAt: string;
  updatedAt: string;
}

export interface RoleDetail {
  id: number;
  roleCode: string;
  roleName: string;
  permissions: PermissionItem[];
  createdAt: string;
  updatedAt: string;
}

export interface ListAdminUsersQuery {
  page?: number;
  pageSize?: number;
  sort?: string;
  keyword?: string;
  status?: AdminUserStatus;
}

export interface CreateAdminUserPayload {
  userName: string;
  displayName: string;
  email?: string;
  status?: AdminUserStatus;
  password?: string;
  roleIds?: number[];
  feishuKey?: string;
}

export interface UpdateAdminUserPayload {
  displayName?: string;
  email?: string;
  status?: AdminUserStatus;
}

export interface CreateRolePayload {
  roleCode: string;
  roleName: string;
  permissionIds?: number[];
}

export interface CreatePermissionPayload {
  permissionCode: string;
  permissionName: string;
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

// ---- 管理员 ----

export function listAdminUsers(query: ListAdminUsersQuery = {}): Promise<Paginated<AdminUserListItem>> {
  return request<Paginated<AdminUserListItem>>(`/api/admin/system/admin-users${buildQuery({ ...query })}`);
}

export function getAdminUser(id: number): Promise<AdminUserDetail> {
  return request<AdminUserDetail>(`/api/admin/system/admin-users/${id}`);
}

export function createAdminUser(payload: CreateAdminUserPayload): Promise<AdminUserDetail> {
  return request<AdminUserDetail>("/api/admin/system/admin-users", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function updateAdminUser(id: number, payload: UpdateAdminUserPayload): Promise<AdminUserDetail> {
  return request<AdminUserDetail>(`/api/admin/system/admin-users/${id}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

export function assignAdminUserRoles(id: number, roleIds: number[]): Promise<{ id: number; roles: RoleRef[] }> {
  return request<{ id: number; roles: RoleRef[] }>(`/api/admin/system/admin-users/${id}/roles`, {
    method: "PUT",
    body: JSON.stringify({ roleIds })
  });
}

export function resetAdminUserPassword(id: number, newPassword: string): Promise<{ id: number; reset: boolean }> {
  return request<{ id: number; reset: boolean }>(`/api/admin/system/admin-users/${id}/reset-password`, {
    method: "POST",
    body: JSON.stringify({ newPassword })
  });
}

// ---- 角色 ----

export interface ListRolesQuery {
  page?: number;
  pageSize?: number;
  sort?: string;
  keyword?: string;
}

export function listRoles(query: ListRolesQuery = {}): Promise<Paginated<RoleListItem>> {
  return request<Paginated<RoleListItem>>(`/api/admin/system/roles${buildQuery({ ...query })}`);
}

export function createRole(payload: CreateRolePayload): Promise<RoleDetail> {
  return request<RoleDetail>("/api/admin/system/roles", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function updateRole(id: number, payload: { roleName?: string }): Promise<RoleDetail> {
  return request<RoleDetail>(`/api/admin/system/roles/${id}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

export function deleteRole(id: number): Promise<{ id: number; deleted: boolean }> {
  return request<{ id: number; deleted: boolean }>(`/api/admin/system/roles/${id}`, {
    method: "DELETE"
  });
}

export function assignRolePermissions(
  id: number,
  permissionIds: number[]
): Promise<{ id: number; permissions: PermissionItem[] }> {
  return request<{ id: number; permissions: PermissionItem[] }>(`/api/admin/system/roles/${id}/permissions`, {
    method: "PUT",
    body: JSON.stringify({ permissionIds })
  });
}

// ---- 权限码 ----

export interface ListPermissionsQuery {
  page?: number;
  pageSize?: number;
  keyword?: string;
  all?: boolean;
}

export function listPermissions(query: ListPermissionsQuery = {}): Promise<Paginated<PermissionItem>> {
  return request<Paginated<PermissionItem>>(`/api/admin/system/permissions${buildQuery({ ...query })}`);
}

export function createPermission(payload: CreatePermissionPayload): Promise<PermissionItem> {
  return request<PermissionItem>("/api/admin/system/permissions", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function deletePermission(id: number): Promise<{ id: number; deleted: boolean }> {
  return request<{ id: number; deleted: boolean }>(`/api/admin/system/permissions/${id}`, {
    method: "DELETE"
  });
}
