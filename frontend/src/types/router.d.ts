import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    /** 缺省视为需要登录；免登录页必须显式写 false */
    requiresAuth?: boolean
    title?: string
  }
}
