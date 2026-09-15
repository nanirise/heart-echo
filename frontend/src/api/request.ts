import axios from 'axios'
import type {
  AxiosError,
  AxiosInstance,
  AxiosRequestConfig,
  InternalAxiosRequestConfig,
} from 'axios'

import { useAuthStore } from '@/stores/auth'
import { ErrorCode } from '@/types/errcode'
import type { ApiResponse } from '@/types/api'

/** 后端接口基础地址（含 /api/v1 后缀），来自 .env */
const baseURL = import.meta.env.VITE_API_BASE_URL

/**
 * 免鉴权端点：不附加 Authorization 头。
 * `/auth/refresh` 也在其中 —— 它靠 refreshToken 认证，
 * 且必须避免「刷新请求自身又过期」造成的死循环。
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

/** 全应用唯一的 axios 实例 */
const instance: AxiosInstance = axios.create({
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
  (error: AxiosError<ApiResponse>) => {
    const body = error.response?.data

    // 契约 §2：HTTP 状态码与业务 code 分离，业务错误要从 body 里取
    if (body !== undefined && typeof body.code === 'number') {
      // TODO 第 4 步：code === ErrorCode.ErrTokenExpired 时刷新令牌并重放原请求
      // TODO 第 5 步：code 是 4010 / 4011 / 4014 时清空登录态并跳登录页
      return Promise.reject(new ApiError(body.code, body.message))
    }

    // 请求根本没到后端：断网、超时、跨域，或被代理拦掉了
    return Promise.reject(new ApiError(NETWORK_ERROR_CODE, error.message))
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
