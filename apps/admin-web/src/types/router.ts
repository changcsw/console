export interface AppRouteMeta {
  title: string;
  icon?: string;
  hidden?: boolean;
  /** 访问该路由所需的权限码（未声明则仅需登录） */
  perm?: string;
  /** 是否允许匿名访问（如登录页 / 403） */
  public?: boolean;
}

declare module "vue-router" {
  // eslint-disable-next-line @typescript-eslint/no-empty-interface
  interface RouteMeta extends AppRouteMeta {}
}
