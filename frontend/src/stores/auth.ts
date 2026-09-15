import { defineStore } from 'pinia'

/**
 * 当前登录用户的公开信息。
 * 对应契约 §3.1 响应中的 user 字段，不包含密码等敏感数据。
 */
export interface UserInfo {
  id: number
  username: string
  email: string
  avatarUrl: string | null
}

/** 认证接口返回的令牌对，对应契约 §3.1 / §3.2 / §3.3 */
export interface TokenPair {
  accessToken: string
  refreshToken: string
}

/**
 * 登录态 store —— 全应用唯一的身份来源。
 *
 * 职责边界：
 * - 只管「存什么」与「改什么」，不负责发请求（发请求是 api/request.ts 的事）
 * - 后续分支的路由守卫依赖 isLogin，布局依赖 user
 */
export const useAuthStore = defineStore('auth', {
  state: () => ({
    /** 短期令牌，附加在每个请求的 Authorization 头 */
    accessToken: null as string | null,
    /** 长期令牌，仅用于换取新的 accessToken */
    refreshToken: null as string | null,
    /** 当前用户信息，未登录为 null */
    user: null as UserInfo | null,
  }),

  getters: {
    /** 是否已登录：以 accessToken 是否存在为准 */
    isLogin: (state): boolean => state.accessToken !== null,
  },

  actions: {
    /** 写入令牌对（登录、注册、刷新成功后调用） */
    setTokens(tokens: TokenPair): void {
      this.accessToken = tokens.accessToken
      this.refreshToken = tokens.refreshToken
    },

    /** 写入用户信息 */
    setUser(user: UserInfo): void {
      this.user = user
    },

    /** 清空登录态（登出、令牌彻底失效时调用） */
    clearAuth(): void {
      this.accessToken = null
      this.refreshToken = null
      this.user = null
    },
  },
})
