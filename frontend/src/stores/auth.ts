import { defineStore } from 'pinia'

import { refreshAccessToken, request } from '@/api/request'

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

/** 登录 / 注册成功的响应：令牌对 + 用户信息（契约 §3.1 / §3.2） */
export interface AuthResult extends TokenPair {
  user: UserInfo
}

/** 登录请求体（契约 §3.2） */
export interface LoginPayload {
  username: string
  password: string
}

/** 注册请求体（契约 §3.1） */
export interface RegisterPayload {
  username: string
  email: string
  password: string
}

/**
 * 登录态 store —— 全应用唯一的身份来源。
 *
 * 职责边界：
 * - 只管「登录态存什么、怎么改」
 * - 发请求的细节交给 `api/request.ts`（拦截器会自动附加 token）
 * - 后续分支的路由守卫依赖 `isLogin`，布局依赖 `user`
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
    // ── 纯数据操作 ──

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

    // ── 业务动作 ──

    /** 登录：成功后写入登录态（契约 §3.2） */
    async login(payload: LoginPayload): Promise<void> {
      const result = await request.post<AuthResult>('/auth/login', payload)
      this.setTokens(result)
      this.setUser(result.user)
    },

    /** 注册：契约 §3.1「注册即登录」，直接写入登录态，不再走登录页 */
    async register(payload: RegisterPayload): Promise<void> {
      const result = await request.post<AuthResult>('/auth/register', payload)
      this.setTokens(result)
      this.setUser(result.user)
    },

    /**
     * 登出：只清空本地登录态。
     * 后端没有登出端点（契约中不存在），token 会自然过期；跳转由调用方负责。
     */
    logout(): void {
      this.clearAuth()
    },

    /**
     * 主动刷新令牌（例如应用启动时发现 token 快过期）。
     * 复用 `request.ts` 里带并发锁的实现，保证全局只有一条刷新路径。
     */
    async refresh(): Promise<string> {
      return refreshAccessToken()
    },
  },

  // 持久化：把指定字段写入 localStorage，刷新页面后仍保持登录
  persist: {
    key: 'heart-echo-auth',
    paths: ['accessToken', 'refreshToken', 'user'],
  },
})
