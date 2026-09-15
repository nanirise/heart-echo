import axios from 'axios'
import type {
  AxiosError,
  AxiosInstance,
  AxiosRequestConfig,
  InternalAxiosRequestConfig,
} from 'axios'

import { useAuthStore } from '@/stores/auth'
import type { TokenPair } from '@/stores/auth'
import { ErrorCode } from '@/types/errcode'
import type { ApiResponse } from '@/types/api'

/** 后端接口基础地址（含 /api/v1 后缀），来自 .env */
const baseURL = import.meta.env.VITE_API_BASE_URL

/**
 * 免鉴权端点：不附加 Authorization 头。
 * `/auth/refresh` 保留在这里是双保险 —— 刷新实际走下面的 refreshClient，
 * 但万一有人拿主实例调它，也不会带上过期的 token。
 */
const WHITE_LIST = ['/auth/login', '/auth/register', '/auth/refresh']

/** 请求没能抵达后端（断网 / 超时 / 跨域）时使用的伪错误码，不属于后端错误码表 */
export const NETWORK_ERROR_CODE = -1

/** 业务错误：把后端的 code 与 message 打包成一个可以 catch 的对象 */
export class ApiError extends Error {
  readonly code: number

  constructor(code: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

/** 本模块内部给请求配置加的重放标记 */
type RetryConfig = InternalAxiosRequestConfig & { _retry?: boolean }

/** 全应用唯一的 axios 实例 */
const instance: AxiosInstance = axios.create({
  baseURL,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
})

/**
 * 刷新令牌专用的「裸」实例：不挂任何拦截器。
 * 这样刷新请求永远不会被自己的拦截器再处理一遍，从根上杜绝死循环。
 */
const refreshClient: AxiosInstance = axios.create({
  baseURL,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
})

// ── 请求拦截器：自动附加 token（白名单除外）──
instance.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const url = config.url ?? ''
  const isWhiteListed = WHITE_LIST.some((path) => url.startsWith(path))

  if (!isWhiteListed) {
    // 延迟获取 store：Pinia 的 store 只能在 app 挂载之后使用。
    // 写在拦截器函数体里，既保证时机正确，也避免与 auth.ts 形成模块级循环依赖。
    const auth = useAuthStore()
    if (auth.accessToken !== null) {
      config.headers.Authorization = `Bearer ${auth.accessToken}`
    }
  }

  return config
})

// ── 刷新令牌 ──

/**
 * 并发锁：同一时刻最多只允许一个刷新请求在飞。
 * 非 null 表示「已经有人在刷新了」，后来的请求直接复用同一个 Promise。
 */
let refreshPromise: Promise<string> | null = null

/** 真正去换新令牌；成功则写入 store，并返回新的 accessToken */
async function doRefresh(): Promise<string> {
  const auth = useAuthStore()

  if (auth.refreshToken === null) {
    throw new ApiError(ErrorCode.ErrRefreshInvalid, '本地没有刷新令牌')
  }

  const response = await refreshClient.post<ApiResponse<TokenPair>>('/auth/refresh', {
    refreshToken: auth.refreshToken,
  })

  const body = response.data
  if (body.code !== ErrorCode.Success) {
    throw new ApiError(body.code, body.message)
  }

  // 契约 §3.3：刷新会同时换发新的 accessToken 与 refreshToken，两个都要更新
  auth.setTokens(body.data)
  return body.data.accessToken
}

/** 带并发锁的刷新入口：多个请求同时过期时，只会真正刷新一次 */
async function refreshAccessToken(): Promise<string> {
  // 已经有人在刷新 —— 复用它的结果，排队等就行
  if (refreshPromise !== null) {
    return refreshPromise
  }

  const pending = doRefresh()
  refreshPromise = pending

  try {
    return await pending
  } finally {
    // 无论成功还是失败，都要把锁摘掉；否则后面的刷新会被永久卡住
    refreshPromise = null
  }
}

/** 清空登录态并整页跳转到登录页 */
function forceLogout(): void {
  const auth = useAuthStore()
  auth.clearAuth()
  // 用整页跳转而不是 router.push：request.ts 是底层模块，
  // 让它依赖路由层会造成反向依赖；整页跳转还能顺带清掉内存里的残留状态。
  window.location.assign('/login')
}

// ── 响应拦截器：业务码分发 ──
instance.interceptors.response.use(
  // ① HTTP 2xx：按契约这里只可能是 code 200，原样放行（解包在下面的 http() 里做）
  (response) => {
    const body = response.data as ApiResponse
    if (body.code === ErrorCode.Success) {
      return response
    }
    // 兜底：万一后端某次改动让 2xx 带上了非 200 的 code
    return Promise.reject(new ApiError(body.code, body.message))
  },

  // ② 非 2xx：后端的业务错误体在这个分支里
  async (error: AxiosError<ApiResponse>) => {
    const body = error.response?.data
    const originalConfig = error.config as RetryConfig | undefined

    // 请求根本没到后端：断网、超时、跨域，或被代理拦掉了
    if (body === undefined || typeof body.code !== 'number') {
      return Promise.reject(new ApiError(NETWORK_ERROR_CODE, error.message))
    }

    const { code, message } = body

    // 4012：access token 过期 —— 刷新后重放原请求（同一个请求只重放一次）
    if (
      code === ErrorCode.ErrTokenExpired &&
      originalConfig !== undefined &&
      originalConfig._retry !== true
    ) {
      originalConfig._retry = true

      try {
        await refreshAccessToken()
        return await instance.request(originalConfig)
      } catch {
        forceLogout()
        return Promise.reject(new ApiError(code, message))
      }
    }

    // 4010 / 4011 / 4014：登录态彻底失效 —— 清空并跳登录页，不重试
    if (
      code === ErrorCode.ErrUnauthorized ||
      code === ErrorCode.ErrTokenInvalid ||
      code === ErrorCode.ErrRefreshInvalid
    ) {
      forceLogout()
    }

    // 其余错误按 message 提示
    return Promise.reject(new ApiError(code, message))
  },
)

/**
 * 统一请求入口：返回**已解包**的业务数据。
 * 调用方拿到的就是契约里各端点的「响应 data」，不需要再判断 code。
 */
async function http<T>(config: AxiosRequestConfig): Promise<T> {
  const response = await instance.request<ApiResponse<T>>(config)
  return response.data.data
}

export const request = {
  get: <T>(url: string, config?: AxiosRequestConfig): Promise<T> =>
    http<T>({ ...config, url, method: 'GET' }),

  post: <T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> =>
    http<T>({ ...config, url, method: 'POST', data }),

  put: <T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> =>
    http<T>({ ...config, url, method: 'PUT', data }),

  delete: <T>(url: string, config?: AxiosRequestConfig): Promise<T> =>
    http<T>({ ...config, url, method: 'DELETE' }),
}

/** 原始 axios 实例，留给需要完全自定义配置的特殊场景 */
export default instance
